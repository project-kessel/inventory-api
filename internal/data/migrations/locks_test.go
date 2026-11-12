package migrations

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/testutil"
)

func openSQLiteForLock(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func TestWithAdvisoryLock_SqliteRunsFn(t *testing.T) {
	db := openSQLiteForLock(t)
	ctx := context.Background()
	called := false

	err := WithAdvisoryLock(ctx, db, func(tx *gorm.DB) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected function to be called")
	}
}

func TestWithAdvisoryLock_SqlitePropagatesError(t *testing.T) {
	db := openSQLiteForLock(t)
	ctx := context.Background()
	myErr := errors.New("boom")

	err := WithAdvisoryLock(ctx, db, func(tx *gorm.DB) error {
		return myErr
	})
	if !errors.Is(err, myErr) {
		t.Fatalf("expected %v, got %v", myErr, err)
	}
}

// TestWithAdvisoryLock_PostgresHoldsLock verifies session binding, exclusion,
// and cleanup on success, callback error, and cancellation. The SQLite fallback
// (no locking) is covered by the tests above.
func TestWithAdvisoryLock_PostgresHoldsLock(t *testing.T) {
	if !testutil.UsingPostgres() {
		t.Skip("advisory locking is PostgreSQL-specific; the SQLite fallback is covered by the tests above")
	}
	callbackError := errors.New("migration callback failed")
	for _, scenario := range []struct {
		name          string
		callbackError error
		cancel        bool
	}{
		{name: "success"},
		{name: "callback_error", callbackError: callbackError},
		{name: "cancellation", cancel: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := testutil.NewTestDB(t, &gorm.Config{}, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			lockCtx, cancelLock := context.WithCancel(ctx)
			defer cancelLock()
			originalPool := db.Statement.ConnPool
			sqlDB, err := db.DB()
			require.NoError(t, err)
			// Reserve the probe before acquiring the lock so it remains a distinct
			// session even after the locked connection returns to the pool.
			probe, err := sqlDB.Conn(ctx)
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, probe.Close(), "close advisory-lock probe connection")
			}()
			var probePID int
			require.NoError(t, probe.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&probePID))
			defer func() {
				// A failed assertion must not leave a probe-owned session lock in the pool.
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cleanupCancel()
				_, err := probe.ExecContext(cleanupCtx, "SELECT pg_advisory_unlock_all()")
				assert.NoError(t, err)
			}()

			type lockSession struct {
				PID      int
				OwnsLock bool
			}
			inLock := make(chan lockSession, 1)
			probeDone := make(chan struct{})
			finishProbe := sync.OnceFunc(func() { close(probeDone) })
			released := make(chan error, 1)
			workerDone := make(chan struct{})
			go func() {
				defer close(workerDone)
				released <- WithAdvisoryLock(lockCtx, db, func(tx *gorm.DB) error {
					var session lockSession
					// The bigint lock key is represented by classid/objid; objsubid=1
					// distinguishes it from the two-integer advisory-lock namespace.
					if err := tx.Raw(`SELECT pg_backend_pid() AS pid, EXISTS (
						SELECT 1 FROM pg_locks WHERE locktype = 'advisory'
						AND pid = pg_backend_pid() AND granted
						AND classid = 0 AND objid = ? AND objsubid = 1
					) AS owns_lock`, MigrationAdvisoryLockKey).Scan(&session).Error; err != nil {
						return err
					}
					inLock <- session
					// Do not release the lock until the independent probe has run.
					select {
					case <-probeDone:
						return scenario.callbackError
					case <-lockCtx.Done():
						return lockCtx.Err()
					}
				})
			}()
			defer func() {
				// Fatal probe failures must release and join the worker before DB cleanup.
				cancelLock()
				finishProbe()
				select {
				case <-workerDone:
				case <-time.After(10 * time.Second):
					t.Error("timed out waiting for advisory-lock worker cleanup")
				}
			}()
			select {
			case session := <-inLock:
				assert.NotEqual(t, probePID, session.PID)
				assert.True(t, session.OwnsLock, "callback SQL must run on the lock-owning session")
			case err := <-released:
				t.Fatalf("WithAdvisoryLock returned before callback SQL completed: %v", err)
			case <-ctx.Done():
				t.Fatalf("waiting for advisory-lock callback: %v", ctx.Err())
			}

			var acquired bool
			require.NoError(t, probe.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", MigrationAdvisoryLockKey).Scan(&acquired))
			require.False(t, acquired, "another session must not acquire the key during the callback")
			if scenario.cancel {
				cancelLock()
			} else {
				finishProbe()
			}
			select {
			case err := <-released:
				if scenario.cancel {
					require.ErrorIs(t, err, context.Canceled)
				} else if scenario.callbackError != nil {
					require.ErrorIs(t, err, scenario.callbackError)
				} else {
					require.NoError(t, err)
				}
			case <-ctx.Done():
				t.Fatalf("waiting for advisory-lock release: %v", ctx.Err())
			}

			require.NoError(t, probe.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", MigrationAdvisoryLockKey).Scan(&acquired))
			require.True(t, acquired, "the key must be released even on callback error or cancellation")
			var unlocked bool
			require.NoError(t, probe.QueryRowContext(ctx, "SELECT pg_advisory_unlock($1)", MigrationAdvisoryLockKey).Scan(&unlocked))
			assert.True(t, unlocked, "the independent session must be able to release its lock")

			assert.Same(t, originalPool, db.Statement.ConnPool, "the original GORM handle must not be rebound")
			var value int
			require.NoError(t, db.WithContext(ctx).Raw("SELECT 42").Scan(&value).Error)
			assert.Equal(t, 42, value, "the original handle must remain usable after cleanup")
		})
	}
}
