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

func TestRunCreatesReporterRepsGenVerIndex(t *testing.T) {
	db := openSQLite(t)
	ctx := context.Background()

	if err := RunTo(ctx, db, log.NewHelper(log.DefaultLogger), "20260930160000"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var count int64
	if err := db.Raw(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'index' AND name = 'idx_reporter_reps_resource_gen_ver'
	`).Scan(&count).Error; err != nil {
		t.Fatalf("query index: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected idx_reporter_reps_resource_gen_ver to exist, count=%d", count)
	}
}

func TestRunDropsReporterRepsGenVerIndex(t *testing.T) {
	db := openSQLite(t)
	ctx := context.Background()
	logger := log.NewHelper(log.DefaultLogger)

	if err := RunTo(ctx, db, logger, "20260930160000"); err != nil {
		t.Fatalf("migrate through original index migration: %v", err)
	}
	if err := db.Exec("CREATE INDEX reporter_reps_preserved_test_idx ON reporter_representations (version)").Error; err != nil {
		t.Fatalf("create unrelated index: %v", err)
	}

	if err := Run(ctx, db, logger); err != nil {
		t.Fatalf("run remaining migrations: %v", err)
	}

	var targetIndexCount int64
	if err := db.Raw(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'index' AND name = 'idx_reporter_reps_resource_gen_ver'
	`).Scan(&targetIndexCount).Error; err != nil {
		t.Fatalf("query target index: %v", err)
	}
	if targetIndexCount != 0 {
		t.Fatalf("expected idx_reporter_reps_resource_gen_ver to be removed, count=%d", targetIndexCount)
	}

	var preservedIndexCount int64
	if err := db.Raw(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'index'
		  AND name = 'reporter_reps_preserved_test_idx'
		  AND tbl_name = 'reporter_representations'
	`).Scan(&preservedIndexCount).Error; err != nil {
		t.Fatalf("query unrelated index: %v", err)
	}
	if preservedIndexCount != 1 {
		t.Fatalf("expected unrelated index to remain, count=%d", preservedIndexCount)
	}

	var recordedMigrationCount int64
	if err := db.Table(MigrationTableName).Where(MigrationIDColumn+" = ?", "20260930170000").Count(&recordedMigrationCount).Error; err != nil {
		t.Fatalf("query drop migration record: %v", err)
	}
	if recordedMigrationCount != 1 {
		t.Fatalf("expected drop migration to be recorded once, count=%d", recordedMigrationCount)
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
