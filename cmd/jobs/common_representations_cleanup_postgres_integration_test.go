//go:build integration

package jobs

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	resourceuc "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestCommonRepCleanupPostgres(t *testing.T) {
	db, _ := setupCommonRepCleanupPostgres(t)

	t.Run("per-resource retention prunes only versions older than the retained window", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		// Active root with an explicit pointer: versions 0..4, current 4, keeps 3 and 4.
		pointerRootID := uuid.New()
		seedCommonRepRoot(t, db, pointerRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, pointerRootID, "pointer-root", "host", "hbi", false).Error)

		// Active root with a NULL pointer: current is MAX(version).
		nullPointerRootID := uuid.New()
		seedCommonRepRoot(t, db, nullPointerRootID, "host", nil, 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, nullPointerRootID, "null-pointer-root", "host", "hbi", false).Error)

		// Shared root with two active reporters: pruned like any other active root.
		sharedRootID := uuid.New()
		seedCommonRepRoot(t, db, sharedRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, sharedRootID, "shared-root-hbi", "host", "hbi", false).Error)
		require.NoError(t, insertCommonRepReporter(t, db, sharedRootID, "shared-root-ocm", "host", "ocm", false).Error)

		// Root with fewer versions than the retention window keeps everything.
		freshRootID := uuid.New()
		seedCommonRepRoot(t, db, freshRootID, "host", uintPointer(1), 0, 1)
		require.NoError(t, insertCommonRepReporter(t, db, freshRootID, "fresh-root", "host", "hbi", false).Error)

		// Root whose pointer does not resolve to an existing row is excluded, not guessed.
		inconsistentRootID := uuid.New()
		seedCommonRepRoot(t, db, inconsistentRootID, "host", uintPointer(4), 0, 1, 2, 3)
		require.NoError(t, insertCommonRepReporter(t, db, inconsistentRootID, "inconsistent-root", "host", "hbi", false).Error)

		// Root with only tombstoned reporters is out of scope: the tombstone
		// cleanup workstream owns it and no row may be touched here.
		tombstonedOnlyRootID := uuid.New()
		seedCommonRepRoot(t, db, tombstonedOnlyRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, tombstonedOnlyRootID, "tombstoned-only-root", "host", "hbi", true).Error)

		options, err := newCommonRepCleanupOptions(100, 100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)

		report, err := usecase.Preview(context.Background(), policy, options, uuid.Nil, nil)
		require.NoError(t, err)
		// Roots without common history are never visited: the pass walks
		// common_representations, so only the six roots with rows are scanned.
		assert.Equal(t, int64(6), report.rootsScanned)
		assert.Equal(t, int64(1), report.rootsWithoutActiveReporter, "the tombstoned-only root is out of scope")
		assert.Equal(t, int64(1), report.excludedInconsistentRoots)
		assert.Equal(t, int64(9), report.prunableRows, "three roots with three prunable versions each")
		assert.Equal(t, int64(3), report.prunableRoots)
		assert.True(t, report.passComplete)

		// Preview must not mutate anything.
		assertCommonRepVersions(t, db, pointerRootID, []uint{0, 1, 2, 3, 4})
		assertCommonRepVersions(t, db, tombstonedOnlyRootID, []uint{0, 1, 2, 3, 4})

		// Read-only transactions must refuse deletion.
		readOnlyTransaction := newCommonRepCleanupTransaction(db, data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 3))
		readOnlyErr := readOnlyTransaction.WithinSerializableCommonRepCleanup(context.Background(), "common_rep_cleanup_read_only_enforcement_test", true, 30*time.Second, 5*time.Second, func(repository commonRepCleanupRepository) error {
			_, err := repository.DeletePrunableRoots(context.Background(), policy, []uuid.UUID{pointerRootID}, 100)
			return err
		})
		require.ErrorContains(t, readOnlyErr, "read-only transaction")

		result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(9), result.deletedRows)
		assert.Equal(t, int64(3), result.impactedRoots)

		assertCommonRepVersions(t, db, pointerRootID, []uint{3, 4}, "only the retained window survives")
		assertCommonRepVersions(t, db, nullPointerRootID, []uint{3, 4}, "a NULL pointer falls back to MAX(version)")
		assertCommonRepVersions(t, db, sharedRootID, []uint{3, 4})
		assertCommonRepVersions(t, db, freshRootID, []uint{0, 1})
		assertCommonRepVersions(t, db, inconsistentRootID, []uint{0, 1, 2, 3}, "an inconsistent pointer excludes the whole root")
		assertCommonRepVersions(t, db, tombstonedOnlyRootID, []uint{0, 1, 2, 3, 4}, "out-of-scope roots are untouched")

		// Roots and reporter rows are never deleted.
		assertCommonRepRootCount(t, db, pointerRootID, 1)
		assertCommonRepReporterRowCount(t, db, sharedRootID, 2, "both shared-root reporters survive")
		var tombstonedReporterCount int64
		require.NoError(t, db.Table("reporter_resources").Where("resource_id = ?", tombstonedOnlyRootID).Count(&tombstonedReporterCount).Error)
		assert.Equal(t, int64(1), tombstonedReporterCount)
	})

	t.Run("keep-versions input widens the retained window", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(3, "")
		require.NoError(t, err)

		rootID := uuid.New()
		seedCommonRepRoot(t, db, rootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, rootID, "keep-three-root", "host", "hbi", false).Error)

		options, err := newCommonRepCleanupOptions(100, 100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.deletedRows)
		assertCommonRepVersions(t, db, rootID, []uint{2, 3, 4})
	})

	t.Run("resource type filter scopes the selection", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "cluster")
		require.NoError(t, err)

		clusterRootID := uuid.New()
		seedCommonRepRoot(t, db, clusterRootID, "cluster", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, clusterRootID, "cluster-root", "cluster", "hbi", false).Error)
		hostRootID := uuid.New()
		seedCommonRepRoot(t, db, hostRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, hostRootID, "host-root", "host", "hbi", false).Error)

		options, err := newCommonRepCleanupOptions(100, 100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)

		report, err := usecase.Preview(context.Background(), policy, options, uuid.Nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(1), report.rootsScanned, "roots of other types are not classified")
		assert.Equal(t, int64(3), report.prunableRows)

		result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.deletedRows)
		assertCommonRepVersions(t, db, clusterRootID, []uint{3, 4})
		assertCommonRepVersions(t, db, hostRootID, []uint{0, 1, 2, 3, 4}, "the unfiltered type is untouched")
	})

	t.Run("gapped history retains the newest existing rows at or below the pointer", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		pointerGapRootID := uuid.New()
		seedCommonRepRoot(t, db, pointerGapRootID, "host", uintPointer(4), 0, 2, 4)
		require.NoError(t, insertCommonRepReporter(t, db, pointerGapRootID, "pointer-gap-root", "host", "hbi", false).Error)

		nullGapRootID := uuid.New()
		seedCommonRepRoot(t, db, nullGapRootID, "host", nil, 0, 2, 4)
		require.NoError(t, insertCommonRepReporter(t, db, nullGapRootID, "null-gap-root", "host", "hbi", false).Error)

		belowPointerRootID := uuid.New()
		seedCommonRepRoot(t, db, belowPointerRootID, "host", uintPointer(2), 0, 1, 2, 3, 5)
		require.NoError(t, insertCommonRepReporter(t, db, belowPointerRootID, "below-pointer-root", "host", "hbi", false).Error)

		sparseRootID := uuid.New()
		seedCommonRepRoot(t, db, sparseRootID, "host", uintPointer(2), 0, 1, 5)
		require.NoError(t, insertCommonRepReporter(t, db, sparseRootID, "sparse-root", "host", "hbi", false).Error)

		options, err := newCommonRepCleanupOptions(100, 100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)

		report, err := usecase.Preview(context.Background(), policy, options, uuid.Nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), report.prunableRows, "one prunable row per gapped root with three or more rows")

		result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.deletedRows)

		assertCommonRepVersions(t, db, pointerGapRootID, []uint{2, 4}, "retention counts existing rows, not numeric slots")
		assertCommonRepVersions(t, db, nullGapRootID, []uint{2, 4}, "a NULL pointer falls back to MAX(version) before ranking")
		assertCommonRepVersions(t, db, belowPointerRootID, []uint{1, 2, 3, 5}, "rows above a non-NULL pointer are never ranked or deleted")
		assertCommonRepVersions(t, db, sparseRootID, []uint{0, 1, 5}, "fewer existing rows than the retention window prunes nothing")
	})

	t.Run("retention is version-anchored, not age-anchored", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		rootID := uuid.New()
		require.NoError(t, db.Exec("INSERT INTO resource (id, type, common_version) VALUES (?, 'host', 4)", rootID).Error)
		// Deliberately conflicting timestamps: the highest version is the oldest row.
		baseTime := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
		for version := uint(0); version <= 4; version++ {
			createdAt := baseTime.Add(time.Duration(4-version) * time.Hour)
			require.NoError(t, insertCommonRepRowAt(t, db, rootID, version, fmt.Sprintf("age-anchor-%d", version), createdAt).Error)
		}
		require.NoError(t, insertCommonRepReporter(t, db, rootID, "age-anchor-root", "host", "hbi", false).Error)

		options, err := newCommonRepCleanupOptions(100, 100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.deletedRows)
		assertCommonRepVersions(t, db, rootID, []uint{3, 4}, "the oldest-timestamp rows survive because ranking is by version")
	})

	t.Run("chunk boundaries stay root-aligned and deep roots prune in bounded batches", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		// Twelve versions behind a chunk-rows cap of five: the boundary probe lands
		// inside this root, so the first chunk must contain the root alone and whole.
		deepRootID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
		seedCommonRepRoot(t, db, deepRootID, "host", uintPointer(11), 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11)
		require.NoError(t, insertCommonRepReporter(t, db, deepRootID, "deep-root", "host", "hbi", false).Error)

		smallRootID := uuid.MustParse("20000000-0000-0000-0000-000000000002")
		seedCommonRepRoot(t, db, smallRootID, "host", uintPointer(2), 0, 1, 2)
		require.NoError(t, insertCommonRepReporter(t, db, smallRootID, "small-root", "host", "hbi", false).Error)

		// A batch size of four forces the deep root through the bounded
		// lowest-version loop: ten prunable rows commit as three batches.
		options, err := newCommonRepCleanupOptions(5, 4, 0, 10, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)

		report, err := usecase.Preview(context.Background(), policy, options, uuid.Nil, nil)
		require.NoError(t, err)
		assert.True(t, report.passComplete)
		assert.GreaterOrEqual(t, report.chunksProcessed, int64(2), "the deep root and the small root land in different chunks")

		result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(11), result.deletedRows)
		assert.Equal(t, 4, result.committedBatches, "ten deep rows at four per statement plus one small-root batch")
		assertCommonRepVersions(t, db, deepRootID, []uint{10, 11}, "the deep root is pruned completely across chunk-aligned batches")
		assertCommonRepVersions(t, db, smallRootID, []uint{1, 2})
	})

	t.Run("resume after the reported cursor completes the pass", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		// Five rows each: with chunk-rows capped at three, each root gets its own
		// root-aligned chunk and the budget stops after the first.
		firstRootID := uuid.MustParse("10000000-0000-0000-0000-00000000000a")
		seedCommonRepRoot(t, db, firstRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, firstRootID, "resume-first-root", "host", "hbi", false).Error)
		secondRootID := uuid.MustParse("20000000-0000-0000-0000-00000000000b")
		seedCommonRepRoot(t, db, secondRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, secondRootID, "resume-second-root", "host", "hbi", false).Error)

		usecase := newTestCommonRepCleanupUsecase(t, db)
		budgeted, err := newCommonRepCleanupOptions(3, 100, 0, 1, time.Minute)
		require.NoError(t, err)
		firstRun, err := usecase.Execute(context.Background(), policy, budgeted, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), firstRun.deletedRows)
		assert.True(t, firstRun.batchLimitReached)
		assert.False(t, firstRun.report.passComplete)
		assert.Equal(t, firstRootID, firstRun.report.lastCursor, "the cursor names the last fully processed root")
		assertCommonRepVersions(t, db, firstRootID, []uint{3, 4})
		assertCommonRepVersions(t, db, secondRootID, []uint{0, 1, 2, 3, 4}, "roots after the stopped cursor are untouched")

		resume, err := newCommonRepCleanupOptions(3, 100, 0, 5, time.Minute)
		require.NoError(t, err)
		secondRun, err := usecase.Execute(context.Background(), policy, resume, firstRun.report.lastCursor, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), secondRun.deletedRows)
		assert.True(t, secondRun.report.passComplete)
		assert.Equal(t, int64(1), secondRun.report.rootsScanned, "resuming after the cursor skips already-passed roots")
		assertCommonRepVersions(t, db, secondRootID, []uint{3, 4})
	})

	t.Run("resume after a mid-chunk budget stop reprocesses the interrupted chunk", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		// Three roots in one chunk: the first bounded batch commits and the
		// budget refuses another, so the chunk itself is incomplete.
		rootIDs := []uuid.UUID{
			uuid.MustParse("10000000-0000-0000-0000-000000000041"),
			uuid.MustParse("20000000-0000-0000-0000-000000000042"),
			uuid.MustParse("30000000-0000-0000-0000-000000000043"),
		}
		for index, rootID := range rootIDs {
			seedCommonRepRoot(t, db, rootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
			require.NoError(t, insertCommonRepReporter(t, db, rootID, fmt.Sprintf("mid-chunk-root-%d", index), "host", "hbi", false).Error)
		}

		usecase := newTestCommonRepCleanupUsecase(t, db)
		stopped, err := newCommonRepCleanupOptions(100, 4, 0, 1, time.Minute)
		require.NoError(t, err)
		firstRun, err := usecase.Execute(context.Background(), policy, stopped, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(4), firstRun.deletedRows, "SQL caps the batch even across multiple roots")
		assertCommonRepVersions(t, db, rootIDs[0], []uint{3, 4})
		assertCommonRepVersions(t, db, rootIDs[1], []uint{1, 2, 3, 4}, "the partial root loses its oldest version first")
		assertCommonRepVersions(t, db, rootIDs[2], []uint{0, 1, 2, 3, 4})
		assert.True(t, firstRun.batchLimitReached)
		assert.False(t, firstRun.report.passComplete)
		assert.Equal(t, uuid.Nil, firstRun.report.lastCursor, "an interrupted chunk does not advance the cursor past unfinished rows")

		resume, err := newCommonRepCleanupOptions(100, 4, 0, 5, time.Minute)
		require.NoError(t, err)
		secondRun, err := usecase.Execute(context.Background(), policy, resume, firstRun.report.lastCursor, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(5), secondRun.deletedRows, "the resume prunes the interrupted chunk's remaining rows")
		assert.True(t, secondRun.report.passComplete)
		assert.Equal(t, int64(3), secondRun.report.rootsScanned, "the interrupted chunk is re-scanned wholesale")

		for _, rootID := range rootIDs {
			assertCommonRepVersions(t, db, rootID, []uint{3, 4})
		}
	})

	t.Run("batch size is a hard row limit across roots", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)
		for index := 0; index < 3; index++ {
			rootID := uuid.New()
			seedCommonRepRoot(t, db, rootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
			require.NoError(t, insertCommonRepReporter(t, db, rootID, fmt.Sprintf("hard-limit-%d", index), "host", "hbi", false).Error)
		}
		options, err := newCommonRepCleanupOptions(100, 4, 0, 10, time.Minute)
		require.NoError(t, err)
		result, err := newTestCommonRepCleanupUsecase(t, db).Execute(context.Background(), policy, options, uuid.Nil, func(_ int, impact commonRepCleanupImpact) {
			assert.LessOrEqual(t, impact.representations, int64(4), "each committed batch must honor the row limit")
		}, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(9), result.deletedRows)
	})

	t.Run("a completed pass is idempotent", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		rootID := uuid.New()
		seedCommonRepRoot(t, db, rootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, rootID, "idempotent-root", "host", "hbi", false).Error)

		usecase := newTestCommonRepCleanupUsecase(t, db)
		options, err := newCommonRepCleanupOptions(100, 100, 0, 5, time.Minute)
		require.NoError(t, err)
		first, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), first.deletedRows)
		assert.True(t, first.report.passComplete)

		second, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Zero(t, second.deletedRows, "a second completed pass finds nothing to prune")
		assert.True(t, second.report.passComplete)
		assert.Equal(t, int64(1), second.report.rootsScanned)
		assert.Zero(t, second.report.prunableRows)
	})

	t.Run("injected failure after deletion rolls back the entire batch", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		rootID := uuid.New()
		seedCommonRepRoot(t, db, rootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, rootID, "rollback-root", "host", "hbi", false).Error)

		require.NoError(t, db.Exec(`CREATE OR REPLACE FUNCTION common_rep_test_fail_mid_delete() RETURNS trigger AS $$
			BEGIN
				RAISE EXCEPTION 'injected cleanup failure';
			END $$ LANGUAGE plpgsql`).Error)
		require.NoError(t, db.Exec(`CREATE TRIGGER common_rep_test_rollback_trigger
			AFTER DELETE ON common_representations
			FOR EACH ROW WHEN (OLD.version = 1)
			EXECUTE FUNCTION common_rep_test_fail_mid_delete()`).Error)
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS common_rep_test_rollback_trigger ON common_representations").Error
			_ = db.Exec("DROP FUNCTION IF EXISTS common_rep_test_fail_mid_delete()").Error
		})

		options, err := newCommonRepCleanupOptions(100, 100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.ErrorContains(t, err, "injected cleanup failure")
		assert.Zero(t, result.deletedRows, "a failed batch contributes no committed progress")
		assert.Zero(t, result.committedBatches)
		assertCommonRepVersions(t, db, rootID, []uint{0, 1, 2, 3, 4}, "rows deleted before the injected failure must be restored by rollback")
	})

	t.Run("committed first batches followed by a failed later batch report only committed progress", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		// Deterministic scan order: the deep root is processed before the poison root.
		firstRootID := uuid.MustParse("10000000-0000-0000-0000-000000000021")
		seedCommonRepRoot(t, db, firstRootID, "host", uintPointer(9), 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		require.NoError(t, insertCommonRepReporter(t, db, firstRootID, "committed-first-root", "host", "hbi", false).Error)
		poisonRootID := uuid.MustParse("20000000-0000-0000-0000-000000000022")
		seedCommonRepRoot(t, db, poisonRootID, "host", uintPointer(5), 0, 1, 2, 3, 4, 5)
		require.NoError(t, insertCommonRepReporter(t, db, poisonRootID, "poison-root", "host", "hbi", false).Error)

		require.NoError(t, db.Exec(`CREATE OR REPLACE FUNCTION common_rep_test_fail_poison() RETURNS trigger AS $$
			BEGIN
				RAISE EXCEPTION 'injected poison failure';
			END $$ LANGUAGE plpgsql`).Error)
		require.NoError(t, db.Exec(fmt.Sprintf(`CREATE TRIGGER common_rep_test_poison_trigger
			AFTER DELETE ON common_representations
			FOR EACH ROW WHEN (OLD.resource_id = '%s')
			EXECUTE FUNCTION common_rep_test_fail_poison()`, poisonRootID)).Error)
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS common_rep_test_poison_trigger ON common_representations").Error
			_ = db.Exec("DROP FUNCTION IF EXISTS common_rep_test_fail_poison()").Error
		})

		// A batch size of four commits the first root in two transactions.
		// Its chunk completes before the poison root's transaction fails.
		options, err := newCommonRepCleanupOptions(5, 4, 0, 5, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		var committedCallbacks int
		result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, func(_ int, _ commonRepCleanupImpact) {
			committedCallbacks++
		}, nil)
		require.ErrorContains(t, err, "injected poison failure")
		assert.Equal(t, 2, result.committedBatches, "the two batches before the poison root committed")
		assert.Equal(t, int64(8), result.deletedRows)
		assert.Equal(t, 2, committedCallbacks, "progress notifications reflect committed batches only")
		assertCommonRepVersions(t, db, firstRootID, []uint{8, 9})
		assertCommonRepVersions(t, db, poisonRootID, []uint{0, 1, 2, 3, 4, 5}, "the failed batch's rows are restored by rollback")
	})

	t.Run("serialization failure is retried and committed exactly once", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		rootID := uuid.New()
		seedCommonRepRoot(t, db, rootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, rootID, "retry-root", "host", "hbi", false).Error)

		require.NoError(t, db.Exec("CREATE SEQUENCE IF NOT EXISTS common_rep_test_retry_seq").Error)
		require.NoError(t, db.Exec(`CREATE OR REPLACE FUNCTION common_rep_test_raise_40001_once() RETURNS trigger AS $$
			BEGIN
				IF nextval('common_rep_test_retry_seq') = 1 THEN
					RAISE EXCEPTION 'injected serialization failure' USING ERRCODE = '40001';
				END IF;
				RETURN OLD;
			END $$ LANGUAGE plpgsql`).Error)
		require.NoError(t, db.Exec(`CREATE TRIGGER common_rep_test_retry_trigger
			BEFORE DELETE ON common_representations
			FOR EACH ROW EXECUTE FUNCTION common_rep_test_raise_40001_once()`).Error)
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS common_rep_test_retry_trigger ON common_representations").Error
			_ = db.Exec("DROP FUNCTION IF EXISTS common_rep_test_raise_40001_once()").Error
			_ = db.Exec("DROP SEQUENCE IF EXISTS common_rep_test_retry_seq").Error
		})

		options, err := newCommonRepCleanupOptions(100, 100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		var committedCallbacks int
		result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, func(_ int, _ commonRepCleanupImpact) {
			committedCallbacks++
		}, nil)
		require.NoError(t, err, "the shared transaction manager must retry the injected serialization failure")
		assert.Equal(t, int64(3), result.deletedRows, "rows are deleted exactly once despite the retry")
		assert.Equal(t, 1, result.committedBatches)
		assert.Equal(t, 1, committedCallbacks)
		assertCommonRepVersions(t, db, rootID, []uint{3, 4})
	})

	t.Run("exhausted serialization retries stop the pass with committed progress", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		cleanRootID := uuid.MustParse("10000000-0000-0000-0000-000000000031")
		seedCommonRepRoot(t, db, cleanRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, cleanRootID, "park-clean-root", "host", "hbi", false).Error)
		// The next chunk contains a hot root whose deletions always conflict.
		hotRootID := uuid.MustParse("20000000-0000-0000-0000-000000000032")
		seedCommonRepRoot(t, db, hotRootID, "host", uintPointer(6), 0, 1, 2, 3, 4, 5, 6)
		require.NoError(t, insertCommonRepReporter(t, db, hotRootID, "park-hot-root", "host", "hbi", false).Error)

		// The hot root always raises a serialization failure, so the transaction
		// manager exhausts its retries and the pass stops at the previous cursor.
		require.NoError(t, db.Exec(fmt.Sprintf(`CREATE OR REPLACE FUNCTION common_rep_test_always_40001() RETURNS trigger AS $$
			BEGIN
				RAISE EXCEPTION 'injected serialization failure' USING ERRCODE = '40001';
			END $$ LANGUAGE plpgsql`)).Error)
		require.NoError(t, db.Exec(fmt.Sprintf(`CREATE TRIGGER common_rep_test_hot_root_trigger
			BEFORE DELETE ON common_representations
			FOR EACH ROW WHEN (OLD.resource_id = '%s')
			EXECUTE FUNCTION common_rep_test_always_40001()`, hotRootID)).Error)
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS common_rep_test_hot_root_trigger ON common_representations").Error
			_ = db.Exec("DROP FUNCTION IF EXISTS common_rep_test_always_40001()").Error
		})

		options, err := newCommonRepCleanupOptions(5, 4, 0, 10, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.ErrorContains(t, err, "injected serialization failure")
		assert.Equal(t, int64(3), result.deletedRows, "the clean root still prunes")
		assert.False(t, result.report.passComplete, "a failed chunk must be retried")
		assert.Equal(t, cleanRootID, result.report.lastCursor)
		assertCommonRepVersions(t, db, cleanRootID, []uint{3, 4})
		assertCommonRepVersions(t, db, hotRootID, []uint{0, 1, 2, 3, 4, 5, 6}, "the failed batch keeps its rows for the rerun")
	})

	t.Run("pointer invalidated after scanning excludes the root during deletion", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		rootID := uuid.New()
		seedCommonRepRoot(t, db, rootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, rootID, "pointer-drift", "host", "hbi", false).Error)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)
		options, err := newCommonRepCleanupOptions(100, 2, 0, 10, time.Minute)
		require.NoError(t, err)
		result, err := newTestCommonRepCleanupUsecase(t, db).Execute(context.Background(), policy, options, uuid.Nil, nil, func(_ commonRepChunkProgress) {
			require.NoError(t, db.Exec("UPDATE resource SET common_version = 99 WHERE id = ?", rootID).Error)
		})
		require.NoError(t, err)
		assert.Zero(t, result.deletedRows)
		assertCommonRepVersions(t, db, rootID, []uint{0, 1, 2, 3, 4})
	})

	t.Run("concurrent report between chunk planning and deletion prunes against the new state", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		rootID := uuid.New()
		seedCommonRepRoot(t, db, rootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, rootID, "concurrent-root", "host", "hbi", false).Error)

		gateLogger := &commonRepCleanupPauseLogger{
			Interface: gormlogger.Default.LogMode(gormlogger.Silent),
			entered:   make(chan struct{}, 1),
			release:   make(chan struct{}),
		}
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(gateLogger.release) }) }
		defer release()
		gatedDB := db.Session(&gorm.Session{Logger: gateLogger})
		transactions := newCommonRepCleanupTransaction(gatedDB, data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 5))
		usecase, err := newCommonRepCleanup(transactions)
		require.NoError(t, err)
		options, err := newCommonRepCleanupOptions(100, 100, 0, 1, time.Minute)
		require.NoError(t, err)

		completed := make(chan struct {
			result commonRepCleanupResult
			err    error
		}, 1)
		go func() {
			result, executeErr := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
			completed <- struct {
				result commonRepCleanupResult
				err    error
			}{result, executeErr}
		}()

		// Wait until the chunk scan has classified the root, then commit a
		// concurrent common-bearing write through a separate connection. The
		// delete transaction must re-classify against the new state rather than
		// trusting the plan.
		select {
		case <-gateLogger.entered:
		case <-time.After(10 * time.Second):
			t.Fatal("cleanup chunk scan did not reach classification")
		}
		concurrentDB := db.Session(&gorm.Session{})
		require.NoError(t, concurrentDB.Exec(`INSERT INTO common_representations (resource_id, version, data, reported_by_reporter_type, reported_by_reporter_instance, transaction_id, created_at)
			VALUES (?, 5, jsonb_build_object('workspace_id', 'concurrent'), 'hbi', 'cleanup-instance', 'concurrent-write', now())`, rootID).Error)
		require.NoError(t, concurrentDB.Exec("UPDATE resource SET common_version = 5 WHERE id = ?", rootID).Error)
		release()

		select {
		case outcome := <-completed:
			require.NoError(t, outcome.err, "the chunked pass must adapt to the concurrent write, not fail")
			assert.Equal(t, int64(4), outcome.result.deletedRows, "deletion re-ranks against the concurrent write's new state")
			assertCommonRepVersions(t, db, rootID, []uint{4, 5}, "version 5 from the concurrent write is retained")
		case <-time.After(30 * time.Second):
			t.Fatal("cleanup did not finish after the concurrent write")
		}
	})

	t.Run("post-prune tombstone, revival, and re-report keep root identity and consumer reads", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		const reporterType = "revivalcase"
		resourceUsecase, resourceRepository := commonRepAPIUsecase(t, db, reporterType)

		reportErr := resourceUsecase.ReportResource(
			commonRepAPIAuthzContext(),
			commonRepAPIReportCommand(t, reporterType, "true-revival-root", "report-id-initial"),
		)
		require.NoError(t, reportErr)

		var rootIDText string
		require.NoError(t, db.Raw("SELECT resource_id FROM reporter_resources WHERE local_resource_id = ?", "true-revival-root").Scan(&rootIDText).Error)
		rootID, err := uuid.Parse(rootIDText)
		require.NoError(t, err)
		for version := uint(1); version <= 4; version++ {
			require.NoError(t, insertCommonRepRow(t, db, rootID, version, fmt.Sprintf("true-revival-txid-%d", version)).Error)
		}
		require.NoError(t, db.Exec("UPDATE resource SET common_version = 4 WHERE id = ?", rootID).Error)

		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)
		options, err := newCommonRepCleanupOptions(100, 100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		firstPrune, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), firstPrune.deletedRows)
		assertCommonRepVersions(t, db, rootID, []uint{3, 4})

		// Tombstone the resource through the real API, then revive it by reporting again.
		key := commonRepReporterKey(t, reporterType, "true-revival-root")
		require.NoError(t, resourceUsecase.Delete(commonRepAPIAuthzContext(), key))
		revivalErr := resourceUsecase.ReportResource(
			commonRepAPIAuthzContext(),
			commonRepAPIReportCommand(t, reporterType, "true-revival-root", "report-id-after-revival"),
		)
		require.NoError(t, revivalErr)

		var revivedRootIDText string
		require.NoError(t, db.Raw("SELECT resource_id FROM reporter_resources WHERE local_resource_id = ?", "true-revival-root").Scan(&revivedRootIDText).Error)
		assert.Equal(t, rootIDText, revivedRootIDText, "revival keeps the same resource root")
		assertCommonRepVersions(t, db, rootID, []uint{3, 4, 5}, "common numbering resumes above the retained maximum")

		versionFive := bizmodel.NewVersion(5)
		current, previous, readErr := resourceRepository.FindCurrentAndPreviousVersionedRepresentations(nil, key, &versionFive, bizmodel.OperationTypeUpdated)
		require.NoError(t, readErr, "the consumer update read of v and v-1 must succeed against pruned and revived state")
		require.NotNil(t, current.CommonVersion())
		require.NotNil(t, previous.CommonVersion())
		assert.Equal(t, uint(5), current.CommonVersion().Uint())
		assert.Equal(t, uint(4), previous.CommonVersion().Uint())

		secondPrune, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(1), secondPrune.deletedRows)
		assertCommonRepVersions(t, db, rootID, []uint{4, 5})
	})

	t.Run("randomized histories match a reference model of the retention rule", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)

		scenarios := generateCommonRepPropertyScenarios(t, 20)
		for scenarioIndex, scenario := range scenarios {
			t.Run(fmt.Sprintf("seed-%d", scenario.seed), func(t *testing.T) {
				isolateCommonRepCleanupPostgresSubtest(t, db)
				policy, err := newCommonRepCleanupPolicy(scenario.keepVersions, "")
				require.NoError(t, err)
				seedCommonRepPropertyScenario(t, db, scenario)

				usecase := newTestCommonRepCleanupUsecase(t, db)
				// Small chunk and batch sizes force chunk boundaries and deep-root loops.
				options, err := newCommonRepCleanupOptions(3, 3, 0, 1000, time.Minute)
				require.NoError(t, err)

				report, err := usecase.Preview(context.Background(), policy, options, uuid.Nil, nil)
				require.NoError(t, err)
				assert.True(t, report.passComplete)
				assert.Equal(t, scenario.expectedPrunableRows, report.prunableRows, "scenario %d preview", scenarioIndex)
				assert.Equal(t, int64(len(scenario.roots)), report.rootsScanned, "scenario %d scanned roots", scenarioIndex)

				result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
				require.NoError(t, err)
				assert.Equal(t, scenario.expectedPrunableRows, result.deletedRows, "scenario %d deleted rows", scenarioIndex)
				assert.True(t, result.report.passComplete)

				for rootIndex := range scenario.roots {
					root := scenario.roots[rootIndex]
					expectedVersions, retained := scenario.expectedRetainedByRoot[root.resourceID.String()]
					if !retained {
						expectedVersions = scenario.originalVersionsByRoot[root.resourceID.String()]
					}
					assertCommonRepVersions(t, db, root.resourceID, expectedVersions,
						"scenario %d root %s", scenarioIndex, root.resourceID)
				}
			})
		}
	})
}

