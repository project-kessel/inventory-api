package data

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/testutil"
)

func ptrVersion(v uint) *bizmodel.Version {
	ver := bizmodel.NewVersion(v)
	return &ver
}

var emptyTxId = bizmodel.NewTransactionId("")

func newTestGormResourceRepository(db *gorm.DB) bizmodel.ResourceRepository {
	mc := metricscollector.NewFakeMetricsCollector()
	return NewResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: 3,
		OperationName:           "test",
	})
}

func gormResourceTxFromDB(gormTx *gorm.DB) bizmodel.ResourceTx {
	return &gormResourceTx{
		gormTx:          gormTx,
		outboxPublisher: noopOutboxPublisher,
	}
}


func TestResourceRepositoryContract(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			testRepositoryContract(t, impl.repo())
		})
	}
}

func testRepositoryContract(t *testing.T, repo bizmodel.ResourceRepository) {
	t.Run("NextResourceId generates valid UUIDs", func(t *testing.T) {
		var id1 bizmodel.ResourceId
		err := repo.Transact(func(tx bizmodel.ResourceTx) error {
			var nextErr error
			id1, nextErr = tx.NextResourceId()
			return nextErr
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id1.UUID())

		var id2 bizmodel.ResourceId
		err = repo.Transact(func(tx bizmodel.ResourceTx) error {
			var nextErr error
			id2, nextErr = tx.NextResourceId()
			return nextErr
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id2.UUID())
		assert.NotEqual(t, id1.UUID(), id2.UUID())
	})

	t.Run("NextReporterResourceId generates valid UUIDs", func(t *testing.T) {
		var id1 bizmodel.ReporterResourceId
		err := repo.Transact(func(tx bizmodel.ResourceTx) error {
			var nextErr error
			id1, nextErr = tx.NextReporterResourceId()
			return nextErr
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id1.UUID())

		var id2 bizmodel.ReporterResourceId
		err = repo.Transact(func(tx bizmodel.ResourceTx) error {
			var nextErr error
			id2, nextErr = tx.NextReporterResourceId()
			return nextErr
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id2.UUID())
		assert.NotEqual(t, id1.UUID(), id2.UUID())
	})

	t.Run("Save and FindResourceByKeys basic workflow", func(t *testing.T) {
		resource := createTestResourceWithLocalId(t, "contract-test-1")
		err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("contract-tx-1")) })
		require.NoError(t, err, "Save should succeed")

		key := createContractReporterResourceKey(t, "contract-test-1", "k8s_cluster", "ocm", "ocm-instance-1")

		var foundResource *bizmodel.Resource
		err = repo.Transact(func(tx bizmodel.ResourceTx) error {
			var findErr error
			foundResource, findErr = tx.FindResourceByKeys(key)
			return findErr
		})
		require.NoError(t, err, "Find should succeed")
		require.NotNil(t, foundResource, "Found resource should not be nil")
		assert.Len(t, foundResource.ReporterResources(), 1, "Should have one reporter resource")
	})

	t.Run("FindResourceByKeys returns ErrResourceNotFound for non-existent", func(t *testing.T) {
		key := createContractReporterResourceKey(t, "non-existent-contract", "k8s_cluster", "ocm", "ocm-instance-1")

		var foundResource *bizmodel.Resource
		err := repo.Transact(func(tx bizmodel.ResourceTx) error {
			var findErr error
			foundResource, findErr = tx.FindResourceByKeys(key)
			return findErr
		})
		require.ErrorIs(t, err, bizmodel.ErrResourceNotFound, "Should return ErrResourceNotFound")
		assert.Nil(t, foundResource, "Found resource should be nil")
	})

	t.Run("Save-Update-Save workflow", func(t *testing.T) {
		// Create initial resource
		resource := createTestResourceWithLocalId(t, "contract-update-test")
		err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("contract-tx-create")) })
		require.NoError(t, err, "Initial save should succeed")

		// Find and update
		key := createContractReporterResourceKey(t, "contract-update-test", "k8s_cluster", "ocm", "ocm-instance-1")

		var foundResource *bizmodel.Resource
		err = repo.Transact(func(tx bizmodel.ResourceTx) error {
			var findErr error
			foundResource, findErr = tx.FindResourceByKeys(key)
			return findErr
		})
		require.NoError(t, err, "Find should succeed")
		require.NotNil(t, foundResource)

		// Update the resource
		apiHref, _ := bizmodel.NewApiHref("https://api.example.com/updated")
		consoleHref, _ := bizmodel.NewConsoleHref("https://console.example.com/updated")
		reporterData, _ := bizmodel.NewRepresentation(map[string]interface{}{"updated": true})
		commonData, _ := bizmodel.NewRepresentation(map[string]interface{}{"workspace_id": "updated-workspace"})
		transactionId := newUniqueTxID("test-transaction-id-update-contract")

		err = foundResource.Update(key, apiHref, &consoleHref, nil, &reporterData, &commonData, transactionId)
		require.NoError(t, err, "Update should succeed")

		// Save updated resource
		err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundResource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("contract-tx-update")) })
		require.NoError(t, err, "Updated save should succeed")

		// Verify update persisted
		var updatedResource *bizmodel.Resource
		err = repo.Transact(func(tx bizmodel.ResourceTx) error {
			var findErr error
			updatedResource, findErr = tx.FindResourceByKeys(key)
			return findErr
		})
		require.NoError(t, err, "Find updated resource should succeed")
		require.NotNil(t, updatedResource)
	})

	t.Run("Save-Delete workflow", func(t *testing.T) {
		// Create resource
		resource := createTestResourceWithLocalId(t, "contract-delete-test")
		err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("contract-tx-create")) })
		require.NoError(t, err, "Initial save should succeed")

		// Find and delete
		key := createContractReporterResourceKey(t, "contract-delete-test", "k8s_cluster", "ocm", "ocm-instance-1")

		var foundResource *bizmodel.Resource
		err = repo.Transact(func(tx bizmodel.ResourceTx) error {
			var findErr error
			foundResource, findErr = tx.FindResourceByKeys(key)
			return findErr
		})
		require.NoError(t, err, "Find should succeed")
		require.NotNil(t, foundResource)

		// Delete the resource
		err = foundResource.Delete(key)
		require.NoError(t, err, "Delete should succeed")

		// Save deleted resource
		err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundResource, bizmodel.OperationTypeDeleted, bizmodel.NewTransactionId("contract-tx-delete")) })
		require.NoError(t, err, "Delete save should succeed")

		// Verify deletion behavior is consistent
		var deletedResource *bizmodel.Resource
		err = repo.Transact(func(tx bizmodel.ResourceTx) error {
			var findErr error
			deletedResource, findErr = tx.FindResourceByKeys(key)
			return findErr
		})
		if errors.Is(err, bizmodel.ErrResourceNotFound) {
			assert.Nil(t, deletedResource, "Deleted resource should not be found with tombstone filter")
		} else {
			require.NoError(t, err, "Find should succeed if tombstone filter removed")
			require.NotNil(t, deletedResource, "Should find tombstoned resource")
		}
	})

	t.Run("Unique constraint enforcement", func(t *testing.T) {
		// Create first resource
		resource1 := createTestResourceWithLocalId(t, "contract-unique-test")
		err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource1, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("contract-tx-1")) })
		require.NoError(t, err, "First save should succeed")

		// Try to create second resource with same composite key
		resource2 := createTestResourceWithLocalId(t, "contract-unique-test")
		err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource2, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("contract-tx-2")) })
		require.Error(t, err, "Second save with duplicate key should fail")

		// Error should indicate constraint violation
		errorMsg := err.Error()
		constraintViolation := strings.Contains(errorMsg, "duplicate") || strings.Contains(errorMsg, bizmodel.ReasonNonUniqueTransactionID)
		assert.True(t, constraintViolation, "Error should mention constraint violation, got: %s", errorMsg)
	})

	t.Run("Case insensitive key matching for non ID fields", func(t *testing.T) {
		// Create resource with mixed case
		resource := createTestResourceWithReporter(t, "Contract-Case-Test", "OCM", "Instance-1")
		err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("contract-case-tx")) })
		require.NoError(t, err, "Save should succeed")

		// Find with different casing
		key := createContractReporterResourceKey(t, "Contract-Case-Test", "k8s_cluster", "ocm", "Instance-1")

		var foundResource *bizmodel.Resource
		err = repo.Transact(func(tx bizmodel.ResourceTx) error {
			var findErr error
			foundResource, findErr = tx.FindResourceByKeys(key)
			return findErr
		})
		require.NoError(t, err, "Case insensitive find should succeed")
		require.NotNil(t, foundResource)
	})

	t.Run("Transaction handling", func(t *testing.T) {
		resource := createTestResourceWithLocalId(t, "contract-tx-test")
		err := repo.Transact(func(tx bizmodel.ResourceTx) error {
			return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("contract-tx"))
		})
		require.NoError(t, err, "Save within Transact should succeed")

		key := createContractReporterResourceKey(t, "contract-tx-test", "k8s_cluster", "ocm", "ocm-instance-1")

		var foundResource *bizmodel.Resource
		err = repo.Transact(func(tx bizmodel.ResourceTx) error {
			var findErr error
			foundResource, findErr = tx.FindResourceByKeys(key)
			return findErr
		})
		require.NoError(t, err, "Find within Transact should succeed")
		require.NotNil(t, foundResource)
	})

}

