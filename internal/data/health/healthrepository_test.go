package health

import (
	"context"
	"errors"
	"testing"

	"github.com/project-kessel/inventory-api/internal/mocks"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/data"
)

func setupGorm(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.Nil(t, err)

	err = data.Migrate(db, log.NewHelper(log.DefaultLogger))
	require.Nil(t, err)

	return db
}

func allowAllRelationsConfig() data.RelationsCompletedConfig {
	opts := data.NewRelationsOptionsRoot()
	cfg, _ := data.NewRelationsConfig(opts).Complete(context.TODO())
	return cfg
}

func TestHealthInit(t *testing.T) {
	db := setupGorm(t)
	ctx := context.TODO()

	mockRepo := &mocks.MockRelationsRepository{}
	mockRepo.On("Health", ctx).Return(errors.New("RELATIONS-API UNHEALTHY"))

	healthRepo := New(db, mockRepo, allowAllRelationsConfig())
	assert.NotNil(t, healthRepo)

	resp, err := healthRepo.IsBackendAvailable(ctx)
	assert.Nil(t, err)
	assert.Equal(t, uint32(500), resp.Code)
	assert.Equal(t, "RELATIONS-API UNHEALTHY", resp.Status)

	resp, err = healthRepo.IsRelationsAvailable(ctx)
	assert.Nil(t, err)
	assert.Equal(t, uint32(500), resp.Code)
	assert.Equal(t, "RELATIONS-API UNHEALTHY", resp.Status)
}

func TestHealthRepo_IsBackendAvailable_AllCases(t *testing.T) {
	ctx := context.TODO()
	relConfig := allowAllRelationsConfig()

	db := setupGorm(t)
	unhealthyRepo := &mocks.MockRelationsRepository{}
	unhealthyRepo.On("Health", ctx).Return(errors.New("RELATIONS-API UNHEALTHY"))

	healthRepo := New(db, unhealthyRepo, relConfig)
	assert.NotNil(t, healthRepo)
	resp, err := healthRepo.IsBackendAvailable(ctx)
	assert.Nil(t, err)
	assert.Equal(t, uint32(500), resp.Code)
	assert.Equal(t, "RELATIONS-API UNHEALTHY", resp.Status)

	sqlDB, _ := db.DB()
	if err := sqlDB.Close(); err != nil {
		t.Logf("Warning: failed to close db: %v", err)
	}
	resp, err = healthRepo.IsBackendAvailable(ctx)
	assert.Nil(t, err)
	assert.Equal(t, uint32(500), resp.Code)
	assert.Contains(t, resp.Status, "STORAGE UNHEALTHY")
	assert.Contains(t, resp.Status, "RELATIONS-API UNHEALTHY")

	db1 := setupGorm(t)
	healthyRepo1 := &mocks.MockRelationsRepository{}
	healthyRepo1.On("Health", ctx).Return(nil)
	healthRepo1 := New(db1, healthyRepo1, relConfig)
	resp, err = healthRepo1.IsBackendAvailable(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint32(200), resp.Code)
	assert.Contains(t, resp.Status, "Storage type sqlite")

	db2 := setupGorm(t)
	healthyRepo2 := &mocks.MockRelationsRepository{}
	healthyRepo2.On("Health", ctx).Return(nil)
	sqlDB2, _ := db2.DB()
	if err := sqlDB2.Close(); err != nil {
		t.Logf("Warning: failed to close db: %v", err)
	}
	healthRepo2 := New(db2, healthyRepo2, relConfig)
	resp, err = healthRepo2.IsBackendAvailable(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint32(500), resp.Code)
	assert.Contains(t, resp.Status, "STORAGE UNHEALTHY: sqlite")
	assert.NotContains(t, resp.Status, "RELATIONS-API UNHEALTHY")

	db3 := setupGorm(t)
	unhealthyRepo3 := &mocks.MockRelationsRepository{}
	unhealthyRepo3.On("Health", ctx).Return(errors.New("RELATIONS-API UNHEALTHY"))
	healthRepo3 := New(db3, unhealthyRepo3, relConfig)
	resp, err = healthRepo3.IsBackendAvailable(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint32(500), resp.Code)
	assert.Equal(t, "RELATIONS-API UNHEALTHY", resp.Status)
}
