package cleanup

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cleanupRepositoryFake struct {
	previewReport model.TombstoneCleanupReport
	previewError  error
	previewLimit  int64
	batches       []model.TombstoneCleanupImpact
	deleteCalls   int
	deleteError   error
}

func (repository *cleanupRepositoryFake) PreviewTombstones(_ context.Context, _ model.TombstoneCleanupPolicy, boundedReporterRowLimit int64) (model.TombstoneCleanupReport, error) {
	repository.previewLimit = boundedReporterRowLimit
	return repository.previewReport, repository.previewError
}

func (repository *cleanupRepositoryFake) DeleteTombstoneBatch(_ context.Context, _ model.TombstoneCleanupPolicy, _ int) (model.TombstoneCleanupImpact, error) {
	repository.deleteCalls++
	if repository.deleteError != nil {
		return model.TombstoneCleanupImpact{}, repository.deleteError
	}
	index := repository.deleteCalls - 1
	if index >= len(repository.batches) {
		return model.TombstoneCleanupImpact{}, nil
	}
	return repository.batches[index], nil
}

type cleanupTransactionFake struct {
	repository       model.TombstoneCleanupRepository
	readOnly         []bool
	operationNames   []string
	transactionError error
}

func (transaction *cleanupTransactionFake) WithinSerializableTombstoneCleanup(
	ctx context.Context,
	operationName string,
	readOnly bool,
	_, _ time.Duration,
	work func(model.TombstoneCleanupRepository) error,
) error {
	transaction.readOnly = append(transaction.readOnly, readOnly)
	transaction.operationNames = append(transaction.operationNames, operationName)
	if err := work(transaction.repository); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return transaction.transactionError
}

func TestPreviewUsesReadOnlyTransactionAndRowBudget(t *testing.T) {
	policy := cleanupPolicy(t)
	impact := cleanupImpact(t, 4, 4, 12, 3, 3, 2)
	bounded := cleanupImpact(t, 2, 2, 6, 0, 2, 0)
	exclusions, err := model.NewTombstoneCleanupExclusions(1, 0, 0, 0, 0, 0)
	require.NoError(t, err)
	report, err := model.NewTombstoneCleanupReport(
		policy.EvaluationTime(), policy.EvaluationTime().Add(time.Second), policy.Cutoff(),
		2, 5, 0, impact, bounded, exclusions,
	)
	require.NoError(t, err)
	repository := &cleanupRepositoryFake{previewReport: report}
	transaction := &cleanupTransactionFake{repository: repository}
	usecase, err := New(transaction)
	require.NoError(t, err)
	options := cleanupOptions(t, 2, 0, 3, time.Minute)

	got, err := usecase.Preview(context.Background(), policy, options)
	require.NoError(t, err)
	assert.Equal(t, report, got)
	assert.Equal(t, int64(6), repository.previewLimit)
	assert.Equal(t, []bool{true}, transaction.readOnly)
	assert.Equal(t, []string{"tombstone_cleanup_preview"}, transaction.operationNames)
	assert.Zero(t, repository.deleteCalls)
}

func TestExecuteAccumulatesCommittedBatchCountsAndStopsAtBudget(t *testing.T) {
	policy := cleanupPolicy(t)
	first := cleanupImpact(t, 2, 2, 5, 1, 2, 0)
	second := cleanupImpact(t, 2, 2, 4, 2, 1, 1)
	repository := &cleanupRepositoryFake{batches: []model.TombstoneCleanupImpact{first, second}}
	transaction := &cleanupTransactionFake{repository: repository}
	usecase, err := New(transaction)
	require.NoError(t, err)
	options := cleanupOptions(t, 2, 0, 2, time.Minute)
	var committedCallbacks int

	result, err := usecase.Execute(context.Background(), policy, options, func(_ int, _ model.TombstoneCleanupImpact) {
		committedCallbacks++
	})
	require.NoError(t, err)
	expected, err := first.Add(second)
	require.NoError(t, err)
	assert.Equal(t, expected, result.Impact())
	assert.Equal(t, 2, result.CommittedBatches())
	assert.True(t, result.BatchLimitReached())
	assert.Equal(t, 2, committedCallbacks)
	assert.Equal(t, 2, repository.deleteCalls)
	assert.Equal(t, []bool{false, false}, transaction.readOnly)
}

