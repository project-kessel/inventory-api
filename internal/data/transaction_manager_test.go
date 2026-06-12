package data

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
)

// =============================================================================
// Begin/Commit/Rollback tests
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

func TestBeginCommit_Success(t *testing.T) {
	repo := newTestRepo(t)

	tx, err := repo.Begin("")
	require.NoError(t, err)
	require.NotNil(t, tx)

	err = tx.Commit()
	assert.NoError(t, err)
}

func TestBeginRollback_Success(t *testing.T) {
	repo := newTestRepo(t)

	tx, err := repo.Begin("")
	require.NoError(t, err)

	err = tx.Rollback()
	assert.NoError(t, err)
}

// =============================================================================
// Transact tests
// =============================================================================

func TestTransact_Success(t *testing.T) {
	repo := newTestRepo(t)

	called := false
	err := repo.Transact("TestOp", func(tx bizmodel.ResourceTx) error {
		called = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, called)
}

func TestTransact_FnError_NoRetry(t *testing.T) {
	repo := newTestRepo(t)

	expectedErr := errors.New("business logic error")
	callCount := 0
	err := repo.Transact("TestOp", func(tx bizmodel.ResourceTx) error {
		callCount++
		return expectedErr
	})

	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 1, callCount, "non-serialization errors should not retry")
}

func TestTransact_SerializationFailure_Retries(t *testing.T) {
	repo := newTestRepo(t)

	callCount := 0
	err := repo.Transact("TestOp", func(tx bizmodel.ResourceTx) error {
		callCount++
		if callCount < 3 {
			return bizmodel.ErrSerializationFailure
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount, "should retry on serialization failure and succeed on third attempt")
}

func TestTransact_SerializationExhaustion(t *testing.T) {
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	repo := NewResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: 2,
	})

	callCount := 0
	err := repo.Transact("TestOp", func(tx bizmodel.ResourceTx) error {
		callCount++
		return bizmodel.ErrSerializationFailure
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed after 2 attempts")
	assert.Equal(t, 2, callCount, "should exhaust all retry attempts")
}

func TestTransact_DefaultRetries(t *testing.T) {
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	repo := NewResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: 0,
	})

	callCount := 0
	err := repo.Transact("TestOp", func(tx bizmodel.ResourceTx) error {
		callCount++
		return bizmodel.ErrSerializationFailure
	})

	assert.Error(t, err)
	assert.Equal(t, 3, callCount, "MaxSerializationRetries 0 should default to 3")
}

// =============================================================================
// FakeResourceRepository tests
// =============================================================================

func TestFakeResourceRepository_BeginCommit(t *testing.T) {
	repo := NewFakeResourceRepository()

	tx, err := repo.Begin("")
	require.NoError(t, err)
	require.NotNil(t, tx)

	err = tx.Commit()
	assert.NoError(t, err)
}

func TestFakeResourceRepository_BeginRollback(t *testing.T) {
	repo := NewFakeResourceRepository()

	tx, err := repo.Begin("")
	require.NoError(t, err)

	err = tx.Rollback()
	assert.NoError(t, err)
}

func TestFakeResourceRepository_Transact_Success(t *testing.T) {
	repo := NewFakeResourceRepository()

	called := false
	err := repo.Transact("TestOp", func(tx bizmodel.ResourceTx) error {
		called = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, called)
}

func TestFakeResourceRepository_Transact_Error(t *testing.T) {
	repo := NewFakeResourceRepository()

	expectedErr := errors.New("test error")
	err := repo.Transact("TestOp", func(tx bizmodel.ResourceTx) error {
		return expectedErr
	})

	assert.ErrorIs(t, err, expectedErr)
}
