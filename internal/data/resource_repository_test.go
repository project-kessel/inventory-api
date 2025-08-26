package data

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
)

func TestResourceRepositoryContract(t *testing.T) {
	implementations := []struct {
		name string
		repo func() ResourceRepository
		db   func() *gorm.DB
	}{
		// TODO: Fix outbox event handling for real repository tests
		// {
		// 	name: "Real Repository with GormTransactionManager",
		// 	repo: func() ResourceRepository {
		// 		db := setupInMemoryDB(t)
		// 		tm := NewGormTransactionManager(3)
		// 		return NewResourceRepository(db, tm)
		// 	},
		// 	db: func() *gorm.DB {
		// 		return setupInMemoryDB(t)
		// 	},
		// },
		// {
		// 	name: "Real Repository with FakeTransactionManager",
		// 	repo: func() ResourceRepository {
		// 		db := setupInMemoryDB(t)
		// 		tm := NewFakeTransactionManager()
		// 		return NewResourceRepository(db, tm)
		// 	},
		// 	db: func() *gorm.DB {
		// 		return setupInMemoryDB(t)
		// 	},
		// },
		{
			name: "Fake Repository",
			repo: func() ResourceRepository {
				return NewFakeResourceRepository()
			},
			db: func() *gorm.DB {
				return nil // Fake doesn't need real DB
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			testRepositoryContract(t, impl.repo(), impl.db())
		})
	}
}

func testRepositoryContract(t *testing.T, repo ResourceRepository, db *gorm.DB) {
	t.Run("NextResourceId generates valid UUIDs", func(t *testing.T) {
		id1, err := repo.NextResourceId()
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id1.UUID())

		id2, err := repo.NextResourceId()
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id2.UUID())
		assert.NotEqual(t, id1.UUID(), id2.UUID())
	})

	t.Run("NextReporterResourceId generates valid UUIDs", func(t *testing.T) {
		id1, err := repo.NextReporterResourceId()
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id1.UUID())

		id2, err := repo.NextReporterResourceId()
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id2.UUID())
		assert.NotEqual(t, id1.UUID(), id2.UUID())
	})

	t.Run("Save and FindResourceByKeys workflow", func(t *testing.T) {
		resource := createTestResource(t)

		err := repo.Save(db, resource, model_legacy.OperationTypeCreated, "test-tx-123")
		require.NoError(t, err)

		key, err := bizmodel.NewReporterResourceKey(
			"test-resource-123",
			"k8s_cluster",
			"ocm",
			"ocm-instance-1",
		)
		require.NoError(t, err)

		foundResource, err := repo.FindResourceByKeys(db, key)
		require.NoError(t, err)
		require.NotNil(t, foundResource)

		assert.Len(t, foundResource.ReporterResources(), 1, "should have reporter resources")
	})

	t.Run("FindResourceByKeys returns nil for non-existent resource", func(t *testing.T) {
		key, err := bizmodel.NewReporterResourceKey(
			"non-existent",
			"k8s_cluster",
			"ocm",
			"ocm-instance-1",
		)
		require.NoError(t, err)

		foundResource, err := repo.FindResourceByKeys(db, key)
		require.NoError(t, err)
		assert.Nil(t, foundResource)
	})

	t.Run("FindResourceByKeys with different keys returns different resources", func(t *testing.T) {
		resource1 := createTestResourceWithLocalId(t, "resource-1")
		resource2 := createTestResourceWithLocalId(t, "resource-2")

		err := repo.Save(db, resource1, model_legacy.OperationTypeCreated, "test-tx-1")
		require.NoError(t, err)

		err = repo.Save(db, resource2, model_legacy.OperationTypeCreated, "test-tx-2")
		require.NoError(t, err)

		key1, err := bizmodel.NewReporterResourceKey("resource-1", "k8s_cluster", "ocm", "ocm-instance-1")
		require.NoError(t, err)

		key2, err := bizmodel.NewReporterResourceKey("resource-2", "k8s_cluster", "ocm", "ocm-instance-1")
		require.NoError(t, err)

		found1, err := repo.FindResourceByKeys(db, key1)
		require.NoError(t, err)
		require.NotNil(t, found1)

		found2, err := repo.FindResourceByKeys(db, key2)
		require.NoError(t, err)
		require.NotNil(t, found2)

		// Resources should be different
		reporters1 := found1.ReporterResources()
		reporters2 := found2.ReporterResources()
		require.Len(t, reporters1, 1)
		require.Len(t, reporters2, 1)
		assert.NotEqual(t, reporters1[0].LocalResourceId(), reporters2[0].LocalResourceId())
	})

	t.Run("Save overwrites existing resource with same key", func(t *testing.T) {
		resource1 := createTestResourceWithLocalId(t, "overwrite-test")
		resource2 := createTestResourceWithLocalId(t, "overwrite-test")

		err := repo.Save(db, resource1, model_legacy.OperationTypeCreated, "test-tx-1")
		require.NoError(t, err)

		err = repo.Save(db, resource2, model_legacy.OperationTypeUpdated, "test-tx-2")
		require.NoError(t, err)

		key, err := bizmodel.NewReporterResourceKey("overwrite-test", "k8s_cluster", "ocm", "ocm-instance-1")
		require.NoError(t, err)

		foundResource, err := repo.FindResourceByKeys(db, key)
		require.NoError(t, err)
		require.NotNil(t, foundResource)

		assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
	})

	t.Run("FindResourceByKeys works with nil transaction", func(t *testing.T) {
		resource := createTestResource(t)

		// First save the resource using a transaction
		err := repo.Save(db, resource, model_legacy.OperationTypeCreated, "test-tx-1")
		require.NoError(t, err)

		key, err := bizmodel.NewReporterResourceKey(
			"test-resource-123", // Known local resource ID from createTestResource
			"k8s_cluster",
			"ocm",
			"ocm-instance-1",
		)
		require.NoError(t, err)

		// Now test that FindResourceByKeys works with nil transaction
		foundResource, err := repo.FindResourceByKeys(nil, key)
		require.NoError(t, err)
		require.NotNil(t, foundResource)

		assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
	})

	t.Run("FindResourceByKeys with nil transaction returns nil for non-existent resource", func(t *testing.T) {
		key, err := bizmodel.NewReporterResourceKey(
			"non-existent-nil-tx",
			"k8s_cluster",
			"ocm",
			"ocm-instance-1",
		)
		require.NoError(t, err)

		// Test with nil transaction
		foundResource, err := repo.FindResourceByKeys(nil, key)
		require.NoError(t, err)
		assert.Nil(t, foundResource)
	})
}

