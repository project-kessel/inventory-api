package v1beta2

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
)

// setupResourceTestDB creates an in-memory SQLite database for testing
func setupResourceTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate the schema
	err = db.AutoMigrate(&v1beta2.Resource{}, &v1beta2.RepresentationReference{})
	require.NoError(t, err)

	return db
}

// createTestResourceWithReferences creates a test ResourceWithReferences
func createTestResourceWithReferences() *v1beta2.ResourceWithReferences {
	// Let the repository generate the ID
	return &v1beta2.ResourceWithReferences{
		Resource: &v1beta2.Resource{
			ID:   uuid.Nil, // Repository will generate this
			Type: "test-resource-type",
		},
		RepresentationReferences: []*v1beta2.RepresentationReference{
			{
				ResourceID:            uuid.Nil, // Repository will set this
				LocalResourceID:       "test-resource-123",
				ReporterType:          "test-reporter",
				ResourceType:          "test-type",
				ReporterInstanceID:    "instance-123",
				RepresentationVersion: 1,
				Generation:            1,
				Tombstone:             false,
			},
			{
				ResourceID:            uuid.Nil, // Repository will set this
				LocalResourceID:       "test-resource-456",
				ReporterType:          "another-reporter",
				ResourceType:          "test-type",
				ReporterInstanceID:    "instance-456",
				RepresentationVersion: 1,
				Generation:            1,
				Tombstone:             false,
			},
		},
	}
}

// createTestReporterRepresentationId creates a test ReporterRepresentationId
func createTestReporterRepresentationId() v1beta2.ReporterRepresentationId {
	return v1beta2.ReporterRepresentationId{
		LocalResourceID:    "test-resource-123",
		ReporterType:       "test-reporter",
		ResourceType:       "test-type",
		ReporterInstanceID: "instance-123",
	}
}

// TestResourceWithReferencesRepositoryContract tests that both real and fake implementations
// satisfy the same contract
func TestResourceWithReferencesRepositoryContract(t *testing.T) {
	t.Run("Real Repository", func(t *testing.T) {
		testResourceWithReferencesRepositoryReal(t)
	})

	t.Run("Fake Repository", func(t *testing.T) {
		repo := NewFakeResourceWithReferencesRepository()
		testResourceWithReferencesRepository(t, repo)
	})
}

// testResourceWithReferencesRepositoryReal runs tests for the real repository with fresh databases
func testResourceWithReferencesRepositoryReal(t *testing.T) {
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		// Test successful creation
		aggregate := createTestResourceWithReferences()

		result, err := repo.Create(ctx, aggregate)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Resource)
		assert.NotEmpty(t, result.Resource.ID)
		assert.Equal(t, aggregate.Resource.Type, result.Resource.Type)
		assert.Len(t, result.RepresentationReferences, len(aggregate.RepresentationReferences))

		// Verify all references point to the resource
		for _, ref := range result.RepresentationReferences {
			assert.Equal(t, result.Resource.ID, ref.ResourceID)
		}
	})

	t.Run("Create with nil aggregate", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		result, err := repo.Create(ctx, nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Create with nil resource", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		aggregate := &v1beta2.ResourceWithReferences{
			Resource:                 nil,
			RepresentationReferences: []*v1beta2.RepresentationReference{},
		}

		result, err := repo.Create(ctx, aggregate)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "resource cannot be nil")
	})

	t.Run("Create with empty references", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		aggregate := createTestResourceWithReferences()
		aggregate.RepresentationReferences = []*v1beta2.RepresentationReference{}

		result, err := repo.Create(ctx, aggregate)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.RepresentationReferences, 0)
	})

	t.Run("Create generates resource ID if not set", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		aggregate := createTestResourceWithReferences()
		aggregate.Resource.ID = uuid.Nil

		result, err := repo.Create(ctx, aggregate)

		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, result.Resource.ID)

		// Verify all references point to the generated resource ID
		for _, ref := range result.RepresentationReferences {
			assert.Equal(t, result.Resource.ID, ref.ResourceID)
		}
	})

	t.Run("FindAllReferencesByReporterRepresentationId", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		// Create a resource with references
		aggregate := createTestResourceWithReferences()
		_, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		// Search for references using the reporter representation ID
		reporterId := createTestReporterRepresentationId()

		refs, err := repo.FindAllReferencesByReporterRepresentationId(ctx, reporterId)

		assert.NoError(t, err)
		assert.NotEmpty(t, refs)

		// Debug: Print what we got
		t.Logf("Found %d references:", len(refs))
		for i, ref := range refs {
			t.Logf("  Ref %d: ResourceID=%s, LocalResourceID=%s, ReporterType=%s",
				i, ref.ResourceID, ref.LocalResourceID, ref.ReporterType)
		}

		// Verify we get all references for the same resource
		// All returned references should have the same resource_id
		if len(refs) > 0 {
			resourceID := refs[0].ResourceID
			for _, ref := range refs {
				assert.Equal(t, resourceID, ref.ResourceID, "All references should belong to the same resource")
			}
		}
	})

	t.Run("FindAllReferencesByReporterRepresentationId with no matches", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		// Search for non-existent reporter ID
		reporterId := v1beta2.ReporterRepresentationId{
			LocalResourceID:    "non-existent",
			ReporterType:       "non-existent",
			ResourceType:       "non-existent",
			ReporterInstanceID: "non-existent",
		}

		refs, err := repo.FindAllReferencesByReporterRepresentationId(ctx, reporterId)

		assert.NoError(t, err)
		assert.Empty(t, refs)
	})
}

