package jobs

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// This file runs the retention cleanup helpers' actual production SQL against a
// real Postgres (see postgres_test_main_test.go for the harness). The jobs' SQL
// is Postgres-only ((? || ' days')::INTERVAL, NOW()), which is why these tests
// cannot run on the SQLite lane. They pin the deletion SAFETY semantics: no
// valid production data may be deleted.

// setupPostgresTestDB provisions an isolated per-test database on the shared
// Postgres instance and migrates it. Per-test DATABASES, not transactions: the
// helpers create temp tables and issue autocommit DELETEs, so transaction
// rollback cannot isolate them.
func setupPostgresTestDB(t *testing.T) *gorm.DB {
	db, _ := setupPostgresTestDBPair(t)
	return db
}

// setupPostgresTestDBPair provisions the per-test database and returns TWO
// independent handles to it. The second (observer) handle is needed because
// the helpers' temp tables are SESSION-local: an observer sharing the job's
// connection pool would make the pool rotate connections mid-run, and the
// job would lose its temp tables ("relation ... does not exist").
func setupPostgresTestDBPair(t *testing.T) (jobDB, observerDB *gorm.DB) {
	t.Helper()
	requirePostgres(t)

	admin, err := gorm.Open(postgres.Open(postgresTestInstance.dsn(postgresTestInstance.adminDB)), &gorm.Config{TranslateError: true})
	require.NoError(t, err)

	dbName := fmt.Sprintf("retention_test_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName)).Error)

	// data.Migrate POISONS the gorm.DB it runs on: WithAdvisoryLock
	// (internal/data/migrations/locks.go) swaps the shared Statement.ConnPool
	// for a wrapper around a dedicated *sql.Conn that it closes on return —
	// gorm.Session only clones the Statement when Context/PrepareStmt/SkipHooks
	// is set, so the swap leaks into the caller. Migrate on a throwaway
	// connection, then open fresh handles for the test itself.
	migrateDB, err := gorm.Open(postgres.Open(postgresTestInstance.dsn(dbName)), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	require.NoError(t, data.Migrate(migrateDB, testLogger()))
	if sqlDB, err := migrateDB.DB(); err == nil {
		_ = sqlDB.Close()
	}

	jobDB, err = gorm.Open(postgres.Open(postgresTestInstance.dsn(dbName)), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	observerDB, err = gorm.Open(postgres.Open(postgresTestInstance.dsn(dbName)), &gorm.Config{TranslateError: true})
	require.NoError(t, err)

	t.Cleanup(func() {
		for _, h := range []*gorm.DB{jobDB, observerDB} {
			if sqlDB, err := h.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		// WITH (FORCE) disconnects any leftover session before dropping (PG 13+).
		require.NoError(t, admin.Exec(fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", dbName)).Error)
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return jobDB, observerDB
}

// Fixture builders with EXPLICIT timestamps — GORM only overwrites zero
// autoCreateTime/autoUpdateTime values, so the retention windows are fully
// controlled by the tests. All timestamps are derived from a per-call now() and
// use day-granularity margins so client/server clock skew cannot flip a window.

func createPGResource(t *testing.T, db *gorm.DB, resourceID uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Create(&model.Resource{ID: resourceID, Type: "host"}).Error)
}

// createPGReporterResource inserts a reporter_resources row for the given
// canonical resource. LocalResourceID is derived from the surrogate id so the
// unique key (local_resource_id, reporter_type, resource_type, reporter_instance_id)
// never collides across fixtures.
func createPGReporterResource(t *testing.T, db *gorm.DB, id, resourceID uuid.UUID, reporterType string, tombstone bool, createdAt, updatedAt time.Time) {
	t.Helper()
	rr := model.ReporterResource{
		ID: id,
		ReporterResourceKey: model.ReporterResourceKey{
			LocalResourceID:    "local-" + id.String(),
			ReporterType:       reporterType,
			ResourceType:       "host",
			ReporterInstanceID: "instance-123",
		},
		ResourceID:            resourceID,
		APIHref:               "https://api.example.com/resource/" + resourceID.String(),
		RepresentationVersion: 0,
		Generation:            1,
		Tombstone:             tombstone,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}
	require.NoError(t, db.Create(&rr).Error)
}

func createPGReporterRepresentation(t *testing.T, db *gorm.DB, reporterResourceID uuid.UUID, version, generation uint, createdAt time.Time) {
	t.Helper()
	rep := model.ReporterRepresentation{
		Representation:     model.Representation{Data: internal.JsonObject{"fixture": true}},
		ReporterResourceID: reporterResourceID,
		Version:            version,
		Generation:         generation,
		TransactionId:      "",
		CreatedAt:          createdAt,
	}
	// Omit the association so GORM does not try to upsert the parent row.
	require.NoError(t, db.Omit("ReporterResource").Create(&rep).Error)
}

func createPGCommonRepresentation(t *testing.T, db *gorm.DB, resourceID uuid.UUID, version uint, reporterType string, createdAt time.Time) {
	t.Helper()
	rep := model.CommonRepresentation{
		Representation:             model.Representation{Data: internal.JsonObject{"fixture": true}},
		ResourceId:                 resourceID,
		Version:                    version,
		ReportedByReporterType:     reporterType,
		ReportedByReporterInstance: "instance-123",
		TransactionId:              "",
		CreatedAt:                  createdAt,
	}
	require.NoError(t, db.Create(&rep).Error)
}

// countPGRows counts rows of the given model, optionally restricted by a where
// clause (empty query counts all rows).
func countPGRows(t *testing.T, db *gorm.DB, m interface{}, query string, args ...interface{}) int64 {
	t.Helper()
	var n int64
	var err error
	if query == "" {
		err = db.Model(m).Count(&n).Error
	} else {
		err = db.Model(m).Where(query, args...).Count(&n).Error
	}
	require.NoError(t, err)
	return n
}

func daysAgo(now time.Time, days int) time.Time {
	return now.Add(-time.Duration(days) * 24 * time.Hour)
}

// ---------------------------------------------------------------------------
// deleteOldReporterRepresentations — safety semantics
// ---------------------------------------------------------------------------

// Per-resource anchoring: the retention window is measured from each resource's
// LATEST representation, not from NOW(). With retentionDays=7 only versions
// within 7 days of the resource's own latest survive.
func TestPostgresDeleteOldReporterRepresentations_PerResourceRetention(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	rrID := uuid.New()
	createPGReporterResource(t, db, rrID, resourceID, "hbi", false, daysAgo(now, 35), daysAgo(now, 35))

	// versions at -35d, -10d and -2d: only -2d is within 7 days of the -2d latest
	createPGReporterRepresentation(t, db, rrID, 0, 1, daysAgo(now, 35))
	createPGReporterRepresentation(t, db, rrID, 1, 1, daysAgo(now, 10))
	createPGReporterRepresentation(t, db, rrID, 2, 1, daysAgo(now, 2))

	deleted, stopReason, err := deleteOldReporterRepresentations(db, logger, false, 7, 30, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(2), deleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ?", rrID),
		"only the latest (-2d) version may survive")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ? AND version = ?", rrID, 2),
		"the surviving version must be the latest one")
}

// The quiesced-resource case a global NOW()-cutoff would get wrong: a resource
// whose latest representation is itself 60 days old. Its LATEST row must
// survive; only versions older than (latest - 7d) may be deleted.
func TestPostgresDeleteOldReporterRepresentations_QuiescedResourceKeepsLatest(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	rrID := uuid.New()
	createPGReporterResource(t, db, rrID, resourceID, "hbi", false, daysAgo(now, 70), daysAgo(now, 70))

	createPGReporterRepresentation(t, db, rrID, 0, 1, daysAgo(now, 70))
	createPGReporterRepresentation(t, db, rrID, 1, 1, daysAgo(now, 60)) // latest

	deleted, stopReason, err := deleteOldReporterRepresentations(db, logger, false, 7, 30, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(1), deleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ?", rrID),
		"the quiesced resource's latest (-60d) version MUST survive")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ? AND version = ?", rrID, 1),
		"the survivor must be the latest version")
}

// Tombstone window: representations of RECENTLY tombstoned resources are still
// retention-cleaned; representations of OLD tombstoned resources are left for
// the tombstoned-resources job (which deletes the resource and cascades).
func TestPostgresDeleteOldReporterRepresentations_TombstoneWindow(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	// recent tombstone (updated -5d): its old reps ARE deletable
	recentResource := uuid.New()
	createPGResource(t, db, recentResource)
	recentRR := uuid.New()
	createPGReporterResource(t, db, recentRR, recentResource, "hbi", true, daysAgo(now, 35), daysAgo(now, 5))
	createPGReporterRepresentation(t, db, recentRR, 0, 1, daysAgo(now, 30))
	createPGReporterRepresentation(t, db, recentRR, 1, 1, daysAgo(now, 2))

	// old tombstone (updated -40d): its reps are NOT this job's business
	oldResource := uuid.New()
	createPGResource(t, db, oldResource)
	oldRR := uuid.New()
	createPGReporterResource(t, db, oldRR, oldResource, "hbi", true, daysAgo(now, 40), daysAgo(now, 40))
	createPGReporterRepresentation(t, db, oldRR, 0, 1, daysAgo(now, 30))
	createPGReporterRepresentation(t, db, oldRR, 1, 1, daysAgo(now, 2))

	deleted, _, err := deleteOldReporterRepresentations(db, logger, false, 7, 30, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted, "only the recent tombstone's -30d version may be deleted")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ?", recentRR))
	assert.Equal(t, int64(2), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ?", oldRR),
		"old-tombstoned resources' reps must be left for the tombstoned-resources job")
}

func TestPostgresDeleteOldReporterRepresentations_ReporterFilter(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	for _, reporterType := range []string{"hbi", "ocm"} {
		resourceID := uuid.New()
		createPGResource(t, db, resourceID)
		rrID := uuid.New()
		createPGReporterResource(t, db, rrID, resourceID, reporterType, false, daysAgo(now, 35), daysAgo(now, 35))
		createPGReporterRepresentation(t, db, rrID, 0, 1, daysAgo(now, 30))
		createPGReporterRepresentation(t, db, rrID, 1, 1, daysAgo(now, 2))
	}

	deleted, _, err := deleteOldReporterRepresentations(db, logger, false, 7, 30, "hbi", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	var hbiRR, ocmRR model.ReporterResource
	require.NoError(t, db.Where("reporter_type = ?", "hbi").First(&hbiRR).Error)
	require.NoError(t, db.Where("reporter_type = ?", "ocm").First(&ocmRR).Error)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ?", hbiRR.ID))
	assert.Equal(t, int64(2), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ?", ocmRR.ID),
		"other reporters' representations must be untouched")
}

func TestPostgresDeleteOldReporterRepresentations_DryRun(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	rrID := uuid.New()
	createPGReporterResource(t, db, rrID, resourceID, "hbi", false, daysAgo(now, 35), daysAgo(now, 35))
	createPGReporterRepresentation(t, db, rrID, 0, 1, daysAgo(now, 30))
	createPGReporterRepresentation(t, db, rrID, 1, 1, daysAgo(now, 2))

	count, stopReason, err := deleteOldReporterRepresentations(db, logger, true, 7, 30, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(1), count, "dry-run must report the correct count")
	assert.Equal(t, int64(2), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ?", rrID),
		"dry-run must not delete anything")
}

func TestPostgresDeleteOldReporterRepresentations_BatchAccounting(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	// 3 resources x (2 old + 1 recent) versions = 6 deletable rows, batch size 2
	// -> 3 full batches, exercising the batch loop across several iterations
	for i := 0; i < 3; i++ {
		resourceID := uuid.New()
		createPGResource(t, db, resourceID)
		rrID := uuid.New()
		createPGReporterResource(t, db, rrID, resourceID, "hbi", false, daysAgo(now, 35), daysAgo(now, 35))
		createPGReporterRepresentation(t, db, rrID, 0, 1, daysAgo(now, 30))
		createPGReporterRepresentation(t, db, rrID, 1, 1, daysAgo(now, 20))
		createPGReporterRepresentation(t, db, rrID, 2, 1, daysAgo(now, 2))
	}

	deleted, stopReason, err := deleteOldReporterRepresentations(db, logger, false, 7, 30, "", 2, 0, 0, nil)

	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(6), deleted)
	assert.Equal(t, int64(3), countPGRows(t, db, &model.ReporterRepresentation{}, ""))
}

func TestPostgresDeleteOldReporterRepresentations_TimeoutStopsGracefully(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	rrID := uuid.New()
	createPGReporterResource(t, db, rrID, resourceID, "hbi", false, daysAgo(now, 35), daysAgo(now, 35))
	createPGReporterRepresentation(t, db, rrID, 0, 1, daysAgo(now, 30))

	deleted, stopReason, err := deleteOldReporterRepresentations(db, logger, false, 7, 30, "", 100, 0, time.Nanosecond, nil)

	require.NoError(t, err, "a graceful timeout must not surface as an error")
	assert.Equal(t, stopReasonTimeout, stopReason)
	assert.Equal(t, int64(0), deleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ?", rrID),
		"timeout must stop before any deletion — the run is resumable")
}

// ---------------------------------------------------------------------------
// deleteOldCommonRepresentations — safety semantics
// ---------------------------------------------------------------------------

func TestPostgresDeleteOldCommonRepresentations_PerResourceRetention(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 30))
	createPGCommonRepresentation(t, db, resourceID, 1, "hbi", daysAgo(now, 10))
	createPGCommonRepresentation(t, db, resourceID, 2, "hbi", daysAgo(now, 2))

	deleted, stopReason, err := deleteOldCommonRepresentations(db, logger, false, 7, 30, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(2), deleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID))
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ? AND version = ?", resourceID, 2),
		"the surviving version must be the latest one")
}

// Quiesced resource: latest common rep is 60d old — it MUST survive; only the
// older -70d version is deleted. This is the case a global NOW()-7d cutoff
// would get wrong (it would delete both).
func TestPostgresDeleteOldCommonRepresentations_QuiescedResourceKeepsLatest(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 70))
	createPGCommonRepresentation(t, db, resourceID, 1, "hbi", daysAgo(now, 60)) // latest

	deleted, stopReason, err := deleteOldCommonRepresentations(db, logger, false, 7, 30, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(1), deleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID),
		"the quiesced resource's latest (-60d) version MUST survive")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ? AND version = ?", resourceID, 1),
		"the survivor must be the latest version")
}

