package data

import (
	"database/sql"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
)

// GormTransactionManager provides GORM-based transaction management with retry logic
// for serialization failures in PostgreSQL and SQLite databases.
type GormTransactionManager struct {
	maxSerializationRetries int
}

// NewGormTransactionManager creates a new GORM-based transaction manager.
func NewGormTransactionManager(maxSerializationRetries int) *GormTransactionManager {
	return &GormTransactionManager{
		maxSerializationRetries: maxSerializationRetries,
	}
}

// HandleSerializableTransaction handles serializable transaction rollbacks, commits, and retries in case of failures.
// It retries the transaction up to maxRetries times before returning an error.
func (tm *GormTransactionManager) HandleSerializableTransaction(db *gorm.DB, txFunc func(tx *gorm.DB) error) error {
	var err error
	for i := 0; i < tm.maxSerializationRetries; i++ {
		tx := db.Begin(&sql.TxOptions{
			Isolation: sql.LevelSerializable,
		})
		err = txFunc(tx)
		if err != nil {
			tx.Rollback()
			if tm.isSerializationFailure(err, i) {
				continue
			}
			// Short-circuit if the error is not a serialization failure
			return fmt.Errorf("transaction failed: %w", err)
		}
		err = tx.Commit().Error
		if err != nil {
			tx.Rollback()
			if tm.isSerializationFailure(err, i) {
				continue
			}
			// Short-circuit if the error is not a serialization failure
			return fmt.Errorf("committing transaction failed: %w", err)
		}
		return nil
	}
	log.Errorf("transaction failed after %d attempts: %v", tm.maxSerializationRetries, err)
	return fmt.Errorf("transaction failed after %d attempts: %w", tm.maxSerializationRetries, err)
}

func (tm *GormTransactionManager) isSerializationFailure(err error, attempt int) bool {
	switch dbErr := err.(type) {
	case *pgconn.PgError:
		if dbErr.Code == "40001" {
			log.Debugf("transaction serialization failure (attempt %d/%d): %v", attempt+1, tm.maxSerializationRetries, err)
			return true
		}
	case sqlite3.Error:
		if dbErr.Code == sqlite3.ErrError {
			// Isolation failures are captured under error code 1 (sqlite3.ErrError)
			log.Debugf("transaction serialization failure (attempt %d/%d): %v", attempt+1, tm.maxSerializationRetries, err)
			return true
		}
	}
	return false
}