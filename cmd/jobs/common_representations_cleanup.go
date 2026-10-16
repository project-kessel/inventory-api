package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

// Exclusion labels for chunk classification. Every visited root has common history by
// construction, so only reporter and pointer states can exclude a root.
const (
	commonRepExclusionNone                = ""
	commonRepExclusionNoActiveReporter    = "no_active_reporter"
	commonRepExclusionInconsistentPointer = "inconsistent_pointer"
)

// commonRepRootClassification is the planning verdict for one resource root inside a
// scanned chunk: either an exclusion reason or the number of rows the retention rule
// would prune from that root.
type commonRepRootClassification struct {
	resourceID   uuid.UUID
	exclusion    string
	prunableRows int64
}

// newCommonRepRootClassification validates one classification row. A root carries
// either an exclusion or a prunable row count, never both.
func newCommonRepRootClassification(resourceID uuid.UUID, exclusion string, prunableRows int64) (commonRepRootClassification, error) {
	if resourceID == uuid.Nil {
		return commonRepRootClassification{}, fmt.Errorf("common representation classification requires a resource id")
	}
	switch exclusion {
	case commonRepExclusionNone:
	case commonRepExclusionNoActiveReporter, commonRepExclusionInconsistentPointer:
		if prunableRows != 0 {
			return commonRepRootClassification{}, fmt.Errorf("an excluded root cannot report prunable rows")
		}
	default:
		return commonRepRootClassification{}, fmt.Errorf("unknown common representation classification exclusion %q", exclusion)
	}
	if prunableRows < 0 {
		return commonRepRootClassification{}, fmt.Errorf("common representation classification prunable rows cannot be negative")
	}
	return commonRepRootClassification{resourceID: resourceID, exclusion: exclusion, prunableRows: prunableRows}, nil
}

// ResourceID returns the classified root.
func (classification commonRepRootClassification) ResourceID() uuid.UUID {
	return classification.resourceID
}

// Exclusion returns the empty string for an eligible root, or the exclusion reason.
func (classification commonRepRootClassification) Exclusion() string { return classification.exclusion }

// PrunableRows returns the rows the retention rule would delete from an eligible root.
func (classification commonRepRootClassification) PrunableRows() int64 {
	return classification.prunableRows
}

// commonRepScanTotals accumulates classification results over one pass. Counts are
// taken from per-chunk transactions, so they describe the scan window, not a single
// database snapshot.
type commonRepScanTotals struct {
	chunksProcessed            int64
	rootsScanned               int64
	prunableRoots              int64
	prunableRows               int64
	rootsWithoutActiveReporter int64
	excludedInconsistentRoots  int64
}

// recordChunk merges one chunk's classifications into the pass totals.
func (totals *commonRepScanTotals) recordChunk(classifications []commonRepRootClassification) error {
	totals.chunksProcessed++
	for index := range classifications {
		classification := classifications[index]
		totals.rootsScanned++
		switch classification.exclusion {
		case commonRepExclusionNoActiveReporter:
			totals.rootsWithoutActiveReporter++
		case commonRepExclusionInconsistentPointer:
			totals.excludedInconsistentRoots++
		case commonRepExclusionNone:
			if classification.prunableRows > 0 {
				totals.prunableRoots++
				prunableRows, err := safeCommonRepCountAdd(totals.prunableRows, classification.prunableRows)
				if err != nil {
					return err
				}
				totals.prunableRows = prunableRows
			}
		}
	}
	return nil
}

// commonRepCleanupReport describes one pass over common representation history. The
// scan walks resource ids in primary-key order, so lastCursor is the resume point and
// passComplete means no prunable row remained below the cursor at scan time.
type commonRepCleanupReport struct {
	startedAt    time.Time
	finishedAt   time.Time
	keepVersions int
	totals       commonRepScanTotals
	lastCursor   uuid.UUID
	passComplete bool
}