func TestFindResourceByKeys(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			// Helper function to get fresh instances for each test
			getFreshInstances := func() bizmodel.ResourceRepository {
				if impl.name == "Fake Repository" {
					return impl.repo()
				}
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			}

			t.Run("Save and FindResourceByKeys workflow", func(t *testing.T) {
				repo := getFreshInstances()

				resource := createTestResource(t)
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-123")) })
				require.NoError(t, err)

				key, err := bizmodel.NewReporterResourceKey(
					"test-resource-123",
					"k8s_cluster",
					"ocm",
					"ocm-instance-1",
				)
				require.NoError(t, err)

				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.NoError(t, err)
				require.NotNil(t, foundResource)
				assert.Len(t, foundResource.ReporterResources(), 1, "should have reporter resources")
			})

			t.Run("FindResourceByKeys returns ErrResourceNotFound for non-existent resource", func(t *testing.T) {
				repo := getFreshInstances()

				key, err := bizmodel.NewReporterResourceKey(
					"non-existent",
					"k8s_cluster",
					"ocm",
					"ocm-instance-1",
				)
				require.NoError(t, err)

				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.ErrorIs(t, err, bizmodel.ErrResourceNotFound)
				assert.Nil(t, foundResource)
			})

			t.Run("FindResourceByKeys with different keys returns different resources", func(t *testing.T) {
				resource1 := createTestResourceWithLocalId(t, "resource-1")
				resource2 := createTestResourceWithLocalId(t, "resource-2")

				repo := getFreshInstances()

				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource1, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-1")) })
				require.NoError(t, err)
				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource2, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-2")) })
				require.NoError(t, err)

				key1, err := bizmodel.NewReporterResourceKey("resource-1", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)
				key2, err := bizmodel.NewReporterResourceKey("resource-2", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				var found1 *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					found1, findErr = tx.FindResourceByKeys(key1)
					return findErr
				})
				require.NoError(t, err)
				require.NotNil(t, found1)

				var found2 *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					found2, findErr = tx.FindResourceByKeys(key2)
					return findErr
				})
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
				repo := getFreshInstances()

				resource := createTestResource(t)
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-1")) })
				require.NoError(t, err)

				key, err := bizmodel.NewReporterResourceKey(
					"test-resource-123",
					"k8s_cluster",
					"ocm",
					"ocm-instance-1",
				)
				require.NoError(t, err)

				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.NoError(t, err)
				require.NotNil(t, foundResource)
				assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
			})

			t.Run("FindResourceByKeys with nil transaction returns ErrResourceNotFound for non-existent resource", func(t *testing.T) {
				repo := getFreshInstances()

				key, err := bizmodel.NewReporterResourceKey(
					"non-existent-nil-tx",
					"k8s_cluster",
					"ocm",
					"ocm-instance-1",
				)
				require.NoError(t, err)

				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.ErrorIs(t, err, bizmodel.ErrResourceNotFound)
				assert.Nil(t, foundResource)
			})

			t.Run("FindResourceByKeys works when reporterInstanceId is not provided in search key", func(t *testing.T) {
				repo := getFreshInstances()

				resource := createTestResourceWithLocalId(t, "test-resource-no-instance-lookup")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-no-instance")) })
				require.NoError(t, err)

				key, err := bizmodel.NewReporterResourceKey(
					"test-resource-no-instance-lookup",
					"k8s_cluster",
					"ocm",
					"",
				)
				require.NoError(t, err)

				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.NoError(t, err)
				require.NotNil(t, foundResource)
				assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
			})

			// Case-insensitive tests
			t.Run("Case-insensitive matching", func(t *testing.T) {
				repo := getFreshInstances()

				// Create a resource with mixed case values
				resource := createTestResourceWithMixedCase(t)
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-case")) })
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

						var foundResource *bizmodel.Resource
						err = repo.Transact(func(tx bizmodel.ResourceTx) error {
							var findErr error
							foundResource, findErr = tx.FindResourceByKeys(key)
							return findErr
						})
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
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() bizmodel.ResourceRepository {
				if impl.name == "Fake Repository" {
					return impl.repo()
				}
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			}

			repo := getFreshInstances()

			resource := createTestResourceWithLocalId(t, "tombstoned-resource")
			err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-tombstone")) })
			require.NoError(t, err)

			key, err := bizmodel.NewReporterResourceKey(
				"tombstoned-resource",
				"k8s_cluster",
				"ocm",
				"ocm-instance-1",
			)
			require.NoError(t, err)

			var foundResource *bizmodel.Resource
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var findErr error
				foundResource, findErr = tx.FindResourceByKeys(key)
				return findErr
			})
			require.NoError(t, err)
			require.NotNil(t, foundResource)

			err = foundResource.Delete(key)
			require.NoError(t, err)

			err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundResource, bizmodel.OperationTypeDeleted, bizmodel.NewTransactionId("test-tx-delete")) })
			require.NoError(t, err)

			// With tombstone filter removed, we should be able to find the tombstoned resource
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var findErr error
				foundResource, findErr = tx.FindResourceByKeys(key)
				return findErr
			})
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
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() bizmodel.ResourceRepository {
				if impl.name == "Fake Repository" {
					return impl.repo()
				}
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			}

			t.Run("should reject duplicate composite key", func(t *testing.T) {
				repo := getFreshInstances()

				// Create first resource
				resource1 := createTestResourceWithLocalId(t, "duplicate-key-test")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource1, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-1")) })
				require.NoError(t, err, "First save should succeed")

				// Create second resource with same composite key components
				// (same LocalResourceID, ReporterType, ResourceType, ReporterInstanceID, RepresentationVersion=0, Generation=0)
				resource2 := createTestResourceWithLocalId(t, "duplicate-key-test") // Same local ID
				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource2, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-2")) })

				// Both implementations should reject this duplicate
				require.Error(t, err, "Second save with duplicate composite key should fail")

				// Error should indicate a constraint violation
				errorMsg := err.Error()
				// Both "duplicate" (fake repo) and ReasonNonUniqueTransactionID (real DB) are acceptable
				constraintViolation := strings.Contains(errorMsg, "duplicate") || strings.Contains(errorMsg, bizmodel.ReasonNonUniqueTransactionID)
				assert.True(t, constraintViolation, "Error should mention constraint violation, got: %s", errorMsg)
			})

			t.Run("should allow same key with different versions", func(t *testing.T) {
				repo := getFreshInstances()

				// Create and save initial resource
				resource := createTestResourceWithLocalId(t, "version-test-resource")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-create")) })
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

				err = resource.Update(key, apiHref, &consoleHref, nil, &reporterData, &commonData, transactionId)
				require.NoError(t, err, "Update should succeed")

				// Save the updated resource (different version/generation should be allowed)
				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("test-tx-update")) })
				require.NoError(t, err, "Save with different version should succeed")
			})

			t.Run("should allow same key components with different resource types", func(t *testing.T) {
				repo := getFreshInstances()

				// Create first resource with k8s_cluster type
				resource1 := createTestResourceWithLocalIdAndType(t, "multi-type-test", "k8s_cluster")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource1, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-1")) })
				require.NoError(t, err, "First save should succeed")

				// Create second resource with same local ID but different resource type
				resource2 := createTestResourceWithLocalIdAndType(t, "multi-type-test", "host")
				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource2, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-2")) })
				require.NoError(t, err, "Save with different resource type should succeed")
			})

			t.Run("should allow same key components with different reporter types", func(t *testing.T) {
				repo := getFreshInstances()

				// Create resource with OCM reporter
				resource1 := createTestResourceWithReporter(t, "reporter-test", "ocm", "ocm-instance-1")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource1, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-1")) })
				require.NoError(t, err, "First save should succeed")

				// Create resource with same local ID but different reporter type
				resource2 := createTestResourceWithReporter(t, "reporter-test", "hbi", "hbi-instance-1")
				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource2, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-2")) })
				require.NoError(t, err, "Save with different reporter type should succeed")
			})

			t.Run("should allow same key components with different reporter instances", func(t *testing.T) {
				repo := getFreshInstances()

				// Create resource with instance-1
				resource1 := createTestResourceWithReporter(t, "instance-test", "ocm", "ocm-instance-1")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource1, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-1")) })
				require.NoError(t, err, "First save should succeed")

				// Create resource with same components but different reporter instance
				resource2 := createTestResourceWithReporter(t, "instance-test", "ocm", "ocm-instance-2")
				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource2, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-2")) })
				require.NoError(t, err, "Save with different reporter instance should succeed")
			})
		})
	}
}

