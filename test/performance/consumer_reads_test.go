//go:build performance

package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/consumer"
	"github.com/project-kessel/inventory-api/internal/data"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ProcessMessage receives events directly; the Kafka poll loop is outside
// this workload, so its constructor only needs an inert consumer.
type inertKafkaConsumer struct{}

func (inertKafkaConsumer) CommitOffsets(offsets []kafka.TopicPartition) ([]kafka.TopicPartition, error) {
	return offsets, nil
}
func (inertKafkaConsumer) SubscribeTopics([]string, kafka.RebalanceCb) error { return nil }
func (inertKafkaConsumer) Poll(int) kafka.Event                              { return nil }
func (inertKafkaConsumer) IsClosed() bool                                    { return false }
func (inertKafkaConsumer) Close() error                                      { return nil }
func (inertKafkaConsumer) AssignmentLost() bool                              { return false }

// observedRelations delegates to the stateless allow-all backend and verifies
// that each event type reaches the tuple operations expected of it.
type observedRelations struct {
	model.RelationsRepository
	creates atomic.Int64
	deletes atomic.Int64
}

func (r *observedRelations) CreateTuples(ctx context.Context, tuples []model.RelationsTuple, upsert bool, fencing *model.FencingCheck) (model.TuplesResult, error) {
	r.creates.Add(1)
	return r.RelationsRepository.CreateTuples(ctx, tuples, upsert, fencing)
}

func (r *observedRelations) DeleteTuples(ctx context.Context, filter model.TupleFilter, fencing *model.FencingCheck) (model.TuplesResult, error) {
	r.deletes.Add(1)
	return r.RelationsRepository.DeleteTuples(ctx, filter, fencing)
}

type consumerCase struct {
	name        string
	message     *kafka.Message
	headers     map[string]string
	wantCreates int64
	wantDeletes int64
}

func newConsumerCase(t *testing.T, key model.ReporterResourceKey, name string, operation model.EventOperationType, commonVersion *model.Version, creates, deletes int64) consumerCase {
	t.Helper()
	event, err := model.NewTupleEvent(key, commonVersion, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"payload": event})
	if err != nil {
		t.Fatal(err)
	}
	return consumerCase{
		name:        name,
		message:     &kafka.Message{Value: payload},
		headers:     map[string]string{"operation": string(operation.OperationType()), "txid": "performance-" + name},
		wantCreates: creates,
		wantDeletes: deletes,
	}
}

