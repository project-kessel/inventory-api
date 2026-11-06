package migrations

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/testutil"
)

const initialMigrationID = "20251120120000"

// openTestDB returns an empty test database on the backend selected through
// TEST_DB: PostgreSQL via Testcontainers (the default, matching
// production) or in-memory SQLite (the dependency-free fallback, selected
// explicitly with TEST_DB=sqlite). No
// schema is applied — these tests run the migrations themselves, which is
// the behavior under test.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.NewTestDB(t, &gorm.Config{}, nil)
}

func tableExists(t *testing.T, db *gorm.DB, table string) bool {
	t.Helper()
	return db.Migrator().HasTable(table)
}

// TestRunCreatesTables checks that migrations create the core resource and
// reporter-resource tables on the selected database backend.
func TestRunCreatesTables(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, log.NewHelper(log.DefaultLogger)); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// spot-check a couple of tables
	if !tableExists(t, db, "resource") {
		t.Fatalf("expected table resource to exist")
	}
	if !tableExists(t, db, "reporter_resources") {
		t.Fatalf("expected table reporter_resources to exist")
	}
}

// TestRunIdempotent verifies that rerunning migrations on an already migrated
// database succeeds without attempting incompatible schema changes.
func TestRunIdempotent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, log.NewHelper(log.DefaultLogger)); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Run(ctx, db, log.NewHelper(log.DefaultLogger)); err != nil {
		t.Fatalf("second migrate (idempotent) failed: %v", err)
	}
}

func TestRunToErrorsOnEmptyTargetID(t *testing.T) {
	ctx := context.Background()

	err := RunTo(ctx, nil, log.NewHelper(log.DefaultLogger), "")
	if err == nil {
		t.Fatalf("expected error when targetID is empty")
	}
}

func TestRunToErrorsOnNilDB(t *testing.T) {
	ctx := context.Background()

	err := RunTo(ctx, nil, log.NewHelper(log.DefaultLogger), initialMigrationID)
	if err == nil {
		t.Fatalf("expected error when db is nil")
	}
}
