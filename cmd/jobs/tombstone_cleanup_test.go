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

func TestNewTombstoneCleanupPolicyFreezesUTCAndUsesStrictCutoff(t *testing.T) {
	startedAt := time.Date(2026, 10, 9, 12, 30, 0, 0, time.FixedZone("trial", -5*60*60))
	policy, err := NewTombstoneCleanupPolicy(startedAt, 7, " HBI ")
	require.NoError(t, err)

	expectedRunAt := startedAt.UTC()
	expectedCutoff := expectedRunAt.Add(-168 * time.Hour)
	assert.Equal(t, expectedRunAt, policy.EvaluationTime())
	assert.Equal(t, expectedCutoff, policy.Cutoff())
	assert.True(t, policy.IsTombstoneEligible(expectedCutoff.Add(-time.Nanosecond)))
	assert.False(t, policy.IsTombstoneEligible(expectedCutoff), "equality is retained")
	assert.False(t, policy.IsTombstoneEligible(expectedCutoff.Add(time.Nanosecond)))
	assert.False(t, policy.IsTombstoneEligible(time.Time{}), "an absent or untrusted timestamp is not an age candidate")

	reporterType, filtered := policy.ReporterType()
	assert.True(t, filtered)
	assert.Equal(t, "hbi", reporterType.String())
}

func TestNewTombstoneCleanupPolicyAcceptsUnfilteredScope(t *testing.T) {
	startedAt := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	policy, err := NewTombstoneCleanupPolicy(startedAt, 1, "")
	require.NoError(t, err)

	_, filtered := policy.ReporterType()
	assert.False(t, filtered)
}

func TestNewTombstoneCleanupPolicyRejectsInvalidInputs(t *testing.T) {
	startedAt := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	for _, days := range []int{0, -1, math.MaxInt} {
		_, err := NewTombstoneCleanupPolicy(startedAt, days, "")
		assert.Error(t, err, "days=%d", days)
	}
	_, err := NewTombstoneCleanupPolicy(time.Time{}, 1, "")
	assert.Error(t, err)
	_, err = NewTombstoneCleanupPolicy(startedAt, 1, "   ")
	assert.Error(t, err)
	assert.Error(t, (TombstoneCleanupPolicy{}).Validate())
}

func TestTombstoneCleanupImpactAddsAffectedRowsWithoutOverflow(t *testing.T) {
	firstRootIDs := []bizmodel.ResourceId{newCleanupResourceID(t), newCleanupResourceID(t)}
	secondRootIDs := []bizmodel.ResourceId{newCleanupResourceID(t), newCleanupResourceID(t)}
	first, err := NewTombstoneCleanupImpactWithRootIDs(3, 3, 8, 4, firstRootIDs, 1)
	require.NoError(t, err)
	second, err := NewTombstoneCleanupImpactWithRootIDs(2, 2, 5, 3, secondRootIDs, 1)
	require.NoError(t, err)
	got, err := first.Add(second)
	require.NoError(t, err)
	want, err := NewTombstoneCleanupImpactWithRootIDs(5, 5, 13, 7, append(firstRootIDs, secondRootIDs...), 2)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = NewTombstoneCleanupImpact(0, 0, -1, 0, 0, 0)
	assert.Error(t, err)
	maximum, err := NewTombstoneCleanupImpact(0, 0, math.MaxInt64, 0, 0, 0)
	require.NoError(t, err)
	_, err = maximum.Add(first)
	assert.Error(t, err)
}

func newCleanupResourceID(t *testing.T) bizmodel.ResourceId {
	t.Helper()
	resourceID, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)
	return resourceID
}

func TestTombstoneCleanupImpactCountsDistinctRootsAcrossBatches(t *testing.T) {
	sharedRoot, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)
	secondRoot, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)

	first, err := NewTombstoneCleanupImpactWithRootIDs(1, 1, 2, 0, []bizmodel.ResourceId{sharedRoot}, 0)
	require.NoError(t, err)
	second, err := NewTombstoneCleanupImpactWithRootIDs(2, 2, 3, 0, []bizmodel.ResourceId{sharedRoot, secondRoot}, 1)
	require.NoError(t, err)

	combined, err := first.Add(second)
	require.NoError(t, err)
	assert.Equal(t, int64(3), combined.ReporterEntities())
	assert.Equal(t, int64(3), combined.ReporterResources())
	assert.Equal(t, int64(5), combined.ReporterRepresentations())
	assert.Equal(t, int64(2), combined.ResourceRootsImpacted(), "the shared root must be counted once")
	assert.Equal(t, int64(1), combined.ResourcesRemovable())
}