func TestExecuteCountsAResourceRootOnlyOnceAcrossBatches(t *testing.T) {
	policy := cleanupPolicy(t)
	sharedRoot, err := model.NewResourceId(uuid.New())
	require.NoError(t, err)
	first, err := model.NewTombstoneCleanupImpactWithRootIDs(1, 1, 2, 0, []model.ResourceId{sharedRoot}, 0)
	require.NoError(t, err)
	second, err := model.NewTombstoneCleanupImpactWithRootIDs(1, 1, 1, 2, []model.ResourceId{sharedRoot}, 1)
	require.NoError(t, err)
	repository := &cleanupRepositoryFake{batches: []model.TombstoneCleanupImpact{first, second}}
	transaction := &cleanupTransactionFake{repository: repository}
	usecase, err := New(transaction)
	require.NoError(t, err)
	options := cleanupOptions(t, 1, 0, 2, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Impact().ResourceRootsImpacted())
	assert.Equal(t, int64(1), result.Impact().ResourcesRemovable())
}

func TestExecuteDoesNotCountBatchWhenTransactionReturnsError(t *testing.T) {
	policy := cleanupPolicy(t)
	repository := &cleanupRepositoryFake{batches: []model.TombstoneCleanupImpact{cleanupImpact(t, 1, 1, 2, 1, 1, 1)}}
	transaction := &cleanupTransactionFake{repository: repository, transactionError: errors.New("commit failed")}
	usecase, err := New(transaction)
	require.NoError(t, err)
	options := cleanupOptions(t, 10, 0, 1, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, nil)
	require.Error(t, err)
	assert.Equal(t, model.TombstoneCleanupImpact{}, result.Impact())
	assert.Zero(t, result.CommittedBatches())
	assert.False(t, result.BatchLimitReached())
}

func TestExecuteCancellationBetweenBatchesReturnsCommittedProgress(t *testing.T) {
	policy := cleanupPolicy(t)
	first := cleanupImpact(t, 1, 1, 2, 1, 1, 1)
	repository := &cleanupRepositoryFake{batches: []model.TombstoneCleanupImpact{first, cleanupImpact(t, 1, 1, 2, 1, 1, 1)}}
	transaction := &cleanupTransactionFake{repository: repository}
	usecase, err := New(transaction)
	require.NoError(t, err)
	options := cleanupOptions(t, 1, time.Hour, 3, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := usecase.Execute(ctx, policy, options, func(int, model.TombstoneCleanupImpact) { cancel() })
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, first, result.Impact())
	assert.Equal(t, 1, result.CommittedBatches())
	assert.Equal(t, 1, repository.deleteCalls)
	assert.Equal(t, 1, len(transaction.readOnly))
}

func TestNewOptionsRejectsInvalidOrOverflowingLimits(t *testing.T) {
	for _, test := range []struct {
		name      string
		batchSize int
		delay     time.Duration
		batches   int
		duration  time.Duration
	}{
		{name: "zero batch", batchSize: 0, batches: 1, duration: time.Minute},
		{name: "negative delay", batchSize: 1, delay: -time.Second, batches: 1, duration: time.Minute},
		{name: "zero batches", batchSize: 1, batches: 0, duration: time.Minute},
		{name: "zero duration", batchSize: 1, batches: 1},
		{name: "row budget overflow", batchSize: math.MaxInt, batches: math.MaxInt, duration: time.Minute},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewOptions(test.batchSize, test.delay, test.batches, test.duration)
			assert.Error(t, err)
		})
	}
}

func cleanupPolicy(t *testing.T) model.TombstoneCleanupPolicy {
	t.Helper()
	policy, err := model.NewTombstoneCleanupPolicy(time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC), 7, " hbi ")
	require.NoError(t, err)
	return policy
}

func cleanupOptions(t *testing.T, batchSize int, delay time.Duration, batches int, duration time.Duration) Options {
	t.Helper()
	options, err := NewOptions(batchSize, delay, batches, duration)
	require.NoError(t, err)
	return options
}

func cleanupImpact(t *testing.T, entities, reporterRows, reporterHistory, commonHistory, roots, removedRoots int64) model.TombstoneCleanupImpact {
	t.Helper()
	rootIDs := make([]model.ResourceId, 0, roots)
	for rootIndex := int64(0); rootIndex < roots; rootIndex++ {
		rootID, err := model.NewResourceId(uuid.New())
		require.NoError(t, err)
		rootIDs = append(rootIDs, rootID)
	}
	impact, err := model.NewTombstoneCleanupImpactWithRootIDs(entities, reporterRows, reporterHistory, commonHistory, rootIDs, removedRoots)
	require.NoError(t, err)
	return impact
}