// commonRepPropertyRoot is one generated root of a property-test scenario.
type commonRepPropertyRoot struct {
	resourceID      uuid.UUID
	resourceType    string
	versions        []uint
	commonVersion   *uint
	reporterTombsto bool
	hasReporter     bool
}

// commonRepPropertyScenario is one randomized world plus its expected outcome.
type commonRepPropertyScenario struct {
	seed                   int64
	keepVersions           int
	roots                  []commonRepPropertyRoot
	expectedPrunableRows   int64
	expectedRetainedByRoot map[string][]uint
	originalVersionsByRoot map[string][]uint
}

// generateCommonRepPropertyScenarios builds deterministic randomized scenarios.
func generateCommonRepPropertyScenarios(t *testing.T, count int) []commonRepPropertyScenario {
	t.Helper()
	scenarios := make([]commonRepPropertyScenario, 0, count)
	for index := 0; index < count; index++ {
		source := rand.New(rand.NewSource(int64(index + 1))) //nolint:gosec // deterministic scenarios, not security
		keepVersions := 2 + source.Intn(3)
		rootCount := 3 + source.Intn(5)
		scenario := commonRepPropertyScenario{
			seed:                   int64(index + 1),
			keepVersions:           keepVersions,
			expectedRetainedByRoot: map[string][]uint{},
			originalVersionsByRoot: map[string][]uint{},
		}
		for rootIndex := 0; rootIndex < rootCount; rootIndex++ {
			versionCount := 1 + source.Intn(11)
			versionSet := map[uint]struct{}{}
			for len(versionSet) < versionCount {
				versionSet[uint(source.Intn(40))] = struct{}{}
			}
			versions := make([]uint, 0, len(versionSet))
			for version := range versionSet {
				versions = append(versions, version)
			}
			sort.Slice(versions, func(left, right int) bool { return versions[left] < versions[right] })

			root := commonRepPropertyRoot{
				resourceID:   uuid.New(),
				resourceType: []string{"host", "cluster"}[source.Intn(2)],
				versions:     versions,
				hasReporter:  source.Intn(10) > 0,
			}
			if root.hasReporter {
				root.reporterTombsto = source.Intn(4) == 0
			}
			switch source.Intn(3) {
			case 0:
				root.commonVersion = nil
			case 1:
				pointer := versions[source.Intn(len(versions))]
				root.commonVersion = &pointer
			default:
				// A pointer that resolves to nothing excludes the root.
				unresolved := uint(50 + source.Intn(10))
				root.commonVersion = &unresolved
			}

			scenario.roots = append(scenario.roots, root)
			scenario.originalVersionsByRoot[root.resourceID.String()] = versions

			// Reference model: only actively reported roots with resolvable
			// pointers are pruned; retention keeps the newest existing rows at
			// or below the authoritative current version.
			if !root.hasReporter || root.reporterTombsto {
				continue
			}
			currentVersion := uint(0)
			if root.commonVersion != nil {
				resolves := false
				for _, version := range versions {
					if version == *root.commonVersion {
						resolves = true
						break
					}
				}
				if !resolves {
					continue
				}
				currentVersion = *root.commonVersion
			} else {
				currentVersion = versions[len(versions)-1]
			}
			ranked := make([]uint, 0, len(versions))
			for _, version := range versions {
				if version <= currentVersion {
					ranked = append(ranked, version)
				}
			}
			// ranked ascends; retain the newest keepVersions rows.
			retainedCount := keepVersions
			if retainedCount > len(ranked) {
				retainedCount = len(ranked)
			}
			retained := ranked[len(ranked)-retainedCount:]
			// Versions above a non-NULL pointer are never ranked and always survive.
			expectedSurvivors := append([]uint(nil), retained...)
			for _, version := range versions {
				if version > currentVersion {
					expectedSurvivors = append(expectedSurvivors, version)
				}
			}
			sort.Slice(expectedSurvivors, func(left, right int) bool { return expectedSurvivors[left] < expectedSurvivors[right] })
			scenario.expectedRetainedByRoot[root.resourceID.String()] = expectedSurvivors
			scenario.expectedPrunableRows += int64(len(ranked) - retainedCount)
		}
		scenarios = append(scenarios, scenario)
	}
	return scenarios
}