func TestResourceRepository_IdempotentOperations(t *testing.T) {
	implementations := []struct {
		name string
		repo func() bizmodel.ResourceRepository
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() bizmodel.ResourceRepository {
				if impl.name == "Fake Repository" {
					return impl.repo()
				}
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			}

			t.Run("report -> delete -> resubmit same delete", func(t *testing.T) {
				repo := getFreshInstances()

				localResourceId := "repo-idempotent-delete-test"

				// 1. REPORT: Create initial resource
				resource := createTestResourceWithLocalId(t, localResourceId)
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("repo-create-1")) })
				require.NoError(t, err, "Initial save should succeed")

				key := createContractReporterResourceKey(t, localResourceId, "k8s_cluster", "ocm", "ocm-instance-1")

				// Verify initial state
				var afterCreate *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					afterCreate, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.NoError(t, err, "Should find resource after creation")
				require.NotNil(t, afterCreate)
				initialState := afterCreate.ReporterResources()[0].Serialize()
				assert.Equal(t, uint(0), initialState.RepresentationVersion, "Initial representationVersion should be 0")
				assert.Equal(t, uint(0), initialState.Generation, "Initial generation should be 0")
				assert.False(t, initialState.Tombstone, "Initial tombstone should be false")

				// 2. DELETE: Delete the resource
				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.NoError(t, err, "Should find resource for delete")
				require.NotNil(t, foundResource)

				err = foundResource.Delete(key)
				require.NoError(t, err, "Delete operation should succeed")

				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundResource, bizmodel.OperationTypeDeleted, bizmodel.NewTransactionId("repo-delete-1")) })
				require.NoError(t, err, "Delete save should succeed")

				// Verify delete state
				var afterDelete1 *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					afterDelete1, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				if errors.Is(err, bizmodel.ErrResourceNotFound) {
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
				var foundResource2 *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource2, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				if errors.Is(err, bizmodel.ErrResourceNotFound) {
					// With tombstone filter, we expect this behavior
					t.Log("Cannot resubmit delete - resource not found due to tombstone filter (expected)")
				} else {
					require.NoError(t, err, "Should find resource for duplicate delete")
					require.NotNil(t, foundResource2)

					err = foundResource2.Delete(key)
					require.NoError(t, err, "Duplicate delete operation should succeed")

					err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundResource2, bizmodel.OperationTypeDeleted, bizmodel.NewTransactionId("repo-delete-2")) })
					require.NoError(t, err, "Duplicate delete save should succeed")

					// Verify state after duplicate delete
					var afterDelete2 *bizmodel.Resource
					err = repo.Transact(func(tx bizmodel.ResourceTx) error {
						var findErr error
						afterDelete2, findErr = tx.FindResourceByKeys(key)
						return findErr
					})
					if !errors.Is(err, bizmodel.ErrResourceNotFound) {
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
				repo := getFreshInstances()

				localResourceId := "repo-idempotent-full-test"

				// 1. REPORT: Create initial resource
				resource1 := createTestResourceWithLocalId(t, localResourceId)
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource1, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("repo-create-1")) })
				require.NoError(t, err, "Initial save should succeed")

				key := createContractReporterResourceKey(t, localResourceId, "k8s_cluster", "ocm", "ocm-instance-1")

				// 2. RESUBMIT SAME REPORT: Should succeed and increment version
				var foundResource1 *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource1, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.NoError(t, err, "Should find resource for update")
				require.NotNil(t, foundResource1)

				apiHref, _ := bizmodel.NewApiHref("https://api.example.com/duplicate")
				consoleHref, _ := bizmodel.NewConsoleHref("https://console.example.com/duplicate")
				reporterData, _ := bizmodel.NewRepresentation(map[string]interface{}{"duplicate": "report"})
				commonData, _ := bizmodel.NewRepresentation(map[string]interface{}{"workspace_id": "duplicate-workspace"})
				transactionId := newUniqueTxID("test-transaction-id-duplicate-idempotent")

				err = foundResource1.Update(key, apiHref, &consoleHref, nil, &reporterData, &commonData, transactionId)
				require.NoError(t, err, "Update should succeed")

				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundResource1, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("repo-update-1")) })
				require.NoError(t, err, "Duplicate report save should succeed")

				// 3. DELETE: Delete the resource
				var foundResource2 *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource2, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.NoError(t, err, "Should find resource for delete")
				require.NotNil(t, foundResource2)

				err = foundResource2.Delete(key)
				require.NoError(t, err, "Delete operation should succeed")

				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundResource2, bizmodel.OperationTypeDeleted, bizmodel.NewTransactionId("repo-delete-1")) })
				require.NoError(t, err, "Delete save should succeed")

				// 4. RESUBMIT SAME DELETE: Should succeed
				var foundResource3 *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource3, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				if errors.Is(err, bizmodel.ErrResourceNotFound) {
					// With tombstone filter, we expect this behavior
					t.Log("Cannot resubmit delete - resource not found due to tombstone filter (expected)")
				} else {
					require.NoError(t, err, "Should find resource for duplicate delete")
					require.NotNil(t, foundResource3)

					err = foundResource3.Delete(key)
					require.NoError(t, err, "Duplicate delete operation should succeed")

					err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundResource3, bizmodel.OperationTypeDeleted, bizmodel.NewTransactionId("repo-delete-2")) })
					require.NoError(t, err, "Duplicate delete save should succeed")
				}
			})

			t.Run("complex idempotency: multiple report and delete cycles", func(t *testing.T) {
				repo := getFreshInstances()

				localResourceId := "repo-complex-idempotent-test"
				key := createContractReporterResourceKey(t, localResourceId, "k8s_cluster", "ocm", "ocm-instance-1")

				// Multiple cycles of report -> delete to test robustness
				for cycle := 0; cycle < 3; cycle++ {
					t.Logf("Cycle %d: Report and Delete", cycle)

					// Report: Check if resource exists, create or update accordingly
					var foundResource *bizmodel.Resource
					err := repo.Transact(func(tx bizmodel.ResourceTx) error {
						var findErr error
						foundResource, findErr = tx.FindResourceByKeys(key)
						return findErr
					})

					if errors.Is(err, bizmodel.ErrResourceNotFound) {
						// Resource doesn't exist - create new one
						t.Logf("Cycle %d: Creating new resource", cycle)
						resource := createTestResourceWithLocalId(t, localResourceId)
						err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId(fmt.Sprintf("repo-cycle-%d-create", cycle))) })
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

						err = foundResource.Update(key, apiHref, &consoleHref, nil, &reporterData, &commonData, transactionId)
						require.NoError(t, err, "Update should succeed in cycle %d", cycle)

						err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundResource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId(fmt.Sprintf("repo-cycle-%d-update", cycle))) })
						require.NoError(t, err, "Update save should succeed in cycle %d", cycle)
					}

					// Verify current state after report/update
					var currentResource *bizmodel.Resource
					err = repo.Transact(func(tx bizmodel.ResourceTx) error {
						var findErr error
						currentResource, findErr = tx.FindResourceByKeys(key)
						return findErr
					})
					require.NoError(t, err, "Should find resource after report/update in cycle %d", cycle)
					require.NotNil(t, currentResource)
					currentState := currentResource.ReporterResources()[0].Serialize()
					t.Logf("Cycle %d after report: Generation=%d, RepVersion=%d, Tombstone=%t",
						cycle, currentState.Generation, currentState.RepresentationVersion, currentState.Tombstone)

					// Delete
					err = currentResource.Delete(key)
					require.NoError(t, err, "Delete should succeed in cycle %d", cycle)

					err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*currentResource, bizmodel.OperationTypeDeleted, bizmodel.NewTransactionId(fmt.Sprintf("repo-cycle-%d-delete", cycle))) })
					require.NoError(t, err, "Delete save should succeed in cycle %d", cycle)

					// Verify state after delete
					var deletedResource *bizmodel.Resource
					err = repo.Transact(func(tx bizmodel.ResourceTx) error {
						var findErr error
						deletedResource, findErr = tx.FindResourceByKeys(key)
						return findErr
					})
					if errors.Is(err, bizmodel.ErrResourceNotFound) {
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
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			// Helper function to get fresh instances for each test
			getFreshInstances := func() bizmodel.ResourceRepository {
				if impl.name == "Fake Repository" {
					return impl.repo()
				}
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			}

			t.Run("Save handles duplicate calls gracefully", func(t *testing.T) {
				repo := getFreshInstances()

				resource := createTestResourceWithLocalId(t, "update-test")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-1")) })
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

				err = resource.Update(key, apiHref, &consoleHref, nil, &updatedReporterData, &updatedCommonData, updatedTransactionId)
				require.NoError(t, err)

				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("test-tx-2")) })
				require.NoError(t, err)

				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.NoError(t, err)
				require.NotNil(t, foundResource)
				assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
			})

			t.Run("Save creates new resource successfully", func(t *testing.T) {
				repo := getFreshInstances()

				resource := createTestResourceWithLocalId(t, "save-new-test")

				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-save")) })
				require.NoError(t, err)

				key, err := bizmodel.NewReporterResourceKey("save-new-test", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.NoError(t, err)
				require.NotNil(t, foundResource)
				assert.Len(t, foundResource.ReporterResources(), 1, "should have one reporter resource")
			})

			t.Run("Save skips representations with zero value primary keys", func(t *testing.T) {
				if impl.name == "Fake Repository" {
					t.Skip("This test is specific to real repository database operations")
				}

				repo := getFreshInstances()

				resource := createTestResourceWithLocalId(t, "zero-pk-test")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("test-tx-zero-pk")) })
				require.NoError(t, err, "Save should succeed and skip representations with zero value primary keys")

				key, err := bizmodel.NewReporterResourceKey("zero-pk-test", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
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
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() bizmodel.ResourceRepository {
				if impl.name == "Fake Repository" {
					return impl.repo()
				}
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			}

			repo := getFreshInstances()

			// Create 2 hosts
			host1 := createTestResourceWithLocalIdAndType(t, "host-1", "host")
			host2 := createTestResourceWithLocalIdAndType(t, "host-2", "host")

			err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(host1, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-create-host1")) })
			require.NoError(t, err, "Should create host1")

			err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(host2, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-create-host2")) })
			require.NoError(t, err, "Should create host2")

			// Verify both hosts can be found
			key1, err := bizmodel.NewReporterResourceKey("host-1", "host", "hbi", "hbi-instance-1")
			require.NoError(t, err)
			key2, err := bizmodel.NewReporterResourceKey("host-2", "host", "hbi", "hbi-instance-1")
			require.NoError(t, err)

			var foundHost1 *bizmodel.Resource
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var findErr error
				foundHost1, findErr = tx.FindResourceByKeys(key1)
				return findErr
			})
			require.NoError(t, err, "Should find host1 after creation")
			require.NotNil(t, foundHost1)

			var foundHost2 *bizmodel.Resource
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var findErr error
				foundHost2, findErr = tx.FindResourceByKeys(key2)
				return findErr
			})
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

			err = foundHost1.Update(key1, apiHref, &consoleHref, nil, &updatedReporterData, &updatedCommonData, updatedTransactionId1)
			require.NoError(t, err, "Should update host1")

			err = foundHost2.Update(key2, apiHref, &consoleHref, nil, &updatedReporterData, &updatedCommonData, updatedTransactionId2)
			require.NoError(t, err, "Should update host2")

			err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundHost1, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("tx-update-host1")) })
			require.NoError(t, err, "Should save updated host1")

			err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundHost2, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("tx-update-host2")) })
			require.NoError(t, err, "Should save updated host2")

			// Verify both updated hosts can still be found
			var updatedHost1 *bizmodel.Resource
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var findErr error
				updatedHost1, findErr = tx.FindResourceByKeys(key1)
				return findErr
			})
			require.NoError(t, err, "Should find host1 after update")
			require.NotNil(t, updatedHost1)

			var updatedHost2 *bizmodel.Resource
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var findErr error
				updatedHost2, findErr = tx.FindResourceByKeys(key2)
				return findErr
			})
			require.NoError(t, err, "Should find host2 after update")
			require.NotNil(t, updatedHost2)

			// Delete both hosts
			err = updatedHost1.Delete(key1)
			require.NoError(t, err, "Should delete host1")

			err = updatedHost2.Delete(key2)
			require.NoError(t, err, "Should delete host2")

			err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*updatedHost1, bizmodel.OperationTypeDeleted, bizmodel.NewTransactionId("tx-delete-host1")) })
			require.NoError(t, err, "Should save deleted host1")

			err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*updatedHost2, bizmodel.OperationTypeDeleted, bizmodel.NewTransactionId("tx-delete-host2")) })
			require.NoError(t, err, "Should save deleted host2")

			// Verify both hosts can be found (tombstoned) with tombstone filter removed
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var findErr error
				foundHost1, findErr = tx.FindResourceByKeys(key1)
				return findErr
			})
			require.NoError(t, err, "Should find tombstoned host1")
			require.NotNil(t, foundHost1)
			assert.True(t, foundHost1.ReporterResources()[0].Serialize().Tombstone, "Host1 should be tombstoned")

			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var findErr error
				foundHost2, findErr = tx.FindResourceByKeys(key2)
				return findErr
			})
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
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() bizmodel.ResourceRepository {
				if impl.name == "Fake Repository" {
					return impl.repo()
				}
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			}

			t.Run("Report resource with just reporter data and no common data", func(t *testing.T) {
				repo := getFreshInstances()

				resource := createTestResourceWithReporterDataOnly(t, "reporter-only-resource")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-reporter-only")) })
				require.NoError(t, err, "Should save resource with only reporter data")

				key, err := bizmodel.NewReporterResourceKey("reporter-only-resource", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.NoError(t, err, "Should find resource with only reporter data")
				require.NotNil(t, foundResource)
			})

			t.Run("Report resource with no reporter data but has common data", func(t *testing.T) {
				repo := getFreshInstances()

				resource := createTestResourceWithCommonDataOnly(t, "common-only-resource")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-common-only")) })
				require.NoError(t, err, "Should save resource with only common data")

				key, err := bizmodel.NewReporterResourceKey("common-only-resource", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
				require.NoError(t, err, "Should find resource with only common data")
				require.NotNil(t, foundResource)
			})

			t.Run("Report resource with both data, then just reporter data, then just common data", func(t *testing.T) {
				repo := getFreshInstances()

				// 1. Report with both reporter and common data
				resourceBoth := createTestResourceWithLocalId(t, "progressive-resource")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resourceBoth, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-both")) })
				require.NoError(t, err, "Should save resource with both data types")

				key, err := bizmodel.NewReporterResourceKey("progressive-resource", "k8s_cluster", "ocm", "ocm-instance-1")
				require.NoError(t, err)

				var foundResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
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

				err = foundResource.Update(key, apiHref, &consoleHref, nil, &reporterOnlyData, &emptyCommonData, updatedTransactionId1)
				require.NoError(t, err, "Should update with reporter data only")

				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundResource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("tx-reporter-update")) })
				require.NoError(t, err, "Should save resource with reporter-only update")

				// 3. Update with just common data
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					foundResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
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

				err = foundResource.Update(key, apiHref, &consoleHref, nil, &emptyReporterData, &commonOnlyData, updatedTransactionId2)
				require.NoError(t, err, "Should update with common data only")

				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*foundResource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("tx-common-update")) })
				require.NoError(t, err, "Should save resource with common-only update")

				// Verify final resource can still be found
				var finalResource *bizmodel.Resource
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					finalResource, findErr = tx.FindResourceByKeys(key)
					return findErr
				})
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
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			// Fresh instances
			db := setupInMemoryDB(t)
			repo := newTestGormResourceRepository(db)

			resource := createTestResourceWithLocalId(t, "serializable-create-conflict")

			// Begin a conflicting serializable transaction and create the same resource
			conflictTx := db.Begin(&sql.TxOptions{Isolation: sql.LevelSerializable})
			conflictRtx := gormResourceTxFromDB(conflictTx)
			// Do a read to simulate how a service would before creating
			foundResource, err := conflictRtx.FindResourceByKeys(resource.ReporterResources()[0].ReporterResourceKey)
			assert.NotNil(t, err)
			assert.Nil(t, foundResource)
			assert.NoError(t, conflictRtx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-conflict")))
			// Do NOT commit yet to hold locks

			// Attempt to create the same resource via a separate serializable transaction managed by TM
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				foundResource, findErr := tx.FindResourceByKeys(resource.ReporterResources()[0].ReporterResourceKey)
				assert.NotNil(t, findErr)
				assert.Nil(t, foundResource)
				return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-create"))
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
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			// Fresh instances
			db := setupInMemoryDB(t)
			repo := newTestGormResourceRepository(db)

			// Create initial resource (committed)
			resource := createTestResourceWithLocalId(t, "serializable-update-conflict")
			assert.NoError(t, repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-initial")) }))

			// Prepare an updated version
			key, err := bizmodel.NewReporterResourceKey("serializable-update-conflict", "k8s_cluster", "ocm", "ocm-instance-1")
			assert.NoError(t, err)
			apiHref, _ := bizmodel.NewApiHref("https://api.example.com/updated")
			consoleHref, _ := bizmodel.NewConsoleHref("https://console.example.com/updated")
			repData, _ := bizmodel.NewRepresentation(map[string]interface{}{"name": "updated"})
			comData, _ := bizmodel.NewRepresentation(map[string]interface{}{"workspace_id": "ws-serial"})
			txId := bizmodel.TransactionId("transaction-id-serializable-update")
			assert.NoError(t, resource.Update(key, apiHref, &consoleHref, nil, &repData, &comData, txId))

			// Begin a conflicting serializable transaction and update the same resource
			conflictTx := db.Begin(&sql.TxOptions{Isolation: sql.LevelSerializable})
			conflictRtx := gormResourceTxFromDB(conflictTx)
			// Do a read to simulate how a service would before saving
			foundResource, err := conflictRtx.FindResourceByKeys(resource.ReporterResources()[0].ReporterResourceKey)
			assert.Nil(t, err)
			assert.NotNil(t, foundResource)
			assert.NoError(t, conflictRtx.Save(resource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("tx-conflict")))
			// Do NOT commit yet to hold locks

			// Attempt to update the same resource via TM-managed serializable transaction
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				return tx.Save(resource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("tx-update"))
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

// noopOutboxPublisher is a no-op OutboxPublisher for use with SQLite in tests
func noopOutboxPublisher(_ *gorm.DB, _ *model_legacy.OutboxEvent) error { return nil }

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

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, transactionId, reporterResourceIdType, apiHref, &consoleHref, &reporterRepresentation, &commonRepresentation, nil)
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

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceTypeType, reporterTypeType, reporterInstanceIdType, transactionId, reporterResourceIdType, apiHref, &consoleHref, &reporterRepresentation, &commonRepresentation, nil)
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

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, transactionId, reporterResourceIdType, apiHref, &consoleHref, &reporterRepresentation, &commonRepresentation, nil)
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

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, transactionId, reporterResourceIdType, apiHref, &consoleHref, &reporterRepresentation, &commonRepresentation, nil)
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

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterType, reporterInstanceId, transactionId, reporterResourceIdType, apiHref, &consoleHref, &reporterRepresentation, &commonRepresentation, nil)
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

	resource, err := bizmodel.NewResource(resourceIdType, localResourceIdType, resourceType, reporterTypeType, reporterInstanceIdType, transactionId, reporterResourceIdType, apiHref, &consoleHref, &reporterRepresentation, &commonRepresentation, nil)
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
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository { return NewFakeResourceRepository() },
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			// Helper to get fresh instances
			getFresh := func() bizmodel.ResourceRepository {
				if impl.name == "Fake Repository" {
					return impl.repo()
				}
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			}

			repo := getFresh()

			key, err := bizmodel.NewReporterResourceKey("localResourceId-latest", "host", "hbi", "hbi-instance-1")
			require.NoError(t, err)

			// Set up test data with multiple versions (same for both implementations)
			resource := createTestResourceWithLocalIdAndType(t, "localResourceId-latest", "host")

			// Save initial version (version 0)
			err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-latest-v0")) })
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
			placeholderApiHref, err := bizmodel.NewApiHref("https://api.example.com/placeholder")
			require.NoError(t, err)
			err = resource.Update(key, placeholderApiHref, nil, nil, &updatedReporter1, &updatedCommon1, transactionId1)
			require.NoError(t, err)
			err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("tx-latest-v1")) })
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
			err = resource.Update(key, placeholderApiHref, nil, nil, &updatedReporter2, &updatedCommon2, transactionId2)
			require.NoError(t, err)
			err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("tx-latest-v2")) })
			require.NoError(t, err)

			// Test FindLatestRepresentations
			var result *bizmodel.Representations
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var findErr error
				result, findErr = tx.FindLatestRepresentations(key)
				return findErr
			})
			require.NoError(t, err)

			// Both implementations should return the latest version (version 2)
			assert.Equal(t, "workspace-v2-latest", result.CommonData()["workspace_id"])
			assert.Equal(t, bizmodel.NewVersion(2), *result.CommonVersion())

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
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			getFreshInstances := func() bizmodel.ResourceRepository {
				if impl.name == "Fake Repository" {
					return impl.repo()
				}
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			}

			t.Run("GetCurrentAndPreviousWorkspaceID extracts workspace IDs correctly", func(t *testing.T) {
				repo := getFreshInstances()

				// Create and update a resource to have versioned representations (same for both implementations)
				resource := createTestResourceWithLocalIdAndType(t, "workspace-test-resource", "host")
				err := repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-ws-test")) })
				require.NoError(t, err)

				// Update to create version 1
				key, err := bizmodel.NewReporterResourceKey("workspace-test-resource", "host", "hbi", "hbi-instance-1")
				require.NoError(t, err)

				updatedCommon, err := bizmodel.NewRepresentation(map[string]interface{}{"workspace_id": "workspace-v1"})
				require.NoError(t, err)
				updatedReporter, err := bizmodel.NewRepresentation(map[string]interface{}{"hostname": "updated-host"})
				require.NoError(t, err)

				transactionId := newUniqueTxID("test-transaction-id-workspace-unique")
				placeholderApiHref, err := bizmodel.NewApiHref("https://api.example.com/placeholder")
				require.NoError(t, err)
				err = resource.Update(key, placeholderApiHref, nil, nil, &updatedReporter, &updatedCommon, transactionId)
				require.NoError(t, err)
				require.NoError(t, repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("tx-ws-update")) }))

				// Get current and previous versions
				version := bizmodel.NewVersion(1)
				var cur, prev *bizmodel.Representations
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					cur, prev, findErr = tx.FindCurrentAndPreviousVersionedRepresentations(key, &version, bizmodel.OperationTypeUpdated)
					return findErr
				})
				require.NoError(t, err)

				currentWS, previousWS := GetCurrentAndPreviousWorkspaceID(cur, prev)
				assert.Equal(t, "workspace-v1", currentWS)
				assert.Equal(t, "test-workspace", previousWS) // From initial creation
			})

			t.Run("GetCurrentAndPreviousWorkspaceID handles version 0", func(t *testing.T) {
				repo := getFreshInstances()

				key, err := bizmodel.NewReporterResourceKey("test-resource-v0", "host", "hbi", "hbi-instance-1")
				require.NoError(t, err)

				// Create a resource without updates (version 0) - same for both implementations
				resource := createTestResourceWithLocalIdAndType(t, "test-resource-v0", "host")
				err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-v0-test")) })
				require.NoError(t, err)

				// Get version 0 representations
				version := bizmodel.NewVersion(0)
				var cur, prev *bizmodel.Representations
				err = repo.Transact(func(tx bizmodel.ResourceTx) error {
					var findErr error
					cur, prev, findErr = tx.FindCurrentAndPreviousVersionedRepresentations(key, &version, bizmodel.OperationTypeCreated)
					return findErr
				})
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
					ptrVersion(1),
					nil,
					nil,
				)
				previous, _ := bizmodel.NewRepresentations(
					bizmodel.Representation(map[string]interface{}{"other_field": "value"}), // missing workspace_id
					ptrVersion(0),
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
	}{
		{
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() bizmodel.ResourceRepository {
				return NewFakeResourceRepository()
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			testHasTransactionIdBeenProcessed(t, impl.repo())
		})
	}
}

