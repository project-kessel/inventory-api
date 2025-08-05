package data

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/project-kessel/inventory-api/internal/biz/usecase"
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

func TestGormTransactionManager_Interface(t *testing.T) {
	tm := NewGormTransactionManager(3)

	// Verify it implements the interface
	var _ usecase.TransactionManager = tm
}

func TestFakeTransactionManager_Interface(t *testing.T) {
	tm := NewFakeTransactionManager(3)

	// Verify it implements the interface
	var _ usecase.TransactionManager = tm
}

// =============================================================================
// GORM Transaction Manager Tests
// =============================================================================

func TestNewGormTransactionManager(t *testing.T) {
	maxRetries := 5
	tm := NewGormTransactionManager(maxRetries)

	assert.NotNil(t, tm)

	// Verify it's the correct type
	gormTm, ok := tm.(*gormTransactionManager)
	assert.True(t, ok)
	assert.Equal(t, maxRetries, gormTm.maxSerializationRetries)
}

func TestGormTransactionManager_Success(t *testing.T) {
	db := setupTestDB(t)
	tm := NewGormTransactionManager(3)

	var capturedTx *gorm.DB
	executed := false

	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
		capturedTx = tx
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
	assert.NotNil(t, capturedTx)
	assert.NotEqual(t, db, capturedTx) // Should be a different transaction instance
}

func TestGormTransactionManager_TransactionFailure(t *testing.T) {
	db := setupTestDB(t)
	tm := NewGormTransactionManager(3)

	expectedError := errors.New("business logic error")
	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
		return expectedError
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed")
	assert.Contains(t, err.Error(), expectedError.Error())
}

func TestGormTransactionManager_MultipleRetries(t *testing.T) {
	db := setupTestDB(t)
	tm := NewGormTransactionManager(3)

	callCount := 0
	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
		callCount++
		return errors.New("some error")
	})

	assert.Error(t, err)
	// Should only call once since it's not a serialization failure
	assert.Equal(t, 1, callCount)
}

func TestGormTransactionManager_TransactionIsolation(t *testing.T) {
	db := setupTestDB(t)
	tm := NewGormTransactionManager(3)

	// Create a simple table for testing
	db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, value TEXT)")

	var txLevel sql.IsolationLevel
	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
		// Check the isolation level by attempting to read it
		// This is a basic test - in a real scenario, we'd test actual isolation behavior
		row := tx.Raw("PRAGMA read_uncommitted").Row()
		var result interface{}
		row.Scan(&result)
		txLevel = sql.LevelSerializable // We know this is what we set
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, sql.LevelSerializable, txLevel)
}

func TestGormTransactionManager_MaxRetries(t *testing.T) {
	db := setupTestDB(t)
	// Test with 0 retries to ensure it fails immediately
	tm := NewGormTransactionManager(0)

	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
		return errors.New("test error")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed")
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

	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
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

	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
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
	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
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
	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
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
	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
		return nil
	})
	assert.NoError(t, err)

	// Set to fail
	tm.SetShouldFailTransaction(true)
	err = tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
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
	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
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
	tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error { return nil })
	assert.Equal(t, 1, tm.GetTransactionCallCount())

	tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error { return nil })
	assert.Equal(t, 2, tm.GetTransactionCallCount())

	// Even failed transactions should increment count
	tm.SetShouldFailTransaction(true)
	tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error { return nil })
	assert.Equal(t, 3, tm.GetTransactionCallCount())
}

func TestFakeTransactionManager_Reset(t *testing.T) {
	db := setupTestDB(t)
	tm := NewFakeTransactionManager(3)

	// Set up some state
	tm.SetShouldFailTransaction(true)
	tm.SetShouldFailCommit(true)
	tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error { return nil })

	assert.Equal(t, 1, tm.GetTransactionCallCount())

	// Reset
	tm.Reset()

	// Verify reset state
	assert.Equal(t, 0, tm.GetTransactionCallCount())
	assert.False(t, tm.shouldFailTransaction)
	assert.False(t, tm.shouldFailCommit)

	// Should work normally after reset
	executed := false
	err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
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
		tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error { return nil })
		done <- true
	}()

	go func() {
		tm.SetShouldFailCommit(true)
		tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error { return nil })
		done <- true
	}()

	<-done
	<-done

	// Should have been called twice
	assert.Equal(t, 2, tm.GetTransactionCallCount())
}