// PINNED KNOWN LIMITATION: tombstoneDays is accepted but NOT applied in this
// helper's SQL (documented in the runbook). Common reps of an old-tombstoned
// resource are still retention-cleaned here — deleting them via the tombstone
// job + orphan job is the designed path instead. If this test ever fails, the
// flag's semantics changed and the docs must change with it.
func TestPostgresDeleteOldCommonRepresentations_TombstoneDaysIgnored(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	rrID := uuid.New()
	// fully tombstoned long ago — retention still processes its common reps
	createPGReporterResource(t, db, rrID, resourceID, "hbi", true, daysAgo(now, 40), daysAgo(now, 40))
	createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 30))
	createPGCommonRepresentation(t, db, resourceID, 1, "hbi", daysAgo(now, 2))

	deleted, _, err := deleteOldCommonRepresentations(db, logger, false, 7, 30, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted,
		"tombstoneDays is ignored: the old version is deleted even though the resource is an old tombstone")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID))
}

func TestPostgresDeleteOldCommonRepresentations_ReporterFilter(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	for _, reporterType := range []string{"hbi", "ocm"} {
		resourceID := uuid.New()
		createPGResource(t, db, resourceID)
		createPGCommonRepresentation(t, db, resourceID, 0, reporterType, daysAgo(now, 30))
		createPGCommonRepresentation(t, db, resourceID, 1, reporterType, daysAgo(now, 2))
	}

	deleted, _, err := deleteOldCommonRepresentations(db, logger, false, 7, 30, "hbi", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	var hbiResource, ocmResource model.CommonRepresentation
	require.NoError(t, db.Where("reported_by_reporter_type = ?", "hbi").Order("version").First(&hbiResource).Error)
	require.NoError(t, db.Where("reported_by_reporter_type = ?", "ocm").Order("version").First(&ocmResource).Error)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", hbiResource.ResourceId))
	assert.Equal(t, int64(2), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", ocmResource.ResourceId),
		"other reporters' representations must be untouched")
}

func TestPostgresDeleteOldCommonRepresentations_DryRun(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 30))
	createPGCommonRepresentation(t, db, resourceID, 1, "hbi", daysAgo(now, 2))

	count, stopReason, err := deleteOldCommonRepresentations(db, logger, true, 7, 30, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(1), count)
	assert.Equal(t, int64(2), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID),
		"dry-run must not delete anything")
}

