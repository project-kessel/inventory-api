package data

import (
	"gorm.io/gorm"
)

// FakeTransactionManager is a test implementation of TransactionManager that doesn't use real transactions
type FakeTransactionManager struct {
	// ExecuteFunc allows tests to customize the behavior
	ExecuteFunc func(db *gorm.DB, txFunc func(tx *gorm.DB) error) error
}

// NewFakeTransactionManager creates a new fake transaction manager
func NewFakeTransactionManager() *FakeTransactionManager {
	return &FakeTransactionManager{
		// Default behavior: just execute the function with the provided db (no transaction)
		ExecuteFunc: func(db *gorm.DB, txFunc func(tx *gorm.DB) error) error {
			return txFunc(db)
		},
	}
}

// ExecuteInSerializableTransaction implements the TransactionManager interface for testing
func (f *FakeTransactionManager) ExecuteInSerializableTransaction(db *gorm.DB, txFunc func(tx *gorm.DB) error) error {
	return f.ExecuteFunc(db, txFunc)
}

// WithCustomBehavior allows tests to customize the transaction behavior
func (f *FakeTransactionManager) WithCustomBehavior(fn func(db *gorm.DB, txFunc func(tx *gorm.DB) error) error) *FakeTransactionManager {
	f.ExecuteFunc = fn
	return f
}
