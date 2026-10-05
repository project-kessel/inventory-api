//go:build performance

package performance

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	krtransport "github.com/go-kratos/kratos/v2/transport"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/google/uuid"
	"github.com/moby/moby/api/types/container"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	usecase "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	servergrpc "github.com/project-kessel/inventory-api/internal/server/grpc"
	service "github.com/project-kessel/inventory-api/internal/service/resources"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const image = "postgres:17.8"

//go:embed baseline.json
var defaultBaseline []byte

type settings struct {
	Background      int           `json:"background"`
	History         int           `json:"history"`
	PayloadBytes    int           `json:"payload_bytes"`
	Rounds          int           `json:"rounds"`
	Updates         int           `json:"updates"`
	Deadline        time.Duration `json:"-"`
	DeadlineSeconds int           `json:"request_timeout_seconds"`
	Profile         string        `json:"profile"`
}
type stats struct {
	Count  int     `json:"count"`
	MinMS  float64 `json:"min_ms"`
	MeanMS float64 `json:"mean_ms"`
	P50MS  float64 `json:"p50_ms"`
	P95MS  float64 `json:"p95_ms"`
	P99MS  float64 `json:"p99_ms"`
	MaxMS  float64 `json:"max_ms"`
}
type result struct {
	Protocol            string           `json:"protocol"`
	Scenario            string           `json:"scenario"`
	Concurrency         int              `json:"concurrency"`
	DatabaseBytes       int64            `json:"database_bytes"`
	TableBytes          map[string]int64 `json:"table_bytes"`
	Settings            settings         `json:"settings"`
	Operations          map[string]stats `json:"operations"`
	Throughput          float64          `json:"throughput_per_second"`
	Errors              []string         `json:"errors,omitempty"`
	Revision            string           `json:"revision"`
	GoVersion           string           `json:"go_version"`
	GOOS                string           `json:"goos"`
	GOARCH              string           `json:"goarch"`
	GOMAXPROCS          int              `json:"gomaxprocs"`
	Image               string           `json:"postgres_image"`
	MaxOpenConnections  int              `json:"max_open_connections"`
	ReadIterations      int              `json:"read_iterations,omitempty"`
	ReadParallelWorkers *int             `json:"read_parallel_workers,omitempty"`
}

type authenticator struct{}

func (authenticator) Authenticate(context.Context, krtransport.Transporter) (*authnapi.Claims, authnapi.Decision) {
	return &authnapi.Claims{SubjectId: "performance", AuthType: authnapi.AuthTypeXRhIdentity}, authnapi.Allow
}

type authorizer struct{}

func (authorizer) Check(_ context.Context, _ metaauthorizer.MetaObject, relation metaauthorizer.Relation, _ authnapi.AuthzContext) (bool, error) {
	return relation == metaauthorizer.RelationReportResource || relation == metaauthorizer.RelationDeleteResource, nil
}

func envInt(t *testing.T, name string, fallback int) int {
	t.Helper()
	value, set := os.LookupEnv(name)
	if !set {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		t.Fatalf("%s must be a non-negative integer, got %q", name, value)
	}
	return n
}
func config(t *testing.T) settings {
	t.Helper()
	s := settings{Background: 100000, History: 227872, PayloadBytes: 1024, Rounds: 20, Updates: 3, Deadline: 10 * time.Second, Profile: os.Getenv("PERF_PROFILE")}
	if s.Profile == "small" {
		s.Background = 100
		s.History = 40
		s.PayloadBytes = 128
		s.Rounds = 3
	}
	s.Background = envInt(t, "PERF_BACKGROUND", s.Background)
	s.History = envInt(t, "PERF_HISTORY", s.History)
	s.PayloadBytes = envInt(t, "PERF_PAYLOAD_BYTES", s.PayloadBytes)
	s.Rounds = envInt(t, "PERF_ROUNDS", s.Rounds)
	s.Updates = envInt(t, "PERF_UPDATES", s.Updates)
	s.Deadline = time.Duration(envInt(t, "PERF_REQUEST_TIMEOUT_SECONDS", 10)) * time.Second
	s.DeadlineSeconds = int(s.Deadline / time.Second)
	if s.Rounds < 1 || s.Updates < 1 || s.History < 4 || s.Deadline <= 0 {
		t.Fatal("rounds, updates, and deadline must be positive; history must be at least four")
	}
	return s
}

