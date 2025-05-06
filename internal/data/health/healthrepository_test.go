package health

import (
	"context"
	"testing"

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
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
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
