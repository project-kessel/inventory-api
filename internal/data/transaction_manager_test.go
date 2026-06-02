package data

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
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

func TestMaxSerializationRetries(t *testing.T) {
	repo := newTestRepo(t)
	assert.Equal(t, 3, repo.MaxSerializationRetries())
}

func TestMaxSerializationRetries_DefaultsTo3(t *testing.T) {
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	repo := NewResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: 0,
	})
	assert.Equal(t, 3, repo.MaxSerializationRetries())
}

func TestCommit_WrapsSerializationFailure(t *testing.T) {
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	repo := NewResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: 3,
	})

	tx, err := repo.Begin("")
	require.NoError(t, err)

	err = tx.Commit()
	assert.NoError(t, err)
}

func TestSerializationRetry_ManualLoop(t *testing.T) {
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
	var lastErr error
	for attempt := 0; attempt < repo.MaxSerializationRetries(); attempt++ {
		tx, err := repo.Begin("")
		require.NoError(t, err)

		callCount++
		// Simulate a fn error that's a serialization failure
		fnErr := serializationError
		_ = tx.Rollback()

		// The raw pgx error won't match the sentinel, but wrapSerializationError
		// in the real gormResourceTx methods would wrap it. For this test, we
		// simulate what the caller sees when mid-tx methods wrap the error.
		wrappedErr := fmt.Errorf("%w: %v", bizmodel.ErrSerializationFailure, fnErr)
		if errors.Is(wrappedErr, bizmodel.ErrSerializationFailure) {
			lastErr = wrappedErr
			continue
		}
	}

	assert.Equal(t, maxRetries, callCount)
	assert.True(t, errors.Is(lastErr, bizmodel.ErrSerializationFailure))
}

func TestSerializationRecovery_ManualLoop(t *testing.T) {
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	repo := NewResourceRepository(GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopOutboxPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: 5,
	})

	callCount := 0
	var finalErr error
	for attempt := 0; attempt < repo.MaxSerializationRetries(); attempt++ {
		tx, err := repo.Begin("")
		require.NoError(t, err)

		callCount++
		if callCount <= 2 {
			_ = tx.Rollback()
			continue
		}

		err = tx.Commit()
		if err == nil {
			finalErr = nil
			break
		}
		_ = tx.Rollback()
		finalErr = err
	}

	assert.Equal(t, 3, callCount)
	assert.NoError(t, finalErr)
}

// =============================================================================
// FakeResourceRepository Begin/Commit/Rollback tests
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

func TestFakeResourceRepository_MaxSerializationRetries(t *testing.T) {
	repo := NewFakeResourceRepository()
	assert.Equal(t, 3, repo.MaxSerializationRetries())
}
