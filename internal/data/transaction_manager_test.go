package data

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
)

// =============================================================================
// Transact serialization retry tests
// =============================================================================

func newTestRepo(t *testing.T) bizmodel.ResourceRepository {
	t.Helper()
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	return NewResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: 3,
	})
}

func TestTransact_Success(t *testing.T) {
	repo := newTestRepo(t)

	executed := false
	err := repo.Transact(func(tx bizmodel.ResourceTx) error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestTransact_TransactionFailure(t *testing.T) {
	repo := newTestRepo(t)

	expectedError := errors.New("business logic error")
	err := repo.Transact(func(tx bizmodel.ResourceTx) error {
		return expectedError
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed")
	assert.Contains(t, err.Error(), expectedError.Error())
}

func TestTransact_NonSerializationFailure_NoRetry(t *testing.T) {
	repo := newTestRepo(t)

	callCount := 0
	err := repo.Transact(func(tx bizmodel.ResourceTx) error {
		callCount++
		return errors.New("some error")
	})

	assert.Error(t, err)
	assert.Equal(t, 1, callCount)
}

func TestTransact_SerializationFailureRetries(t *testing.T) {
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	maxRetries := 3
	repo := NewResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: maxRetries,
	})

	serializationError := &pgconn.PgError{
		Code:    "40001",
		Message: "could not serialize access due to concurrent update",
	}

	callCount := 0
	err := repo.Transact(func(tx bizmodel.ResourceTx) error {
		callCount++
		return serializationError
	})

	assert.Equal(t, maxRetries, callCount)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed after 3 attempts")
	assert.Contains(t, err.Error(), "could not serialize access due to concurrent update")

	assert.Equal(t, maxRetries, metricscollector.GetSerializationFailureCount())
	assert.Equal(t, 1, metricscollector.GetSerializationExhaustionCount())
}

func TestTransact_SerializationFailureRecovery(t *testing.T) {
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	repo := NewResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: 5,
	})

	serializationError := &pgconn.PgError{
		Code:    "40001",
		Message: "could not serialize access due to concurrent update",
	}

	callCount := 0
	err := repo.Transact(func(tx bizmodel.ResourceTx) error {
		callCount++
		if callCount <= 2 {
			return serializationError
		}
		return nil
	})

	assert.Equal(t, 3, callCount)
	assert.NoError(t, err)

	assert.Equal(t, 2, metricscollector.GetSerializationFailureCount())
	assert.Equal(t, 0, metricscollector.GetSerializationExhaustionCount())
}

func TestTransact_DeeplyWrappedSerializationError(t *testing.T) {
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	repo := NewResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: 2,
	})

	baseError := &pgconn.PgError{
		Code:    "40001",
		Message: "could not serialize access due to read/write dependencies among transactions",
	}
	layer1Error := fmt.Errorf("database error: %w", baseError)
	layer2Error := fmt.Errorf("repository operation failed: %w", layer1Error)
	layer3Error := fmt.Errorf("service layer error: %w", layer2Error)

	callCount := 0
	err := repo.Transact(func(tx bizmodel.ResourceTx) error {
		callCount++
		if callCount == 1 {
			return layer3Error
		}
		return nil
	})

	assert.Equal(t, 2, callCount)
	assert.NoError(t, err)

	assert.Equal(t, 1, metricscollector.GetSerializationFailureCount())
	assert.Equal(t, 0, metricscollector.GetSerializationExhaustionCount())
}

func TestTransact_ZeroMaxRetries(t *testing.T) {
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	repo := NewResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: 0,
	})

	err := repo.Transact(func(tx bizmodel.ResourceTx) error {
		return errors.New("test error")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed")
}

// =============================================================================
// FakeResourceRepository Transact tests
// =============================================================================

func TestFakeResourceRepository_TransactSuccess(t *testing.T) {
	repo := NewFakeResourceRepository()

	executed := false
	err := repo.Transact(func(tx bizmodel.ResourceTx) error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestFakeResourceRepository_TransactFailure(t *testing.T) {
	repo := NewFakeResourceRepository()

	expectedError := errors.New("business logic error")
	err := repo.Transact(func(tx bizmodel.ResourceTx) error {
		return expectedError
	})

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
}
