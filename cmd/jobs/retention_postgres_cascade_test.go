package jobs

import (
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Tombstone-cascade behavioral tests: deleteOldTombstonedResourcesCascade must
// remove an old-tombstoned resource's reporter_resources row (cascade removes
// its reporter_representations) AND its common_representations in the same
// run, while (a) never touching resources that still have a non-eligible
// reporter row, (b) never deleting the parent resource row, and (c) honoring
// the selected deletion order.
//
// Eligibility: tombstone = true AND updated_at older than the tombstone-days
// cutoff. A common rep dies only when NO reporter_resources row remains for
// its resource (anti-join on resource_id, re-checked per batch).

func TestPostgresCascade_OnlyOldTombstonesDeleted(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	// Three single-reporter resources: old tombstone, recent tombstone, active.
	oldID, oldResource := uuid.New(), uuid.New()
	recentID, recentResource := uuid.New(), uuid.New()
	activeID, activeResource := uuid.New(), uuid.New()
	for _, f := range []struct {
		id, res  uuid.UUID
		tomb     bool
		ageDays  int
		reporter string
	}{
		{oldID, oldResource, true, 40, "hbi"},
		{recentID, recentResource, true, 5, "hbi"},
		{activeID, activeResource, false, 5, "hbi"},
	} {
		createPGResource(t, db, f.res)
		createPGReporterResource(t, db, f.id, f.res, f.reporter, f.tomb, daysAgo(now, 60), daysAgo(now, f.ageDays))
		createPGCommonRepresentation(t, db, f.res, 0, f.reporter, daysAgo(now, 40))
	}

	result, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 10, 10, 0, 0, nil)
	require.NoError(t, err)
	assert.Empty(t, result.StopReason)

	assert.Equal(t, int64(1), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "id = ?", oldID),
		"old tombstones must be deleted")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", recentID),
		"recent tombstones must survive")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", activeID),
		"active resources must survive")
}

func TestPostgresCascade_DeletesRepsAndCommonReps_ParentResourceSurvives(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	rrID := uuid.New()
	createPGResource(t, db, resourceID)
	createPGReporterResource(t, db, rrID, resourceID, "hbi", true, daysAgo(now, 60), daysAgo(now, 40))
	createPGReporterRepresentation(t, db, rrID, 0, 1, daysAgo(now, 45))
	createPGReporterRepresentation(t, db, rrID, 1, 1, daysAgo(now, 41))
	createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 45))
	createPGCommonRepresentation(t, db, resourceID, 1, "hbi", daysAgo(now, 41))

	result, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 10, 10, 0, 0, nil)
	require.NoError(t, err)
	assert.Empty(t, result.StopReason)

	assert.Equal(t, int64(1), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(2), result.CommonRepresentationsDeleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "resource_id = ?", resourceID),
		"the tombstoned reporter_resources row must be gone")
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ?", rrID),
		"reporter_representations must be removed by the DB cascade")
	assert.Equal(t, int64(0), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID),
		"all common_representations of the fully-deleted resource must be gone")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.Resource{}, "id = ?", resourceID),
		"the parent resource row must survive (known TODO, same as the tombstone job)")
}

func TestPostgresCascade_MultiReporter_ActiveReporterSurvives(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	// One resource reported by HBI (old-tombstoned) and OCM (active): the
	// HBI row is deleted, but the OCM row keeps ALL common reps alive.
	resourceID := uuid.New()
	hbiID, ocmID := uuid.New(), uuid.New()
	createPGResource(t, db, resourceID)
	createPGReporterResource(t, db, hbiID, resourceID, "hbi", true, daysAgo(now, 60), daysAgo(now, 40))
	createPGReporterResource(t, db, ocmID, resourceID, "ocm", false, daysAgo(now, 60), daysAgo(now, 3))
	createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 45))
	createPGCommonRepresentation(t, db, resourceID, 1, "ocm", daysAgo(now, 10))

	result, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 10, 10, 0, 0, nil)
	require.NoError(t, err)
	assert.Empty(t, result.StopReason)

	assert.Equal(t, int64(1), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(0), result.CommonRepresentationsDeleted,
		"a resource with a surviving active reporter must keep all its common reps")
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "id = ?", hbiID))
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", ocmID))
	assert.Equal(t, int64(2), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID))
}

