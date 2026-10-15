package jobs

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCommonRepCleanupPolicyRequiresTwoNewestVersions(t *testing.T) {
	policy, err := newCommonRepCleanupPolicy(2, " host ")
	require.NoError(t, err)
	assert.Equal(t, 2, policy.KeepVersions())

	resourceType, filtered := policy.ResourceType()
	assert.True(t, filtered)
	assert.Equal(t, "host", resourceType.String())

	unfiltered, err := newCommonRepCleanupPolicy(5, "")
	require.NoError(t, err)
	_, filtered = unfiltered.ResourceType()
	assert.False(t, filtered)
	assert.Equal(t, 5, unfiltered.KeepVersions())
}

func TestNewCommonRepCleanupPolicyRejectsInvalidInputs(t *testing.T) {
	for _, keepVersions := range []int{0, 1, -1} {
		_, err := newCommonRepCleanupPolicy(keepVersions, "")
		assert.Error(t, err, "keepVersions=%d", keepVersions)
	}
	_, err := newCommonRepCleanupPolicy(2, "   ")
	assert.Error(t, err)
	assert.Error(t, (commonRepCleanupPolicy{}).Validate())
}

func TestCommonRepCleanupImpactAddsRowsAndUnionsRoots(t *testing.T) {
	sharedRoot, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)
	secondRoot, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)

	first, err := newCommonRepCleanupImpactWithRootIDs(3, []bizmodel.ResourceId{sharedRoot})
	require.NoError(t, err)
	second, err := newCommonRepCleanupImpactWithRootIDs(2, []bizmodel.ResourceId{sharedRoot, secondRoot})
	require.NoError(t, err)

	combined, err := first.Add(second)
	require.NoError(t, err)
	assert.Equal(t, int64(5), combined.Representations())
	assert.Equal(t, int64(2), combined.ResourceRootsImpacted(), "the shared root must be counted once")

	_, err = newCommonRepCleanupImpact(-1, 0)
	assert.Error(t, err)
	_, err = newCommonRepCleanupImpact(0, 1)
	assert.Error(t, err, "distinct roots cannot exceed rows")
	maximum, err := newCommonRepCleanupImpact(math.MaxInt64, 1)
	require.NoError(t, err)
	_, err = maximum.Add(first)
	assert.Error(t, err, "row count overflow")
}

func TestCommonRepCleanupReportSeparatesTotalAndBoundedImpact(t *testing.T) {
	evaluatedAt := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	total, err := newCommonRepCleanupImpact(17, 4)
	require.NoError(t, err)
	bounded, err := newCommonRepCleanupImpact(6, 2)
	require.NoError(t, err)
	report, err := newCommonRepCleanupReport(
		evaluatedAt, evaluatedAt.Add(2*time.Second), 2, 6,
		5, 3, 1, total, bounded,
	)
	require.NoError(t, err)

	assert.Equal(t, evaluatedAt, report.EvaluatedAt())
	assert.Equal(t, int64(6), report.BoundedRowLimit())
	assert.Equal(t, 2, report.KeepVersions())
	assert.Equal(t, int64(5), report.ScopedActiveRoots())
	assert.Equal(t, int64(3), report.RootsWithoutActiveReporter())
	assert.Equal(t, int64(1), report.ExcludedInconsistentRoots())
	assert.Equal(t, total, report.TotalPrunable())
	assert.Equal(t, bounded, report.BoundedPrunable())
}

func TestCommonRepCleanupReportRejectsInconsistentTotals(t *testing.T) {
	evaluatedAt := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	total, err := newCommonRepCleanupImpact(2, 1)
	require.NoError(t, err)
	exceeding, err := newCommonRepCleanupImpact(3, 1)
	require.NoError(t, err)
	_, err = newCommonRepCleanupReport(evaluatedAt, evaluatedAt, 2, 2, 1, 0, 0, total, exceeding)
	assert.Error(t, err, "bounded impact cannot exceed total prunability")
	_, err = newCommonRepCleanupReport(evaluatedAt, evaluatedAt, 1, 2, 1, 0, 0, total, total)
	assert.Error(t, err, "retention below two versions is rejected")
	_, err = newCommonRepCleanupReport(time.Time{}, evaluatedAt, 2, 2, 1, 0, 0, total, total)
	assert.Error(t, err, "evaluation time is required")
	_, err = newCommonRepCleanupReport(evaluatedAt, evaluatedAt, 2, 1, 1, 0, 0, total, total)
	assert.Error(t, err, "bounded rows cannot exceed the configured row limit")
}

type commonRepRepositoryFake struct {
	previewReport commonRepCleanupReport
	previewError  error
	previewLimit  int64
	batches       []commonRepCleanupImpact
	deleteCalls   int
	deleteError   error
}

func (repository *commonRepRepositoryFake) PreviewCommonHistory(_ context.Context, _ commonRepCleanupPolicy, boundedRowLimit int64) (commonRepCleanupReport, error) {
	repository.previewLimit = boundedRowLimit
	return repository.previewReport, repository.previewError
}

func (repository *commonRepRepositoryFake) DeleteCommonHistoryBatch(_ context.Context, _ commonRepCleanupPolicy, _ int) (commonRepCleanupImpact, error) {
	repository.deleteCalls++
	if repository.deleteError != nil {
		return commonRepCleanupImpact{}, repository.deleteError
	}
	index := repository.deleteCalls - 1
	if index >= len(repository.batches) {
		return commonRepCleanupImpact{}, nil
	}
	return repository.batches[index], nil
}