// seedCommonRepPropertyScenario loads one generated scenario into the database.
func seedCommonRepPropertyScenario(t *testing.T, db *gorm.DB, scenario commonRepPropertyScenario) {
	t.Helper()
	for _, root := range scenario.roots {
		seedCommonRepRoot(t, db, root.resourceID, root.resourceType, root.commonVersion, root.versions...)
		if root.hasReporter {
			require.NoError(t, insertCommonRepReporter(t, db, root.resourceID,
				fmt.Sprintf("property-%s", root.resourceID), root.resourceType, "hbi", root.reporterTombsto).Error)
		}
	}
}

func setupCommonRepCleanupPostgres(t *testing.T) (*gorm.DB, testcontainers.Container) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	password := uuid.NewString()
	postgresContainer, err := testcontainers.Run(
		ctx,
		"postgres:17.8",
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER":     "common_rep_cleanup",
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       "common_rep_cleanup",
		}),
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(3*time.Minute)),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		if err := postgresContainer.Terminate(cleanupContext); err != nil {
			t.Errorf("terminate common representation cleanup PostgreSQL test container: %v", err)
		}
	})

	host, err := postgresContainer.Host(ctx)
	require.NoError(t, err)
	port, err := postgresContainer.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	dsn := fmt.Sprintf("host=%s port=%s user=common_rep_cleanup password=%s dbname=common_rep_cleanup sslmode=disable", host, port.Port(), password)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(16)
	require.NoError(t, data.Migrate(db, nil))
	// Migrations may bind their GORM session to a dedicated advisory-lock connection.
	require.NoError(t, sqlDB.Close())
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	sqlDB, err = db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(16)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, postgresContainer
}

