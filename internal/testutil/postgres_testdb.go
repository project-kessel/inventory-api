package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // Register the pgx driver for database/sql maintenance connections
	"github.com/moby/moby/api/types/container"
	inventoryerrors "github.com/project-kessel/inventory-api/internal/errors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	// DefaultPostgresImage is the PostgreSQL image used for test databases.
	// It matches the image used by the performance suite and the kind
	// deployment so all test paths exercise the same major version.
	DefaultPostgresImage = "postgres:17.8"

	// PostgresImageEnvVar overrides the image used for the shared PostgreSQL
	// test container, for example to trial a different major version.
	PostgresImageEnvVar = "TEST_POSTGRES_IMAGE"

	// postgresTestUser and postgresMaintenanceDB name the user and database
	// the container is initialized with. The maintenance database only hosts
	// CREATE/DROP DATABASE statements; tests never store data in it.
	postgresTestUser      = "test"
	postgresMaintenanceDB = "test"

	// postgresMaxNameLen is PostgreSQL's NAMEDATALEN limit minus one.
	postgresMaxNameLen = 63

	// postgresTestNamePrefix prefixes per-test database names.
	postgresTestNamePrefix = "t"
)

// postgresServer is the PostgreSQL testcontainer shared by every test in a
// test binary, together with the maintenance connection used to create and
// drop databases.
type postgresServer struct {
	container testcontainers.Container
	admin     *sql.DB
	dsnArgs   string // host/port/credentials; dbname is appended per database
}

var (
	postgresOnce   sync.Once
	postgresShared *postgresServer
	postgresErr    error

	postgresTemplatesMu sync.Mutex
	// postgresTemplates caches one migrated template database per migrate
	// function so migrations run once per test binary instead of once per
	// test. Keys are code pointers of the migrate functions.
	postgresTemplates = map[uintptr]*postgresTemplate{}
)

// postgresTemplate is a database holding the fully migrated schema that test
// databases are copied from.
type postgresTemplate struct {
	name string
	once sync.Once
	err  error
}

// NewPostgresTestDB returns a GORM handle to an isolated, migrated PostgreSQL
// database for the test. See PostgresTestDSN for the isolation model and
// TerminateSharedPostgres for container lifetime.
//
// The returned handle's connection pool is closed when the test finishes.
func NewPostgresTestDB(t *testing.T, migrate func(*gorm.DB) error, cfg *gorm.Config) *gorm.DB {
	t.Helper()

	dsn := PostgresTestDSN(t, migrate)

	if cfg == nil {
		cfg = &gorm.Config{}
	}
	db, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		t.Fatalf("open postgres test database: %v", err)
	}
	t.Cleanup(func() { closePostgresTestDB(t, db) })
	return db
}

// PostgresTestDSN provisions an isolated PostgreSQL database for the test and
// returns its DSN. Use NewPostgresTestDB unless the test needs to open
// handles itself.
//
// The first call in a test binary starts one PostgreSQL testcontainer and,
// for each distinct migrate function, builds a template database by running
// migrate once. Every call then copies the template into a private database,
// giving tests full schema isolation without paying the migration cost per
// test. Databases are dropped when their test finishes; the container is
// stopped by TerminateSharedPostgres.
//
// Pass a stable function value (a package-level function, not a fresh
// closure) as migrate so its template is reused; distinct functions get
// distinct templates. migrate may be nil when no schema is needed, in which
// case databases are copied from PostgreSQL's pristine template0.
func PostgresTestDSN(t *testing.T, migrate func(*gorm.DB) error) string {
	t.Helper()

	srv := sharedPostgresServer(t)
	tpl := templateFor(migrate)
	tpl.ensure(t, srv, migrate)

	name := postgresTestDBName(t)
	if err := createDatabase(srv, fmt.Sprintf("CREATE DATABASE %q TEMPLATE %q", name, tpl.name)); err != nil {
		t.Fatalf("create postgres test database: %v", err)
	}

	t.Cleanup(func() { dropDatabase(t, srv, name) })

	return srv.dsn(name)
}

// TerminateSharedPostgres stops the shared PostgreSQL testcontainer and closes
// its maintenance connection. It is a no-op when no PostgreSQL test database
// was created. Call it from TestMain after m.Run so the container is removed
// even when Ryuk is disabled, which scripts/testcontainers-run.sh does for
// Podman. Report any returned error and fail the test binary, even if m.Run
// succeeded. Call only after all tests and their database cleanup have finished.
func TerminateSharedPostgres() error {
	postgresOnce.Do(func() {}) // never start the server just to stop it
	if postgresShared == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := postgresShared.terminate(ctx); err != nil {
		return err // retain the container handle so cleanup can be retried
	}

	postgresShared = nil
	return nil
}

