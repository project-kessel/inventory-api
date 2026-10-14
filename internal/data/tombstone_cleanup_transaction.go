package data

import (
	"context"
	"fmt"
	"time"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"gorm.io/gorm"
)

type gormTombstoneCleanupTransaction struct {
	db                 *gorm.DB
	transactionManager bizmodel.TransactionManager
}

// NewTombstoneCleanupTransaction adapts the shared transaction manager to cleanup repository work.
func NewTombstoneCleanupTransaction(db *gorm.DB, transactionManager bizmodel.TransactionManager) bizmodel.TombstoneCleanupTransaction {
	return &gormTombstoneCleanupTransaction{db: db, transactionManager: transactionManager}
}

// WithinSerializableTombstoneCleanup runs bounded repository work with transaction-local timeouts.
func (transaction *gormTombstoneCleanupTransaction) WithinSerializableTombstoneCleanup(
	ctx context.Context,
	operationName string,
	readOnly bool,
	statementTimeout time.Duration,
	lockTimeout time.Duration,
	work func(bizmodel.TombstoneCleanupRepository) error,
) error {
	if transaction == nil || transaction.db == nil || transaction.transactionManager == nil {
		return fmt.Errorf("tombstone cleanup transaction adapter is not configured")
	}
	if transaction.db.Name() != "postgres" {
		return fmt.Errorf("tombstone cleanup requires PostgreSQL")
	}
	if statementTimeout <= 0 || lockTimeout <= 0 || lockTimeout > statementTimeout {
		return fmt.Errorf("tombstone cleanup transaction timeouts must be positive and lock timeout cannot exceed statement timeout")
	}
	if work == nil {
		return fmt.Errorf("tombstone cleanup transaction work is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	return transaction.transactionManager.HandleSerializableTransaction(operationName, transaction.db.WithContext(ctx), func(tx *gorm.DB) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if tx.Error != nil {
			return tx.Error
		}
		if readOnly {
			if err := tx.Exec("SET TRANSACTION READ ONLY").Error; err != nil {
				return fmt.Errorf("set cleanup transaction read-only: %w", err)
			}
		}
		if err := setLocalTombstoneCleanupTimeout(tx, "statement_timeout", statementTimeout); err != nil {
			return fmt.Errorf("set tombstone cleanup statement timeout: %w", err)
		}
		if err := setLocalTombstoneCleanupTimeout(tx, "lock_timeout", lockTimeout); err != nil {
			return fmt.Errorf("set tombstone cleanup lock timeout: %w", err)
		}
		return work(&gormTombstoneCleanupRepository{db: tx.WithContext(ctx)})
	})
}

func setLocalTombstoneCleanupTimeout(tx *gorm.DB, setting string, timeout time.Duration) error {
	if setting != "statement_timeout" && setting != "lock_timeout" {
		return fmt.Errorf("unsupported PostgreSQL timeout setting %q", setting)
	}
	milliseconds := int64(timeout / time.Millisecond)
	if timeout%time.Millisecond != 0 {
		milliseconds++
	}
	if milliseconds < 1 {
		milliseconds = 1
	}
	return tx.Exec(fmt.Sprintf("SET LOCAL %s = '%dms'", setting, milliseconds)).Error
}