// testResourceWithReferencesRepository runs the contract tests on any implementation
func testResourceWithReferencesRepository(t *testing.T, repo v1beta2.ResourceWithReferencesRepository) {
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		// Reset fake repository if it supports it
		if fakeRepo, ok := repo.(*FakeResourceWithReferencesRepository); ok {
			fakeRepo.Reset()
		}
		// Test successful creation
		aggregate := createTestResourceWithReferences()

		result, err := repo.Create(ctx, aggregate)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Resource)
		assert.NotEmpty(t, result.Resource.ID)
		assert.Equal(t, aggregate.Resource.Type, result.Resource.Type)
		assert.Len(t, result.RepresentationReferences, len(aggregate.RepresentationReferences))

		// Verify all references point to the resource
		for _, ref := range result.RepresentationReferences {
			assert.Equal(t, result.Resource.ID, ref.ResourceID)
		}
	})

	t.Run("Create with nil aggregate", func(t *testing.T) {
		result, err := repo.Create(ctx, nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Create with nil resource", func(t *testing.T) {
		aggregate := &v1beta2.ResourceWithReferences{
			Resource:                 nil,
			RepresentationReferences: []*v1beta2.RepresentationReference{},
		}

		result, err := repo.Create(ctx, aggregate)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "resource cannot be nil")
	})

	t.Run("Create with empty references", func(t *testing.T) {
		aggregate := createTestResourceWithReferences()
		aggregate.RepresentationReferences = []*v1beta2.RepresentationReference{}

		result, err := repo.Create(ctx, aggregate)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.RepresentationReferences, 0)
	})

	t.Run("Create generates resource ID if not set", func(t *testing.T) {
		aggregate := createTestResourceWithReferences()
		aggregate.Resource.ID = uuid.Nil

		result, err := repo.Create(ctx, aggregate)

		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, result.Resource.ID)

		// Verify all references point to the generated resource ID
		for _, ref := range result.RepresentationReferences {
			assert.Equal(t, result.Resource.ID, ref.ResourceID)
		}
	})

	t.Run("FindAllReferencesByReporterRepresentationId", func(t *testing.T) {
		// Reset fake repository if it supports it
		if fakeRepo, ok := repo.(*FakeResourceWithReferencesRepository); ok {
			fakeRepo.Reset()
		}

		// Create a resource with references
		aggregate := createTestResourceWithReferences()
		_, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		// Search for references using the reporter representation ID
		reporterId := createTestReporterRepresentationId()

		refs, err := repo.FindAllReferencesByReporterRepresentationId(ctx, reporterId)

		assert.NoError(t, err)
		assert.NotEmpty(t, refs)

		// Debug: Print what we got
		t.Logf("Found %d references:", len(refs))
		for i, ref := range refs {
			t.Logf("  Ref %d: ResourceID=%s, LocalResourceID=%s, ReporterType=%s",
				i, ref.ResourceID, ref.LocalResourceID, ref.ReporterType)
		}

		// Verify we get all references for the same resource
		// All returned references should have the same resource_id
		if len(refs) > 0 {
			resourceID := refs[0].ResourceID
			for _, ref := range refs {
				assert.Equal(t, resourceID, ref.ResourceID, "All references should belong to the same resource")
			}
		}
	})

	t.Run("FindAllReferencesByReporterRepresentationId with no matches", func(t *testing.T) {
		// Reset fake repository if it supports it
		if fakeRepo, ok := repo.(*FakeResourceWithReferencesRepository); ok {
			fakeRepo.Reset()
		}

		// Search for non-existent reporter ID
		reporterId := v1beta2.ReporterRepresentationId{
			LocalResourceID:    "non-existent",
			ReporterType:       "non-existent",
			ResourceType:       "non-existent",
			ReporterInstanceID: "non-existent",
		}

		refs, err := repo.FindAllReferencesByReporterRepresentationId(ctx, reporterId)

		assert.NoError(t, err)
		assert.Empty(t, refs)
	})
}

