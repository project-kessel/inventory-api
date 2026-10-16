package jobs

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

func TestNewCommonRepRootClassificationRejectsInvalidRows(t *testing.T) {
	_, err := newCommonRepRootClassification(uuid.Nil, "", 1)
	assert.Error(t, err, "a nil resource id is not a root")

	_, err = newCommonRepRootClassification(uuid.New(), "unknown", 0)
	assert.Error(t, err, "unknown exclusion labels are rejected")

	_, err = newCommonRepRootClassification(uuid.New(), commonRepExclusionInconsistentPointer, 2)
	assert.Error(t, err, "an excluded root cannot report prunable rows")

	_, err = newCommonRepRootClassification(uuid.New(), "", -1)
	assert.Error(t, err, "prunable rows cannot be negative")

	classification, err := newCommonRepRootClassification(uuid.New(), "", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(3), classification.PrunableRows())
	assert.Equal(t, commonRepExclusionNone, classification.Exclusion())

	classification, err = newCommonRepRootClassification(uuid.New(), commonRepExclusionNoActiveReporter, 0)
	require.NoError(t, err)
	assert.Equal(t, commonRepExclusionNoActiveReporter, classification.Exclusion())
}

func TestCommonRepCumulativeImpactCountsRowsAndDistinctRoots(t *testing.T) {
	sharedRoot, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)
	secondRoot, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)

	cumulative := newCommonRepCumulativeImpact()
	first, err := newCommonRepCleanupImpactWithRootIDs(3, []bizmodel.ResourceId{sharedRoot})
	require.NoError(t, err)
	second, err := newCommonRepCleanupImpactWithRootIDs(2, []bizmodel.ResourceId{sharedRoot, secondRoot})
	require.NoError(t, err)
	require.NoError(t, cumulative.record(first))
	require.NoError(t, cumulative.record(second))

	combined := cumulative.impact()
	assert.Equal(t, int64(5), combined.Representations())
	assert.Equal(t, int64(2), combined.ResourceRootsImpacted(), "the shared root must be counted once")
	assert.True(t, combined.rootIDsKnown)
}

func TestCommonRepCumulativeImpactRejectsInvalidBatches(t *testing.T) {
	sharedRoot, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)
	knownRoots, err := newCommonRepCleanupImpactWithRootIDs(2, []bizmodel.ResourceId{sharedRoot})
	require.NoError(t, err)

	// Unknown root identities with a nonzero distinct-root count must be rejected
	// rather than silently dropping roots from the cumulative set.
	unknownRoots, err := newCommonRepCleanupImpact(2, 1)
	require.NoError(t, err)
	cumulative := newCommonRepCumulativeImpact()
	require.NoError(t, cumulative.record(knownRoots))
	err = cumulative.record(unknownRoots)
	require.ErrorContains(t, err, "without root identities")

	// Zero rows with root identities is inconsistent and must be rejected.
	_, err = newCommonRepCleanupImpactWithRootIDs(0, []bizmodel.ResourceId{sharedRoot})
	assert.Error(t, err, "roots cannot exceed rows when no rows were deleted")
}

func TestCommonRepCumulativeImpactOverflowsCheckedArithmetic(t *testing.T) {
	maximumRoot, err := bizmodel.NewResourceId(uuid.New())
	require.NoError(t, err)
	// Known root identities are required so aggregation reaches the checked
	// addition rather than stopping at the unknown-identities guard.
	maximum, err := newCommonRepCleanupImpactWithRootIDs(math.MaxInt64, []bizmodel.ResourceId{maximumRoot})
	require.NoError(t, err)
	increment, err := newCommonRepCleanupImpactWithRootIDs(1, []bizmodel.ResourceId{maximumRoot})
	require.NoError(t, err)

	cumulative := newCommonRepCumulativeImpact()
	require.NoError(t, cumulative.record(maximum))
	err = cumulative.record(increment)
	require.ErrorContains(t, err, "overflow", "row count overflow must be the reported failure")
}