func isolateCommonRepCleanupPostgresSubtest(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("TRUNCATE TABLE reporter_representations, reporter_resources, common_representations, resource CASCADE").Error)
}

func newTestCommonRepCleanupUsecase(t *testing.T, db *gorm.DB) *commonRepCleanupUsecase {
	t.Helper()
	transactionManager := data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 5)
	usecase, err := newCommonRepCleanup(newCommonRepCleanupTransaction(db, transactionManager))
	require.NoError(t, err)
	return usecase
}

func uintPointer(value uint) *uint { return &value }

// seedCommonRepRoot creates a resource root and its common representation history.
// A nil commonVersion leaves the pointer NULL, exercising the MAX(version) fallback.
func seedCommonRepRoot(t *testing.T, db *gorm.DB, resourceID uuid.UUID, resourceType string, commonVersion *uint, versions ...uint) {
	t.Helper()
	require.NoError(t, db.Exec("INSERT INTO resource (id, type, common_version) VALUES (?, ?, ?)", resourceID, resourceType, commonVersion).Error)
	for _, version := range versions {
		require.NoError(t, insertCommonRepRow(t, db, resourceID, version, fmt.Sprintf("seed-%s-%d", resourceID, version)).Error)
	}
}

func insertCommonRepRow(t *testing.T, db *gorm.DB, resourceID uuid.UUID, version uint, transactionID string) *gorm.DB {
	t.Helper()
	return insertCommonRepRowAt(t, db, resourceID, version, transactionID, time.Now())
}

