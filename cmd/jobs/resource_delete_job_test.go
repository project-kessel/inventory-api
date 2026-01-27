package jobs

import (
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testLogger() *log.Helper {
	return log.NewHelper(log.NewStdLogger(io.Discard))
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.NoError(t, err)

	err = data.Migrate(db, testLogger())
	require.NoError(t, err)

	return db
}

func createTestReporterResource(t *testing.T, db *gorm.DB, resourceType, reporterType string) uuid.UUID {
	t.Helper()

	resourceID := uuid.New()
	reporterResourceID := uuid.New()

	resource := model.Resource{
		ID:   resourceID,
		Type: resourceType,
	}
	require.NoError(t, db.Create(&resource).Error)

	reporterResource := model.ReporterResource{
		ID: reporterResourceID,
		ReporterResourceKey: model.ReporterResourceKey{
			LocalResourceID:    "local-" + resourceID.String(),
			ReporterType:       reporterType,
			ResourceType:       resourceType,
			ReporterInstanceID: "instance-123",
		},
		ResourceID:            resourceID,
		APIHref:               "https://api.example.com/resource/" + resourceID.String(),
		ConsoleHref:           "https://console.example.com/resource/" + resourceID.String(),
		RepresentationVersion: 1,
		Generation:            1,
		Tombstone:             false,
	}
	require.NoError(t, db.Create(&reporterResource).Error)

	return reporterResourceID
}

func createTestCommonRepresentation(t *testing.T, db *gorm.DB, reporterType string) uuid.UUID {
	t.Helper()

	resourceID := uuid.New()

	resource := model.Resource{
		ID:   resourceID,
		Type: "host",
	}
	require.NoError(t, db.Create(&resource).Error)

	commonRep := model.CommonRepresentation{
		ResourceId:                 resourceID,
		Version:                    1,
		ReportedByReporterType:     reporterType,
		ReportedByReporterInstance: "instance-123",
	}
	require.NoError(t, db.Create(&commonRep).Error)

	return resourceID
}

func createTestResource(t *testing.T, db *gorm.DB, resourceType string) uuid.UUID {
	t.Helper()

	resourceID := uuid.New()

	resource := model.Resource{
		ID:   resourceID,
		Type: resourceType,
	}
	require.NoError(t, db.Create(&resource).Error)

	return resourceID
}

func TestDeleteBatchedReporterResources_DryRun(t *testing.T) {
	logger := testLogger()

	tests := []struct {
		name          string
		setupRecords  int
		resourceType  string
		reporterType  string
		expectedCount int64
	}{
		{
			name:          "no matching records",
			setupRecords:  0,
			resourceType:  "host",
			reporterType:  "hbi",
			expectedCount: 0,
		},
		{
			name:          "single record",
			setupRecords:  1,
			resourceType:  "host",
			reporterType:  "hbi",
			expectedCount: 1,
		},
		{
			name:          "multiple records",
			setupRecords:  10,
			resourceType:  "host",
			reporterType:  "hbi",
			expectedCount: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)

			for i := 0; i < tt.setupRecords; i++ {
				createTestReporterResource(t, db, tt.resourceType, tt.reporterType)
			}

			count, err := deleteBatchedReporterResources(db, logger, true, tt.resourceType, tt.reporterType, 100, 0)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)

			var remaining int64
			db.Model(&model.ReporterResource{}).
				Where("resource_type = ? AND reporter_type = ?", tt.resourceType, tt.reporterType).
				Count(&remaining)
			assert.Equal(t, tt.expectedCount, remaining, "dry-run should not delete any records")
		})
	}
}

func TestDeleteBatchedReporterResources_ActualDeletion(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	const recordCount = 10
	resourceType := "host"
	reporterType := "hbi"

	for i := 0; i < recordCount; i++ {
		createTestReporterResource(t, db, resourceType, reporterType)
	}

	var initialCount int64
	db.Model(&model.ReporterResource{}).
		Where("resource_type = ? AND reporter_type = ?", resourceType, reporterType).
		Count(&initialCount)
	assert.Equal(t, int64(recordCount), initialCount)

	deletedCount, err := deleteBatchedReporterResources(db, logger, false, resourceType, reporterType, 100, 0)

	assert.NoError(t, err)
	assert.Equal(t, int64(recordCount), deletedCount)

	var remainingCount int64
	db.Model(&model.ReporterResource{}).
		Where("resource_type = ? AND reporter_type = ?", resourceType, reporterType).
		Count(&remainingCount)
	assert.Equal(t, int64(0), remainingCount)
}

