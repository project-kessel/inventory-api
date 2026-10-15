package jobs

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

// commonRepCleanupPolicy freezes the per-resource retention rule for one invocation.
// The rule keeps the authoritative current common representation and its keepVersions-1
// predecessors for every resource that still has an active (non-tombstoned) reporter.
type commonRepCleanupPolicy struct {
	keepVersions int
	resourceType bizmodel.ResourceType
	hasType      bool
	valid        bool
}

// newCommonRepCleanupPolicy constructs a per-resource retention policy. keepVersions
// must be at least 2 because update consumption reads event version v and v-1.
func newCommonRepCleanupPolicy(keepVersions int, resourceType string) (commonRepCleanupPolicy, error) {
	if keepVersions < 2 {
		return commonRepCleanupPolicy{}, fmt.Errorf("common representation retention must keep at least the current version and its predecessor")
	}
	policy := commonRepCleanupPolicy{keepVersions: keepVersions, valid: true}
	if strings.TrimSpace(resourceType) != "" {
		validatedResourceType, err := bizmodel.NewResourceType(resourceType)
		if err != nil {
			return commonRepCleanupPolicy{}, fmt.Errorf("invalid resource type filter: %w", err)
		}
		policy.resourceType = validatedResourceType
		policy.hasType = true
	} else if resourceType != "" {
		return commonRepCleanupPolicy{}, fmt.Errorf("resource type filter cannot contain only whitespace")
	}
	return policy, nil
}

// Validate rejects the zero value, which is not a constructed cleanup policy.
func (policy commonRepCleanupPolicy) Validate() error {
	if !policy.valid {
		return fmt.Errorf("common representation cleanup policy is not initialized")
	}
	return nil
}

// KeepVersions returns the number of newest common representations retained per resource.
func (policy commonRepCleanupPolicy) KeepVersions() int { return policy.keepVersions }

// ResourceType returns the normalized resource type filter and whether a filter was selected.
func (policy commonRepCleanupPolicy) ResourceType() (bizmodel.ResourceType, bool) {
	return policy.resourceType, policy.hasType
}

// commonRepCleanupImpact contains deleted common-representation rows and impacted roots.
type commonRepCleanupImpact struct {
	representations int64
	resourceRootIDs []bizmodel.ResourceId
	rootIDsKnown    bool
	distinctRoots   int64
}

// newCommonRepCleanupImpact constructs non-negative impact counts from aggregate counts.
// Root identities are unknown; distinctRoots must come from COUNT(DISTINCT resource_id).
func newCommonRepCleanupImpact(representations, distinctRoots int64) (commonRepCleanupImpact, error) {
	if representations < 0 || distinctRoots < 0 {
		return commonRepCleanupImpact{}, fmt.Errorf("common representation cleanup impact cannot be negative")
	}
	if distinctRoots > representations {
		return commonRepCleanupImpact{}, fmt.Errorf("impacted roots cannot exceed deleted representation rows")
	}
	return commonRepCleanupImpact{
		representations: representations,
		distinctRoots:   distinctRoots,
		rootIDsKnown:    false,
	}, nil
}

// newCommonRepCleanupImpactWithRootIDs constructs batch impact while retaining private
// root identity only for correct distinct-root aggregation across committed batches.
func newCommonRepCleanupImpactWithRootIDs(representations int64, resourceRootIDs []bizmodel.ResourceId) (commonRepCleanupImpact, error) {
	if representations < 0 {
		return commonRepCleanupImpact{}, fmt.Errorf("common representation cleanup impact cannot be negative")
	}
	if int64(len(resourceRootIDs)) > representations {
		return commonRepCleanupImpact{}, fmt.Errorf("impacted roots cannot exceed deleted representation rows")
	}
	return commonRepCleanupImpact{
		representations: representations,
		resourceRootIDs: append([]bizmodel.ResourceId(nil), resourceRootIDs...),
		rootIDsKnown:    true,
		distinctRoots:   int64(len(resourceRootIDs)),
	}, nil
}

// Representations returns the number of common representation rows proposed or removed.
func (impact commonRepCleanupImpact) Representations() int64 { return impact.representations }

// ResourceRootsImpacted returns distinct resource roots touched by the scope.
func (impact commonRepCleanupImpact) ResourceRootsImpacted() int64 { return impact.distinctRoots }

