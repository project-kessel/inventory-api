package data

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/testutil"
)

// Helper function to create a pointer to a uint
func ptrUint(v uint) *uint {
	return &v
}

func TestResourceRepositoryContract(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
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

func testRepositoryContract(t *testing.T, repo bizmodel.ResourceRepository, db *gorm.DB) {
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

	t.Run("Save and FindResourceByKeys basic workflow", func(t *testing.T) {
		resource := createTestResourceWithLocalId(t, "contract-test-1")
		err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "contract-tx-1")
		require.NoError(t, err, "Save should succeed")

		key := createContractReporterResourceKey(t, "contract-test-1", "k8s_cluster", "ocm", "ocm-instance-1")

		foundResource, err := repo.FindResourceByKeys(db, key)
		require.NoError(t, err, "Find should succeed")
		require.NotNil(t, foundResource, "Found resource should not be nil")
		assert.Len(t, foundResource.ReporterResources(), 1, "Should have one reporter resource")
	})

	t.Run("FindResourceByKeys returns ErrRecordNotFound for non-existent", func(t *testing.T) {
		key := createContractReporterResourceKey(t, "non-existent-contract", "k8s_cluster", "ocm", "ocm-instance-1")

		foundResource, err := repo.FindResourceByKeys(db, key)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound, "Should return ErrRecordNotFound")
		assert.Nil(t, foundResource, "Found resource should be nil")
	})

	t.Run("Save-Update-Save workflow", func(t *testing.T) {
		// Create initial resource
		resource := createTestResourceWithLocalId(t, "contract-update-test")
		err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "contract-tx-create")
		require.NoError(t, err, "Initial save should succeed")

		// Find and update
		key := createContractReporterResourceKey(t, "contract-update-test", "k8s_cluster", "ocm", "ocm-instance-1")

		foundResource, err := repo.FindResourceByKeys(db, key)
		require.NoError(t, err, "Find should succeed")
		require.NotNil(t, foundResource)

		// Update the resource
		apiHref, _ := bizmodel.NewApiHref("https://api.example.com/updated")
		consoleHref, _ := bizmodel.NewConsoleHref("https://console.example.com/updated")
		reporterData, _ := bizmodel.NewRepresentation(map[string]interface{}{"updated": true})
		commonData, _ := bizmodel.NewRepresentation(map[string]interface{}{"workspace_id": "updated-workspace"})
		transactionId := newUniqueTxID("test-transaction-id-update-contract")

		err = foundResource.Update(key, apiHref, consoleHref, nil, reporterData, commonData, transactionId)
		require.NoError(t, err, "Update should succeed")

		// Save updated resource
		err = repo.Save(db, *foundResource, bizmodel.OperationTypeUpdated, "contract-tx-update")
		require.NoError(t, err, "Updated save should succeed")

		// Verify update persisted
		updatedResource, err := repo.FindResourceByKeys(db, key)
		require.NoError(t, err, "Find updated resource should succeed")
		require.NotNil(t, updatedResource)
	})

	t.Run("Save-Delete workflow", func(t *testing.T) {
		// Create resource
		resource := createTestResourceWithLocalId(t, "contract-delete-test")
		err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "contract-tx-create")
		require.NoError(t, err, "Initial save should succeed")

		// Find and delete
		key := createContractReporterResourceKey(t, "contract-delete-test", "k8s_cluster", "ocm", "ocm-instance-1")

		foundResource, err := repo.FindResourceByKeys(db, key)
		require.NoError(t, err, "Find should succeed")
		require.NotNil(t, foundResource)

		// Delete the resource
		err = foundResource.Delete(key)
		require.NoError(t, err, "Delete should succeed")

		// Save deleted resource
		err = repo.Save(db, *foundResource, bizmodel.OperationTypeDeleted, "contract-tx-delete")
		require.NoError(t, err, "Delete save should succeed")

		// Verify deletion behavior is consistent
		deletedResource, err := repo.FindResourceByKeys(db, key)
		if err == gorm.ErrRecordNotFound {
			assert.Nil(t, deletedResource, "Deleted resource should not be found with tombstone filter")
		} else {
			require.NoError(t, err, "Find should succeed if tombstone filter removed")
			require.NotNil(t, deletedResource, "Should find tombstoned resource")
		}
	})

	t.Run("Unique constraint enforcement", func(t *testing.T) {
		// Create first resource
		resource1 := createTestResourceWithLocalId(t, "contract-unique-test")
		err := repo.Save(db, resource1, bizmodel.OperationTypeCreated, "contract-tx-1")
		require.NoError(t, err, "First save should succeed")

		// Try to create second resource with same composite key
		resource2 := createTestResourceWithLocalId(t, "contract-unique-test")
		err = repo.Save(db, resource2, bizmodel.OperationTypeCreated, "contract-tx-2")
		require.Error(t, err, "Second save with duplicate key should fail")

		// Error should indicate constraint violation
		errorMsg := err.Error()
		constraintViolation := strings.Contains(errorMsg, "duplicate") || strings.Contains(errorMsg, "NON-UNIQUE TRANSACTION ID")
		assert.True(t, constraintViolation, "Error should mention constraint violation, got: %s", errorMsg)
	})

	t.Run("Case insensitive key matching for non ID fields", func(t *testing.T) {
		// Create resource with mixed case
		resource := createTestResourceWithReporter(t, "Contract-Case-Test", "OCM", "Instance-1")
		err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "contract-case-tx")
		require.NoError(t, err, "Save should succeed")

		// Find with different casing
		key := createContractReporterResourceKey(t, "Contract-Case-Test", "k8s_cluster", "ocm", "Instance-1")

		foundResource, err := repo.FindResourceByKeys(db, key)
		require.NoError(t, err, "Case insensitive find should succeed")
		require.NotNil(t, foundResource)
	})

	t.Run("Transaction handling", func(t *testing.T) {
		// Test with nil transaction (only works for fake repository)
		if db == nil {
			// Fake repository test
			resource := createTestResourceWithLocalId(t, "contract-nil-tx-test")
			err := repo.Save(nil, resource, bizmodel.OperationTypeCreated, "contract-nil-tx")
			require.NoError(t, err, "Save with nil transaction should succeed in fake repo")

			key := createContractReporterResourceKey(t, "contract-nil-tx-test", "k8s_cluster", "ocm", "ocm-instance-1")

			foundResource, err := repo.FindResourceByKeys(nil, key)
			require.NoError(t, err, "Find with nil transaction should succeed in fake repo")
			require.NotNil(t, foundResource)
		} else {
			// Real repository test - use actual db transaction
			resource := createTestResourceWithLocalId(t, "contract-real-tx-test")
			err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "contract-real-tx")
			require.NoError(t, err, "Save with db transaction should succeed in real repo")

			key := createContractReporterResourceKey(t, "contract-real-tx-test", "k8s_cluster", "ocm", "ocm-instance-1")

			foundResource, err := repo.FindResourceByKeys(db, key)
			require.NoError(t, err, "Find with db transaction should succeed in real repo")
			require.NotNil(t, foundResource)
		}
	})

	t.Run("Lifecycle: Create-Update-Delete-Recreate", func(t *testing.T) {
		localResourceId := "contract-lifecycle-test"

		// 1. Create
		resource := createTestResourceWithLocalId(t, localResourceId)
		err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "contract-create")
		require.NoError(t, err, "Create should succeed")

		key := createContractReporterResourceKey(t, localResourceId, "k8s_cluster", "ocm", "ocm-instance-1")

		// 2. Update
		foundResource, err := repo.FindResourceByKeys(db, key)
		require.NoError(t, err, "Find for update should succeed")

		apiHref, _ := bizmodel.NewApiHref("https://api.example.com/contract-updated")
		consoleHref, _ := bizmodel.NewConsoleHref("https://console.example.com/contract-updated")
		reporterData, _ := bizmodel.NewRepresentation(map[string]interface{}{"contract": "updated"})
		commonData, _ := bizmodel.NewRepresentation(map[string]interface{}{"workspace_id": "contract-workspace"})
		transactionId := newUniqueTxID("test-transaction-id-lifecycle-contract")

		err = foundResource.Update(key, apiHref, consoleHref, nil, reporterData, commonData, transactionId)
		require.NoError(t, err, "Update should succeed")

		err = repo.Save(db, *foundResource, bizmodel.OperationTypeUpdated, "contract-update")
		require.NoError(t, err, "Update save should succeed")

		// 3. Delete
		deletedResource, err := repo.FindResourceByKeys(db, key)
		require.NoError(t, err, "Find for delete should succeed")

		err = deletedResource.Delete(key)
		require.NoError(t, err, "Delete should succeed")

		err = repo.Save(db, *deletedResource, bizmodel.OperationTypeDeleted, "contract-delete")
		require.NoError(t, err, "Delete save should succeed")

		// 4. Verify delete behavior
		postDeleteResource, err := repo.FindResourceByKeys(db, key)
		// Both implementations should behave the same way here
		if err == gorm.ErrRecordNotFound {
			assert.Nil(t, postDeleteResource, "Consistent not found behavior")
		} else {
			require.NoError(t, err, "Consistent found behavior")
			require.NotNil(t, postDeleteResource, "Consistent non-nil resource")
		}

		// 5. Recreate (this should work the same way in both implementations)
		newResource := createTestResourceWithLocalId(t, localResourceId)
		err = repo.Save(db, newResource, bizmodel.OperationTypeCreated, "contract-recreate")

		// The behavior should be identical between implementations
		recreateResource, findErr := repo.FindResourceByKeys(db, key)
		if err == nil {
			// Recreate succeeded
			require.NoError(t, err, "Recreate should succeed consistently")
			require.NoError(t, findErr, "Find after recreate should succeed")
			require.NotNil(t, recreateResource, "Recreated resource should be found")
		} else {
			// Recreate failed - both should fail the same way
			require.Error(t, err, "Recreate should fail consistently")
		}
	})

}