// nolint:unused // Keep for when outbox event handling is fixed
func setupInMemoryDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&datamodel.Resource{}, &datamodel.ReporterResource{},
		&datamodel.ReporterRepresentation{}, &datamodel.CommonRepresentation{})
	require.NoError(t, err)

	return db
}

func createTestResource(t *testing.T) bizmodel.Resource {
	return createTestResourceWithLocalId(t, "test-resource-123")
}

func createTestResourceWithLocalId(t *testing.T, localResourceId string) bizmodel.Resource {
	resourceId := uuid.New()
	reporterResourceId := uuid.New()

	reporterData := internal.JsonObject{
		"name":      "test-cluster",
		"namespace": "default",
	}

	commonData := internal.JsonObject{
		"workspace_id": "test-workspace",
		"labels":       map[string]interface{}{"env": "test"},
	}

	localResourceIdType, err := bizmodel.NewLocalResourceId(localResourceId)
	require.NoError(t, err)

	resourceType, err := bizmodel.NewResourceType("k8s_cluster")
	require.NoError(t, err)

	reporterType, err := bizmodel.NewReporterType("ocm")
	require.NoError(t, err)

	reporterInstanceId, err := bizmodel.NewReporterInstanceId("ocm-instance-1")
	require.NoError(t, err)

	apiHref, err := bizmodel.NewApiHref("https://api.example.com/resource/123")
	require.NoError(t, err)

	consoleHref, err := bizmodel.NewConsoleHref("https://console.example.com/resource/123")
	require.NoError(t, err)

	reporterRepresentation, err := bizmodel.NewRepresentation(reporterData)
	require.NoError(t, err)

	commonRepresentation, err := bizmodel.NewRepresentation(commonData)
	require.NoError(t, err)

	resourceIdType, err := bizmodel.NewResourceId(resourceId)
	require.NoError(t, err)

	reporterResourceIdType, err := bizmodel.NewReporterResourceId(reporterResourceId)
	require.NoError(t, err)

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, reporterResourceIdType, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
	require.NoError(t, err)

	return resource
}
