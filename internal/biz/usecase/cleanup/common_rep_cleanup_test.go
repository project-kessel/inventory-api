package cleanup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commonRepMemoryStore provides a single chunk with mutable prunable row counts.
// PostgreSQL tests exercise the actual retention query and transaction behavior.
type commonRepMemoryStore struct {
	roots       []uuid.UUID
	remaining   map[uuid.UUID]int64
	commitError error
}

func (store *commonRepMemoryStore) ReadChunk(_ context.Context, _ bizmodel.CommonRepCleanupPolicy, after uuid.UUID, _ int) ([]bizmodel.CommonRepRootClassification, uuid.UUID, bool, error) {
	var rows []bizmodel.CommonRepRootClassification
	for _, rootID := range store.roots {
		if rootID.String() > after.String() {
			classification, err := bizmodel.NewCommonRepRootClassification(bizmodel.DeserializeResourceId(rootID), "", store.remaining[rootID])
			if err != nil {
				return nil, uuid.Nil, false, err
			}
			rows = append(rows, classification)
		}
	}
	return rows, uuid.Nil, false, nil
}

func (store *commonRepMemoryStore) DeletePrunableRoots(_ context.Context, _ bizmodel.CommonRepCleanupPolicy, rootIDs []bizmodel.ResourceId, limit int) (bizmodel.CommonRepCleanupImpact, error) {
	var rows int64
	var roots []bizmodel.ResourceId
	for _, rootID := range rootIDs {
		deleted := min(store.remaining[rootID.UUID()], int64(limit)-rows)
		store.remaining[rootID.UUID()] -= deleted
		rows += deleted
		if deleted > 0 {
			roots = append(roots, rootID)
		}
	}
	return bizmodel.NewCommonRepCleanupImpact(rows, roots)
}

func (store *commonRepMemoryStore) WithinSerializableCommonRepCleanup(ctx context.Context, _ string, readOnly bool, _, _ time.Duration, work func(bizmodel.CommonRepCleanupRepository) error) error {
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

func commonRepTestRun(t *testing.T, rows ...int64) (*Usecase, *commonRepMemoryStore, bizmodel.CommonRepCleanupPolicy, Options) {
	t.Helper()
	store := &commonRepMemoryStore{remaining: make(map[uuid.UUID]int64)}
	for index, count := range rows {
		rootID := uuid.UUID{15: byte(index + 1)}
		store.roots = append(store.roots, rootID)
		store.remaining[rootID] = count
	}
	usecase, err := New(store)
	require.NoError(t, err)
	policy, err := bizmodel.NewCommonRepCleanupPolicy(2, "")
	require.NoError(t, err)
	options, err := NewOptions(100, 4, 0, 10, time.Minute)
	require.NoError(t, err)
	return usecase, store, policy, options
}

func TestCommonRepPreviewDoesNotDelete(t *testing.T) {
	usecase, store, policy, options := commonRepTestRun(t, 3, 2)
	report, err := usecase.Preview(context.Background(), policy, options, uuid.Nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(5), report.PrunableRows)
	assert.True(t, report.PassComplete)
	assert.Equal(t, map[uuid.UUID]int64{store.roots[0]: 3, store.roots[1]: 2}, store.remaining)
}

func TestCommonRepExecuteUsesBoundedBatchesAndCountsDistinctRoots(t *testing.T) {
	usecase, store, policy, options := commonRepTestRun(t, 9, 2)
	var batches []int64
	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, func(_ int, impact bizmodel.CommonRepCleanupImpact) {
		batches = append(batches, impact.Representations())
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, []int64{4, 4, 3}, batches)
	assert.Equal(t, int64(11), result.DeletedRows)
	assert.Equal(t, int64(2), result.ImpactedRoots)
	assert.True(t, result.Report.PassComplete)
	assert.Equal(t, map[uuid.UUID]int64{store.roots[0]: 0, store.roots[1]: 0}, store.remaining)
}

func TestCommonRepBudgetStopResumesTheInterruptedChunk(t *testing.T) {
	usecase, store, policy, options := commonRepTestRun(t, 3, 3, 3)
	options.MaxBatches = 1
	first, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(4), first.DeletedRows)
	assert.True(t, first.BatchLimitReached)
	assert.False(t, first.Report.PassComplete)
	assert.Equal(t, uuid.Nil, first.Report.LastCursor)
	options.MaxBatches = 10
	second, err := usecase.Execute(context.Background(), policy, options, first.Report.LastCursor, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(5), second.DeletedRows)
	assert.True(t, second.Report.PassComplete)
	assert.Equal(t, int64(0), store.remaining[store.roots[2]])
}

func TestCommonRepFailedCommitDoesNotReportAttemptedDeletes(t *testing.T) {
	usecase, store, policy, options := commonRepTestRun(t, 3)
	store.commitError = errors.New("commit failed")
	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
	require.ErrorIs(t, err, store.commitError)
	assert.Zero(t, result.DeletedRows)
	assert.Zero(t, result.CommittedBatches)
	assert.False(t, result.Report.PassComplete)
}

func TestCommonRepCancellationReturnsCommittedProgress(t *testing.T) {
	usecase, _, policy, options := commonRepTestRun(t, 9)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := usecase.Execute(ctx, policy, options, uuid.Nil, func(_ int, _ bizmodel.CommonRepCleanupImpact) { cancel() }, nil)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int64(4), result.DeletedRows)
	assert.Equal(t, 1, result.CommittedBatches)
	assert.False(t, result.Report.PassComplete)
}

func TestCommonRepPreviewCancellationReturnsPartialReport(t *testing.T) {
	usecase, _, policy, options := commonRepTestRun(t, 9)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := usecase.Preview(ctx, policy, options, uuid.Nil, nil)
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, report.PassComplete)
	assert.False(t, report.FinishedAt.IsZero())
}

func TestCommonRepCleanupOptionsValidation(t *testing.T) {
	for _, options := range []Options{
		{ChunkRows: 0, BatchSize: 1, MaxBatches: 1, MaxDuration: time.Minute},
		{ChunkRows: 1, BatchSize: 0, MaxBatches: 1, MaxDuration: time.Minute},
		{ChunkRows: 1, BatchSize: 1, MaxBatches: 0, MaxDuration: time.Minute},
		{ChunkRows: 1, BatchSize: 1, MaxBatches: 1},
		{ChunkRows: 1, BatchSize: 1, MaxBatches: 1, MaxDuration: time.Minute, BatchDelay: -1},
	} {
		assert.Error(t, options.Validate())
	}
}
