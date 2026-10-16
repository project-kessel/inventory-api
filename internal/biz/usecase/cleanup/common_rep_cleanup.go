package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

const (
	commonRepDefaultStatementTimeout = 30 * time.Second
	commonRepDefaultLockTimeout      = 5 * time.Second
)

// Report describes cleanup scan progress.
type Report struct {
	StartedAt                  time.Time
	FinishedAt                 time.Time
	KeepVersions               int
	ChunksProcessed            int64
	RootsScanned               int64
	PrunableRoots              int64
	PrunableRows               int64
	RootsWithoutActiveReporter int64
	ExcludedInconsistentRoots  int64
	LastCursor                 uuid.UUID
	PassComplete               bool
}

// ChunkProgress describes cleanup chunk observations.
type ChunkProgress struct {
	ChunkNumber  int64
	RootsScanned int64
	PrunableRows int64
	Cursor       uuid.UUID
}

// Result describes cleanup committed execution progress.
type Result struct {
	Report            Report
	DeletedRows       int64
	ImpactedRoots     int64
	CommittedBatches  int
	BatchLimitReached bool
}

// Options bounds chunk scan size, delete batch size, pacing, batch
// count, and total time for one invocation.
type Options struct {
	ChunkRows   int
	BatchSize   int
	BatchDelay  time.Duration
	MaxBatches  int
	MaxDuration time.Duration
}

// NewOptions validates positive finite execution limits.
func NewOptions(chunkRows int, batchSize int, batchDelay time.Duration, maxBatches int, maxDuration time.Duration) (Options, error) {
	options := Options{ChunkRows: chunkRows, BatchSize: batchSize, BatchDelay: batchDelay, MaxBatches: maxBatches, MaxDuration: maxDuration}
	if err := options.Validate(); err != nil {
		return Options{}, err
	}
	return options, nil
}

// Validate rejects non-positive limits and negative delay.
func (options Options) Validate() error {
	if options.ChunkRows <= 0 {
		return fmt.Errorf("chunk rows must be positive")
	}
	if options.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}
	if options.BatchDelay < 0 {
		return fmt.Errorf("batch delay cannot be negative")
	}
	if options.MaxBatches <= 0 {
		return fmt.Errorf("maximum batches must be positive")
	}
	if options.MaxDuration <= 0 {
		return fmt.Errorf("maximum duration must be positive")
	}
	return nil
}

// Usecase describes cleanup orchestration.
type Usecase struct {
	transactions bizmodel.CommonRepCleanupTransaction
}

// New constructs the cleanup application service.
func New(transactions bizmodel.CommonRepCleanupTransaction) (*Usecase, error) {
	if transactions == nil {
		return nil, fmt.Errorf("common representation cleanup transactions are required")
	}
	return &Usecase{transactions: transactions}, nil
}

// Preview uses the same scan as execution, with read-only transactions and no deletes.
func (usecase *Usecase) Preview(ctx context.Context, policy bizmodel.CommonRepCleanupPolicy, options Options, startAfter uuid.UUID, onChunk func(ChunkProgress)) (Report, error) {
	result, err := usecase.run(ctx, policy, options, startAfter, false, nil, onChunk)
	return result.Report, err
}

// Execute commits bounded deletions and reports progress only after each commit.
func (usecase *Usecase) Execute(ctx context.Context, policy bizmodel.CommonRepCleanupPolicy, options Options, startAfter uuid.UUID, onBatch func(int, bizmodel.CommonRepCleanupImpact), onChunk func(ChunkProgress)) (Result, error) {
	return usecase.run(ctx, policy, options, startAfter, true, onBatch, onChunk)
}