// newCommonRepCleanupReport validates pass totals and timing.
func newCommonRepCleanupReport(
	startedAt time.Time,
	finishedAt time.Time,
	keepVersions int,
	totals commonRepScanTotals,
	lastCursor uuid.UUID,
	passComplete bool,
) (commonRepCleanupReport, error) {
	if startedAt.IsZero() || finishedAt.IsZero() || finishedAt.Before(startedAt) {
		return commonRepCleanupReport{}, fmt.Errorf("common representation cleanup report requires a valid scan window")
	}
	if keepVersions < 2 {
		return commonRepCleanupReport{}, fmt.Errorf("common representation cleanup report requires a retention rule of at least two versions")
	}
	if totals.chunksProcessed < 0 || totals.rootsScanned < 0 || totals.prunableRoots < 0 ||
		totals.prunableRows < 0 || totals.rootsWithoutActiveReporter < 0 || totals.excludedInconsistentRoots < 0 {
		return commonRepCleanupReport{}, fmt.Errorf("common representation cleanup report counts cannot be negative")
	}
	classified := totals.prunableRoots + totals.rootsWithoutActiveReporter + totals.excludedInconsistentRoots
	if classified > totals.rootsScanned {
		return commonRepCleanupReport{}, fmt.Errorf("classified roots cannot exceed scanned roots")
	}
	return commonRepCleanupReport{
		startedAt:    startedAt.UTC(),
		finishedAt:   finishedAt.UTC(),
		keepVersions: keepVersions,
		totals:       totals,
		lastCursor:   lastCursor,
		passComplete: passComplete,
	}, nil
}

// StartedAt returns the UTC time the pass began.
func (report commonRepCleanupReport) StartedAt() time.Time { return report.startedAt }

// FinishedAt returns the UTC time the pass ended.
func (report commonRepCleanupReport) FinishedAt() time.Time { return report.finishedAt }

// KeepVersions returns the per-resource retention rule used for the pass.
func (report commonRepCleanupReport) KeepVersions() int { return report.keepVersions }

// ChunksProcessed returns the number of chunks the pass scanned.
func (report commonRepCleanupReport) ChunksProcessed() int64 { return report.totals.chunksProcessed }

// RootsScanned returns the number of distinct roots the pass classified. Roots
// without common history are never visited, and roots excluded by a resource
// type filter are not classified.
func (report commonRepCleanupReport) RootsScanned() int64 { return report.totals.rootsScanned }

// PrunableRoots returns roots the retention rule would prune.
func (report commonRepCleanupReport) PrunableRoots() int64 { return report.totals.prunableRoots }

// PrunableRows returns common representation rows the retention rule would prune.
func (report commonRepCleanupReport) PrunableRows() int64 { return report.totals.prunableRows }

// RootsWithoutActiveReporter returns visited roots with common history but no active
// reporter; they are out of scope and belong to the tombstone cleanup workstream.
func (report commonRepCleanupReport) RootsWithoutActiveReporter() int64 {
	return report.totals.rootsWithoutActiveReporter
}

// ExcludedInconsistentRoots returns visited roots whose common_version pointer does
// not resolve to an existing common representation; they are excluded, not repaired.
func (report commonRepCleanupReport) ExcludedInconsistentRoots() int64 {
	return report.totals.excludedInconsistentRoots
}

// LastCursor returns the resource id the pass reached. A completed pass ignores it;
// an interrupted pass resumes after it with --start-after.
func (report commonRepCleanupReport) LastCursor() uuid.UUID { return report.lastCursor }

// PassComplete reports whether the scan reached the end of the key space. Rows that
// concurrent writers inserted below the cursor during the pass are picked up by the
// next pass, not by this flag.
func (report commonRepCleanupReport) PassComplete() bool { return report.passComplete }