func TestFindResourceByKeys(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
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
			getFreshInstances := func() (bizmodel.ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			t.Run("Save and FindResourceByKeys workflow", func(t *testing.T) {
				repo, db := getFreshInstances()

				resource := createTestResource(t)
				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "test-tx-123")
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

				err := repo.Save(db, resource1, bizmodel.OperationTypeCreated, "test-tx-1")
				require.NoError(t, err)
				err = repo.Save(db, resource2, bizmodel.OperationTypeCreated, "test-tx-2")
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
				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "test-tx-1")
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
				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "test-tx-no-instance")
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
				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "test-tx-case")
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
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
			db: func() *gorm.DB {
				return nil
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() (bizmodel.ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			repo, db := getFreshInstances()

			resource := createTestResourceWithLocalId(t, "tombstoned-resource")
			err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "test-tx-tombstone")
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

			err = repo.Save(db, *foundResource, bizmodel.OperationTypeDeleted, "test-tx-delete")
			require.NoError(t, err)

			// With tombstone filter removed, we should be able to find the tombstoned resource
			foundResource, err = repo.FindResourceByKeys(db, key)
			require.NoError(t, err)
			require.NotNil(t, foundResource)

			// Verify we got the tombstoned resource back
			reporterResources := foundResource.ReporterResources()
			require.Len(t, reporterResources, 1, "should have one reporter resource")

			// The resource should still be the same one we deleted
			assert.Equal(t, "tombstoned-resource", reporterResources[0].LocalResourceId())
		})
	}
}

func TestUniqueConstraint_ReporterResourceCompositeKey(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
			db: func() *gorm.DB {
				return nil
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() (bizmodel.ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			t.Run("should reject duplicate composite key", func(t *testing.T) {
				repo, db := getFreshInstances()

				// Create first resource
				resource1 := createTestResourceWithLocalId(t, "duplicate-key-test")
				err := repo.Save(db, resource1, bizmodel.OperationTypeCreated, "test-tx-1")
				require.NoError(t, err, "First save should succeed")

				// Create second resource with same composite key components
				// (same LocalResourceID, ReporterType, ResourceType, ReporterInstanceID, RepresentationVersion=0, Generation=0)
				resource2 := createTestResourceWithLocalId(t, "duplicate-key-test") // Same local ID
				err = repo.Save(db, resource2, bizmodel.OperationTypeCreated, "test-tx-2")

				// Both implementations should reject this duplicate
				require.Error(t, err, "Second save with duplicate composite key should fail")

				// Error should indicate a constraint violation
				errorMsg := err.Error()
				// Both "duplicate" (fake repo) and "NON-UNIQUE TRANSACTION ID" (real DB) are acceptable
				constraintViolation := strings.Contains(errorMsg, "duplicate") || strings.Contains(errorMsg, "NON-UNIQUE TRANSACTION ID")
				assert.True(t, constraintViolation, "Error should mention constraint violation, got: %s", errorMsg)
			})

			t.Run("should allow same key with different versions", func(t *testing.T) {
				repo, db := getFreshInstances()

				// Create and save initial resource
				resource := createTestResourceWithLocalId(t, "version-test-resource")
				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "test-tx-create")
				require.NoError(t, err, "Initial save should succeed")

				// Update the resource (this increments representation version and potentially generation)
				key, err := bizmodel.NewReporterResourceKey(
					"version-test-resource",
					"k8s_cluster",
					"ocm",
					"ocm-instance-1",
				)
				require.NoError(t, err)

				apiHref, _ := bizmodel.NewApiHref("https://api.example.com/updated")
				consoleHref, _ := bizmodel.NewConsoleHref("https://console.example.com/updated")
				reporterData, _ := bizmodel.NewRepresentation(map[string]interface{}{"update": "1"})
				commonData, _ := bizmodel.NewRepresentation(map[string]interface{}{"update": "1"})
				transactionId := newUniqueTxID("test-transaction-id-version-unique")

				err = resource.Update(key, apiHref, consoleHref, nil, reporterData, commonData, transactionId)
				require.NoError(t, err, "Update should succeed")

				// Save the updated resource (different version/generation should be allowed)
				err = repo.Save(db, resource, bizmodel.OperationTypeUpdated, "test-tx-update")
				require.NoError(t, err, "Save with different version should succeed")
			})

			t.Run("should allow same key components with different resource types", func(t *testing.T) {
				repo, db := getFreshInstances()

				// Create first resource with k8s_cluster type
				resource1 := createTestResourceWithLocalIdAndType(t, "multi-type-test", "k8s_cluster")
				err := repo.Save(db, resource1, bizmodel.OperationTypeCreated, "test-tx-1")
				require.NoError(t, err, "First save should succeed")

				// Create second resource with same local ID but different resource type
				resource2 := createTestResourceWithLocalIdAndType(t, "multi-type-test", "host")
				err = repo.Save(db, resource2, bizmodel.OperationTypeCreated, "test-tx-2")
				require.NoError(t, err, "Save with different resource type should succeed")
			})

			t.Run("should allow same key components with different reporter types", func(t *testing.T) {
				repo, db := getFreshInstances()

				// Create resource with OCM reporter
				resource1 := createTestResourceWithReporter(t, "reporter-test", "ocm", "ocm-instance-1")
				err := repo.Save(db, resource1, bizmodel.OperationTypeCreated, "test-tx-1")
				require.NoError(t, err, "First save should succeed")

				// Create resource with same local ID but different reporter type
				resource2 := createTestResourceWithReporter(t, "reporter-test", "hbi", "hbi-instance-1")
				err = repo.Save(db, resource2, bizmodel.OperationTypeCreated, "test-tx-2")
				require.NoError(t, err, "Save with different reporter type should succeed")
			})

			t.Run("should allow same key components with different reporter instances", func(t *testing.T) {
				repo, db := getFreshInstances()

				// Create resource with instance-1
				resource1 := createTestResourceWithReporter(t, "instance-test", "ocm", "ocm-instance-1")
				err := repo.Save(db, resource1, bizmodel.OperationTypeCreated, "test-tx-1")
				require.NoError(t, err, "First save should succeed")

				// Create resource with same components but different reporter instance
				resource2 := createTestResourceWithReporter(t, "instance-test", "ocm", "ocm-instance-2")
				err = repo.Save(db, resource2, bizmodel.OperationTypeCreated, "test-tx-2")
				require.NoError(t, err, "Save with different reporter instance should succeed")
			})
		})
	}
}