func TestAssembleCommonRepDeleteGroupsPacksRootsAndIsolatesDeepRoots(t *testing.T) {
	smallOne := commonRepPrunableRoot{resourceID: uuid.New(), plannedRows: 2}
	smallTwo := commonRepPrunableRoot{resourceID: uuid.New(), plannedRows: 1}
	smallThree := commonRepPrunableRoot{resourceID: uuid.New(), plannedRows: 2}
	deepRoot := commonRepPrunableRoot{resourceID: uuid.New(), plannedRows: 9}
	exactRoot := commonRepPrunableRoot{resourceID: uuid.New(), plannedRows: 4}

	groups := assembleCommonRepDeleteGroups([]commonRepPrunableRoot{smallOne, smallTwo, deepRoot, smallThree, exactRoot}, 4)

	require.Len(t, groups, 3)
	// smallOne + smallTwo sit together when the deep root flushes them.
	assert.False(t, groups[0].deep)
	assert.Equal(t, []commonRepPrunableRoot{smallOne, smallTwo}, groups[0].roots)
	// The deep root exceeds the batch size alone and is isolated.
	assert.True(t, groups[1].deep)
	assert.Equal(t, []commonRepPrunableRoot{deepRoot}, groups[1].roots)
	// A root exactly at the batch size joins the pending group instead of going deep.
	assert.False(t, groups[2].deep)
	assert.Equal(t, []commonRepPrunableRoot{smallThree, exactRoot}, groups[2].roots)
}

func TestCommonRepCleanupReportValidatesScanWindowAndTotals(t *testing.T) {
	startedAt := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	totals := commonRepScanTotals{
		chunksProcessed: 2, rootsScanned: 10, prunableRoots: 4, prunableRows: 9,
		rootsWithoutActiveReporter: 5, excludedInconsistentRoots: 1,
	}
	report, err := newCommonRepCleanupReport(startedAt, startedAt.Add(time.Second), 2, totals, uuid.New(), true)
	require.NoError(t, err)
	assert.Equal(t, int64(2), report.ChunksProcessed())
	assert.Equal(t, int64(10), report.RootsScanned())
	assert.Equal(t, int64(4), report.PrunableRoots())
	assert.Equal(t, int64(9), report.PrunableRows())
	assert.Equal(t, int64(5), report.RootsWithoutActiveReporter())
	assert.Equal(t, int64(1), report.ExcludedInconsistentRoots())
	assert.True(t, report.PassComplete())

	_, err = newCommonRepCleanupReport(time.Time{}, startedAt, 2, totals, uuid.Nil, true)
	assert.Error(t, err, "scan window must have a start")
	_, err = newCommonRepCleanupReport(startedAt, startedAt, 1, totals, uuid.Nil, true)
	assert.Error(t, err, "retention below two versions is rejected")
	_, err = newCommonRepCleanupReport(startedAt, startedAt.Add(-time.Second), 2, totals, uuid.Nil, false)
	assert.Error(t, err, "finished before started is rejected")
	inconsistent := totals
	inconsistent.prunableRoots = 11
	_, err = newCommonRepCleanupReport(startedAt, startedAt.Add(time.Second), 2, inconsistent, uuid.Nil, false)
	assert.Error(t, err, "classified roots cannot exceed scanned roots")
	negative := totals
	negative.prunableRows = -1
	_, err = newCommonRepCleanupReport(startedAt, startedAt.Add(time.Second), 2, negative, uuid.Nil, false)
	assert.Error(t, err, "counts cannot be negative")
}

// commonRepChunkFixture is one planned chunk served by the repository fake.
type commonRepChunkFixture struct {
	classifications []commonRepRootClassification
	boundary        uuid.UUID
	found           bool
}

type commonRepRepositoryFake struct {
	chunks     []commonRepChunkFixture
	chunkCalls int

	deleteRootsImpacts []commonRepCleanupImpact
	deleteRootsCalls   int
	deleteRootsError   error
	deletedRootIDs     [][]uuid.UUID

	deepDeletedRows []int64
	deepCalls       int
	deepError       error
}

