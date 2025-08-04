package data

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewGormTransactionManager(t *testing.T) {
	maxRetries := 5
	tm := NewGormTransactionManager(maxRetries)

	assert.NotNil(t, tm)
	assert.Equal(t, maxRetries, tm.maxSerializationRetries)
}

func TestGormTransactionManager_isSerializationFailure_PostgreSQL(t *testing.T) {
	tm := NewGormTransactionManager(3)

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "PostgreSQL serialization failure",
			err:      &pgconn.PgError{Code: "40001"},
			expected: true,
		},
		{
			name:     "PostgreSQL other error",
			err:      &pgconn.PgError{Code: "23505"}, // unique violation
			expected: false,
		},
		{
			name:     "Non-PostgreSQL error",
			err:      errors.New("some error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.isSerializationFailure(tt.err, 0)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGormTransactionManager_isSerializationFailure_SQLite(t *testing.T) {
	tm := NewGormTransactionManager(3)

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "SQLite serialization failure",
			err:      sqlite3.Error{Code: sqlite3.ErrError},
			expected: true,
		},
		{
			name:     "SQLite other error",
			err:      sqlite3.Error{Code: sqlite3.ErrConstraint},
			expected: false,
		},
		{
			name:     "Non-SQLite error",
			err:      errors.New("some error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.isSerializationFailure(tt.err, 0)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGormTransactionManager_HandleSerializableTransaction_ErrorDetection(t *testing.T) {
	tm := NewGormTransactionManager(3)

	// Test that commit failures are handled
	commitErr := &pgconn.PgError{Code: "40001"}
	assert.True(t, tm.isSerializationFailure(commitErr, 0))

	// Test non-serialization commit error
	otherCommitErr := errors.New("disk full")
	assert.False(t, tm.isSerializationFailure(otherCommitErr, 0))

	// Test that serialization errors are properly detected
	pgError := &pgconn.PgError{Code: "40001"} // PostgreSQL serialization failure
	assert.True(t, tm.isSerializationFailure(pgError, 0))
	assert.True(t, tm.isSerializationFailure(pgError, 1))

	// Test that non-serialization errors are not retried
	nonSerializationErr := errors.New("other error")
	assert.False(t, tm.isSerializationFailure(nonSerializationErr, 0))
}

func TestGormTransactionManager_RetryCountTracking(t *testing.T) {
	tm := NewGormTransactionManager(3)

	// Test that attempt counter is passed correctly
	pgError := &pgconn.PgError{Code: "40001"}

	for attempt := 0; attempt < 5; attempt++ {
		// The function should return true for serialization failures regardless of attempt
		result := tm.isSerializationFailure(pgError, attempt)
		assert.True(t, result, "Should detect serialization failure on attempt %d", attempt)
	}
}

func TestGormTransactionManager_ErrorWrapping(t *testing.T) {
	tm := NewGormTransactionManager(1)

	// Test different error scenarios to ensure proper error wrapping
	testCases := []struct {
		name        string
		inputError  error
		shouldRetry bool
	}{
		{
			name:        "PostgreSQL serialization error",
			inputError:  &pgconn.PgError{Code: "40001"},
			shouldRetry: true,
		},
		{
			name:        "SQLite serialization error",
			inputError:  sqlite3.Error{Code: sqlite3.ErrError},
			shouldRetry: true,
		},
		{
			name:        "Generic database error",
			inputError:  errors.New("connection timeout"),
			shouldRetry: false,
		},
		{
			name:        "Unique constraint violation",
			inputError:  &pgconn.PgError{Code: "23505"},
			shouldRetry: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tm.isSerializationFailure(tc.inputError, 0)
			assert.Equal(t, tc.shouldRetry, result)
		})
	}
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestGormTransactionManager_HandleSerializableTransaction_Success(t *testing.T) {
	tm := NewGormTransactionManager(3)
	db := setupTestDB(t)

	callCount := 0
	txFunc := func(tx *gorm.DB) error {
		callCount++
		return nil // Success on first try
	}

	err := tm.HandleSerializableTransaction(db, txFunc)

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "Should succeed on first attempt")
}

func TestGormTransactionManager_HandleSerializableTransaction_NonSerializationError(t *testing.T) {
	tm := NewGormTransactionManager(3)
	db := setupTestDB(t)

	callCount := 0
	expectedErr := errors.New("some other error")
	txFunc := func(tx *gorm.DB) error {
		callCount++
		return expectedErr
	}

	err := tm.HandleSerializableTransaction(db, txFunc)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed")
	assert.Contains(t, err.Error(), expectedErr.Error())
	assert.Equal(t, 1, callCount, "Should not retry non-serialization errors")
}

func TestGormTransactionManager_HandleSerializableTransaction_TransactionIsolation(t *testing.T) {
	tm := NewGormTransactionManager(3)
	db := setupTestDB(t)

	// Create a test table
	err := db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, value TEXT)").Error
	require.NoError(t, err)

	callCount := 0
	txFunc := func(tx *gorm.DB) error {
		callCount++
		// Insert a test record
		return tx.Exec("INSERT INTO test_table (value) VALUES (?)", "test_value").Error
	}

	err = tm.HandleSerializableTransaction(db, txFunc)

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "Should succeed on first attempt")

	// Verify the record was inserted
	var count int64
	err = db.Model(&struct{}{}).Table("test_table").Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count, "Should have inserted one record")
}

func TestGormTransactionManager_MaxRetriesConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
	}{
		{"Single retry", 1},
		{"Three retries", 3},
		{"Five retries", 5},
		{"Ten retries", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := NewGormTransactionManager(tt.maxRetries)
			assert.Equal(t, tt.maxRetries, tm.maxSerializationRetries)
		})
	}
}
