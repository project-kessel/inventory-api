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

// TestTombstoneCleanupPostgres checks the row budget against independent table counts.
func TestTombstoneCleanupPostgres(t *testing.T) {
	db := setupTombstoneCleanupPostgres(t)
	cutoff := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	seed := func(reporterRows, commonRows int, active bool) (uuid.UUID, uuid.UUID) {
		root, id := uuid.New(), uuid.New()
		require.NoError(t, db.Exec("INSERT INTO resource(id,type,common_version) VALUES (?,'host',0)", root).Error)
		require.NoError(t, db.Exec(`INSERT INTO reporter_resources(id,resource_id,local_resource_id,reporter_type,resource_type,reporter_instance_id,api_href,generation,representation_version,tombstone,updated_at)
   VALUES (?,?,?,'hbi','host','row-budget','https://example.test',0,0,?,?)`, id, root, id.String(), !active, cutoff.Add(-time.Hour)).Error)
		require.NoError(t, db.Exec(`INSERT INTO reporter_representations(reporter_resource_id,version,generation,data,tombstone) SELECT ?,n,0,'{}',FALSE FROM generate_series(0,?-1)n`, id, reporterRows).Error)
		require.NoError(t, db.Exec(`INSERT INTO common_representations(resource_id,version,data) SELECT ?,n,'{}' FROM generate_series(0,?-1)n`, root, commonRows).Error)
		return root, id
	}
	count := func(query string, args ...any) int64 {
		var n int64
		require.NoError(t, db.Raw(query, args...).Scan(&n).Error)
		return n
	}
	total := func() int64 {
		return count(`SELECT (SELECT COUNT(*) FROM reporter_resources)+(SELECT COUNT(*) FROM reporter_representations)+(SELECT COUNT(*) FROM common_representations)+(SELECT COUNT(*) FROM resource)`)
	}
	reset := func() {
		require.NoError(t, db.Exec("TRUNCATE reporter_representations,reporter_resources,common_representations,resource CASCADE").Error)
	}
	for _, size := range []int{5, 11, 1000} {
		t.Run(fmt.Sprint("budget-", size), func(t *testing.T) {
			reset()
			root, id := seed(37, 39, false)
			active, _ := seed(3, 4, true)
			o := options{tombstoneDays: 30, cutoff: cutoff, rowsPerTransaction: size, transactionLimit: 1}
			before := total()
			preview, err := cleanup(context.Background(), db, o, false, nil, nil, nil)
			require.NoError(t, err)
			require.EqualValues(t, 37, preview.reporterHistory)
			require.Equal(t, before, total())
			previous := total()
			var priorDeleted int64
			verify := func(r result) {
				now := total()
				require.LessOrEqual(t, previous-now, int64(size))
				require.Equal(t, previous-now, r.deletedRows()-priorDeleted)
				previous = now
				priorDeleted = r.deletedRows()
			}
			first, err := cleanup(context.Background(), db, o, true, nil, verify, nil)
			require.NoError(t, err)
			if size < 78 {
				require.False(t, first.complete)
				require.EqualValues(t, 1, count("SELECT COUNT(*) FROM reporter_resources WHERE id=?", id))
				require.EqualValues(t, 1, count("SELECT COUNT(*) FROM reporter_representations WHERE reporter_resource_id=? AND version=0", id))
			}
			o.transactionLimit = 0
			priorDeleted = 0
			previous = total()
			run, err := cleanup(context.Background(), db, o, true, nil, verify, nil)
			require.NoError(t, err)
			require.True(t, run.complete)
			require.Zero(t, count("SELECT COUNT(*) FROM resource WHERE id=?", root))
			require.EqualValues(t, 4, count("SELECT COUNT(*) FROM common_representations WHERE resource_id=?", active))
			again, err := cleanup(context.Background(), db, o, true, nil, nil, nil)
			require.NoError(t, err)
			require.Zero(t, again.candidates)
		})
	}
	t.Run("shared roots changed candidates and cancellation", func(t *testing.T) {
		reset()
		root, id := seed(19, 23, false)
		require.NoError(t, db.Exec(`INSERT INTO reporter_resources(id,resource_id,local_resource_id,reporter_type,resource_type,reporter_instance_id,api_href,console_href,generation,representation_version,tombstone,created_at,updated_at) SELECT gen_random_uuid(),resource_id,local_resource_id||'-active',reporter_type,resource_type,reporter_instance_id,api_href,console_href,generation,representation_version,FALSE,created_at,updated_at FROM reporter_resources WHERE id=?`, id).Error)
		o := options{tombstoneDays: 30, cutoff: cutoff, rowsPerTransaction: 5}
		ctx, cancel := context.WithCancel(context.Background())
		partial, err := cleanup(ctx, db, o, true, nil, func(result) { cancel() }, nil)
		require.ErrorIs(t, err, context.Canceled)
		require.EqualValues(t, 5, partial.reporterHistory)
		run, err := cleanup(context.Background(), db, o, true, nil, nil, nil)
		require.NoError(t, err)
		require.True(t, run.complete)
		require.EqualValues(t, 23, count("SELECT COUNT(*) FROM common_representations WHERE resource_id=?", root))
		_, changed := seed(3, 4, false)
		run, err = cleanup(context.Background(), db, o, true, func(result) {
			require.NoError(t, db.Exec("UPDATE reporter_resources SET updated_at=? WHERE id=?", cutoff, changed).Error)
		}, nil, nil)
		require.NoError(t, err)
		require.EqualValues(t, 1, run.skipped)
	})
	t.Run("later failure rolls back finalization but retains earlier chunks", func(t *testing.T) {
		reset()
		root, id := seed(13, 17, false)
		require.NoError(t, db.Exec(`CREATE FUNCTION fail_root() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'injected root failure'; END; $$ LANGUAGE plpgsql`).Error)
		require.NoError(t, db.Exec(`CREATE TRIGGER fail_root BEFORE DELETE ON resource FOR EACH ROW EXECUTE FUNCTION fail_root()`).Error)
		run, err := cleanup(context.Background(), db, options{tombstoneDays: 30, cutoff: cutoff, rowsPerTransaction: 5}, true, nil, nil, nil)
		require.ErrorContains(t, err, "injected root failure")
		require.Greater(t, run.batches, 0)
		require.EqualValues(t, 1, count("SELECT COUNT(*) FROM reporter_resources WHERE id=?", id))
		require.EqualValues(t, 1, count("SELECT COUNT(*) FROM resource WHERE id=?", root))
		require.EqualValues(t, 1, count("SELECT COUNT(*) FROM reporter_representations WHERE reporter_resource_id=?", id))
		require.EqualValues(t, 2, count("SELECT COUNT(*) FROM common_representations WHERE resource_id=?", root))
		require.NoError(t, db.Exec("DROP TRIGGER fail_root ON resource").Error)
		require.NoError(t, db.Exec("DROP FUNCTION fail_root()").Error)
	})
	t.Run("new reporter between common chunks preserves the surviving root", func(t *testing.T) {
		reset()
		root, id := seed(1, 39, false)
		added := false
		run, err := cleanup(context.Background(), db, options{tombstoneDays: 30, cutoff: cutoff, rowsPerTransaction: 5}, true, nil, func(progress result) {
			if !added && progress.commonHistory > 0 {
				added = true
				require.NoError(t, db.Exec(`INSERT INTO reporter_resources(id,resource_id,local_resource_id,reporter_type,resource_type,reporter_instance_id,api_href,generation,representation_version,tombstone,updated_at)
      VALUES (?,?,?,'hbi','host','new-reporter','https://example.test',0,0,FALSE,?)`, uuid.New(), root, uuid.NewString(), cutoff).Error)
				require.NoError(t, db.Exec("INSERT INTO common_representations(resource_id,version,data) VALUES (?,999,'{}')", root).Error)
			}
		}, nil)
		require.NoError(t, err)
		require.True(t, added)
		require.True(t, run.complete)
		require.EqualValues(t, 5, run.commonHistory)
		require.EqualValues(t, 35, count("SELECT COUNT(*) FROM common_representations WHERE resource_id=?", root))
		require.EqualValues(t, 3, count("SELECT COUNT(*) FROM common_representations WHERE resource_id=? AND version IN (0,38,999)", root))
		require.Zero(t, count("SELECT COUNT(*) FROM reporter_resources WHERE id=?", id))
		require.EqualValues(t, 1, count("SELECT COUNT(*) FROM resource WHERE id=?", root))
	})
	t.Run("shallow parents share a transaction", func(t *testing.T) {
		reset()
		for i := 0; i < 4; i++ {
			seed(1, 1, false)
		}
		run, err := cleanup(context.Background(), db, options{tombstoneDays: 30, cutoff: cutoff, rowsPerTransaction: 16}, true, nil, nil, nil)
		require.NoError(t, err)
		require.Equal(t, 1, run.batches)
		require.EqualValues(t, 16, run.deletedRows())
	})
}