func TestTombstoneCleanupReportSeparatesEligibleAndBoundedImpact(t *testing.T) {
	evaluatedAt := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	cutoff := evaluatedAt.Add(-168 * time.Hour)
	total, err := NewTombstoneCleanupImpact(5, 5, 17, 9, 4, 2)
	require.NoError(t, err)
	bounded, err := NewTombstoneCleanupImpact(2, 2, 6, 3, 2, 1)
	require.NoError(t, err)
	exclusions, err := NewTombstoneCleanupExclusions(1, 0, 0, 1, 0, 0)
	require.NoError(t, err)
	snapshotAt := evaluatedAt.Add(2 * time.Second)
	report, err := NewTombstoneCleanupReport(evaluatedAt, snapshotAt, cutoff, 2, 7, 0, total, bounded, exclusions)
	require.NoError(t, err)

	assert.Equal(t, evaluatedAt, report.EvaluatedAt())
	assert.Equal(t, snapshotAt, report.SnapshotAt())
	assert.Equal(t, cutoff, report.Cutoff())
	assert.Equal(t, int64(2), report.BoundedRowLimit())
	assert.Equal(t, int64(7), report.AgeCandidates())
	assert.Zero(t, report.UnknownTimestampRows())
	assert.Equal(t, total, report.TotalEligible())
	assert.Equal(t, bounded, report.BoundedImpact())
	assert.Equal(t, exclusions, report.Exclusions())
}

func TestTombstoneCleanupReportRejectsInconsistentTotals(t *testing.T) {
	evaluatedAt := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	cutoff := evaluatedAt.Add(-168 * time.Hour)
	total, err := NewTombstoneCleanupImpact(2, 2, 4, 1, 2, 1)
	require.NoError(t, err)
	bounded, err := NewTombstoneCleanupImpact(3, 3, 4, 1, 2, 1)
	require.NoError(t, err)
	exclusions, err := NewTombstoneCleanupExclusions(0, 0, 0, 0, 0, 0)
	require.NoError(t, err)
	_, err = NewTombstoneCleanupReport(evaluatedAt, evaluatedAt, cutoff, 2, 2, 0, total, bounded, exclusions)
	assert.Error(t, err, "bounded impact cannot exceed total eligibility")
	_, err = NewTombstoneCleanupReport(evaluatedAt, evaluatedAt, cutoff, 2, 3, 0, total, total, exclusions)
	assert.Error(t, err, "age candidates must reconcile to eligible plus protected rows")
	_, err = NewTombstoneCleanupReport(evaluatedAt, evaluatedAt, cutoff, 2, 2, -1, total, total, exclusions)
	assert.Error(t, err, "unknown timestamp count cannot be negative")
}

type cleanupRepositoryFake struct {
	previewReport TombstoneCleanupReport
	previewError  error
	previewLimit  int64
	batches       []TombstoneCleanupImpact
	deleteCalls   int
	deleteError   error
}

func (repository *cleanupRepositoryFake) PreviewTombstones(_ context.Context, _ TombstoneCleanupPolicy, boundedReporterRowLimit int64) (TombstoneCleanupReport, error) {
	repository.previewLimit = boundedReporterRowLimit
	return repository.previewReport, repository.previewError
}

func (repository *cleanupRepositoryFake) DeleteTombstoneBatch(_ context.Context, _ TombstoneCleanupPolicy, _ int) (TombstoneCleanupImpact, error) {
	repository.deleteCalls++
	if repository.deleteError != nil {
		return TombstoneCleanupImpact{}, repository.deleteError
	}
	index := repository.deleteCalls - 1
	if index >= len(repository.batches) {
		return TombstoneCleanupImpact{}, nil
	}
	return repository.batches[index], nil
}

type cleanupTransactionFake struct {
	repository       TombstoneCleanupRepository
	readOnly         []bool
	operationNames   []string
	transactionError error
}

