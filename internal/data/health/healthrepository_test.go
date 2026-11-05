package health

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/config/relations"
	relationsGrpc "github.com/project-kessel/inventory-api/internal/config/relations/kessel"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/testutil"
)

// migrateHealthDB brings a fresh test database up to the current schema. It
// is a named function so the PostgreSQL test backend can key its template
// database on a stable value and run the migrations only once per test
// binary; see testutil.PostgresTestDSN.
func migrateHealthDB(db *gorm.DB) error {
	return data.Migrate(db, log.NewHelper(log.DefaultLogger))
}

// setupGorm returns a fully migrated test database on the backend selected
// through TEST_DB: PostgreSQL via Testcontainers (the default, matching
// production) or in-memory SQLite (the dependency-free fallback, selected
// explicitly with TEST_DB=sqlite).
func setupGorm(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.NewTestDB(t, &gorm.Config{TranslateError: true}, migrateHealthDB)
}

func TestHealthInit(t *testing.T) {
	db := setupGorm(t)
	ctx := context.TODO()
	relationsConfig, _ := relations.NewConfig(relations.NewOptions()).Complete(ctx)
	kesselConfig, _ := relationsGrpc.NewConfig(relationsGrpc.NewOptions()).Complete(ctx)
	relationsRepo, _ := data.NewGRPCRelationsRepository(ctx, kesselConfig, log.NewHelper(log.DefaultLogger))

	healthRepo := New(db, relationsRepo, relationsConfig)
	assert.NotNil(t, healthRepo)

	resp, err := healthRepo.IsBackendAvailable(ctx)
	assert.Nil(t, err)
	assert.Equal(t, 500, resp.Code())
	assert.Equal(t, "RELATIONS-API UNHEALTHY", resp.Status())

	resp, err = healthRepo.IsRelationsAvailable(ctx)
	assert.Nil(t, err)
	assert.Equal(t, 500, resp.Code())
	assert.Equal(t, "RELATIONS-API UNHEALTHY", resp.Status())
}

func TestHealthRepo_IsBackendAvailable_AllCases(t *testing.T) {
	ctx := context.TODO()
	relationsConfig, _ := relations.NewConfig(relations.NewOptions()).Complete(ctx)
	kesselConfig, _ := relationsGrpc.NewConfig(relationsGrpc.NewOptions()).Complete(ctx)
	relationsRepo, _ := data.NewGRPCRelationsRepository(ctx, kesselConfig, log.NewHelper(log.DefaultLogger))

	db := setupGorm(t)
	healthRepo := New(db, relationsRepo, relationsConfig)
	assert.NotNil(t, healthRepo)
	resp, err := healthRepo.IsBackendAvailable(ctx)
	assert.Nil(t, err)
	assert.Equal(t, 500, resp.Code())
	assert.Equal(t, "RELATIONS-API UNHEALTHY", resp.Status())

	sqlDB, _ := db.DB()
	if err := sqlDB.Close(); err != nil {
		t.Logf("Warning: failed to close db: %v", err)
	}
	resp, err = healthRepo.IsBackendAvailable(ctx)
	assert.Nil(t, err)
	assert.Equal(t, 500, resp.Code())
	assert.Contains(t, resp.Status(), "STORAGE UNHEALTHY")
	assert.Contains(t, resp.Status(), "RELATIONS-API UNHEALTHY")

	db1 := setupGorm(t)
	simpleRelations1 := data.NewSimpleRelationsRepository()
	healthRepo1 := New(db1, simpleRelations1, relationsConfig)
	resp, err = healthRepo1.IsBackendAvailable(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.Code())
	// The reported storage type is the GORM dialector name, which matches the
	// TEST_DB backend selection ("postgres" or "sqlite").
	assert.Contains(t, resp.Status(), fmt.Sprintf("Storage type %s", testutil.TestDBBackend()))

	db2 := setupGorm(t)
	simpleRelations2 := data.NewSimpleRelationsRepository()
	sqlDB2, _ := db2.DB()
	if err := sqlDB2.Close(); err != nil {
		t.Logf("Warning: failed to close db: %v", err)
	}
	healthRepo2 := New(db2, simpleRelations2, relationsConfig)
	resp, err = healthRepo2.IsBackendAvailable(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.Code())
	assert.Contains(t, resp.Status(), fmt.Sprintf("STORAGE UNHEALTHY: %s", testutil.TestDBBackend()))
	assert.NotContains(t, resp.Status(), "RELATIONS-API UNHEALTHY")

	db3 := setupGorm(t)
	simpleRelations3 := data.NewSimpleRelationsRepository()
	simpleRelations3.SetHealthError(errors.New("RELATIONS-API UNHEALTHY"))
	healthRepo3 := New(db3, simpleRelations3, relationsConfig)
	resp, err = healthRepo3.IsBackendAvailable(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.Code())
	assert.Equal(t, "RELATIONS-API UNHEALTHY", resp.Status())

}