func testHasTransactionIdBeenProcessed(t *testing.T, repo bizmodel.ResourceRepository) {
	t.Run("Empty transaction ID returns false", func(t *testing.T) {
		var processed bool
		err := repo.Transact(func(tx bizmodel.ResourceTx) error {
			var checkErr error
			processed, checkErr = tx.HasTransactionIdBeenProcessed(emptyTxId)
			return checkErr
		})
		require.NoError(t, err)
		assert.False(t, processed, "Empty transaction ID should return false")
	})

	t.Run("Non-existent transaction ID returns false", func(t *testing.T) {
		transactionId := "non-existent-transaction-123"

		var processed bool
		err := repo.Transact(func(tx bizmodel.ResourceTx) error {
			var checkErr error
			processed, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId))
			return checkErr
		})
		require.NoError(t, err)
		assert.False(t, processed, "Non-existent transaction ID should return false")
	})

	t.Run("Transaction ID tracking for fake repository", func(t *testing.T) {
		// This test is specific to the fake repository implementation
		if fakeRepo, ok := repo.(*fakeResourceRepository); ok {
			transactionId := "test-transaction-456"

			// Initially should not be processed
			var processed bool
			err := fakeRepo.Transact(func(tx bizmodel.ResourceTx) error {
				var checkErr error
				processed, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId))
				return checkErr
			})
			require.NoError(t, err)
			assert.False(t, processed, "Transaction ID should not be processed initially")

			// Mark as processed
			fakeRepo.markTransactionIdAsProcessed(transactionId)

			// Now should be processed
			err = fakeRepo.Transact(func(tx bizmodel.ResourceTx) error {
				var checkErr error
				processed, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId))
				return checkErr
			})
			require.NoError(t, err)
			assert.True(t, processed, "Transaction ID should be processed after marking")

			// Different transaction ID should still be false
			differentTransactionId := "different-transaction-789"
			err = fakeRepo.Transact(func(tx bizmodel.ResourceTx) error {
				var checkErr error
				processed, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(differentTransactionId))
				return checkErr
			})
			require.NoError(t, err)
			assert.False(t, processed, "Different transaction ID should not be processed")
		}
	})

	t.Run("Real repository basic functionality", func(t *testing.T) {
		// This test is specific to the real repository implementation
		// We test basic functionality without complex database setup
		if _, ok := repo.(*resourceRepository); ok {
			transactionId := "test-transaction-789"

			// Test that the method doesn't crash and returns false for non-existent transaction
			var processed bool
			err := repo.Transact(func(tx bizmodel.ResourceTx) error {
				var checkErr error
				processed, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId))
				return checkErr
			})
			require.NoError(t, err)
			assert.False(t, processed, "Non-existent transaction ID should return false")

			// Test empty transaction ID
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var checkErr error
				processed, checkErr = tx.HasTransactionIdBeenProcessed(emptyTxId)
				return checkErr
			})
			require.NoError(t, err)
			assert.False(t, processed, "Empty transaction ID should return false")
		}
	})

	t.Run("Multiple transaction IDs are tracked independently", func(t *testing.T) {
		transactionId1 := "transaction-1"
		transactionId2 := "transaction-2"
		transactionId3 := "transaction-3"

		// Initially none should be processed
		var processed1 bool
		err := repo.Transact(func(tx bizmodel.ResourceTx) error {
			var checkErr error
			processed1, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId1))
			return checkErr
		})
		require.NoError(t, err)
		assert.False(t, processed1, "Transaction ID 1 should not be processed initially")

		var processed2 bool
		err = repo.Transact(func(tx bizmodel.ResourceTx) error {
			var checkErr error
			processed2, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId2))
			return checkErr
		})
		require.NoError(t, err)
		assert.False(t, processed2, "Transaction ID 2 should not be processed initially")

		var processed3 bool
		err = repo.Transact(func(tx bizmodel.ResourceTx) error {
			var checkErr error
			processed3, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId3))
			return checkErr
		})
		require.NoError(t, err)
		assert.False(t, processed3, "Transaction ID 3 should not be processed initially")

		// Mark one as processed (for fake repository)
		if fakeRepo, ok := repo.(*fakeResourceRepository); ok {
			fakeRepo.markTransactionIdAsProcessed(transactionId2)

			// Check all again
			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var checkErr error
				processed1, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId1))
				return checkErr
			})
			require.NoError(t, err)
			assert.False(t, processed1, "Transaction ID 1 should still not be processed")

			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var checkErr error
				processed2, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId2))
				return checkErr
			})
			require.NoError(t, err)
			assert.True(t, processed2, "Transaction ID 2 should now be processed")

			err = repo.Transact(func(tx bizmodel.ResourceTx) error {
				var checkErr error
				processed3, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId3))
				return checkErr
			})
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
					var processed bool
					err := fakeRepo.Transact(func(tx bizmodel.ResourceTx) error {
						var checkErr error
						processed, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId))
						return checkErr
					})
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
					var processed bool
					err := fakeRepo.Transact(func(tx bizmodel.ResourceTx) error {
						var checkErr error
						processed, checkErr = tx.HasTransactionIdBeenProcessed(bizmodel.NewTransactionId(transactionId))
						return checkErr
					})
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
			name: "Real Repository",
			repo: func() bizmodel.ResourceRepository {
				db := setupInMemoryDB(t)
				return newTestGormResourceRepository(db)
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

		err := resource1.Update(key1, apiHref, &consoleHref, nil, &reporterData, &commonData, duplicateTxID)
		require.NoError(t, err)

		err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource1, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-duplicate-1")) })
		require.NoError(t, err, "First save should succeed")

		// Create second resource with the same TransactionID
		resource2 := createTestResourceWithLocalId(t, "duplicate-tx-test-2")
		key2 := createContractReporterResourceKey(t, "duplicate-tx-test-2", "k8s_cluster", "ocm", "ocm-instance-1")

		err = resource2.Update(key2, apiHref, &consoleHref, nil, &reporterData, &commonData, duplicateTxID)
		require.NoError(t, err)

		// This should fail due to unique constraint violation
		err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource2, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-duplicate-2")) })
		require.Error(t, err, "Second save should fail due to duplicate TransactionID")
		assert.Contains(t, err.Error(), bizmodel.ReasonNonUniqueTransactionID)
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
		consoleHref1 := "https://console.example.com/resource/1"
		reporterResource1, err := datamodel.NewReporterResource(
			reporterResourceID,
			"local-id-1",
			"ocm",
			"k8s_cluster",
			"instance-1",
			uuid.New(), // resourceID
			"https://api.example.com/resource/1",
			&consoleHref1,
			0,
			0,
			false,
		)
		require.NoError(t, err)

		reporterResourceID2 := uuid.New()
		consoleHref2 := "https://console.example.com/resource/2"
		reporterResource2, err := datamodel.NewReporterResource(
			reporterResourceID2,
			"local-id-2",
			"ocm",
			"k8s_cluster",
			"instance-1",
			uuid.New(), // resourceID
			"https://api.example.com/resource/2",
			&consoleHref2,
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

}

func TestCommonVersionIncrementAndResetCycle(t *testing.T) {
	db := setupInMemoryDB(t)
	repo := newTestGormResourceRepository(db)

	localResourceID := "resource-common-version-cycle"
	resourceId := uuid.New()
	reporterResourceId := uuid.New()

	localResourceIdType, err := bizmodel.NewLocalResourceId(localResourceID)
	require.NoError(t, err)
	resourceType, err := bizmodel.NewResourceType("k8s_cluster")
	require.NoError(t, err)
	reporterType, err := bizmodel.NewReporterType("ocm")
	require.NoError(t, err)
	reporterInstanceId, err := bizmodel.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	apiHref, err := bizmodel.NewApiHref("/api/resources/cycle-test")
	require.NoError(t, err)
	consoleHref, err := bizmodel.NewConsoleHref("/console/resources/cycle-test")
	require.NoError(t, err)
	resourceIdType, err := bizmodel.NewResourceId(resourceId)
	require.NoError(t, err)
	reporterResourceIdType, err := bizmodel.NewReporterResourceId(reporterResourceId)
	require.NoError(t, err)

	key, err := bizmodel.NewReporterResourceKey(localResourceIdType, resourceType, reporterType, reporterInstanceId)
	require.NoError(t, err)

	// Step 1: Create with common representation → CommonVersion = 0
	cycleRep := bizmodel.Representation(internal.JsonObject{"cluster_id": "cycle-cluster"})
	cycleCommon := bizmodel.Representation(internal.JsonObject{"workspace_id": "cycle-workspace"})
	resource, err := bizmodel.NewResource(
		resourceIdType,
		localResourceIdType,
		resourceType,
		reporterType,
		reporterInstanceId,
		newUniqueTxID("cycle-create"),
		reporterResourceIdType,
		apiHref,
		&consoleHref,
		&cycleRep,
		&cycleCommon,
		nil,
	)
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, emptyTxId) })
	require.NoError(t, err)

	var found *bizmodel.Resource
	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	require.NotNil(t, found)
	snap, _, _, _, err := found.Serialize()
	require.NoError(t, err)
	require.NotNil(t, snap.CommonVersion, "CommonVersion should be set after create with common rep")
	assert.Equal(t, uint(0), *snap.CommonVersion, "CommonVersion should be 0 after initial create")

	// Step 2: Update with common representation → CommonVersion increments to 1
	cycleRepV2 := bizmodel.Representation(internal.JsonObject{"cluster_id": "cycle-cluster-v2"})
	cycleCommonV2 := bizmodel.Representation(internal.JsonObject{"workspace_id": "cycle-workspace-v2"})
	err = found.Update(
		key, apiHref, &consoleHref, nil,
		&cycleRepV2,
		&cycleCommonV2,
		newUniqueTxID("cycle-update-with-common"),
	)
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*found, bizmodel.OperationTypeUpdated, emptyTxId) })
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	require.NotNil(t, found)
	snap, _, _, _, err = found.Serialize()
	require.NoError(t, err)
	require.NotNil(t, snap.CommonVersion, "CommonVersion should be set after update with common rep")
	assert.Equal(t, uint(1), *snap.CommonVersion, "CommonVersion should increment to 1 after update with common rep")

	// Step 3: Update without common representation → CommonVersion resets to nil
	cycleRepReporterOnly := bizmodel.Representation(internal.JsonObject{"cluster_id": "cycle-cluster-reporter-only"})
	err = found.Update(
		key, apiHref, &consoleHref, nil,
		&cycleRepReporterOnly,
		nil,
		newUniqueTxID("cycle-update-without-common"),
	)
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*found, bizmodel.OperationTypeUpdated, emptyTxId) })
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	require.NotNil(t, found)
	snap, _, _, _, err = found.Serialize()
	require.NoError(t, err)
	assert.Nil(t, snap.CommonVersion, "CommonVersion should be nil after update without common rep")
}

