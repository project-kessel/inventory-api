package jobs

import (
	"fmt"
	"testing"

	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateResourceBatch_SetsRepresentationVersionToLatest verifies that the
// reporter_resource's representation_version points at the resource's LATEST version
// (numVersions-1), not 0. The performance test's FindCurrentAndPreviousVersionedRepresentations
// query (Q2) reads this version to look up current + previous representations; a value of 0
// degenerates that query into version = 0 lookups instead of testing the current+previous path.
func TestGenerateResourceBatch_SetsRepresentationVersionToLatest(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	const start, end = 0, 6
	versionsPerResource := 5
	var createdResources, createdReporterResources, createdReporterRepresentations, createdCommonRepresentations int

	err := generateResourceBatch(db, logger, start, end, 100, versionsPerResource, 60, 0, 0,
		&createdResources, &createdReporterResources, &createdReporterRepresentations, &createdCommonRepresentations)
	require.NoError(t, err)

	for i := start; i < end; i++ {
		numVersions := versionsPerResource + (i % 3) - 1
		expectedLatest := uint(numVersions - 1)

		var rr model.ReporterResource
		require.NoError(t, db.Where("local_resource_id = ?", fmt.Sprintf("test-resource-%d", i)).First(&rr).Error)
		assert.Equal(t, expectedLatest, rr.RepresentationVersion,
			"resource %d: representation_version must point at the latest version", i)

		// Both the current and the previous version must exist as common representations,
		// so the current+previous query finds real rows
		var current, previous int64
		db.Model(&model.CommonRepresentation{}).
			Where("resource_id = ? AND version = ?", rr.ResourceID, expectedLatest).
			Count(&current)
		assert.Equal(t, int64(1), current, "resource %d: current version must exist", i)
		db.Model(&model.CommonRepresentation{}).
			Where("resource_id = ? AND version = ?", rr.ResourceID, expectedLatest-1).
			Count(&previous)
		assert.Equal(t, int64(1), previous, "resource %d: previous version must exist", i)
	}

	assert.Equal(t, end-start, createdResources)
	assert.Equal(t, end-start, createdReporterResources)
}
