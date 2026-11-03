package tombstonecleanup

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Retain the existing upper bound on candidate fetches per transaction.
const maxRowsPerTransaction = 65535 - 1

// NewCommand creates an online, preview-by-default tombstone cleanup command.
func NewCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	options := options{tombstoneDays: 30, rowsPerTransaction: 1000, batchDelay: 500 * time.Millisecond}
	var execute bool
	command := &cobra.Command{
		Use:   "tombstoned-resources-cleanup-job",
		Short: "Preview or remove expired tombstoned reporters and newly orphaned data",
		Long:  "Remove reporters tombstoned before a fixed age cutoff, then their newly orphaned common history and roots. Reads and writes may continue. Defaults to a read-only preview; --execute commits deletions.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if err := validateInvocation(options); err != nil {
				return err
			}
			ctx, stop := tombstoneCleanupSignalContext(command.Context())
			defer stop()
			return runTombstoneCleanupJob(ctx, storageOptions, loggerOptions, options, execute)
		},
	}
	f := command.Flags()
	f.IntVar(&options.tombstoneDays, "tombstone-days", 30, "Days since the tombstoned reporter was last updated; positive")
	f.BoolVar(&execute, "execute", false, "Commit deletions; default is read-only preview")
	f.IntVar(&options.rowsPerTransaction, "rows-per-transaction", 1000, "Maximum deleted rows across histories, reporter parents and roots per transaction; minimum 5")
	f.DurationVar(&options.batchDelay, "batch-delay", 500*time.Millisecond, "Interruptible delay between committed batches")
	f.IntVar(&options.transactionLimit, "transaction-limit", 0, "Maximum committed transactions for this run; 0 processes the entire worklist")
	return command
}

// validateInvocation rejects values that cannot be represented safely before
// opening PostgreSQL. Unlimited batches (zero) remain supported.
func validateInvocation(options options) error {
	earliestCutoff := time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
	maxDays := (time.Now().UTC().Unix() - earliestCutoff.Unix()) / 86400
	if options.tombstoneDays <= 0 || int64(options.tombstoneDays) > maxDays {
		return fmt.Errorf("tombstone-days must be between 1 and %d so the cutoff does not predate year 1", maxDays)
	}
	if options.rowsPerTransaction < 5 || options.rowsPerTransaction > maxRowsPerTransaction {
		return fmt.Errorf("rows-per-transaction must be between 5 and %d", maxRowsPerTransaction)
	}
	if options.transactionLimit < 0 || options.batchDelay < 0 {
		return fmt.Errorf("transaction limit and batch delay must not be negative")
	}
	if options.transactionLimit > 0 && int64(options.transactionLimit) > math.MaxInt64/int64(options.rowsPerTransaction) {
		return fmt.Errorf("rows-per-transaction multiplied by transaction-limit would overflow the row limit")
	}
	return nil
}

// openTombstoneCleanupDatabase opens PostgreSQL with the invocation context so the
// caller can cancel connection validation and all subsequent GORM operations.
func openTombstoneCleanupDatabase(ctx context.Context, storageConfig storage.CompletedConfig) (*gorm.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}
	db, err := gorm.Open(postgres.Open(storageConfig.DSN), &gorm.Config{
		TranslateError:       true,
		DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("error accessing database connection: %w", err)
	}
	// Bound connection establishment only; all cleanup SQL retains the original
	// invocation context and has no job/statement runtime deadline.
	pingContext, cancelPing := context.WithTimeout(ctx, 30*time.Second)
	pingErr := sqlDB.PingContext(pingContext)
	cancelPing()
	if pingErr != nil {
		pingErr = fmt.Errorf("error pinging database: %w", pingErr)
		if closeErr := sqlDB.Close(); closeErr != nil {
			return nil, errors.Join(pingErr, fmt.Errorf("error closing database: %w", closeErr))
		}
		return nil, pingErr
	}
	return db.WithContext(ctx), nil
}

// tombstoneCleanupSignalContext lets process termination cancel active SQL and pacing.
func tombstoneCleanupSignalContext(parentContext context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parentContext, os.Interrupt, syscall.SIGTERM)
}

// runTombstoneCleanupJob opens PostgreSQL and reports only committed deletion counts.
func runTombstoneCleanupJob(ctx context.Context, storageOptions *storage.Options, loggerOptions common.LoggerOptions, options options, execute bool) (jobErr error) {
	if storageOptions == nil || storageOptions.Database != storage.Postgres {
		return fmt.Errorf("tombstone cleanup requires PostgreSQL storage options")
	}
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	helper := log.NewHelper(log.With(logger, "job", "tombstoned_resources_cleanup", "dry_run", !execute))
	var r result
	defer func() { logResult(helper, options, execute, "tombstone cleanup result", r, jobErr) }()
	db, err := openTombstoneCleanupDatabase(ctx, storage.NewConfig(storageOptions).Complete())
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			jobErr = errors.Join(jobErr, fmt.Errorf("error closing database: %w", closeErr))
		}
	}()
	helper.Infow("msg", "tombstone cleanup configuration", "tombstone_days", options.tombstoneDays, "rows_per_transaction", options.rowsPerTransaction, "batch_delay", options.batchDelay.String(), "transaction_limit", options.transactionLimit)

	r, err = cleanup(ctx, db, options, execute, func(r result) { logResult(helper, options, execute, "tombstone cleanup selection complete", r, nil) }, func(r result) { logResult(helper, options, execute, "tombstone cleanup batch committed", r, nil) }, func(r result) {
		helper.Infow("msg", "tombstone cleanup batch started", "batch", r.batches+1, "rows_per_transaction", options.rowsPerTransaction, "remaining_worklist_rows", r.remaining, "committed_batches", r.batches)
	})
	return err
}

// logResult reports scoped progress and the final invocation outcome.
func logResult(helper *log.Helper, options options, execute bool, message string, r result, jobErr error) {
	deletedRows := int64(0)
	if execute {
		deletedRows = r.deletedRows()
	}
	fields := []any{
		"msg", message, "transaction_limit", options.transactionLimit, "worklist_limit", options.worklistLimit(),
		"candidates", r.candidates, "selection_duration", r.selectionDuration.String(),
		"row_limit", options.worklistLimit(), "rows_deleted", deletedRows,
		"batch_reporter_representations", r.lastBatch.reporterHistory,
		"batch_common_representations", r.lastBatch.commonHistory,
		"batch_reporter_resources", r.lastBatch.parents, "batch_resource_roots", r.lastBatch.roots,
		"batch_duration", r.batchDuration.String(), "committed_batches", r.batches,
		"selected", r.selected, "skipped", r.skipped,
		"reporter_resources", r.reporterResources, "reporter_representations", r.reporterHistory,
		"common_representations", r.commonHistory, "resource_roots", r.roots,
		"affected_resources", r.affected, "remaining_worklist_rows", r.remaining, "complete", r.complete,
	}
	if !execute {
		fields = append(fields, "total_eligible_reporter_history", r.totalReporterHistory,
			"total_eligible_common_history", r.totalCommonHistory,
			"total_removable_roots", r.totalRoots, "total_affected_resources", r.totalAffected)
	}
	if !r.cutoff.IsZero() {
		fields = append(fields, "cutoff", r.cutoff.Format(time.RFC3339Nano))
	}
	if message == "tombstone cleanup result" {
		status := "succeeded"
		if jobErr != nil {
			status = "failed"
		}
		fields = append(fields, "status", status)
	}
	if jobErr != nil {
		fields = append(fields, "error", jobErr.Error())
		helper.Errorw(fields...)
	} else {
		helper.Infow(fields...)
	}
}