type commonRepTransactionFake struct {
	repository       commonRepCleanupRepository
	readOnly         []bool
	operationNames   []string
	transactionError error
}

func (transaction *commonRepTransactionFake) WithinSerializableCommonRepCleanup(
	ctx context.Context,
	operationName string,
	readOnly bool,
	_, _ time.Duration,
	work func(commonRepCleanupRepository) error,
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

func TestCommonRepPreviewUsesReadOnlyTransactionAndRowBudget(t *testing.T) {
	policy := commonRepPolicy(t)
	total, err := newCommonRepCleanupImpact(9, 3)
	require.NoError(t, err)
	bounded, err := newCommonRepCleanupImpact(2, 1)
	require.NoError(t, err)
	report, err := newCommonRepCleanupReport(
		time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 14, 12, 0, 1, 0, time.UTC),
		2, 2, 3, 0, 0, total, bounded,
	)
	require.NoError(t, err)
	repository := &commonRepRepositoryFake{previewReport: report}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 2, 0, 1, time.Minute)

	got, err := usecase.Preview(context.Background(), policy, options)
	require.NoError(t, err)
	assert.Equal(t, report, got)
	assert.Equal(t, int64(2), repository.previewLimit)
	assert.Equal(t, []bool{true}, transaction.readOnly)
	assert.Equal(t, []string{"common_rep_cleanup_preview"}, transaction.operationNames)
	assert.Zero(t, repository.deleteCalls)
}

func TestCommonRepExecuteAccumulatesCommittedBatchesAndStopsAtBudget(t *testing.T) {
	policy := commonRepPolicy(t)
	first := commonRepImpact(t, 2, 2)
	second := commonRepImpact(t, 2, 1)
	repository := &commonRepRepositoryFake{batches: []commonRepCleanupImpact{first, second}}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 2, 0, 2, time.Minute)
	var committedCallbacks int

	result, err := usecase.Execute(context.Background(), policy, options, func(_ int, _ commonRepCleanupImpact) {
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

func TestCommonRepExecuteCountsAResourceRootOnlyOnceAcrossBatches(t *testing.T) {
	policy := commonRepPolicy(t)
	sharedRoot, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)
	first, err := newCommonRepCleanupImpactWithRootIDs(2, []bizmodel.ResourceId{sharedRoot})
	require.NoError(t, err)
	second, err := newCommonRepCleanupImpactWithRootIDs(1, []bizmodel.ResourceId{sharedRoot})
	require.NoError(t, err)
	repository := &commonRepRepositoryFake{batches: []commonRepCleanupImpact{first, second}}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 2, 0, 2, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), result.Impact().Representations())
	assert.Equal(t, int64(1), result.Impact().ResourceRootsImpacted())
}

func TestCommonRepExecuteDoesNotCountBatchWhenTransactionReturnsError(t *testing.T) {
	policy := commonRepPolicy(t)
	repository := &commonRepRepositoryFake{batches: []commonRepCleanupImpact{commonRepImpact(t, 1, 1)}}
	transaction := &commonRepTransactionFake{repository: repository, transactionError: errors.New("commit failed")}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 10, 0, 1, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, nil)
	require.Error(t, err)
	assert.Equal(t, commonRepCleanupImpact{}, result.Impact())
	assert.Zero(t, result.CommittedBatches())
	assert.False(t, result.BatchLimitReached())
}

func TestCommonRepExecuteCancellationBetweenBatchesReturnsCommittedProgress(t *testing.T) {
	policy := commonRepPolicy(t)
	first := commonRepImpact(t, 1, 1)
	repository := &commonRepRepositoryFake{batches: []commonRepCleanupImpact{first, commonRepImpact(t, 1, 1)}}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 1, time.Hour, 3, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := usecase.Execute(ctx, policy, options, func(int, commonRepCleanupImpact) { cancel() })
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, first, result.Impact())
	assert.Equal(t, 1, result.CommittedBatches())
	assert.Equal(t, 1, repository.deleteCalls)
}

func TestNewCommonRepCleanupOptionsRejectsInvalidOrOverflowingLimits(t *testing.T) {
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
			_, err := newCommonRepCleanupOptions(test.batchSize, test.delay, test.batches, test.duration)
			assert.Error(t, err)
		})
	}
}

func commonRepPolicy(t *testing.T) commonRepCleanupPolicy {
	t.Helper()
	policy, err := newCommonRepCleanupPolicy(2, " host ")
	require.NoError(t, err)
	return policy
}

func commonRepOptions(t *testing.T, batchSize int, delay time.Duration, batches int, duration time.Duration) commonRepCleanupOptions {
	t.Helper()
	options, err := newCommonRepCleanupOptions(batchSize, delay, batches, duration)
	require.NoError(t, err)
	return options
}

func commonRepImpact(t *testing.T, representations, roots int64) commonRepCleanupImpact {
	t.Helper()
	rootIDs := make([]bizmodel.ResourceId, 0, roots)
	for rootIndex := int64(0); rootIndex < roots; rootIndex++ {
		rootID, err := bizmodel.NewResourceId(uuid.New())
		require.NoError(t, err)
		rootIDs = append(rootIDs, rootID)
	}
	impact, err := newCommonRepCleanupImpactWithRootIDs(representations, rootIDs)
	require.NoError(t, err)
	return impact
}