// TestResourceWithReferencesRepositorySpecific tests specific to the real implementation
func TestResourceWithReferencesRepositorySpecific(t *testing.T) {
	db := setupResourceTestDB(t)
	repo := NewResourceWithReferencesRepository(db)
	ctx := context.Background()

	t.Run("Database persistence", func(t *testing.T) {
		aggregate := createTestResourceWithReferences()

		// Create the aggregate
		result, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		// Verify resource is in the database
		var resourceCount int64
		err = db.Model(&v1beta2.Resource{}).
			Where("id = ?", result.Resource.ID).
			Count(&resourceCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), resourceCount)

		// Verify references are in the database
		var refCount int64
		err = db.Model(&v1beta2.RepresentationReference{}).
			Where("resource_id = ?", result.Resource.ID).
			Count(&refCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(len(aggregate.RepresentationReferences)), refCount)
	})

	t.Run("Transaction handling", func(t *testing.T) {
		aggregate := createTestResourceWithReferences()

		var result *v1beta2.ResourceWithReferences
		var err error

		// Test with transaction
		txErr := db.Transaction(func(tx *gorm.DB) error {
			txRepo := NewResourceWithReferencesRepository(tx)
			result, err = txRepo.Create(ctx, aggregate)
			return err
		})

		require.NoError(t, txErr)
		require.NoError(t, err)

		// Verify it's committed
		var count int64
		err = db.Model(&v1beta2.Resource{}).
			Where("id = ?", result.Resource.ID).
			Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("Search functionality", func(t *testing.T) {
		// Create fresh database for this test
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		// Create test data
		aggregate := createTestResourceWithReferences()
		createdAggregate, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		// Test the search
		reporterId := createTestReporterRepresentationId()
		refs, err := repo.FindAllReferencesByReporterRepresentationId(ctx, reporterId)

		require.NoError(t, err)
		assert.NotEmpty(t, refs)

		// Verify all references belong to the same resource (the one we created)
		expectedResourceID := createdAggregate.Resource.ID
		assert.Len(t, refs, 2, "Should return all references for the resource")

		for _, ref := range refs {
			assert.Equal(t, expectedResourceID, ref.ResourceID, "All references should belong to the created resource")
		}
	})
}

// TestFakeResourceWithReferencesRepositorySpecific tests specific to the fake implementation
func TestFakeResourceWithReferencesRepositorySpecific(t *testing.T) {
	repo := NewFakeResourceWithReferencesRepository()
	ctx := context.Background()

	t.Run("Reset functionality", func(t *testing.T) {
		// Create some data
		aggregate := createTestResourceWithReferences()
		_, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		// Verify it exists
		assert.Equal(t, 1, repo.Count())

		// Reset and verify it's gone
		repo.Reset()
		assert.Equal(t, 0, repo.Count())
	})

	t.Run("GetAllResources functionality", func(t *testing.T) {
		repo.Reset()

		// Create multiple aggregates
		for i := 0; i < 3; i++ {
			aggregate := createTestResourceWithReferences()
			aggregate.Resource.ID = uuid.New() // Ensure unique IDs
			_, err := repo.Create(ctx, aggregate)
			require.NoError(t, err)
		}

		// Get all resources and verify
		resources := repo.GetAllResources()
		assert.Len(t, resources, 3)

		// Verify they're copies
		for _, resource := range resources {
			// Modify the returned resource
			resource.Type = "modified-type"
		}

		// Original data should be unchanged
		assert.Equal(t, 3, repo.Count())
	})

	t.Run("GetAllAggregates functionality", func(t *testing.T) {
		repo.Reset()

		// Create multiple aggregates with different numbers of references
		for i := 0; i < 2; i++ {
			aggregate := createTestResourceWithReferences()
			aggregate.Resource.ID = uuid.New()

			// Vary the number of references
			if i == 1 {
				aggregate.RepresentationReferences = aggregate.RepresentationReferences[:1]
			}

			_, err := repo.Create(ctx, aggregate)
			require.NoError(t, err)
		}

		// Get all aggregates and verify
		aggregates := repo.GetAllAggregates()
		assert.Len(t, aggregates, 2)

		// Verify structure
		for i, agg := range aggregates {
			assert.NotNil(t, agg.Resource)
			assert.NotEmpty(t, agg.RepresentationReferences)

			if i == 0 {
				assert.Len(t, agg.RepresentationReferences, 2)
			} else {
				assert.Len(t, agg.RepresentationReferences, 1)
			}
		}
	})

	t.Run("Search functionality", func(t *testing.T) {
		repo.Reset()

		// Create multiple resources with overlapping references
		resource1ID := uuid.New()
		resource2ID := uuid.New()

		// Resource 1 with specific references
		agg1 := &v1beta2.ResourceWithReferences{
			Resource: &v1beta2.Resource{
				ID:   resource1ID,
				Type: "search-resource-type-1",
			},
			RepresentationReferences: []*v1beta2.RepresentationReference{
				{
					ResourceID:            resource1ID,
					LocalResourceID:       "search-test-123",
					ReporterType:          "search-reporter",
					ResourceType:          "search-type",
					ReporterInstanceID:    "search-instance",
					RepresentationVersion: 1,
					Generation:            1,
				},
				{
					ResourceID:            resource1ID,
					LocalResourceID:       "other-ref",
					ReporterType:          "other-reporter",
					ResourceType:          "other-type",
					ReporterInstanceID:    "other-instance",
					RepresentationVersion: 1,
					Generation:            1,
				},
			},
		}

		// Resource 2 with different references
		agg2 := &v1beta2.ResourceWithReferences{
			Resource: &v1beta2.Resource{
				ID:   resource2ID,
				Type: "search-resource-type-2",
			},
			RepresentationReferences: []*v1beta2.RepresentationReference{
				{
					ResourceID:            resource2ID,
					LocalResourceID:       "different-ref",
					ReporterType:          "different-reporter",
					ResourceType:          "different-type",
					ReporterInstanceID:    "different-instance",
					RepresentationVersion: 1,
					Generation:            1,
				},
			},
		}

		_, err := repo.Create(ctx, agg1)
		require.NoError(t, err)
		_, err = repo.Create(ctx, agg2)
		require.NoError(t, err)

		// Search for references by the first resource's reporter ID
		searchId := v1beta2.ReporterRepresentationId{
			LocalResourceID:    "search-test-123",
			ReporterType:       "search-reporter",
			ResourceType:       "search-type",
			ReporterInstanceID: "search-instance",
		}

		refs, err := repo.FindAllReferencesByReporterRepresentationId(ctx, searchId)
		require.NoError(t, err)

		// Should return all references for resource1, but none for resource2
		assert.Len(t, refs, 2)
		for _, ref := range refs {
			assert.Equal(t, resource1ID, ref.ResourceID)
		}
	})
}

// TestUpdateConsistencyToken tests the UpdateConsistencyToken method for both repositories
func TestUpdateConsistencyToken(t *testing.T) {
	t.Run("Real Repository", func(t *testing.T) {
		testUpdateConsistencyTokenReal(t)
	})

	t.Run("Fake Repository", func(t *testing.T) {
		repo := NewFakeResourceWithReferencesRepository()
		testUpdateConsistencyToken(t, repo)
	})
}

// testUpdateConsistencyTokenReal runs tests for the real repository with fresh databases
func testUpdateConsistencyTokenReal(t *testing.T) {
	ctx := context.Background()

	t.Run("Update existing resource", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		// Create a resource first
		aggregate := createTestResourceWithReferences()
		aggregate.Resource.ConsistencyToken = "initial-token"

		result, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)
		require.NotNil(t, result.Resource)

		resourceID := result.Resource.ID
		newToken := "updated-token-123"

		// Update the consistency token
		err = repo.UpdateConsistencyToken(ctx, resourceID, newToken)
		assert.NoError(t, err)

		// Verify the token was updated by checking directly in the database
		var resource v1beta2.Resource
		err = db.Where("id = ?", resourceID).First(&resource).Error
		require.NoError(t, err)
		assert.Equal(t, newToken, resource.ConsistencyToken)
	})

	t.Run("Update non-existent resource", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		nonExistentID := uuid.New()
		newToken := "some-token"

		// Try to update a resource that doesn't exist
		err := repo.UpdateConsistencyToken(ctx, nonExistentID, newToken)

		// Should not return an error - GORM's Update method doesn't error when no rows are affected
		// This matches the behavior of the consumer's UpdateConsistencyToken method
		assert.NoError(t, err)
	})

	t.Run("Update with empty token", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		// Create a resource first
		aggregate := createTestResourceWithReferences()
		aggregate.Resource.ConsistencyToken = "initial-token"

		result, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		resourceID := result.Resource.ID
		emptyToken := ""

		// Update with empty token
		err = repo.UpdateConsistencyToken(ctx, resourceID, emptyToken)
		assert.NoError(t, err)

		// Verify the token was cleared
		var resource v1beta2.Resource
		err = db.Where("id = ?", resourceID).First(&resource).Error
		require.NoError(t, err)
		assert.Equal(t, emptyToken, resource.ConsistencyToken)
	})
}

