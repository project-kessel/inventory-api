//go:build integration

package jobs

import (
	"context"
	"fmt"
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

		// Active root with no common history at all: not a candidate, never a failure.
		noHistoryRootID := uuid.New()
		seedCommonRepRoot(t, db, noHistoryRootID, "host", nil)
		require.NoError(t, insertCommonRepReporter(t, db, noHistoryRootID, "no-history-root", "host", "hbi", false).Error)

		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)

		report, err := usecase.Preview(context.Background(), policy, options)
		require.NoError(t, err)
		assert.Equal(t, int64(6), report.ScopedActiveRoots())
		assert.Equal(t, int64(1), report.RootsWithoutActiveReporter(), "the tombstoned-only root is out of scope")
		assert.Equal(t, int64(1), report.RootsWithoutCommonHistory(), "the root without common history is not a candidate")
		assert.Equal(t, int64(1), report.ExcludedInconsistentRoots())
		assert.Equal(t, int64(9), report.TotalPrunable().Representations(), "three roots with three prunable versions each")
		assert.Equal(t, int64(3), report.TotalPrunable().ResourceRootsImpacted())
		assert.Equal(t, int64(9), report.BoundedPrunable().Representations())

		// Preview must not mutate anything.
		assertCommonRepVersions(t, db, pointerRootID, []uint{0, 1, 2, 3, 4})
		assertCommonRepVersions(t, db, tombstonedOnlyRootID, []uint{0, 1, 2, 3, 4})

		// Read-only transactions must refuse deletion.
		readOnlyTransaction := newCommonRepCleanupTransaction(db, data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 3))
		readOnlyErr := readOnlyTransaction.WithinSerializableCommonRepCleanup(context.Background(), "common_rep_cleanup_read_only_enforcement_test", true, 30*time.Second, 5*time.Second, func(repository commonRepCleanupRepository) error {
			_, err := repository.DeleteCommonHistoryBatch(context.Background(), policy, 10)
			return err
		})
		require.ErrorContains(t, readOnlyErr, "read-only transaction")

		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, 1, result.CommittedBatches())
		assert.Equal(t, int64(9), result.Impact().Representations())
		assert.Equal(t, int64(3), result.Impact().ResourceRootsImpacted())

		assertCommonRepVersions(t, db, pointerRootID, []uint{3, 4}, "only the retained window survives")
		assertCommonRepVersions(t, db, nullPointerRootID, []uint{3, 4}, "a NULL pointer falls back to MAX(version)")
		assertCommonRepVersions(t, db, sharedRootID, []uint{3, 4})
		assertCommonRepVersions(t, db, freshRootID, []uint{0, 1})
		assertCommonRepVersions(t, db, inconsistentRootID, []uint{0, 1, 2, 3}, "an inconsistent pointer excludes the whole root")
		assertCommonRepVersions(t, db, tombstonedOnlyRootID, []uint{0, 1, 2, 3, 4}, "out-of-scope roots are untouched")
		assertCommonRepVersions(t, db, noHistoryRootID, nil)

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

		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.Impact().Representations())
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

		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)

		report, err := usecase.Preview(context.Background(), policy, options)
		require.NoError(t, err)
		assert.Equal(t, int64(1), report.ScopedActiveRoots())
		assert.Equal(t, int64(3), report.TotalPrunable().Representations())

		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.Impact().Representations())
		assertCommonRepVersions(t, db, clusterRootID, []uint{3, 4})
		assertCommonRepVersions(t, db, hostRootID, []uint{0, 1, 2, 3, 4}, "the unfiltered type is untouched")
	})

	t.Run("bounded batches commit exact accounting and rerun remaining eligibility", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		firstRootID := uuid.New()
		seedCommonRepRoot(t, db, firstRootID, "host", uintPointer(9), 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		require.NoError(t, insertCommonRepReporter(t, db, firstRootID, "batch-first-root", "host", "hbi", false).Error)
		secondRootID := uuid.New()
		seedCommonRepRoot(t, db, secondRootID, "host", uintPointer(9), 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		require.NoError(t, insertCommonRepReporter(t, db, secondRootID, "batch-second-root", "host", "hbi", false).Error)

		usecase := newTestCommonRepCleanupUsecase(t, db)
		boundedOptions, err := newCommonRepCleanupOptions(3, 0, 1, time.Minute)
		require.NoError(t, err)
		report, err := usecase.Preview(context.Background(), policy, boundedOptions)
		require.NoError(t, err)
		assert.Equal(t, int64(16), report.TotalPrunable().Representations())
		assert.Equal(t, int64(3), report.BoundedPrunable().Representations(), "the bounded projection covers one batch prefix")

		firstRun, err := usecase.Execute(context.Background(), policy, boundedOptions, nil)
		require.NoError(t, err)
		assert.Equal(t, 1, firstRun.CommittedBatches())
		assert.True(t, firstRun.BatchLimitReached())
		assert.Equal(t, int64(3), firstRun.Impact().Representations())

		rerunOptions, err := newCommonRepCleanupOptions(3, 0, 5, time.Minute)
		require.NoError(t, err)
		secondRun, err := usecase.Execute(context.Background(), policy, rerunOptions, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(13), secondRun.Impact().Representations(), "the rerun removes the remaining eligibility")
		assertCommonRepVersions(t, db, firstRootID, []uint{8, 9})
		assertCommonRepVersions(t, db, secondRootID, []uint{8, 9})
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

		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)

		report, err := usecase.Preview(context.Background(), policy, options)
		require.NoError(t, err)
		assert.Equal(t, int64(3), report.TotalPrunable().Representations(), "one prunable row per gapped root with three or more rows")

		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.Impact().Representations())

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

		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.Impact().Representations())
		assertCommonRepVersions(t, db, rootID, []uint{3, 4}, "the oldest-timestamp rows survive because ranking is by version")
	})

	t.Run("equal current versions break ties by resource id", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		firstRootID := uuid.MustParse("10000000-0000-0000-0000-00000000000a")
		secondRootID := uuid.MustParse("10000000-0000-0000-0000-00000000000b")
		for _, rootID := range []uuid.UUID{firstRootID, secondRootID} {
			seedCommonRepRoot(t, db, rootID, "host", uintPointer(5), 0, 1, 2, 3, 4, 5)
			require.NoError(t, insertCommonRepReporter(t, db, rootID, "tie-root-"+rootID.String(), "host", "hbi", false).Error)
		}

		options, err := newCommonRepCleanupOptions(4, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(4), result.Impact().Representations())
		assert.Equal(t, int64(1), result.Impact().ResourceRootsImpacted())
		assertCommonRepVersions(t, db, firstRootID, []uint{4, 5}, "the lexicographically smaller resource id wins the tie")
		assertCommonRepVersions(t, db, secondRootID, []uint{0, 1, 2, 3, 4, 5})
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

		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.ErrorContains(t, err, "injected cleanup failure")
		assert.Zero(t, result.Impact().Representations(), "a failed batch contributes no committed progress")
		assert.Zero(t, result.CommittedBatches())
		assertCommonRepVersions(t, db, rootID, []uint{0, 1, 2, 3, 4}, "rows deleted before the injected failure must be restored by rollback")
	})

	t.Run("committed first batch followed by failed later batch reports only committed progress", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		firstRootID := uuid.New()
		seedCommonRepRoot(t, db, firstRootID, "host", uintPointer(9), 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		require.NoError(t, insertCommonRepReporter(t, db, firstRootID, "committed-first-root", "host", "hbi", false).Error)
		poisonRootID := uuid.New()
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

		options, err := newCommonRepCleanupOptions(4, 0, 5, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		var committedCallbacks int
		result, err := usecase.Execute(context.Background(), policy, options, func(_ int, _ commonRepCleanupImpact) {
			committedCallbacks++
		})
		require.ErrorContains(t, err, "injected poison failure")
		assert.Equal(t, 2, result.CommittedBatches(), "the two batches before the poison root committed")
		assert.Equal(t, int64(8), result.Impact().Representations())
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

		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		var committedCallbacks int
		result, err := usecase.Execute(context.Background(), policy, options, func(_ int, _ commonRepCleanupImpact) {
			committedCallbacks++
		})
		require.NoError(t, err, "the shared transaction manager must retry the injected serialization failure")
		assert.Equal(t, int64(3), result.Impact().Representations(), "rows are deleted exactly once despite the retry")
		assert.Equal(t, 1, result.CommittedBatches())
		assert.Equal(t, 1, committedCallbacks)
		assertCommonRepVersions(t, db, rootID, []uint{3, 4})
	})

	t.Run("batches target resources with the most versions first", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		// The low-version root is seeded first; only version-count ordering can
		// guarantee the high-version root is selected ahead of it.
		lowRootID := uuid.New()
		seedCommonRepRoot(t, db, lowRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, lowRootID, "low-version-root", "host", "hbi", false).Error)

		highRootID := uuid.New()
		seedCommonRepRoot(t, db, highRootID, "host", uintPointer(9), 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		require.NoError(t, insertCommonRepReporter(t, db, highRootID, "high-version-root", "host", "hbi", false).Error)

		usecase := newTestCommonRepCleanupUsecase(t, db)
		options, err := newCommonRepCleanupOptions(3, 0, 1, time.Minute)
		require.NoError(t, err)

		report, err := usecase.Preview(context.Background(), policy, options)
		require.NoError(t, err)
		assert.Equal(t, int64(11), report.TotalPrunable().Representations())
		assert.Equal(t, int64(3), report.BoundedPrunable().Representations())
		assert.Equal(t, int64(1), report.BoundedPrunable().ResourceRootsImpacted(), "the bounded prefix must target only the high-version root")

		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.Impact().Representations())
		assert.Equal(t, int64(1), result.Impact().ResourceRootsImpacted())
		assertCommonRepVersions(t, db, highRootID, []uint{3, 4, 5, 6, 7, 8, 9}, "the high-version root loses its oldest three versions")
		assertCommonRepVersions(t, db, lowRootID, []uint{0, 1, 2, 3, 4}, "the low-version root stays untouched while higher-version resources remain")
	})

	t.Run("concurrent report during an in-flight batch is retried against the new state", func(t *testing.T) {
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
		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)

		completed := make(chan struct {
			result commonRepCleanupResult
			err    error
		}, 1)
		go func() {
			result, executeErr := usecase.Execute(context.Background(), policy, options, nil)
			completed <- struct {
				result commonRepCleanupResult
				err    error
			}{result, executeErr}
		}()

		// Wait until the cleanup transaction has selected its candidates, then
		// commit a concurrent common-bearing write through a separate connection.
		select {
		case <-gateLogger.entered:
		case <-time.After(10 * time.Second):
			t.Fatal("cleanup transaction did not reach candidate selection")
		}
		concurrentDB := db.Session(&gorm.Session{})
		require.NoError(t, concurrentDB.Exec(`INSERT INTO common_representations (resource_id, version, data, reported_by_reporter_type, reported_by_reporter_instance, transaction_id, created_at)
			VALUES (?, 5, jsonb_build_object('workspace_id', 'concurrent'), 'hbi', 'cleanup-instance', 'concurrent-write', now())`, rootID).Error)
		require.NoError(t, concurrentDB.Exec("UPDATE resource SET common_version = 5 WHERE id = ?", rootID).Error)
		release()

		select {
		case outcome := <-completed:
			require.NoError(t, outcome.err, "the serialization conflict must be retried, not surfaced")
			assert.Equal(t, int64(3), outcome.result.Impact().Representations(), "the retry prunes against the concurrent write's new state")
			assert.Equal(t, 1, outcome.result.CommittedBatches())
			assertCommonRepVersions(t, db, rootID, []uint{3, 4, 5}, "version 5 from the concurrent write is retained")
		case <-time.After(30 * time.Second):
			t.Fatal("cleanup did not finish after the concurrent write")
		}
	})

	t.Run("post-prune tombstone and revival keeps root identity and resumes numbering", func(t *testing.T) {
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
		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		firstPrune, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), firstPrune.Impact().Representations())
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

		secondPrune, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(1), secondPrune.Impact().Representations())
		assertCommonRepVersions(t, db, rootID, []uint{4, 5})
	})

	t.Run("re-report after pruning resumes numbering and consumer reads stay available", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		const reporterType = "commonrep"
		resourceUsecase, resourceRepository := commonRepAPIUsecase(t, db, reporterType)

		// Create the resource through the real API so the base state is API-valid.
		reportErr := resourceUsecase.ReportResource(
			commonRepAPIAuthzContext(),
			commonRepAPIReportCommand(t, reporterType, "revival-root", "report-id-initial"),
		)
		require.NoError(t, reportErr)

		var rootIDText string
		require.NoError(t, db.Raw("SELECT resource_id FROM reporter_resources WHERE local_resource_id = ?", "revival-root").Scan(&rootIDText).Error)
		rootID, err := uuid.Parse(rootIDText)
		require.NoError(t, err)
		// Advance history to five versions with the pointer at the current version.
		for version := uint(1); version <= 4; version++ {
			require.NoError(t, insertCommonRepRow(t, db, rootID, version, fmt.Sprintf("revival-txid-%d", version)).Error)
		}
		require.NoError(t, db.Exec("UPDATE resource SET common_version = 4 WHERE id = ?", rootID).Error)

		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)
		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		firstPrune, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), firstPrune.Impact().Representations())
		assertCommonRepVersions(t, db, rootID, []uint{3, 4})

		// A new report through the API must resume numbering above the retained maximum.
		reportErr = resourceUsecase.ReportResource(
			commonRepAPIAuthzContext(),
			commonRepAPIReportCommand(t, reporterType, "revival-root", "report-id-after-prune"),
		)
		require.NoError(t, reportErr)

		key := commonRepReporterKey(t, reporterType, "revival-root")
		versionFive := bizmodel.NewVersion(5)
		current, previous, readErr := resourceRepository.FindCurrentAndPreviousVersionedRepresentations(nil, key, &versionFive, bizmodel.OperationTypeUpdated)
		require.NoError(t, readErr, "the consumer update read of v and v-1 must succeed against pruned state")
		require.NotNil(t, current)
		require.NotNil(t, previous)
		require.NotNil(t, current.CommonVersion())
		require.NotNil(t, previous.CommonVersion())
		assert.Equal(t, uint(5), current.CommonVersion().Uint())
		assert.Equal(t, uint(4), previous.CommonVersion().Uint())

		// Pruning again removes the newly superseded version while keeping v and v-1.
		secondPrune, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(1), secondPrune.Impact().Representations(), "only the newly superseded version 3 is pruned")
		assertCommonRepVersions(t, db, rootID, []uint{4, 5})
	})
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

// commonRepCleanupPauseLogger pauses the cleanup transaction after candidate
// selection so a test can commit a concurrent write before the batch proceeds.
type commonRepCleanupPauseLogger struct {
	gormlogger.Interface
	entered chan struct{}
	release chan struct{}
}

func (logger *commonRepCleanupPauseLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	if strings.Contains(sql, "SELECT resource_id, version FROM prunable") {
		select {
		case logger.entered <- struct{}{}:
		default:
		}
		<-logger.release
	}
	logger.Interface.Trace(ctx, begin, fc, err)
}