func TestResourceRepository_IdempotentOperations(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
			db: func() *gorm.DB {
				return nil
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() (bizmodel.ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			t.Run("report -> delete -> resubmit same delete", func(t *testing.T) {
				repo, db := getFreshInstances()

				localResourceId := "repo-idempotent-delete-test"

				// 1. REPORT: Create initial resource
				resource := createTestResourceWithLocalId(t, localResourceId)
				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "repo-create-1")
				require.NoError(t, err, "Initial save should succeed")

				key := createContractReporterResourceKey(t, localResourceId, "k8s_cluster", "ocm", "ocm-instance-1")

				// Verify initial state
				afterCreate, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err, "Should find resource after creation")
				require.NotNil(t, afterCreate)
				initialState := afterCreate.ReporterResources()[0].Serialize()
				assert.Equal(t, uint(0), initialState.RepresentationVersion, "Initial representationVersion should be 0")
				assert.Equal(t, uint(0), initialState.Generation, "Initial generation should be 0")
				assert.False(t, initialState.Tombstone, "Initial tombstone should be false")

				// 2. DELETE: Delete the resource
				foundResource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err, "Should find resource for delete")
				require.NotNil(t, foundResource)

				err = foundResource.Delete(key)
				require.NoError(t, err, "Delete operation should succeed")

				err = repo.Save(db, *foundResource, bizmodel.OperationTypeDeleted, "repo-delete-1")
				require.NoError(t, err, "Delete save should succeed")

				// Verify delete state
				afterDelete1, err := repo.FindResourceByKeys(db, key)
				if err == gorm.ErrRecordNotFound {
					// If tombstone filter is active, we can't verify the exact state
					// but the delete succeeded, which is what we're testing
					t.Log("Delete succeeded, resource not found due to tombstone filter")
				} else {
					require.NoError(t, err, "Should find tombstoned resource")
					require.NotNil(t, afterDelete1)
					deleteState1 := afterDelete1.ReporterResources()[0].Serialize()
					assert.Equal(t, uint(1), deleteState1.RepresentationVersion, "RepresentationVersion should increment after delete")
					assert.Equal(t, uint(0), deleteState1.Generation, "Generation should remain 0 after delete")
					assert.True(t, deleteState1.Tombstone, "Resource should be tombstoned")
				}

				// 3. RESUBMIT SAME DELETE: Should succeed (handle duplicate gracefully)
				foundResource2, err := repo.FindResourceByKeys(db, key)
				if err == gorm.ErrRecordNotFound {
					// With tombstone filter, we expect this behavior
					t.Log("Cannot resubmit delete - resource not found due to tombstone filter (expected)")
				} else {
					require.NoError(t, err, "Should find resource for duplicate delete")
					require.NotNil(t, foundResource2)

					err = foundResource2.Delete(key)
					require.NoError(t, err, "Duplicate delete operation should succeed")

					err = repo.Save(db, *foundResource2, bizmodel.OperationTypeDeleted, "repo-delete-2")
					require.NoError(t, err, "Duplicate delete save should succeed")

					// Verify state after duplicate delete
					afterDelete2, err := repo.FindResourceByKeys(db, key)
					if err != gorm.ErrRecordNotFound {
						require.NoError(t, err, "Should find resource after duplicate delete")
						require.NotNil(t, afterDelete2)
						deleteState2 := afterDelete2.ReporterResources()[0].Serialize()
						// RepresentationVersion should NOT increment for duplicate deletes on already tombstoned resources
						assert.Equal(t, uint(1), deleteState2.RepresentationVersion, "RepresentationVersion should remain unchanged for duplicate delete on tombstoned resource")
						assert.True(t, deleteState2.Tombstone, "Resource should still be tombstoned")
					}
				}
			})

			t.Run("report -> resubmit same report -> delete -> resubmit same delete", func(t *testing.T) {
				repo, db := getFreshInstances()

				localResourceId := "repo-idempotent-full-test"

				// 1. REPORT: Create initial resource
				resource1 := createTestResourceWithLocalId(t, localResourceId)
				err := repo.Save(db, resource1, bizmodel.OperationTypeCreated, "repo-create-1")
				require.NoError(t, err, "Initial save should succeed")

				key := createContractReporterResourceKey(t, localResourceId, "k8s_cluster", "ocm", "ocm-instance-1")

				// 2. RESUBMIT SAME REPORT: Should succeed and increment version
				foundResource1, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err, "Should find resource for update")
				require.NotNil(t, foundResource1)

				apiHref, _ := bizmodel.NewApiHref("https://api.example.com/duplicate")
				consoleHref, _ := bizmodel.NewConsoleHref("https://console.example.com/duplicate")
				reporterData, _ := bizmodel.NewRepresentation(map[string]interface{}{"duplicate": "report"})
				commonData, _ := bizmodel.NewRepresentation(map[string]interface{}{"workspace_id": "duplicate-workspace"})
				transactionId := newUniqueTxID("test-transaction-id-duplicate-idempotent")

				err = foundResource1.Update(key, apiHref, consoleHref, nil, reporterData, commonData, transactionId)
				require.NoError(t, err, "Update should succeed")

				err = repo.Save(db, *foundResource1, bizmodel.OperationTypeUpdated, "repo-update-1")
				require.NoError(t, err, "Duplicate report save should succeed")

				// 3. DELETE: Delete the resource
				foundResource2, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err, "Should find resource for delete")
				require.NotNil(t, foundResource2)

				err = foundResource2.Delete(key)
				require.NoError(t, err, "Delete operation should succeed")

				err = repo.Save(db, *foundResource2, bizmodel.OperationTypeDeleted, "repo-delete-1")
				require.NoError(t, err, "Delete save should succeed")

				// 4. RESUBMIT SAME DELETE: Should succeed
				foundResource3, err := repo.FindResourceByKeys(db, key)
				if err == gorm.ErrRecordNotFound {
					// With tombstone filter, we expect this behavior
					t.Log("Cannot resubmit delete - resource not found due to tombstone filter (expected)")
				} else {
					require.NoError(t, err, "Should find resource for duplicate delete")
					require.NotNil(t, foundResource3)

					err = foundResource3.Delete(key)
					require.NoError(t, err, "Duplicate delete operation should succeed")

					err = repo.Save(db, *foundResource3, bizmodel.OperationTypeDeleted, "repo-delete-2")
					require.NoError(t, err, "Duplicate delete save should succeed")
				}
			})

			t.Run("complex idempotency: multiple report and delete cycles", func(t *testing.T) {
				repo, db := getFreshInstances()

				localResourceId := "repo-complex-idempotent-test"
				key := createContractReporterResourceKey(t, localResourceId, "k8s_cluster", "ocm", "ocm-instance-1")

				// Multiple cycles of report -> delete to test robustness
				for cycle := 0; cycle < 3; cycle++ {
					t.Logf("Cycle %d: Report and Delete", cycle)

					// Report: Check if resource exists, create or update accordingly
					foundResource, err := repo.FindResourceByKeys(db, key)

					if err == gorm.ErrRecordNotFound {
						// Resource doesn't exist - create new one
						t.Logf("Cycle %d: Creating new resource", cycle)
						resource := createTestResourceWithLocalId(t, localResourceId)
						err := repo.Save(db, resource, bizmodel.OperationTypeCreated, fmt.Sprintf("repo-cycle-%d-create", cycle))
						require.NoError(t, err, "Save should succeed in cycle %d", cycle)
					} else {
						// Resource exists (potentially tombstoned) - update it
						require.NoError(t, err, "Should find existing resource in cycle %d", cycle)
						require.NotNil(t, foundResource)
						t.Logf("Cycle %d: Updating existing resource (generation should increment if tombstoned)", cycle)

						apiHref, _ := bizmodel.NewApiHref(fmt.Sprintf("https://api.example.com/cycle-%d", cycle))
						consoleHref, _ := bizmodel.NewConsoleHref(fmt.Sprintf("https://console.example.com/cycle-%d", cycle))
						reporterData, _ := bizmodel.NewRepresentation(map[string]interface{}{"cycle": cycle})
						commonData, _ := bizmodel.NewRepresentation(map[string]interface{}{"workspace_id": fmt.Sprintf("cycle-%d-workspace", cycle)})
						transactionId := newUniqueTxID(fmt.Sprintf("test-transaction-id-cycle-%d-idempotent", cycle))

						err = foundResource.Update(key, apiHref, consoleHref, nil, reporterData, commonData, transactionId)
						require.NoError(t, err, "Update should succeed in cycle %d", cycle)

						err = repo.Save(db, *foundResource, bizmodel.OperationTypeUpdated, fmt.Sprintf("repo-cycle-%d-update", cycle))
						require.NoError(t, err, "Update save should succeed in cycle %d", cycle)
					}

					// Verify current state after report/update
					currentResource, err := repo.FindResourceByKeys(db, key)
					require.NoError(t, err, "Should find resource after report/update in cycle %d", cycle)
					require.NotNil(t, currentResource)
					currentState := currentResource.ReporterResources()[0].Serialize()
					t.Logf("Cycle %d after report: Generation=%d, RepVersion=%d, Tombstone=%t",
						cycle, currentState.Generation, currentState.RepresentationVersion, currentState.Tombstone)

					// Delete
					err = currentResource.Delete(key)
					require.NoError(t, err, "Delete should succeed in cycle %d", cycle)

					err = repo.Save(db, *currentResource, bizmodel.OperationTypeDeleted, fmt.Sprintf("repo-cycle-%d-delete", cycle))
					require.NoError(t, err, "Delete save should succeed in cycle %d", cycle)

					// Verify state after delete
					deletedResource, err := repo.FindResourceByKeys(db, key)
					if err == gorm.ErrRecordNotFound {
						t.Logf("Cycle %d: Resource not found after delete (tombstone filter active)", cycle)
					} else {
						require.NoError(t, err, "Should find tombstoned resource in cycle %d", cycle)
						require.NotNil(t, deletedResource)
						deleteState := deletedResource.ReporterResources()[0].Serialize()
						assert.True(t, deleteState.Tombstone, "Resource should be tombstoned in cycle %d", cycle)
						t.Logf("Cycle %d after delete: Generation=%d, RepVersion=%d, Tombstone=%t",
							cycle, deleteState.Generation, deleteState.RepresentationVersion, deleteState.Tombstone)
					}
				}
			})
		})
	}
}