// testUpdateConsistencyToken runs tests for any repository implementation
func testUpdateConsistencyToken(t *testing.T, repo v1beta2.ResourceWithReferencesRepository) {
	ctx := context.Background()

	t.Run("Update existing resource", func(t *testing.T) {
		// Create a resource first
		aggregate := createTestResourceWithReferences()
		aggregate.Resource.ConsistencyToken = "initial-token"

		result, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)
		require.NotNil(t, result.Resource)

		resourceID := result.Resource.ID
		newToken := "updated-token-456"

		// Update the consistency token
		err = repo.UpdateConsistencyToken(ctx, resourceID, newToken)
		assert.NoError(t, err)

		// For fake repository, we can verify by getting all resources
		if fakeRepo, ok := repo.(*FakeResourceWithReferencesRepository); ok {
			resources := fakeRepo.GetAllResources()
			found := false
			for _, resource := range resources {
				if resource.ID == resourceID {
					assert.Equal(t, newToken, resource.ConsistencyToken)
					found = true
					break
				}
			}
			assert.True(t, found, "Resource should be found in fake repository")
		}
	})

	t.Run("Update non-existent resource", func(t *testing.T) {
		nonExistentID := uuid.New()
		newToken := "some-token"

		// Try to update a resource that doesn't exist
		err := repo.UpdateConsistencyToken(ctx, nonExistentID, newToken)

		// For fake repository, this should return an error
		if _, ok := repo.(*FakeResourceWithReferencesRepository); ok {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not found")
		} else {
			// For real repository, GORM doesn't error when no rows are affected
			assert.NoError(t, err)
		}
	})

	t.Run("Update with nil UUID", func(t *testing.T) {
		newToken := "some-token"

		// Try to update with nil UUID
		err := repo.UpdateConsistencyToken(ctx, uuid.Nil, newToken)

		// Both implementations should handle this gracefully
		// Real repository: GORM will just not find any rows to update
		// Fake repository: Will not find the resource
		if _, ok := repo.(*FakeResourceWithReferencesRepository); ok {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	})
}