func (transaction *cleanupTransactionFake) WithinSerializableTombstoneCleanup(
	ctx context.Context,
	operationName string,
	readOnly bool,
	_, _ time.Duration,
	work func(TombstoneCleanupRepository) error,
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
	exclusions, err := NewTombstoneCleanupExclusions(1, 0, 0, 0, 0, 0)
	require.NoError(t, err)
	report, err := NewTombstoneCleanupReport(
		policy.EvaluationTime(), policy.EvaluationTime().Add(time.Second), policy.Cutoff(),
		2, 5, 0, impact, bounded, exclusions,
	)
	require.NoError(t, err)
	repository := &cleanupRepositoryFake{previewReport: report}
	transaction := &cleanupTransactionFake{repository: repository}
	usecase, err := newTombstoneCleanup(transaction)
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
	repository := &cleanupRepositoryFake{batches: []TombstoneCleanupImpact{first, second}}
	transaction := &cleanupTransactionFake{repository: repository}
	usecase, err := newTombstoneCleanup(transaction)
	require.NoError(t, err)
	options := cleanupOptions(t, 2, 0, 2, time.Minute)
	var committedCallbacks int

	result, err := usecase.Execute(context.Background(), policy, options, func(_ int, _ TombstoneCleanupImpact) {
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
	sharedRoot, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)
	first, err := NewTombstoneCleanupImpactWithRootIDs(1, 1, 2, 0, []bizmodel.ResourceId{sharedRoot}, 0)
	require.NoError(t, err)
	second, err := NewTombstoneCleanupImpactWithRootIDs(1, 1, 1, 2, []bizmodel.ResourceId{sharedRoot}, 1)
	require.NoError(t, err)
	repository := &cleanupRepositoryFake{batches: []TombstoneCleanupImpact{first, second}}
	transaction := &cleanupTransactionFake{repository: repository}
	usecase, err := newTombstoneCleanup(transaction)
	require.NoError(t, err)
	options := cleanupOptions(t, 1, 0, 2, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Impact().ResourceRootsImpacted())
	assert.Equal(t, int64(1), result.Impact().ResourcesRemovable())
}

func TestExecuteDoesNotCountBatchWhenTransactionReturnsError(t *testing.T) {
	policy := cleanupPolicy(t)
	repository := &cleanupRepositoryFake{batches: []TombstoneCleanupImpact{cleanupImpact(t, 1, 1, 2, 1, 1, 1)}}
	transaction := &cleanupTransactionFake{repository: repository, transactionError: errors.New("commit failed")}
	usecase, err := newTombstoneCleanup(transaction)
	require.NoError(t, err)
	options := cleanupOptions(t, 10, 0, 1, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, nil)
	require.Error(t, err)
	assert.Equal(t, TombstoneCleanupImpact{}, result.Impact())
	assert.Zero(t, result.CommittedBatches())
	assert.False(t, result.BatchLimitReached())
}

func TestExecuteCancellationBetweenBatchesReturnsCommittedProgress(t *testing.T) {
	policy := cleanupPolicy(t)
	first := cleanupImpact(t, 1, 1, 2, 1, 1, 1)
	repository := &cleanupRepositoryFake{batches: []TombstoneCleanupImpact{first, cleanupImpact(t, 1, 1, 2, 1, 1, 1)}}
	transaction := &cleanupTransactionFake{repository: repository}
	usecase, err := newTombstoneCleanup(transaction)
	require.NoError(t, err)
	options := cleanupOptions(t, 1, time.Hour, 3, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := usecase.Execute(ctx, policy, options, func(int, TombstoneCleanupImpact) { cancel() })
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
			_, err := newTombstoneCleanupOptions(test.batchSize, test.delay, test.batches, test.duration)
			assert.Error(t, err)
		})
	}
}

func cleanupPolicy(t *testing.T) TombstoneCleanupPolicy {
	t.Helper()
	policy, err := NewTombstoneCleanupPolicy(time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC), 7, " hbi ")
	require.NoError(t, err)
	return policy
}

func cleanupOptions(t *testing.T, batchSize int, delay time.Duration, batches int, duration time.Duration) tombstoneCleanupOptions {
	t.Helper()
	options, err := newTombstoneCleanupOptions(batchSize, delay, batches, duration)
	require.NoError(t, err)
	return options
}

func cleanupImpact(t *testing.T, entities, reporterRows, reporterHistory, commonHistory, roots, removedRoots int64) TombstoneCleanupImpact {
	t.Helper()
	rootIDs := make([]bizmodel.ResourceId, 0, roots)
	for rootIndex := int64(0); rootIndex < roots; rootIndex++ {
		rootID, err := bizmodel.NewResourceId(uuid.New())
		require.NoError(t, err)
		rootIDs = append(rootIDs, rootID)
	}
	impact, err := NewTombstoneCleanupImpactWithRootIDs(entities, reporterRows, reporterHistory, commonHistory, rootIDs, removedRoots)
	require.NoError(t, err)
	return impact
}