func TestSave(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
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
			getFreshInstances := func() (bizmodel.ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			t.Run("Save handles duplicate calls gracefully", func(t *testing.T) {
				repo, db := getFreshInstances()

				resource := createTestResourceWithLocalId(t, "update-test")
				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "test-tx-1")
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

				updatedTransactionId := newUniqueTxID("updated-transaction-id-save-test-unique")

				err = resource.Update(key, apiHref, consoleHref, nil, updatedReporterData, updatedCommonData, updatedTransactionId)
				require.NoError(t, err)

				err = repo.Save(db, resource, bizmodel.OperationTypeUpdated, "test-tx-2")
				require.NoError(t, err)

				foundResource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err)
				require.NotNil(t, foundResource)
				assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
			})

			t.Run("Save creates new resource successfully", func(t *testing.T) {
				repo, db := getFreshInstances()

				resource := createTestResourceWithLocalId(t, "save-new-test")

				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "test-tx-save")
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
				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "test-tx-zero-pk")
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
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
			db: func() *gorm.DB {
				return nil
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() (bizmodel.ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			repo, db := getFreshInstances()

			// Create 2 hosts
			host1 := createTestResourceWithLocalIdAndType(t, "host-1", "host")
			host2 := createTestResourceWithLocalIdAndType(t, "host-2", "host")

			err := repo.Save(db, host1, bizmodel.OperationTypeCreated, "tx-create-host1")
			require.NoError(t, err, "Should create host1")

			err = repo.Save(db, host2, bizmodel.OperationTypeCreated, "tx-create-host2")
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

			updatedTransactionId1 := newUniqueTxID("updated-transaction-id-multiple-hosts-unique-host1")
			updatedTransactionId2 := newUniqueTxID("updated-transaction-id-multiple-hosts-unique-host2")

			err = foundHost1.Update(key1, apiHref, consoleHref, nil, updatedReporterData, updatedCommonData, updatedTransactionId1)
			require.NoError(t, err, "Should update host1")

			err = foundHost2.Update(key2, apiHref, consoleHref, nil, updatedReporterData, updatedCommonData, updatedTransactionId2)
			require.NoError(t, err, "Should update host2")

			err = repo.Save(db, *foundHost1, bizmodel.OperationTypeUpdated, "tx-update-host1")
			require.NoError(t, err, "Should save updated host1")

			err = repo.Save(db, *foundHost2, bizmodel.OperationTypeUpdated, "tx-update-host2")
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

			err = repo.Save(db, *updatedHost1, bizmodel.OperationTypeDeleted, "tx-delete-host1")
			require.NoError(t, err, "Should save deleted host1")

			err = repo.Save(db, *updatedHost2, bizmodel.OperationTypeDeleted, "tx-delete-host2")
			require.NoError(t, err, "Should save deleted host2")

			// Verify both hosts can be found (tombstoned) with tombstone filter removed
			foundHost1, err = repo.FindResourceByKeys(db, key1)
			require.NoError(t, err, "Should find tombstoned host1")
			require.NotNil(t, foundHost1)
			assert.True(t, foundHost1.ReporterResources()[0].Serialize().Tombstone, "Host1 should be tombstoned")

			foundHost2, err = repo.FindResourceByKeys(db, key2)
			require.NoError(t, err, "Should find tombstoned host2")
			require.NotNil(t, foundHost2)
			assert.True(t, foundHost2.ReporterResources()[0].Serialize().Tombstone, "Host2 should be tombstoned")
		})
	}
}

