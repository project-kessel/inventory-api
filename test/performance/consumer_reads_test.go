//go:build performance

package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// runConsumerReads measures the same repository method used by the CDC
// consumer. It excludes Kafka delivery, tuple calculation, and Relations API
// calls so the database lookup can be compared with the reporter-query fixture.
func runConsumerReads(t *testing.T, ctx context.Context, seedDB *gorm.DB, dsn string, s settings) {
	t.Helper()
	if err := resetAndSeed(ctx, seedDB, s, "history", 1); err != nil {
		t.Fatal(err)
	}
	// Add common history and a revival generation. Keep the old generation's
	// tombstone so upper-bound reads must distinguish generation from version.
	if err := seedDB.WithContext(ctx).Exec(`
		INSERT INTO common_representations (resource_id,version,data,reported_by_reporter_type,reported_by_reporter_instance,transaction_id,created_at)
		SELECT rr.resource_id,n,jsonb_build_object('workspace_id','perf-'||n),
			'hbi','perf','consumer-common-'||n,now()
		FROM reporter_resources rr CROSS JOIN generate_series(0,?) n
		WHERE rr.local_resource_id='perf-history-0' AND n<>1`, s.History-1).Error; err != nil {
		t.Fatal(err)
	}
	if err := seedDB.WithContext(ctx).Exec(`
		INSERT INTO reporter_representations (reporter_resource_id,version,generation,data,common_version,transaction_id,tombstone,created_at)
		SELECT id,v.version,1,jsonb_build_object('payload','revived','sequence',v.version),?,
			'consumer-revival-'||v.version,v.version=2,now()
		FROM reporter_resources CROSS JOIN (VALUES (0),(1),(2)) v(version)
		WHERE local_resource_id='perf-history-0'`, s.History-1).Error; err != nil {
		t.Fatal(err)
	}
	if err := seedDB.WithContext(ctx).Exec("UPDATE resource SET common_version=? WHERE id=(SELECT resource_id FROM reporter_resources WHERE local_resource_id='perf-history-0')", s.History-1).Error; err != nil {
		t.Fatal(err)
	}
	if err := seedDB.WithContext(ctx).Exec("UPDATE reporter_resources SET generation=1,representation_version=2 WHERE local_resource_id='perf-history-0'").Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"resource", "common_representations", "reporter_resources", "reporter_representations"} {
		if err := seedDB.WithContext(ctx).Exec("ANALYZE " + table).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, fixture := range []struct {
		table string
		want  int64
	}{
		{"common_representations", int64(s.Background + s.History)},
		{"reporter_representations", int64(s.Background + s.History + 3)},
	} {
		var count int64
		if err := seedDB.WithContext(ctx).Table(fixture.table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != fixture.want {
			t.Fatalf("%s seed: got %d rows, want %d", fixture.table, count, fixture.want)
		}
	}
	databaseBytes, tableBytes, err := databaseFootprint(ctx, seedDB)
	if err != nil {
		t.Fatal(err)
	}

	parallelWorkers := envInt(t, "PERF_READ_PARALLEL_WORKERS", 0)
	iterations := envInt(t, "PERF_READ_ITERATIONS", 30)
	if parallelWorkers < 0 || parallelWorkers > 8 || iterations < 1 {
		t.Fatal("PERF_READ_PARALLEL_WORKERS must be 0..8 and PERF_READ_ITERATIONS must be positive")
	}
	readDSN := fmt.Sprintf("%s options='-c max_parallel_workers_per_gather=%d'", dsn, parallelWorkers)
	readDB, err := gorm.Open(postgres.Open(readDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := readDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(16)
	t.Cleanup(func() { _ = sqlDB.Close() })
	var configured string
	if err := readDB.WithContext(ctx).Raw("SHOW max_parallel_workers_per_gather").Scan(&configured).Error; err != nil {
		t.Fatal(err)
	}
	if configured != strconv.Itoa(parallelWorkers) {
		t.Fatalf("read connection parallel workers: got %s, want %d", configured, parallelWorkers)
	}
	if err := savePreviousReporterPlan(ctx, readDB, s); err != nil {
		t.Fatal(err)
	}
	repo := data.NewResourceRepository(readDB, data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 10), nil)
	localID, err := model.NewLocalResourceId("perf-history-0")
	if err != nil {
		t.Fatal(err)
	}
	resourceType, err := model.NewResourceType("host")
	if err != nil {
		t.Fatal(err)
	}
	reporterType, err := model.NewReporterType("hbi")
	if err != nil {
		t.Fatal(err)
	}
	instanceID, err := model.NewReporterInstanceId("perf")
	if err != nil {
		t.Fatal(err)
	}
	key, err := model.NewReporterResourceKey(localID, resourceType, reporterType, instanceID)
	if err != nil {
		t.Fatal(err)
	}
	commonVersion := model.NewVersion(uint(s.History - 1))
	// The final seeded version is a tombstone; use the preceding live version.
	reporterVersion := model.NewVersion(uint(s.History - 1))
	generation := model.NewGeneration(0)
	commonOnly := model.NewRepresentationVersions(&commonVersion, nil, nil)
	reporterOnly := model.NewRepresentationVersions(nil, &reporterVersion, &generation)
	combined := model.NewRepresentationVersions(&commonVersion, &reporterVersion, &generation)
	reads := map[string]func(context.Context) error{
		"resource": func(requestCtx context.Context) error {
			resource, err := repo.FindResourceByKeys(readDB.WithContext(requestCtx), key)
			if err != nil {
				return err
			}
			if resource == nil {
				return fmt.Errorf("resource lookup returned nil")
			}
			return nil
		},
		"common": func(requestCtx context.Context) error {
			current, previous, err := repo.FindCurrentAndPreviousVersionedRepresentations(readDB.WithContext(requestCtx), key, commonOnly, model.OperationTypeUpdated)
			if err != nil {
				return err
			}
			if current == nil || !current.HasCommon() || current.CommonVersion().Uint() != uint(s.History-1) || previous == nil || !previous.HasCommon() || previous.CommonVersion().Uint() != uint(s.History-2) {
				return fmt.Errorf("common current/previous lookup returned incomplete state")
			}
			return nil
		},
		"reporter_current": func(requestCtx context.Context) error {
			current, _, err := repo.FindCurrentAndPreviousVersionedRepresentations(readDB.WithContext(requestCtx), key, reporterOnly, model.OperationTypeCreated)
			if err != nil {
				return err
			}
			if current == nil || !current.HasReporter() || current.ReporterVersion().Uint() != uint(s.History-1) {
				return fmt.Errorf("current reporter lookup returned incomplete state")
			}
			return nil
		},
		"reporter_previous": func(requestCtx context.Context) error {
			current, previous, err := repo.FindCurrentAndPreviousVersionedRepresentations(readDB.WithContext(requestCtx), key, reporterOnly, model.OperationTypeUpdated)
			if err != nil {
				return err
			}
			if current == nil || !current.HasReporter() || current.ReporterVersion().Uint() != uint(s.History-1) || previous == nil || !previous.HasReporter() || previous.ReporterVersion().Uint() != uint(s.History-2) {
				return fmt.Errorf("previous reporter lookup returned incomplete state")
			}
			return nil
		},
		"combined": func(requestCtx context.Context) error {
			current, previous, err := repo.FindCurrentAndPreviousVersionedRepresentations(readDB.WithContext(requestCtx), key, combined, model.OperationTypeUpdated)
			if err != nil {
				return err
			}
			if current == nil || !current.HasCommon() || !current.HasReporter() || current.CommonVersion().Uint() != uint(s.History-1) || current.ReporterVersion().Uint() != uint(s.History-1) || previous == nil || !previous.HasCommon() || !previous.HasReporter() || previous.CommonVersion().Uint() != uint(s.History-2) || previous.ReporterVersion().Uint() != uint(s.History-2) {
				return fmt.Errorf("combined current/previous lookup returned incomplete state")
			}
			return nil
		},
	}
	// Revival must return the previous generation's tombstone; delete must
	// skip the current generation's tombstone and return its last live row.
	revivedVersion, tombstoneVersion := model.NewVersion(0), model.NewVersion(2)
	revivedGeneration := model.NewGeneration(1)
	for _, testCase := range []struct {
		name      string
		version   model.Version
		operation model.EventOperationType
		want      uint
		previous  bool
	}{
		{"reporter_revival", revivedVersion, model.OperationTypeUpdated, uint(s.History), true},
		{"reporter_delete", tombstoneVersion, model.OperationTypeDeleted, 1, false},
	} {
		reads[testCase.name] = func(requestCtx context.Context) error {
			versions := model.NewRepresentationVersions(nil, &testCase.version, &revivedGeneration)
			current, previous, err := repo.FindCurrentAndPreviousVersionedRepresentations(readDB.WithContext(requestCtx), key, versions, testCase.operation)
			if err != nil {
				return err
			}
			result := current
			if testCase.previous {
				result = previous
			}
			if current == nil || !current.HasReporter() || result == nil || !result.HasReporter() || result.ReporterVersion().Uint() != testCase.want {
				return fmt.Errorf("%s returned incorrect reporter version", testCase.name)
			}
			return nil
		}
	}
	operations := []string{"resource", "common", "reporter_current", "reporter_previous", "combined", "reporter_revival", "reporter_delete"}
	for _, workers := range []int{1, 4, 8} {
		t.Run(fmt.Sprintf("clients_%d", workers), func(t *testing.T) {
			var mu sync.Mutex
			samples := make(map[string][]time.Duration)
			var failures []string
			var ready, finished sync.WaitGroup
			start := make(chan struct{})
			for worker := 0; worker < workers; worker++ {
				ready.Add(1)
				finished.Add(1)
				go func(worker int) {
					defer finished.Done()
					for _, operation := range operations {
						for warmup := 0; warmup < 3; warmup++ {
							requestCtx, cancel := context.WithTimeout(ctx, s.Deadline)
							err := reads[operation](requestCtx)
							cancel()
							if err != nil {
								mu.Lock()
								failures = append(failures, fmt.Sprintf("worker %d warmup %s: %v", worker, operation, err))
								mu.Unlock()
								ready.Done()
								return
							}
						}
					}
					ready.Done()
					<-start
					for iteration := 0; iteration < iterations; iteration++ {
						for _, operation := range operations {
							requestCtx, cancel := context.WithTimeout(ctx, s.Deadline)
							began := time.Now()
							err := reads[operation](requestCtx)
							elapsed := time.Since(began)
							cancel()
							mu.Lock()
							if err != nil {
								failures = append(failures, fmt.Sprintf("worker %d iteration %d %s: %v", worker, iteration, operation, err))
							} else {
								samples[operation] = append(samples[operation], elapsed)
							}
							mu.Unlock()
							if err != nil {
								return
							}
						}
					}
				}(worker)
			}
			ready.Wait()
			began := time.Now()
			close(start)
			finished.Wait()
			r := result{Protocol: "repository", Scenario: "consumer", Concurrency: workers, DatabaseBytes: databaseBytes, TableBytes: tableBytes, Settings: s, Operations: make(map[string]stats), Errors: failures, Revision: revision(), GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GOMAXPROCS: runtime.GOMAXPROCS(0), Image: image, MaxOpenConnections: 16, ReadIterations: iterations, ReadParallelWorkers: &parallelWorkers}
			count := 0
			for operation, durations := range samples {
				r.Operations[operation] = summarize(durations)
				count += len(durations)
			}
			r.Throughput = float64(count) / time.Since(began).Seconds()
			if err := writeResult(r); err != nil {
				t.Error(err)
			}
			if len(failures) > 0 {
				t.Fatalf("consumer reads failed: %v", failures)
			}
			for _, operation := range operations {
				if got := r.Operations[operation].Count; got != workers*iterations {
					t.Errorf("%s: got %d timed samples, want %d", operation, got, workers*iterations)
				}
			}
			if err := checkBaseline(r); err != nil {
				t.Error(err)
			}
		})
	}
}

// savePreviousReporterPlan profiles the same previous-representation predicate
// used by the repository. EXPLAIN ANALYZE executes the query, so keep it out of
// timed samples and do not treat its execution time as a request measurement.
func savePreviousReporterPlan(ctx context.Context, db *gorm.DB, s settings) error {
	var plan string
	query := `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)
		SELECT rrep.data, rrep.version
		FROM reporter_resources rr
		JOIN reporter_representations rrep ON rr.id = rrep.reporter_resource_id
		WHERE rr.local_resource_id = ? AND rr.resource_type = ?
			AND rr.reporter_type = ? AND rr.reporter_instance_id = ?
			AND ((rrep.generation = ? AND rrep.version < ?) OR rrep.generation < ?)
		ORDER BY rrep.generation DESC, rrep.version DESC LIMIT 1`
	if err := db.WithContext(ctx).Raw(query, "perf-history-0", "host", "hbi", "perf", 0, s.History-1, 0).Scan(&plan).Error; err != nil {
		return fmt.Errorf("explain previous reporter lookup: %w", err)
	}
	if !json.Valid([]byte(plan)) {
		return fmt.Errorf("invalid JSON from previous reporter EXPLAIN")
	}
	dir := os.Getenv("PERF_RESULTS_DIR")
	if dir == "" {
		dir = "results"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "consumer-previous-plan.json"), []byte(plan), 0644)
}
