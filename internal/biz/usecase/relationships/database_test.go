package relationships

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/project-kessel/inventory-api/internal/testutil"
	"gorm.io/gorm"
)

// Use a package-level migration function so PostgreSQL tests reuse one migrated
// template while each test receives its own isolated database.
func migrateRelationshipUsecaseDB(db *gorm.DB) error {
	return data.Migrate(db, nil)
}

func newTestResourceRepository(t *testing.T, mc *metricscollector.MetricsCollector) model.ResourceRepository {
	t.Helper()
	db := testutil.NewTestDB(t, &gorm.Config{TranslateError: true}, migrateRelationshipUsecaseDB)
	transactionManager := data.NewGormTransactionManager(mc, 3)
	// Match production's WAL outbox on PostgreSQL; SQLite has no logical WAL.
	publisher := data.SetOutboxPublisher(storage.OutboxModeNone)
	if testutil.UsingPostgres() {
		publisher = data.SetOutboxPublisher(storage.OutboxModeWAL)
	}
	return data.NewResourceRepository(db, transactionManager, publisher)
}