// terminate attempts both cleanup steps, even if closing the pool fails.
func (s *postgresServer) terminate(ctx context.Context) error {
	var cleanupErrors []error
	if err := s.admin.Close(); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("close postgres maintenance connection: %w", err))
	}
	if err := s.container.Terminate(ctx); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("terminate postgres test container %s: %w", s.container.GetContainerID(), err))
	}
	if len(cleanupErrors) > 0 {
		return inventoryerrors.NewAggregate(cleanupErrors)
	}
	return nil
}

// sharedPostgresServer starts the shared container on first use and returns
// it, failing the test with setup guidance when no runtime is reachable.
func sharedPostgresServer(t *testing.T) *postgresServer {
	t.Helper()

	postgresOnce.Do(func() {
		postgresShared, postgresErr = startPostgresServer()
	})
	if postgresErr != nil {
		t.Fatal(formatPostgresHint(postgresErr))
	}
	return postgresShared
}

func startPostgresServer() (server *postgresServer, err error) {
	image := strings.TrimSpace(os.Getenv(PostgresImageEnvVar))
	if image == "" {
		image = DefaultPostgresImage
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	password := uuid.NewString()
	started := time.Now()
	ctr, err := testcontainers.Run(ctx, image,
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER":     postgresTestUser,
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       postgresMaintenanceDB,
		}),
		testcontainers.WithExposedPorts("5432/tcp"),
		// Production runs wal_level=logical so the Debezium outbox pipeline
		// can consume pg_logical_emit_message output; match it so tests can
		// exercise the real WAL outbox publisher and read it back through
		// logical replication slots.
		testcontainers.WithCmd("postgres", "-c", "wal_level=logical"),
		// The official image logs "database system is ready to accept
		// connections" once for the temporary init server and once for the
		// final server; waiting for the second occurrence avoids connecting
		// while initialization is still running.
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(2*time.Minute),
		),
		testcontainers.WithHostConfigModifier(func(h *container.HostConfig) { h.ShmSize = 256 << 20 }),
	)
	// Run can return a container together with a startup error. Until setup
	// succeeds, this function owns cleanup, including when Ryuk is disabled.
	defer func() {
		if server == nil && ctr != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()
			if cleanupErr := ctr.Terminate(cleanupCtx); cleanupErr != nil {
				err = fmt.Errorf("%w; terminate failed postgres container: %v", err, cleanupErr)
			}
		}
	}()
	if err != nil {
		return nil, fmt.Errorf("start container %s: %w", image, err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve container host: %w", err)
	}
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return nil, fmt.Errorf("resolve container port: %w", err)
	}

	srv := &postgresServer{
		container: ctr,
		dsnArgs: fmt.Sprintf("host=%s port=%s user=%s password=%s sslmode=disable",
			host, port.Port(), postgresTestUser, password),
	}

	// database/sql, not GORM: the maintenance connection only issues
	// CREATE/DROP DATABASE, which cannot run inside transactions.
	admin, err := sql.Open("pgx", srv.dsn(postgresMaintenanceDB))
	if err != nil {
		return nil, fmt.Errorf("open maintenance connection: %w", err)
	}
	pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
	defer pingCancel()
	if err := admin.PingContext(pingCtx); err != nil {
		_ = admin.Close()
		return nil, fmt.Errorf("ping postgres container: %w", err)
	}
	srv.admin = admin

	fmt.Printf("testutil: started PostgreSQL %s in %s (host=%s port=%s)\n",
		image, time.Since(started).Round(time.Millisecond), host, port.Port())
	return srv, nil
}

// dsn renders the DSN for a database on the shared server. name must be a
// sanitized identifier (see postgresTestDBName).
func (s *postgresServer) dsn(name string) string {
	return fmt.Sprintf("%s dbname=%s application_name=%s", s.dsnArgs, name, name)
}

// templateFor returns the template database for a migrate function, creating
// its bookkeeping entry on first use.
func templateFor(migrate func(*gorm.DB) error) *postgresTemplate {
	if migrate == nil {
		return pristineTemplate()
	}

	key := reflect.ValueOf(migrate).Pointer()

	postgresTemplatesMu.Lock()
	defer postgresTemplatesMu.Unlock()
	if postgresTemplates[key] == nil {
		postgresTemplates[key] = &postgresTemplate{
			name: fmt.Sprintf("kessel_template_%d", len(postgresTemplates)+1),
		}
	}
	return postgresTemplates[key]
}

