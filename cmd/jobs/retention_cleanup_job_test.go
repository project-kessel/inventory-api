package jobs

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetentionCleanupJob_DryRun(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	// Create test data with different ages
	oldDate := time.Now().AddDate(0, 0, -14) // 14 days old
	recentDate := time.Now().AddDate(0, 0, -3) // 3 days old

	// Create old reporter representation
	oldReporterResourceID := createTestReporterResource(t, db, "host", "hbi")
	oldReporterRep := createTestReporterRepresentation(t, oldReporterResourceID, 0, 0)
	oldReporterRep.CreatedAt = oldDate
	require.NoError(t, db.Create(&oldReporterRep).Error)

	// Create recent reporter representation
	recentReporterResourceID := createTestReporterResource(t, db, "host", "hbi")
	recentReporterRep := createTestReporterRepresentation(t, recentReporterResourceID, 0, 0)
	recentReporterRep.CreatedAt = recentDate
	require.NoError(t, db.Create(&recentReporterRep).Error)

	// Dry run should count old records without deleting
	cutoffDate := time.Now().AddDate(0, 0, -7)
	count, err := retentionDeleteBatchedReporterRepresentations(db, logger, true, cutoffDate, "", 1000, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "Should find 1 old reporter representation")

	// Verify no records were actually deleted
	var totalCount int64
	db.Model(&model.ReporterRepresentation{}).Count(&totalCount)
	assert.Equal(t, int64(2), totalCount, "All records should still exist after dry run")
}

func createTestReporterRepresentation(t *testing.T, reporterResourceID uuid.UUID, version uint, generation uint) *model.ReporterRepresentation {
	t.Helper()
	rep, err := model.NewReporterRepresentation(
		internal.JsonObject{"test": "data"},
		reporterResourceID,
		version,
		generation,
		nil, // commonVersion
		"",  // transactionId
		false, // tombstone
		nil, // reporterVersion
	)
	require.NoError(t, err)
	return rep
}

func TestRetentionCleanupJob_ActualDeletion(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	// Create test data with different ages
	oldDate := time.Now().AddDate(0, 0, -14) // 14 days old
	recentDate := time.Now().AddDate(0, 0, -3) // 3 days old

	// Create old reporter representation
	oldReporterResourceID := createTestReporterResource(t, db, "host", "hbi")
	oldReporterRep := createTestReporterRepresentation(t, oldReporterResourceID, 0, 0)
	oldReporterRep.CreatedAt = oldDate
	require.NoError(t, db.Create(&oldReporterRep).Error)

	// Create recent reporter representation
	recentReporterResourceID := createTestReporterResource(t, db, "host", "hbi")
	recentReporterRep := createTestReporterRepresentation(t, recentReporterResourceID, 0, 0)
	recentReporterRep.CreatedAt = recentDate
	require.NoError(t, db.Create(&recentReporterRep).Error)

	// Execute deletion for records older than 7 days
	cutoffDate := time.Now().AddDate(0, 0, -7)
	count, err := retentionDeleteBatchedReporterRepresentations(db, logger, false, cutoffDate, "", 1000, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "Should delete 1 old reporter representation")

	// Verify only old record was deleted
	var remainingCount int64
	db.Model(&model.ReporterRepresentation{}).Count(&remainingCount)
	assert.Equal(t, int64(1), remainingCount, "Only recent record should remain")

	// Verify the remaining record is the recent one
	var remaining model.ReporterRepresentation
	db.First(&remaining)
	assert.Equal(t, recentReporterResourceID, remaining.ReporterResourceID)
}