func TestPostgresDeleteOldCommonRepresentations_BatchAccounting(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	// 7 deletable versions across resources, batch size 2 -> 4 batches (2+2+2+1),
	// exercising the worklist drain between batches
	for i := 0; i < 3; i++ {
		resourceID := uuid.New()
		createPGResource(t, db, resourceID)
		createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 30))
		createPGCommonRepresentation(t, db, resourceID, 1, "hbi", daysAgo(now, 20))
		createPGCommonRepresentation(t, db, resourceID, 2, "hbi", daysAgo(now, 2))
	}
	extraResource := uuid.New()
	createPGResource(t, db, extraResource)
	createPGCommonRepresentation(t, db, extraResource, 0, "hbi", daysAgo(now, 30))
	createPGCommonRepresentation(t, db, extraResource, 1, "hbi", daysAgo(now, 2))

	deleted, stopReason, err := deleteOldCommonRepresentations(db, logger, false, 7, 30, "", 2, 0, 0, nil)

	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(7), deleted)
	assert.Equal(t, int64(4), countPGRows(t, db, &model.CommonRepresentation{}, ""))
}

func TestPostgresDeleteOldCommonRepresentations_TimeoutStopsGracefully(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 30))

	deleted, stopReason, err := deleteOldCommonRepresentations(db, logger, false, 7, 30, "", 100, 0, time.Nanosecond, nil)

	require.NoError(t, err)
	assert.Equal(t, stopReasonTimeout, stopReason)
	assert.Equal(t, int64(0), deleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID))
}

