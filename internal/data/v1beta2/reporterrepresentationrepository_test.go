package v1beta2

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
)

// setupReporterTestDB creates an in-memory SQLite database for testing
func setupReporterTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate the schema
	err = db.AutoMigrate(&v1beta2.ReporterRepresentation{})
	require.NoError(t, err)

	return db
}

// createTestReporterRepresentation creates a test ReporterRepresentation
func createTestReporterRepresentation() *v1beta2.ReporterRepresentation {
	return &v1beta2.ReporterRepresentation{
		BaseRepresentation: v1beta2.BaseRepresentation{
			Data: model.JsonObject{"reporter": "data"},
		},
		LocalResourceID:    "reporter-resource-123",
		ReporterType:       "test-reporter",
		ResourceType:       "test-type",
		Version:            1,
		ReporterInstanceID: "instance-123",
		Generation:         1,
		APIHref:            "https://api.example.com/resource/123",
		ConsoleHref:        "https://console.example.com/resource/123",
		CommonVersion:      1,
		Tombstone:          false,
		ReporterVersion:    "1.0.0",
	}
}

// TestReporterRepresentationRepositoryContract tests that both real and fake implementations
// satisfy the same contract
func TestReporterRepresentationRepositoryContract(t *testing.T) {
	testCases := []struct {
		name string
		repo func() v1beta2.ReporterRepresentationRepository
	}{
		{
			name: "Real Repository",
			repo: func() v1beta2.ReporterRepresentationRepository {
				db := setupReporterTestDB(t)
				return NewReporterRepresentationRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() v1beta2.ReporterRepresentationRepository {
				return NewFakeReporterRepresentationRepository()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testReporterRepresentationRepository(t, tc.repo())
		})
	}
}

// testReporterRepresentationRepository runs the contract tests on any implementation
func testReporterRepresentationRepository(t *testing.T, repo v1beta2.ReporterRepresentationRepository) {
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		// Test successful creation
		representation := createTestReporterRepresentation()

		result, err := repo.Create(ctx, representation)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, representation.LocalResourceID, result.LocalResourceID)
		assert.Equal(t, representation.ReporterType, result.ReporterType)
		assert.Equal(t, representation.ResourceType, result.ResourceType)
		assert.Equal(t, representation.Version, result.Version)
		assert.Equal(t, representation.ReporterInstanceID, result.ReporterInstanceID)
		assert.Equal(t, representation.Generation, result.Generation)
		assert.Equal(t, representation.APIHref, result.APIHref)
		assert.Equal(t, representation.ConsoleHref, result.ConsoleHref)
		assert.Equal(t, representation.CommonVersion, result.CommonVersion)
		assert.Equal(t, representation.Tombstone, result.Tombstone)
		assert.Equal(t, representation.ReporterVersion, result.ReporterVersion)
		assert.Equal(t, representation.Data, result.Data)
	})

	t.Run("Create with nil representation", func(t *testing.T) {
		result, err := repo.Create(ctx, nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Create duplicate", func(t *testing.T) {
		representation := createTestReporterRepresentation()
		representation.LocalResourceID = "duplicate-reporter-test"

		// First creation should succeed
		_, err := repo.Create(ctx, representation)
		assert.NoError(t, err)

		// Second creation with same composite key should fail
		_, err = repo.Create(ctx, representation)
		assert.Error(t, err)
	})

	t.Run("Create with different composite keys", func(t *testing.T) {
		baseRep := createTestReporterRepresentation()
		baseRep.LocalResourceID = "composite-test"

		// Same resource but different version
		rep1 := *baseRep
		rep1.Version = 1
		result1, err := repo.Create(ctx, &rep1)
		assert.NoError(t, err)
		assert.Equal(t, 1, result1.Version)

		// Same resource but different generation
		rep2 := *baseRep
		rep2.Generation = 2
		result2, err := repo.Create(ctx, &rep2)
		assert.NoError(t, err)
		assert.Equal(t, 2, result2.Generation)

		// Same resource but different reporter instance
		rep3 := *baseRep
		rep3.ReporterInstanceID = "different-instance"
		result3, err := repo.Create(ctx, &rep3)
		assert.NoError(t, err)
		assert.Equal(t, "different-instance", result3.ReporterInstanceID)
	})

	t.Run("Create with tombstone", func(t *testing.T) {
		representation := createTestReporterRepresentation()
		representation.LocalResourceID = "tombstone-test"
		representation.Tombstone = true

		result, err := repo.Create(ctx, representation)

		assert.NoError(t, err)
		assert.True(t, result.Tombstone)
	})
}

