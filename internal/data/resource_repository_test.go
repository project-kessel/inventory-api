package data

import (
	"database/sql"
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

}

func TestFindResourceByKeys(t *testing.T) {
	implementations := []struct {
		name string
		repo func() ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() ResourceRepository {
				db := setupInMemoryDB(t)
				tm := NewGormTransactionManager(3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
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
			// Helper function to get fresh instances for each test
			getFreshInstances := func() (ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				tm := NewGormTransactionManager(3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			t.Run("Save and FindResourceByKeys workflow", func(t *testing.T) {
				repo, db := getFreshInstances()

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

			t.Run("FindResourceByKeys returns ErrRecordNotFound for non-existent resource", func(t *testing.T) {
				repo, db := getFreshInstances()

				key, err := bizmodel.NewReporterResourceKey(
					"non-existent",
					"k8s_cluster",
					"ocm",
					"ocm-instance-1",
				)
				require.NoError(t, err)

				foundResource, err := repo.FindResourceByKeys(db, key)
				require.ErrorIs(t, err, gorm.ErrRecordNotFound)
				assert.Nil(t, foundResource)
			})

			t.Run("FindResourceByKeys with different keys returns different resources", func(t *testing.T) {
				resource1 := createTestResourceWithLocalId(t, "resource-1")
				resource2 := createTestResourceWithLocalId(t, "resource-2")

				repo, db := getFreshInstances()

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

				// Verify they are different resources
				reporters1 := found1.ReporterResources()
				reporters2 := found2.ReporterResources()
				require.Len(t, reporters1, 1)
				require.Len(t, reporters2, 1)
				assert.NotEqual(t, reporters1[0].LocalResourceId(), reporters2[0].LocalResourceId())
			})

			t.Run("FindResourceByKeys works with nil transaction", func(t *testing.T) {
				repo, db := getFreshInstances()

				resource := createTestResource(t)
				err := repo.Save(db, resource, model_legacy.OperationTypeCreated, "test-tx-1")
				require.NoError(t, err)

				key, err := bizmodel.NewReporterResourceKey(
					"test-resource-123",
					"k8s_cluster",
					"ocm",
					"ocm-instance-1",
				)
				require.NoError(t, err)

				foundResource, err := repo.FindResourceByKeys(nil, key)
				require.NoError(t, err)
				require.NotNil(t, foundResource)
				assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
			})

			t.Run("FindResourceByKeys with nil transaction returns ErrRecordNotFound for non-existent resource", func(t *testing.T) {
				repo, _ := getFreshInstances()

				key, err := bizmodel.NewReporterResourceKey(
					"non-existent-nil-tx",
					"k8s_cluster",
					"ocm",
					"ocm-instance-1",
				)
				require.NoError(t, err)

				foundResource, err := repo.FindResourceByKeys(nil, key)
				require.ErrorIs(t, err, gorm.ErrRecordNotFound)
				assert.Nil(t, foundResource)
			})

			t.Run("FindResourceByKeys works when reporterInstanceId is not provided in search key", func(t *testing.T) {
				repo, db := getFreshInstances()

				resource := createTestResourceWithLocalId(t, "test-resource-no-instance-lookup")
				err := repo.Save(db, resource, model_legacy.OperationTypeCreated, "test-tx-no-instance")
				require.NoError(t, err)

				key, err := bizmodel.NewReporterResourceKey(
					"test-resource-no-instance-lookup",
					"k8s_cluster",
					"ocm",
					"",
				)
				require.NoError(t, err)

				foundResource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err)
				require.NotNil(t, foundResource)
				assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
			})

			// Case-insensitive tests
			t.Run("Case-insensitive matching", func(t *testing.T) {
				repo, db := getFreshInstances()

				// Create a resource with mixed case values
				resource := createTestResourceWithMixedCase(t)
				err := repo.Save(db, resource, model_legacy.OperationTypeCreated, "test-tx-case")
				require.NoError(t, err)

				testCases := []struct {
					name               string
					localResourceId    string
					resourceType       string
					reporterType       string
					reporterInstanceId string
					description        string
				}{
					{
						name:               "local_resource_id",
						localResourceId:    "Test-Mixed-Case-Resource",
						resourceType:       "K8S_Cluster",
						reporterType:       "OCM",
						reporterInstanceId: "Mixed-Instance-123",
						description:        "should find resource by local_resource_id",
					},
					{
						name:               "lowercase resource_type",
						localResourceId:    "Test-Mixed-Case-Resource",
						resourceType:       "k8s_cluster",
						reporterType:       "OCM",
						reporterInstanceId: "Mixed-Instance-123",
						description:        "should find resource when resource_type is lowercase",
					},
					{
						name:               "lowercase reporter_type",
						localResourceId:    "Test-Mixed-Case-Resource",
						resourceType:       "K8S_Cluster",
						reporterType:       "ocm",
						reporterInstanceId: "Mixed-Instance-123",
						description:        "should find resource when reporter_type is lowercase",
					},
					{
						name:               "lowercase reporter_instance_id",
						localResourceId:    "Test-Mixed-Case-Resource",
						resourceType:       "K8S_Cluster",
						reporterType:       "OCM",
						reporterInstanceId: "mixed-instance-123",
						description:        "should find resource when reporter_instance_id is lowercase",
					},
					{
						name:               "all lowercase",
						localResourceId:    "Test-Mixed-Case-Resource",
						resourceType:       "k8s_cluster",
						reporterType:       "ocm",
						reporterInstanceId: "mixed-instance-123",
						description:        "should find resource when all fields are lowercase",
					},
				}

				for _, tc := range testCases {
					t.Run(tc.name, func(t *testing.T) {
						localResourceIdType, err := bizmodel.NewLocalResourceId(tc.localResourceId)
						require.NoError(t, err)

						resourceTypeType, err := bizmodel.NewResourceType(tc.resourceType)
						require.NoError(t, err)

						reporterTypeType, err := bizmodel.NewReporterType(tc.reporterType)
						require.NoError(t, err)

						reporterInstanceIdType, err := bizmodel.NewReporterInstanceId(tc.reporterInstanceId)
						require.NoError(t, err)

						key, err := bizmodel.NewReporterResourceKey(
							localResourceIdType,
							resourceTypeType,
							reporterTypeType,
							reporterInstanceIdType,
						)
						require.NoError(t, err)

						foundResource, err := repo.FindResourceByKeys(db, key)
						require.NoError(t, err, tc.description)
						require.NotNil(t, foundResource, tc.description)
						assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
					})
				}
			})
		})
	}
}

func TestFindResourceByKeys_TombstoneFilter(t *testing.T) {
	implementations := []struct {
		name string
		repo func() ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() ResourceRepository {
				db := setupInMemoryDB(t)
				tm := NewGormTransactionManager(3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() ResourceRepository {
				return NewFakeResourceRepository()
			},
			db: func() *gorm.DB {
				return nil
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() (ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				tm := NewGormTransactionManager(3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			repo, db := getFreshInstances()

			resource := createTestResourceWithLocalId(t, "tombstoned-resource")
			err := repo.Save(db, resource, model_legacy.OperationTypeCreated, "test-tx-tombstone")
			require.NoError(t, err)

			key, err := bizmodel.NewReporterResourceKey(
				"tombstoned-resource",
				"k8s_cluster",
				"ocm",
				"ocm-instance-1",
			)
			require.NoError(t, err)

			foundResource, err := repo.FindResourceByKeys(db, key)
			require.NoError(t, err)
			require.NotNil(t, foundResource)

			err = foundResource.Delete(key)
			require.NoError(t, err)

			err = repo.Save(db, *foundResource, model_legacy.OperationTypeDeleted, "test-tx-delete")
			require.NoError(t, err)

			foundResource, err = repo.FindResourceByKeys(db, key)
			require.ErrorIs(t, err, gorm.ErrRecordNotFound)
			assert.Nil(t, foundResource)
		})
	}
}

func TestSave(t *testing.T) {
	implementations := []struct {
		name string
		repo func() ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() ResourceRepository {
				db := setupInMemoryDB(t)
				tm := NewGormTransactionManager(3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
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
			// Helper function to get fresh instances for each test
			getFreshInstances := func() (ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				tm := NewGormTransactionManager(3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			t.Run("Save handles duplicate calls gracefully", func(t *testing.T) {
				repo, db := getFreshInstances()

				resource := createTestResourceWithLocalId(t, "update-test")
				err := repo.Save(db, resource, model_legacy.OperationTypeCreated, "test-tx-1")
				require.NoError(t, err)

				key, err := bizmodel.NewReporterResourceKey("update-test", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				apiHref, err := bizmodel.NewApiHref("https://api.example.com/updated")
				require.NoError(t, err)

				consoleHref, err := bizmodel.NewConsoleHref("https://console.example.com/updated")
				require.NoError(t, err)

				updatedReporterData, err := bizmodel.NewRepresentation(map[string]interface{}{
					"name":      "updated-cluster",
					"namespace": "updated-namespace",
				})
				require.NoError(t, err)

				updatedCommonData, err := bizmodel.NewRepresentation(map[string]interface{}{
					"workspace_id": "updated-workspace",
					"labels":       map[string]interface{}{"env": "updated"},
				})
				require.NoError(t, err)

				err = resource.Update(key, apiHref, consoleHref, nil, updatedReporterData, updatedCommonData)
				require.NoError(t, err)

				err = repo.Save(db, resource, model_legacy.OperationTypeUpdated, "test-tx-2")
				require.NoError(t, err)

				foundResource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err)
				require.NotNil(t, foundResource)
				assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
			})

			t.Run("Save creates new resource successfully", func(t *testing.T) {
				repo, db := getFreshInstances()

				resource := createTestResourceWithLocalId(t, "save-new-test")

				err := repo.Save(db, resource, model_legacy.OperationTypeCreated, "test-tx-save")
				require.NoError(t, err)

				key, err := bizmodel.NewReporterResourceKey("save-new-test", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				foundResource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err)
				require.NotNil(t, foundResource)
				assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
			})

			t.Run("Save skips representations with zero value primary keys", func(t *testing.T) {
				if impl.name == "Fake Repository" {
					t.Skip("This test is specific to real repository database operations")
				}

				repo, db := getFreshInstances()

				resource := createTestResourceWithLocalId(t, "zero-pk-test")
				err := repo.Save(db, resource, model_legacy.OperationTypeCreated, "test-tx-zero-pk")
				require.NoError(t, err, "Save should succeed and skip representations with zero value primary keys")

				key, err := bizmodel.NewReporterResourceKey("zero-pk-test", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				foundResource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err, "Resource should be found even if representations were skipped")
				require.NotNil(t, foundResource)
			})
		})
	}
}

func TestResourceRepository_MultipleHostsLifecycle(t *testing.T) {
	implementations := []struct {
		name string
		repo func() ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() ResourceRepository {
				db := setupInMemoryDB(t)
				tm := NewGormTransactionManager(3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() ResourceRepository {
				return NewFakeResourceRepository()
			},
			db: func() *gorm.DB {
				return nil
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() (ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				tm := NewGormTransactionManager(3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			repo, db := getFreshInstances()

			// Create 2 hosts
			host1 := createTestResourceWithLocalIdAndType(t, "host-1", "host")
			host2 := createTestResourceWithLocalIdAndType(t, "host-2", "host")

			err := repo.Save(db, host1, model_legacy.OperationTypeCreated, "tx-create-host1")
			require.NoError(t, err, "Should create host1")

			err = repo.Save(db, host2, model_legacy.OperationTypeCreated, "tx-create-host2")
			require.NoError(t, err, "Should create host2")

			// Verify both hosts can be found
			key1, err := bizmodel.NewReporterResourceKey("host-1", "host", "hbi", "hbi-instance-1")
			require.NoError(t, err)
			key2, err := bizmodel.NewReporterResourceKey("host-2", "host", "hbi", "hbi-instance-1")
			require.NoError(t, err)

			foundHost1, err := repo.FindResourceByKeys(db, key1)
			require.NoError(t, err, "Should find host1 after creation")
			require.NotNil(t, foundHost1)

			foundHost2, err := repo.FindResourceByKeys(db, key2)
			require.NoError(t, err, "Should find host2 after creation")
			require.NotNil(t, foundHost2)

			// Update both hosts
			apiHref, err := bizmodel.NewApiHref("https://api.example.com/updated")
			require.NoError(t, err)
			consoleHref, err := bizmodel.NewConsoleHref("https://console.example.com/updated")
			require.NoError(t, err)
			updatedReporterData, err := bizmodel.NewRepresentation(map[string]interface{}{
				"hostname": "updated-host",
				"status":   "running",
			})
			require.NoError(t, err)
			updatedCommonData, err := bizmodel.NewRepresentation(map[string]interface{}{
				"workspace_id": "updated-workspace",
				"tags":         map[string]interface{}{"env": "prod"},
			})
			require.NoError(t, err)

			err = foundHost1.Update(key1, apiHref, consoleHref, nil, updatedReporterData, updatedCommonData)
			require.NoError(t, err, "Should update host1")

			err = foundHost2.Update(key2, apiHref, consoleHref, nil, updatedReporterData, updatedCommonData)
			require.NoError(t, err, "Should update host2")

			err = repo.Save(db, *foundHost1, model_legacy.OperationTypeUpdated, "tx-update-host1")
			require.NoError(t, err, "Should save updated host1")

			err = repo.Save(db, *foundHost2, model_legacy.OperationTypeUpdated, "tx-update-host2")
			require.NoError(t, err, "Should save updated host2")

			// Verify both updated hosts can still be found
			updatedHost1, err := repo.FindResourceByKeys(db, key1)
			require.NoError(t, err, "Should find host1 after update")
			require.NotNil(t, updatedHost1)

			updatedHost2, err := repo.FindResourceByKeys(db, key2)
			require.NoError(t, err, "Should find host2 after update")
			require.NotNil(t, updatedHost2)

			// Delete both hosts
			err = updatedHost1.Delete(key1)
			require.NoError(t, err, "Should delete host1")

			err = updatedHost2.Delete(key2)
			require.NoError(t, err, "Should delete host2")

			err = repo.Save(db, *updatedHost1, model_legacy.OperationTypeDeleted, "tx-delete-host1")
			require.NoError(t, err, "Should save deleted host1")

			err = repo.Save(db, *updatedHost2, model_legacy.OperationTypeDeleted, "tx-delete-host2")
			require.NoError(t, err, "Should save deleted host2")

			// Verify both hosts are no longer found (tombstoned)
			foundHost1, err = repo.FindResourceByKeys(db, key1)
			require.ErrorIs(t, err, gorm.ErrRecordNotFound, "Should not find deleted host1")
			assert.Nil(t, foundHost1)

			foundHost2, err = repo.FindResourceByKeys(db, key2)
			require.ErrorIs(t, err, gorm.ErrRecordNotFound, "Should not find deleted host2")
			assert.Nil(t, foundHost2)
		})
	}
}

func TestResourceRepository_PartialDataScenarios(t *testing.T) {
	implementations := []struct {
		name string
		repo func() ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() ResourceRepository {
				db := setupInMemoryDB(t)
				tm := NewGormTransactionManager(3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() ResourceRepository {
				return NewFakeResourceRepository()
			},
			db: func() *gorm.DB {
				return nil
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() (ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				tm := NewGormTransactionManager(3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			t.Run("Report resource with just reporter data and no common data", func(t *testing.T) {
				repo, db := getFreshInstances()

				resource := createTestResourceWithReporterDataOnly(t, "reporter-only-resource")
				err := repo.Save(db, resource, model_legacy.OperationTypeCreated, "tx-reporter-only")
				require.NoError(t, err, "Should save resource with only reporter data")

				key, err := bizmodel.NewReporterResourceKey("reporter-only-resource", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				foundResource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err, "Should find resource with only reporter data")
				require.NotNil(t, foundResource)
			})

			t.Run("Report resource with no reporter data but has common data", func(t *testing.T) {
				repo, db := getFreshInstances()

				resource := createTestResourceWithCommonDataOnly(t, "common-only-resource")
				err := repo.Save(db, resource, model_legacy.OperationTypeCreated, "tx-common-only")
				require.NoError(t, err, "Should save resource with only common data")

				key, err := bizmodel.NewReporterResourceKey("common-only-resource", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				foundResource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err, "Should find resource with only common data")
				require.NotNil(t, foundResource)
			})

			t.Run("Report resource with both data, then just reporter data, then just common data", func(t *testing.T) {
				repo, db := getFreshInstances()

				// 1. Report with both reporter and common data
				resourceBoth := createTestResourceWithLocalId(t, "progressive-resource")
				err := repo.Save(db, resourceBoth, model_legacy.OperationTypeCreated, "tx-both")
				require.NoError(t, err, "Should save resource with both data types")

				key, err := bizmodel.NewReporterResourceKey("progressive-resource", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				foundResource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err, "Should find resource after initial save")
				require.NotNil(t, foundResource)

				// 2. Update with just reporter data
				apiHref, err := bizmodel.NewApiHref("https://api.example.com/reporter-update")
				require.NoError(t, err)
				consoleHref, err := bizmodel.NewConsoleHref("https://console.example.com/reporter-update")
				require.NoError(t, err)
				reporterOnlyData, err := bizmodel.NewRepresentation(map[string]interface{}{
					"name":      "reporter-updated-cluster",
					"namespace": "reporter-updated",
				})
				require.NoError(t, err)
				emptyCommonData, err := bizmodel.NewRepresentation(map[string]interface{}{
					"workspace_id": "minimal-workspace",
				})
				require.NoError(t, err)

				err = foundResource.Update(key, apiHref, consoleHref, nil, reporterOnlyData, emptyCommonData)
				require.NoError(t, err, "Should update with reporter data only")

				err = repo.Save(db, *foundResource, model_legacy.OperationTypeUpdated, "tx-reporter-update")
				require.NoError(t, err, "Should save resource with reporter-only update")

				// 3. Update with just common data
				foundResource, err = repo.FindResourceByKeys(db, key)
				require.NoError(t, err, "Should find resource after reporter-only update")

				emptyReporterData, err := bizmodel.NewRepresentation(map[string]interface{}{
					"name": "minimal-cluster",
				})
				require.NoError(t, err)
				commonOnlyData, err := bizmodel.NewRepresentation(map[string]interface{}{
					"workspace_id": "common-updated-workspace",
					"environment":  "common-updated",
				})
				require.NoError(t, err)

				err = foundResource.Update(key, apiHref, consoleHref, nil, emptyReporterData, commonOnlyData)
				require.NoError(t, err, "Should update with common data only")

				err = repo.Save(db, *foundResource, model_legacy.OperationTypeUpdated, "tx-common-update")
				require.NoError(t, err, "Should save resource with common-only update")

				// Verify final resource can still be found
				finalResource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err, "Should find resource after all updates")
				require.NotNil(t, finalResource)
			})
		})
	}
}

func setupInMemoryDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&datamodel.Resource{}, &datamodel.ReporterResource{},
		&datamodel.ReporterRepresentation{}, &datamodel.CommonRepresentation{},
		&model_legacy.OutboxEvent{})
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

func createTestResourceWithLocalIdAndType(t *testing.T, localResourceId, resourceType string) bizmodel.Resource {
	resourceId := uuid.New()
	reporterResourceId := uuid.New()

	var reporterData internal.JsonObject
	var reporterType string
	var reporterInstanceId string

	if resourceType == "host" {
		reporterData = internal.JsonObject{
			"hostname": "test-host",
			"status":   "active",
		}
		reporterType = "hbi"
		reporterInstanceId = "hbi-instance-1"
	} else {
		reporterData = internal.JsonObject{
			"name":      "test-cluster",
			"namespace": "default",
		}
		reporterType = "ocm"
		reporterInstanceId = "ocm-instance-1"
	}

	commonData := internal.JsonObject{
		"workspace_id": "test-workspace",
		"labels":       map[string]interface{}{"env": "test"},
	}

	localResourceIdType, err := bizmodel.NewLocalResourceId(localResourceId)
	require.NoError(t, err)

	resourceTypeType, err := bizmodel.NewResourceType(resourceType)
	require.NoError(t, err)

	reporterTypeType, err := bizmodel.NewReporterType(reporterType)
	require.NoError(t, err)

	reporterInstanceIdType, err := bizmodel.NewReporterInstanceId(reporterInstanceId)
	require.NoError(t, err)

	resourceIdType, err := bizmodel.NewResourceId(resourceId)
	require.NoError(t, err)

	reporterResourceIdType, err := bizmodel.NewReporterResourceId(reporterResourceId)
	require.NoError(t, err)

	apiHref, err := bizmodel.NewApiHref("https://api.example.com/resource/123")
	require.NoError(t, err)

	consoleHref, err := bizmodel.NewConsoleHref("https://console.example.com/resource/123")
	require.NoError(t, err)

	reporterRepresentation, err := bizmodel.NewRepresentation(reporterData)
	require.NoError(t, err)

	commonRepresentation, err := bizmodel.NewRepresentation(commonData)
	require.NoError(t, err)

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceTypeType, reporterTypeType, reporterInstanceIdType, reporterResourceIdType, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
	require.NoError(t, err)

	return resource
}

func createTestResourceWithReporterDataOnly(t *testing.T, localResourceId string) bizmodel.Resource {
	resourceId := uuid.New()
	reporterResourceId := uuid.New()

	reporterData := internal.JsonObject{
		"name":      "test-cluster-reporter-only",
		"namespace": "reporter-namespace",
		"cpu":       "4",
		"memory":    "8Gi",
	}

	// Minimal common data (required for validation)
	commonData := internal.JsonObject{
		"workspace_id": "minimal-workspace",
	}

	localResourceIdType, err := bizmodel.NewLocalResourceId(localResourceId)
	require.NoError(t, err)

	resourceType, err := bizmodel.NewResourceType("k8s_cluster")
	require.NoError(t, err)

	reporterType, err := bizmodel.NewReporterType("ocm")
	require.NoError(t, err)

	reporterInstanceId, err := bizmodel.NewReporterInstanceId("ocm-instance-1")
	require.NoError(t, err)

	resourceIdType, err := bizmodel.NewResourceId(resourceId)
	require.NoError(t, err)

	reporterResourceIdType, err := bizmodel.NewReporterResourceId(reporterResourceId)
	require.NoError(t, err)

	apiHref, err := bizmodel.NewApiHref("https://api.example.com/reporter-only")
	require.NoError(t, err)

	consoleHref, err := bizmodel.NewConsoleHref("https://console.example.com/reporter-only")
	require.NoError(t, err)

	reporterRepresentation, err := bizmodel.NewRepresentation(reporterData)
	require.NoError(t, err)

	commonRepresentation, err := bizmodel.NewRepresentation(commonData)
	require.NoError(t, err)

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, reporterResourceIdType, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
	require.NoError(t, err)

	return resource
}

func createTestResourceWithCommonDataOnly(t *testing.T, localResourceId string) bizmodel.Resource {
	resourceId := uuid.New()
	reporterResourceId := uuid.New()

	// Minimal reporter data (required for validation)
	reporterData := internal.JsonObject{
		"name": "minimal-cluster",
	}

	commonData := internal.JsonObject{
		"workspace_id": "test-workspace-common-only",
		"labels":       map[string]interface{}{"env": "test", "type": "common-only"},
		"owner":        "test-team",
		"region":       "us-east-1",
	}

	localResourceIdType, err := bizmodel.NewLocalResourceId(localResourceId)
	require.NoError(t, err)

	resourceType, err := bizmodel.NewResourceType("k8s_cluster")
	require.NoError(t, err)

	reporterType, err := bizmodel.NewReporterType("ocm")
	require.NoError(t, err)

	reporterInstanceId, err := bizmodel.NewReporterInstanceId("ocm-instance-1")
	require.NoError(t, err)

	resourceIdType, err := bizmodel.NewResourceId(resourceId)
	require.NoError(t, err)

	reporterResourceIdType, err := bizmodel.NewReporterResourceId(reporterResourceId)
	require.NoError(t, err)

	apiHref, err := bizmodel.NewApiHref("https://api.example.com/common-only")
	require.NoError(t, err)

	consoleHref, err := bizmodel.NewConsoleHref("https://console.example.com/common-only")
	require.NoError(t, err)

	reporterRepresentation, err := bizmodel.NewRepresentation(reporterData)
	require.NoError(t, err)

	commonRepresentation, err := bizmodel.NewRepresentation(commonData)
	require.NoError(t, err)

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, reporterResourceIdType, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
	require.NoError(t, err)

	return resource
}

func createTestResourceWithMixedCase(t *testing.T) bizmodel.Resource {
	resourceId := uuid.New()
	reporterResourceId := uuid.New()

	reporterData := internal.JsonObject{
		"name":      "test-cluster-mixed",
		"namespace": "default",
	}

	commonData := internal.JsonObject{
		"workspace_id": "test-workspace-mixed",
		"labels":       map[string]interface{}{"env": "test"},
	}

	localResourceIdType, err := bizmodel.NewLocalResourceId("Test-Mixed-Case-Resource")
	require.NoError(t, err)

	resourceType, err := bizmodel.NewResourceType("K8S_Cluster")
	require.NoError(t, err)

	reporterType, err := bizmodel.NewReporterType("OCM")
	require.NoError(t, err)

	reporterInstanceId, err := bizmodel.NewReporterInstanceId("Mixed-Instance-123")
	require.NoError(t, err)

	resourceIdType, err := bizmodel.NewResourceId(resourceId)
	require.NoError(t, err)

	reporterResourceIdType, err := bizmodel.NewReporterResourceId(reporterResourceId)
	require.NoError(t, err)

	apiHref, err := bizmodel.NewApiHref("https://api.example.com/mixed-case")
	require.NoError(t, err)

	consoleHref, err := bizmodel.NewConsoleHref("https://console.example.com/mixed-case")
	require.NoError(t, err)

	reporterRepresentation, err := bizmodel.NewRepresentation(reporterData)
	require.NoError(t, err)

	commonRepresentation, err := bizmodel.NewRepresentation(commonData)
	require.NoError(t, err)

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, reporterResourceIdType, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
	require.NoError(t, err)

	return resource
}
