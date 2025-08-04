package usecase

import "gorm.io/gorm"

// TransactionManager defines the interface for handling serializable transactions
// with retry logic for serialization failures.
type TransactionManager interface {
	// HandleSerializableTransaction handles serializable transaction rollbacks, commits, and retries in case of failures.
	// It retries the transaction up to the configured max retries before returning an error.
	HandleSerializableTransaction(db *gorm.DB, txFunc func(tx *gorm.DB) error) error
}