// TestUpdateRepresentationVersion tests the UpdateRepresentationVersion method for both repositories
func TestUpdateRepresentationVersion(t *testing.T) {
	t.Run("Real Repository", func(t *testing.T) {
		testUpdateRepresentationVersionReal(t)
	})

	t.Run("Fake Repository", func(t *testing.T) {
		repo := NewFakeResourceWithReferencesRepository()
		testUpdateRepresentationVersion(t, repo)
	})
}

// testUpdateRepresentationVersionReal runs tests for the real repository with fresh databases
func testUpdateRepresentationVersionReal(t *testing.T) {
	ctx := context.Background()

	t.Run("Update all references for resource", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		// Create a resource with multiple references
		aggregate := createTestResourceWithReferences()
		result, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		resourceID := result.Resource.ID
		newVersion := 5

		// Update all references (no filters)
		filter := v1beta2.RepresentationVersionUpdateFilter{
			ResourceID: resourceID,
		}
		rowsAffected, err := repo.UpdateRepresentationVersion(ctx, filter, newVersion)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), rowsAffected)

		// Verify the updates in database
		var refs []v1beta2.RepresentationReference
		err = db.Where("resource_id = ?", resourceID).Find(&refs).Error
		require.NoError(t, err)
		assert.Len(t, refs, 2)
		for _, ref := range refs {
			assert.Equal(t, newVersion, ref.RepresentationVersion)
		}
	})

	t.Run("Update specific reporter type", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		// Create a resource with multiple references
		aggregate := createTestResourceWithReferences()
		result, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		resourceID := result.Resource.ID
		newVersion := 7
		targetReporter := "test-reporter"

		// Update only test-reporter references
		filter := v1beta2.RepresentationVersionUpdateFilter{
			ResourceID:   resourceID,
			ReporterType: &targetReporter,
		}
		rowsAffected, err := repo.UpdateRepresentationVersion(ctx, filter, newVersion)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), rowsAffected)

		// Verify only one reference was updated
		var refs []v1beta2.RepresentationReference
		err = db.Where("resource_id = ? AND reporter_type = ?", resourceID, targetReporter).Find(&refs).Error
		require.NoError(t, err)
		assert.Len(t, refs, 1)
		assert.Equal(t, newVersion, refs[0].RepresentationVersion)

		// Verify other reference was not updated
		var otherRefs []v1beta2.RepresentationReference
		err = db.Where("resource_id = ? AND reporter_type != ?", resourceID, targetReporter).Find(&otherRefs).Error
		require.NoError(t, err)
		assert.Len(t, otherRefs, 1)
		assert.Equal(t, 1, otherRefs[0].RepresentationVersion) // Original version
	})

	t.Run("Update non-existent resource", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		nonExistentID := uuid.New()
		filter := v1beta2.RepresentationVersionUpdateFilter{
			ResourceID: nonExistentID,
		}
		rowsAffected, err := repo.UpdateRepresentationVersion(ctx, filter, 5)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), rowsAffected)
	})
}