func (repository *commonRepRepositoryFake) ReadChunk(_ context.Context, _ commonRepCleanupPolicy, _ uuid.UUID, _ int) ([]commonRepRootClassification, uuid.UUID, bool, error) {
	index := repository.chunkCalls
	repository.chunkCalls++
	if index >= len(repository.chunks) {
		return nil, uuid.Nil, false, nil
	}
	chunk := repository.chunks[index]
	return chunk.classifications, chunk.boundary, chunk.found, nil
}

func (repository *commonRepRepositoryFake) DeletePrunableRoots(_ context.Context, _ commonRepCleanupPolicy, resourceIDs []uuid.UUID) (commonRepCleanupImpact, error) {
	repository.deleteRootsCalls++
	repository.deletedRootIDs = append(repository.deletedRootIDs, append([]uuid.UUID(nil), resourceIDs...))
	if repository.deleteRootsError != nil {
		return commonRepCleanupImpact{}, repository.deleteRootsError
	}
	index := repository.deleteRootsCalls - 1
	if index >= len(repository.deleteRootsImpacts) {
		return newCommonRepCleanupImpactWithRootIDs(0, nil)
	}
	return repository.deleteRootsImpacts[index], nil
}

func (repository *commonRepRepositoryFake) DeleteLowestPrunableVersions(_ context.Context, _ commonRepCleanupPolicy, _ uuid.UUID, _ int64) (int64, error) {
	repository.deepCalls++
	if repository.deepError != nil {
		return 0, repository.deepError
	}
	index := repository.deepCalls - 1
	if index >= len(repository.deepDeletedRows) {
		return 0, nil
	}
	return repository.deepDeletedRows[index], nil
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

func TestCommonRepPreviewScansAllChunksReadOnlyAndAggregatesTotals(t *testing.T) {
	policy := commonRepPolicy(t)
	firstRoot := commonRepClassification(t, uuid.New(), "", 3)
	secondRoot := commonRepClassification(t, uuid.New(), commonRepExclusionNoActiveReporter, 0)
	thirdRoot := commonRepClassification(t, uuid.New(), commonRepExclusionInconsistentPointer, 0)
	repository := &commonRepRepositoryFake{chunks: []commonRepChunkFixture{
		{classifications: []commonRepRootClassification{firstRoot, secondRoot}, boundary: firstRoot.ResourceID(), found: true},
		{classifications: []commonRepRootClassification{thirdRoot}, boundary: thirdRoot.ResourceID(), found: false},
	}}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 2, 1, time.Minute)
	var chunkCursors []uuid.UUID

	report, err := usecase.Preview(context.Background(), policy, options, uuid.Nil, func(progress commonRepChunkProgress) {
		chunkCursors = append(chunkCursors, progress.cursor)
	})
	require.NoError(t, err)

	assert.Equal(t, int64(2), report.ChunksProcessed())
	assert.Equal(t, int64(3), report.RootsScanned())
	assert.Equal(t, int64(1), report.PrunableRoots())
	assert.Equal(t, int64(3), report.PrunableRows())
	assert.Equal(t, int64(1), report.RootsWithoutActiveReporter())
	assert.Equal(t, int64(1), report.ExcludedInconsistentRoots())
	assert.True(t, report.PassComplete())
	// The final chunk reports no new boundary, so the cursor holds the last processed one.
	assert.Equal(t, []uuid.UUID{firstRoot.ResourceID(), firstRoot.ResourceID()}, chunkCursors)
	assert.Equal(t, []bool{true, true}, transaction.readOnly, "preview transactions must be read-only")
	assert.Equal(t, []string{"common_rep_cleanup_preview_chunk", "common_rep_cleanup_preview_chunk"}, transaction.operationNames)
	assert.Zero(t, repository.deleteRootsCalls)
	assert.Zero(t, repository.deepCalls)
}

func TestCommonRepPreviewStopsAndReportsPartialProgressWhenCanceled(t *testing.T) {
	policy := commonRepPolicy(t)
	firstRoot := commonRepClassification(t, uuid.New(), "", 3)
	repository := &commonRepRepositoryFake{chunks: []commonRepChunkFixture{
		{classifications: []commonRepRootClassification{firstRoot}, boundary: firstRoot.ResourceID(), found: true},
		{classifications: nil, boundary: uuid.Nil, found: false},
	}}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 2, 1, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	report, err := usecase.Preview(ctx, policy, options, uuid.Nil, func(progress commonRepChunkProgress) {
		cancel()
	})
	require.NoError(t, err, "a canceled preview reports partial progress rather than failing")
	assert.False(t, report.PassComplete())
	assert.Equal(t, int64(1), report.ChunksProcessed())
	assert.Equal(t, firstRoot.ResourceID(), report.LastCursor())
}