// run advances the cursor only after a whole chunk finishes. A budget stop or
// failure within a chunk therefore resumes by safely reprocessing that chunk.
func (usecase *Usecase) run(ctx context.Context, policy bizmodel.CommonRepCleanupPolicy, options Options, startAfter uuid.UUID, execute bool, onBatch func(int, bizmodel.CommonRepCleanupImpact), onChunk func(ChunkProgress)) (result Result, runErr error) {
	if usecase == nil || usecase.transactions == nil {
		return result, fmt.Errorf("common representation cleanup is not configured")
	}
	if err := policy.Validate(); err != nil {
		return result, err
	}
	if err := options.Validate(); err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, options.MaxDuration)
	defer cancel()
	result.Report = Report{StartedAt: time.Now().UTC(), KeepVersions: policy.KeepVersions(), LastCursor: startAfter}
	roots := make(map[bizmodel.ResourceId]struct{})
	defer func() {
		result.Report.FinishedAt = time.Now().UTC()
		result.ImpactedRoots = int64(len(roots))
	}()

	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if execute && result.CommittedBatches >= options.MaxBatches {
			result.BatchLimitReached = true
			return result, nil
		}
		var classifications []bizmodel.CommonRepRootClassification
		var boundary uuid.UUID
		var found bool
		err := usecase.withSerializable(ctx, "common_rep_cleanup_scan", true, func(repository bizmodel.CommonRepCleanupRepository) error {
			var err error
			classifications, boundary, found, err = repository.ReadChunk(ctx, policy, result.Report.LastCursor, options.ChunkRows)
			return err
		})
		if err != nil {
			return result, fmt.Errorf("scan common representation history: %w", err)
		}

		progress := ChunkProgress{ChunkNumber: result.Report.ChunksProcessed + 1, RootsScanned: int64(len(classifications)), Cursor: boundary}
		resourceIDs := make([]bizmodel.ResourceId, 0, len(classifications))
		for _, classification := range classifications {
			switch classification.Exclusion() {
			case bizmodel.CommonRepExclusionNoActiveReporter:
				result.Report.RootsWithoutActiveReporter++
			case bizmodel.CommonRepExclusionInconsistentPointer:
				result.Report.ExcludedInconsistentRoots++
			case bizmodel.CommonRepExclusionNone:
				if classification.PrunableRows() > 0 {
					result.Report.PrunableRoots++
					progress.PrunableRows += classification.PrunableRows()
					resourceIDs = append(resourceIDs, classification.ResourceID())
				}
			}
		}
		result.Report.ChunksProcessed++
		result.Report.RootsScanned += progress.RootsScanned
		result.Report.PrunableRows += progress.PrunableRows
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
			if result.CommittedBatches >= options.MaxBatches {
				result.BatchLimitReached = true
				return result, nil
			}
			var impact bizmodel.CommonRepCleanupImpact
			err := usecase.withSerializable(ctx, "common_rep_cleanup_batch", false, func(repository bizmodel.CommonRepCleanupRepository) error {
				var err error
				impact, err = repository.DeletePrunableRoots(ctx, policy, resourceIDs, options.BatchSize)
				return err
			})
			if err != nil {
				return result, fmt.Errorf("delete common representation history: %w", err)
			}
			if impact.Representations() > 0 {
				result.DeletedRows += impact.Representations()
				for _, rootID := range impact.ResourceRootIDs() {
					roots[rootID] = struct{}{}
				}
				result.CommittedBatches++
				if onBatch != nil {
					onBatch(result.CommittedBatches, impact)
				}
			}
			if impact.Representations() < int64(options.BatchSize) {
				break
			}
			if err := waitForCommonRepNextBatch(ctx, options.BatchDelay); err != nil {
				return result, err
			}
		}
		if found {
			result.Report.LastCursor = boundary
		}
		if !found {
			result.Report.PassComplete = true
			return result, nil
		}
		if execute && len(resourceIDs) > 0 {
			if err := waitForCommonRepNextBatch(ctx, options.BatchDelay); err != nil {
				return result, err
			}
		}
	}
}

func (usecase *Usecase) withSerializable(ctx context.Context, operationName string, readOnly bool, work func(bizmodel.CommonRepCleanupRepository) error) error {
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