func TestCommonVersionAfterDeleteAndRecreate(t *testing.T) {
	db := setupInMemoryDB(t)
	repo := newTestGormResourceRepository(db)

	localResourceID := "resource-delete-recreate"

	localResourceIdType, err := bizmodel.NewLocalResourceId(localResourceID)
	require.NoError(t, err)
	resourceType, err := bizmodel.NewResourceType("k8s_cluster")
	require.NoError(t, err)
	reporterType, err := bizmodel.NewReporterType("ocm")
	require.NoError(t, err)
	reporterInstanceId, err := bizmodel.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	apiHref, err := bizmodel.NewApiHref("/api/resources/delete-recreate")
	require.NoError(t, err)
	consoleHref, err := bizmodel.NewConsoleHref("/console/resources/delete-recreate")
	require.NoError(t, err)

	key, err := bizmodel.NewReporterResourceKey(localResourceIdType, resourceType, reporterType, reporterInstanceId)
	require.NoError(t, err)

	// --- First lifecycle: create + update twice → common versions 0, 1, 2 ---

	resource1IdType, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)
	resource1ReporterIdType, err := bizmodel.NewReporterResourceId(uuid.New())
	require.NoError(t, err)

	drRep1 := bizmodel.Representation(internal.JsonObject{"cluster_id": "v1"})
	drCommon1 := bizmodel.Representation(internal.JsonObject{"workspace_id": "ws-v1"})
	resource1, err := bizmodel.NewResource(
		resource1IdType, localResourceIdType, resourceType, reporterType, reporterInstanceId,
		newUniqueTxID("dr-create-1"),
		resource1ReporterIdType, apiHref, &consoleHref,
		&drRep1,
		&drCommon1,
		nil,
	)
	require.NoError(t, err)
	require.NoError(t, repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource1, bizmodel.OperationTypeCreated, emptyTxId) }))

	var found *bizmodel.Resource
	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	drRep2 := bizmodel.Representation(internal.JsonObject{"cluster_id": "v2"})
	drCommon2 := bizmodel.Representation(internal.JsonObject{"workspace_id": "ws-v2"})
	require.NoError(t, found.Update(key, apiHref, &consoleHref, nil,
		&drRep2,
		&drCommon2,
		newUniqueTxID("dr-update-1a"),
	))
	require.NoError(t, repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*found, bizmodel.OperationTypeUpdated, emptyTxId) }))

	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	drRep3 := bizmodel.Representation(internal.JsonObject{"cluster_id": "v3"})
	drCommon3 := bizmodel.Representation(internal.JsonObject{"workspace_id": "ws-v3"})
	require.NoError(t, found.Update(key, apiHref, &consoleHref, nil,
		&drRep3,
		&drCommon3,
		newUniqueTxID("dr-update-1b"),
	))
	require.NoError(t, repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*found, bizmodel.OperationTypeUpdated, emptyTxId) }))

	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	snap, _, _, _, err := found.Serialize()
	require.NoError(t, err)
	require.Equal(t, uint(2), *snap.CommonVersion, "sanity: first lifecycle should reach common_version 2")

	// Delete the first lifecycle resource
	require.NoError(t, found.Delete(key))
	require.NoError(t, repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*found, bizmodel.OperationTypeDeleted, emptyTxId) }))

	// --- Second lifecycle: new UUIDs, same logical identity ---
	// The system always generates fresh UUIDs on recreation; common_representations
	// rows from the first lifecycle are scoped to the old resource_id and must not
	// influence version numbering for the new one.

	resource2IdType, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)
	resource2ReporterIdType, err := bizmodel.NewReporterResourceId(uuid.New())
	require.NoError(t, err)

	drRep1New := bizmodel.Representation(internal.JsonObject{"cluster_id": "v1-new"})
	drCommon1New := bizmodel.Representation(internal.JsonObject{"workspace_id": "ws-v1-new"})
	resource2, err := bizmodel.NewResource(
		resource2IdType, localResourceIdType, resourceType, reporterType, reporterInstanceId,
		newUniqueTxID("dr-create-2"),
		resource2ReporterIdType, apiHref, &consoleHref,
		&drRep1New,
		&drCommon1New,
		nil,
	)
	require.NoError(t, err)
	require.NoError(t, repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource2, bizmodel.OperationTypeCreated, emptyTxId) }))

	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	snap, _, _, _, err = found.Serialize()
	require.NoError(t, err)
	assert.Equal(t, uint(0), *snap.CommonVersion, "second lifecycle should start at common_version 0")

	drRep2New := bizmodel.Representation(internal.JsonObject{"cluster_id": "v2-new"})
	drCommon2New := bizmodel.Representation(internal.JsonObject{"workspace_id": "ws-v2-new"})
	require.NoError(t, found.Update(key, apiHref, &consoleHref, nil,
		&drRep2New,
		&drCommon2New,
		newUniqueTxID("dr-update-2a"),
	))
	require.NoError(t, repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*found, bizmodel.OperationTypeUpdated, emptyTxId) }))

	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	snap, _, _, _, err = found.Serialize()
	require.NoError(t, err)
	assert.Equal(t, uint(1), *snap.CommonVersion,
		"second lifecycle after one update should be common_version 1, not influenced by first lifecycle's history")
}