// Two old-tombstoned reporter rows for the SAME resource, forced into
// different batches (batchSize=1): the common reps may die only after the
// SECOND deletion — this pins that the orphan anti-join is re-evaluated per
// batch against live data, not decided once at collect time.
func TestPostgresCascade_MultiReporter_BothOldAcrossBatches(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	hbiID, ocmID := uuid.New(), uuid.New()
	createPGResource(t, db, resourceID)
	createPGReporterResource(t, db, hbiID, resourceID, "hbi", true, daysAgo(now, 60), daysAgo(now, 45))
	createPGReporterResource(t, db, ocmID, resourceID, "ocm", true, daysAgo(now, 60), daysAgo(now, 40))
	for v := 0; v < 3; v++ {
		createPGCommonRepresentation(t, db, resourceID, uint(v), "hbi", daysAgo(now, 45-v))
	}

	// Run 1: --max-batches 1 deletes exactly one reporter row (the older
	// HBI one, oldest-first). The OCM row survives, so NO common rep may die.
	result, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 1, 10, 0, 0, &retentionRunOptions{MaxBatches: 1})
	require.NoError(t, err)
	assert.Equal(t, stopReasonMaxBatches, result.StopReason)
	assert.Equal(t, int64(1), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(0), result.CommonRepresentationsDeleted,
		"the per-batch anti-join must hold off while the OCM reporter row survives")
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "id = ?", hbiID))
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", ocmID))
	assert.Equal(t, int64(3), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID))

	// Run 2: completes the resource. Now the anti-join passes and the
	// interleaved orphan pass removes all three common reps.
	result, err = deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 1, 10, 0, 0, nil)
	require.NoError(t, err)
	assert.Empty(t, result.StopReason)
	assert.Equal(t, int64(1), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(3), result.CommonRepresentationsDeleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "resource_id = ?", resourceID))
	assert.Equal(t, int64(0), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID))
	assert.Equal(t, int64(1), countPGRows(t, db, &model.Resource{}, "id = ?", resourceID))
}

func TestPostgresCascade_OldestFirst(t *testing.T) {
	db, observerDB := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	// 8 old-tombstoned resources, inserted newest-first by updated_at.
	for _, ageDays := range []int{36, 38, 40, 42, 44, 46, 48, 50} {
		id := uuid.New()
		resourceID := uuid.New()
		createPGResource(t, db, resourceID)
		createPGReporterResource(t, db, id, resourceID, "hbi", true, daysAgo(now, 60), daysAgo(now, ageDays))
		createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, ageDays))
	}

	// Keepers: a recent tombstone and an active resource, both with reps.
	recentTombstone, recentResource := uuid.New(), uuid.New()
	createPGResource(t, db, recentResource)
	createPGReporterResource(t, db, recentTombstone, recentResource, "hbi", true, daysAgo(now, 60), daysAgo(now, 5))
	createPGCommonRepresentation(t, db, recentResource, 0, "hbi", daysAgo(now, 5))
	activeID, activeResource := uuid.New(), uuid.New()
	createPGResource(t, db, activeResource)
	createPGReporterResource(t, db, activeID, activeResource, "hbi", false, daysAgo(now, 60), daysAgo(now, 5))
	createPGCommonRepresentation(t, db, activeResource, 0, "hbi", daysAgo(now, 5))

	sample := func() map[string]time.Time {
		rows := []struct {
			ID        uuid.UUID `gorm:"column:id"`
			UpdatedAt time.Time `gorm:"column:updated_at"`
		}{}
		require.NoError(t, observerDB.Raw("SELECT id, updated_at FROM reporter_resources WHERE tombstone = true AND updated_at < ?", daysAgo(now, 20)).Scan(&rows).Error)
		out := make(map[string]time.Time, len(rows))
		for _, r := range rows {
			out[r.ID.String()] = r.UpdatedAt
		}
		return out
	}

	var result cascadeCleanupResult
	var err error
	runWithOldestFirstCheck(t, func() {
		result, err = deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 2, 10, 300, 0, nil)
	}, sample)

	require.NoError(t, err)
	assert.Empty(t, result.StopReason)
	assert.Equal(t, int64(8), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(8), result.CommonRepresentationsDeleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", recentTombstone),
		"recent tombstones must survive")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", activeID),
		"active resources must survive")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", recentResource))
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", activeResource))
}