// commonRepCumulativeImpact tracks committed batch impact for one invocation with
// incremental accounting: each committed batch inserts its root identities into a set
// once, so distinct roots are counted without rebuilding the cumulative set per batch.
type commonRepCumulativeImpact struct {
	representations int64
	roots           map[bizmodel.ResourceId]struct{}
}

func newCommonRepCumulativeImpact() *commonRepCumulativeImpact {
	return &commonRepCumulativeImpact{roots: make(map[bizmodel.ResourceId]struct{})}
}

// record merges one committed batch. It must be called only after the batch's
// transaction commits, so failed or retried attempts never contribute.
func (cumulative *commonRepCumulativeImpact) record(batch commonRepCleanupImpact) error {
	if batch.representations < 0 {
		return fmt.Errorf("common representation cleanup impact cannot be negative")
	}
	if batch.distinctRoots > 0 && !batch.rootIDsKnown {
		return fmt.Errorf("cannot account distinct common representation cleanup roots without root identities")
	}
	representations, err := safeCommonRepCountAdd(cumulative.representations, batch.representations)
	if err != nil {
		return err
	}
	cumulative.representations = representations
	for _, rootID := range batch.resourceRootIDs {
		cumulative.roots[rootID] = struct{}{}
	}
	return nil
}

// impact materializes the cumulative counts. Recorded root identities never exceed
// recorded rows because each recorded batch satisfies that invariant by construction.
func (cumulative *commonRepCumulativeImpact) impact() commonRepCleanupImpact {
	rootIDs := make([]bizmodel.ResourceId, 0, len(cumulative.roots))
	for rootID := range cumulative.roots {
		rootIDs = append(rootIDs, rootID)
	}
	return commonRepCleanupImpact{
		representations: cumulative.representations,
		resourceRootIDs: rootIDs,
		rootIDsKnown:    true,
		distinctRoots:   int64(len(rootIDs)),
	}
}

// commonRepCleanupReport separates full prunability from the configured bounded projection.
type commonRepCleanupReport struct {
	evaluatedAt                time.Time
	snapshotAt                 time.Time
	boundedRowLimit            int64
	keepVersions               int
	scopedActiveRoots          int64
	rootsWithoutActiveReporter int64
	rootsWithoutCommonHistory  int64
	excludedInconsistentRoots  int64
	totalPrunable              commonRepCleanupImpact
	boundedPrunable            commonRepCleanupImpact
}

// newCommonRepCleanupReport validates a read-only report and its candidate accounting.
func newCommonRepCleanupReport(
	evaluatedAt time.Time,
	snapshotAt time.Time,
	keepVersions int,
	boundedRowLimit int64,
	scopedActiveRoots int64,
	rootsWithoutActiveReporter int64,
	rootsWithoutCommonHistory int64,
	excludedInconsistentRoots int64,
	totalPrunable commonRepCleanupImpact,
	boundedPrunable commonRepCleanupImpact,
) (commonRepCleanupReport, error) {
	if evaluatedAt.IsZero() || snapshotAt.IsZero() {
		return commonRepCleanupReport{}, fmt.Errorf("common representation cleanup report requires valid evaluation and snapshot times")
	}
	if keepVersions < 2 {
		return commonRepCleanupReport{}, fmt.Errorf("common representation cleanup report requires a retention rule of at least two versions")
	}
	if boundedRowLimit < 0 || scopedActiveRoots < 0 || rootsWithoutActiveReporter < 0 ||
		rootsWithoutCommonHistory < 0 || excludedInconsistentRoots < 0 {
		return commonRepCleanupReport{}, fmt.Errorf("common representation cleanup report counts cannot be negative")
	}
	if boundedPrunable.representations > boundedRowLimit {
		return commonRepCleanupReport{}, fmt.Errorf("bounded rows exceed configured row limit")
	}
	if boundedPrunable.representations > totalPrunable.representations ||
		boundedPrunable.distinctRoots > totalPrunable.distinctRoots {
		return commonRepCleanupReport{}, fmt.Errorf("bounded common representation impact cannot exceed total prunability")
	}
	return commonRepCleanupReport{
		evaluatedAt:                evaluatedAt.UTC(),
		snapshotAt:                 snapshotAt.UTC(),
		boundedRowLimit:            boundedRowLimit,
		keepVersions:               keepVersions,
		scopedActiveRoots:          scopedActiveRoots,
		rootsWithoutActiveReporter: rootsWithoutActiveReporter,
		rootsWithoutCommonHistory:  rootsWithoutCommonHistory,
		excludedInconsistentRoots:  excludedInconsistentRoots,
		totalPrunable:              totalPrunable,
		boundedPrunable:            boundedPrunable,
	}, nil
}