// The worklist is a one-time snapshot: a timed-out (or otherwise interrupted)
// run leaves the remaining rows for the NEXT run, which then deletes them.
func TestPostgresDeleteOldCommonRepresentations_ResumableAcrossRuns(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 30))
	createPGCommonRepresentation(t, db, resourceID, 1, "hbi", daysAgo(now, 20))
	createPGCommonRepresentation(t, db, resourceID, 2, "hbi", daysAgo(now, 2))

	// first run: times out before deleting anything
	deleted, stopReason, err := deleteOldCommonRepresentations(db, logger, false, 7, 30, "", 100, 0, time.Nanosecond, nil)
	require.NoError(t, err)
	assert.Equal(t, stopReasonTimeout, stopReason)
	assert.Equal(t, int64(0), deleted)

	// second run: completes the work
	deleted, stopReason, err = deleteOldCommonRepresentations(db, logger, false, 7, 30, "", 100, 0, 0, nil)
	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(2), deleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID))
}

// ---------------------------------------------------------------------------
// deleteOrphanedCommonRepresentations — safety semantics on real Postgres
// (complements the SQLite suite; the anti-join correctness is what matters here)
// ---------------------------------------------------------------------------

// A multi-reporter resource with one reporter row removed is NOT orphaned: the
// anti-join is on reporter_resources.resource_id (the canonical resource ID),
// so as long as ANY reporter still reports the resource, all versions survive.
func TestPostgresDeleteOrphanedCommonRepresentations_MultiReporterResourceSurvives(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	createPGReporterResource(t, db, uuid.New(), resourceID, "hbi", false, time.Now().UTC(), time.Now().UTC())
	removed := uuid.New()
	createPGReporterResource(t, db, removed, resourceID, "ocm", false, time.Now().UTC(), time.Now().UTC())
	require.NoError(t, db.Delete(&model.ReporterResource{}, removed).Error)

	createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(time.Now().UTC(), 30))
	createPGCommonRepresentation(t, db, resourceID, 1, "hbi", daysAgo(time.Now().UTC(), 2))

	deleted, _, err := deleteOrphanedCommonRepresentations(db, logger, false, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted)
	assert.Equal(t, int64(2), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID),
		"a resource still reported by one reporter must keep ALL versions")
}

