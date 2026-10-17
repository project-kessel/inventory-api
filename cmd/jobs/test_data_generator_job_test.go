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

	err := generateResourceBatch(db, logger, start, end, 100, versionsPerResource, 60, 0, 0, false,
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

// TestHistorySkewVersions_Distribution pins the --history-skew version-count
// function: bucket proportions over a full 1000-index cycle, value bounds,
// and determinism (no RNG — reruns must generate identical data).
func TestHistorySkewVersions_Distribution(t *testing.T) {
	buckets := map[int]int{}
	total := 0
	for i := 0; i < 1000; i++ {
		n := historySkewVersions(i)
		assert.GreaterOrEqual(t, n, 1)
		assert.LessOrEqual(t, n, 50)

		switch {
		case n <= 3:
			buckets[0]++
		case n <= 7:
			buckets[1]++
		case n <= 15:
			buckets[2]++
		default:
			buckets[3]++
		}
		total += n

		// Determinism: a pure function of i.
		assert.Equal(t, n, historySkewVersions(i), "i=%d", i)
	}

	assert.Equal(t, 600, buckets[0], "60%% of resources get 1-3 versions")
	assert.Equal(t, 250, buckets[1], "25%% get 4-7")
	assert.Equal(t, 100, buckets[2], "10%% get 8-15")
	assert.Equal(t, 50, buckets[3], "5%% get 16-50")
	assert.Equal(t, 5298, total, "exact sum over one full 1000-index cycle (mean 5.298)")
}

// TestGenerateResourceBatch_HistorySkew verifies that with --history-skew the
// per-resource version counts follow historySkewVersions and the
// representation_version still points at the resource's LATEST version (Q2,
// the current+previous consumer query, reads it).
func TestGenerateResourceBatch_HistorySkew(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	const start, end = 0, 1000 // one full bucket cycle: i%1000 == i
	var createdResources, createdReporterResources, createdReporterRepresentations, createdCommonRepresentations int

	err := generateResourceBatch(db, logger, start, end, 1000, 5, 60, 0, 0, true,
		&createdResources, &createdReporterResources, &createdReporterRepresentations, &createdCommonRepresentations)
	require.NoError(t, err)

	expectedTotal := 0
	for i := start; i < end; i++ {
		numVersions := historySkewVersions(i)
		expectedTotal += numVersions

		var rr model.ReporterResource
		require.NoError(t, db.Where("local_resource_id = ?", fmt.Sprintf("test-resource-%d", i)).First(&rr).Error)
		assert.Equal(t, uint(numVersions-1), rr.RepresentationVersion,
			"resource %d: representation_version must point at the latest version", i)

		var reps int64
		db.Model(&model.CommonRepresentation{}).Where("resource_id = ?", rr.ResourceID).Count(&reps)
		assert.Equal(t, int64(numVersions), reps, "resource %d: common representation count", i)
	}

	assert.Equal(t, end-start, createdResources)
	assert.Equal(t, expectedTotal, createdCommonRepresentations,
		"total common representations must equal the sum of the skewed version counts")
	assert.Equal(t, expectedTotal, createdReporterRepresentations)
}