func TestDeleteBatchedReporterResources_FiltersByType(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	createTestReporterResource(t, db, "host", "hbi")
	createTestReporterResource(t, db, "host", "ocm")
	createTestReporterResource(t, db, "k8s-cluster", "hbi")

	deletedCount, err := deleteBatchedReporterResources(db, logger, false, "host", "hbi", 100, 0)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), deletedCount)

	var remainingCount int64
	db.Model(&model.ReporterResource{}).Count(&remainingCount)
	assert.Equal(t, int64(2), remainingCount)
}

func TestDeleteBatchedCommonRepresentations_DryRun(t *testing.T) {
	logger := testLogger()

	tests := []struct {
		name          string
		setupRecords  int
		reporterType  string
		expectedCount int64
	}{
		{
			name:          "no matching records",
			setupRecords:  0,
			reporterType:  "hbi",
			expectedCount: 0,
		},
		{
			name:          "single record",
			setupRecords:  1,
			reporterType:  "hbi",
			expectedCount: 1,
		},
		{
			name:          "multiple records",
			setupRecords:  10,
			reporterType:  "hbi",
			expectedCount: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)

			for i := 0; i < tt.setupRecords; i++ {
				createTestCommonRepresentation(t, db, tt.reporterType)
			}

			count, err := deleteBatchedCommonRepresentations(db, logger, true, tt.reporterType, 100, 0)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)

			var remaining int64
			db.Model(&model.CommonRepresentation{}).
				Where("reported_by_reporter_type = ?", tt.reporterType).
				Count(&remaining)
			assert.Equal(t, tt.expectedCount, remaining, "dry-run should not delete any records")
		})
	}
}

func TestDeleteBatchedResources_DryRun(t *testing.T) {
	logger := testLogger()

	tests := []struct {
		name          string
		setupRecords  int
		resourceType  string
		expectedCount int64
	}{
		{
			name:          "no matching records",
			setupRecords:  0,
			resourceType:  "host",
			expectedCount: 0,
		},
		{
			name:          "single record",
			setupRecords:  1,
			resourceType:  "host",
			expectedCount: 1,
		},
		{
			name:          "multiple records",
			setupRecords:  10,
			resourceType:  "host",
			expectedCount: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)

			for i := 0; i < tt.setupRecords; i++ {
				createTestResource(t, db, tt.resourceType)
			}

			count, err := deleteBatchedResources(db, logger, true, tt.resourceType, 100, 0)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)

			var remaining int64
			db.Model(&model.Resource{}).
				Where("type = ?", tt.resourceType).
				Count(&remaining)
			assert.Equal(t, tt.expectedCount, remaining, "dry-run should not delete any records")
		})
	}
}

func TestDeleteBatchedResources_ActualDeletion(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	const recordCount = 10
	resourceType := "host"

	for i := 0; i < recordCount; i++ {
		createTestResource(t, db, resourceType)
	}

	var initialCount int64
	db.Model(&model.Resource{}).
		Where("type = ?", resourceType).
		Count(&initialCount)
	assert.Equal(t, int64(recordCount), initialCount)

	deletedCount, err := deleteBatchedResources(db, logger, false, resourceType, 100, 0)

	assert.NoError(t, err)
	assert.Equal(t, int64(recordCount), deletedCount)

	var remainingCount int64
	db.Model(&model.Resource{}).
		Where("type = ?", resourceType).
		Count(&remainingCount)
	assert.Equal(t, int64(0), remainingCount)
}

func TestDeleteBatchedResources_FiltersByResourceType(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	createTestResource(t, db, "host")
	createTestResource(t, db, "k8s-cluster")
	createTestResource(t, db, "host")

	deletedCount, err := deleteBatchedResources(db, logger, false, "host", 100, 0)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), deletedCount)

	var remainingCount int64
	db.Model(&model.Resource{}).Count(&remainingCount)
	assert.Equal(t, int64(1), remainingCount)
}

func TestDeleteBatchedReporterResources_EmptyBatchTermination(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	deletedCount, err := deleteBatchedReporterResources(db, logger, false, "nonexistent-type", "nonexistent-reporter", 100, 0)

	assert.NoError(t, err)
	assert.Equal(t, int64(0), deletedCount)
}
