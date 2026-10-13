package jobs

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Oldest-first deletion tests (requirement: oldest data is deleted first).
//
// A full run deletes every deletable row regardless of order, so ordering is
// only observable MID-run. Each test runs the helper with a small batch size
// and a generous inter-batch delay, while a poller samples the still-deletable
// rows. The invariant: whenever part of the deletable set is gone, no deleted
// row may be NEWER than a still-remaining one (i.e. the remaining set is
// always a suffix of the age-sorted deletable list).
//
// Deletable rows are inserted NEWEST-first so that physical/insertion order is
// anti-correlated with age — an unordered LIMIT therefore takes the newest
// rows first and violates the invariant.

// runWithOldestFirstCheck runs fn in the background while repeatedly calling
// sample (returning the still-deletable rows keyed by identifier, with their
// ordering timestamp) and asserting the oldest-first invariant at every
// observed partial state.
func runWithOldestFirstCheck(t *testing.T, run func(), sample func() map[string]time.Time) {
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
			for deletedKey, deletedAt := range initial {
				if _, stillPresent := current[deletedKey]; stillPresent {
					continue
				}
				for remainingKey, remainingAt := range current {
					if deletedAt.After(remainingAt) {
						t.Errorf("oldest-first violated: deleted row %s (%v) is newer than remaining row %s (%v)",
							deletedKey, deletedAt, remainingKey, remainingAt)
					}
				}
			}
		}
	}
}

// oldestFirstAges returns n deletable ages in days, NEWEST first: 26, 28, ...
// (all clearly older than the 7-day retention window anchored at a -2d
// latest row, and pairwise distinct so ties cannot mask an ordering bug).
func oldestFirstAges(n int) []int {
	ages := make([]int, n)
	for i := range ages {
		ages[i] = 26 + 2*i
	}
	return ages
}

func TestPostgresReporterReps_OldestDeletedFirst(t *testing.T) {
	db, observerDB := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	reporterResourceID := uuid.New()
	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	createPGReporterResource(t, db, reporterResourceID, resourceID, "hbi", false, daysAgo(now, 60), daysAgo(now, 60))

	// 8 deletable versions, inserted newest-first (version i has age -(26+2i) days).
	for i, ageDays := range oldestFirstAges(8) {
		createPGReporterRepresentation(t, db, reporterResourceID, uint(i), 1, daysAgo(now, ageDays))
	}
	// Latest version anchors the retention window and must survive.
	createPGReporterRepresentation(t, db, reporterResourceID, 8, 1, daysAgo(now, 2))

	sample := func() map[string]time.Time {
		rows := []struct {
			ReporterResourceID uuid.UUID `gorm:"column:reporter_resource_id"`
			Version            uint      `gorm:"column:version"`
			CreatedAt          time.Time `gorm:"column:created_at"`
		}{}
		require.NoError(t, observerDB.Raw("SELECT reporter_resource_id, version, created_at FROM reporter_representations WHERE created_at < ?", daysAgo(now, 9)).Scan(&rows).Error)
		out := make(map[string]time.Time, len(rows))
		for _, r := range rows {
			out[fmt.Sprintf("%s:%d", r.ReporterResourceID, r.Version)] = r.CreatedAt
		}
		return out
	}

	var deleted int64
	var err error
	runWithOldestFirstCheck(t, func() {
		deleted, _, err = deleteOldReporterRepresentations(db, logger, false, 7, 30, "", 2, 300, 0, nil)
	}, sample)

	require.NoError(t, err)
	assert.Equal(t, int64(8), deleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterRepresentation{}, ""),
		"only the anchored latest version may remain")
}