func TestResourceRepository_PartialDataScenarios(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
			db: func() *gorm.DB {
				return nil
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() (bizmodel.ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			t.Run("Report resource with just reporter data and no common data", func(t *testing.T) {
				repo, db := getFreshInstances()

				resource := createTestResourceWithReporterDataOnly(t, "reporter-only-resource")
				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "tx-reporter-only")
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
				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "tx-common-only")
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
				err := repo.Save(db, resourceBoth, bizmodel.OperationTypeCreated, "tx-both")
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

				updatedTransactionId1 := newUniqueTxID("updated-transaction-id-partial-data-unique-reporter")

				err = foundResource.Update(key, apiHref, consoleHref, nil, reporterOnlyData, emptyCommonData, updatedTransactionId1)
				require.NoError(t, err, "Should update with reporter data only")

				err = repo.Save(db, *foundResource, bizmodel.OperationTypeUpdated, "tx-reporter-update")
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

				updatedTransactionId2 := newUniqueTxID("updated-transaction-id-partial-data-unique-common")

				err = foundResource.Update(key, apiHref, consoleHref, nil, emptyReporterData, commonOnlyData, updatedTransactionId2)
				require.NoError(t, err, "Should update with common data only")

				err = repo.Save(db, *foundResource, bizmodel.OperationTypeUpdated, "tx-common-update")
				require.NoError(t, err, "Should save resource with common-only update")

				// Verify final resource can still be found
				finalResource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err, "Should find resource after all updates")
				require.NotNil(t, finalResource)
			})
		})
	}
}

func TestSerializableCreateFails(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			// Fresh instances
			db := setupInMemoryDB(t)
			mc := metricscollector.NewFakeMetricsCollector()
			tm := NewGormTransactionManager(mc, 3)
			repo := NewResourceRepository(db, tm)

			resource := createTestResourceWithLocalId(t, "serializable-create-conflict")

			// Begin a conflicting serializable transaction and create the same resource
			conflictTx := db.Begin(&sql.TxOptions{Isolation: sql.LevelSerializable})
			// Do a read to simulate how a service would before creating
			foundResource, err := repo.FindResourceByKeys(conflictTx, resource.ReporterResources()[0].ReporterResourceKey)
			assert.NotNil(t, err)
			assert.Nil(t, foundResource)
			assert.NoError(t, repo.Save(conflictTx, resource, bizmodel.OperationTypeCreated, "tx-conflict"))
			// Do NOT commit yet to hold locks

			// Attempt to create the same resource via a separate serializable transaction managed by TM
			err = tm.HandleSerializableTransaction("test_create_resource", db, func(tx *gorm.DB) error {
				foundResource, err := repo.FindResourceByKeys(tx, resource.ReporterResources()[0].ReporterResourceKey)
				assert.NotNil(t, err)
				assert.Nil(t, foundResource)
				return repo.Save(tx, resource, bizmodel.OperationTypeCreated, "tx-create")
			})
			assert.Error(t, err)
			assert.ErrorContains(t, err, "transaction failed")

			// Cleanup
			assert.NoError(t, conflictTx.Commit().Error)
		})
	}
}

func TestSerializableUpdateFails(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			// Fresh instances
			db := setupInMemoryDB(t)
			mc := metricscollector.NewFakeMetricsCollector()
			tm := NewGormTransactionManager(mc, 3)
			repo := NewResourceRepository(db, tm)

			// Create initial resource (committed)
			resource := createTestResourceWithLocalId(t, "serializable-update-conflict")
			assert.NoError(t, repo.Save(db, resource, bizmodel.OperationTypeCreated, "tx-initial"))

			// Prepare an updated version
			key, err := bizmodel.NewReporterResourceKey("serializable-update-conflict", "k8s_cluster", "ocm", "ocm-instance-1")
			assert.NoError(t, err)
			apiHref, _ := bizmodel.NewApiHref("https://api.example.com/updated")
			consoleHref, _ := bizmodel.NewConsoleHref("https://console.example.com/updated")
			repData, _ := bizmodel.NewRepresentation(map[string]interface{}{"name": "updated"})
			comData, _ := bizmodel.NewRepresentation(map[string]interface{}{"workspace_id": "ws-serial"})
			assert.NoError(t, resource.Update(key, apiHref, consoleHref, nil, repData, comData, "transaction-id-serializable-update"))

			// Begin a conflicting serializable transaction and update the same resource
			conflictTx := db.Begin(&sql.TxOptions{Isolation: sql.LevelSerializable})
			// Do a read to simulate how a service would before saving
			foundResource, err := repo.FindResourceByKeys(conflictTx, resource.ReporterResources()[0].ReporterResourceKey)
			assert.Nil(t, err)
			assert.NotNil(t, foundResource)
			assert.NoError(t, repo.Save(conflictTx, resource, bizmodel.OperationTypeUpdated, "tx-conflict"))
			// Do NOT commit yet to hold locks

			// Attempt to update the same resource via TM-managed serializable transaction
			err = tm.HandleSerializableTransaction("test_update_resource", db, func(tx *gorm.DB) error {
				return repo.Save(tx, resource, bizmodel.OperationTypeUpdated, "tx-update")
			})
			assert.Error(t, err)
			assert.ErrorContains(t, err, "transaction failed")

			// Cleanup
			assert.NoError(t, conflictTx.Commit().Error)
		})
	}
}

func setupInMemoryDB(t *testing.T) *gorm.DB {
	db := testutil.NewSQLiteTestDB(t, &gorm.Config{TranslateError: true})

	err := Migrate(db, nil)
	require.NoError(t, err)

	return db
}

// newUniqueTxID creates a unique TransactionID with the given prefix
func newUniqueTxID(prefix string) bizmodel.TransactionId {
	return bizmodel.NewTransactionId(fmt.Sprintf("%s-%s", prefix, uuid.New().String()))
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

	transactionId := newUniqueTxID(fmt.Sprintf("test-transaction-id-basic-%s", localResourceId))

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, transactionId, reporterResourceIdType, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
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

	transactionId := newUniqueTxID(fmt.Sprintf("test-transaction-id-with-type-%s-%s", localResourceId, resourceType))

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceTypeType, reporterTypeType, reporterInstanceIdType, transactionId, reporterResourceIdType, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
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

	transactionId := newUniqueTxID(fmt.Sprintf("test-transaction-id-reporter-only-%s", localResourceId))

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, transactionId, reporterResourceIdType, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
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

	transactionId := newUniqueTxID(fmt.Sprintf("test-transaction-id-common-only-%s", localResourceId))

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, transactionId, reporterResourceIdType, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
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

	transactionId := newUniqueTxID("test-transaction-id-mixed-case-unique")

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, transactionId, reporterResourceIdType, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
	require.NoError(t, err)

	return resource
}

func createTestResourceWithReporter(t *testing.T, localResourceId, reporterType, reporterInstanceId string) bizmodel.Resource {
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

	transactionId := newUniqueTxID(fmt.Sprintf("test-transaction-id-with-reporter-%s-%s", localResourceId, reporterType))

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterTypeType, reporterInstanceIdType, transactionId, reporterResourceIdType, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
	require.NoError(t, err)

	return resource
}

func createContractReporterResourceKey(t *testing.T, localResourceId, resourceType, reporterType, reporterInstanceId string) bizmodel.ReporterResourceKey {
	localResourceIdType, err := bizmodel.NewLocalResourceId(localResourceId)
	require.NoError(t, err)
	resourceTypeType, err := bizmodel.NewResourceType(resourceType)
	require.NoError(t, err)
	reporterTypeType, err := bizmodel.NewReporterType(reporterType)
	require.NoError(t, err)
	reporterInstanceIdType, err := bizmodel.NewReporterInstanceId(reporterInstanceId)
	require.NoError(t, err)

	key, err := bizmodel.NewReporterResourceKey(localResourceIdType, resourceTypeType, reporterTypeType, reporterInstanceIdType)
	require.NoError(t, err)
	return key
}

