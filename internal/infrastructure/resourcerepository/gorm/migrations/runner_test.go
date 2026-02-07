package migrations

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const initialMigrationID = "20251120120000"

func openSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func tableExists(t *testing.T, db *gorm.DB, table string) bool {
	t.Helper()
	return db.Migrator().HasTable(table)
}

func TestRunCreatesTables(t *testing.T) {
	db := openSQLite(t)
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

func TestRunIdempotent(t *testing.T) {
	db := openSQLite(t)
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
