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
		// Create test data
		aggregate := createTestResourceWithReferences()
		_, err := repo.Create(ctx, aggregate)
		require.NoError(t, err)

		// Test the search
		reporterId := createTestReporterRepresentationId()
		refs, err := repo.FindAllReferencesByReporterRepresentationId(ctx, reporterId)

		require.NoError(t, err)
		assert.NotEmpty(t, refs)

		// Verify all references belong to the same resource
		if len(refs) > 0 {
			resourceID := refs[0].ResourceID
			for _, ref := range refs {
				assert.Equal(t, resourceID, ref.ResourceID)
			}
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
