package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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

// ResourceType returns the normalized resource type filter and whether a filter was selected.
func (policy commonRepCleanupPolicy) ResourceType() (bizmodel.ResourceType, bool) {
	return policy.resourceType, policy.hasType
}

// commonRepCleanupImpact describes rows from one committed batch. Root IDs stay
// internal so repeated batches of the same root can be counted once.
type commonRepCleanupImpact struct {
	representations int64
	resourceRootIDs []uuid.UUID
}

const (
	commonRepExclusionNone                = ""
	commonRepExclusionNoActiveReporter    = "no_active_reporter"
	commonRepExclusionInconsistentPointer = "inconsistent_pointer"
	commonRepDefaultStatementTimeout      = 30 * time.Second
	commonRepDefaultLockTimeout           = 5 * time.Second
)

type commonRepRootClassification struct {
	resourceID   uuid.UUID
	exclusion    string
	prunableRows int64
}

type commonRepCleanupReport struct {
	startedAt                  time.Time
	finishedAt                 time.Time
	keepVersions               int
	chunksProcessed            int64
	rootsScanned               int64
	prunableRoots              int64
	prunableRows               int64
	rootsWithoutActiveReporter int64
	excludedInconsistentRoots  int64
	lastCursor                 uuid.UUID
	passComplete               bool
}

type commonRepChunkProgress struct {
	chunkNumber  int64
	rootsScanned int64
	prunableRows int64
	cursor       uuid.UUID
}

type commonRepCleanupResult struct {
	report            commonRepCleanupReport
	deletedRows       int64
	impactedRoots     int64
	committedBatches  int
	batchLimitReached bool
}

// commonRepCleanupRepository scans root-aligned chunks and rechecks retention
// inside every delete transaction. Deletion must enforce rowLimit in SQL.
type commonRepCleanupRepository interface {
	ReadChunk(context.Context, commonRepCleanupPolicy, uuid.UUID, int) ([]commonRepRootClassification, uuid.UUID, bool, error)
	DeletePrunableRoots(context.Context, commonRepCleanupPolicy, []uuid.UUID, int) (commonRepCleanupImpact, error)
}

// commonRepCleanupTransaction scopes repository work to serializable transactions.
// Retried callbacks replace attempted counts; progress is recorded after commit.
type commonRepCleanupTransaction interface {
	WithinSerializableCommonRepCleanup(context.Context, string, bool, time.Duration, time.Duration, func(commonRepCleanupRepository) error) error
}

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

type commonRepCleanupUsecase struct {
	transactions commonRepCleanupTransaction
}

func newCommonRepCleanup(transactions commonRepCleanupTransaction) (*commonRepCleanupUsecase, error) {
	if transactions == nil {
		return nil, fmt.Errorf("common representation cleanup transactions are required")
	}
	return &commonRepCleanupUsecase{transactions: transactions}, nil
}

// Preview uses the same scan as execution, with read-only transactions and no deletes.
func (usecase *commonRepCleanupUsecase) Preview(ctx context.Context, policy commonRepCleanupPolicy, options commonRepCleanupOptions, startAfter uuid.UUID, onChunk func(commonRepChunkProgress)) (commonRepCleanupReport, error) {
	result, err := usecase.run(ctx, policy, options, startAfter, false, nil, onChunk)
	return result.report, err
}

// Execute commits bounded deletions and reports progress only after each commit.
func (usecase *commonRepCleanupUsecase) Execute(ctx context.Context, policy commonRepCleanupPolicy, options commonRepCleanupOptions, startAfter uuid.UUID, onBatch func(int, commonRepCleanupImpact), onChunk func(commonRepChunkProgress)) (commonRepCleanupResult, error) {
	return usecase.run(ctx, policy, options, startAfter, true, onBatch, onChunk)
}