func insertCommonRepRowAt(t *testing.T, db *gorm.DB, resourceID uuid.UUID, version uint, transactionID string, createdAt time.Time) *gorm.DB {
	t.Helper()
	return db.Exec(
		`INSERT INTO common_representations (resource_id, version, data, reported_by_reporter_type, reported_by_reporter_instance, transaction_id, created_at)
		 VALUES (?, ?, '{"workspace_id": "common-rep-cleanup-test"}'::jsonb, 'hbi', 'cleanup-instance', ?, ?)`,
		resourceID, version, transactionID, createdAt,
	)
}

func insertCommonRepReporter(t *testing.T, db *gorm.DB, resourceID uuid.UUID, localResourceID string, resourceType string, reporterType string, tombstone bool) *gorm.DB {
	t.Helper()
	return db.Exec(
		`INSERT INTO reporter_resources (id, local_resource_id, reporter_type, resource_type, reporter_instance_id, resource_id, api_href, representation_version, generation, tombstone, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'cleanup-instance', ?, ?, 0, 0, ?, now(), now())`,
		uuid.New(), localResourceID, reporterType, resourceType, resourceID, "https://common-rep-cleanup.example.test/"+localResourceID, tombstone,
	)
}

func assertCommonRepVersions(t *testing.T, db *gorm.DB, resourceID uuid.UUID, expected []uint, message ...interface{}) {
	t.Helper()
	var versions []uint
	require.NoError(t, db.Raw("SELECT version FROM common_representations WHERE resource_id = ? ORDER BY version ASC", resourceID).Scan(&versions).Error)
	if len(message) > 0 {
		assert.Equal(t, expected, versions, "%v", message)
	} else {
		assert.Equal(t, expected, versions)
	}
}