// EvaluatedAt returns the UTC time at which the invocation policy was frozen.
func (report commonRepCleanupReport) EvaluatedAt() time.Time { return report.evaluatedAt }

// SnapshotAt returns the UTC snapshot timestamp used by the read-only database report.
func (report commonRepCleanupReport) SnapshotAt() time.Time { return report.snapshotAt }

// KeepVersions returns the per-resource retention rule used for the report.
func (report commonRepCleanupReport) KeepVersions() int { return report.keepVersions }

// BoundedRowLimit returns the configured row budget used for the bounded projection.
func (report commonRepCleanupReport) BoundedRowLimit() int64 { return report.boundedRowLimit }

// ScopedActiveRoots returns resources with at least one active reporter in scope.
func (report commonRepCleanupReport) ScopedActiveRoots() int64 { return report.scopedActiveRoots }

// RootsWithoutActiveReporter returns roots with common history but no active reporter;
// they are out of scope and belong to the tombstone cleanup workstream.
func (report commonRepCleanupReport) RootsWithoutActiveReporter() int64 {
	return report.rootsWithoutActiveReporter
}

// RootsWithoutCommonHistory returns scoped active roots that have no common
// representations at all; they are not cleanup candidates.
func (report commonRepCleanupReport) RootsWithoutCommonHistory() int64 {
	return report.rootsWithoutCommonHistory
}

// ExcludedInconsistentRoots returns roots whose common_version pointer does not resolve
// to an existing common representation; they are excluded rather than guessed.
func (report commonRepCleanupReport) ExcludedInconsistentRoots() int64 {
	return report.excludedInconsistentRoots
}

// TotalPrunable returns full prunable impact across the scoped active roots.
func (report commonRepCleanupReport) TotalPrunable() commonRepCleanupImpact {
	return report.totalPrunable
}

// BoundedPrunable returns projected impact for the deterministic configured row budget.
func (report commonRepCleanupReport) BoundedPrunable() commonRepCleanupImpact {
	return report.boundedPrunable
}

// commonRepCleanupRepository reads common history prunability or deletes one bounded set.
// The authoritative current version is the resource.common_version pointer, falling back
// to MAX(version) only when the pointer is NULL; selection is per resource and never
// anchored to the job run date. Roots without an active reporter are out of scope, and
// roots whose pointer does not resolve must be excluded and reported, not repaired.
type commonRepCleanupRepository interface {
	// PreviewCommonHistory reports complete prunability, out-of-scope roots, and the
	// deterministic bounded prefix that selects resources with the most versions first
	// and the oldest versions within each resource, without writing.
	PreviewCommonHistory(ctx context.Context, policy commonRepCleanupPolicy, boundedRowLimit int64) (commonRepCleanupReport, error)
	// DeleteCommonHistoryBatch reselects and rechecks the same per-resource retention rule
	// inside its transaction, then deletes at most batchSize prunable common representation
	// rows. It must never delete the retained newest versions, any root row, or any row
	// belonging to a root without an active reporter; callers may treat its impact as
	// committed only after the enclosing transaction commits.
	DeleteCommonHistoryBatch(ctx context.Context, policy commonRepCleanupPolicy, batchSize int) (commonRepCleanupImpact, error)
}

// commonRepCleanupTransaction scopes cleanup repository work to one bounded PostgreSQL transaction.
type commonRepCleanupTransaction interface {
	// WithinSerializableCommonRepCleanup enforces read-only mode for previews and
	// serializable isolation for every transaction. A write transaction may retry its
	// callback after a serialization failure, so the callback must reselect eligibility,
	// reset attempted impact, and emit committed-progress notifications only after this
	// method returns success. Cancellation rolls back the active transaction but does not
	// undo earlier commits.
	WithinSerializableCommonRepCleanup(
		ctx context.Context,
		operationName string,
		readOnly bool,
		statementTimeout time.Duration,
		lockTimeout time.Duration,
		work func(commonRepCleanupRepository) error,
	) error
}