var (
	pristineTemplateOnce sync.Once
	pristineTemplateVal  *postgresTemplate
)

// pristineTemplate is the template used when no migrate function is given; it
// copies PostgreSQL's untouched template0.
func pristineTemplate() *postgresTemplate {
	pristineTemplateOnce.Do(func() {
		pristineTemplateVal = &postgresTemplate{name: "template0"}
	})
	return pristineTemplateVal
}

// ensure migrates the template database on first use. Failures are recorded
// on the template instead of failing the test immediately: t.Fatalf inside
// sync.Once.Do would runtime.Goexit before the once is marked done, causing
// every subsequent caller to retry the failed setup.
func (tpl *postgresTemplate) ensure(t *testing.T, srv *postgresServer, migrate func(*gorm.DB) error) {
	t.Helper()

	if tpl.name == "template0" {
		return // pristine cluster template needs no migrations
	}

	tpl.once.Do(func() {
		tpl.err = buildTemplate(tpl, srv, migrate)
	})
	if tpl.err != nil {
		t.Fatalf("prepare postgres template database: %v", tpl.err)
	}
}

func buildTemplate(tpl *postgresTemplate, srv *postgresServer, migrate func(*gorm.DB) error) error {
	started := time.Now()
	if err := createDatabase(srv, fmt.Sprintf("CREATE DATABASE %q", tpl.name)); err != nil {
		return fmt.Errorf("create template database: %w", err)
	}

	// Migrate through a dedicated handle that is fully closed afterwards:
	// CREATE DATABASE ... TEMPLATE refuses to copy a database that still has
	// open connections.
	db, err := gorm.Open(postgres.Open(srv.dsn(tpl.name)), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open template database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("unwrap template handle: %w", err)
	}

	// The migrate function is supplied by the caller, who knows which schema
	// the tests need; testutil itself must not depend on the data layer.
	if err := migrate(db); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("migrate template database: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close template handle: %w", err)
	}

	fmt.Printf("testutil: migrated PostgreSQL template %s in %s\n",
		tpl.name, time.Since(started).Round(time.Millisecond))
	return nil
}

// postgresIdentifierSanitizer reduces a test name to the characters allowed in
// an unquoted PostgreSQL identifier.
var postgresIdentifierSanitizer = regexp.MustCompile(`[^a-z0-9_]+`)

// postgresTestDBName derives a unique, syntactically safe database name from
// the test name. The name stays short enough for PostgreSQL's 63-byte
// NAMEDATALEN limit even after prefix, separator, and unique suffix are
// added.
func postgresTestDBName(t *testing.T) string {
	safe := strings.ToLower(t.Name())
	safe = postgresIdentifierSanitizer.ReplaceAllString(safe, "_")
	safe = strings.Trim(safe, "_")

	// Reserve room for prefix, separator, and the 8-character unique suffix.
	budget := postgresMaxNameLen - len(postgresTestNamePrefix) - 1 - 9
	if len(safe) > budget {
		safe = safe[:budget]
	}
	return fmt.Sprintf("%s_%s_%s", postgresTestNamePrefix, safe, uuid.NewString()[:8])
}

func createDatabase(srv *postgresServer, statement string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := srv.admin.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("%s: %w", statement, err)
	}
	return nil
}

// dropDatabase removes a test database. WITH (FORCE) (PostgreSQL 13+)
// terminates lingering connections so the drop succeeds even if a test leaked
// a pooled connection; retries paper over drops that race with connection
// teardown. Failures are logged, not fatal: the container is removed by
// TerminateSharedPostgres regardless.
func dropDatabase(t *testing.T, srv *postgresServer, name string) {
	t.Helper()

	statement := fmt.Sprintf("DROP DATABASE %q WITH (FORCE)", name)
	for attempt := 1; attempt <= 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := srv.admin.ExecContext(ctx, statement)
		cancel()
		if err == nil {
			return
		}
		if attempt == 3 {
			t.Logf("drop test database %s: %v (container termination will clean it up)", name, err)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func closePostgresTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	sqlDB, err := db.DB()
	if err != nil {
		t.Logf("unwrap postgres test handle: %v", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		t.Logf("close postgres test handle: %v", err)
	}
}