func TestPostgresCommonReps_OldestDeletedFirst(t *testing.T) {
	db, observerDB := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	// 8 deletable versions, inserted newest-first.
	for i, ageDays := range oldestFirstAges(8) {
		createPGCommonRepresentation(t, db, resourceID, uint(i), "hbi", daysAgo(now, ageDays))
	}
	// Latest version anchors the retention window and must survive.
	createPGCommonRepresentation(t, db, resourceID, 8, "hbi", daysAgo(now, 2))

	sample := func() map[string]time.Time {
		rows := []struct {
			ResourceId uuid.UUID `gorm:"column:resource_id"`
			Version    uint      `gorm:"column:version"`
			CreatedAt  time.Time `gorm:"column:created_at"`
		}{}
		require.NoError(t, observerDB.Raw("SELECT resource_id, version, created_at FROM common_representations WHERE created_at < ?", daysAgo(now, 9)).Scan(&rows).Error)
		out := make(map[string]time.Time, len(rows))
		for _, r := range rows {
			out[fmt.Sprintf("%s:%d", r.ResourceId, r.Version)] = r.CreatedAt
		}
		return out
	}

	var deleted int64
	var err error
	runWithOldestFirstCheck(t, func() {
		deleted, _, err = deleteOldCommonRepresentations(db, logger, false, 7, 30, "", 2, 300, 0, nil)
	}, sample)

	require.NoError(t, err)
	assert.Equal(t, int64(8), deleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, ""),
		"only the anchored latest version may remain")
}

func TestPostgresOrphanedCommonReps_OldestDeletedFirst(t *testing.T) {
	db, observerDB := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	// The orphaned resource: no reporter_resources rows, so ALL 8 of its
	// versions (any age) are deletable by the orphan job.
	orphanedResource := uuid.New()
	createPGResource(t, db, orphanedResource)
	for i, ageDays := range oldestFirstAges(8) {
		createPGCommonRepresentation(t, db, orphanedResource, uint(i), "hbi", daysAgo(now, ageDays))
	}

	// Keeper: a reported resource's reps must be untouched.
	keeperResource := uuid.New()
	createPGResource(t, db, keeperResource)
	reporterResourceID := uuid.New()
	createPGReporterResource(t, db, reporterResourceID, keeperResource, "hbi", false, daysAgo(now, 60), daysAgo(now, 60))
	createPGCommonRepresentation(t, db, keeperResource, 0, "hbi", daysAgo(now, 30))

	sample := func() map[string]time.Time {
		rows := []struct {
			ResourceId uuid.UUID `gorm:"column:resource_id"`
			Version    uint      `gorm:"column:version"`
			CreatedAt  time.Time `gorm:"column:created_at"`
		}{}
		require.NoError(t, observerDB.Raw("SELECT resource_id, version, created_at FROM common_representations WHERE resource_id = ?", orphanedResource).Scan(&rows).Error)
		out := make(map[string]time.Time, len(rows))
		for _, r := range rows {
			out[fmt.Sprintf("%s:%d", r.ResourceId, r.Version)] = r.CreatedAt
		}
		return out
	}

	var deleted int64
	var err error
	runWithOldestFirstCheck(t, func() {
		deleted, _, err = deleteOrphanedCommonRepresentations(db, logger, false, "", 2, 300, 0, nil)
	}, sample)

	require.NoError(t, err)
	assert.Equal(t, int64(8), deleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", orphanedResource))
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", keeperResource),
		"reported resources' reps must be untouched")
}

func TestPostgresTombstonedResources_OldestDeletedFirst(t *testing.T) {
	db, observerDB := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()

	// 8 old-tombstoned reporter_resources, inserted newest-first by updated_at
	// (the tombstone job's ordering key). Ages start beyond the 30-day
	// tombstone window so every row is deletable.
	for _, ageDays := range []int{36, 38, 40, 42, 44, 46, 48, 50} {
		id := uuid.New()
		resourceID := uuid.New()
		createPGResource(t, db, resourceID)
		createPGReporterResource(t, db, id, resourceID, "hbi", true, daysAgo(now, 60), daysAgo(now, ageDays))
	}

	// Keepers: a recent tombstone and an active resource.
	recentTombstone := uuid.New()
	recentResource := uuid.New()
	createPGResource(t, db, recentResource)
	createPGReporterResource(t, db, recentTombstone, recentResource, "hbi", true, daysAgo(now, 60), daysAgo(now, 5))
	activeID := uuid.New()
	activeResource := uuid.New()
	createPGResource(t, db, activeResource)
	createPGReporterResource(t, db, activeID, activeResource, "hbi", false, daysAgo(now, 60), daysAgo(now, 5))

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

	var deleted int64
	var err error
	runWithOldestFirstCheck(t, func() {
		deleted, _, err = deleteOldTombstonedResources(db, logger, false, 30, "", 2, 300, 0, nil)
	}, sample)

	require.NoError(t, err)
	assert.Equal(t, int64(8), deleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", recentTombstone),
		"recent tombstones must survive")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", activeID),
		"active resources must survive")
}
