package memory

import (
	"errors"
	"testing"

	"github.com/project-kessel/inventory-api/internal/infrastructure/resourcerepository"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	assert.NoError(t, err)

	// Enable foreign keys for SQLite
	db.Exec("PRAGMA foreign_keys = ON")

	return db
}

// =============================================================================
// Interface Compliance Tests
// =============================================================================

func TestFakeTransactionManager_Interface(t *testing.T) {
	tm := NewFakeTransactionManager(3)

	// Verify it implements the interface
	var _ resourcerepository.TransactionManager = tm
}

// =============================================================================
// Fake Transaction Manager Tests
// =============================================================================

func TestNewFakeTransactionManager(t *testing.T) {
	maxRetries := 5
	tm := NewFakeTransactionManager(maxRetries)

	assert.NotNil(t, tm)
	assert.Equal(t, maxRetries, tm.maxSerializationRetries)
	assert.Equal(t, 0, tm.GetTransactionCallCount())
	assert.False(t, tm.shouldFailTransaction)
	assert.False(t, tm.shouldFailCommit)
}

func TestFakeTransactionManager_Success(t *testing.T) {
	db := setupTestDB(t)
	tm := NewFakeTransactionManager(3)

	var capturedTx *gorm.DB
	executed := false

	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		capturedTx = tx
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
	assert.Equal(t, db, capturedTx) // Fake passes the same DB instance
	assert.Equal(t, 1, tm.GetTransactionCallCount())
}

func TestFakeTransactionManager_TransactionFailure(t *testing.T) {
	db := setupTestDB(t)
	tm := NewFakeTransactionManager(3)
	tm.SetShouldFailTransaction(true)

	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		assert.Fail(t, "Transaction function should not be called when set to fail")
		return nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated transaction failure")
	assert.Equal(t, 1, tm.GetTransactionCallCount())
}

func TestFakeTransactionManager_CommitFailure(t *testing.T) {
	db := setupTestDB(t)
	tm := NewFakeTransactionManager(3)
	tm.SetShouldFailCommit(true)

	executed := false
	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		executed = true
		return nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated commit failure")
	assert.True(t, executed) // Function should be executed, but commit fails
	assert.Equal(t, 1, tm.GetTransactionCallCount())
}

func TestFakeTransactionManager_FunctionError(t *testing.T) {
	db := setupTestDB(t)
	tm := NewFakeTransactionManager(3)

	expectedError := errors.New("business logic error")
	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		return expectedError
	})

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	assert.Equal(t, 1, tm.GetTransactionCallCount())
}

func TestFakeTransactionManager_SetShouldFailTransaction(t *testing.T) {
	db := setupTestDB(t)
	tm := NewFakeTransactionManager(3)

	// Initially should succeed
	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		return nil
	})
	assert.NoError(t, err)

	// Set to fail
	tm.SetShouldFailTransaction(true)
	err = tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		assert.Fail(t, "Transaction function should not be called when set to fail")
		return nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated transaction failure")
	assert.Equal(t, 2, tm.GetTransactionCallCount())
}

func TestFakeTransactionManager_SetShouldFailCommit(t *testing.T) {
	db := setupTestDB(t)
	tm := NewFakeTransactionManager(3)

	tm.SetShouldFailCommit(true)

	executed := false
	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		executed = true
		return nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated commit failure")
	assert.True(t, executed) // Function should be executed, but commit fails
	assert.Equal(t, 1, tm.GetTransactionCallCount())
}

func TestFakeTransactionManager_GetTransactionCallCount(t *testing.T) {
	db := setupTestDB(t)
	tm := NewFakeTransactionManager(3)

	assert.Equal(t, 0, tm.GetTransactionCallCount())

	// Execute multiple transactions
	_ = tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error { return nil })
	assert.Equal(t, 1, tm.GetTransactionCallCount())

	_ = tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error { return nil })
	assert.Equal(t, 2, tm.GetTransactionCallCount())

	// Even failed transactions should increment count
	tm.SetShouldFailTransaction(true)
	_ = tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error { return nil })
	assert.Equal(t, 3, tm.GetTransactionCallCount())
}

func TestFakeTransactionManager_Reset(t *testing.T) {
	db := setupTestDB(t)
	tm := NewFakeTransactionManager(3)

	// Set up some state
	tm.SetShouldFailTransaction(true)
	tm.SetShouldFailCommit(true)
	_ = tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error { return nil })

	assert.Equal(t, 1, tm.GetTransactionCallCount())

	// Reset
	tm.Reset()

	// Verify reset state
	assert.Equal(t, 0, tm.GetTransactionCallCount())
	assert.False(t, tm.shouldFailTransaction)
	assert.False(t, tm.shouldFailCommit)

	// Should work normally after reset
	executed := false
	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
	assert.Equal(t, 1, tm.GetTransactionCallCount())
}

func TestFakeTransactionManager_ConcurrentSafety(t *testing.T) {
	db := setupTestDB(t)
	tm := NewFakeTransactionManager(3)

	// This is a basic test for concurrent safety
	// In practice, you'd want more sophisticated concurrent testing
	done := make(chan bool, 2)

	go func() {
		tm.SetShouldFailTransaction(true)
		_ = tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error { return nil })
		done <- true
	}()

	go func() {
		tm.SetShouldFailCommit(true)
		_ = tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error { return nil })
		done <- true
	}()

	<-done
	<-done

	// Should have been called twice
	assert.Equal(t, 2, tm.GetTransactionCallCount())
}
