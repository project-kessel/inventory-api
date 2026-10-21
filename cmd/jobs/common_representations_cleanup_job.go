package jobs

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewCommonRepresentationsCleanupJobCommand creates a preview-by-default job
// for a maintenance window with resource writes fenced and replication drained.
func NewCommonRepresentationsCleanupJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	options := commonRepCleanupOptions{keepVersions: 2, batchSize: 1000, batchDelay: 500 * time.Millisecond, maxBatches: 0}
	var execute, writersPaused bool
	command := &cobra.Command{
		Use:   "common-representations-cleanup-job",
		Short: "Preview or batch-delete old common representation history",
		Long: `Keep each active resource's current representation and its newest existing
predecessors (two versions by default), regardless of their age.
Preview is the default. Execution requires --execute --writers-paused: operators
must block all resource writers and verify replication is fully drained first.
Reads and the drained consumer may continue. These flags do not establish or
verify maintenance mode. Execution selects a temporary worklist once and deletes
bounded batches; rerun after interruption to select the remaining history.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if execute && !writersPaused {
				return fmt.Errorf("--execute requires --writers-paused; fence all resource writers and verify replication has drained before cleanup")
			}
			if err := validateCommonRepCleanupInvocation(options.keepVersions); err != nil {
				return err
			}
			if options.batchSize <= 0 || options.maxBatches < 0 || options.batchDelay < 0 {
				return fmt.Errorf("batch size must be positive; maximum batches and batch delay must not be negative")
			}
			if options.resourceType != "" {
				resourceType, err := bizmodel.NewResourceType(options.resourceType)
				if err != nil || strings.TrimSpace(options.resourceType) == "" {
					return fmt.Errorf("invalid resource type filter %q", options.resourceType)
				}
				options.resourceType = resourceType.Serialize()
			}
			ctx, stopSignals := commonRepCleanupSignalContext(command.Context())
			defer stopSignals()
			return runCommonRepresentationsCleanupJob(ctx, storageOptions, loggerOptions, options, execute)
		},
	}
	flags := command.Flags()
	flags.IntVar(&options.keepVersions, "keep-versions", options.keepVersions, "Newest current/predecessor versions retained regardless of age; minimum 2")
	flags.StringVar(&options.resourceType, "resource-type", "", "Optional resource type filter")
	flags.BoolVar(&execute, "execute", false, "Commit bounded deletions; default is read-only preview")
	flags.BoolVar(&writersPaused, "writers-paused", false, "Acknowledge all resource writers are fenced and replication is fully drained; does not establish either condition")
	flags.IntVar(&options.batchSize, "batch-size", options.batchSize, "Maximum history rows deleted per transaction")
	flags.DurationVar(&options.batchDelay, "batch-delay", options.batchDelay, "Interruptible delay between committed batches")
	flags.IntVar(&options.maxBatches, "max-batches", options.maxBatches, "Maximum committed batches; 0 processes the entire worklist")
	return command
}

// validateCommonRepCleanupInvocation protects the history needed for update consumption.
func validateCommonRepCleanupInvocation(keepVersions int) error {
	if keepVersions < 2 {
		return fmt.Errorf("--keep-versions must be at least 2 because update consumption reads version v and v-1")
	}
	return nil
}

// openCommonRepCleanupDatabase opens PostgreSQL with the invocation context so the
// caller can cancel connection validation and all subsequent GORM operations.
func openCommonRepCleanupDatabase(ctx context.Context, storageConfig storage.CompletedConfig) (*gorm.DB, error) {
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
		_ = sqlDB.Close()
		return nil, fmt.Errorf("error pinging database: %w", err)
	}
	return db.WithContext(ctx), nil
}

// commonRepCleanupSignalContext lets process termination cancel active SQL and pacing.
func commonRepCleanupSignalContext(parentContext context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parentContext, os.Interrupt, syscall.SIGTERM)
}

// runCommonRepresentationsCleanupJob opens PostgreSQL and logs committed cleanup progress.
func runCommonRepresentationsCleanupJob(ctx context.Context, storageOptions *storage.Options, loggerOptions common.LoggerOptions, options commonRepCleanupOptions, execute bool) error {
	if storageOptions == nil || storageOptions.Database != storage.Postgres {
		return fmt.Errorf("common representation cleanup requires PostgreSQL storage options")
	}
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "common_representations_cleanup"))
	db, err := openCommonRepCleanupDatabase(ctx, storage.NewConfig(storageOptions).Complete())
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close() //nolint:errcheck
	if execute {
		logHelper.Warn("Cleanup execution enabled; resource writers must remain fenced and replication drained")
	}
	result, cleanupErr := cleanupCommonRepresentations(ctx, db, options, execute, func(batch int, deleted int64, duration time.Duration) {
		logHelper.Infow("msg", "common representation cleanup batch committed", "batch", batch, "common_representations_deleted", deleted, "duration", duration.String())
	})
	logHelper.Infow("msg", "common representation cleanup result",
		"execution_requested", execute, "keep_versions", options.keepVersions,
		"resource_type_filter", options.resourceType, "prunable_common_representations", result.candidateRows,
		"selection_duration", result.selectionDuration.String(), "committed_batches", result.committedBatches,
		"common_representations_deleted", result.deletedRows, "remaining_worklist_rows", result.remainingRows, "complete", result.complete)
	return cleanupErr
}
