//go:build integration

package tombstonecleanup

import (
	"bytes"
	"context"
	"fmt"
	"sort"
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

var cutoff = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// fixture seeds migrated tables directly and counts rows independently of the job.
type fixture struct {
	t  *testing.T
	db *gorm.DB
}

// root inserts a resource whose common history has versions 0..commonRows-1 and
// whose pointer references version 0.
func (f fixture) root(commonRows int) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	require.NoError(f.t, f.db.Exec("INSERT INTO resource(id,type,common_version) VALUES (?,'host',0)", id).Error)
	f.common(id, 0, commonRows)
	return id
}

func (f fixture) common(root uuid.UUID, from, rows int) {
	f.t.Helper()
	require.NoError(f.t, f.db.Exec("INSERT INTO common_representations(resource_id,version,data) SELECT ?,n,'{}' FROM generate_series(?::int,?::int-1) n", root, from, from+rows).Error)
}

// reporter inserts a parent whose reporter history has versions 0..reporterRows-1;
// the last version is its current snapshot. A nil updatedAt stores NULL.
func (f fixture) reporter(root uuid.UUID, tombstone bool, updatedAt *time.Time, reporterRows int) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	require.NoError(f.t, f.db.Exec(`INSERT INTO reporter_resources(id,resource_id,local_resource_id,reporter_type,resource_type,reporter_instance_id,api_href,generation,representation_version,tombstone,updated_at)
  VALUES (?,?,?,'hbi','host','cleanup-test','https://example.test',0,?,?,?)`, id, root, id.String(), max(reporterRows-1, 0), tombstone, updatedAt).Error)
	require.NoError(f.t, f.db.Exec("INSERT INTO reporter_representations(reporter_resource_id,version,generation,data,tombstone) SELECT ?,n,0,'{}',FALSE FROM generate_series(0,?::int-1) n", id, reporterRows).Error)
	return id
}

func (f fixture) count(query string, args ...any) int64 {
	f.t.Helper()
	var n int64
	require.NoError(f.t, f.db.Raw(query, args...).Scan(&n).Error)
	return n
}

func (f fixture) parentExists(id uuid.UUID) bool {
	return f.count("SELECT COUNT(*) FROM reporter_resources WHERE id=?", id) == 1
}

func (f fixture) rootExists(id uuid.UUID) bool {
	return f.count("SELECT COUNT(*) FROM resource WHERE id=?", id) == 1
}

func (f fixture) reporterRows(id uuid.UUID) int64 {
	return f.count("SELECT COUNT(*) FROM reporter_representations WHERE reporter_resource_id=?", id)
}

func (f fixture) commonRows(root uuid.UUID) int64 {
	return f.count("SELECT COUNT(*) FROM common_representations WHERE resource_id=?", root)
}

func (f fixture) total() int64 {
	return f.count(`SELECT (SELECT COUNT(*) FROM reporter_resources)+(SELECT COUNT(*) FROM reporter_representations)
  +(SELECT COUNT(*) FROM common_representations)+(SELECT COUNT(*) FROM resource)`)
}

func (f fixture) reset() {
	f.t.Helper()
	require.NoError(f.t, f.db.Exec("TRUNCATE reporter_representations,reporter_resources,common_representations,resource CASCADE").Error)
}

func at(offset time.Duration) *time.Time {
	value := cutoff.Add(offset)
	return &value
}

func runCleanup(t *testing.T, db *gorm.DB, rowsPerTransaction, maxBatches int, report progress) (result, error) {
	t.Helper()
	return cleanup(context.Background(), db, options{cutoff: cutoff, rowsPerTransaction: rowsPerTransaction, maxBatches: maxBatches}, true, report)
}

