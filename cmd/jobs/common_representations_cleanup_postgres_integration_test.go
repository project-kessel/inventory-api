//go:build integration

package jobs

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestCommonRepCleanupPostgres verifies retention and transactional worklist behavior
// against real PostgreSQL, including cancellation, rollback, and deep histories.
func TestCommonRepCleanupPostgres(t *testing.T) {
	db, _ := setupCommonRepCleanupPostgres(t)
	options := commonRepCleanupOptions{keepVersions: 2, batchSize: 3, maxBatches: 0}
	base := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	seed := func(versions []uint, ages []int, pointer *uint, resourceType string, active bool) uuid.UUID {
		t.Helper()
		id := uuid.New()
		seedCommonRepRoot(t, db, id, resourceType, pointer, versions...)
		require.NoError(t, insertCommonRepReporter(t, db, id, id.String(), resourceType, "hbi", !active).Error)
		for index, version := range versions {
			require.NoError(t, db.Exec("UPDATE common_representations SET created_at=? WHERE resource_id=? AND version=?", base.Add(-time.Duration(ages[index])*24*time.Hour), id, version).Error)
		}
		return id
	}
	t.Run("version retention ignores age and timestamps", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		type expectedRoot struct {
			id       uuid.UUID
			versions []uint
		}
		cases := []expectedRoot{
			{seed([]uint{0, 1, 2, 3, 4, 5, 6, 7}, []int{6, 5, 4, 3, 2, 1, 0, 0}, uintPointer(7), "host", true), []uint{6, 7}},
			// Old and recent resources follow the same version-count rule.
			{seed([]uint{0, 1, 2, 3}, []int{120, 119, 118, 100}, uintPointer(3), "host", true), []uint{2, 3}},
			// Even a lower version with the newest timestamp is outside keep-two.
			{seed([]uint{0, 1, 2, 3, 4}, []int{0, 20, 19, 18, 17}, uintPointer(4), "host", true), []uint{3, 4}},
			{seed([]uint{0, 4, 10, 14}, []int{40, 30, 20, 0}, uintPointer(14), "host", true), []uint{10, 14}},
			{seed([]uint{0, 1, 2, 3}, []int{20, 19, 18, 0}, nil, "host", true), []uint{2, 3}},
			// Preserve the authoritative current/predecessor and every above-pointer row.
			{seed([]uint{0, 1, 2, 3, 4}, []int{40, 30, 20, 10, 0}, uintPointer(2), "host", true), []uint{1, 2, 3, 4}},
			{seed([]uint{0}, []int{100}, uintPointer(0), "host", true), []uint{0}},
			{seed([]uint{0, 1}, []int{100, 0}, uintPointer(1), "host", true), []uint{0, 1}},
			{seed([]uint{0, 1, 2}, []int{40, 30, 0}, uintPointer(99), "host", true), []uint{0, 1, 2}},
			{seed([]uint{0, 1, 2}, []int{40, 30, 0}, uintPointer(2), "host", false), []uint{0, 1, 2}},
		}
		require.NoError(t, insertCommonRepReporter(t, db, cases[0].id, "second-reporter", "host", "ocm", false).Error)
		noReporter := seed([]uint{0, 1, 2}, []int{40, 30, 0}, uintPointer(2), "host", true)
		require.NoError(t, db.Exec("DELETE FROM reporter_resources WHERE resource_id=?", noReporter).Error)
		otherType := seed([]uint{0, 1, 2}, []int{40, 30, 0}, uintPointer(2), "other", true)
		filtered := options
		filtered.resourceType = "host"
		preview, err := cleanupCommonRepresentations(context.Background(), db, filtered, false, nil)
		require.NoError(t, err)
		require.Equal(t, int64(16), preview.candidateRows)
		require.Zero(t, preview.deletedRows)
		assertCommonRepVersions(t, db, cases[0].id, []uint{0, 1, 2, 3, 4, 5, 6, 7})
		keepThree := filtered
		keepThree.keepVersions = 3
		query, args := commonRepCleanupCandidateQuery(keepThree)
		var staleCandidates []uint
		require.NoError(t, db.Raw("SELECT version FROM (?) candidates WHERE resource_id=? ORDER BY version", gorm.Expr(query, args...), cases[1].id).Scan(&staleCandidates).Error)
		assert.Equal(t, []uint{0}, staleCandidates)

		// Stop after one batch, then rebuild the worklist from remaining history.
		firstOptions := filtered
		firstOptions.maxBatches = 1
		first, err := cleanupCommonRepresentations(context.Background(), db, firstOptions, true, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), first.deletedRows)
		assert.Equal(t, int64(13), first.remainingRows)
		assert.False(t, first.complete)
		second, err := cleanupCommonRepresentations(context.Background(), db, filtered, true, func(_ int, rows int64, _ time.Duration) { assert.LessOrEqual(t, rows, int64(3)) })
		require.NoError(t, err)
		assert.Equal(t, int64(13), second.deletedRows)
		assert.True(t, second.complete)
		for _, item := range cases {
			assertCommonRepVersions(t, db, item.id, item.versions)
		}
		assertCommonRepVersions(t, db, noReporter, []uint{0, 1, 2})
		assertCommonRepVersions(t, db, otherType, []uint{0, 1, 2})
		final, err := cleanupCommonRepresentations(context.Background(), db, filtered, true, nil)
		require.NoError(t, err)
		assert.Zero(t, final.deletedRows)
		assert.True(t, final.complete)
	})

	t.Run("cancellation preserves committed work and cleans pooled session", func(t *testing.T) {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		defer sqlDB.SetMaxOpenConns(16)
		isolateCommonRepCleanupPostgresSubtest(t, db)
		id := seed([]uint{0, 1, 2, 3, 4, 5, 6}, []int{40, 39, 38, 37, 36, 35, 0}, uintPointer(6), "host", true)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result, err := cleanupCommonRepresentations(ctx, db, options, true, func(_ int, _ int64, _ time.Duration) { cancel() })
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int64(3), result.deletedRows)
		assert.Equal(t, 1, result.committedBatches)
		assertCommonRepVersions(t, db, id, []uint{3, 4, 5, 6})
		// Force reuse of the same pool: no abandoned temporary table may remain.
		rest, err := cleanupCommonRepresentations(context.Background(), db, options, true, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(2), rest.deletedRows)
		assertCommonRepVersions(t, db, id, []uint{5, 6})
	})

	t.Run("failure after a successful batch counts only committed rows", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		id := seed([]uint{0, 1, 2, 3, 4, 5, 6}, []int{40, 39, 38, 37, 36, 35, 0}, uintPointer(6), "host", true)
		require.NoError(t, db.Exec(`CREATE FUNCTION common_rep_test_fail_delete() RETURNS trigger AS $$
   BEGIN IF OLD.version = 4 THEN RAISE EXCEPTION 'injected cleanup failure'; END IF; RETURN OLD; END; $$ LANGUAGE plpgsql`).Error)
		require.NoError(t, db.Exec(`CREATE TRIGGER common_rep_test_rollback_trigger BEFORE DELETE ON common_representations FOR EACH ROW EXECUTE FUNCTION common_rep_test_fail_delete()`).Error)
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS common_rep_test_rollback_trigger ON common_representations").Error
			_ = db.Exec("DROP FUNCTION IF EXISTS common_rep_test_fail_delete()").Error
		})
		result, err := cleanupCommonRepresentations(context.Background(), db, options, true, nil)
		require.ErrorContains(t, err, "injected cleanup failure")
		assert.Equal(t, int64(3), result.deletedRows)
		assert.Equal(t, 1, result.committedBatches)
		assertCommonRepVersions(t, db, id, []uint{3, 4, 5, 6})
	})

	t.Run("history and temporary worklist roll back together", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		id := seed([]uint{0, 1, 2, 3}, []int{40, 30, 20, 0}, uintPointer(3), "host", true)
		require.NoError(t, db.Connection(func(connection *gorm.DB) error {
			require.NoError(t, connection.Exec("CREATE TEMP TABLE common_rep_cleanup_candidates (resource_id uuid, version bigint, PRIMARY KEY(resource_id, version))").Error)
			defer connection.Exec("DROP TABLE pg_temp.common_rep_cleanup_candidates")
			query, args := commonRepCleanupCandidateQuery(options)
			require.NoError(t, connection.Exec("INSERT INTO pg_temp.common_rep_cleanup_candidates ?", gorm.Expr(query, args...)).Error)
			rollback := errors.New("rollback batch")
			err := commonRepCleanupTransaction(context.Background(), connection, false, func(tx *gorm.DB) error {
				deleted, err := deleteCommonRepCleanupBatch(tx, 1)
				require.NoError(t, err)
				require.Equal(t, int64(1), deleted)
				return rollback
			})
			require.ErrorIs(t, err, rollback)
			var count int64
			require.NoError(t, connection.Raw("SELECT COUNT(*) FROM pg_temp.common_rep_cleanup_candidates").Scan(&count).Error)
			assert.Equal(t, int64(2), count)
			assertCommonRepVersions(t, db, id, []uint{0, 1, 2, 3})
			// A missing history row must not be silently skipped from the worklist.
			require.NoError(t, connection.Exec("INSERT INTO pg_temp.common_rep_cleanup_candidates VALUES (?, -1)", id).Error)
			err = commonRepCleanupTransaction(context.Background(), connection, false, func(tx *gorm.DB) error { _, err := deleteCommonRepCleanupBatch(tx, 3); return err })
			require.ErrorContains(t, err, "worklist mismatch")
			require.NoError(t, connection.Raw("SELECT COUNT(*) FROM pg_temp.common_rep_cleanup_candidates").Scan(&count).Error)
			assert.Equal(t, int64(3), count)
			return nil
		}))
	})

	t.Run("deep and multi-resource selection timing", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		deep := uuid.New()
		seedCommonRepRoot(t, db, deep, "host", uintPointer(49999))
		require.NoError(t, insertCommonRepReporter(t, db, deep, "deep", "host", "hbi", false).Error)
		require.NoError(t, db.Exec(`INSERT INTO common_representations(resource_id, version, data, reported_by_reporter_type, reported_by_reporter_instance, transaction_id, created_at)
   SELECT ?, n, '{}'::jsonb, 'hbi', 'cleanup-instance', 'deep-'||n,
   CASE WHEN n=49999 THEN now() ELSE now()-INTERVAL '30 days' END FROM generate_series(0,49999) n`, deep).Error)
		require.NoError(t, db.Exec(`INSERT INTO resource(id,type,common_version)
   SELECT ('00000000-0000-4000-8000-'||lpad(n::text,12,'0'))::uuid,'host',4 FROM generate_series(1,10000) n`).Error)
		require.NoError(t, db.Exec(`INSERT INTO reporter_resources(id,local_resource_id,reporter_type,resource_type,reporter_instance_id,resource_id,tombstone,api_href,representation_version,generation)
   SELECT ('00000000-0000-4000-9000-'||lpad(n::text,12,'0'))::uuid,'perf-'||n,'hbi','host','cleanup-instance',('00000000-0000-4000-8000-'||lpad(n::text,12,'0'))::uuid,FALSE,'https://example.test',0,0 FROM generate_series(1,10000) n`).Error)
		require.NoError(t, db.Exec(`INSERT INTO common_representations(resource_id,version,data,reported_by_reporter_type,reported_by_reporter_instance,transaction_id,created_at)
   SELECT ('00000000-0000-4000-8000-'||lpad(r::text,12,'0'))::uuid,v,'{}'::jsonb,'hbi','cleanup-instance','perf-'||r||'-'||v,
   CASE WHEN v=4 THEN now() ELSE now()-INTERVAL '30 days' END FROM generate_series(1,10000) r CROSS JOIN generate_series(0,4) v`).Error)
		perf := options
		perf.batchSize = 1000
		perf.maxBatches = 1
		result, err := cleanupCommonRepresentations(context.Background(), db, perf, true, func(_ int, _ int64, duration time.Duration) {
			t.Logf("100,000 rows: first 1,000-row deletion=%s", duration)
		})
		require.NoError(t, err)
		assert.Equal(t, int64(79998), result.candidateRows)
		assert.Equal(t, int64(1000), result.deletedRows)
		t.Logf("100,000 rows including 50,000-version root: worklist selection=%s", result.selectionDuration)
	})
}

