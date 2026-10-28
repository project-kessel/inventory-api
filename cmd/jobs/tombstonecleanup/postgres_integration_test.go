//go:build integration

package tombstonecleanup

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestTombstoneCleanupPostgres exercises eligibility, cascades and restart behavior
// using the real schema and actual PostgreSQL transactions.
func TestTombstoneCleanupPostgres(t *testing.T) {
	db := setupTombstoneCleanupPostgres(t)
	cutoff := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	old := cutoff.Add(-time.Hour)
	options := options{tombstoneDays: 30, batchSize: 3, cutoff: cutoff}
	seed := func(resourceID uuid.UUID, tombstone bool, updatedAt time.Time, reporterRows, commonRows int) uuid.UUID {
		t.Helper()
		require.NoError(t, db.Exec("INSERT INTO resource(id,type) VALUES (?, 'host') ON CONFLICT DO NOTHING", resourceID).Error)
		id := uuid.New()
		require.NoError(t, db.Exec(`INSERT INTO reporter_resources(id,resource_id,local_resource_id,reporter_type,resource_type,reporter_instance_id,api_href,generation,representation_version,tombstone,updated_at)
		 VALUES (?, ?, ?, 'hbi','host','cleanup-test','https://example.test',0,?,?,?)`, id, resourceID, id.String(), reporterRows, tombstone, updatedAt).Error)
		for version := 0; version < reporterRows; version++ {
			require.NoError(t, db.Exec(`INSERT INTO reporter_representations(reporter_resource_id,generation,version,data,tombstone)
			 VALUES (?,0,?,'{}',FALSE)`, id, version).Error)
		}
		for version := 0; version < commonRows; version++ {
			require.NoError(t, db.Exec("INSERT INTO common_representations(resource_id,version,data) VALUES (?,?,'{}') ON CONFLICT DO NOTHING", resourceID, version).Error)
		}
		return id
	}
	count := func(table, predicate string, arguments ...any) int64 {
		t.Helper()
		var count int64
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM "+table+" WHERE "+predicate, arguments...).Scan(&count).Error)
		return count
	}
	reset := func(t *testing.T) {
		t.Helper()
		require.NoError(t, db.Exec("TRUNCATE TABLE reporter_representations, reporter_resources, common_representations, resource CASCADE").Error)
	}
	t.Run("strict age cutoff and shared roots preserve unrelated data", func(t *testing.T) {
		reset(t)
		deletedRoot := uuid.New()
		deleted := seed(deletedRoot, true, old, 4, 3)
		sharedRoot := uuid.New()
		sharedDeleted := seed(sharedRoot, true, old, 2, 5)
		sharedActive := seed(sharedRoot, false, old, 3, 0)
		recentRoot := uuid.New()
		recentSharedDeleted := seed(recentRoot, true, old, 2, 2)
		recentSharedKept := seed(recentRoot, true, cutoff.Add(time.Hour), 1, 0)
		boundary := seed(uuid.New(), true, cutoff, 2, 2)
		active := seed(uuid.New(), false, old, 2, 2)
		// Existing orphans belong to the separate orphan-cleanup job.
		orphanRoot, missingRoot := uuid.New(), uuid.New()
		require.NoError(t, db.Exec("INSERT INTO resource(id,type) VALUES (?,'host')", orphanRoot).Error)
		for _, root := range []uuid.UUID{orphanRoot, missingRoot} {
			require.NoError(t, db.Exec("INSERT INTO common_representations(resource_id,version,data) VALUES (?,0,'{}')", root).Error)
		}
		preview, err := cleanup(context.Background(), db, options, false, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(3), preview.candidates)
		require.Equal(t, int64(8), preview.reporterHistory)
		require.Equal(t, int64(3), preview.commonHistory)
		require.Equal(t, int64(1), preview.roots)
		require.Zero(t, preview.batches)
		require.Equal(t, int64(1), count("reporter_resources", "id=?", deleted))
		run, err := cleanup(context.Background(), db, options, true, nil, nil)
		require.NoError(t, err)
		require.True(t, run.complete)
		require.Equal(t, int64(3), run.reporterResources)
		require.Equal(t, int64(8), run.reporterHistory)
		require.Equal(t, int64(3), run.commonHistory)
		require.Equal(t, int64(1), run.roots)
		for _, id := range []uuid.UUID{deleted, sharedDeleted, recentSharedDeleted} {
			require.Zero(t, count("reporter_resources", "id=?", id))
			require.Zero(t, count("reporter_representations", "reporter_resource_id=?", id))
		}
		for _, id := range []uuid.UUID{sharedActive, recentSharedKept, boundary, active} {
			require.Equal(t, int64(1), count("reporter_resources", "id=?", id))
		}
		require.Zero(t, count("resource", "id=?", deletedRoot))
		require.Equal(t, int64(5), count("common_representations", "resource_id=?", sharedRoot))
		require.Equal(t, int64(2), count("common_representations", "resource_id=?", recentRoot))
		require.Equal(t, int64(1), count("resource", "id=?", orphanRoot))
		for _, root := range []uuid.UUID{orphanRoot, missingRoot} {
			require.Equal(t, int64(1), count("common_representations", "resource_id=?", root))
		}
	})
	t.Run("oldest first regardless of history size with deterministic ID ties", func(t *testing.T) {
		reset(t)
		smallRoot, largeRoot := uuid.New(), uuid.New()
		small := seed(smallRoot, true, old.Add(-time.Hour), 1, 2)
		large := seed(largeRoot, true, old.Add(time.Minute), 5, 3)
		tieOlder := seed(uuid.New(), true, old.Add(-time.Minute), 3, 1)
		tieA := seed(uuid.New(), true, old, 3, 1)
		tieB := seed(uuid.New(), true, old, 3, 1)
		if tieA.String() > tieB.String() {
			tieA, tieB = tieB, tieA
		}
		empty := seed(uuid.New(), true, old.Add(-2*time.Hour), 0, 0)
		capped := options
		capped.batchSize, capped.maxBatches = 1, 1
		preview, err := cleanup(context.Background(), db, capped, false, nil, nil)
		require.NoError(t, err)
		require.Zero(t, preview.reporterHistory)
		require.Zero(t, preview.commonHistory)
		require.Equal(t, int64(15), preview.totalReporterHistory)
		require.Equal(t, int64(1), count("reporter_resources", "id=?", large))
		all := capped
		all.maxBatches = 0
		expected := []uuid.UUID{empty, small, tieOlder, tieA, tieB, large}
		run, err := cleanup(context.Background(), db, all, true, nil, func(progress result) {
			for index, id := range expected {
				remaining := int64(1)
				if index < progress.batches {
					remaining = 0
				}
				require.Equal(t, remaining, count("reporter_resources", "id=?", id))
			}
		})
		require.NoError(t, err)
		require.True(t, run.complete)
		require.Equal(t, len(expected), run.batches)
	})
	t.Run("revived worklist entries are skipped and drained", func(t *testing.T) {
		reset(t)
		root := uuid.New()
		id := seed(root, true, old, 3, 2)
		run, err := cleanup(context.Background(), db, options, true, func(_ result) {
			require.NoError(t, db.Exec("UPDATE reporter_resources SET tombstone=FALSE,generation=1,representation_version=0,updated_at=? WHERE id=?", cutoff.Add(time.Hour), id).Error)
		}, nil)
		require.NoError(t, err)
		require.True(t, run.complete)
		require.Equal(t, int64(1), run.skipped)
		require.Zero(t, run.reporterResources)
		require.Equal(t, int64(3), count("reporter_representations", "reporter_resource_id=?", id))
		require.Equal(t, int64(2), count("common_representations", "resource_id=?", root))
	})
	t.Run("batch cap cancellation and rerun count only committed work", func(t *testing.T) {
		reset(t)
		for index := 0; index < 5; index++ {
			seed(uuid.New(), true, old, 2, 2)
		}
		capped := options
		capped.batchSize, capped.maxBatches = 2, 1
		preview, err := cleanup(context.Background(), db, capped, false, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(5), preview.candidates)
		require.Equal(t, int64(2), preview.reporterResources)
		require.Equal(t, int64(4), preview.reporterHistory)
		require.Equal(t, int64(10), preview.totalReporterHistory)
		require.Equal(t, int64(2), preview.roots)
		require.Equal(t, int64(5), preview.totalRoots)
		first, err := cleanup(context.Background(), db, capped, true, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(2), first.reporterResources)
		require.Equal(t, int64(3), first.remaining)
		require.False(t, first.complete)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		uncapped := capped
		uncapped.maxBatches = 0
		second, err := cleanup(ctx, db, uncapped, true, nil, func(_ result) { cancel() })
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, 1, second.batches)
		require.Equal(t, int64(2), second.reporterResources)
		require.Equal(t, int64(1), count("reporter_resources", "TRUE"))
		third, err := cleanup(context.Background(), db, uncapped, true, nil, nil)
		require.NoError(t, err)
		require.True(t, third.complete)
		require.Equal(t, int64(1), third.reporterResources)
		final, err := cleanup(context.Background(), db, uncapped, true, nil, nil)
		require.NoError(t, err)
		require.Zero(t, final.candidates)
		require.True(t, final.complete)
	})
	t.Run("root failure rolls back parent cascade and common deletion", func(t *testing.T) {
		reset(t)
		root := uuid.New()
		id := seed(root, true, old, 3, 2)
		require.NoError(t, db.Exec(`CREATE FUNCTION tombstone_cleanup_fail() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'injected root failure'; END; $$ LANGUAGE plpgsql`).Error)
		require.NoError(t, db.Exec(`CREATE TRIGGER tombstone_cleanup_fail BEFORE DELETE ON resource FOR EACH ROW EXECUTE FUNCTION tombstone_cleanup_fail()`).Error)
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS tombstone_cleanup_fail ON resource").Error
			_ = db.Exec("DROP FUNCTION IF EXISTS tombstone_cleanup_fail()").Error
		})
		run, err := cleanup(context.Background(), db, options, true, nil, nil)
		require.ErrorContains(t, err, "injected root failure")
		require.Zero(t, run.batches)
		require.Zero(t, run.reporterResources)
		require.Equal(t, int64(1), count("reporter_resources", "id=?", id))
		require.Equal(t, int64(3), count("reporter_representations", "reporter_resource_id=?", id))
		require.Equal(t, int64(2), count("common_representations", "resource_id=?", root))
	})
	t.Run("revival committed during a batch is preserved after serialization retry", func(t *testing.T) {
		reset(t)
		root := uuid.New()
		id := seed(root, true, old, 3, 2)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		guard, err := sqlDB.Conn(ctx)
		require.NoError(t, err)
		defer guard.Close() //nolint:errcheck
		_, err = guard.ExecContext(ctx, "SELECT pg_advisory_lock(5217601)")
		require.NoError(t, err)
		defer guard.ExecContext(context.Background(), "SELECT pg_advisory_unlock(5217601)") //nolint:errcheck
		// Statement-level blocking lets the writer commit after eligibility was
		// read, but before DELETE takes any reporter row locks.
		require.NoError(t, db.Exec(`CREATE FUNCTION tombstone_cleanup_pause() RETURNS trigger AS $$ BEGIN PERFORM pg_advisory_xact_lock(5217601); RETURN NULL; END; $$ LANGUAGE plpgsql`).Error)
		require.NoError(t, db.Exec(`CREATE TRIGGER tombstone_cleanup_pause BEFORE DELETE ON reporter_resources FOR EACH STATEMENT EXECUTE FUNCTION tombstone_cleanup_pause()`).Error)
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS tombstone_cleanup_pause ON reporter_resources").Error
			_ = db.Exec("DROP FUNCTION IF EXISTS tombstone_cleanup_pause()").Error
		})
		type outcome struct {
			result result
			err    error
		}
		done := make(chan outcome, 1)
		go func() {
			run, err := cleanup(ctx, db, options, true, nil, nil)
			done <- outcome{run, err}
		}()
		require.Eventually(t, func() bool {
			var waiting int64
			return db.Raw("SELECT COUNT(*) FROM pg_locks WHERE locktype='advisory' AND NOT granted").Scan(&waiting).Error == nil && waiting > 0
		}, 3*time.Second, 10*time.Millisecond)
		require.NoError(t, db.Exec("UPDATE reporter_resources SET tombstone=FALSE,generation=1,representation_version=0,updated_at=? WHERE id=?", cutoff.Add(time.Hour), id).Error)
		_, err = guard.ExecContext(ctx, "SELECT pg_advisory_unlock(5217601)")
		require.NoError(t, err)
		select {
		case finished := <-done:
			require.NoError(t, finished.err)
			require.Equal(t, int64(1), finished.result.skipped)
			require.Zero(t, finished.result.reporterResources)
			require.True(t, finished.result.complete)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		require.Equal(t, int64(1), count("reporter_resources", "id=? AND tombstone IS FALSE", id))
		require.Equal(t, int64(3), count("reporter_representations", "reporter_resource_id=?", id))
		require.Equal(t, int64(2), count("common_representations", "resource_id=?", root))
	})
	t.Run("affected roots are unique across batches", func(t *testing.T) {
		reset(t)
		root := uuid.New()
		seed(root, true, old, 2, 3)
		seed(root, true, old, 2, 0)
		single := options
		single.batchSize = 1
		run, err := cleanup(context.Background(), db, single, true, nil, nil)
		require.NoError(t, err)
		require.Equal(t, 2, run.batches)
		require.Equal(t, int64(2), run.reporterResources)
		require.Equal(t, int64(1), run.affected)
		require.Equal(t, int64(1), run.roots)
		require.Equal(t, int64(3), run.commonHistory)
	})
	t.Run("deep history is a parent batch rather than a history row limit", func(t *testing.T) {
		reset(t)
		root := uuid.New()
		id := seed(root, true, old, 0, 0)
		require.NoError(t, db.Exec(`INSERT INTO reporter_representations(reporter_resource_id,generation,version,data,tombstone)
		 SELECT ?,0,n,'{}',FALSE FROM generate_series(0,9999)n`, id).Error)
		require.NoError(t, db.Exec(`INSERT INTO common_representations(resource_id,version,data)
		 SELECT ?,n,'{}' FROM generate_series(0,9999)n`, root).Error)
		deep := options
		deep.batchSize = 1
		run, err := cleanup(context.Background(), db, deep, true, nil, nil)
		require.NoError(t, err)
		require.Equal(t, 1, run.batches)
		require.Equal(t, int64(1), run.reporterResources)
		require.Equal(t, int64(10000), run.reporterHistory)
		require.Equal(t, int64(10000), run.commonHistory)
		require.True(t, run.complete)
	})
}

// setupTombstoneCleanupPostgres starts a private PostgreSQL with real migrations.
func setupTombstoneCleanupPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	password := uuid.NewString()
	container, err := testcontainers.Run(ctx, "postgres:17.8",
		testcontainers.WithEnv(map[string]string{"POSTGRES_USER": "cleanup", "POSTGRES_PASSWORD": password, "POSTGRES_DB": "cleanup"}),
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(3*time.Minute)),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		require.NoError(t, container.Terminate(ctx))
	})
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	dsn := fmt.Sprintf("host=%s port=%s user=cleanup password=%s dbname=cleanup sslmode=disable", host, port.Port(), password)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, data.Migrate(db, nil))
	require.NoError(t, sqlDB.Close()) // Reopen after migrations release their advisory-lock session.
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	sqlDB, err = db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(16)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