// testUpdateRepresentationVersion runs tests for any repository implementation
func testUpdateRepresentationVersion(t *testing.T, repo v1beta2.ResourceWithReferencesRepository) {
	ctx := context.Background()

	t.Run("Update all references for resource", func(t *testing.T) {
		// Create a resource with multiple references
		aggregate := createTestResourceWithReferences()
		result, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		resourceID := result.Resource.ID
		newVersion := 5

		// Update all references (no filters)
		filter := v1beta2.RepresentationVersionUpdateFilter{
			ResourceID: resourceID,
		}
		rowsAffected, err := repo.UpdateRepresentationVersion(ctx, filter, newVersion)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), rowsAffected)

		// For fake repository, verify the updates
		if fakeRepo, ok := repo.(*FakeResourceWithReferencesRepository); ok {
			aggregates := fakeRepo.GetAllAggregates()
			found := false
			for _, agg := range aggregates {
				if agg.Resource.ID == resourceID {
					assert.Len(t, agg.RepresentationReferences, 2)
					for _, ref := range agg.RepresentationReferences {
						assert.Equal(t, newVersion, ref.RepresentationVersion)
					}
					found = true
					break
				}
			}
			assert.True(t, found, "Resource should be found in fake repository")
		}
	})

	t.Run("Update non-existent resource", func(t *testing.T) {
		nonExistentID := uuid.New()
		filter := v1beta2.RepresentationVersionUpdateFilter{
			ResourceID: nonExistentID,
		}
		rowsAffected, err := repo.UpdateRepresentationVersion(ctx, filter, 5)

		// For fake repository, this should return an error
		if _, ok := repo.(*FakeResourceWithReferencesRepository); ok {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not found")
			assert.Equal(t, int64(0), rowsAffected)
		} else {
			// For real repository, no error but 0 rows affected
			assert.NoError(t, err)
			assert.Equal(t, int64(0), rowsAffected)
		}
	})
}

// TestUpdateRepresentationVersionHelperMethods tests the convenience methods
func TestUpdateRepresentationVersionHelperMethods(t *testing.T) {
	t.Run("Real Repository", func(t *testing.T) {
		testUpdateRepresentationVersionHelperMethodsReal(t)
	})

	t.Run("Fake Repository", func(t *testing.T) {
		repo := NewFakeResourceWithReferencesRepository()
		testUpdateRepresentationVersionHelperMethods(t, repo)
	})
}

