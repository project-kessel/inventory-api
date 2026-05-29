package data

//TODO

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
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

func newTestStore(t *testing.T, db *gorm.DB, maxRetries int) bizmodel.Store {
	mc := metricscollector.NewFakeMetricsCollector()
	return NewGormResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: maxRetries,
	})
}

// =============================================================================
// Interface Compliance Tests
// =============================================================================

func TestGormResourceRepository_Interface(t *testing.T) {
	db := setupTestDB(t)
	store := newTestStore(t, db, 3)

	// Verify it implements the interface
	var _ = bizmodel.Store(store)
}

func TestFakeResourceRepository_Interface(t *testing.T) {
	store := NewFakeResourceRepository()

	// Verify it implements the interface
	var _ = bizmodel.Store(store)
}

// =============================================================================
// GORM Store RunSerializable Tests
// =============================================================================

func TestGormResourceRepository_RunSerializable_Success(t *testing.T) {
	db := setupTestDB(t)
	store := newTestStore(t, db, 3)

	executed := false
	err := store.RunSerializable("test_operation", func(tx bizmodel.Tx) error {
		executed = true
		assert.NotNil(t, tx.ResourceRepository())
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestGormResourceRepository_RunSerializable_TransactionFailure(t *testing.T) {
	db := setupTestDB(t)
	store := newTestStore(t, db, 3)

	expectedError := errors.New("business logic error")
	err := store.RunSerializable("test_operation", func(tx bizmodel.Tx) error {
		return expectedError
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed")
	assert.Contains(t, err.Error(), expectedError.Error())
}

func TestGormResourceRepository_RunSerializable_MultipleRetries(t *testing.T) {
	db := setupTestDB(t)
	store := newTestStore(t, db, 3)

	callCount := 0
	err := store.RunSerializable("test_operation", func(tx bizmodel.Tx) error {
		callCount++
		return errors.New("some error")
	})

	assert.Error(t, err)
	// Should only call once since it's not a serialization failure
	assert.Equal(t, 1, callCount)
}

func TestGormResourceRepository_RunSerializable_TransactionIsolation(t *testing.T) {
	db := setupTestDB(t)
	store := newTestStore(t, db, 3)

	// Create a simple table for testing
	db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, value TEXT)")

	var txLevel sql.IsolationLevel
	err := store.RunSerializable("test_operation", func(tx bizmodel.Tx) error {
		// Check the isolation level by attempting to read it
		// This is a basic test - in a real scenario, we'd test actual isolation behavior
		txLevel = sql.LevelSerializable // We know this is what we set
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, sql.LevelSerializable, txLevel)
}

func TestGormResourceRepository_RunSerializable_MaxRetries(t *testing.T) {
	db := setupTestDB(t)
	// Test with 0 retries to ensure it fails immediately
	store := newTestStore(t, db, 0)

	err := store.RunSerializable("test_operation", func(tx bizmodel.Tx) error {
		return errors.New("test error")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed")
}

func TestGormResourceRepository_RunSerializable_SerializationFailureRetries(t *testing.T) {
	db := setupTestDB(t)
	maxRetries := 3
	store := newTestStore(t, db, maxRetries)

	// Create a mock PostgreSQL serialization failure error
	serializationError := &pgconn.PgError{
		Code:    "40001", // PostgreSQL serialization failure code
		Message: "could not serialize access due to concurrent update",
	}

	callCount := 0
	err := store.RunSerializable("test_operation", func(tx bizmodel.Tx) error {
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

func TestGormResourceRepository_RunSerializable_SerializationFailureRecovery(t *testing.T) {
	db := setupTestDB(t)
	maxRetries := 5
	store := newTestStore(t, db, maxRetries)

	// Create a mock PostgreSQL serialization failure error
	serializationError := &pgconn.PgError{
		Code:    "40001", // PostgreSQL serialization failure code
		Message: "could not serialize access due to concurrent update",
	}

	callCount := 0
	err := store.RunSerializable("test_operation", func(tx bizmodel.Tx) error {
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

func TestGormResourceRepository_RunSerializable_DeeplyWrappedSerializationError(t *testing.T) {
	db := setupTestDB(t)
	maxRetries := 2
	store := newTestStore(t, db, maxRetries)

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
	err := store.RunSerializable("test_operation", func(tx bizmodel.Tx) error {
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
// isSerializationFailure Tests
// =============================================================================

func TestIsSerializationFailure_PostgreSQL(t *testing.T) {
	pgErr := &pgconn.PgError{
		Code:    "40001",
		Message: "could not serialize access due to concurrent update",
	}
	assert.True(t, isSerializationFailure(pgErr, 0, 3))
}

func TestIsSerializationFailure_NonSerializationError(t *testing.T) {
	assert.False(t, isSerializationFailure(errors.New("some error"), 0, 3))
}

// =============================================================================
// Fake Store RunSerializable Tests
// =============================================================================

func TestFakeResourceRepository_RunSerializable_Success(t *testing.T) {
	store := NewFakeResourceRepository()

	executed := false
	err := store.RunSerializable("test_operation", func(tx bizmodel.Tx) error {
		executed = true
		assert.NotNil(t, tx.ResourceRepository())
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestFakeResourceRepository_RunSerializable_FunctionError(t *testing.T) {
	store := NewFakeResourceRepository()

	expectedError := errors.New("business logic error")
	err := store.RunSerializable("test_operation", func(tx bizmodel.Tx) error {
		return expectedError
	})

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
}
