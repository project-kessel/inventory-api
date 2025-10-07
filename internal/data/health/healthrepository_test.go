package health

import (
	"context"
	"errors"
	"testing"

	"github.com/project-kessel/inventory-api/internal/mocks"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/authz/kessel"
	"github.com/project-kessel/inventory-api/internal/data"
)

func setupGorm(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.Nil(t, err)

	err = data.Migrate(db, log.NewHelper(log.DefaultLogger))
	require.Nil(t, err)

	return db
}

func TestHealthInit(t *testing.T) {
	db := setupGorm(t)
	ctx := context.TODO()
	authConfig, _ := authz.NewConfig(authz.NewOptions()).Complete(ctx)
	kesselConfig, _ := kessel.NewConfig(kessel.NewOptions()).Complete(ctx)
	authorizer, _ := kessel.New(ctx, kesselConfig, log.NewHelper(log.DefaultLogger))

	healthRepo := New(db, authorizer, authConfig)
	assert.NotNil(t, healthRepo)

	// just check default negative case for now when using sqlite db, and empty config
	// storage should be okay, relations api should not because tokenConfig is empty
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
	authConfig, _ := authz.NewConfig(authz.NewOptions()).Complete(ctx)
	kesselConfig, _ := kessel.NewConfig(kessel.NewOptions()).Complete(ctx)
	authorizer, _ := kessel.New(ctx, kesselConfig, log.NewHelper(log.DefaultLogger))

	db := setupGorm(t)
	healthRepo := New(db, authorizer, authConfig)
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
	mockAuthz1 := &mocks.MockAuthz{}
	mockAuthz1.On("Health", ctx).Return(&kesselv1.GetReadyzResponse{Status: "OK"}, nil)
	healthRepo1 := New(db1, mockAuthz1, authConfig)
	resp, err = healthRepo1.IsBackendAvailable(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint32(200), resp.Code)
	assert.Contains(t, resp.Status, "Storage type sqlite")

	db2 := setupGorm(t)
	mockAuthz2 := &mocks.MockAuthz{}
	mockAuthz2.On("Health", ctx).Return(&kesselv1.GetReadyzResponse{Status: "OK"}, nil)
	sqlDB2, _ := db2.DB()
	if err := sqlDB2.Close(); err != nil {
		t.Logf("Warning: failed to close db: %v", err)
	}
	healthRepo2 := New(db2, mockAuthz2, authConfig)
	resp, err = healthRepo2.IsBackendAvailable(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint32(500), resp.Code)
	assert.Contains(t, resp.Status, "STORAGE UNHEALTHY: sqlite")
	assert.NotContains(t, resp.Status, "RELATIONS-API UNHEALTHY")

	db3 := setupGorm(t)
	mockAuthz3 := &mocks.MockAuthz{}
	mockAuthz3.On("Health", ctx).Return((*kesselv1.GetReadyzResponse)(nil), errors.New("RELATIONS-API UNHEALTHY"))
	healthRepo3 := New(db3, mockAuthz3, authConfig)
	resp, err = healthRepo3.IsBackendAvailable(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint32(500), resp.Code)
	assert.Equal(t, "RELATIONS-API UNHEALTHY", resp.Status)

}
