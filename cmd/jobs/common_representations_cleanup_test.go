package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommonRepCleanupValidatesRetentionAndBounds(t *testing.T) {
	policy, err := newCommonRepCleanupPolicy(2, " host ")
	require.NoError(t, err)
	resourceType, filtered := policy.ResourceType()
	assert.True(t, filtered)
	assert.Equal(t, "host", resourceType.String())
	for _, keep := range []int{-1, 0, 1} {
		_, err := newCommonRepCleanupPolicy(keep, "")
		assert.Error(t, err)
	}
	_, err = newCommonRepCleanupPolicy(2, "   ")
	assert.Error(t, err)
	assert.Error(t, (commonRepCleanupPolicy{}).Validate())
	for _, options := range []commonRepCleanupOptions{
		{chunkRows: 0, batchSize: 1, maxBatches: 1, maxDuration: time.Minute},
		{chunkRows: 1, batchSize: 0, maxBatches: 1, maxDuration: time.Minute},
		{chunkRows: 1, batchSize: 1, maxBatches: 0, maxDuration: time.Minute},
		{chunkRows: 1, batchSize: 1, maxBatches: 1},
		{chunkRows: 1, batchSize: 1, maxBatches: 1, maxDuration: time.Minute, batchDelay: -1},
	} {
		assert.Error(t, options.Validate())
	}
}

// commonRepMemoryStore provides a single chunk with mutable prunable row counts.
// PostgreSQL tests exercise the actual retention query and transaction behavior.
type commonRepMemoryStore struct {
	roots       []uuid.UUID
	remaining   map[uuid.UUID]int64
	commitError error
}

func (store *commonRepMemoryStore) ReadChunk(_ context.Context, _ commonRepCleanupPolicy, after uuid.UUID, _ int) ([]commonRepRootClassification, uuid.UUID, bool, error) {
	var rows []commonRepRootClassification
	for _, rootID := range store.roots {
		if rootID.String() > after.String() {
			rows = append(rows, commonRepRootClassification{resourceID: rootID, prunableRows: store.remaining[rootID]})
		}
	}
	return rows, uuid.Nil, false, nil
}

func (store *commonRepMemoryStore) DeletePrunableRoots(_ context.Context, _ commonRepCleanupPolicy, rootIDs []uuid.UUID, limit int) (commonRepCleanupImpact, error) {
	impact := commonRepCleanupImpact{}
	for _, rootID := range rootIDs {
		deleted := min(store.remaining[rootID], int64(limit)-impact.representations)
		store.remaining[rootID] -= deleted
		impact.representations += deleted
		if deleted > 0 {
			impact.resourceRootIDs = append(impact.resourceRootIDs, rootID)
		}
	}
	return impact, nil
}

func (store *commonRepMemoryStore) WithinSerializableCommonRepCleanup(ctx context.Context, _ string, readOnly bool, _, _ time.Duration, work func(commonRepCleanupRepository) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := work(store); err != nil {
		return err
	}
	if !readOnly {
		return store.commitError
	}
	return nil
}

func commonRepTestRun(t *testing.T, rows ...int64) (*commonRepCleanupUsecase, *commonRepMemoryStore, commonRepCleanupPolicy, commonRepCleanupOptions) {
	t.Helper()
	store := &commonRepMemoryStore{remaining: make(map[uuid.UUID]int64)}
	for index, count := range rows {
		rootID := uuid.UUID{15: byte(index + 1)}
		store.roots = append(store.roots, rootID)
		store.remaining[rootID] = count
	}
	usecase, err := newCommonRepCleanup(store)
	require.NoError(t, err)
	policy, err := newCommonRepCleanupPolicy(2, "")
	require.NoError(t, err)
	options, err := newCommonRepCleanupOptions(100, 4, 0, 10, time.Minute)
	require.NoError(t, err)
	return usecase, store, policy, options
}

func TestCommonRepPreviewDoesNotDelete(t *testing.T) {
	usecase, store, policy, options := commonRepTestRun(t, 3, 2)
	report, err := usecase.Preview(context.Background(), policy, options, uuid.Nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(5), report.prunableRows)
	assert.True(t, report.passComplete)
	assert.Equal(t, map[uuid.UUID]int64{store.roots[0]: 3, store.roots[1]: 2}, store.remaining)
}

func TestCommonRepExecuteUsesBoundedBatchesAndCountsDistinctRoots(t *testing.T) {
	usecase, store, policy, options := commonRepTestRun(t, 9, 2)
	var batches []int64
	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, func(_ int, impact commonRepCleanupImpact) {
		batches = append(batches, impact.representations)
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, []int64{4, 4, 3}, batches)
	assert.Equal(t, int64(11), result.deletedRows)
	assert.Equal(t, int64(2), result.impactedRoots)
	assert.True(t, result.report.passComplete)
	assert.Equal(t, map[uuid.UUID]int64{store.roots[0]: 0, store.roots[1]: 0}, store.remaining)
}

func TestCommonRepBudgetStopResumesTheInterruptedChunk(t *testing.T) {
	usecase, store, policy, options := commonRepTestRun(t, 3, 3, 3)
	options.maxBatches = 1
	first, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(4), first.deletedRows)
	assert.True(t, first.batchLimitReached)
	assert.False(t, first.report.passComplete)
	assert.Equal(t, uuid.Nil, first.report.lastCursor)
	options.maxBatches = 10
	second, err := usecase.Execute(context.Background(), policy, options, first.report.lastCursor, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(5), second.deletedRows)
	assert.True(t, second.report.passComplete)
	assert.Equal(t, int64(0), store.remaining[store.roots[2]])
}

func TestCommonRepFailedCommitDoesNotReportAttemptedDeletes(t *testing.T) {
	usecase, store, policy, options := commonRepTestRun(t, 3)
	store.commitError = errors.New("commit failed")
	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
	require.ErrorIs(t, err, store.commitError)
	assert.Zero(t, result.deletedRows)
	assert.Zero(t, result.committedBatches)
	assert.False(t, result.report.passComplete)
}

func TestCommonRepCancellationReturnsCommittedProgress(t *testing.T) {
	usecase, _, policy, options := commonRepTestRun(t, 9)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := usecase.Execute(ctx, policy, options, uuid.Nil, func(_ int, _ commonRepCleanupImpact) { cancel() }, nil)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int64(4), result.deletedRows)
	assert.Equal(t, 1, result.committedBatches)
	assert.False(t, result.report.passComplete)
}

func TestCommonRepPreviewCancellationReturnsPartialReport(t *testing.T) {
	usecase, _, policy, options := commonRepTestRun(t, 9)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := usecase.Preview(ctx, policy, options, uuid.Nil, nil)
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, report.passComplete)
	assert.False(t, report.finishedAt.IsZero())
}