// runWithLargestFirstCheck is the largest-history-first analogue of
// runWithOldestFirstCheck: while the run is in progress, no deleted row may
// have a SMALLER history size than a still-remaining one (sizes are distinct
// in the fixtures that use this, so the invariant is strict).
func runWithLargestFirstCheck(t *testing.T, run func(), sample func() map[string]int) {
	t.Helper()

	initial := sample()
	require.NotEmpty(t, initial, "fixture must produce deletable rows")

	done := make(chan struct{})
	go func() {
		defer close(done)
		run()
	}()

	ticker := time.NewTicker(15 * time.Millisecond)
	defer ticker.Stop()

	observedPartial := false
	for {
		select {
		case <-done:
			if !observedPartial {
				t.Log("warning: no partial-deletion state was observed; ordering was not exercised")
			}
			return
		case <-ticker.C:
			current := sample()
			if len(current) == len(initial) || len(current) == 0 {
				continue
			}
			observedPartial = true
			for deletedKey, deletedSize := range initial {
				if _, stillPresent := current[deletedKey]; stillPresent {
					continue
				}
				for remainingKey, remainingSize := range current {
					if deletedSize < remainingSize {
						t.Errorf("largest-history-first violated: deleted row %s (history %d) is smaller than remaining row %s (history %d)",
							deletedKey, deletedSize, remainingKey, remainingSize)
					}
				}
			}
		}
	}
}

func TestPostgresCascade_LargestHistoryFirst(t *testing.T) {
	db, observerDB := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	// 8 old-tombstoned resources with DISTINCT history sizes 1..8. The
	// tombstone age is ANTI-correlated with the history size (biggest
	// history = newest tombstone), so an implementation that ignores
	// history_size and orders by age alone deletes the smallest histories
	// first and violates the invariant deterministically.
	//
	// The size map is FROZEN before the run: live counts shrink as the job
	// deletes, so the poller cannot re-count mid-run.
	frozenSizes := make(map[string]int, 8)
	for i := 0; i < 8; i++ {
		size := i + 1
		ageDays := 36 + 2*(7-i) // size 8 -> age 36, size 1 -> age 50
		id := uuid.New()
		resourceID := uuid.New()
		createPGResource(t, db, resourceID)
		createPGReporterResource(t, db, id, resourceID, "hbi", true, daysAgo(now, 60), daysAgo(now, ageDays))
		for v := 0; v < size; v++ {
			createPGCommonRepresentation(t, db, resourceID, uint(v), "hbi", daysAgo(now, ageDays))
		}
		frozenSizes[id.String()] = size
	}

	sample := func() map[string]int {
		rows := []struct {
			ID uuid.UUID `gorm:"column:id"`
		}{}
		require.NoError(t, observerDB.Raw("SELECT id FROM reporter_resources WHERE tombstone = true AND updated_at < ?", daysAgo(now, 20)).Scan(&rows).Error)
		out := make(map[string]int, len(rows))
		for _, r := range rows {
			out[r.ID.String()] = frozenSizes[r.ID.String()]
		}
		return out
	}

	var result cascadeCleanupResult
	var err error
	runWithLargestFirstCheck(t, func() {
		result, err = deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderLargestHistoryFirst, "", 1, 10, 300, 0, nil)
	}, sample)

	require.NoError(t, err)
	assert.Empty(t, result.StopReason)
	assert.Equal(t, int64(8), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(36), result.CommonRepresentationsDeleted, "sum of history sizes 1..8")
}

