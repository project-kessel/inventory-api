package data

import (
	"errors"
	"sync"

	"gorm.io/gorm"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

// fakeTransactionManager is a test implementation of TransactionManager
// that simulates transaction behavior without actual database transactions
type fakeTransactionManager struct {
	mu                      sync.Mutex
	shouldFailTransaction   bool
	shouldFailCommit        bool
	transactionCallCount    int
	maxSerializationRetries int
}

// NewFakeTransactionManager creates a new fake transaction manager for testing
func NewFakeTransactionManager(maxSerializationRetries int) *fakeTransactionManager {
	return &fakeTransactionManager{
		maxSerializationRetries: maxSerializationRetries,
	}
}

// SetShouldFailTransaction configures the fake to fail during transaction execution
func (ftm *fakeTransactionManager) SetShouldFailTransaction(shouldFail bool) {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()
	ftm.shouldFailTransaction = shouldFail
}

// SetShouldFailCommit configures the fake to fail during commit
func (ftm *fakeTransactionManager) SetShouldFailCommit(shouldFail bool) {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()
	ftm.shouldFailCommit = shouldFail
}

// GetTransactionCallCount returns the number of times HandleSerializableTransaction was called
func (ftm *fakeTransactionManager) GetTransactionCallCount() int {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()
	return ftm.transactionCallCount
}

// Reset resets the fake transaction manager state
func (ftm *fakeTransactionManager) Reset() {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()
	ftm.shouldFailTransaction = false
	ftm.shouldFailCommit = false
	ftm.transactionCallCount = 0
}

// HandleSerializableTransaction simulates serializable transaction behavior
func (ftm *fakeTransactionManager) HandleSerializableTransaction(operationName string, db *gorm.DB, txFunc func(tx *gorm.DB) error) error {
	ftm.mu.Lock()
	ftm.transactionCallCount++
	shouldFailTx := ftm.shouldFailTransaction
	shouldFailCommit := ftm.shouldFailCommit
	ftm.mu.Unlock()

	if shouldFailTx {
		return errors.New("simulated transaction failure")
	}

	// Execute the transaction function with the same db (no actual transaction in fake)
	err := txFunc(db)
	if err != nil {
		return err
	}

	if shouldFailCommit {
		return errors.New("simulated commit failure")
	}

	return nil
}

// Ensure fakeTransactionManager implements TransactionManager interface
var _ bizmodel.TransactionManager = (*fakeTransactionManager)(nil)