// TestReporterRepresentationRepositorySpecific tests specific to the real implementation
func TestReporterRepresentationRepositorySpecific(t *testing.T) {
	db := setupReporterTestDB(t)
	repo := NewReporterRepresentationRepository(db)
	ctx := context.Background()

	t.Run("Database persistence", func(t *testing.T) {
		representation := createTestReporterRepresentation()
		representation.LocalResourceID = "persistence-test"

		// Create the representation
		_, err := repo.Create(ctx, representation)
		require.NoError(t, err)

		// Verify it's in the database
		var count int64
		err = db.Model(&v1beta2.ReporterRepresentation{}).
			Where("local_resource_id = ? AND reporter_type = ? AND resource_type = ? AND version = ? AND reporter_instance_id = ? AND generation = ?",
				representation.LocalResourceID,
				representation.ReporterType,
				representation.ResourceType,
				representation.Version,
				representation.ReporterInstanceID,
				representation.Generation).
			Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("Primary key constraint", func(t *testing.T) {
		representation := createTestReporterRepresentation()
		representation.LocalResourceID = "pk-test"

		// Create first representation
		_, err := repo.Create(ctx, representation)
		require.NoError(t, err)

		// Try to create with same primary key - should fail
		_, err = repo.Create(ctx, representation)
		assert.Error(t, err)
	})

	t.Run("Transaction handling", func(t *testing.T) {
		representation := createTestReporterRepresentation()
		representation.LocalResourceID = "transaction-test"

		// Test with transaction
		err := db.Transaction(func(tx *gorm.DB) error {
			txRepo := NewReporterRepresentationRepository(tx)
			_, err := txRepo.Create(ctx, representation)
			return err
		})

		require.NoError(t, err)

		// Verify it's committed
		var count int64
		err = db.Model(&v1beta2.ReporterRepresentation{}).
			Where("local_resource_id = ?", representation.LocalResourceID).
			Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}

// TestFakeReporterRepresentationRepositorySpecific tests specific to the fake implementation
func TestFakeReporterRepresentationRepositorySpecific(t *testing.T) {
	repo := NewFakeReporterRepresentationRepository()
	ctx := context.Background()

	t.Run("Reset functionality", func(t *testing.T) {
		// Create some data
		representation := createTestReporterRepresentation()
		_, err := repo.Create(ctx, representation)
		require.NoError(t, err)

		// Verify it exists
		assert.Equal(t, 1, repo.Count())

		// Reset and verify it's gone
		repo.Reset()
		assert.Equal(t, 0, repo.Count())
	})

	t.Run("GetAll functionality", func(t *testing.T) {
		repo.Reset()

		// Create multiple representations with different composite keys
		for i := 0; i < 3; i++ {
			rep := createTestReporterRepresentation()
			rep.LocalResourceID = fmt.Sprintf("test-%d", i)
			rep.Version = i + 1
			rep.Generation = i + 1
			_, err := repo.Create(ctx, rep)
			require.NoError(t, err)
		}

		// Get all and verify
		all := repo.GetAll()
		assert.Len(t, all, 3)

		// Verify they're copies
		for _, rep := range all {
			// Modify the returned representation
			rep.ReporterVersion = "modified"

			// Verify original data is unchanged by creating same representation again
			originalRep := createTestReporterRepresentation()
			originalRep.LocalResourceID = rep.LocalResourceID
			originalRep.Version = rep.Version
			originalRep.Generation = rep.Generation

			_, err := repo.Create(ctx, originalRep)
			assert.Error(t, err) // Should fail because it already exists
		}
	})

	t.Run("Composite key uniqueness", func(t *testing.T) {
		repo.Reset()

		base := createTestReporterRepresentation()
		base.LocalResourceID = "composite-unique-test"

		// Create with different parts of composite key
		variations := []*v1beta2.ReporterRepresentation{
			{
				BaseRepresentation: base.BaseRepresentation,
				LocalResourceID:    base.LocalResourceID,
				ReporterType:       "different-type",
				ResourceType:       base.ResourceType,
				Version:            base.Version,
				ReporterInstanceID: base.ReporterInstanceID,
				Generation:         base.Generation,
			},
			{
				BaseRepresentation: base.BaseRepresentation,
				LocalResourceID:    base.LocalResourceID,
				ReporterType:       base.ReporterType,
				ResourceType:       "different-resource",
				Version:            base.Version,
				ReporterInstanceID: base.ReporterInstanceID,
				Generation:         base.Generation,
			},
			{
				BaseRepresentation: base.BaseRepresentation,
				LocalResourceID:    base.LocalResourceID,
				ReporterType:       base.ReporterType,
				ResourceType:       base.ResourceType,
				Version:            999,
				ReporterInstanceID: base.ReporterInstanceID,
				Generation:         base.Generation,
			},
		}

		// All should succeed as they have different composite keys
		for i, variation := range variations {
			_, err := repo.Create(ctx, variation)
			assert.NoError(t, err, "Variation %d should succeed", i)
		}

		assert.Equal(t, len(variations), repo.Count())
	})
}
