package data

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewFakeTransactionManager(t *testing.T) {
	fake := NewFakeTransactionManager()
	
	assert.NotNil(t, fake)
	assert.Equal(t, 0, fake.CallCount)
	assert.False(t, fake.ShouldFail)
	assert.Nil(t, fake.FailureError)
	assert.Equal(t, 0, fake.RetryCount)
	assert.Equal(t, 0, fake.RetryAttempts)
}

func TestFakeTransactionManager_HandleSerializableTransaction_Success(t *testing.T) {
	fake := NewFakeTransactionManager()
	db := setupFakeTestDB(t)
	
	callCount := 0
	txFunc := func(tx *gorm.DB) error {
		callCount++
		return nil
	}
	
	err := fake.HandleSerializableTransaction(db, txFunc)
	
	assert.NoError(t, err)
	assert.Equal(t, 1, fake.CallCount)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, 1, fake.RetryAttempts)
}

func TestFakeTransactionManager_HandleSerializableTransaction_Failure(t *testing.T) {
	fake := NewFakeTransactionManager()
	db := setupFakeTestDB(t)
	
	expectedErr := errors.New("simulated failure")
	fake.SimulateFailure(expectedErr)
	
	callCount := 0
	txFunc := func(tx *gorm.DB) error {
		callCount++
		return nil
	}
	
	err := fake.HandleSerializableTransaction(db, txFunc)
	
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, 1, fake.CallCount)
	assert.Equal(t, 0, callCount) // Should not call txFunc when configured to fail immediately
}

func TestFakeTransactionManager_HandleSerializableTransaction_CustomFailure(t *testing.T) {
	fake := NewFakeTransactionManager()
	db := setupFakeTestDB(t)
	
	fake.ShouldFail = true
	// FailureError is nil, so it should use the default error message
	
	err := fake.HandleSerializableTransaction(db, txFunc)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fake transaction manager simulated failure")
	assert.Equal(t, 1, fake.CallCount)
}

func TestFakeTransactionManager_HandleSerializableTransaction_WithRetries(t *testing.T) {
	fake := NewFakeTransactionManager()
	db := setupFakeTestDB(t)
	
	fake.SimulateRetries(3)
	
	callCount := 0
	txFunc := func(tx *gorm.DB) error {
		callCount++
		return nil
	}
	
	err := fake.HandleSerializableTransaction(db, txFunc)
	
	assert.NoError(t, err)
	assert.Equal(t, 1, fake.CallCount)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, 4, fake.RetryAttempts) // 3 retries + 1 successful attempt
}

func TestFakeTransactionManager_HandleSerializableTransaction_RetriesWithFailure(t *testing.T) {
	fake := NewFakeTransactionManager()
	db := setupFakeTestDB(t)
	
	fake.SimulateRetries(2)
	expectedErr := errors.New("failure after retries")
	fake.SimulateFailure(expectedErr)
	
	callCount := 0
	txFunc := func(tx *gorm.DB) error {
		callCount++
		return nil
	}
	
	err := fake.HandleSerializableTransaction(db, txFunc)
	
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, 1, fake.CallCount)
	assert.Equal(t, 0, callCount) // Should fail on the last retry attempt
	assert.Equal(t, 3, fake.RetryAttempts) // 2 retries + 1 final failure attempt
}

func TestFakeTransactionManager_Reset(t *testing.T) {
	fake := NewFakeTransactionManager()
	db := setupFakeTestDB(t)
	
	// Set up some state
	fake.SimulateFailure(errors.New("test error"))
	fake.SimulateRetries(5)
	
	// Execute a transaction to change the state
	_ = fake.HandleSerializableTransaction(db, func(tx *gorm.DB) error { return nil })
	
	// Verify state has changed
	assert.Equal(t, 1, fake.CallCount)
	assert.True(t, fake.ShouldFail)
	assert.NotNil(t, fake.FailureError)
	assert.Equal(t, 5, fake.RetryCount)
	assert.Equal(t, 6, fake.RetryAttempts)
	
	// Reset and verify clean state
	fake.Reset()
	
	assert.Equal(t, 0, fake.CallCount)
	assert.False(t, fake.ShouldFail)
	assert.Nil(t, fake.FailureError)
	assert.Equal(t, 0, fake.RetryCount)
	assert.Equal(t, 0, fake.RetryAttempts)
}

func TestFakeTransactionManager_MultipleTransactions(t *testing.T) {
	fake := NewFakeTransactionManager()
	db := setupFakeTestDB(t)
	
	txFunc := func(tx *gorm.DB) error {
		return nil
	}
	
	// Execute multiple transactions
	for i := 0; i < 5; i++ {
		err := fake.HandleSerializableTransaction(db, txFunc)
		assert.NoError(t, err)
	}
	
	assert.Equal(t, 5, fake.CallCount)
	assert.Equal(t, 5, fake.RetryAttempts)
}

func TestFakeTransactionManager_SimulateFailure(t *testing.T) {
	fake := NewFakeTransactionManager()
	expectedErr := errors.New("test failure")
	
	fake.SimulateFailure(expectedErr)
	
	assert.True(t, fake.ShouldFail)
	assert.Equal(t, expectedErr, fake.FailureError)
}

func TestFakeTransactionManager_SimulateRetries(t *testing.T) {
	fake := NewFakeTransactionManager()
	retryCount := 7
	
	fake.SimulateRetries(retryCount)
	
	assert.Equal(t, retryCount, fake.RetryCount)
}

// setupFakeTestDB creates an in-memory SQLite database for testing
func setupFakeTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// Helper function for tests
func txFunc(tx *gorm.DB) error {
	return nil
}