func assertCommonRepRootCount(t *testing.T, db *gorm.DB, resourceID uuid.UUID, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM resource WHERE id = ?", resourceID).Scan(&count).Error)
	assert.Equal(t, expected, count)
}

func assertCommonRepReporterRowCount(t *testing.T, db *gorm.DB, resourceID uuid.UUID, expected int64, message ...interface{}) {
	t.Helper()
	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM reporter_resources WHERE resource_id = ?", resourceID).Scan(&count).Error)
	if len(message) > 0 {
		assert.Equal(t, expected, count, "%v", message)
	} else {
		assert.Equal(t, expected, count)
	}
}

func commonRepReporterKey(t *testing.T, reporterType string, localResourceID string) bizmodel.ReporterResourceKey {
	t.Helper()
	localID, err := bizmodel.NewLocalResourceId(localResourceID)
	require.NoError(t, err)
	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	reporterTypeValue, err := bizmodel.NewReporterType(reporterType)
	require.NoError(t, err)
	reporterInstanceID, err := bizmodel.NewReporterInstanceId("cleanup-instance")
	require.NoError(t, err)
	key, err := bizmodel.NewReporterResourceKey(localID, resourceType, reporterTypeValue, reporterInstanceID)
	require.NoError(t, err)
	return key
}

func commonRepAPIUsecase(
	t *testing.T,
	db *gorm.DB,
	reporterType string,
) (*resourceuc.Usecase, bizmodel.ResourceRepository) {
	t.Helper()
	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	reporterTypeValue, err := bizmodel.NewReporterType(reporterType)
	require.NoError(t, err)
	schemaRepository := data.NewInMemorySchemaRepository()
	commonSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema":"http://json-schema.org/draft-07/schema#",
		"type":"object",
		"properties":{"workspace_id":{"type":"string"}},
		"required":["workspace_id"]
	}`)
	reporterSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema":"http://json-schema.org/draft-07/schema#",
		"type":"object",
		"properties":{"local_resource_id":{"type":"string"}},
		"required":[]
	}`)
	resourceSchemaRepresentation, err := bizmodel.NewResourceSchemaRepresentation(resourceType, commonSchema)
	require.NoError(t, err)
	require.NoError(t, schemaRepository.CreateResourceSchema(context.Background(), resourceSchemaRepresentation))
	reporterSchemaRepresentation, err := bizmodel.NewReporterSchemaRepresentation(resourceType, reporterTypeValue, reporterSchema)
	require.NoError(t, err)
	require.NoError(t, schemaRepository.CreateReporterSchema(context.Background(), reporterSchemaRepresentation))

	metrics := metricscollector.NewFakeMetricsCollector()
	transactionManager := data.NewGormTransactionManager(metrics, 5)
	resourceRepository := data.NewResourceRepository(db, transactionManager, data.SetOutboxPublisher(storage.OutboxModeNone))
	relationsRepository := &data.AllowAllRelationsRepository{Logger: log.NewHelper(log.DefaultLogger)}
	usecase := resourceuc.New(
		resourceRepository,
		schemaRepository,
		relationsRepository,
		"common-rep-cleanup-integration",
		log.DefaultLogger,
		nil,
		nil,
		resourceuc.NewUsecaseConfig(),
		metrics,
		nil,
		nil,
	)
	return usecase, resourceRepository
}