func TestNullCommonVersionPersistence(t *testing.T) {
	db := setupInMemoryDB(t)
	repo := newTestGormResourceRepository(db)

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
	emptyCommonRepresentation := bizmodel.Representation(internal.JsonObject{})

	resourceIdType, err := bizmodel.NewResourceId(resourceId)
	require.NoError(t, err)

	reporterResourceIdType, err := bizmodel.NewReporterResourceId(reporterResourceId)
	require.NoError(t, err)

	txID := newUniqueTxID("null-common-version-test")

	resource, err := bizmodel.NewResource(
		resourceIdType,
		localResourceIdType,
		resourceType,
		reporterType,
		reporterInstanceId,
		txID,
		reporterResourceIdType,
		apiHref,
		&consoleHref,
		&reporterRepresentation,
		nil,
		nil,
	)
	_ = emptyCommonRepresentation
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, emptyTxId) })
	require.NoError(t, err, "Should save resource without common representation")

	key, err := bizmodel.NewReporterResourceKey(
		localResourceIdType,
		resourceType,
		reporterType,
		reporterInstanceId,
	)
	require.NoError(t, err)

	var retrievedResource *bizmodel.Resource
	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		retrievedResource, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err, "Should find resource by keys")
	require.NotNil(t, retrievedResource, "Retrieved resource should not be nil")

	resourceSnapshot, _, _, _, err := retrievedResource.Serialize()
	require.NoError(t, err, "Should serialize resource")

	require.Nil(t, resourceSnapshot.CommonVersion, "CommonVersion should be nil for resource without common representation")
}