func TestCommonRepPreviewResumesAfterStartCursor(t *testing.T) {
	policy := commonRepPolicy(t)
	root := commonRepClassification(t, uuid.New(), "", 2)
	repository := &commonRepRepositoryFake{chunks: []commonRepChunkFixture{
		{classifications: []commonRepRootClassification{root}, boundary: root.ResourceID(), found: false},
	}}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 2, 1, time.Minute)

	resumeCursor := uuid.New()
	report, err := usecase.Preview(context.Background(), policy, options, resumeCursor, nil)
	require.NoError(t, err)
	assert.Equal(t, resumeCursor, report.LastCursor(), "a completed pass reports the final chunk boundary")
}

func TestCommonRepExecuteCommitsGroupedBatchesAndCompletesThePass(t *testing.T) {
	policy := commonRepPolicy(t)
	firstRoot := commonRepClassification(t, uuid.New(), "", 2)
	secondRoot := commonRepClassification(t, uuid.New(), "", 2)
	repository := &commonRepRepositoryFake{
		chunks: []commonRepChunkFixture{
			{classifications: []commonRepRootClassification{firstRoot, secondRoot}, boundary: secondRoot.ResourceID(), found: true},
			{classifications: nil, boundary: uuid.Nil, found: false},
		},
		deleteRootsImpacts: []commonRepCleanupImpact{
			commonRepImpact(t, 2, 1),
			commonRepImpact(t, 2, 1),
		},
	}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 2, 5, time.Minute)
	var committedCallbacks int

	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, func(_ int, _ commonRepCleanupImpact) {
		committedCallbacks++
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(4), result.Impact().Representations())
	assert.Equal(t, int64(2), result.Impact().ResourceRootsImpacted())
	assert.Equal(t, 2, result.CommittedBatches())
	assert.False(t, result.BatchLimitReached())
	assert.True(t, result.Report().PassComplete())
	assert.Empty(t, result.ParkedRoots())
	assert.Equal(t, 2, committedCallbacks)
	assert.Equal(t, [][]uuid.UUID{{firstRoot.ResourceID()}, {secondRoot.ResourceID()}}, repository.deletedRootIDs)
	assert.Equal(t, []bool{true, false, false, true}, transaction.readOnly, "chunk scans are read-only and deletions are not")
}

func TestCommonRepExecuteStopsAtBatchBudgetAndReportsIncompletePass(t *testing.T) {
	policy := commonRepPolicy(t)
	firstRoot := commonRepClassification(t, uuid.New(), "", 2)
	secondRoot := commonRepClassification(t, uuid.New(), "", 2)
	repository := &commonRepRepositoryFake{
		chunks: []commonRepChunkFixture{
			{classifications: []commonRepRootClassification{firstRoot, secondRoot}, boundary: secondRoot.ResourceID(), found: true},
			{classifications: nil, boundary: uuid.Nil, found: false},
		},
		deleteRootsImpacts: []commonRepCleanupImpact{commonRepImpact(t, 2, 1)},
	}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 2, 1, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.CommittedBatches())
	assert.True(t, result.BatchLimitReached())
	assert.False(t, result.Report().PassComplete(), "a budget stop leaves the pass incomplete")
	assert.Equal(t, int64(2), result.Impact().Representations())
}