const (
	commonRepDefaultStatementTimeout = 30 * time.Second
	commonRepDefaultLockTimeout      = 5 * time.Second
)

// commonRepCleanupOptions bounds cleanup batch size, pacing, batch count, and total time.
type commonRepCleanupOptions struct {
	batchSize   int
	batchDelay  time.Duration
	maxBatches  int
	maxDuration time.Duration
}

// newCommonRepCleanupOptions validates positive finite execution limits.
func newCommonRepCleanupOptions(batchSize int, batchDelay time.Duration, maxBatches int, maxDuration time.Duration) (commonRepCleanupOptions, error) {
	options := commonRepCleanupOptions{batchSize: batchSize, batchDelay: batchDelay, maxBatches: maxBatches, maxDuration: maxDuration}
	if err := options.Validate(); err != nil {
		return commonRepCleanupOptions{}, err
	}
	return options, nil
}

// Validate rejects non-positive limits, negative delay, and overflowing row budgets.
func (options commonRepCleanupOptions) Validate() error {
	if options.batchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}
	if options.batchDelay < 0 {
		return fmt.Errorf("batch delay cannot be negative")
	}
	if options.maxBatches <= 0 {
		return fmt.Errorf("maximum batches must be positive")
	}
	if options.maxDuration <= 0 {
		return fmt.Errorf("maximum duration must be positive")
	}
	if int64(options.batchSize) > math.MaxInt64/int64(options.maxBatches) {
		return fmt.Errorf("bounded row limit overflows int64")
	}
	return nil
}

// BatchSize returns the maximum common representation rows in one committed batch.
func (options commonRepCleanupOptions) BatchSize() int { return options.batchSize }

// BatchDelay returns the interruptible delay between committed batches.
func (options commonRepCleanupOptions) BatchDelay() time.Duration { return options.batchDelay }

// MaxBatches returns the maximum number of destructive batches in one invocation.
func (options commonRepCleanupOptions) MaxBatches() int { return options.maxBatches }

// MaxDuration returns the total invocation deadline, including delays.
func (options commonRepCleanupOptions) MaxDuration() time.Duration { return options.maxDuration }

// BoundedRowLimit returns the finite upper bound used in preview projections.
func (options commonRepCleanupOptions) BoundedRowLimit() int64 {
	return int64(options.batchSize) * int64(options.maxBatches)
}

// commonRepCleanupResult contains only counts from transactions that committed successfully.
type commonRepCleanupResult struct {
	impact            commonRepCleanupImpact
	committedBatches  int
	batchLimitReached bool
}

// Impact returns the aggregate impact of committed batches.
func (result commonRepCleanupResult) Impact() commonRepCleanupImpact { return result.impact }

// CommittedBatches returns the number of batches whose transactions committed.
func (result commonRepCleanupResult) CommittedBatches() int { return result.committedBatches }

// BatchLimitReached reports whether the configured batch count stopped execution.
func (result commonRepCleanupResult) BatchLimitReached() bool { return result.batchLimitReached }

// commonRepCleanupUsecase orchestrates read-only preview and bounded cleanup execution.
type commonRepCleanupUsecase struct {
	transactions commonRepCleanupTransaction
}

// newCommonRepCleanup constructs the cleanup use case with its transaction port.
func newCommonRepCleanup(transactions commonRepCleanupTransaction) (*commonRepCleanupUsecase, error) {
	if transactions == nil {
		return nil, fmt.Errorf("common representation cleanup transaction port is required")
	}
	return &commonRepCleanupUsecase{transactions: transactions}, nil
}

// Preview returns full prunability and bounded impact from one read-only snapshot.
func (usecase *commonRepCleanupUsecase) Preview(ctx context.Context, policy commonRepCleanupPolicy, options commonRepCleanupOptions) (commonRepCleanupReport, error) {
	if err := usecase.validate(policy, options); err != nil {
		return commonRepCleanupReport{}, err
	}
	runContext, cancel := context.WithTimeout(ctx, options.maxDuration)
	defer cancel()

	var report commonRepCleanupReport
	err := usecase.withSerializable(runContext, "common_rep_cleanup_preview", true, func(repository commonRepCleanupRepository) error {
		// The transaction adapter may invoke this callback again after a retry.
		report = commonRepCleanupReport{}
		preview, err := repository.PreviewCommonHistory(runContext, policy, options.BoundedRowLimit())
		if err != nil {
			return err
		}
		report = preview
		return nil
	})
	if err != nil {
		return commonRepCleanupReport{}, fmt.Errorf("preview common representation history: %w", err)
	}
	return report, nil
}