// commonRepCleanupRepository reads one chunk of common history prunability or deletes
// bounded prunable rows. The authoritative current version is the
// resource.common_version pointer, falling back to MAX(version) only when the pointer
// is NULL; selection is per resource and never anchored to the job run date. Roots
// without an active reporter are out of scope, and roots whose pointer does not
// resolve must be excluded and reported, not repaired.
type commonRepCleanupRepository interface {
	// ReadChunk discovers the next chunk boundary at least chunkRows rows past the
	// cursor and classifies every root in (cursor, boundary], root-aligned so no
	// root is ever split across chunks. When fewer than chunkRows rows remain,
	// found is false and the classifications cover all remaining roots.
	ReadChunk(ctx context.Context, policy commonRepCleanupPolicy, after uuid.UUID, chunkRows int) (classifications []commonRepRootClassification, boundary uuid.UUID, found bool, err error)
	// DeletePrunableRoots re-classifies the given roots inside the caller's
	// transaction and deletes every prunable row they still have. It must never
	// delete retained versions, root rows, or rows belonging to an excluded root;
	// callers may treat its impact as committed only after the enclosing
	// transaction commits.
	DeletePrunableRoots(ctx context.Context, policy commonRepCleanupPolicy, resourceIDs []uuid.UUID) (commonRepCleanupImpact, error)
	// DeleteLowestPrunableVersions re-derives the retention threshold for one root
	// inside the caller's transaction and deletes at most rowLimit of its lowest
	// prunable versions, so pathological roots are pruned in bounded statements
	// instead of one unbounded delete.
	DeleteLowestPrunableVersions(ctx context.Context, policy commonRepCleanupPolicy, resourceID uuid.UUID, rowLimit int64) (int64, error)
}