func TestCommonVersionDropThenReAdd(t *testing.T) {
	db := setupInMemoryDB(t)
	repo := newTestGormResourceRepository(db)

	localResourceID := "resource-common-version-drop-readd"
	resourceId := uuid.New()
	reporterResourceId := uuid.New()

	localResourceIdType, err := bizmodel.NewLocalResourceId(localResourceID)
	require.NoError(t, err)
	resourceType, err := bizmodel.NewResourceType("k8s_cluster")
	require.NoError(t, err)
	reporterType, err := bizmodel.NewReporterType("ocm")
	require.NoError(t, err)
	reporterInstanceId, err := bizmodel.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	apiHref, err := bizmodel.NewApiHref("/api/resources/drop-readd-test")
	require.NoError(t, err)
	consoleHref, err := bizmodel.NewConsoleHref("/console/resources/drop-readd-test")
	require.NoError(t, err)
	resourceIdType, err := bizmodel.NewResourceId(resourceId)
	require.NoError(t, err)
	reporterResourceIdType, err := bizmodel.NewReporterResourceId(reporterResourceId)
	require.NoError(t, err)

	key, err := bizmodel.NewReporterResourceKey(localResourceIdType, resourceType, reporterType, reporterInstanceId)
	require.NoError(t, err)

	// Step 1: Create with common representation → CommonVersion = 0
	daRep := bizmodel.Representation(internal.JsonObject{"cluster_id": "drop-readd-cluster"})
	daCommon := bizmodel.Representation(internal.JsonObject{"workspace_id": "drop-readd-workspace"})
	resource, err := bizmodel.NewResource(
		resourceIdType,
		localResourceIdType,
		resourceType,
		reporterType,
		reporterInstanceId,
		newUniqueTxID("drop-readd-create"),
		reporterResourceIdType,
		apiHref,
		&consoleHref,
		&daRep,
		&daCommon,
		nil,
	)
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, emptyTxId) })
	require.NoError(t, err)

	var found *bizmodel.Resource
	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	require.NotNil(t, found)
	snap, _, _, _, err := found.Serialize()
	require.NoError(t, err)
	assert.Equal(t, uint(0), *snap.CommonVersion, "CommonVersion should be 0 after initial create")

	// Step 2 (swapped): Update WITHOUT common representation → CommonVersion resets to nil
	daRepReporterOnly := bizmodel.Representation(internal.JsonObject{"cluster_id": "drop-readd-cluster-reporter-only"})
	err = found.Update(
		key, apiHref, &consoleHref, nil,
		&daRepReporterOnly,
		nil,
		newUniqueTxID("drop-readd-update-without"),
	)
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*found, bizmodel.OperationTypeUpdated, emptyTxId) })
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	require.NotNil(t, found)
	snap, _, _, _, err = found.Serialize()
	require.NoError(t, err)
	assert.Nil(t, snap.CommonVersion, "CommonVersion should be nil after update without common rep")

	// Step 3 (swapped): Update WITH common representation → CommonVersion resumes at 1
	daRepV2 := bizmodel.Representation(internal.JsonObject{"cluster_id": "drop-readd-cluster-v2"})
	daCommonV2 := bizmodel.Representation(internal.JsonObject{"workspace_id": "drop-readd-workspace-v2"})
	err = found.Update(
		key, apiHref, &consoleHref, nil,
		&daRepV2,
		&daCommonV2,
		newUniqueTxID("drop-readd-update-with"),
	)
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*found, bizmodel.OperationTypeUpdated, emptyTxId) })
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	require.NotNil(t, found)
	snap, _, _, _, err = found.Serialize()
	require.NoError(t, err)
	require.NotNil(t, snap.CommonVersion, "CommonVersion should be set after re-adding common rep")
	assert.Equal(t, uint(1), *snap.CommonVersion, "CommonVersion should be 1 (continuing from last used version, not re-initializing to 0)")
}