// Execute commits at most MaxBatches eligible batches and reports only committed impact.
func (usecase *commonRepCleanupUsecase) Execute(
	ctx context.Context,
	policy commonRepCleanupPolicy,
	options commonRepCleanupOptions,
	onBatchCommitted func(batchNumber int, impact commonRepCleanupImpact),
) (commonRepCleanupResult, error) {
	if err := usecase.validate(policy, options); err != nil {
		return commonRepCleanupResult{}, err
	}
	runContext, cancel := context.WithTimeout(ctx, options.maxDuration)
	defer cancel()

	var result commonRepCleanupResult
	cumulative := newCommonRepCumulativeImpact()
	finish := func(err error) (commonRepCleanupResult, error) {
		result.impact = cumulative.impact()
		return result, err
	}
	for result.committedBatches < options.maxBatches {
		if err := runContext.Err(); err != nil {
			return finish(fmt.Errorf("execute common representation cleanup: %w", err))
		}

		var batchImpact commonRepCleanupImpact
		err := usecase.withSerializable(runContext, "common_rep_cleanup_batch", false, func(repository commonRepCleanupRepository) error {
			// Serialization retries must overwrite, not accumulate, attempted counts.
			batchImpact = commonRepCleanupImpact{}
			impact, err := repository.DeleteCommonHistoryBatch(runContext, policy, options.batchSize)
			if err != nil {
				return err
			}
			batchImpact = impact
			return nil
		})
		if err != nil {
			return finish(fmt.Errorf("execute common representation cleanup batch: %w", err))
		}
		if batchImpact.Representations() == 0 {
			return finish(nil)
		}

		if err := cumulative.record(batchImpact); err != nil {
			return finish(fmt.Errorf("accumulate committed common representation cleanup impact: %w", err))
		}
		result.committedBatches++
		if onBatchCommitted != nil {
			onBatchCommitted(result.committedBatches, batchImpact)
		}

		if batchImpact.Representations() < int64(options.batchSize) {
			return finish(nil)
		}
		if result.committedBatches == options.maxBatches {
			result.batchLimitReached = true
			return finish(nil)
		}
		if options.batchDelay > 0 {
			if err := waitForCommonRepNextBatch(runContext, options.batchDelay); err != nil {
				return finish(fmt.Errorf("wait between common representation cleanup batches: %w", err))
			}
		}
	}
	return finish(nil)
}

func (usecase *commonRepCleanupUsecase) validate(policy commonRepCleanupPolicy, options commonRepCleanupOptions) error {
	if usecase == nil || usecase.transactions == nil {
		return fmt.Errorf("common representation cleanup use case is not configured")
	}
	if err := policy.Validate(); err != nil {
		return err
	}
	return options.Validate()
}

func (usecase *commonRepCleanupUsecase) withSerializable(
	ctx context.Context,
	operationName string,
	readOnly bool,
	work func(commonRepCleanupRepository) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	remaining := commonRepDefaultStatementTimeout
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		remaining = time.Until(deadline)
		if remaining <= 0 {
			return context.DeadlineExceeded
		}
	}
	return usecase.transactions.WithinSerializableCommonRepCleanup(
		ctx,
		operationName,
		readOnly,
		minimumCommonRepDuration(commonRepDefaultStatementTimeout, remaining),
		minimumCommonRepDuration(commonRepDefaultLockTimeout, remaining),
		work,
	)
}

func minimumCommonRepDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func waitForCommonRepNextBatch(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func safeCommonRepCountAdd(left, right int64) (int64, error) {
	maxInt64 := int64(^uint64(0) >> 1)
	if left < 0 || right < 0 || left > maxInt64-right {
		return 0, fmt.Errorf("common representation cleanup count overflow or negative count")
	}
	return left + right, nil
}