func TestCommonRepExecutePrunesDeepRootsInBoundedLoops(t *testing.T) {
	policy := commonRepPolicy(t)
	deepRoot := commonRepClassification(t, uuid.New(), "", 5)
	repository := &commonRepRepositoryFake{
		chunks: []commonRepChunkFixture{
			{classifications: []commonRepRootClassification{deepRoot}, boundary: deepRoot.ResourceID(), found: true},
			{classifications: nil, boundary: uuid.Nil, found: false},
		},
		deepDeletedRows: []int64{2, 2, 1, 0},
	}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 2, 10, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, result.CommittedBatches(), "five planned rows at two per statement commit three batches")
	assert.Equal(t, int64(5), result.Impact().Representations())
	assert.Equal(t, int64(1), result.Impact().ResourceRootsImpacted())
	assert.Zero(t, repository.deleteRootsCalls, "a deep root never enters the grouped path")
	assert.True(t, result.Report().PassComplete())
}

func TestCommonRepExecuteParksGroupRootsAfterSerializationExhaustion(t *testing.T) {
	policy := commonRepPolicy(t)
	firstRoot := commonRepClassification(t, uuid.New(), "", 2)
	secondRoot := commonRepClassification(t, uuid.New(), "", 2)
	repository := &commonRepRepositoryFake{
		chunks: []commonRepChunkFixture{
			{classifications: []commonRepRootClassification{firstRoot, secondRoot}, boundary: secondRoot.ResourceID(), found: true},
			{classifications: nil, boundary: uuid.Nil, found: false},
		},
		deleteRootsImpacts: []commonRepCleanupImpact{commonRepImpact(t, 2, 1)},
	}
	transaction := &commonRepTransactionFake{repository: &commonRepSelectiveDeleteFake{inner: repository, failRoot: secondRoot.ResourceID()}}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 2, 5, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
	require.NoError(t, err, "serialization-exhausted roots are parked, not fatal")
	assert.Equal(t, 1, result.CommittedBatches())
	assert.Equal(t, int64(2), result.Impact().Representations())
	assert.Equal(t, []uuid.UUID{secondRoot.ResourceID()}, result.ParkedRoots())
	assert.True(t, result.Report().PassComplete(), "the scan still completes")
}

// commonRepSelectiveDeleteFake fails DeletePrunableRoots with an exhausted
// serialization error whenever the requested group contains the fail root.
type commonRepSelectiveDeleteFake struct {
	inner    *commonRepRepositoryFake
	failRoot uuid.UUID
	failures int
}

func (repository *commonRepSelectiveDeleteFake) ReadChunk(ctx context.Context, policy commonRepCleanupPolicy, after uuid.UUID, chunkRows int) ([]commonRepRootClassification, uuid.UUID, bool, error) {
	return repository.inner.ReadChunk(ctx, policy, after, chunkRows)
}

func (repository *commonRepSelectiveDeleteFake) DeletePrunableRoots(ctx context.Context, policy commonRepCleanupPolicy, resourceIDs []uuid.UUID) (commonRepCleanupImpact, error) {
	for _, resourceID := range resourceIDs {
		if resourceID == repository.failRoot {
			repository.failures++
			return commonRepCleanupImpact{}, fmt.Errorf("transaction failed after 5 attempts: %w", &pgconn.PgError{
				Code:    "40001",
				Message: "could not serialize access due to concurrent update",
			})
		}
	}
	return repository.inner.DeletePrunableRoots(ctx, policy, resourceIDs)
}

func (repository *commonRepSelectiveDeleteFake) DeleteLowestPrunableVersions(ctx context.Context, policy commonRepCleanupPolicy, resourceID uuid.UUID, rowLimit int64) (int64, error) {
	return repository.inner.DeleteLowestPrunableVersions(ctx, policy, resourceID, rowLimit)
}

func TestCommonRepExecuteParksDeepRootsAfterSerializationExhaustion(t *testing.T) {
	policy := commonRepPolicy(t)
	deepRoot := commonRepClassification(t, uuid.New(), "", 5)
	repository := &commonRepRepositoryFake{
		chunks: []commonRepChunkFixture{
			{classifications: []commonRepRootClassification{deepRoot}, boundary: deepRoot.ResourceID(), found: true},
			{classifications: nil, boundary: uuid.Nil, found: false},
		},
	}
	transaction := &commonRepTransactionFake{repository: &commonRepSelectiveDeepFake{inner: repository}}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 2, 10, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{deepRoot.ResourceID()}, result.ParkedRoots())
	assert.Zero(t, result.CommittedBatches())
}

