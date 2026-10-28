package tombstonecleanup

import (
	"context"
	"errors"
	"fmt"
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

// NewCommand creates an online, preview-by-default tombstone cleanup command.
func NewCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	options := options{tombstoneDays: 30, batchSize: 1000, batchDelay: 500 * time.Millisecond}
	var execute, boundaryVerified bool
	command := &cobra.Command{
		Use:   "tombstoned-resources-cleanup-job",
		Short: "Preview or remove expired tombstoned reporters and newly orphaned data",
		Long:  "Remove reporters tombstoned before a fixed age cutoff, then their newly orphaned common history and roots. Writes may continue. Execution requires manual verification of publication, successful consumption and restart-safe offsets through a pre-cleanup boundary; the acknowledgment does not verify it.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if options.tombstoneDays <= 0 {
				return fmt.Errorf("tombstone-days must be positive")
			}
			if options.batchSize <= 0 || options.maxBatches < 0 || options.batchDelay < 0 {
				return fmt.Errorf("batch size must be positive; maximum batches and batch delay must not be negative")
			}
			if execute && !boundaryVerified {
				return fmt.Errorf("--execute requires --consumer-boundary-verified; verify publication, successful consumption and restart-safe offsets first")
			}
			ctx, stop := tombstoneCleanupSignalContext(command.Context())
			defer stop()
			return runTombstoneCleanupJob(ctx, storageOptions, loggerOptions, options, execute)
		},
	}
	f := command.Flags()
	f.IntVar(&options.tombstoneDays, "tombstone-days", 30, "Days since the tombstoned reporter was last updated; positive")
	f.BoolVar(&execute, "execute", false, "Commit deletions; default is read-only preview")
	f.BoolVar(&boundaryVerified, "consumer-boundary-verified", false, "Acknowledge the runbook's manual pipeline boundary verification; does not establish it")
	f.IntVar(&options.batchSize, "batch-size", 1000, "Maximum reporter parents per transaction; cascading history is additional")
	f.DurationVar(&options.batchDelay, "batch-delay", 500*time.Millisecond, "Interruptible delay between committed batches")
	f.IntVar(&options.maxBatches, "max-batches", 0, "Maximum committed batches; 0 processes the entire worklist")
	return command
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
	if err := sqlDB.PingContext(ctx); err != nil {
		pingErr := fmt.Errorf("error pinging database: %w", err)
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
	helper.Infow("msg", "tombstone cleanup configuration", "tombstone_days", options.tombstoneDays, "batch_size", options.batchSize, "batch_delay", options.batchDelay.String(), "max_batches", options.maxBatches)
	report := func(message string, r result) {
		fields := []any{
			"msg", message, "cutoff", r.cutoff.Format(time.RFC3339Nano),
			"candidates", r.candidates, "selection_duration", r.selectionDuration.String(),
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
		helper.Infow(fields...)
	}
	r, err := cleanup(ctx, db, options, execute, func(r result) { report("tombstone cleanup selection complete", r) }, func(r result) { report("tombstone cleanup batch committed", r) })
	report("tombstone cleanup result", r)
	return err
}