// Within equal history sizes, the older tombstone dies first (the
// updated_at tiebreak of the largest-history-first order).
func TestPostgresCascade_LargestHistoryFirst_AgeTiebreak(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	olderID, olderResource := uuid.New(), uuid.New()
	newerID, newerResource := uuid.New(), uuid.New()
	for _, f := range []struct {
		id, res uuid.UUID
		ageDays int
	}{
		{olderID, olderResource, 45},
		{newerID, newerResource, 40},
	} {
		createPGResource(t, db, f.res)
		createPGReporterResource(t, db, f.id, f.res, "hbi", true, daysAgo(now, 60), daysAgo(now, f.ageDays))
		createPGCommonRepresentation(t, db, f.res, 0, "hbi", daysAgo(now, f.ageDays))
		createPGCommonRepresentation(t, db, f.res, 1, "hbi", daysAgo(now, f.ageDays))
	}

	// batchSize 1 + MaxBatches 1: exactly one deletion, which must be the
	// OLDER of the two equal-history resources. Its common reps die in the
	// interleaved orphan pass; the newer resource is untouched.
	result, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderLargestHistoryFirst, "", 1, 10, 0, 0, &retentionRunOptions{MaxBatches: 1})
	require.NoError(t, err)
	assert.Equal(t, stopReasonMaxBatches, result.StopReason)
	assert.Equal(t, int64(1), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "id = ?", olderID),
		"the older tombstone of the equal-history pair must be the one deleted")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", newerID))
	assert.Equal(t, int64(0), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", olderResource),
		"the interleaved orphan pass must have cleaned the deleted resource's reps")
	assert.Equal(t, int64(2), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", newerResource))
}

func TestPostgresCascade_DryRun(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	// Fixture:
	//   res1: hbi old tomb, 2 reps           — eligible (both scopes)
	//   res2: hbi old tomb, 3 reps           — eligible (both scopes)
	//   res3: hbi old tomb + ocm ACTIVE, 2   — blocked: a reporter row survives
	//   res4: hbi RECENT tomb, 2 reps        — not eligible (30-day cutoff)
	//   res5: ocm old tomb, 1 rep            — eligible unscoped, blocked when
	//                                           scoped to hbi (its ocm row is
	//                                           not in the hbi worklist)
	//   res6: no reporter_resources at all, 2 — pre-existing orphan; the real
	//                                           run's unscoped final drain
	//                                           deletes it, so the dry run must
	//                                           count it
	build := func(res uuid.UUID, rrID uuid.UUID, reporter string, tomb bool, ageDays int, reps int) {
		createPGResource(t, db, res)
		createPGReporterResource(t, db, rrID, res, reporter, tomb, daysAgo(now, 60), daysAgo(now, ageDays))
		for v := 0; v < reps; v++ {
			createPGCommonRepresentation(t, db, res, uint(v), reporter, daysAgo(now, ageDays))
		}
	}

	res1, rr1 := uuid.New(), uuid.New()
	build(res1, rr1, "hbi", true, 40, 2)
	res2, rr2 := uuid.New(), uuid.New()
	build(res2, rr2, "hbi", true, 40, 3)
	res3, rr3 := uuid.New(), uuid.New()
	build(res3, rr3, "hbi", true, 40, 2)
	ocmID := uuid.New()
	createPGReporterResource(t, db, ocmID, res3, "ocm", false, daysAgo(now, 60), daysAgo(now, 3))
	res4, rr4 := uuid.New(), uuid.New()
	build(res4, rr4, "hbi", true, 5, 2)
	res5, rr5 := uuid.New(), uuid.New()
	build(res5, rr5, "ocm", true, 40, 1)
	res6 := uuid.New()
	createPGResource(t, db, res6)
	for v := 0; v < 2; v++ {
		createPGCommonRepresentation(t, db, res6, uint(v), "hbi", daysAgo(now, 45))
	}

	t.Run("unscoped", func(t *testing.T) {
		result, err := deleteOldTombstonedResourcesCascade(db, logger, true, 30, DeletionOrderOldestFirst, "", 10, 10, 0, 0, nil)
		require.NoError(t, err)

		assert.Equal(t, int64(4), result.ReporterResourcesDeleted,
			"the four old-tombstoned reporter rows (res1, res2, res3-hbi, res5-ocm)")
		assert.Equal(t, int64(8), result.CommonRepresentationsDeleted,
			"reps of resources with NO surviving reporter row (2 + 3 + 1) PLUS the 2 pre-existing orphans")
	})

	t.Run("scoped_to_hbi", func(t *testing.T) {
		result, err := deleteOldTombstonedResourcesCascade(db, logger, true, 30, DeletionOrderOldestFirst, "hbi", 10, 10, 0, 0, nil)
		require.NoError(t, err)

		assert.Equal(t, int64(3), result.ReporterResourcesDeleted,
			"only the hbi old-tombstoned rows enter the scoped worklist (res5 is ocm)")
		assert.Equal(t, int64(7), result.CommonRepresentationsDeleted,
			"eligible hbi resources (2 + 3) plus pre-existing orphans (2); res3 blocked by its active ocm row, res5 keeps its ocm reporter row")
	})

	// Nothing was modified by either dry run.
	assert.Equal(t, int64(6), countPGRows(t, db, &model.ReporterResource{}, ""),
		"dry-run must not delete reporter_resources")
	assert.Equal(t, int64(12), countPGRows(t, db, &model.CommonRepresentation{}, ""),
		"dry-run must not delete common_representations")
	assert.Equal(t, int64(6), countPGRows(t, db, &model.Resource{}, ""))
}

