package cleanup

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

const (
	defaultStatementTimeout = 30 * time.Second
	defaultLockTimeout      = 5 * time.Second
)

// Options bounds cleanup batch size, pacing, batch count, and total invocation time.
type Options struct {
	batchSize   int
	batchDelay  time.Duration
	maxBatches  int
	maxDuration time.Duration
}

// NewOptions validates positive finite execution limits.
func NewOptions(batchSize int, batchDelay time.Duration, maxBatches int, maxDuration time.Duration) (Options, error) {
	options := Options{batchSize: batchSize, batchDelay: batchDelay, maxBatches: maxBatches, maxDuration: maxDuration}
	if err := options.Validate(); err != nil {
		return Options{}, err
	}
	return options, nil
}

// Validate rejects non-positive limits, negative delay, and overflowing row budgets.
func (options Options) Validate() error {
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
		return fmt.Errorf("bounded reporter row limit overflows int64")
	}
	return nil
}

// BatchSize returns the maximum reporter metadata rows in one committed batch.
func (options Options) BatchSize() int { return options.batchSize }

// BatchDelay returns the interruptible delay between committed batches.
func (options Options) BatchDelay() time.Duration { return options.batchDelay }

// MaxBatches returns the maximum number of destructive batches in one invocation.
func (options Options) MaxBatches() int { return options.maxBatches }

// MaxDuration returns the total invocation deadline, including delays.
func (options Options) MaxDuration() time.Duration { return options.maxDuration }

// BoundedReporterRowLimit returns the finite upper bound used in preview projections.
func (options Options) BoundedReporterRowLimit() int64 {
	return int64(options.batchSize) * int64(options.maxBatches)
}

// Result contains only counts from transactions that committed successfully.
type Result struct {
	impact            model.TombstoneCleanupImpact
	committedBatches  int
	batchLimitReached bool
}

// Impact returns the aggregate impact of committed batches.
func (result Result) Impact() model.TombstoneCleanupImpact { return result.impact }

// CommittedBatches returns the number of batches whose transactions committed.
func (result Result) CommittedBatches() int { return result.committedBatches }

// BatchLimitReached reports whether the configured batch count stopped execution.
func (result Result) BatchLimitReached() bool { return result.batchLimitReached }

// Usecase orchestrates read-only preview and bounded tombstone cleanup execution.
type Usecase struct {
	transactions model.TombstoneCleanupTransaction
}

// New constructs the cleanup use case with its transaction port.
func New(transactions model.TombstoneCleanupTransaction) (*Usecase, error) {
	if transactions == nil {
		return nil, fmt.Errorf("tombstone cleanup transaction port is required")
	}
	return &Usecase{transactions: transactions}, nil
}

// Preview returns full eligibility and bounded impact from one read-only snapshot.
func (usecase *Usecase) Preview(ctx context.Context, policy model.TombstoneCleanupPolicy, options Options) (model.TombstoneCleanupReport, error) {
	if err := usecase.validate(policy, options); err != nil {
		return model.TombstoneCleanupReport{}, err
	}
	runContext, cancel := context.WithTimeout(ctx, options.maxDuration)
	defer cancel()

	var report model.TombstoneCleanupReport
	err := usecase.withSerializable(runContext, "tombstone_cleanup_preview", true, func(repository model.TombstoneCleanupRepository) error {
		// The transaction adapter may invoke this callback again after a retry.
		report = model.TombstoneCleanupReport{}
		preview, err := repository.PreviewTombstones(runContext, policy, options.BoundedReporterRowLimit())
		if err != nil {
			return err
		}
		report = preview
		return nil
	})
	if err != nil {
		return model.TombstoneCleanupReport{}, fmt.Errorf("preview tombstoned resources: %w", err)
	}
	return report, nil
}

// Execute commits at most MaxBatches eligible batches and reports only committed impact.
func (usecase *Usecase) Execute(
	ctx context.Context,
	policy model.TombstoneCleanupPolicy,
	options Options,
	onBatchCommitted func(batchNumber int, impact model.TombstoneCleanupImpact),
) (Result, error) {
	if err := usecase.validate(policy, options); err != nil {
		return Result{}, err
	}
	runContext, cancel := context.WithTimeout(ctx, options.maxDuration)
	defer cancel()

	var result Result
	for result.committedBatches < options.maxBatches {
		if err := runContext.Err(); err != nil {
			return result, fmt.Errorf("execute tombstone cleanup: %w", err)
		}

		var batchImpact model.TombstoneCleanupImpact
		err := usecase.withSerializable(runContext, "tombstone_cleanup_batch", false, func(repository model.TombstoneCleanupRepository) error {
			// Serialization retries must overwrite, not accumulate, attempted counts.
			batchImpact = model.TombstoneCleanupImpact{}
			impact, err := repository.DeleteTombstoneBatch(runContext, policy, options.batchSize)
			if err != nil {
				return err
			}
			batchImpact = impact
			return nil
		})
		if err != nil {
			return result, fmt.Errorf("execute tombstone cleanup batch: %w", err)
		}
		if batchImpact.ReporterResources() == 0 {
			return result, nil
		}

		totalImpact, err := result.impact.Add(batchImpact)
		if err != nil {
			return result, fmt.Errorf("accumulate committed tombstone cleanup impact: %w", err)
		}
		result.impact = totalImpact
		result.committedBatches++
		if onBatchCommitted != nil {
			onBatchCommitted(result.committedBatches, batchImpact)
		}

		if batchImpact.ReporterResources() < int64(options.batchSize) {
			return result, nil
		}
		if result.committedBatches == options.maxBatches {
			result.batchLimitReached = true
			return result, nil
		}
		if options.batchDelay > 0 {
			if err := waitForNextBatch(runContext, options.batchDelay); err != nil {
				return result, fmt.Errorf("wait between tombstone cleanup batches: %w", err)
			}
		}
	}
	return result, nil
}

func (usecase *Usecase) validate(policy model.TombstoneCleanupPolicy, options Options) error {
	if usecase == nil || usecase.transactions == nil {
		return fmt.Errorf("tombstone cleanup use case is not configured")
	}
	if err := policy.Validate(); err != nil {
		return err
	}
	return options.Validate()
}

func (usecase *Usecase) withSerializable(
	ctx context.Context,
	operationName string,
	readOnly bool,
	work func(model.TombstoneCleanupRepository) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	remaining := defaultStatementTimeout
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		remaining = time.Until(deadline)
		if remaining <= 0 {
			return context.DeadlineExceeded
		}
	}
	return usecase.transactions.WithinSerializableTombstoneCleanup(
		ctx,
		operationName,
		readOnly,
		minimumDuration(defaultStatementTimeout, remaining),
		minimumDuration(defaultLockTimeout, remaining),
		work,
	)
}

func minimumDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func waitForNextBatch(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