func TestResourceLifecyclePerformance(t *testing.T) {
	s := config(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	password := uuid.NewString()
	ctr, err := testcontainers.Run(ctx, image, testcontainers.WithEnv(map[string]string{"POSTGRES_USER": "perf", "POSTGRES_PASSWORD": password, "POSTGRES_DB": "perf"}), testcontainers.WithExposedPorts("5432/tcp"), testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")), testcontainers.WithHostConfigModifier(func(h *container.HostConfig) { h.ShmSize = 256 << 20 }))
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		cleanup, c := context.WithTimeout(context.Background(), time.Minute)
		defer c()
		_ = ctr.Terminate(cleanup)
	})
	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	dsn := fmt.Sprintf("host=%s port=%s user=perf password=%s dbname=perf sslmode=disable", host, port.Port(), password)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(16)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := data.Migrate(db, nil); err != nil {
		t.Fatal(err)
	}
	// Migrations bind a GORM session to a dedicated advisory-lock connection.
	// Use a fresh handle for requests after that connection has been released.
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	runtimeSQLDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	runtimeSQLDB.SetMaxOpenConns(16)
	runtimeSQLDB.SetMaxIdleConns(16)
	t.Cleanup(func() { _ = runtimeSQLDB.Close() })
	schema := data.NewInMemorySchemaRepository()
	schemaJSON := data.NewJsonSchemaWithWorkspacesFromString(`{"type":"object"}`)
	resourceType, _ := model.NewResourceType("host")
	reporterType, _ := model.NewReporterType("hbi")
	rs, err := model.NewResourceSchemaRepresentation(resourceType, schemaJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err = schema.CreateResourceSchema(ctx, rs); err != nil {
		t.Fatal(err)
	}
	rr, err := model.NewReporterSchemaRepresentation(resourceType, reporterType, schemaJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err = schema.CreateReporterSchema(ctx, rr); err != nil {
		t.Fatal(err)
	}
	mc := metricscollector.NewFakeMetricsCollector()
	repo := data.NewResourceRepository(db, data.NewGormTransactionManager(mc, 100), func(*gorm.DB, *model_legacy.OutboxEvent) error { return nil })
	uc := usecase.New(repo, schema, data.NewSimpleRelationsRepository(), "rbac", log.NewStdLogger(io.Discard), nil, nil, usecase.NewUsecaseConfig(), mc, authorizer{}, nil)
	validator, err := protovalidate.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Run("grpc", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		srv, err := servergrpc.NewWithDeps(servergrpc.ServerConfig{Authenticator: authenticator{}, Logger: log.NewStdLogger(io.Discard), Metrics: metrics.Server(), Validator: validator, Meter: otel.GetMeterProvider().Meter("performance"), ServerOptions: []kgrpc.ServerOption{kgrpc.Listener(listener)}})
		if err != nil {
			t.Fatal(err)
		}
		pb.RegisterKesselInventoryServiceServer(srv, service.NewKesselInventoryServiceV1beta2(uc))
		startErr := make(chan error, 1)
		go func() { startErr <- srv.Start(context.Background()) }()
		t.Cleanup(func() { _ = srv.Stop(context.Background()) })
		conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		client := pb.NewKesselInventoryServiceClient(conn)
		invoke := func(ctx context.Context, op, id string, step int) error {
			if op == "delete" {
				_, err := client.DeleteResource(ctx, deleteRequest(id))
				return err
			}
			_, err := client.ReportResource(ctx, reportRequest(id, step, s.PayloadBytes))
			return err
		}
		healthCtx, done := context.WithTimeout(ctx, 10*time.Second)
		defer done()
		if err := invoke(healthCtx, "create", uuid.NewString(), 0); err != nil {
			select {
			case serverErr := <-startErr:
				t.Fatalf("service readiness: %v (server start: %v)", err, serverErr)
			default:
			}
			t.Fatalf("service readiness: %v", err)
		}
		for _, scenario := range []string{"fresh", "history"} {
			for _, workers := range []int{1, 4} {
				t.Run(fmt.Sprintf("%s/%d", scenario, workers), func(t *testing.T) {
					if err := resetAndSeed(ctx, db, s, scenario, workers); err != nil {
						t.Fatal(err)
					}
					databaseBytes, tableBytes, err := databaseFootprint(ctx, db)
					if err != nil {
						t.Fatal(err)
					}
					samples := map[string][]time.Duration{}
					var mu sync.Mutex
					var wg sync.WaitGroup
					start := make(chan struct{})
					errors := []string{}
					begin := time.Now()
					for worker := 0; worker < workers; worker++ {
						wg.Add(1)
						go func(worker int) {
							defer wg.Done()
							<-start
							id := fmt.Sprintf("perf-%s-%d", scenario, worker)
							if scenario == "history" {
								id = fmt.Sprintf("perf-history-%d", worker)
							}
							for round := 0; round < s.Rounds; round++ {
								if round > 0 {
									// Restore the tombstone starting state before the next measured cycle.
									prepCtx, cancel := context.WithTimeout(ctx, s.Deadline)
									err := invoke(prepCtx, "delete", id, 0)
									cancel()
									if err != nil {
										mu.Lock()
										errors = append(errors, fmt.Sprintf("worker %d round %d preparation: %v", worker, round, err))
										mu.Unlock()
										return
									}
								}
								ops := []string{"create"}
								for i := 0; i < s.Updates; i++ {
									ops = append(ops, "update")
								}
								ops = append(ops, "delete", "recreate")
								for step, op := range ops {
									requestCtx, c := context.WithTimeout(ctx, s.Deadline)
									started := time.Now()
									err := invoke(requestCtx, op, id, round*len(ops)+step)
									elapsed := time.Since(started)
									c()
									mu.Lock()
									if err != nil {
										errors = append(errors, fmt.Sprintf("worker %d round %d %s: %v", worker, round, op, err))
									} else {
										samples[op] = append(samples[op], elapsed)
									}
									mu.Unlock()
									if err != nil {
										return
									}
								}
							}
						}(worker)
					}
					close(start)
					wg.Wait()
					elapsed := time.Since(begin)
					r := result{Protocol: "grpc", Scenario: scenario, Concurrency: workers, DatabaseBytes: databaseBytes, TableBytes: tableBytes, Settings: s, Operations: map[string]stats{}, Errors: errors, Revision: revision(), GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GOMAXPROCS: runtime.GOMAXPROCS(0), Image: image, MaxOpenConnections: 16}
					total := 0
					for op, v := range samples {
						r.Operations[op] = summarize(v)
						total += len(v)
					}
					r.Throughput = float64(total) / elapsed.Seconds()
					if err := writeResult(r); err != nil {
						t.Error(err)
					}
					if len(errors) > 0 {
						t.Fatalf("request failures: %s", strings.Join(errors, "; "))
					}
					if err := verifyFinal(db, scenario, workers, s); err != nil {
						t.Error(err)
					}
					if err := checkBaseline(r); err != nil {
						t.Error(err)
					}
				})
			}
		}
	})
	t.Run("consumer_reads", func(t *testing.T) {
		runConsumerReads(t, ctx, db, dsn, s)
	})
}

func reportRequest(id string, step, payloadBytes int) *pb.ReportResourceRequest {
	common, _ := structpb.NewStruct(map[string]any{"workspace_id": "perf", "sequence": step})
	reporter, _ := structpb.NewStruct(map[string]any{"payload": strings.Repeat("x", payloadBytes), "sequence": step})
	return &pb.ReportResourceRequest{Type: "host", ReporterType: "hbi", ReporterInstanceId: "perf", Representations: &pb.ResourceRepresentations{Metadata: &pb.RepresentationMetadata{LocalResourceId: id, ApiHref: "/perf/" + id}, Common: common, Reporter: reporter}}
}
func deleteRequest(id string) *pb.DeleteResourceRequest {
	instance := "perf"
	return &pb.DeleteResourceRequest{Reference: &pb.ResourceReference{ResourceType: "host", ResourceId: id, Reporter: &pb.ReporterReference{Type: "hbi", InstanceId: &instance}}}
}

// History is total across targets. Each history target has one current reporter
// resource and many versions of its representation, unlike the SQL-only fixture.
func resetAndSeed(ctx context.Context, db *gorm.DB, s settings, scenario string, workers int) error {
	if err := db.WithContext(ctx).Exec("TRUNCATE TABLE reporter_representations, common_representations, reporter_resources, resource CASCADE").Error; err != nil {
		return err
	}
	payload := strings.Repeat("x", s.PayloadBytes)
	if s.Background > 0 {
		sql := `INSERT INTO resource (id,type,common_version,ktn,created_at,updated_at) SELECT ('00000000-0000-4000-8000-' || lpad(to_hex(n),12,'0'))::uuid,'host',1,'',now(),now() FROM generate_series(1,?) n`
		if e := db.WithContext(ctx).Exec(sql, s.Background).Error; e != nil {
			return e
		}
		sql = `INSERT INTO common_representations (resource_id,version,data,reported_by_reporter_type,reported_by_reporter_instance,transaction_id,created_at) SELECT ('00000000-0000-4000-8000-' || lpad(to_hex(n),12,'0'))::uuid,1,jsonb_build_object('workspace_id','perf'),'hbi','perf','bg-common-'||n,now() FROM generate_series(1,?) n`
		if e := db.WithContext(ctx).Exec(sql, s.Background).Error; e != nil {
			return e
		}
		sql = `INSERT INTO reporter_resources (id,local_resource_id,reporter_type,resource_type,reporter_instance_id,resource_id,api_href,representation_version,generation,tombstone,created_at,updated_at) SELECT ('00000000-0000-4001-8000-' || lpad(to_hex(n),12,'0'))::uuid,'background-'||n,'hbi','host','perf',('00000000-0000-4000-8000-' || lpad(to_hex(n),12,'0'))::uuid,'/perf/background-'||n,1,0,false,now(),now() FROM generate_series(1,?) n`
		if e := db.WithContext(ctx).Exec(sql, s.Background).Error; e != nil {
			return e
		}
		sql = `INSERT INTO reporter_representations (reporter_resource_id,version,generation,data,common_version,transaction_id,tombstone,created_at) SELECT ('00000000-0000-4001-8000-' || lpad(to_hex(n),12,'0'))::uuid,1,0,jsonb_build_object('payload',?::text),1,'bg-reporter-'||n,false,now() FROM generate_series(1,?) n`
		if e := db.WithContext(ctx).Exec(sql, payload, s.Background).Error; e != nil {
			return e
		}
	}
	if scenario == "history" && s.History > 0 {
		for worker := 0; worker < workers; worker++ {
			id := fmt.Sprintf("perf-history-%d", worker)
			count := s.History / workers
			if worker < s.History%workers {
				count++
			}
			if count < 1 {
				continue
			}
			resourceID := uuid.New()
			reporterID := uuid.New()
			if e := db.WithContext(ctx).Exec(`INSERT INTO resource (id,type,common_version,ktn,created_at,updated_at) VALUES (?,'host',1,'',now(),now())`, resourceID).Error; e != nil {
				return e
			}
			if e := db.WithContext(ctx).Exec(`INSERT INTO common_representations (resource_id,version,data,reported_by_reporter_type,reported_by_reporter_instance,transaction_id,created_at) VALUES (?,1,'{"workspace_id":"perf"}'::jsonb,'hbi','perf',?,now())`, resourceID, "history-common-"+id).Error; e != nil {
				return e
			}
			if e := db.WithContext(ctx).Exec(`INSERT INTO reporter_resources (id,local_resource_id,reporter_type,resource_type,reporter_instance_id,resource_id,api_href,representation_version,generation,tombstone,created_at,updated_at) VALUES (?,?,'hbi','host','perf',?,'/perf/history',?,0,true,now(),now())`, reporterID, id, resourceID, count).Error; e != nil {
				return e
			}
			if e := db.WithContext(ctx).Exec(`INSERT INTO reporter_representations (reporter_resource_id,version,generation,data,common_version,transaction_id,tombstone,created_at) SELECT ?,n,0,jsonb_build_object('payload',?::text,'sequence',n),1,?||n,(n=?),now() FROM generate_series(1,?) n`, reporterID, payload, "history-reporter-"+id+"-", count, count).Error; e != nil {
				return e
			}
		}
	}
	for _, query := range []string{"ANALYZE resource", "ANALYZE common_representations", "ANALYZE reporter_resources", "ANALYZE reporter_representations"} {
		if e := db.WithContext(ctx).Exec(query).Error; e != nil {
			return e
		}
	}
	var resourceCount, representationCount int64
	if err := db.WithContext(ctx).Table("resource").Count(&resourceCount).Error; err != nil {
		return err
	}
	if err := db.WithContext(ctx).Table("reporter_representations").Count(&representationCount).Error; err != nil {
		return err
	}
	expectedResources, expectedRepresentations := s.Background, s.Background
	if scenario == "history" {
		expectedResources += workers
		expectedRepresentations += s.History
	}
	if resourceCount != int64(expectedResources) || representationCount != int64(expectedRepresentations) {
		return fmt.Errorf("seed counts: resources %d/%d, reporter representations %d/%d", resourceCount, expectedResources, representationCount, expectedRepresentations)
	}
	return nil
}

func databaseFootprint(ctx context.Context, db *gorm.DB) (int64, map[string]int64, error) {
	var databaseBytes int64
	if err := db.WithContext(ctx).Raw("SELECT pg_database_size(current_database())").Scan(&databaseBytes).Error; err != nil {
		return 0, nil, err
	}
	tableBytes := make(map[string]int64)
	for _, table := range []string{"resource", "reporter_resources", "reporter_representations", "common_representations"} {
		var size int64
		if err := db.WithContext(ctx).Raw("SELECT pg_total_relation_size(?::regclass)", table).Scan(&size).Error; err != nil {
			return 0, nil, fmt.Errorf("size %s: %w", table, err)
		}
		tableBytes[table] = size
	}
	return databaseBytes, tableBytes, nil
}
func verifyFinal(db *gorm.DB, scenario string, workers int, s settings) error {
	for i := 0; i < workers; i++ {
		id := fmt.Sprintf("perf-%s-%d", scenario, i)
		if scenario == "history" {
			id = fmt.Sprintf("perf-history-%d", i)
		}
		var row struct {
			Version    int
			Generation int
			Tombstone  bool
		}
		if e := db.Raw(`SELECT representation_version AS version,generation,tombstone FROM reporter_resources WHERE local_resource_id=? AND reporter_type='hbi' AND reporter_instance_id='perf'`, id).Scan(&row).Error; e != nil {
			return e
		}
		expectedGeneration := 2*s.Rounds - 1
		if scenario == "history" {
			expectedGeneration++
		}
		if row.Version != 0 || row.Generation != expectedGeneration || row.Tombstone {
			return fmt.Errorf("invalid final resource state for worker %d: %+v", i, row)
		}
	}
	return nil
}
func summarize(v []time.Duration) stats {
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	n := len(v)
	if n == 0 {
		return stats{}
	}
	sum := time.Duration(0)
	for _, d := range v {
		sum += d
	}
	pct := func(p float64) float64 {
		i := int(float64(n)*p+0.999999) - 1
		if i < 0 {
			i = 0
		}
		return float64(v[i]) / float64(time.Millisecond)
	}
	return stats{Count: n, MinMS: float64(v[0]) / float64(time.Millisecond), MeanMS: float64(sum) / float64(n) / float64(time.Millisecond), P50MS: pct(.5), P95MS: pct(.95), P99MS: pct(.99), MaxMS: float64(v[n-1]) / float64(time.Millisecond)}
}
func revision() string {
	b, e := exec.Command("git", "rev-parse", "HEAD").Output()
	if e != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}
func writeResult(r result) error {
	dir := os.Getenv("PERF_RESULTS_DIR")
	if dir == "" {
		dir = "results"
	}
	if e := os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(fmt.Sprintf("%s/%s-%s-%d.json", dir, r.Protocol, r.Scenario, r.Concurrency), b, 0644)
}
func checkBaseline(r result) error {
	path := os.Getenv("PERF_BASELINE")
	b := defaultBaseline
	if path != "" {
		var e error
		b, e = os.ReadFile(path)
		if e != nil {
			return e
		}
	}
	var limits map[string]float64
	if e := json.Unmarshal(b, &limits); e != nil {
		return e
	}
	for op, stat := range r.Operations {
		key := fmt.Sprintf("%s/%d/%s", r.Scenario, r.Concurrency, op)
		limit, ok := limits[key]
		if !ok {
			return fmt.Errorf("missing baseline: %s", key)
		}
		// Small verifies the harness with too few samples for a latency gate.
		if r.Settings.Profile != "small" && stat.P95MS > limit {
			return fmt.Errorf("%s p95 %.2fms exceeds %.2fms", key, stat.P95MS, limit)
		}
	}
	prefix := fmt.Sprintf("%s/%d/", r.Scenario, r.Concurrency)
	for key := range limits {
		if strings.HasPrefix(key, prefix) {
			if _, ok := r.Operations[strings.TrimPrefix(key, prefix)]; !ok {
				return fmt.Errorf("stale baseline: %s", key)
			}
		}
	}
	return nil
}