func TestFindLatestRepresentations(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB { return setupInMemoryDB(t) },
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository { return NewFakeResourceRepository() },
			db:   func() *gorm.DB { return nil },
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			// Helper to get fresh instances
			getFresh := func() (bizmodel.ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm), db
			}

			repo, db := getFresh()

			key, err := bizmodel.NewReporterResourceKey("localResourceId-latest", "host", "hbi", "hbi-instance-1")
			require.NoError(t, err)

			// Set up test data with multiple versions (same for both implementations)
			resource := createTestResourceWithLocalIdAndType(t, "localResourceId-latest", "host")

			// Save initial version (version 0)
			err = repo.Save(db, resource, bizmodel.OperationTypeCreated, "tx-latest-v0")
			require.NoError(t, err)

			// Update to version 1
			updatedCommon1, err := bizmodel.NewRepresentation(map[string]interface{}{
				"workspace_id": "workspace-v1",
				"region":       "us-west-1",
			})
			require.NoError(t, err)
			updatedReporter1, err := bizmodel.NewRepresentation(map[string]interface{}{
				"hostname": "host-v1",
			})
			require.NoError(t, err)

			transactionId1 := bizmodel.NewTransactionId("test-transaction-id-v1")
			err = resource.Update(key, "", "", nil, updatedReporter1, updatedCommon1, transactionId1)
			require.NoError(t, err)
			err = repo.Save(db, resource, bizmodel.OperationTypeUpdated, "tx-latest-v1")
			require.NoError(t, err)

			// Update to version 2 (this should be the latest)
			updatedCommon2, err := bizmodel.NewRepresentation(map[string]interface{}{
				"workspace_id": "workspace-v2-latest",
				"region":       "us-east-1",
				"environment":  "production",
			})
			require.NoError(t, err)
			updatedReporter2, err := bizmodel.NewRepresentation(map[string]interface{}{
				"hostname": "host-v2-latest",
			})
			require.NoError(t, err)

			transactionId2 := bizmodel.NewTransactionId("test-transaction-id-v2")
			err = resource.Update(key, "", "", nil, updatedReporter2, updatedCommon2, transactionId2)
			require.NoError(t, err)
			err = repo.Save(db, resource, bizmodel.OperationTypeUpdated, "tx-latest-v2")
			require.NoError(t, err)

			// Test FindLatestRepresentations
			result, err := repo.FindLatestRepresentations(db, key)
			require.NoError(t, err)

			// Both implementations should return the latest version (version 2)
			assert.Equal(t, "workspace-v2-latest", result.CommonData()["workspace_id"])
			assert.Equal(t, uint(2), *result.CommonVersion())

			// Verify it contains the latest data
			assert.Equal(t, "production", result.CommonData()["environment"])
			assert.Equal(t, "us-east-1", result.CommonData()["region"])
		})
	}
}

func TestFindCurrentAndPreviousVersionedRepresentations(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB { return setupInMemoryDB(t) },
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
			db: func() *gorm.DB { return nil },
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() (bizmodel.ResourceRepository, *gorm.DB) {
				if impl.name == "Fake Repository" {
					return impl.repo(), impl.db()
				}
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				repo := NewResourceRepository(db, tm)
				return repo, db
			}

			t.Run("GetCurrentAndPreviousWorkspaceID extracts workspace IDs correctly", func(t *testing.T) {
				repo, db := getFreshInstances()

				// Create and update a resource to have versioned representations (same for both implementations)
				resource := createTestResourceWithLocalIdAndType(t, "workspace-test-resource", "host")
				err := repo.Save(db, resource, bizmodel.OperationTypeCreated, "tx-ws-test")
				require.NoError(t, err)

				// Update to create version 1
				key, err := bizmodel.NewReporterResourceKey("workspace-test-resource", "host", "hbi", "hbi-instance-1")
				require.NoError(t, err)

				updatedCommon, err := bizmodel.NewRepresentation(map[string]interface{}{"workspace_id": "workspace-v1"})
				require.NoError(t, err)
				updatedReporter, err := bizmodel.NewRepresentation(map[string]interface{}{"hostname": "updated-host"})
				require.NoError(t, err)

				transactionId := newUniqueTxID("test-transaction-id-workspace-unique")
				err = resource.Update(key, "", "", nil, updatedReporter, updatedCommon, transactionId)
				require.NoError(t, err)
				require.NoError(t, repo.Save(db, resource, bizmodel.OperationTypeUpdated, "tx-ws-update"))

				// Get current and previous versions
				version := uint(1)
				cur, prev, err := repo.FindCurrentAndPreviousVersionedRepresentations(db, key, &version, bizmodel.OperationTypeUpdated)
				require.NoError(t, err)

				currentWS, previousWS := GetCurrentAndPreviousWorkspaceID(cur, prev)
				assert.Equal(t, "workspace-v1", currentWS)
				assert.Equal(t, "test-workspace", previousWS) // From initial creation
			})

			t.Run("GetCurrentAndPreviousWorkspaceID handles version 0", func(t *testing.T) {
				repo, db := getFreshInstances()

				key, err := bizmodel.NewReporterResourceKey("test-resource-v0", "host", "hbi", "hbi-instance-1")
				require.NoError(t, err)

				// Create a resource without updates (version 0) - same for both implementations
				resource := createTestResourceWithLocalIdAndType(t, "test-resource-v0", "host")
				err = repo.Save(db, resource, bizmodel.OperationTypeCreated, "tx-v0-test")
				require.NoError(t, err)

				// Get version 0 representations
				version := uint(0)
				cur, prev, err := repo.FindCurrentAndPreviousVersionedRepresentations(db, key, &version, bizmodel.OperationTypeCreated)
				require.NoError(t, err)

				currentWS, previousWS := GetCurrentAndPreviousWorkspaceID(cur, prev)
				assert.Equal(t, "test-workspace", currentWS)
				assert.Equal(t, "", previousWS) // No previous version for version 0
			})

			t.Run("GetCurrentAndPreviousWorkspaceID handles empty representations", func(t *testing.T) {
				// Test the function directly with nil data
				currentWS, previousWS := GetCurrentAndPreviousWorkspaceID(nil, nil)
				assert.Equal(t, "", currentWS)
				assert.Equal(t, "", previousWS)
			})

			t.Run("GetCurrentAndPreviousWorkspaceID handles invalid workspace_id types", func(t *testing.T) {
				// Test the function directly with invalid data
				current, _ := bizmodel.NewRepresentations(
					bizmodel.Representation(map[string]interface{}{"workspace_id": 123}), // non-string
					ptrUint(1),
					nil,
					nil,
				)
				previous, _ := bizmodel.NewRepresentations(
					bizmodel.Representation(map[string]interface{}{"other_field": "value"}), // missing workspace_id
					ptrUint(0),
					nil,
					nil,
				)
				currentWS, previousWS := GetCurrentAndPreviousWorkspaceID(current, previous)
				assert.Equal(t, "", currentWS)
				assert.Equal(t, "", previousWS)
			})
		})
	}
}

func TestHasTransactionIdBeenProcessed(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
			db: func() *gorm.DB {
				return nil // Fake doesn't need real DB
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			testHasTransactionIdBeenProcessed(t, impl.repo(), impl.db())
		})
	}
}