// commonRepSelectiveDeepFake fails deep-root deletion with an exhausted
// serialization error.
type commonRepSelectiveDeepFake struct {
	inner *commonRepRepositoryFake
}

func (repository *commonRepSelectiveDeepFake) ReadChunk(ctx context.Context, policy commonRepCleanupPolicy, after uuid.UUID, chunkRows int) ([]commonRepRootClassification, uuid.UUID, bool, error) {
	return repository.inner.ReadChunk(ctx, policy, after, chunkRows)
}

func (repository *commonRepSelectiveDeepFake) DeletePrunableRoots(ctx context.Context, policy commonRepCleanupPolicy, resourceIDs []uuid.UUID) (commonRepCleanupImpact, error) {
	return repository.inner.DeletePrunableRoots(ctx, policy, resourceIDs)
}

func (repository *commonRepSelectiveDeepFake) DeleteLowestPrunableVersions(_ context.Context, _ commonRepCleanupPolicy, _ uuid.UUID, _ int64) (int64, error) {
	return 0, fmt.Errorf("transaction failed after 5 attempts: %w", &pgconn.PgError{
		Code:    "40001",
		Message: "could not serialize access due to concurrent update",
	})
}

func TestCommonRepExecuteCountsAResourceRootOnlyOnceAcrossDeepBatches(t *testing.T) {
	policy := commonRepPolicy(t)
	sharedRoot := commonRepClassification(t, uuid.New(), "", 4)
	repository := &commonRepRepositoryFake{
		chunks: []commonRepChunkFixture{
			{classifications: []commonRepRootClassification{sharedRoot}, boundary: sharedRoot.ResourceID(), found: true},
			{classifications: nil, boundary: uuid.Nil, found: false},
		},
		deepDeletedRows: []int64{2, 2, 0},
	}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 2, 5, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, result.CommittedBatches())
	assert.Equal(t, int64(4), result.Impact().Representations())
	assert.Equal(t, int64(1), result.Impact().ResourceRootsImpacted(), "one deep-loop root is counted once across its batches")
}

func TestCommonRepExecuteRetryOverwritesAttemptedImpactAndNotifiesOncePerCommit(t *testing.T) {
	policy := commonRepPolicy(t)
	root := commonRepClassification(t, uuid.New(), "", 3)
	repository := &commonRepRepositoryFake{
		chunks: []commonRepChunkFixture{
			{classifications: []commonRepRootClassification{root}, boundary: root.ResourceID(), found: true},
			{classifications: nil, boundary: uuid.Nil, found: false},
		},
	}
	transaction := &commonRepRetryingTransactionFake{repository: repository, attempts: []commonRepCleanupImpact{commonRepImpact(t, 2, 1), commonRepImpact(t, 3, 1)}}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 10, 5, time.Minute)
	var committedCallbacks int

	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, func(_ int, _ commonRepCleanupImpact) {
		committedCallbacks++
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, transaction.workInvocations, "the transaction adapter must be able to re-invoke work after a serialization retry")
	assert.Equal(t, 1, repository.deleteRootsCalls, "attempted deletions are not re-counted across retries")
	assert.Equal(t, int64(3), result.Impact().Representations(), "only the retried attempt's counts may contribute")
	assert.Equal(t, 1, result.CommittedBatches())
	assert.Equal(t, 1, committedCallbacks, "progress notifications run only after the commit succeeds")
}

// commonRepRetryingTransactionFake simulates the transaction adapter invoking the
// work callback twice within one transaction after a serialization failure.
type commonRepRetryingTransactionFake struct {
	repository      *commonRepRepositoryFake
	attempts        []commonRepCleanupImpact
	workInvocations int
}