func commonRepAPIAuthzContext() context.Context {
	return authnapi.NewAuthzContext(context.Background(), authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject: &authnapi.Claims{
			SubjectId: authnapi.SubjectId("common-rep-cleanup-integration-user"),
			AuthType:  authnapi.AuthTypeXRhIdentity,
		},
	})
}

func commonRepAPIReportCommand(
	t *testing.T,
	reporterType string,
	localResourceID string,
	transactionID string,
) resourceuc.ReportResourceCommand {
	t.Helper()
	localID, err := bizmodel.NewLocalResourceId(localResourceID)
	require.NoError(t, err)
	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	reporterTypeValue, err := bizmodel.NewReporterType(reporterType)
	require.NoError(t, err)
	reporterInstanceID, err := bizmodel.NewReporterInstanceId("cleanup-instance")
	require.NoError(t, err)
	apiHref, err := bizmodel.NewApiHref("https://common-rep-cleanup.example.test/api/" + localResourceID)
	require.NoError(t, err)
	consoleHref, err := bizmodel.NewConsoleHref("https://common-rep-cleanup.example.test/console/" + localResourceID)
	require.NoError(t, err)
	reporterRepresentation, err := bizmodel.NewRepresentation(map[string]any{"local_resource_id": localResourceID})
	require.NoError(t, err)
	commonRepresentation, err := bizmodel.NewRepresentation(map[string]any{"workspace_id": "common-rep-cleanup-api-workspace"})
	require.NoError(t, err)
	transactionIDValue := bizmodel.NewTransactionId(transactionID)
	return resourceuc.ReportResourceCommand{
		LocalResourceId:        localID,
		ResourceType:           resourceType,
		ReporterType:           reporterTypeValue,
		ReporterInstanceId:     reporterInstanceID,
		ApiHref:                apiHref,
		ConsoleHref:            &consoleHref,
		TransactionId:          &transactionIDValue,
		ReporterRepresentation: &reporterRepresentation,
		CommonRepresentation:   &commonRepresentation,
		WriteVisibility:        resourceuc.WriteVisibilityMinimizeLatency,
	}
}

// commonRepCleanupPauseLogger pauses the cleanup chunk scan after classification
// so a test can commit a concurrent write before the pass proceeds to deletion.
type commonRepCleanupPauseLogger struct {
	gormlogger.Interface
	entered chan struct{}
	release chan struct{}
}

func (logger *commonRepCleanupPauseLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	if strings.Contains(sql, "WITH chunk AS") {
		select {
		case logger.entered <- struct{}{}:
		default:
		}
		<-logger.release
	}
	logger.Interface.Trace(ctx, begin, fc, err)
}