// setupCommonRepCleanupPostgres starts an isolated database with the real schema
// and registers container and connection cleanup with the test.
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

// isolateCommonRepCleanupPostgresSubtest clears resource fixtures between subtests.
func isolateCommonRepCleanupPostgresSubtest(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("TRUNCATE TABLE reporter_representations, reporter_resources, common_representations, resource CASCADE").Error)
}

// uintPointer supplies a nullable current-version value for test fixtures.
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

// insertCommonRepRow inserts a history version reported at the current time.
func insertCommonRepRow(t *testing.T, db *gorm.DB, resourceID uuid.UUID, version uint, transactionID string) *gorm.DB {
	t.Helper()
	return insertCommonRepRowAt(t, db, resourceID, version, transactionID, time.Now())
}

// insertCommonRepRowAt inserts a history version with a controlled timestamp.
func insertCommonRepRowAt(t *testing.T, db *gorm.DB, resourceID uuid.UUID, version uint, transactionID string, createdAt time.Time) *gorm.DB {
	t.Helper()
	return db.Exec(
		`INSERT INTO common_representations (resource_id, version, data, reported_by_reporter_type, reported_by_reporter_instance, transaction_id, created_at)
		 VALUES (?, ?, '{"workspace_id": "common-rep-cleanup-test"}'::jsonb, 'hbi', 'cleanup-instance', ?, ?)`,
		resourceID, version, transactionID, createdAt,
	)
}

// insertCommonRepReporter associates an active or tombstoned reporter with a root.
func insertCommonRepReporter(t *testing.T, db *gorm.DB, resourceID uuid.UUID, localResourceID string, resourceType string, reporterType string, tombstone bool) *gorm.DB {
	t.Helper()
	return db.Exec(
		`INSERT INTO reporter_resources (id, local_resource_id, reporter_type, resource_type, reporter_instance_id, resource_id, api_href, representation_version, generation, tombstone, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'cleanup-instance', ?, ?, 0, 0, ?, now(), now())`,
		uuid.New(), localResourceID, reporterType, resourceType, resourceID, "https://common-rep-cleanup.example.test/"+localResourceID, tombstone,
	)
}

// assertCommonRepVersions checks the exact retained history in version order.
func assertCommonRepVersions(t *testing.T, db *gorm.DB, resourceID uuid.UUID, expected []uint) {
	t.Helper()
	var versions []uint
	require.NoError(t, db.Raw("SELECT version FROM common_representations WHERE resource_id = ? ORDER BY version ASC", resourceID).Scan(&versions).Error)
	assert.Equal(t, expected, versions)
}
