package gorm

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/testutil"
	"gorm.io/gorm"
)

// NewTestResourceRepository creates a fully configured [ResourceRepository]
// backed by an in-memory SQLite database with all migrations applied.
// It is intended for use in tests that need a real, migrated database.
func NewTestResourceRepository(t *testing.T) *ResourceRepository {
	t.Helper()

	db := newTestDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, 3)

	return NewResourceRepository(db, tm)
}

// newTestDB creates a migrated in-memory SQLite database for testing.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := testutil.NewSQLiteTestDB(t, &gorm.Config{TranslateError: true})
	if err := Migrate(db, nil); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}
