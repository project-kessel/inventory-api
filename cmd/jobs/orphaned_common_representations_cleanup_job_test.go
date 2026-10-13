package jobs

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// createTestRepresentationSet creates a resource with the given number of common_representations
// versions (versions 0..versions-1, reported by repReporterType) and one reporter_resources row
// per entry in rrReporterTypes. Calling it with no rrReporterTypes creates an orphaned set: a
// resource whose representations no longer have any reporter_resources rows referencing them.
func createTestRepresentationSet(t *testing.T, db *gorm.DB, repReporterType string, versions int, rrReporterTypes ...string) uuid.UUID {
	t.Helper()

	resourceID := uuid.New()

	resource := model.Resource{
		ID:   resourceID,
		Type: "host",
	}
	require.NoError(t, db.Create(&resource).Error)

	for _, rrType := range rrReporterTypes {
		reporterResource := model.ReporterResource{
			ID: uuid.New(),
			ReporterResourceKey: model.ReporterResourceKey{
				LocalResourceID:    "local-" + resourceID.String() + "-" + rrType,
				ReporterType:       rrType,
				ResourceType:       "host",
				ReporterInstanceID: "instance-123",
			},
			ResourceID:            resourceID,
			APIHref:               "https://api.example.com/resource/" + resourceID.String(),
			RepresentationVersion: uint(versions - 1),
			Generation:            1,
			Tombstone:             false,
		}
		require.NoError(t, db.Create(&reporterResource).Error)
	}

	for v := 0; v < versions; v++ {
		commonRep := model.CommonRepresentation{
			ResourceId:                 resourceID,
			Version:                    uint(v),
			ReportedByReporterType:     repReporterType,
			ReportedByReporterInstance: "instance-123",
		}
		require.NoError(t, db.Create(&commonRep).Error)
	}

	return resourceID
}

func TestDeleteOrphanedCommonRepresentations_DryRun(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	// 3 orphaned resources (2 versions each) + 2 live resources (2 versions each)
	for i := 0; i < 3; i++ {
		createTestRepresentationSet(t, db, "hbi", 2)
	}
	for i := 0; i < 2; i++ {
		createTestRepresentationSet(t, db, "hbi", 2, "hbi")
	}

	count, _, err := deleteOrphanedCommonRepresentations(db, logger, true, "", 100, 0, 0)

	assert.NoError(t, err)
	assert.Equal(t, int64(6), count)

	var remaining int64
	db.Model(&model.CommonRepresentation{}).Count(&remaining)
	assert.Equal(t, int64(10), remaining, "dry-run should not delete any records")
}

func TestDeleteOrphanedCommonRepresentations_ActualDeletion(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	var orphanIDs, liveIDs []uuid.UUID
	for i := 0; i < 3; i++ {
		orphanIDs = append(orphanIDs, createTestRepresentationSet(t, db, "hbi", 2))
	}
	for i := 0; i < 2; i++ {
		liveIDs = append(liveIDs, createTestRepresentationSet(t, db, "hbi", 2, "hbi"))
	}

	deleted, timedOut, err := deleteOrphanedCommonRepresentations(db, logger, false, "", 100, 0, 0)

	assert.NoError(t, err)
	assert.False(t, timedOut, "a run that deletes everything must not report a timeout")
	assert.Equal(t, int64(6), deleted)

	for _, id := range orphanIDs {
		var n int64
		db.Model(&model.CommonRepresentation{}).Where("resource_id = ?", id).Count(&n)
		assert.Equal(t, int64(0), n, "orphaned resource %s should be fully deleted", id)
	}
	for _, id := range liveIDs {
		var n int64
		db.Model(&model.CommonRepresentation{}).Where("resource_id = ?", id).Count(&n)
		assert.Equal(t, int64(2), n, "live resource %s must keep all versions", id)
	}
}

func TestDeleteOrphanedCommonRepresentations_MultiReporterResourceSurvives(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	// A resource reported by two reporters. When one reporter_resources row is deleted
	// (as the tombstoned-resources cleanup job does per reporter resource), the resource
	// still has a live reporter row, so its representations must survive.
	createTestRepresentationSet(t, db, "hbi", 3, "hbi", "ocm")
	var removed model.ReporterResource
	require.NoError(t, db.Where("reporter_type = ?", "ocm").First(&removed).Error)
	require.NoError(t, db.Delete(&removed).Error)

	deleted, _, err := deleteOrphanedCommonRepresentations(db, logger, false, "", 100, 0, 0)

	assert.NoError(t, err)
	assert.Equal(t, int64(0), deleted)

	var remaining int64
	db.Model(&model.CommonRepresentation{}).Count(&remaining)
	assert.Equal(t, int64(3), remaining)
}

func TestDeleteOrphanedCommonRepresentations_FiltersByReporterType(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	createTestRepresentationSet(t, db, "hbi", 2) // orphan, reported by hbi
	createTestRepresentationSet(t, db, "ocm", 2) // orphan, reported by ocm

	deleted, _, err := deleteOrphanedCommonRepresentations(db, logger, false, "hbi", 100, 0, 0)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), deleted)

	var remaining int64
	db.Model(&model.CommonRepresentation{}).Count(&remaining)
	assert.Equal(t, int64(2), remaining, "orphaned representations of other reporters must be untouched")
}

func TestDeleteOrphanedCommonRepresentations_EmptyBatchTermination(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	// No orphans at all: every resource still has a reporter row
	for i := 0; i < 3; i++ {
		createTestRepresentationSet(t, db, "hbi", 2, "hbi")
	}

	deleted, _, err := deleteOrphanedCommonRepresentations(db, logger, false, "", 100, 0, 0)

	assert.NoError(t, err)
	assert.Equal(t, int64(0), deleted)

	var remaining int64
	db.Model(&model.CommonRepresentation{}).Count(&remaining)
	assert.Equal(t, int64(6), remaining)
}

func TestDeleteOrphanedCommonRepresentations_BatchAccounting(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	// 7 orphaned versions with batch size 2 -> deletes across multiple batches
	// (2+2+2+1), exercising the temp-table drain between batches
	for i := 0; i < 2; i++ {
		createTestRepresentationSet(t, db, "hbi", 2)
	}
	createTestRepresentationSet(t, db, "hbi", 3)

	deleted, _, err := deleteOrphanedCommonRepresentations(db, logger, false, "", 2, 0, 0)

	assert.NoError(t, err)
	assert.Equal(t, int64(7), deleted)

	var remaining int64
	db.Model(&model.CommonRepresentation{}).Count(&remaining)
	assert.Equal(t, int64(0), remaining)
}

func TestDeleteOrphanedCommonRepresentations_TimeoutStopsGracefully(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	// 3 orphaned resources (2 versions each)
	for i := 0; i < 3; i++ {
		createTestRepresentationSet(t, db, "hbi", 2)
	}

	// 1ns timeout: the deadline has certainly passed by the time the first
	// batch-loop check runs (the collect phase does several DB round trips in
	// between), so the job must stop before deleting anything and report a
	// partial, resumable run rather than an error.
	deleted, timedOut, err := deleteOrphanedCommonRepresentations(db, logger, false, "", 100, 0, time.Nanosecond)

	assert.NoError(t, err, "a graceful timeout must not surface as an error")
	assert.True(t, timedOut, "the run should be reported as partial (timed out)")
	assert.Equal(t, int64(0), deleted)

	var remaining int64
	db.Model(&model.CommonRepresentation{}).Count(&remaining)
	assert.Equal(t, int64(6), remaining, "timeout must stop before any deletion — the run is resumable")
}
