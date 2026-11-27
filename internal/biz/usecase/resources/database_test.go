package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/project-kessel/inventory-api/internal/testutil"
	"gorm.io/gorm"
)

func withResourceRepository() harnessOption {
	return func(c *harnessConfig) { c.withResourceRepository = true }
}

func TestHarness_WithoutDatabase(t *testing.T) {
	t.Setenv("TEST_DB", "invalid-unused-database")

	t.Run("construction", func(t *testing.T) {
		h := newTestHarness(t)
		require.NotNil(t, h.usecase)
		assert.Nil(t, h.resourceRepo)
	})

	t.Run("check self", func(t *testing.T) {
		h := newTestHarness(t, withMeta(true))
		require.Nil(t, h.resourceRepo)
		key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
		relation, err := model.NewRelation("view")
		require.NoError(t, err)

		result, err := h.usecase.CheckSelf(h.ctx, relation, resourceRefFromKey(key), model.NewConsistencyMinimizeLatency())
		require.NoError(t, err)
		assert.True(t, result.Allowed())
	})
}

// Use a package-level migration function so PostgreSQL tests reuse one migrated
// template while each test receives its own isolated database.
func migrateResourceUsecaseDB(db *gorm.DB) error {
	return data.Migrate(db, nil)
}

func newTestResourceRepository(t *testing.T, mc *metricscollector.MetricsCollector) model.ResourceRepository {
	t.Helper()
	db := testutil.NewTestDB(t, &gorm.Config{TranslateError: true}, migrateResourceUsecaseDB)
	transactionManager := data.NewGormTransactionManager(mc, 3)
	// Match production's WAL outbox on PostgreSQL; SQLite has no logical WAL.
	publisher := data.SetOutboxPublisher(storage.OutboxModeNone)
	if testutil.UsingPostgres() {
		publisher = data.SetOutboxPublisher(storage.OutboxModeWAL)
	}
	return data.NewResourceRepository(db, transactionManager, publisher)
}
