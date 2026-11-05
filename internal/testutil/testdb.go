package testutil

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// Database backends available for tests that exercise SQL/GORM behavior.
const (
	// TestDBBackendEnvVar selects the database backend used by NewTestDB.
	// Accepted values are BackendPostgres and BackendSQLite. An empty value
	// selects the default backend.
	TestDBBackendEnvVar = "TEST_DB"

	// BackendPostgres runs database tests against PostgreSQL through
	// Testcontainers. It requires a reachable container runtime (Docker or
	// Podman); run tests through scripts/testcontainers-run.sh or set
	// DOCKER_HOST to select an endpoint.
	BackendPostgres = "postgres"

	// BackendSQLite runs database tests against in-memory SQLite. It needs no
	// external dependencies but does not reproduce PostgreSQL behavior
	// (JSONB constraints, advisory locks, transaction conflicts). Select it
	// explicitly with TEST_DB=sqlite when no container runtime is available.
	BackendSQLite = "sqlite"
)

// TestDBBackend returns the database backend selected through
// TestDBBackendEnvVar, defaulting to BackendPostgres.
//
// PostgreSQL is the default because it is what production runs and SQLite
// cannot reproduce PostgreSQL-specific behavior (JSONB constraints, advisory
// locks, transaction conflicts). Select BackendSQLite explicitly with
// TEST_DB=sqlite when no container runtime is available.
func TestDBBackend() string {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv(TestDBBackendEnvVar)))
	if backend == "" {
		return BackendPostgres
	}
	return backend
}

// UsingPostgres reports whether tests are running against the PostgreSQL
// backend. Use this to branch over backend-specific behavior that cannot be
// expressed generically (for example, setting a lock timeout).
func UsingPostgres() bool {
	return TestDBBackend() == BackendPostgres
}

// NewTestDB returns a migrated test database on the backend selected through
// TestDBBackendEnvVar, defaulting to PostgreSQL.
//
// The migrate function brings the database schema up to date. On SQLite it
// runs once per call, matching the per-test in-memory database. On PostgreSQL
// it runs once per test binary to build a template database that every test
// database is copied from, so pass a stable function value (a package-level
// function, not a fresh closure) to avoid building more templates than
// needed. migrate may be nil when the test does not need a schema.
//
// PostgreSQL requires a container runtime; run tests through
// scripts/testcontainers-run.sh (or the make target that wraps it) so runtime
// detection and Podman quirks are handled, and add a TestMain calling
// TerminateSharedPostgres so the shared container is removed reliably.
func NewTestDB(t *testing.T, cfg *gorm.Config, migrate func(*gorm.DB) error) *gorm.DB {
	t.Helper()

	switch TestDBBackend() {
	case BackendPostgres:
		return NewPostgresTestDB(t, migrate, cfg)
	case BackendSQLite:
		db := NewSQLiteTestDB(t, cfg)
		if migrate != nil {
			if err := migrate(db); err != nil {
				t.Fatalf("migrate sqlite test database: %v", err)
			}
		}
		return db
	default:
		t.Fatalf("unknown %s value %q: expected %q or %q",
			TestDBBackendEnvVar, TestDBBackend(), BackendPostgres, BackendSQLite)
		return nil
	}
}

// formatPostgresHint explains how to get a working container runtime for the
// PostgreSQL test backend. It is included in setup failure messages because
// Testcontainers errors alone rarely point volunteers at the fix.
func formatPostgresHint(err error) string {
	return fmt.Sprintf(`PostgreSQL test database unavailable: %v

Database tests default to PostgreSQL, which needs a container runtime. Either:
  - run the tests through scripts/testcontainers-run.sh (or a make target that
    wraps it, such as make test), which detects Docker and Podman endpoints
    automatically, or
  - set DOCKER_HOST to a reachable runtime endpoint, or
  - select the dependency-free fallback with %s=%s.

See test/GUIDELINES.md for details.`, err, TestDBBackendEnvVar, BackendSQLite)
}