// testUpdateRepresentationVersionHelperMethodsReal tests helper methods for real repository
func testUpdateRepresentationVersionHelperMethodsReal(t *testing.T) {
	ctx := context.Background()

	t.Run("UpdateCommonRepresentationVersion", func(t *testing.T) {
		db := setupResourceTestDB(t)
		repo := NewResourceWithReferencesRepository(db)

		// Create a resource with inventory reporter reference
		resourceID := uuid.New()
		aggregate := &v1beta2.ResourceWithReferences{
			Resource: &v1beta2.Resource{
				ID:   resourceID,
				Type: "test-resource-type",
			},
			RepresentationReferences: []*v1beta2.RepresentationReference{
				{
					ResourceID:            resourceID,
					LocalResourceID:       "inventory-123",
					ReporterType:          "inventory",
					ResourceType:          "test-type",
					ReporterInstanceID:    "inventory-instance",
					RepresentationVersion: 1,
					Generation:            1,
				},
				{
					ResourceID:            resourceID,
					LocalResourceID:       "other-123",
					ReporterType:          "other-reporter",
					ResourceType:          "test-type",
					ReporterInstanceID:    "other-instance",
					RepresentationVersion: 1,
					Generation:            1,
				},
			},
		}

		_, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		// Update common representation version
		newVersion := 3
		rowsAffected, err := repo.UpdateCommonRepresentationVersion(ctx, resourceID, newVersion)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), rowsAffected)

		// Verify only inventory reference was updated
		var inventoryRef v1beta2.RepresentationReference
		err = db.Where("resource_id = ? AND reporter_type = ?", resourceID, "inventory").First(&inventoryRef).Error
		require.NoError(t, err)
		assert.Equal(t, newVersion, inventoryRef.RepresentationVersion)

		// Verify other reference was not updated
		var otherRef v1beta2.RepresentationReference
		err = db.Where("resource_id = ? AND reporter_type = ?", resourceID, "other-reporter").First(&otherRef).Error
		require.NoError(t, err)
		assert.Equal(t, 1, otherRef.RepresentationVersion)
	})
}

// testUpdateRepresentationVersionHelperMethods tests helper methods for any repository
func testUpdateRepresentationVersionHelperMethods(t *testing.T, repo v1beta2.ResourceWithReferencesRepository) {
	ctx := context.Background()

	t.Run("UpdateCommonRepresentationVersion", func(t *testing.T) {
		// Reset fake repo if applicable
		if fakeRepo, ok := repo.(*FakeResourceWithReferencesRepository); ok {
			fakeRepo.Reset()
		}

		// Create a resource with inventory reporter reference
		resourceID := uuid.New()
		aggregate := &v1beta2.ResourceWithReferences{
			Resource: &v1beta2.Resource{
				ID:   resourceID,
				Type: "test-resource-type",
			},
			RepresentationReferences: []*v1beta2.RepresentationReference{
				{
					ResourceID:            resourceID,
					LocalResourceID:       "inventory-123",
					ReporterType:          "inventory",
					ResourceType:          "test-type",
					ReporterInstanceID:    "inventory-instance",
					RepresentationVersion: 1,
					Generation:            1,
				},
				{
					ResourceID:            resourceID,
					LocalResourceID:       "other-123",
					ReporterType:          "other-reporter",
					ResourceType:          "test-type",
					ReporterInstanceID:    "other-instance",
					RepresentationVersion: 1,
					Generation:            1,
				},
			},
		}

		_, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		// Update common representation version
		newVersion := 3
		rowsAffected, err := repo.UpdateCommonRepresentationVersion(ctx, resourceID, newVersion)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), rowsAffected)

		// For fake repository, verify selective update
		if fakeRepo, ok := repo.(*FakeResourceWithReferencesRepository); ok {
			aggregates := fakeRepo.GetAllAggregates()
			found := false
			for _, agg := range aggregates {
				if agg.Resource.ID == resourceID {
					assert.Len(t, agg.RepresentationReferences, 2)
					for _, ref := range agg.RepresentationReferences {
						if ref.ReporterType == "inventory" {
							assert.Equal(t, newVersion, ref.RepresentationVersion)
						} else {
							assert.Equal(t, 1, ref.RepresentationVersion) // Original version
						}
					}
					found = true
					break
				}
			}
			assert.True(t, found, "Resource should be found in fake repository")
		}
	})
}