func (transaction *commonRepRetryingTransactionFake) WithinSerializableCommonRepCleanup(
	ctx context.Context,
	operationName string,
	readOnly bool,
	_, _ time.Duration,
	work func(commonRepCleanupRepository) error,
) error {
	if !readOnly {
		for _, attempt := range transaction.attempts {
			transaction.workInvocations++
			transaction.repository.deleteRootsImpacts = []commonRepCleanupImpact{attempt}
			transaction.repository.deleteRootsCalls = 0
			if err := work(transaction.repository); err != nil {
				return err
			}
		}
	} else if err := work(transaction.repository); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func TestCommonRepExecuteDoesNotCountBatchWhenTransactionReturnsError(t *testing.T) {
	policy := commonRepPolicy(t)
	root := commonRepClassification(t, uuid.New(), "", 2)
	repository := &commonRepRepositoryFake{
		chunks: []commonRepChunkFixture{
			{classifications: []commonRepRootClassification{root}, boundary: root.ResourceID(), found: true},
			{classifications: nil, boundary: uuid.Nil, found: false},
		},
		deleteRootsImpacts: []commonRepCleanupImpact{commonRepImpact(t, 2, 1)},
	}
	transaction := &commonRepTransactionFake{repository: repository, transactionError: errors.New("commit failed")}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 10, 5, time.Minute)

	result, err := usecase.Execute(context.Background(), policy, options, uuid.Nil, nil, nil)
	require.Error(t, err)
	assert.Zero(t, result.Impact().Representations())
	assert.Zero(t, result.CommittedBatches())
	assert.False(t, result.BatchLimitReached())
}

func TestCommonRepExecuteCancellationBetweenBatchesReturnsCommittedProgress(t *testing.T) {
	policy := commonRepPolicy(t)
	firstRoot := commonRepClassification(t, uuid.New(), "", 2)
	secondRoot := commonRepClassification(t, uuid.New(), "", 2)
	repository := &commonRepRepositoryFake{
		chunks: []commonRepChunkFixture{
			{classifications: []commonRepRootClassification{firstRoot, secondRoot}, boundary: secondRoot.ResourceID(), found: true},
			{classifications: nil, boundary: uuid.Nil, found: false},
		},
		deleteRootsImpacts: []commonRepCleanupImpact{commonRepImpact(t, 2, 1), commonRepImpact(t, 2, 1)},
	}
	transaction := &commonRepTransactionFake{repository: repository}
	usecase, err := newCommonRepCleanup(transaction)
	require.NoError(t, err)
	options := commonRepOptions(t, 100, 2, 3, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := usecase.Execute(ctx, policy, options, uuid.Nil, func(_ int, _ commonRepCleanupImpact) { cancel() }, nil)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int64(2), result.Impact().Representations())
	assert.Equal(t, 1, result.CommittedBatches())
	assert.Equal(t, 1, repository.deleteRootsCalls)
}

func TestNewCommonRepCleanupOptionsRejectsInvalidLimits(t *testing.T) {
	for _, test := range []struct {
		name      string
		chunkRows int
		batchSize int
		delay     time.Duration
		batches   int
		duration  time.Duration
	}{
		{name: "zero chunk rows", chunkRows: 0, batchSize: 1, batches: 1, duration: time.Minute},
		{name: "negative chunk rows", chunkRows: -1, batchSize: 1, batches: 1, duration: time.Minute},
		{name: "zero batch size", chunkRows: 1, batchSize: 0, batches: 1, duration: time.Minute},
		{name: "negative delay", chunkRows: 1, batchSize: 1, delay: -time.Second, batches: 1, duration: time.Minute},
		{name: "zero batches", chunkRows: 1, batchSize: 1, batches: 0, duration: time.Minute},
		{name: "zero duration", chunkRows: 1, batchSize: 1, batches: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := newCommonRepCleanupOptions(test.chunkRows, test.batchSize, test.delay, test.batches, test.duration)
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

func commonRepOptions(t *testing.T, chunkRows int, batchSize int, batches int, duration time.Duration) commonRepCleanupOptions {
	t.Helper()
	options, err := newCommonRepCleanupOptions(chunkRows, batchSize, 0, batches, duration)
	require.NoError(t, err)
	return options
}

func commonRepClassification(t *testing.T, resourceID uuid.UUID, exclusion string, prunableRows int64) commonRepRootClassification {
	t.Helper()
	classification, err := newCommonRepRootClassification(resourceID, exclusion, prunableRows)
	require.NoError(t, err)
	return classification
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