func TestTombstoneCleanupGuards(t *testing.T) {
	db := setupTombstoneCleanupPostgres(t)
	cutoff := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	old := cutoff.Add(-time.Hour)
	options := options{tombstoneDays: 30, rowsPerTransaction: 1000, cutoff: cutoff}
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
	count := func(query string, arguments ...any) int64 {
		t.Helper()
		var count int64
		require.NoError(t, db.Raw(query, arguments...).Scan(&count).Error)
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
		preview, err := cleanup(context.Background(), db, options, false, nil, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(3), preview.candidates)
		require.Equal(t, int64(8), preview.reporterHistory)
		require.Equal(t, int64(3), preview.commonHistory)
		require.Equal(t, int64(1), preview.roots)
		require.Zero(t, preview.batches)
		require.Equal(t, int64(1), count("SELECT COUNT(*) FROM reporter_resources WHERE id=?", deleted))
		run, err := cleanup(context.Background(), db, options, true, nil, nil, nil)
		require.NoError(t, err)
		require.True(t, run.complete)
		require.Equal(t, int64(3), run.reporterResources)
		require.Equal(t, int64(8), run.reporterHistory)
		require.Equal(t, int64(3), run.commonHistory)
		require.Equal(t, int64(1), run.roots)
		for _, id := range []uuid.UUID{deleted, sharedDeleted, recentSharedDeleted} {
			require.Zero(t, count("SELECT COUNT(*) FROM reporter_resources WHERE id=?", id))
			require.Zero(t, count("SELECT COUNT(*) FROM reporter_representations WHERE reporter_resource_id=?", id))
		}
		for _, id := range []uuid.UUID{sharedActive, recentSharedKept, boundary, active} {
			require.Equal(t, int64(1), count("SELECT COUNT(*) FROM reporter_resources WHERE id=?", id))
		}
		require.Zero(t, count("SELECT COUNT(*) FROM resource WHERE id=?", deletedRoot))
		require.Equal(t, int64(5), count("SELECT COUNT(*) FROM common_representations WHERE resource_id=?", sharedRoot))
		require.Equal(t, int64(2), count("SELECT COUNT(*) FROM common_representations WHERE resource_id=?", recentRoot))
		require.Equal(t, int64(1), count("SELECT COUNT(*) FROM resource WHERE id=?", orphanRoot))
		for _, root := range []uuid.UUID{orphanRoot, missingRoot} {
			require.Equal(t, int64(1), count("SELECT COUNT(*) FROM common_representations WHERE resource_id=?", root))
		}
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
			run, err := cleanup(ctx, db, options, true, nil, nil, nil)
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
		require.Equal(t, int64(1), count("SELECT COUNT(*) FROM reporter_resources WHERE id=? AND tombstone IS FALSE", id))
		require.Equal(t, int64(3), count("SELECT COUNT(*) FROM reporter_representations WHERE reporter_resource_id=?", id))
		require.Equal(t, int64(2), count("SELECT COUNT(*) FROM common_representations WHERE resource_id=?", root))
	})
	t.Run("concurrent child insertion cannot hide deletions in the FK cascade", func(t *testing.T) {
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
			run, err := cleanup(ctx, db, options, true, nil, nil, nil)
			done <- outcome{run, err}
		}()
		require.Eventually(t, func() bool {
			var waiting int64
			return db.Raw("SELECT COUNT(*) FROM pg_locks WHERE locktype='advisory' AND NOT granted").Scan(&waiting).Error == nil && waiting > 0
		}, 3*time.Second, 10*time.Millisecond)
		require.NoError(t, db.Exec("INSERT INTO reporter_representations(reporter_resource_id,version,generation,data,tombstone) VALUES (?,999,0,'{}',FALSE)", id).Error)
		_, err = guard.ExecContext(ctx, "SELECT pg_advisory_unlock(5217601)")
		require.NoError(t, err)
		select {
		case finished := <-done:
			require.NoError(t, finished.err)
			require.Zero(t, finished.result.skipped)
			require.EqualValues(t, 4, finished.result.reporterHistory)
			require.EqualValues(t, 1, finished.result.reporterResources)
			require.True(t, finished.result.complete)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		require.Zero(t, count("SELECT COUNT(*) FROM reporter_resources WHERE id=?", id))
		require.Zero(t, count("SELECT COUNT(*) FROM reporter_representations WHERE reporter_resource_id=?", id))
		require.Zero(t, count("SELECT COUNT(*) FROM common_representations WHERE resource_id=?", root))
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
