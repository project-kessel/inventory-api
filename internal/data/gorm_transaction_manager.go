package data

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/usecase"
)

type gormTransactionManager struct {
	maxSerializationRetries int
}

// NewGormTransactionManager creates a new GORM-based transaction manager
func NewGormTransactionManager(maxSerializationRetries int) usecase.TransactionManager {
	return &gormTransactionManager{
		maxSerializationRetries: maxSerializationRetries,
	}
}

// HandleSerializableTransaction executes the provided function within a serializable transaction
// It automatically handles retries in case of serialization failures
func (tm *gormTransactionManager) HandleSerializableTransaction(db *gorm.DB, txFunc func(tx *gorm.DB) error) error {
	var err error
	for i := 0; i < tm.maxSerializationRetries; i++ {
		tx := db.Begin(&sql.TxOptions{
			Isolation: sql.LevelSerializable,
		})
		err = txFunc(tx)
		if err != nil {
			tx.Rollback()
			if tm.isSerializationFailure(err, i, tm.maxSerializationRetries) {
				continue
			}
			return fmt.Errorf("transaction failed: %w", err)
		}
		err = tx.Commit().Error
		if err != nil {
			tx.Rollback()
			if tm.isSerializationFailure(err, i, tm.maxSerializationRetries) {
				continue
			}
			return fmt.Errorf("committing transaction failed: %w", err)
		}
		return nil
	}
	log.Errorf("transaction failed after %d attempts: %v", tm.maxSerializationRetries, err)
	return fmt.Errorf("transaction failed after %d attempts: %w", tm.maxSerializationRetries, err)
}

func (tm *gormTransactionManager) isSerializationFailure(err error, attempt, maxRetries int) bool {
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