// Fully orphaned resources (no reporter_resources rows at all) are deleted in
// full — ALL versions, including the latest.
func TestPostgresDeleteOrphanedCommonRepresentations_FullyOrphanedDeleted(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	orphanID := uuid.New()
	createPGResource(t, db, orphanID)
	createPGCommonRepresentation(t, db, orphanID, 0, "hbi", daysAgo(now, 30))
	createPGCommonRepresentation(t, db, orphanID, 1, "hbi", daysAgo(now, 2))

	liveID := uuid.New()
	createPGResource(t, db, liveID)
	createPGReporterResource(t, db, uuid.New(), liveID, "hbi", false, now, now)
	createPGCommonRepresentation(t, db, liveID, 0, "hbi", daysAgo(now, 30))
	createPGCommonRepresentation(t, db, liveID, 1, "hbi", daysAgo(now, 2))

	deleted, _, err := deleteOrphanedCommonRepresentations(db, logger, false, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", orphanID))
	assert.Equal(t, int64(2), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", liveID))
}

// ---------------------------------------------------------------------------
// deleteOldTombstonedResources — safety semantics
// ---------------------------------------------------------------------------

func TestPostgresDeleteOldTombstonedResources_OnlyOldTombstonesDeleted(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	oldTombstone := uuid.New()
	createPGResource(t, db, oldTombstone)
	oldRR := uuid.New()
	createPGReporterResource(t, db, oldRR, oldTombstone, "hbi", true, daysAgo(now, 40), daysAgo(now, 40))

	recentTombstone := uuid.New()
	createPGResource(t, db, recentTombstone)
	recentRR := uuid.New()
	createPGReporterResource(t, db, recentRR, recentTombstone, "hbi", true, daysAgo(now, 35), daysAgo(now, 5))

	active := uuid.New()
	createPGResource(t, db, active)
	activeRR := uuid.New()
	createPGReporterResource(t, db, activeRR, active, "hbi", false, daysAgo(now, 40), daysAgo(now, 40))

	deleted, stopReason, err := deleteOldTombstonedResources(db, logger, false, 30, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(1), deleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "id = ?", oldRR), "old tombstone must be deleted")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", recentRR),
		"recently tombstoned resource must survive the 30-day window")
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "id = ?", activeRR),
		"active resource must NEVER be deleted by this job")
}

