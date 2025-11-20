package migrations

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"errors"

	"gorm.io/gorm"
)

func withTx(db *gorm.DB, fn func(*gorm.DB) error) error {
	if db == nil {
		return errors.New("nil db")
	}
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func isPostgres(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	dialector := db.Dialector.Name()
	return dialector == "postgres"
}

func hashBigKey(a, b string) int64 {
	h := sha1.Sum([]byte(a + "|" + b))
	u := binary.BigEndian.Uint64(h[0:8])
	return int64(u)
}

func WithAdvisoryLock(ctx context.Context, db *gorm.DB, lockID string, lockType string, fn func(*gorm.DB) error) error {
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

	bigKey := hashBigKey(lockID, lockType)
	if _, err := conn.ExecContext(ctx, "select pg_advisory_lock($1)", bigKey); err != nil {
		return err
	}
	defer func() {
		// Best-effort unlock. If this fails, connection close will also release locks.
		_, _ = conn.ExecContext(context.Background(), "select pg_advisory_unlock($1)", bigKey)
	}()

	return fn(db)
}