// TestPostgresCascade_TimeoutStopsBeforeFirstBatch pins the deadline check at
// the top of the batch loop: an already-expired timeout stops the run before
// any deletion and skips the final orphan drain.
func TestPostgresCascade_TimeoutStopsBeforeFirstBatch(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	res := uuid.New()
	createPGResource(t, db, res)
	createPGReporterResource(t, db, uuid.New(), res, "hbi", true, daysAgo(now, 60), daysAgo(now, 40))
	for v := 0; v < 2; v++ {
		createPGCommonRepresentation(t, db, res, uint(v), "hbi", daysAgo(now, 45))
	}

	result, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 10, 10, 0, time.Nanosecond, nil)
	require.NoError(t, err)

	assert.Equal(t, stopReasonTimeout, result.StopReason)
	assert.Equal(t, int64(0), result.ReporterResourcesDeleted, "nothing deleted before the expired timeout")
	assert.Equal(t, int64(0), result.CommonRepresentationsDeleted, "the orphan drain must not run after a partial stop")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, ""),
		"the old-tombstoned resource must survive")
	assert.Equal(t, int64(2), countPGRows(t, db, &model.CommonRepresentation{}, ""))
}

func TestPostgresCascade_BatchAccounting_Interleaving(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	// 3 old-tombstoned resources with 2 common reps each. batchSize 2 puts
	// A+B in batch 1 and C in batch 2; orphanBatchSize 2 caps the interleaved
	// orphan pass below the per-batch orphan population, so a backlog builds
	// up that only the final drain cleans — exercising the accounting across
	// the interleaved pass and the drain.
	resourceIDs := make([]uuid.UUID, 3)
	for i := range resourceIDs {
		resourceIDs[i] = uuid.New()
		id := uuid.New()
		createPGResource(t, db, resourceIDs[i])
		createPGReporterResource(t, db, id, resourceIDs[i], "hbi", true, daysAgo(now, 60), daysAgo(now, 40+i))
		createPGReporterRepresentation(t, db, id, 0, 1, daysAgo(now, 45))
		for v := 0; v < 2; v++ {
			createPGCommonRepresentation(t, db, resourceIDs[i], uint(v), "hbi", daysAgo(now, 45-v-i))
		}
	}

	result, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 2, 2, 0, 0, nil)
	require.NoError(t, err)
	assert.Empty(t, result.StopReason)

	assert.Equal(t, int64(3), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(6), result.CommonRepresentationsDeleted,
		"interleaved orphan passes (2+2) plus the final drain (2) must total every rep")
	for i, res := range resourceIDs {
		assert.Equal(t, int64(0), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", res),
			"resource %d reps", i)
		assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterRepresentation{}, ""),
			"all reporter reps gone via cascade")
	}
	assert.Equal(t, int64(3), countPGRows(t, db, &model.Resource{}, ""),
		"parent resource rows survive")
}