// commonRepCleanupTransaction scopes cleanup repository work to one bounded PostgreSQL transaction.
type commonRepCleanupTransaction interface {
	// WithinSerializableCommonRepCleanup enforces read-only mode for scans and
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

// commonRepCleanupOptions bounds chunk scan size, delete batch size, pacing, batch
// count, and total time for one invocation.
type commonRepCleanupOptions struct {
	chunkRows   int
	batchSize   int
	batchDelay  time.Duration
	maxBatches  int
	maxDuration time.Duration
}

// newCommonRepCleanupOptions validates positive finite execution limits.
func newCommonRepCleanupOptions(chunkRows int, batchSize int, batchDelay time.Duration, maxBatches int, maxDuration time.Duration) (commonRepCleanupOptions, error) {
	options := commonRepCleanupOptions{chunkRows: chunkRows, batchSize: batchSize, batchDelay: batchDelay, maxBatches: maxBatches, maxDuration: maxDuration}
	if err := options.Validate(); err != nil {
		return commonRepCleanupOptions{}, err
	}
	return options, nil
}

// Validate rejects non-positive limits and negative delay.
func (options commonRepCleanupOptions) Validate() error {
	if options.chunkRows <= 0 {
		return fmt.Errorf("chunk rows must be positive")
	}
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
	return nil
}

// ChunkRows returns the maximum common representation rows scanned per chunk.
func (options commonRepCleanupOptions) ChunkRows() int { return options.chunkRows }

// BatchSize returns the maximum common representation rows in one committed batch.
func (options commonRepCleanupOptions) BatchSize() int { return options.batchSize }

// BatchDelay returns the interruptible delay between committed batches.
func (options commonRepCleanupOptions) BatchDelay() time.Duration { return options.batchDelay }

// MaxBatches returns the maximum number of destructive batches in one invocation.
func (options commonRepCleanupOptions) MaxBatches() int { return options.maxBatches }

// MaxDuration returns the total invocation deadline, including delays.
func (options commonRepCleanupOptions) MaxDuration() time.Duration { return options.maxDuration }

// commonRepChunkProgress reports one scanned chunk to an optional observer.
type commonRepChunkProgress struct {
	chunkNumber  int64
	rootsScanned int64
	prunableRows int64
	cursor       uuid.UUID
}

// commonRepCleanupResult contains committed impact and pass progress from one
// destructive invocation.
type commonRepCleanupResult struct {
	report            commonRepCleanupReport
	impact            commonRepCleanupImpact
	committedBatches  int
	batchLimitReached bool
	parkedRoots       []uuid.UUID
}

// Report returns the scan progress of the invocation.
func (result commonRepCleanupResult) Report() commonRepCleanupReport { return result.report }

// Impact returns the aggregate impact of committed batches.
func (result commonRepCleanupResult) Impact() commonRepCleanupImpact { return result.impact }

// CommittedBatches returns the number of batches whose transactions committed.
func (result commonRepCleanupResult) CommittedBatches() int { return result.committedBatches }

// BatchLimitReached reports whether the configured batch count stopped execution.
func (result commonRepCleanupResult) BatchLimitReached() bool { return result.batchLimitReached }

// ParkedRoots returns roots whose deletions repeatedly hit serialization conflicts
// and were skipped; rerunning the job retries them.
func (result commonRepCleanupResult) ParkedRoots() []uuid.UUID { return result.parkedRoots }

// commonRepPrunableRoot carries one eligible root and its planned row count from
// chunk classification into batch assembly.
type commonRepPrunableRoot struct {
	resourceID  uuid.UUID
	plannedRows int64
}

// commonRepDeleteGroup is one unit of destructive work: either several small roots
// deleted by one statement, or one deep root whose planned rows exceed the batch size
// and is deleted in bounded lowest-version batches.
type commonRepDeleteGroup struct {
	roots []commonRepPrunableRoot
	deep  bool
}

// assembleCommonRepDeleteGroups packs eligible roots into delete groups in scan
// order. A group closes once its planned rows reach the batch size; a root whose
// planned rows alone exceed the batch size becomes its own deep group so no delete
// statement is ever unbounded by a pathological root.
func assembleCommonRepDeleteGroups(roots []commonRepPrunableRoot, batchSize int64) []commonRepDeleteGroup {
	groups := make([]commonRepDeleteGroup, 0)
	current := make([]commonRepPrunableRoot, 0, len(roots))
	currentRows := int64(0)
	flush := func() {
		if len(current) > 0 {
			groups = append(groups, commonRepDeleteGroup{roots: current})
			current = make([]commonRepPrunableRoot, 0, len(roots))
			currentRows = 0
		}
	}
	for _, root := range roots {
		if root.plannedRows > batchSize {
			flush()
			groups = append(groups, commonRepDeleteGroup{roots: []commonRepPrunableRoot{root}, deep: true})
			continue
		}
		current = append(current, root)
		currentRows += root.plannedRows
		if currentRows >= batchSize {
			flush()
		}
	}
	flush()
	return groups
}

// prunableCommonRepRoots filters a chunk's classifications down to eligible roots
// with planned deletions, preserving scan order.
func prunableCommonRepRoots(classifications []commonRepRootClassification) []commonRepPrunableRoot {
	roots := make([]commonRepPrunableRoot, 0, len(classifications))
	for index := range classifications {
		classification := classifications[index]
		if classification.exclusion == commonRepExclusionNone && classification.prunableRows > 0 {
			roots = append(roots, commonRepPrunableRoot{resourceID: classification.resourceID, plannedRows: classification.prunableRows})
		}
	}
	return roots
}

// isCommonRepSerializationExhaustion reports whether an error still wraps a
// PostgreSQL serialization failure after the shared transaction manager exhausted
// its retries. The pass parks such roots and continues instead of failing.
func isCommonRepSerializationExhaustion(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "40001"
}

// commonRepCleanupUsecase orchestrates read-only preview passes and chunked cleanup execution.
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

// Preview scans the pass read-only from the cursor and reports prunability. The scan
// stops and reports partial progress when the invocation budget is exhausted.
func (usecase *commonRepCleanupUsecase) Preview(
	ctx context.Context,
	policy commonRepCleanupPolicy,
	options commonRepCleanupOptions,
	startAfter uuid.UUID,
	onChunk func(progress commonRepChunkProgress),
) (commonRepCleanupReport, error) {
	if err := usecase.validate(policy, options); err != nil {
		return commonRepCleanupReport{}, err
	}
	runContext, cancel := context.WithTimeout(ctx, options.MaxDuration())
	defer cancel()

	startedAt := time.Now().UTC()
	totals := commonRepScanTotals{}
	cursor := startAfter
	passComplete := false
	for {
		if err := runContext.Err(); err != nil {
			break
		}
		var classifications []commonRepRootClassification
		var boundary uuid.UUID
		var found bool
		err := usecase.withSerializable(runContext, "common_rep_cleanup_preview_chunk", true, func(repository commonRepCleanupRepository) error {
			read, readBoundary, readFound, readErr := repository.ReadChunk(runContext, policy, cursor, options.ChunkRows())
			if readErr != nil {
				return readErr
			}
			classifications, boundary, found = read, readBoundary, readFound
			return nil
		})
		if err != nil {
			return commonRepCleanupReport{}, fmt.Errorf("preview common representation history: %w", err)
		}
		chunkPrunableRows := int64(0)
		for index := range classifications {
			if classifications[index].exclusion == commonRepExclusionNone {
				chunkPrunableRows += classifications[index].prunableRows
			}
		}
		if err := totals.recordChunk(classifications); err != nil {
			return commonRepCleanupReport{}, fmt.Errorf("preview common representation history: %w", err)
		}
		if found {
			cursor = boundary
		}
		if onChunk != nil {
			onChunk(commonRepChunkProgress{
				chunkNumber:  totals.chunksProcessed,
				rootsScanned: int64(len(classifications)),
				prunableRows: chunkPrunableRows,
				cursor:       cursor,
			})
		}
		if !found {
			passComplete = true
			break
		}
	}
	report, err := newCommonRepCleanupReport(startedAt, time.Now().UTC(), policy.KeepVersions(), totals, cursor, passComplete)
	if err != nil {
		return commonRepCleanupReport{}, fmt.Errorf("preview common representation history: %w", err)
	}
	return report, nil
}

// Execute scans the pass from the cursor and commits bounded deletions for every
// eligible root. Roots whose deletions repeatedly hit serialization conflicts are
// parked and reported rather than failing the invocation.
func (usecase *commonRepCleanupUsecase) Execute(
	ctx context.Context,
	policy commonRepCleanupPolicy,
	options commonRepCleanupOptions,
	startAfter uuid.UUID,
	onBatch func(batchNumber int, impact commonRepCleanupImpact),
	onChunk func(progress commonRepChunkProgress),
) (commonRepCleanupResult, error) {
	if err := usecase.validate(policy, options); err != nil {
		return commonRepCleanupResult{}, err
	}
	runContext, cancel := context.WithTimeout(ctx, options.MaxDuration())
	defer cancel()

	startedAt := time.Now().UTC()
	cumulative := newCommonRepCumulativeImpact()
	totals := commonRepScanTotals{}
	cursor := startAfter
	result := commonRepCleanupResult{}
	passComplete := false
	parked := make(map[uuid.UUID]struct{})
	stopped := false

	finish := func(err error) (commonRepCleanupResult, error) {
		parkedRoots := make([]uuid.UUID, 0, len(parked))
		for rootID := range parked {
			parkedRoots = append(parkedRoots, rootID)
		}
		result.impact = cumulative.impact()
		result.parkedRoots = parkedRoots
		report, reportErr := newCommonRepCleanupReport(startedAt, time.Now().UTC(), policy.KeepVersions(), totals, cursor, passComplete)
		if reportErr != nil {
			return result, fmt.Errorf("execute common representation history cleanup: %w", reportErr)
		}
		result.report = report
		if err != nil {
			return result, fmt.Errorf("execute common representation history cleanup: %w", err)
		}
		return result, nil
	}

	// Budget checks only refuse work that has not started, so a budget consumed
	// exactly by the final batch of the final chunk still reports a complete pass.
	for !stopped {
		if err := runContext.Err(); err != nil {
			return finish(err)
		}
		if result.committedBatches >= options.MaxBatches() {
			result.batchLimitReached = true
			break
		}

		var classifications []commonRepRootClassification
		var boundary uuid.UUID
		var found bool
		err := usecase.withSerializable(runContext, "common_rep_cleanup_chunk_scan", true, func(repository commonRepCleanupRepository) error {
			read, readBoundary, readFound, readErr := repository.ReadChunk(runContext, policy, cursor, options.ChunkRows())
			if readErr != nil {
				return readErr
			}
			classifications, boundary, found = read, readBoundary, readFound
			return nil
		})
		if err != nil {
			return finish(err)
		}
		chunkPrunableRows := int64(0)
		for index := range classifications {
			if classifications[index].exclusion == commonRepExclusionNone {
				chunkPrunableRows += classifications[index].prunableRows
			}
		}
		if err := totals.recordChunk(classifications); err != nil {
			return finish(err)
		}
		if onChunk != nil {
			onChunk(commonRepChunkProgress{
				chunkNumber:  totals.chunksProcessed,
				rootsScanned: int64(len(classifications)),
				prunableRows: chunkPrunableRows,
				cursor:       boundary,
			})
		}

		for _, group := range assembleCommonRepDeleteGroups(prunableCommonRepRoots(classifications), int64(options.BatchSize())) {
			if stopped {
				break
			}
			if err := runContext.Err(); err != nil {
				return finish(err)
			}
			if result.committedBatches >= options.MaxBatches() {
				result.batchLimitReached = true
				stopped = true
				break
			}

			if group.deep {
				root := group.roots[0]
			deep:
				for {
					if err := runContext.Err(); err != nil {
						return finish(err)
					}
					if result.committedBatches >= options.MaxBatches() {
						result.batchLimitReached = true
						stopped = true
						break deep
					}
					var deleted int64
					err := usecase.withSerializable(runContext, "common_rep_cleanup_deep_batch", false, func(repository commonRepCleanupRepository) error {
						// Serialization retries must overwrite, not accumulate, attempted rows.
						deleted = 0
						deletedRows, deleteErr := repository.DeleteLowestPrunableVersions(runContext, policy, root.resourceID, int64(options.BatchSize()))
						if deleteErr != nil {
							return deleteErr
						}
						deleted = deletedRows
						return nil
					})
					if err != nil {
						if isCommonRepSerializationExhaustion(err) {
							parked[root.resourceID] = struct{}{}
							break deep
						}
						return finish(err)
					}
					if deleted == 0 {
						break deep
					}
					rootID, rootIDErr := bizmodel.NewResourceId(root.resourceID)
					if rootIDErr != nil {
						return finish(fmt.Errorf("prunable common representation has invalid resource root identity: %w", rootIDErr))
					}
					batchImpact, impactErr := newCommonRepCleanupImpactWithRootIDs(deleted, []bizmodel.ResourceId{rootID})
					if impactErr != nil {
						return finish(impactErr)
					}
					if err := cumulative.record(batchImpact); err != nil {
						return finish(err)
					}
					result.committedBatches++
					if onBatch != nil {
						onBatch(result.committedBatches, batchImpact)
					}
					if options.BatchDelay() > 0 {
						if err := waitForCommonRepNextBatch(runContext, options.BatchDelay()); err != nil {
							return finish(err)
						}
					}
				}
				continue
			}

			resourceIDs := make([]uuid.UUID, 0, len(group.roots))
			for _, root := range group.roots {
				resourceIDs = append(resourceIDs, root.resourceID)
			}
			var batchImpact commonRepCleanupImpact
			err := usecase.withSerializable(runContext, "common_rep_cleanup_batch", false, func(repository commonRepCleanupRepository) error {
				// Serialization retries must overwrite, not accumulate, attempted impact.
				batchImpact = commonRepCleanupImpact{}
				deleteImpact, deleteErr := repository.DeletePrunableRoots(runContext, policy, resourceIDs)
				if deleteErr != nil {
					return deleteErr
				}
				batchImpact = deleteImpact
				return nil
			})
			if err != nil {
				if isCommonRepSerializationExhaustion(err) {
					for _, root := range group.roots {
						parked[root.resourceID] = struct{}{}
					}
					continue
				}
				return finish(err)
			}
			// A group may have drifted to nothing between planning and deletion.
			if batchImpact.Representations() == 0 {
				continue
			}
			if err := cumulative.record(batchImpact); err != nil {
				return finish(err)
			}
			result.committedBatches++
			if onBatch != nil {
				onBatch(result.committedBatches, batchImpact)
			}
			if options.BatchDelay() > 0 {
				if err := waitForCommonRepNextBatch(runContext, options.BatchDelay()); err != nil {
					return finish(err)
				}
			}
		}

		// The cursor advances only after every group of the chunk committed, so a
		// budget stop mid-chunk reports the previous chunk boundary and a resume
		// re-processes the interrupted chunk instead of skipping its roots.
		if !stopped && found {
			cursor = boundary
		}
		if !found {
			if !stopped {
				passComplete = true
			}
			break
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