func testHasTransactionIdBeenProcessed(t *testing.T, repo bizmodel.ResourceRepository, db *gorm.DB) {
	t.Run("Empty transaction ID returns false", func(t *testing.T) {
		processed, err := repo.HasTransactionIdBeenProcessed(db, "")
		require.NoError(t, err)
		assert.False(t, processed, "Empty transaction ID should return false")
	})

	t.Run("Non-existent transaction ID returns false", func(t *testing.T) {
		transactionId := "non-existent-transaction-123"

		processed, err := repo.HasTransactionIdBeenProcessed(db, transactionId)
		require.NoError(t, err)
		assert.False(t, processed, "Non-existent transaction ID should return false")
	})

	t.Run("Transaction ID tracking for fake repository", func(t *testing.T) {
		// This test is specific to the fake repository implementation
		if fakeRepo, ok := repo.(*fakeResourceRepository); ok {
			transactionId := "test-transaction-456"

			// Initially should not be processed
			processed, err := fakeRepo.HasTransactionIdBeenProcessed(db, transactionId)
			require.NoError(t, err)
			assert.False(t, processed, "Transaction ID should not be processed initially")

			// Mark as processed
			fakeRepo.markTransactionIdAsProcessed(transactionId)

			// Now should be processed
			processed, err = fakeRepo.HasTransactionIdBeenProcessed(db, transactionId)
			require.NoError(t, err)
			assert.True(t, processed, "Transaction ID should be processed after marking")

			// Different transaction ID should still be false
			differentTransactionId := "different-transaction-789"
			processed, err = fakeRepo.HasTransactionIdBeenProcessed(db, differentTransactionId)
			require.NoError(t, err)
			assert.False(t, processed, "Different transaction ID should not be processed")
		}
	})

	t.Run("Real repository basic functionality", func(t *testing.T) {
		// This test is specific to the real repository implementation
		// We test basic functionality without complex database setup
		if realRepo, ok := repo.(*resourceRepository); ok {
			transactionId := "test-transaction-789"

			// Test that the method doesn't crash and returns false for non-existent transaction
			processed, err := realRepo.HasTransactionIdBeenProcessed(db, transactionId)
			require.NoError(t, err)
			assert.False(t, processed, "Non-existent transaction ID should return false")

			// Test empty transaction ID
			processed, err = realRepo.HasTransactionIdBeenProcessed(db, "")
			require.NoError(t, err)
			assert.False(t, processed, "Empty transaction ID should return false")
		}
	})

	t.Run("Multiple transaction IDs are tracked independently", func(t *testing.T) {
		transactionId1 := "transaction-1"
		transactionId2 := "transaction-2"
		transactionId3 := "transaction-3"

		// Initially none should be processed
		processed1, err := repo.HasTransactionIdBeenProcessed(db, transactionId1)
		require.NoError(t, err)
		assert.False(t, processed1, "Transaction ID 1 should not be processed initially")

		processed2, err := repo.HasTransactionIdBeenProcessed(db, transactionId2)
		require.NoError(t, err)
		assert.False(t, processed2, "Transaction ID 2 should not be processed initially")

		processed3, err := repo.HasTransactionIdBeenProcessed(db, transactionId3)
		require.NoError(t, err)
		assert.False(t, processed3, "Transaction ID 3 should not be processed initially")

		// Mark one as processed (for fake repository)
		if fakeRepo, ok := repo.(*fakeResourceRepository); ok {
			fakeRepo.markTransactionIdAsProcessed(transactionId2)

			// Check all again
			processed1, err = repo.HasTransactionIdBeenProcessed(db, transactionId1)
			require.NoError(t, err)
			assert.False(t, processed1, "Transaction ID 1 should still not be processed")

			processed2, err = repo.HasTransactionIdBeenProcessed(db, transactionId2)
			require.NoError(t, err)
			assert.True(t, processed2, "Transaction ID 2 should now be processed")

			processed3, err = repo.HasTransactionIdBeenProcessed(db, transactionId3)
			require.NoError(t, err)
			assert.False(t, processed3, "Transaction ID 3 should still not be processed")
		}
	})

	t.Run("Concurrent access to transaction ID tracking", func(t *testing.T) {
		// This test is specific to the fake repository implementation
		if fakeRepo, ok := repo.(*fakeResourceRepository); ok {
			transactionId := "concurrent-transaction-test"

			// Test concurrent reads
			done := make(chan bool, 10)
			for i := 0; i < 10; i++ {
				go func() {
					processed, err := fakeRepo.HasTransactionIdBeenProcessed(db, transactionId)
					require.NoError(t, err)
					assert.False(t, processed, "Concurrent read should return false")
					done <- true
				}()
			}

			// Wait for all goroutines to complete
			for i := 0; i < 10; i++ {
				<-done
			}

			// Mark as processed
			fakeRepo.markTransactionIdAsProcessed(transactionId)

			// Test concurrent reads after marking
			for i := 0; i < 10; i++ {
				go func() {
					processed, err := fakeRepo.HasTransactionIdBeenProcessed(db, transactionId)
					require.NoError(t, err)
					assert.True(t, processed, "Concurrent read should return true after marking")
					done <- true
				}()
			}

			// Wait for all goroutines to complete
			for i := 0; i < 10; i++ {
				<-done
			}
		}
	})
}

func TestTransactionIDUniqueConstraint(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
		db   func() *gorm.DB
	}{
		{
			name: "Real Repository with GormTransactionManager",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				mc := metricscollector.NewFakeMetricsCollector()
				tm := NewGormTransactionManager(mc, 3)
				return NewResourceRepository(db, tm)
			},
			db: func() *gorm.DB {
				return setupInMemoryDB(t)
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			testTransactionIDUniqueConstraint(t, impl.repo(), impl.db())
		})
	}
}

