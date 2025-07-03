package usecase

import (
	"gorm.io/gorm"
)

// TransactionManager handles database transactions with retry logic for serialization failures
type TransactionManager interface {
	// ExecuteInSerializableTransaction executes the given function within a serializable transaction
	// with automatic retry logic for serialization failures
	ExecuteInSerializableTransaction(db *gorm.DB, txFunc func(tx *gorm.DB) error) error
}
