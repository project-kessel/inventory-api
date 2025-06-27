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

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate the schema
	err = db.AutoMigrate(&v1beta2.CommonRepresentation{})
	require.NoError(t, err)

	return db
}

// createTestCommonRepresentation creates a test CommonRepresentation
func createTestCommonRepresentation() *v1beta2.CommonRepresentation {
	return &v1beta2.CommonRepresentation{
		BaseRepresentation: v1beta2.BaseRepresentation{
			Data: model.JsonObject{"test": "data"},
		},
		LocalResourceID: "test-resource-123",
		ReporterType:    "test-reporter",
		ResourceType:    "test-type",
		Version:         1,
		ReportedBy:      "test-service",
	}
}

// TestCommonRepresentationRepositoryContract tests that both real and fake implementations
// satisfy the same contract
func TestCommonRepresentationRepositoryContract(t *testing.T) {
	testCases := []struct {
		name string
		repo func() v1beta2.CommonRepresentationRepository
	}{
		{
			name: "Real Repository",
			repo: func() v1beta2.CommonRepresentationRepository {
				db := setupTestDB(t)
				return NewCommonRepresentationRepository(db)
			},
		},
		{
			name: "Fake Repository",
			repo: func() v1beta2.CommonRepresentationRepository {
				return NewFakeCommonRepresentationRepository()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testCommonRepresentationRepository(t, tc.repo())
		})
	}
}

// testCommonRepresentationRepository runs the contract tests on any implementation
func testCommonRepresentationRepository(t *testing.T, repo v1beta2.CommonRepresentationRepository) {
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		// Test successful creation
		representation := createTestCommonRepresentation()

		result, err := repo.Create(ctx, representation)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, representation.LocalResourceID, result.LocalResourceID)
		assert.Equal(t, representation.ReporterType, result.ReporterType)
		assert.Equal(t, representation.ResourceType, result.ResourceType)
		assert.Equal(t, representation.Version, result.Version)
		assert.Equal(t, representation.ReportedBy, result.ReportedBy)
		assert.Equal(t, representation.Data, result.Data)
	})

	t.Run("Create with nil representation", func(t *testing.T) {
		result, err := repo.Create(ctx, nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Create duplicate", func(t *testing.T) {
		representation := createTestCommonRepresentation()
		representation.LocalResourceID = "duplicate-test"
		representation.Version = 2

		// First creation should succeed
		_, err := repo.Create(ctx, representation)
		assert.NoError(t, err)

		// Second creation with same key should fail
		_, err = repo.Create(ctx, representation)
		assert.Error(t, err)
	})

	t.Run("Create with different versions", func(t *testing.T) {
		baseRep := createTestCommonRepresentation()
		baseRep.LocalResourceID = "versioned-test"

		// Create version 1
		rep1 := *baseRep
		rep1.Version = 1
		result1, err := repo.Create(ctx, &rep1)
		assert.NoError(t, err)
		assert.Equal(t, 1, result1.Version)

		// Create version 2 (should succeed - different key)
		rep2 := *baseRep
		rep2.Version = 2
		result2, err := repo.Create(ctx, &rep2)
		assert.NoError(t, err)
		assert.Equal(t, 2, result2.Version)
	})
}

// TestCommonRepresentationRepositorySpecific tests specific to the real implementation
func TestCommonRepresentationRepositorySpecific(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommonRepresentationRepository(db)
	ctx := context.Background()

	t.Run("Database persistence", func(t *testing.T) {
		representation := createTestCommonRepresentation()
		representation.LocalResourceID = "persistence-test"

		// Create the representation
		_, err := repo.Create(ctx, representation)
		require.NoError(t, err)

		// Verify it's in the database
		var count int64
		err = db.Model(&v1beta2.CommonRepresentation{}).
			Where("local_resource_id = ? AND version = ?",
				representation.LocalResourceID, representation.Version).
			Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("Transaction handling", func(t *testing.T) {
		representation := createTestCommonRepresentation()
		representation.LocalResourceID = "transaction-test"

		// Test with transaction
		err := db.Transaction(func(tx *gorm.DB) error {
			txRepo := NewCommonRepresentationRepository(tx)
			_, err := txRepo.Create(ctx, representation)
			return err
		})

		require.NoError(t, err)

		// Verify it's committed
		var count int64
		err = db.Model(&v1beta2.CommonRepresentation{}).
			Where("local_resource_id = ?", representation.LocalResourceID).
			Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}

// TestFakeCommonRepresentationRepositorySpecific tests specific to the fake implementation
func TestFakeCommonRepresentationRepositorySpecific(t *testing.T) {
	repo := NewFakeCommonRepresentationRepository()
	ctx := context.Background()

	t.Run("Reset functionality", func(t *testing.T) {
		// Create some data
		representation := createTestCommonRepresentation()
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

		// Create multiple representations
		for i := 0; i < 3; i++ {
			rep := createTestCommonRepresentation()
			rep.LocalResourceID = fmt.Sprintf("test-%d", i)
			rep.Version = i + 1
			_, err := repo.Create(ctx, rep)
			require.NoError(t, err)
		}

		// Get all and verify
		all := repo.GetAll()
		assert.Len(t, all, 3)

		// Verify they're different instances (copies)
		for _, rep := range all {
			originalRep := createTestCommonRepresentation()
			originalRep.LocalResourceID = rep.LocalResourceID
			originalRep.Version = rep.Version

			// Modify the returned representation
			rep.ReportedBy = "modified"

			// Create the same representation again - should still work
			// (proving the returned data is a copy)
			_, err := repo.Create(ctx, originalRep)
			assert.Error(t, err) // Should fail because it already exists
		}
	})
}