// The DB cascade removes reporter_representations but NOT common_representations
// — the latter become orphans, which is exactly the orphan job's input.
func TestPostgresDeleteOldTombstonedResources_CascadeRemovesReporterRepsNotCommonReps(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	resourceID := uuid.New()
	createPGResource(t, db, resourceID)
	rrID := uuid.New()
	createPGReporterResource(t, db, rrID, resourceID, "hbi", true, daysAgo(now, 40), daysAgo(now, 40))
	createPGReporterRepresentation(t, db, rrID, 0, 1, daysAgo(now, 30))
	createPGReporterRepresentation(t, db, rrID, 1, 1, daysAgo(now, 2))
	createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 30))
	createPGCommonRepresentation(t, db, resourceID, 1, "hbi", daysAgo(now, 2))

	deleted, _, err := deleteOldTombstonedResources(db, logger, false, 30, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "id = ?", rrID))
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterRepresentation{}, "reporter_resource_id = ?", rrID),
		"the FK cascade must remove the reporter representations")
	assert.Equal(t, int64(2), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID),
		"common representations survive as orphans — the orphan job's input")

	// and the orphan job then removes exactly those
	orphanDeleted, _, err := deleteOrphanedCommonRepresentations(db, logger, false, "", 100, 0, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), orphanDeleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.CommonRepresentation{}, "resource_id = ?", resourceID))
}

func TestPostgresDeleteOldTombstonedResources_ReporterFilter(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	for _, reporterType := range []string{"hbi", "ocm"} {
		resourceID := uuid.New()
		createPGResource(t, db, resourceID)
		createPGReporterResource(t, db, uuid.New(), resourceID, reporterType, true, daysAgo(now, 40), daysAgo(now, 40))
	}

	deleted, _, err := deleteOldTombstonedResources(db, logger, false, 30, "hbi", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	assert.Equal(t, int64(1), countPGRows(t, db, &model.ReporterResource{}, "reporter_type = ?", "ocm"))
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "reporter_type = ?", "hbi"))
}

func TestPostgresDeleteOldTombstonedResources_DryRun(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	for i := 0; i < 2; i++ {
		resourceID := uuid.New()
		createPGResource(t, db, resourceID)
		createPGReporterResource(t, db, uuid.New(), resourceID, "hbi", true, daysAgo(now, 40), daysAgo(now, 40))
	}

	count, stopReason, err := deleteOldTombstonedResources(db, logger, true, 30, "", 100, 0, 0, nil)

	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(2), count)
	assert.Equal(t, int64(2), countPGRows(t, db, &model.ReporterResource{}, "tombstone = ?", true),
		"dry-run must not delete anything")
}

func TestPostgresDeleteOldTombstonedResources_BatchAccounting(t *testing.T) {
	db := setupPostgresTestDB(t)
	logger := testLogger()
	now := time.Now().UTC()

	// 5 old tombstones, batch size 2 -> 3 batches (2+2+1)
	for i := 0; i < 5; i++ {
		resourceID := uuid.New()
		createPGResource(t, db, resourceID)
		createPGReporterResource(t, db, uuid.New(), resourceID, "hbi", true, daysAgo(now, 40), daysAgo(now, 40))
	}

	deleted, stopReason, err := deleteOldTombstonedResources(db, logger, false, 30, "", 2, 0, 0, nil)

	require.NoError(t, err)
	assert.Empty(t, stopReason)
	assert.Equal(t, int64(5), deleted)
	assert.Equal(t, int64(0), countPGRows(t, db, &model.ReporterResource{}, "tombstone = ?", true))
}