func TestRetentionCleanupJob_WithReporterTypeFilter(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	oldDate := time.Now().AddDate(0, 0, -14)

	// Create old reporter representation with reporter_type "hbi"
	hbiReporterResourceID := createTestReporterResource(t, db, "host", "hbi")
	hbiReporterRep := createTestReporterRepresentation(t, hbiReporterResourceID, 0, 0)
	hbiReporterRep.CreatedAt = oldDate
	require.NoError(t, db.Create(&hbiReporterRep).Error)

	// Create old reporter representation with reporter_type "ocm"
	ocmReporterResourceID := createTestReporterResource(t, db, "k8s-cluster", "ocm")
	ocmReporterRep := createTestReporterRepresentation(t, ocmReporterResourceID, 0, 0)
	ocmReporterRep.CreatedAt = oldDate
	require.NoError(t, db.Create(&ocmReporterRep).Error)

	// Delete only "hbi" reporter representations
	cutoffDate := time.Now().AddDate(0, 0, -7)
	count, err := retentionDeleteBatchedReporterRepresentations(db, logger, false, cutoffDate, "hbi", 1000, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "Should delete only 1 hbi reporter representation")

	// Verify only hbi record was deleted
	var remainingCount int64
	db.Model(&model.ReporterRepresentation{}).Count(&remainingCount)
	assert.Equal(t, int64(1), remainingCount, "Only ocm record should remain")

	// Verify the remaining record is the ocm one
	var remaining model.ReporterRepresentation
	db.First(&remaining)
	assert.Equal(t, ocmReporterResourceID, remaining.ReporterResourceID)
}

func TestRetentionCleanupJob_CommonRepresentations(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	oldDate := time.Now().AddDate(0, 0, -14)
	recentDate := time.Now().AddDate(0, 0, -3)

	// Create resources
	oldResourceID := uuid.New()
	recentResourceID := uuid.New()

	oldResource := model.Resource{ID: oldResourceID, Type: "host"}
	recentResource := model.Resource{ID: recentResourceID, Type: "host"}
	require.NoError(t, db.Create(&oldResource).Error)
	require.NoError(t, db.Create(&recentResource).Error)

	// Create old common representation
	oldCommonRep, err := model.NewCommonRepresentation(
		oldResourceID,
		internal.JsonObject{"common": "old_data"},
		0,
		"hbi",
		"instance-123",
		"",
	)
	require.NoError(t, err)
	oldCommonRep.CreatedAt = oldDate
	require.NoError(t, db.Create(&oldCommonRep).Error)

	// Create recent common representation
	recentCommonRep, err := model.NewCommonRepresentation(
		recentResourceID,
		internal.JsonObject{"common": "recent_data"},
		0,
		"hbi",
		"instance-123",
		"",
	)
	require.NoError(t, err)
	recentCommonRep.CreatedAt = recentDate
	require.NoError(t, db.Create(&recentCommonRep).Error)

	// Delete old common representations
	cutoffDate := time.Now().AddDate(0, 0, -7)
	count, err := retentionDeleteBatchedCommonRepresentations(db, logger, false, cutoffDate, "", 1000, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "Should delete 1 old common representation")

	// Verify only recent record remains
	var remainingCount int64
	db.Model(&model.CommonRepresentation{}).Count(&remainingCount)
	assert.Equal(t, int64(1), remainingCount, "Only recent record should remain")
}

func TestRetentionCleanupJob_BatchProcessing(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	oldDate := time.Now().AddDate(0, 0, -14)

	// Create 25 old reporter representations
	for i := 0; i < 25; i++ {
		reporterResourceID := createTestReporterResource(t, db, "host", "hbi")
		rep := createTestReporterRepresentation(t, reporterResourceID, uint(i), 0)
		rep.CreatedAt = oldDate
		require.NoError(t, db.Create(&rep).Error)
	}

	// Delete in small batches of 10
	cutoffDate := time.Now().AddDate(0, 0, -7)
	count, err := retentionDeleteBatchedReporterRepresentations(db, logger, false, cutoffDate, "", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(25), count, "Should delete all 25 records across multiple batches")

	// Verify all records were deleted
	var remainingCount int64
	db.Model(&model.ReporterRepresentation{}).Count(&remainingCount)
	assert.Equal(t, int64(0), remainingCount, "All old records should be deleted")
}