func TestPostgresCascade_RecentTombstoneCommonRepsUntouched(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	// A recent tombstone (<30d) keeps its reporter row AND its common reps,
	// even while an old tombstone in the same run loses everything.
	oldRes, oldRR := uuid.New(), uuid.New()
	createPGResource(t, db, oldRes)
	createPGReporterResource(t, db, oldRR, oldRes, "hbi", true, daysAgo(now, 60), daysAgo(now, 40))
	createPGCommonRepresentation(t, db, oldRes, 0, "hbi", daysAgo(now, 40))

	recentRes, recentRR := uuid.New(), uuid.New()
	createPGResource(t, db, recentRes)
	createPGReporterResource(t, db, recentRR, recentRes, "hbi", true, daysAgo(now, 60), daysAgo(now, 5))
	createPGCommonRepresentation(t, db, recentRes, 0, "hbi", daysAgo(now, 5))
	createPGCommonRepresentation(t, db, recentRes, 1, "hbi", daysAgo(now, 6))

	result, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 10, 10, 0, 0, nil)
	require.NoError(t, err)
	assert.Empty(t, result.StopReason)

	assert.Equal(t, int64(1), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(1), result.CommonRepresentationsDeleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", recentRR),
		"recent tombstones must survive")
	assert.Equal(t, int64(2), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", recentRes),
		"a surviving reporter row (even tombstoned) keeps the resource's common reps alive")
}

func TestPostgresCascade_ResumableAcrossRuns(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	// 4 old-tombstoned resources with 1 common rep each.
	resourceIDs := make([]uuid.UUID, 4)
	for i := range resourceIDs {
		resourceIDs[i] = uuid.New()
		id := uuid.New()
		createPGResource(t, db, resourceIDs[i])
		createPGReporterResource(t, db, id, resourceIDs[i], "hbi", true, daysAgo(now, 60), daysAgo(now, 40+i))
		createPGCommonRepresentation(t, db, resourceIDs[i], 0, "hbi", daysAgo(now, 40))
	}

	// Run 1: two batches of one, then a graceful --max-batches stop. The
	// interleaved orphan pass has already cleaned the two deleted resources'
	// reps; the other two resources are untouched.
	result, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 1, 10, 0, 0, &retentionRunOptions{MaxBatches: 2})
	require.NoError(t, err)
	assert.Equal(t, stopReasonMaxBatches, result.StopReason)
	assert.Equal(t, int64(2), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(2), result.CommonRepresentationsDeleted)
	assert.Equal(t, int64(2), countPGRows(t, db, &model.ReporterResource{}, "tombstone = ? AND updated_at < ?", true, daysAgo(now, 30)))
	assert.Equal(t, int64(2), countPGRows(t, db, &model.CommonRepresentation{}, ""))

	// Run 2: a fresh run recollects the remaining worklist and finishes,
	// including the final drain.
	result, err = deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 1, 10, 0, 0, nil)
	require.NoError(t, err)
	assert.Empty(t, result.StopReason)
	assert.Equal(t, int64(2), result.ReporterResourcesDeleted)
	assert.Equal(t, int64(2), result.CommonRepresentationsDeleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "tombstone = ? AND updated_at < ?", true, daysAgo(now, 30)))
	assert.Equal(t, int64(0), countPGRows(t, db, &model.CommonRepresentation{}, ""))
	assert.Equal(t, int64(4), countPGRows(t, db, &model.Resource{}, ""),
		"parent resource rows survive both runs")
}

func TestPostgresCascade_ZeroHistoryResource(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	// An old-tombstoned resource with NO common representations (the
	// COALESCE(history_size, 0) path): its deletion must not stall the loop —
	// the interleaved orphan batch deleting 0 rows is not an exit condition.
	zeroRes, zeroRR := uuid.New(), uuid.New()
	createPGResource(t, db, zeroRes)
	createPGReporterResource(t, db, zeroRR, zeroRes, "hbi", true, daysAgo(now, 60), daysAgo(now, 45))

	normalRes, normalRR := uuid.New(), uuid.New()
	createPGResource(t, db, normalRes)
	createPGReporterResource(t, db, normalRR, normalRes, "hbi", true, daysAgo(now, 60), daysAgo(now, 40))
	createPGCommonRepresentation(t, db, normalRes, 0, "hbi", daysAgo(now, 40))
	createPGCommonRepresentation(t, db, normalRes, 1, "hbi", daysAgo(now, 41))

	// batchSize 1 forces the zero-history resource into its own batch.
	result, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 1, 10, 0, 0, nil)
	require.NoError(t, err)
	assert.Empty(t, result.StopReason)

	assert.Equal(t, int64(2), result.ReporterResourcesDeleted,
		"the zero-history resource must not prevent the run from completing")
	assert.Equal(t, int64(2), result.CommonRepresentationsDeleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "id IN ?", []uuid.UUID{zeroRR, normalRR}))
	assert.Equal(t, int64(0), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", normalRes))
}

