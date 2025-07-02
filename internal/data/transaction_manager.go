package data

import (
	"database/sql"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
)

// TransactionManager implements the usecase.TransactionManager interface using real database transactions
type TransactionManager struct {
	MaxSerializationRetries int
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(maxRetries int) *TransactionManager {
	return &TransactionManager{
		MaxSerializationRetries: maxRetries,
	}
}

// ExecuteInSerializableTransaction implements the usecase.TransactionManager interface
// This is based on the handleSerializableTransaction method from the resource repository
func (tm *TransactionManager) ExecuteInSerializableTransaction(db *gorm.DB, txFunc func(tx *gorm.DB) error) error {
	var err error
	for i := 0; i < tm.MaxSerializationRetries; i++ {
		tx := db.Begin(&sql.TxOptions{
			Isolation: sql.LevelSerializable,
		})
		err = txFunc(tx)
		if err != nil {
			tx.Rollback()
			if tm.isSerializationFailure(err, i, tm.MaxSerializationRetries) {
				continue
			}
			// Short-circuit if the error is not a serialization failure
			return fmt.Errorf("transaction failed: %w", err)
		}
		err = tx.Commit().Error
		if err != nil {
			tx.Rollback()
			if tm.isSerializationFailure(err, i, tm.MaxSerializationRetries) {
				continue
			}
			// Short-circuit if the error is not a serialization failure
			return fmt.Errorf("committing transaction failed: %w", err)
		}
		return nil
	}
	log.Errorf("transaction failed after %d attempts: %v", tm.MaxSerializationRetries, err)
	return fmt.Errorf("transaction failed after %d attempts: %w", tm.MaxSerializationRetries, err)
}

// isSerializationFailure checks if the error is a serialization failure that should be retried
func (tm *TransactionManager) isSerializationFailure(err error, attempt, maxRetries int) bool {
	switch dbErr := err.(type) {
	case *pgconn.PgError:
		if dbErr.Code == "40001" {
			log.Debugf("transaction serialization failure (attempt %d/%d): %v", attempt+1, maxRetries, err)
			return true
		}
	case sqlite3.Error:
		if dbErr.Code == sqlite3.ErrError {
			// Isolation failures are captured under error code 1 (sqlite3.ErrError)
			log.Debugf("transaction serialization failure (attempt %d/%d): %v", attempt+1, maxRetries, err)
			return true
		}
	}
	return false
}
