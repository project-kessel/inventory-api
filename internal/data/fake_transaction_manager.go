package data

import (
	"fmt"

	"gorm.io/gorm"
)

// FakeTransactionManager is a test implementation of TransactionManager
// that executes operations without actual transaction management.
// Useful for unit testing and in-memory testing scenarios.
type FakeTransactionManager struct {
	// CallCount tracks how many times HandleSerializableTransaction was called
	CallCount int
	// ShouldFail can be set to make the transaction manager simulate failures
	ShouldFail bool
	// FailureError is the error to return when ShouldFail is true
	FailureError error
	// RetryCount can be set to simulate retry behavior
	RetryCount int
	// RetryAttempts tracks the number of retry attempts made
	RetryAttempts int
}

// NewFakeTransactionManager creates a new fake transaction manager for testing.
func NewFakeTransactionManager() *FakeTransactionManager {
	return &FakeTransactionManager{
		CallCount:     0,
		ShouldFail:    false,
		FailureError:  nil,
		RetryCount:    0,
		RetryAttempts: 0,
	}
}

// HandleSerializableTransaction simulates transaction handling without actual database transactions.
// For testing purposes, it simply calls the transaction function with the provided DB.
func (f *FakeTransactionManager) HandleSerializableTransaction(db *gorm.DB, txFunc func(tx *gorm.DB) error) error {
	f.CallCount++
	
	// Simulate retry behavior if configured
	for attempt := 0; attempt <= f.RetryCount; attempt++ {
		f.RetryAttempts++
		
		// Check if we should fail on this attempt
		if f.ShouldFail && attempt == f.RetryCount {
			if f.FailureError != nil {
				return f.FailureError
			}
			return fmt.Errorf("fake transaction manager simulated failure")
		}
		
		// If not the final attempt and we're simulating retries, continue
		if f.RetryCount > 0 && attempt < f.RetryCount {
			continue
		}
		
		// Execute the transaction function
		return txFunc(db)
	}
	
	return nil
}

// Reset resets the fake transaction manager's state for reuse in tests.
func (f *FakeTransactionManager) Reset() {
	f.CallCount = 0
	f.ShouldFail = false
	f.FailureError = nil
	f.RetryCount = 0
	f.RetryAttempts = 0
}

// SimulateFailure configures the fake to simulate a transaction failure.
func (f *FakeTransactionManager) SimulateFailure(err error) {
	f.ShouldFail = true
	f.FailureError = err
}

// SimulateRetries configures the fake to simulate retry behavior.
func (f *FakeTransactionManager) SimulateRetries(retryCount int) {
	f.RetryCount = retryCount
}