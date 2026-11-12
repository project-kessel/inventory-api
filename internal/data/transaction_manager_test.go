package data

//TODO

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/testutil"
)

// setupTestDB returns a schema-less test database on the backend selected
// through TEST_DB: PostgreSQL via Testcontainers (the default, matching
// production) or in-memory SQLite (the dependency-free fallback, selected
// explicitly with TEST_DB=sqlite). The
// transaction manager tests exercise begin/commit/retry behavior and need no
// schema.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	if !testutil.UsingPostgres() {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		assert.NoError(t, err)

		// Enable foreign keys for SQLite (PostgreSQL enforces them by default)
		db.Exec("PRAGMA foreign_keys = ON")

		return db
	}

	return testutil.NewTestDB(t, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}, nil)
}

// =============================================================================
// Interface Compliance Tests
// =============================================================================

func TestGormTransactionManager_Interface(t *testing.T) {
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, 3)

	// Verify it implements the interface
	var _ = bizmodel.TransactionManager(tm)
}

func TestFakeTransactionManager_Interface(t *testing.T) {
	tm := NewFakeTransactionManager(3)

	// Verify it implements the interface
	var _ = bizmodel.TransactionManager(tm)
}

// =============================================================================
// GORM Transaction Manager Tests
// =============================================================================

func TestNewGormTransactionManager(t *testing.T) {
	maxRetries := 5
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, maxRetries)

	assert.NotNil(t, tm)

	// Verify it's the correct type
	gormTm, ok := tm.(*gormTransactionManager)
	assert.True(t, ok)
	assert.Equal(t, maxRetries, gormTm.maxSerializationRetries)
	assert.NotNil(t, gormTm.metricsCollector)
}

func TestGormTransactionManager_Success(t *testing.T) {
	db := setupTestDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, 3)

	var capturedTx *gorm.DB
	executed := false

	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
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
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, 3)

	expectedError := errors.New("business logic error")
	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		return expectedError
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed")
	assert.Contains(t, err.Error(), expectedError.Error())
}

func TestGormTransactionManager_MultipleRetries(t *testing.T) {
	db := setupTestDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, 3)

	callCount := 0
	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		callCount++
		return errors.New("some error")
	})

	assert.Error(t, err)
	// Should only call once since it's not a serialization failure
	assert.Equal(t, 1, callCount)
}

// TestGormTransactionManager_TransactionIsolation checks PostgreSQL's reported
// isolation level inside a managed transaction; SQLite exercises successful
// transaction execution without a session-level isolation query.
func TestGormTransactionManager_TransactionIsolation(t *testing.T) {
	db := setupTestDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, 3)

	// Create a simple table for testing
	db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, value TEXT)")

	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		// Verify the transaction actually runs at serializable isolation. On
		// PostgreSQL this reads the session's isolation level directly; SQLite
		// has no such statement and serializable behavior is implicit.
		if testutil.UsingPostgres() {
			var level string
			if err := tx.Raw("SHOW TRANSACTION ISOLATION LEVEL").Scan(&level).Error; err != nil {
				return err
			}
			assert.Equal(t, "serializable", strings.ToLower(strings.TrimSpace(level)))
		}
		return nil
	})

	assert.NoError(t, err)
}

func TestGormTransactionManager_MaxRetries(t *testing.T) {
	db := setupTestDB(t)
	// Test with 0 retries to ensure it fails immediately
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, 0)

	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		return errors.New("test error")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed")
}

func TestGormTransactionManager_SerializationFailureRetries(t *testing.T) {
	db := setupTestDB(t)
	maxRetries := 3
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, maxRetries)

	// Create a mock PostgreSQL serialization failure error
	serializationError := &pgconn.PgError{
		Code:    "40001", // PostgreSQL serialization failure code
		Message: "could not serialize access due to concurrent update",
	}

	callCount := 0
	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		callCount++
		return serializationError
	})

	// Should call the function maxRetries times (initial attempt + retries)
	assert.Equal(t, maxRetries, callCount)

	// Should return an error indicating all retries were exhausted
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed after 3 attempts")
	assert.Contains(t, err.Error(), "could not serialize access due to concurrent update")

	// Check that metrics were recorded
	assert.Equal(t, maxRetries, metricscollector.GetSerializationFailureCount())
	assert.Equal(t, 1, metricscollector.GetSerializationExhaustionCount())
}

func TestSerializationRetryBackoffIsBounded(t *testing.T) {
	assert.Equal(t, time.Millisecond, serializationRetryBackoff(0))
	assert.Equal(t, 2*time.Millisecond, serializationRetryBackoff(1))
	assert.Equal(t, 16*time.Millisecond, serializationRetryBackoff(4))
	assert.Equal(t, 20*time.Millisecond, serializationRetryBackoff(10))
	assert.Equal(t, 20*time.Millisecond, serializationRetryBackoff(100))
}