func TestTombstoneCleanupPostgres(t *testing.T) {
	db := setupTombstoneCleanupPostgres(t)

	t.Run("preview matches execution and preserves protected data", func(t *testing.T) {
		f := fixture{t, db}
		f.reset()
		deletedRoot := f.root(3)
		deleted := f.reporter(deletedRoot, true, at(-time.Hour), 4)
		sharedRoot := f.root(5)
		sharedDeleted := f.reporter(sharedRoot, true, at(-time.Hour), 2)
		sharedActive := f.reporter(sharedRoot, false, at(-time.Hour), 3)
		recentRoot := f.root(2)
		recentDeleted := f.reporter(recentRoot, true, at(-time.Hour), 2)
		recentKept := f.reporter(recentRoot, true, at(time.Hour), 1)
		boundary := f.reporter(f.root(2), true, at(0), 2)
		nullUpdated := f.reporter(f.root(2), true, nil, 2)
		active := f.reporter(f.root(2), false, at(-time.Hour), 2)
		// Pre-existing orphans are out of scope.
		orphanRoot, missingRoot := f.root(1), uuid.New()
		f.common(missingRoot, 0, 1)
		before := f.total()

		preview, err := cleanup(context.Background(), db, options{cutoff: cutoff, rowsPerTransaction: 1000}, false, nil)
		require.NoError(t, err)
		require.Equal(t, before, f.total(), "preview must not mutate")
		require.Equal(t, counts{reporterHistory: 8, commonHistory: 3, parents: 3, roots: 1}, preview.eligible)
		require.EqualValues(t, 3, preview.affected)

		run, err := runCleanup(t, db, 1000, 0, nil)
		require.NoError(t, err)
		require.True(t, run.complete)
		require.Equal(t, preview.eligible, run.deleted)
		require.Equal(t, before-preview.eligible.total(), f.total())
		for _, id := range []uuid.UUID{deleted, sharedDeleted, recentDeleted} {
			require.False(t, f.parentExists(id))
			require.Zero(t, f.reporterRows(id))
		}
		for _, id := range []uuid.UUID{sharedActive, recentKept, boundary, nullUpdated, active} {
			require.True(t, f.parentExists(id))
		}
		require.False(t, f.rootExists(deletedRoot))
		require.EqualValues(t, 5, f.commonRows(sharedRoot))
		require.EqualValues(t, 2, f.commonRows(recentRoot))
		require.True(t, f.rootExists(orphanRoot))
		require.EqualValues(t, 1, f.commonRows(orphanRoot))
		require.EqualValues(t, 1, f.commonRows(missingRoot))
	})

	t.Run("deep history spans many transactions within the row budget", func(t *testing.T) {
		f := fixture{t, db}
		f.reset()
		const rows = 10 // two parents per batch
		roots := make([]uuid.UUID, 3)
		parents := make([]uuid.UUID, 3)
		for i := range parents {
			roots[i] = f.root(23)
			parents[i] = f.reporter(roots[i], true, at(-time.Duration(3-i)*time.Hour), 37)
		}
		activeRoot := f.root(4)
		active := f.reporter(activeRoot, false, at(-time.Hour), 3)

		previous, previousDeleted := f.total(), int64(0)
		verify := func(_ string, r result) {
			now := f.total()
			require.LessOrEqual(t, previous-now, int64(rows), "one committed transaction exceeded the row budget")
			require.Equal(t, previous-now, r.deleted.total()-previousDeleted, "reported deletions differ from table counts")
			previous, previousDeleted = now, r.deleted.total()
		}

		first, err := runCleanup(t, db, rows, 1, verify)
		require.NoError(t, err)
		require.False(t, first.complete, "a capped run with eligible parents left is incomplete")
		require.Equal(t, 1, first.batches)
		require.Greater(t, first.transactions, 10)
		require.False(t, f.parentExists(parents[0]))
		require.False(t, f.parentExists(parents[1]))
		require.False(t, f.rootExists(roots[0]))
		require.True(t, f.parentExists(parents[2]), "younger parent is outside the first batch")
		require.EqualValues(t, 37, f.reporterRows(parents[2]))
		require.EqualValues(t, 23, f.commonRows(roots[2]))

		previous, previousDeleted = f.total(), 0
		second, err := runCleanup(t, db, rows, 0, verify)
		require.NoError(t, err)
		require.True(t, second.complete)
		require.False(t, f.rootExists(roots[2]))
		require.True(t, f.parentExists(active))
		require.EqualValues(t, 3, f.reporterRows(active))
		require.EqualValues(t, 4, f.commonRows(activeRoot))

		again, err := runCleanup(t, db, rows, 0, nil)
		require.NoError(t, err)
		require.Zero(t, again.candidates)
		require.True(t, again.complete)
	})

	t.Run("shallow parents share one batch", func(t *testing.T) {
		f := fixture{t, db}
		f.reset()
		for i := 0; i < 4; i++ {
			f.reporter(f.root(1), true, at(-time.Hour), 1)
		}
		run, err := runCleanup(t, db, 20, 0, nil)
		require.NoError(t, err)
		require.Equal(t, 1, run.batches)
		require.Equal(t, counts{reporterHistory: 4, commonHistory: 4, parents: 4, roots: 4}, run.deleted)
		require.Zero(t, f.total())
	})

	t.Run("oldest first with parent id breaking ties", func(t *testing.T) {
		f := fixture{t, db}
		f.reset()
		oldest := f.reporter(f.root(1), true, at(-3*time.Hour), 1)
		tied := []uuid.UUID{f.reporter(f.root(1), true, at(-2*time.Hour), 1), f.reporter(f.root(1), true, at(-2*time.Hour), 1)}
		sort.Slice(tied, func(i, j int) bool { return bytes.Compare(tied[i][:], tied[j][:]) < 0 })
		for _, expected := range []uuid.UUID{oldest, tied[0], tied[1]} {
			_, err := runCleanup(t, db, 5, 1, nil) // one parent per run
			require.NoError(t, err)
			require.False(t, f.parentExists(expected))
		}
	})

	t.Run("cancellation keeps committed chunks and a rerun resumes", func(t *testing.T) {
		f := fixture{t, db}
		f.reset()
		root := f.root(23)
		parent := f.reporter(root, true, at(-time.Hour), 19)
		ctx, cancel := context.WithCancel(context.Background())
		partial, err := cleanup(ctx, db, options{cutoff: cutoff, rowsPerTransaction: 5, batchDelay: time.Minute}, true, func(message string, _ result) {
			if message == "tombstone cleanup history chunk committed" {
				cancel()
			}
		})
		require.ErrorIs(t, err, context.Canceled)
		require.EqualValues(t, 5, partial.deleted.total())
		require.True(t, f.parentExists(parent))
		require.EqualValues(t, 14, f.reporterRows(parent))
		require.EqualValues(t, 1, f.count("SELECT COUNT(*) FROM reporter_representations WHERE reporter_resource_id=? AND version=18", parent), "current snapshot is kept")

		resumed, err := runCleanup(t, db, 5, 0, nil)
		require.NoError(t, err)
		require.True(t, resumed.complete)
		require.False(t, f.parentExists(parent))
		require.False(t, f.rootExists(root))
		require.Zero(t, f.commonRows(root))
	})

	t.Run("parent revived during the batch is skipped with its current snapshot", func(t *testing.T) {
		f := fixture{t, db}
		f.reset()
		root := f.root(2)
		parent := f.reporter(root, true, at(-time.Hour), 19)
		revived := false
		run, err := runCleanup(t, db, 5, 0, func(message string, _ result) {
			if message == "tombstone cleanup history chunk committed" && !revived {
				revived = true
				require.NoError(t, db.Exec("UPDATE reporter_resources SET tombstone=FALSE, updated_at=? WHERE id=?", cutoff.Add(time.Hour), parent).Error)
			}
		})
		require.NoError(t, err)
		require.True(t, run.complete)
		require.EqualValues(t, 1, run.skipped)
		require.Zero(t, run.deleted.parents)
		require.True(t, f.parentExists(parent))
		// History committed before the revival stays deleted; the rest is kept.
		require.EqualValues(t, 14, f.reporterRows(parent))
		require.True(t, f.rootExists(root))
		require.EqualValues(t, 2, f.commonRows(root))
	})

	t.Run("new reporter on the root during the batch preserves the root", func(t *testing.T) {
		f := fixture{t, db}
		f.reset()
		root := f.root(39)
		parent := f.reporter(root, true, at(-time.Hour), 1)
		added := false
		run, err := runCleanup(t, db, 10, 0, func(message string, _ result) {
			if message == "tombstone cleanup history chunk committed" && !added {
				added = true
				f.reporter(root, false, at(time.Hour), 1)
				f.common(root, 999, 1)
			}
		})
		require.NoError(t, err)
		require.True(t, run.complete)
		require.EqualValues(t, 10, run.deleted.commonHistory, "common trimming stops once the root has a surviving reporter")
		require.False(t, f.parentExists(parent))
		require.True(t, f.rootExists(root))
		require.EqualValues(t, 30, f.commonRows(root))
		require.EqualValues(t, 3, f.count("SELECT COUNT(*) FROM common_representations WHERE resource_id=? AND version IN (0,38,999)", root))
	})

	t.Run("history added during the batch cannot exceed the row budget", func(t *testing.T) {
		f := fixture{t, db}
		f.reset()
		root := f.root(1)
		parent := f.reporter(root, true, at(-time.Hour), 1)
		added := false
		_, err := runCleanup(t, db, 5, 0, func(message string, _ result) {
			if message == "tombstone cleanup history chunk committed" && !added {
				added = true
				require.NoError(t, db.Exec("INSERT INTO reporter_representations(reporter_resource_id,version,generation,data,tombstone) SELECT ?,n,0,'{}',FALSE FROM generate_series(100,109) n", parent).Error)
			}
		})
		require.ErrorContains(t, err, "above rows-per-transaction 5")
		require.True(t, f.parentExists(parent))
		require.EqualValues(t, 11, f.reporterRows(parent))
		require.True(t, f.rootExists(root))
	})

	t.Run("failed finalization rolls back but keeps committed trims", func(t *testing.T) {
		f := fixture{t, db}
		f.reset()
		root := f.root(17)
		parent := f.reporter(root, true, at(-time.Hour), 13)
		require.NoError(t, db.Exec(`CREATE FUNCTION fail_root() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'injected root failure'; END; $$ LANGUAGE plpgsql`).Error)
		require.NoError(t, db.Exec(`CREATE TRIGGER fail_root BEFORE DELETE ON resource FOR EACH ROW EXECUTE FUNCTION fail_root()`).Error)
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS fail_root ON resource").Error
			_ = db.Exec("DROP FUNCTION IF EXISTS fail_root()").Error
		})
		run, err := runCleanup(t, db, 5, 0, nil)
		require.ErrorContains(t, err, "injected root failure")
		require.Zero(t, run.batches)
		require.EqualValues(t, 12+15, run.deleted.total(), "all history except the protected snapshots was trimmed")
		require.True(t, f.parentExists(parent))
		require.True(t, f.rootExists(root))
		require.EqualValues(t, 1, f.reporterRows(parent))
		require.EqualValues(t, 2, f.commonRows(root))
	})

	t.Run("revival committed during finalization is preserved after serialization retry", func(t *testing.T) {
		f := fixture{t, db}
		f.reset()
		root := f.root(2)
		parent := f.reporter(root, true, at(-time.Hour), 3)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		guard, err := sqlDB.Conn(ctx)
		require.NoError(t, err)
		defer guard.Close() //nolint:errcheck
		_, err = guard.ExecContext(ctx, "SELECT pg_advisory_lock(5217601)")
		require.NoError(t, err)
		// The statement-level trigger lets the revival commit after finalization has
		// read the parent but before it locks the row.
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
			r, err := cleanup(ctx, db, options{cutoff: cutoff, rowsPerTransaction: 1000}, true, nil)
			done <- outcome{r, err}
		}()
		require.Eventually(t, func() bool {
			return f.count("SELECT COUNT(*) FROM pg_locks WHERE locktype='advisory' AND NOT granted") > 0
		}, 5*time.Second, 10*time.Millisecond)
		require.NoError(t, db.Exec("UPDATE reporter_resources SET tombstone=FALSE, generation=1, representation_version=0, updated_at=? WHERE id=?", cutoff.Add(time.Hour), parent).Error)
		_, err = guard.ExecContext(ctx, "SELECT pg_advisory_unlock(5217601)")
		require.NoError(t, err)
		select {
		case finished := <-done:
			require.NoError(t, finished.err)
			require.EqualValues(t, 1, finished.result.skipped)
			require.Zero(t, finished.result.deleted.parents)
			require.True(t, finished.result.complete)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		require.EqualValues(t, 1, f.count("SELECT COUNT(*) FROM reporter_resources WHERE id=? AND NOT tombstone", parent))
		require.EqualValues(t, 1, f.reporterRows(parent), "trimmed history stays deleted; the snapshot is kept")
		require.True(t, f.rootExists(root))
		require.EqualValues(t, 2, f.commonRows(root))
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