func TestCommonVersionFirstAddedOnUpdate(t *testing.T) {
	db := setupInMemoryDB(t)
	repo := newTestGormResourceRepository(db)

	localResourceID := "resource-no-common-then-update"
	resourceId := uuid.New()
	reporterResourceId := uuid.New()

	localResourceIdType, err := bizmodel.NewLocalResourceId(localResourceID)
	require.NoError(t, err)
	resourceType, err := bizmodel.NewResourceType("k8s_cluster")
	require.NoError(t, err)
	reporterType, err := bizmodel.NewReporterType("ocm")
	require.NoError(t, err)
	reporterInstanceId, err := bizmodel.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	apiHref, err := bizmodel.NewApiHref("/api/resources/first-add-test")
	require.NoError(t, err)
	consoleHref, err := bizmodel.NewConsoleHref("/console/resources/first-add-test")
	require.NoError(t, err)
	resourceIdType, err := bizmodel.NewResourceId(resourceId)
	require.NoError(t, err)
	reporterResourceIdType, err := bizmodel.NewReporterResourceId(reporterResourceId)
	require.NoError(t, err)

	key, err := bizmodel.NewReporterResourceKey(localResourceIdType, resourceType, reporterType, reporterInstanceId)
	require.NoError(t, err)

	// Step 1: Create without common representation → CommonVersion = nil
	faRep := bizmodel.Representation(internal.JsonObject{"cluster_id": "first-add-cluster"})
	resource, err := bizmodel.NewResource(
		resourceIdType,
		localResourceIdType,
		resourceType,
		reporterType,
		reporterInstanceId,
		newUniqueTxID("first-add-create"),
		reporterResourceIdType,
		apiHref,
		&consoleHref,
		&faRep,
		nil,
		nil,
	)
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(resource, bizmodel.OperationTypeCreated, emptyTxId) })
	require.NoError(t, err)

	var found *bizmodel.Resource
	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	require.NotNil(t, found)
	snap, _, _, _, err := found.Serialize()
	require.NoError(t, err)
	assert.Nil(t, snap.CommonVersion, "CommonVersion should be nil before any common rep is provided")

	// Step 2: Update with common representation for the first time → CommonVersion initializes to 0
	faRepV2 := bizmodel.Representation(internal.JsonObject{"cluster_id": "first-add-cluster-v2"})
	faCommon := bizmodel.Representation(internal.JsonObject{"workspace_id": "first-add-workspace"})
	err = found.Update(
		key, apiHref, &consoleHref, nil,
		&faRepV2,
		&faCommon,
		newUniqueTxID("first-add-update"),
	)
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error { return tx.Save(*found, bizmodel.OperationTypeUpdated, emptyTxId) })
	require.NoError(t, err)

	err = repo.Transact(func(tx bizmodel.ResourceTx) error {
		var findErr error
		found, findErr = tx.FindResourceByKeys(key)
		return findErr
	})
	require.NoError(t, err)
	require.NotNil(t, found)
	snap, _, _, _, err = found.Serialize()
	require.NoError(t, err)
	require.NotNil(t, snap.CommonVersion, "CommonVersion should be set after first update with common rep")
	assert.Equal(t, uint(0), *snap.CommonVersion, "CommonVersion should initialize to 0 on first addition via update")
}