func TestGormTransactionManager_SerializationFailureRecovery(t *testing.T) {
	db := setupTestDB(t)
	maxRetries := 5
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, maxRetries)

	// Create a mock PostgreSQL serialization failure error
	serializationError := &pgconn.PgError{
		Code:    "40001", // PostgreSQL serialization failure code
		Message: "could not serialize access due to concurrent update",
	}

	callCount := 0
	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		callCount++
		// Fail for the first 2 attempts, then succeed
		if callCount <= 2 {
			return serializationError
		}
		return nil
	})

	// Should call the function 3 times (2 failures + 1 success)
	assert.Equal(t, 3, callCount)

	// Should succeed after retries
	assert.NoError(t, err)

	// Check that serialization failures were recorded but not exhaustion
	assert.Equal(t, 2, metricscollector.GetSerializationFailureCount())
	assert.Equal(t, 0, metricscollector.GetSerializationExhaustionCount())
}

// TestGormTransactionManager_PostgresSerializationConflictRecovery forces a real
// PostgreSQL 40001 conflict and verifies retry with a fresh snapshot: both
// committed increments survive, but writes from the failed attempt are rolled back.
// SQLite is skipped because it cannot reproduce this PostgreSQL snapshot conflict.
func TestGormTransactionManager_PostgresSerializationConflictRecovery(t *testing.T) {
	if !testutil.UsingPostgres() {
		t.Skip("real serialization conflicts require PostgreSQL; SQLite retry coverage remains above")
	}
	db := setupTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db = db.WithContext(ctx)
	require.NoError(t, db.Exec("CREATE TABLE serialization_counter (id INTEGER PRIMARY KEY, value INTEGER NOT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO serialization_counter VALUES (1, 0)").Error)
	require.NoError(t, db.Exec("CREATE TABLE serialization_attempts (attempt INTEGER PRIMARY KEY)").Error)

	snapshotTaken := make(chan struct{})
	writerResult := make(chan error, 1)
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		select {
		case <-snapshotTaken:
			// Autocommit completes before the stale snapshot is allowed to write.
			writerResult <- db.Exec("UPDATE serialization_counter SET value = value + 1 WHERE id = 1").Error
		case <-ctx.Done():
			writerResult <- ctx.Err()
		}
	}()
	defer func() {
		cancel()
		select {
		case <-writerDone:
		case <-time.After(10 * time.Second):
			t.Error("timed out waiting for serialization-conflict writer cleanup")
		}
	}()

	tm := NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 3)
	attempts := 0
	var observedValues []int
	var conflictError error
	err := tm.HandleSerializableTransaction("real_serialization_recovery", db, func(tx *gorm.DB) error {
		attempts++
		var value int
		if err := tx.Raw("SELECT value FROM serialization_counter WHERE id = 1").Scan(&value).Error; err != nil {
			return err
		}
		observedValues = append(observedValues, value)
		if err := tx.Exec("INSERT INTO serialization_attempts VALUES (?)", attempts).Error; err != nil {
			return err
		}
		if attempts == 1 {
			close(snapshotTaken)
			select {
			case err := <-writerResult:
				if err != nil {
					return fmt.Errorf("conflicting writer: %w", err)
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		err := tx.Exec("UPDATE serialization_counter SET value = ? WHERE id = 1", value+1).Error
		if attempts == 1 {
			conflictError = err
		}
		return err
	})
	require.NoError(t, err)
	var pgError *pgconn.PgError
	require.ErrorAs(t, conflictError, &pgError)
	assert.Equal(t, "40001", pgError.SQLState())
	assert.Equal(t, 2, attempts)
	assert.Equal(t, []int{0, 1}, observedValues, "retry must use a fresh snapshot")

	var persistedValue int
	require.NoError(t, db.Raw("SELECT value FROM serialization_counter WHERE id = 1").Scan(&persistedValue).Error)
	assert.Equal(t, 2, persistedValue, "both committed increments must survive")
	var persistedAttempts []int
	require.NoError(t, db.Raw("SELECT attempt FROM serialization_attempts ORDER BY attempt").Scan(&persistedAttempts).Error)
	assert.Equal(t, []int{2}, persistedAttempts, "writes from the failed attempt must be rolled back")
}

func TestGormTransactionManager_DeeplyWrappedSerializationError(t *testing.T) {
	db := setupTestDB(t)
	maxRetries := 2
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, maxRetries)

	// Create a serialization error wrapped multiple layers deep
	baseError := &pgconn.PgError{
		Code:    "40001",
		Message: "could not serialize access due to read/write dependencies among transactions",
	}

	// Wrap it multiple times to simulate complex error handling chains
	layer1Error := fmt.Errorf("database error: %w", baseError)
	layer2Error := fmt.Errorf("repository operation failed: %w", layer1Error)
	layer3Error := fmt.Errorf("service layer error: %w", layer2Error)

	callCount := 0
	err := tm.HandleSerializableTransaction("test_operation", db, func(tx *gorm.DB) error {
		callCount++
		if callCount == 1 {
			return layer3Error
		}
		return nil
	})

	// Should retry even with deeply wrapped errors
	assert.Equal(t, 2, callCount)
	assert.NoError(t, err)

	// Check that the serialization failure was detected and recorded
	assert.Equal(t, 1, metricscollector.GetSerializationFailureCount())
	assert.Equal(t, 0, metricscollector.GetSerializationExhaustionCount())
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
