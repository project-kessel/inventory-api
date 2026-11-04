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

// maxRowsPerTransaction keeps batch IDs, which are bound twice in one statement,
// well below PostgreSQL's 65,535 bind-parameter limit.
const maxRowsPerTransaction = 50000

// NewCommand creates an online, preview-by-default tombstone cleanup command.
func NewCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	options := options{tombstoneDays: 30, rowsPerTransaction: 1000, batchDelay: 500 * time.Millisecond}
	var execute bool
	command := &cobra.Command{
		Use:   "tombstoned-resources-cleanup-job",
		Short: "Preview or remove expired tombstoned reporters and newly orphaned data",
		Long: `Remove reporters tombstoned before a fixed age cutoff, oldest first, with their
history, and remove common history and roots left without any reporter.
Reads and writes may continue. Every transaction deletes at most
--rows-per-transaction rows; a batch holds rows-per-transaction/5 parents.
Defaults to a read-only preview; --execute commits deletions.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if err := validateInvocation(options); err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runTombstoneCleanupJob(ctx, storageOptions, loggerOptions, options, execute)
		},
	}
	f := command.Flags()
	f.IntVar(&options.tombstoneDays, "tombstone-days", options.tombstoneDays, "Days since the tombstoned reporter was last updated; positive")
	f.BoolVar(&execute, "execute", false, "Commit deletions; default is read-only preview")
	f.IntVar(&options.rowsPerTransaction, "rows-per-transaction", options.rowsPerTransaction, fmt.Sprintf("Maximum rows deleted per transaction across all tables (%d-%d); a batch holds this divided by %d parents", rowsPerParentAtFinalization, maxRowsPerTransaction, rowsPerParentAtFinalization))
	f.DurationVar(&options.batchDelay, "batch-delay", options.batchDelay, "Interruptible delay between batches and between full history chunks")
	f.IntVar(&options.maxBatches, "max-batches", 0, "Maximum parent batches for this run; 0 processes every eligible parent")
	return command
}

// validateInvocation rejects invalid controls before opening PostgreSQL.
func validateInvocation(options options) error {
	if options.tombstoneDays <= 0 {
		return fmt.Errorf("tombstone-days must be positive")
	}
	if options.rowsPerTransaction < rowsPerParentAtFinalization || options.rowsPerTransaction > maxRowsPerTransaction {
		return fmt.Errorf("rows-per-transaction must be between %d and %d", rowsPerParentAtFinalization, maxRowsPerTransaction)
	}
	if options.maxBatches < 0 || options.batchDelay < 0 {
		return fmt.Errorf("max-batches and batch-delay must not be negative")
	}
	// The worklist stores parents-per-batch × max-batches + 1 parents.
	if int64(options.maxBatches) > (math.MaxInt64-1)/int64(options.parentsPerBatch()) {
		return fmt.Errorf("max-batches is too large for rows-per-transaction")
	}
	return nil
}

// openTombstoneCleanupDatabase opens PostgreSQL with the invocation context so the
// caller can cancel connection validation and all subsequent GORM operations.
func openTombstoneCleanupDatabase(ctx context.Context, storageConfig storage.CompletedConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(storageConfig.DSN), &gorm.Config{TranslateError: true, DisableAutomaticPing: true})
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("error accessing database connection: %w", err)
	}
	// Bound connection establishment only; cleanup SQL keeps the invocation context.
	pingContext, cancelPing := context.WithTimeout(ctx, 30*time.Second)
	defer cancelPing()
	if err := sqlDB.PingContext(pingContext); err != nil {
		return nil, errors.Join(fmt.Errorf("error pinging database: %w", err), sqlDB.Close())
	}
	return db.WithContext(ctx), nil
}

// runTombstoneCleanupJob opens PostgreSQL and logs committed progress and the outcome.
func runTombstoneCleanupJob(ctx context.Context, storageOptions *storage.Options, loggerOptions common.LoggerOptions, options options, execute bool) (jobErr error) {
	if storageOptions == nil || storageOptions.Database != storage.Postgres {
		return fmt.Errorf("tombstone cleanup requires PostgreSQL storage options")
	}
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	helper := log.NewHelper(log.With(logger, "job", "tombstoned_resources_cleanup", "dry_run", !execute))
	helper.Infow("msg", "tombstone cleanup configuration", "tombstone_days", options.tombstoneDays,
		"rows_per_transaction", options.rowsPerTransaction, "parents_per_batch", options.parentsPerBatch(),
		"batch_delay", options.batchDelay.String(), "max_batches", options.maxBatches, "max_parents", options.maxParents())
	var r result
	defer func() { logResult(helper, "tombstone cleanup result", execute, r, &jobErr) }()
	db, err := openTombstoneCleanupDatabase(ctx, storage.NewConfig(storageOptions).Complete())
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer func() { jobErr = errors.Join(jobErr, sqlDB.Close()) }()
	r, err = cleanup(ctx, db, options, execute, func(message string, r result) {
		logResult(helper, message, execute, r, nil)
	})
	return err
}

// logResult reports progress; passing jobErr marks the final outcome with a status.
func logResult(helper *log.Helper, message string, execute bool, r result, jobErr *error) {
	fields := []any{"msg", message, "candidates", r.candidates, "remaining_worklist_parents", r.remaining,
		"selection_duration", r.selectionDuration.String(), "complete", r.complete}
	if !r.cutoff.IsZero() {
		fields = append(fields, "cutoff", r.cutoff.Format(time.RFC3339Nano))
	}
	if execute {
		fields = append(fields, "committed_batches", r.batches, "committed_transactions", r.transactions,
			"batch_duration", r.batchDuration.String(), "skipped_parents", r.skipped,
			"rows_deleted", r.deleted.total(), "reporter_representations_deleted", r.deleted.reporterHistory,
			"common_representations_deleted", r.deleted.commonHistory,
			"reporter_resources_deleted", r.deleted.parents, "resources_deleted", r.deleted.roots)
	} else {
		fields = append(fields, "affected_resources", r.affected,
			"eligible_reporter_representations", r.eligible.reporterHistory,
			"eligible_common_representations", r.eligible.commonHistory,
			"eligible_reporter_resources", r.eligible.parents, "eligible_resources", r.eligible.roots)
	}
	if jobErr == nil {
		helper.Infow(fields...)
		return
	}
	if *jobErr == nil {
		helper.Infow(append(fields, "status", "succeeded")...)
		return
	}
	helper.Errorw(append(fields, "status", "failed", "error", (*jobErr).Error())...)
}