// run advances the cursor only after a whole chunk finishes. A budget stop or
// failure within a chunk therefore resumes by safely reprocessing that chunk.
func (usecase *commonRepCleanupUsecase) run(ctx context.Context, policy commonRepCleanupPolicy, options commonRepCleanupOptions, startAfter uuid.UUID, execute bool, onBatch func(int, commonRepCleanupImpact), onChunk func(commonRepChunkProgress)) (result commonRepCleanupResult, runErr error) {
	if usecase == nil || usecase.transactions == nil {
		return result, fmt.Errorf("common representation cleanup is not configured")
	}
	if err := policy.Validate(); err != nil {
		return result, err
	}
	if err := options.Validate(); err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, options.maxDuration)
	defer cancel()
	result.report = commonRepCleanupReport{startedAt: time.Now().UTC(), keepVersions: policy.keepVersions, lastCursor: startAfter}
	roots := make(map[uuid.UUID]struct{})
	defer func() {
		result.report.finishedAt = time.Now().UTC()
		result.impactedRoots = int64(len(roots))
	}()

	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if execute && result.committedBatches >= options.maxBatches {
			result.batchLimitReached = true
			return result, nil
		}
		var classifications []commonRepRootClassification
		var boundary uuid.UUID
		var found bool
		err := usecase.withSerializable(ctx, "common_rep_cleanup_scan", true, func(repository commonRepCleanupRepository) error {
			var err error
			classifications, boundary, found, err = repository.ReadChunk(ctx, policy, result.report.lastCursor, options.chunkRows)
			return err
		})
		if err != nil {
			return result, fmt.Errorf("scan common representation history: %w", err)
		}

		progress := commonRepChunkProgress{chunkNumber: result.report.chunksProcessed + 1, rootsScanned: int64(len(classifications)), cursor: boundary}
		resourceIDs := make([]uuid.UUID, 0, len(classifications))
		for _, classification := range classifications {
			switch classification.exclusion {
			case commonRepExclusionNoActiveReporter:
				result.report.rootsWithoutActiveReporter++
			case commonRepExclusionInconsistentPointer:
				result.report.excludedInconsistentRoots++
			case commonRepExclusionNone:
				if classification.prunableRows > 0 {
					result.report.prunableRoots++
					progress.prunableRows += classification.prunableRows
					resourceIDs = append(resourceIDs, classification.resourceID)
				}
			}
		}
		result.report.chunksProcessed++
		result.report.rootsScanned += progress.rootsScanned
		result.report.prunableRows += progress.prunableRows
		if onChunk != nil {
			onChunk(progress)
		}

		// Every root uses the same SQL-limited deletion path, including deep roots.
		// Re-evaluate until a batch returns fewer rows than its limit. Concurrent
		// writes cannot expand a transaction beyond that limit.
		for execute && len(resourceIDs) > 0 {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			if result.committedBatches >= options.maxBatches {
				result.batchLimitReached = true
				return result, nil
			}
			var impact commonRepCleanupImpact
			err := usecase.withSerializable(ctx, "common_rep_cleanup_batch", false, func(repository commonRepCleanupRepository) error {
				var err error
				impact, err = repository.DeletePrunableRoots(ctx, policy, resourceIDs, options.batchSize)
				return err
			})
			if err != nil {
				return result, fmt.Errorf("delete common representation history: %w", err)
			}
			if impact.representations > 0 {
				result.deletedRows += impact.representations
				for _, rootID := range impact.resourceRootIDs {
					roots[rootID] = struct{}{}
				}
				result.committedBatches++
				if onBatch != nil {
					onBatch(result.committedBatches, impact)
				}
			}
			if impact.representations < int64(options.batchSize) {
				break
			}
			if err := waitForCommonRepNextBatch(ctx, options.batchDelay); err != nil {
				return result, err
			}
		}
		if found {
			result.report.lastCursor = boundary
		}
		if !found {
			result.report.passComplete = true
			return result, nil
		}
		if execute && len(resourceIDs) > 0 {
			if err := waitForCommonRepNextBatch(ctx, options.batchDelay); err != nil {
				return result, err
			}
		}
	}
}

func (usecase *commonRepCleanupUsecase) withSerializable(ctx context.Context, operationName string, readOnly bool, work func(commonRepCleanupRepository) error) error {
	return usecase.transactions.WithinSerializableCommonRepCleanup(ctx, operationName, readOnly, commonRepDefaultStatementTimeout, commonRepDefaultLockTimeout, work)
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
