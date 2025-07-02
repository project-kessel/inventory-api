package data

import (
	"gorm.io/gorm"
)

// FakeTransactionManager is a test implementation of TransactionManager that doesn't use real transactions
type FakeTransactionManager struct {
	// ExecuteFunc allows tests to customize the behavior
	ExecuteFunc func(db *gorm.DB, txFunc func(tx *gorm.DB) error) error

	// nextError can be set to force the next transaction to fail
	nextError error

	// callCount tracks how many times ExecuteInSerializableTransaction was called
	callCount int
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
	f.callCount++

	// If there's a configured error, return it and clear it
	if f.nextError != nil {
		err := f.nextError
		f.nextError = nil
		return err
	}

	return f.ExecuteFunc(db, txFunc)
}

// WithCustomBehavior allows tests to customize the transaction behavior
func (f *FakeTransactionManager) WithCustomBehavior(fn func(db *gorm.DB, txFunc func(tx *gorm.DB) error) error) *FakeTransactionManager {
	f.ExecuteFunc = fn
	return f
}

// SetNextError configures the fake to return a specific error on the next call
func (f *FakeTransactionManager) SetNextError(err error) {
	f.nextError = err
}

// GetCallCount returns the number of times ExecuteInSerializableTransaction was called
func (f *FakeTransactionManager) GetCallCount() int {
	return f.callCount
}