// runConsumerReads times the consumer's common-representation paths on this
// branch. Reporter history remains in the fixture but this API does not read it.
func runConsumerReads(t *testing.T, ctx context.Context, seedDB *gorm.DB, dsn string, s settings) {
	t.Helper()
	if err := resetAndSeed(ctx, seedDB, s, "history", 1); err != nil {
		t.Fatal(err)
	}
	// v0 -> v1 changes workspace; v1 -> v2 keeps it unchanged. Delete
	// fetches the latest (v2) representation.
	if err := seedDB.WithContext(ctx).Exec(`
		INSERT INTO common_representations (resource_id, version, data,
			reported_by_reporter_type, reported_by_reporter_instance, transaction_id, created_at)
		SELECT rr.resource_id, v.version, jsonb_build_object('workspace_id', v.workspace),
			'hbi', 'perf', 'consumer-common-' || v.version, now()
		FROM reporter_resources rr
		CROSS JOIN (VALUES (0, 'perf-previous'), (2, 'perf')) AS v(version, workspace)
		WHERE rr.local_resource_id = ?`, "perf-history-0").Error; err != nil {
		t.Fatal(err)
	}
	if err := seedDB.WithContext(ctx).Exec("UPDATE resource SET common_version = 2 WHERE id = (SELECT resource_id FROM reporter_resources WHERE local_resource_id = ?)", "perf-history-0").Error; err != nil {
		t.Fatal(err)
	}
	if err := seedDB.WithContext(ctx).Exec("ANALYZE common_representations").Error; err != nil {
		t.Fatal(err)
	}
	databaseBytes, tableBytes, err := databaseFootprint(ctx, seedDB)
	if err != nil {
		t.Fatal(err)
	}

	parallelWorkers := envInt("PERF_READ_PARALLEL_WORKERS", 0)
	iterations := envInt("PERF_READ_ITERATIONS", 30)
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

	consumerOptions := consumer.NewOptions()
	consumerOptions.BootstrapServers = []string{"localhost:9092"}
	consumerConfig, configErrors := consumer.NewConfig(consumerOptions).Complete()
	if len(configErrors) > 0 {
		t.Fatalf("consumer config: %v", configErrors)
	}
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	relations := &observedRelations{RelationsRepository: data.NewAllowAllRelationsRepository(logger)}
	// Enable tuple replication while keeping its backend stateless.
	inventoryConsumer, err := consumer.New(consumerConfig, readDB, data.NewInMemorySchemaRepository(), relations, true, nil, logger, inertKafkaConsumer{})
	if err != nil {
		t.Fatal(err)
	}
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
	commonOne, commonTwo := model.NewVersion(1), model.NewVersion(2)
	cases := []consumerCase{
		newConsumerCase(t, key, "create", model.OperationTypeCreated, &commonOne, 1, 0),
		newConsumerCase(t, key, "update_common", model.OperationTypeUpdated, &commonOne, 1, 1),
		newConsumerCase(t, key, "update_unchanged", model.OperationTypeUpdated, &commonTwo, 0, 0),
		newConsumerCase(t, key, "delete", model.OperationTypeDeleted, &commonTwo, 0, 1),
	}
	process := func(testCase consumerCase) error {
		response, err := inventoryConsumer.ProcessMessage(testCase.headers, true, testCase.message)
		if err != nil {
			return err
		}
		if response != model.MinimizeLatencyToken.Serialize() {
			return fmt.Errorf("unexpected allow-all consistency token %q", response)
		}
		return nil
	}
	for _, testCase := range cases {
		beforeCreates, beforeDeletes := relations.creates.Load(), relations.deletes.Load()
		if err := process(testCase); err != nil {
			t.Fatalf("preflight %s: %v", testCase.name, err)
		}
		gotCreates := relations.creates.Load() - beforeCreates
		gotDeletes := relations.deletes.Load() - beforeDeletes
		if gotCreates != testCase.wantCreates || gotDeletes != testCase.wantDeletes {
			t.Fatalf("preflight %s: create %d/%d, delete %d/%d", testCase.name, gotCreates, testCase.wantCreates, gotDeletes, testCase.wantDeletes)
		}
	}

	for _, clients := range []int{1, 4, 8} {
		t.Run(fmt.Sprintf("clients_%d", clients), func(t *testing.T) {
			var mu sync.Mutex
			samples := make(map[string][]time.Duration)
			var failures []string
			var ready, finished sync.WaitGroup
			start := make(chan struct{})
			for worker := 0; worker < clients; worker++ {
				ready.Add(1)
				finished.Add(1)
				go func(worker int) {
					defer finished.Done()
					for _, testCase := range cases {
						for warmup := 0; warmup < 3; warmup++ {
							if err := process(testCase); err != nil {
								mu.Lock()
								failures = append(failures, fmt.Sprintf("worker %d warmup %s: %v", worker, testCase.name, err))
								mu.Unlock()
								ready.Done()
								return
							}
						}
					}
					ready.Done()
					<-start
					for iteration := 0; iteration < iterations; iteration++ {
						for _, testCase := range cases {
							began := time.Now()
							err := process(testCase)
							elapsed := time.Since(began)
							mu.Lock()
							if err != nil {
								failures = append(failures, fmt.Sprintf("worker %d iteration %d %s: %v", worker, iteration, testCase.name, err))
							} else {
								samples[testCase.name] = append(samples[testCase.name], elapsed)
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
			r := result{Protocol: "consumer", Scenario: "consumer", Concurrency: clients, DatabaseBytes: databaseBytes, TableBytes: tableBytes, Settings: s, Operations: make(map[string]stats), Errors: failures, Revision: revision(), GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GOMAXPROCS: runtime.GOMAXPROCS(0), Image: image, MaxOpenConnections: 16, ReadIterations: iterations, ReadParallelWorkers: &parallelWorkers}
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
				t.Fatalf("consumer messages failed: %v", failures)
			}
			for _, testCase := range cases {
				if got := r.Operations[testCase.name].Count; got != clients*iterations {
					t.Errorf("%s: got %d timed samples, want %d", testCase.name, got, clients*iterations)
				}
			}
			if err := checkBaseline(r); err != nil {
				t.Error(err)
			}
		})
	}
}
