package data

import "gorm.io/gorm"

// TransactionManager provides an abstraction for handling database transactions
// with proper isolation levels and retry mechanisms.
type TransactionManager interface {
	// HandleSerializableTransaction executes the provided function within a serializable transaction.
	// It automatically handles retries in case of serialization failures.
	HandleSerializableTransaction(operationName string, db *gorm.DB, txFunc func(tx *gorm.DB) error) error
}
