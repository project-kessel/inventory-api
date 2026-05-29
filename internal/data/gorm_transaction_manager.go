package data

import (
	"errors"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
)

// isSerializationFailure detects PostgreSQL serialization failures (SQLSTATE 40001)
// and SQLite busy/locked errors that warrant a transaction retry.
func isSerializationFailure(err error, attempt, maxRetries int) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "40001" {
			log.Errorf("transaction serialization failure (attempt %d/%d): %v", attempt+1, maxRetries, err)
			return true
		}
	}

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.Code == sqlite3.ErrError {
			log.Errorf("transaction serialization failure (attempt %d/%d): %v", attempt+1, maxRetries, err)
			return true
		}
	}
	return false
}