func TestPostgresCascade_InvalidDeletionOrder(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()

	_, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, "newest-first", "", 10, 10, 0, 0, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid deletion order")
}

// Runtime controls for the cascade job, using the job's REAL control-table
// name (tombstone_cascade_cleanup) — pause/resume parks between batches with
// the worklist preserved, and a control stop exits gracefully having deleted
// nothing (the final orphan drain must NOT run after a partial stop).
func TestPostgresCascade_RuntimeControls(t *testing.T) {
	setup := func(t *testing.T, db *gorm.DB, now time.Time) {
		for i := 0; i < 6; i++ {
			id := uuid.New()
			resourceID := uuid.New()
			createPGResource(t, db, resourceID)
			createPGReporterResource(t, db, id, resourceID, "hbi", true, daysAgo(now, 60), daysAgo(now, 31+i))
			createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 40))
		}
	}
	remaining := func(t *testing.T, db *gorm.DB, now time.Time) int64 {
		return countPGRows(t, db, &model.ReporterResource{}, "tombstone = ? AND updated_at < ?", true, daysAgo(now, 30))
	}
	run := func(db *gorm.DB, logger *log.Helper, opts *retentionRunOptions) (cascadeCleanupResult, error) {
		return deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 2, 10, 30, 0, opts)
	}

	t.Run("stop_before_start_deletes_nothing_and_skips_drain", func(t *testing.T) {
		db, observer := setupPostgresTestDBPair(t)
		logger := testLogger()
		now := time.Now().UTC()
		setup(t, db, now)
		require.NoError(t, observer.Exec(
			"INSERT INTO job_runtime_controls (job_name, desired_state, updated_by, note) VALUES ('tombstone_cascade_cleanup', 'stop', 'test', 'stop before start')").Error)

		result, err := run(db, logger, &retentionRunOptions{
			ControlJobName:      "tombstone_cascade_cleanup",
			ControlPollInterval: controlsTestPollInterval,
		})

		require.NoError(t, err)
		assert.Equal(t, stopReasonControlStop, result.StopReason)
		assert.Equal(t, int64(0), result.ReporterResourcesDeleted)
		assert.Equal(t, int64(0), result.CommonRepresentationsDeleted)
		assert.Equal(t, int64(6), remaining(t, db, now))
		assert.Equal(t, int64(6), countPGRows(t, db, &model.CommonRepresentation{}, ""),
			"a stopped run must not run the final orphan drain")
	})

	t.Run("pause_then_resume_completes", func(t *testing.T) {
		db, observer := setupPostgresTestDBPair(t)
		logger := testLogger()
		now := time.Now().UTC()
		setup(t, db, now)
		require.NoError(t, observer.Exec(
			"INSERT INTO job_runtime_controls (job_name, desired_state, updated_by, note) VALUES ('tombstone_cascade_cleanup', 'paused', 'test', 'pause at start')").Error)

		opts := &retentionRunOptions{
			ControlJobName:      "tombstone_cascade_cleanup",
			ControlPollInterval: controlsTestPollInterval,
		}
		var result cascadeCleanupResult
		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			result, err = run(db, logger, opts)
		}()

		// The job must be parked in the pause loop, not finished.
		time.Sleep(150 * time.Millisecond)
		select {
		case <-done:
			t.Fatal("job completed while it should be paused")
		default:
		}

		require.NoError(t, observer.Exec(
			"UPDATE job_runtime_controls SET desired_state = 'running', updated_by = 'test' WHERE job_name = 'tombstone_cascade_cleanup'").Error)
		<-done

		require.NoError(t, err)
		assert.Empty(t, result.StopReason)
		assert.Equal(t, int64(6), result.ReporterResourcesDeleted)
		assert.Equal(t, int64(6), result.CommonRepresentationsDeleted)
		assert.Equal(t, int64(0), remaining(t, db, now))
		assert.Equal(t, int64(0), countPGRows(t, db, &model.CommonRepresentation{}, ""))
	})
}
