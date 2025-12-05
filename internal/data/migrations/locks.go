package migrations

import (
	"context"
	"database/sql"

	"gorm.io/gorm"
)

const (
	MigrationAdvisoryLockKey int64 = 1234567890
)

// function variable to allow tests to stub advisory locking behavior without
// requiring a real database connection.
var withAdvisoryLock = WithAdvisoryLock

func isPostgres(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	return db.Name() == "postgres"
}

// WithAdvisoryLock executes fn while holding a PostgreSQL advisory lock to serialize
// migration operations. For non-Postgres databases, fn executes without locking.
//
// CRITICAL: The lock is acquired on a dedicated connection, and GORM operations are bound
// to that specific connection via connWrapper. This ensures the advisory lock actually
// protects the migration operations. Without this binding, GORM would use random pooled
// connections that don't hold the lock, allowing concurrent execution.
//
// The lock uses pg_advisory_lock(MigrationAdvisoryLockKey). Advisory locks are
// automatically released when the connection closes.
//
// This design allows migrations to use DDL that cannot run in transactions
// (e.g., CREATE INDEX CONCURRENTLY) while still ensuring only one process runs
// migrations at a time across multiple application instances.
func WithAdvisoryLock(ctx context.Context, db *gorm.DB, fn func(*gorm.DB) error) error {
	if !isPostgres(db) {
		return fn(db)
	}

	// Acquire a session-level advisory lock on a dedicated connection to serialize migrations
	// without wrapping them in a single transaction. This enables DDL that cannot run inside
	// a transaction (e.g., CREATE INDEX CONCURRENTLY) while still ensuring exclusivity.
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", MigrationAdvisoryLockKey); err != nil {
		return err
	}
	defer func() {
		// Best-effort unlock. If this fails, connection close will also release locks.
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", MigrationAdvisoryLockKey)
	}()

	// Create a new GORM DB session bound to the connection that holds the lock.
	// This is CRITICAL: without binding to the locked connection, GORM would use
	// random connections from the pool, defeating the purpose of the advisory lock.
	connDB := db.Session(&gorm.Session{
		PrepareStmt: false,
		NewDB:       true,
	})
	connDB.Statement.ConnPool = &connWrapper{conn: conn}

	return fn(connDB)
}

// connWrapper wraps *sql.Conn to satisfy gorm.ConnPool interface,
type connWrapper struct {
	conn *sql.Conn
}

func (c *connWrapper) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return c.conn.PrepareContext(ctx, query)
}

func (c *connWrapper) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return c.conn.ExecContext(ctx, query, args...)
}

func (c *connWrapper) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return c.conn.QueryContext(ctx, query, args...)
}

func (c *connWrapper) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return c.conn.QueryRowContext(ctx, query, args...)
}
