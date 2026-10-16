//go:build integration

package jobs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