func testTransactionIDUniqueConstraint(t *testing.T, repo bizmodel.ResourceRepository, db *gorm.DB) {
	t.Run("should enforce unique TransactionID constraint on duplicate operations", func(t *testing.T) {
		// Create first resource with a specific TransactionID
		duplicateTxID := newUniqueTxID("duplicate-test")
		resource1 := createTestResourceWithLocalId(t, "duplicate-tx-test-1")

		// Update the resource to use our specific TransactionID
		key1 := createContractReporterResourceKey(t, "duplicate-tx-test-1", "k8s_cluster", "ocm", "ocm-instance-1")
		apiHref, _ := bizmodel.NewApiHref("https://api.example.com/duplicate")
		consoleHref, _ := bizmodel.NewConsoleHref("https://console.example.com/duplicate")
		reporterData, _ := bizmodel.NewRepresentation(map[string]interface{}{"duplicate": "test1"})
		commonData, _ := bizmodel.NewRepresentation(map[string]interface{}{"workspace_id": "duplicate-workspace"})

		err := resource1.Update(key1, apiHref, consoleHref, nil, reporterData, commonData, duplicateTxID)
		require.NoError(t, err)

		err = repo.Save(db, resource1, bizmodel.OperationTypeCreated, "tx-duplicate-1")
		require.NoError(t, err, "First save should succeed")

		// Create second resource with the same TransactionID
		resource2 := createTestResourceWithLocalId(t, "duplicate-tx-test-2")
		key2 := createContractReporterResourceKey(t, "duplicate-tx-test-2", "k8s_cluster", "ocm", "ocm-instance-1")

		err = resource2.Update(key2, apiHref, consoleHref, nil, reporterData, commonData, duplicateTxID)
		require.NoError(t, err)

		// This should fail due to unique constraint violation
		err = repo.Save(db, resource2, bizmodel.OperationTypeCreated, "tx-duplicate-2")
		require.Error(t, err, "Second save should fail due to duplicate TransactionID")
		assert.Contains(t, err.Error(), "NON-UNIQUE TRANSACTION ID")
	})

	t.Run("should enforce unique TransactionID constraint on CommonRepresentation", func(t *testing.T) {
		// Create two CommonRepresentations with the same TransactionID
		duplicateTxID := newUniqueTxID("common-duplicate-test")

		commonRep1, err := datamodel.NewCommonRepresentation(
			uuid.New(),
			internal.JsonObject{"workspace_id": "test-workspace-1"},
			1,
			"ocm",
			"ocm-instance-1",
			duplicateTxID.Serialize(),
		)
		require.NoError(t, err)

		commonRep2, err := datamodel.NewCommonRepresentation(
			uuid.New(),
			internal.JsonObject{"workspace_id": "test-workspace-2"},
			1,
			"ocm",
			"ocm-instance-1",
			duplicateTxID.Serialize(),
		)
		require.NoError(t, err)

		// Save first CommonRepresentation
		err = db.Create(&commonRep1).Error
		require.NoError(t, err, "First CommonRepresentation should save successfully")

		// Try to save second CommonRepresentation with same TransactionID
		err = db.Create(&commonRep2).Error
		require.Error(t, err, "Second CommonRepresentation should fail due to duplicate TransactionID")
		assert.Contains(t, err.Error(), "duplicated key not allowed")
	})

	t.Run("should enforce unique TransactionID constraint on ReporterRepresentation", func(t *testing.T) {
		// Create two ReporterRepresentations with the same TransactionID
		duplicateTxID := newUniqueTxID("reporter-duplicate-test")
		reporterResourceID := uuid.New()

		// Create corresponding ReporterResource rows so the foreign key
		// constraint on ReporterRepresentation is satisfied.
		reporterResource1, err := datamodel.NewReporterResource(
			reporterResourceID,
			"local-id-1",
			"ocm",
			"k8s_cluster",
			"instance-1",
			uuid.New(), // resourceID
			"https://api.example.com/resource/1",
			"https://console.example.com/resource/1",
			0,
			0,
			false,
		)
		require.NoError(t, err)

		reporterResourceID2 := uuid.New()
		reporterResource2, err := datamodel.NewReporterResource(
			reporterResourceID2,
			"local-id-2",
			"ocm",
			"k8s_cluster",
			"instance-1",
			uuid.New(), // resourceID
			"https://api.example.com/resource/2",
			"https://console.example.com/resource/2",
			0,
			0,
			false,
		)
		require.NoError(t, err)

		require.NoError(t, db.Create(&reporterResource1).Error)
		require.NoError(t, db.Create(&reporterResource2).Error)

		commonVersion := uint(1)
		reporterRep1, err := datamodel.NewReporterRepresentation(
			internal.JsonObject{"name": "test-resource-1"},
			reporterResourceID,
			1,
			1,
			&commonVersion,
			duplicateTxID.Serialize(),
			false,
			nil,
		)
		require.NoError(t, err)

		reporterRep2, err := datamodel.NewReporterRepresentation(
			internal.JsonObject{"name": "test-resource-2"},
			reporterResourceID2, // Different reporter resource ID
			1,
			1,
			&commonVersion,
			duplicateTxID.Serialize(), // Same TransactionID
			false,
			nil,
		)
		require.NoError(t, err)

		// Save first ReporterRepresentation
		err = db.Create(&reporterRep1).Error
		require.NoError(t, err, "First ReporterRepresentation should save successfully")

		// Try to save second ReporterRepresentation with same TransactionID
		err = db.Create(&reporterRep2).Error
		require.Error(t, err, "Second ReporterRepresentation should fail due to duplicate TransactionID")
		assert.Contains(t, err.Error(), "duplicated key not allowed")
	})

	t.Run("should allow multiple empty TransactionIDs", func(t *testing.T) {
		// Create two CommonRepresentations with empty TransactionIDs
		commonRep1, err := datamodel.NewCommonRepresentation(
			uuid.New(),
			internal.JsonObject{"workspace_id": "test-workspace-1"},
			1,
			"ocm",
			"ocm-instance-1",
			"", // Empty TransactionID
		)
		require.NoError(t, err)

		commonRep2, err := datamodel.NewCommonRepresentation(
			uuid.New(),
			internal.JsonObject{"workspace_id": "test-workspace-2"},
			1,
			"ocm",
			"ocm-instance-1",
			"", // Empty TransactionID
		)
		require.NoError(t, err)

		// Both should save successfully since empty strings are excluded from unique constraint
		err = db.Create(&commonRep1).Error
		require.NoError(t, err, "First CommonRepresentation with empty TransactionID should save successfully")

		err = db.Create(&commonRep2).Error
		require.NoError(t, err, "Second CommonRepresentation with empty TransactionID should save successfully")
	})

	t.Run("should persist and read resources with NULL common_version", func(t *testing.T) {
		// Create a resource without a common representation (common_version should be NULL)
		localResourceID := "resource-without-common-rep"
		resourceId := uuid.New()
		reporterResourceId := uuid.New()

		reporterData := internal.JsonObject{"cluster_id": "test-cluster-123"}

		localResourceIdType, err := bizmodel.NewLocalResourceId(localResourceID)
		require.NoError(t, err)

		resourceType, err := bizmodel.NewResourceType("k8s_cluster")
		require.NoError(t, err)

		reporterType, err := bizmodel.NewReporterType("ocm")
		require.NoError(t, err)

		reporterInstanceId, err := bizmodel.NewReporterInstanceId("test-instance")
		require.NoError(t, err)

		apiHref, err := bizmodel.NewApiHref("/api/resources/test-cluster-123")
		require.NoError(t, err)

		consoleHref, err := bizmodel.NewConsoleHref("/console/resources/test-cluster-123")
		require.NoError(t, err)

		reporterRepresentation := bizmodel.Representation(reporterData)
		emptyCommonRepresentation := bizmodel.Representation(internal.JsonObject{}) // empty common representation

		resourceIdType, err := bizmodel.NewResourceId(resourceId)
		require.NoError(t, err)

		reporterResourceIdType, err := bizmodel.NewReporterResourceId(reporterResourceId)
		require.NoError(t, err)

		txID := newUniqueTxID("null-common-version-test")

		// Create a resource without common representation data (empty map)
		resource, err := bizmodel.NewResource(
			resourceIdType,
			localResourceIdType,
			resourceType,
			reporterType,
			reporterInstanceId,
			txID,
			reporterResourceIdType,
			apiHref,
			consoleHref,
			reporterRepresentation,
			emptyCommonRepresentation, // empty common representation data
			nil,
		)
		require.NoError(t, err)

		// Save the resource
		err = repo.Save(db, resource, biz.OperationTypeCreated, "")
		require.NoError(t, err, "Should save resource without common representation")

		// Read the resource back
		key, err := bizmodel.NewReporterResourceKey(
			localResourceIdType,
			resourceType,
			reporterType,
			reporterInstanceId,
		)
		require.NoError(t, err)

		retrievedResource, err := repo.FindResourceByKeys(db, key)
		require.NoError(t, err, "Should find resource by keys")
		require.NotNil(t, retrievedResource, "Retrieved resource should not be nil")

		// Serialize to check the CommonVersion
		resourceSnapshot, _, _, _, err := retrievedResource.Serialize()
		require.NoError(t, err, "Should serialize resource")

		// Assert that CommonVersion is nil (not an implicit default)
		if resourceSnapshot.CommonVersion != nil {
			t.Errorf("Expected CommonVersion to be nil for resource without common representation, got: %v", *resourceSnapshot.CommonVersion)
		}
	})
}
