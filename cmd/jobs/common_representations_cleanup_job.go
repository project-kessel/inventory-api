package jobs

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	commonRepCleanupDefaultKeepVersions = 2
	commonRepCleanupDefaultChunkRows    = 20000
	commonRepCleanupDefaultBatchSize    = 1000
	commonRepCleanupDefaultBatchDelay   = 500 * time.Millisecond
	commonRepCleanupDefaultMaxBatches   = 1
	commonRepCleanupDefaultMaxDuration  = 5 * time.Minute
)

// NewCommonRepresentationsCleanupJobCommand creates a dry-run-by-default common
// representation history cleanup command.
func NewCommonRepresentationsCleanupJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var (
		execute      bool
		keepVersions int
		resourceType string
		startAfter   string
		chunkRows    int
		batchSize    int
		batchDelay   time.Duration
		maxBatches   int
		maxDuration  time.Duration
	)

	command := &cobra.Command{
		Use:   "common-representations-cleanup-job",
		Short: "Preview or prune prunable common representation history on active resources",
		Long: `Previews common representation history pruning by default. The pass walks
common representations in primary-key order and applies a per-resource retention
rule that keeps the authoritative current common representation and its
predecessors; only older versions on resources that still have an active reporter
are pruned. Destructive execution requires --execute. The scan is resumable:
an interrupted pass reports its last cursor and continues after it with
--start-after. Pending consumer events whose versions were pruned degrade to
create-only or no-op processing rather than failing; see the common representation
cleanup runbook for the residual tuple-residue risk and the consumer-health
guidance before mass pruning.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			signalContext, stopSignals := commonRepCleanupSignalContext(command.Context())
			defer stopSignals()
			if err := validateCommonRepCleanupInvocation(keepVersions); err != nil {
				return err
			}
			return runCommonRepresentationsCleanupJob(
				signalContext,
				storageOptions,
				loggerOptions,
				execute,
				keepVersions,
				resourceType,
				startAfter,
				chunkRows,
				batchSize,
				batchDelay,
				maxBatches,
				maxDuration,
			)
		},
	}

	flags := command.Flags()
	flags.BoolVar(&execute, "execute", false, "Commit bounded common representation cleanup batches; default is read-only preview")
	flags.IntVar(&keepVersions, "keep-versions", commonRepCleanupDefaultKeepVersions, "Number of newest common representations retained per resource; must be at least 2 because update consumption reads version v and v-1 (default 2)")
	flags.StringVar(&resourceType, "resource-type", "", "Optional resource type filter")
	flags.StringVar(&startAfter, "start-after", uuid.Nil.String(), "Resume the pass after this resource id, as reported in the last cursor of an interrupted run")
	flags.IntVar(&chunkRows, "chunk-rows", commonRepCleanupDefaultChunkRows, "Maximum common representation rows classified per chunk scan (default 20000)")
	flags.IntVar(&batchSize, "batch-size", commonRepCleanupDefaultBatchSize, "Maximum common representation rows per transaction (default 1000)")
	flags.DurationVar(&batchDelay, "batch-delay", commonRepCleanupDefaultBatchDelay, "Interruptible delay between committed batches (default 500ms)")
	flags.IntVar(&maxBatches, "max-batches", commonRepCleanupDefaultMaxBatches, "Maximum number of destructive batches in one invocation (default 1)")
	flags.DurationVar(&maxDuration, "max-duration", commonRepCleanupDefaultMaxDuration, "Maximum total invocation duration, including batch delays (default 5m)")

	return command
}

// openCommonRepCleanupDatabase opens PostgreSQL with the invocation context so the
// job deadline bounds connection validation and all subsequent GORM operations.
func openCommonRepCleanupDatabase(ctx context.Context, storageConfig storage.CompletedConfig, logger *log.Helper) (*gorm.DB, error) {
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

func commonRepCleanupSignalContext(parentContext context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parentContext, os.Interrupt, syscall.SIGTERM)
}

func validateCommonRepCleanupInvocation(keepVersions int) error {
	if keepVersions < 2 {
		return fmt.Errorf("--keep-versions must be at least 2 because update consumption reads version v and v-1")
	}
	return nil
}

func runCommonRepresentationsCleanupJob(
	parentContext context.Context,
	storageOptions *storage.Options,
	loggerOptions common.LoggerOptions,
	execute bool,
	keepVersions int,
	resourceType string,
	startAfter string,
	chunkRows int,
	batchSize int,
	batchDelay time.Duration,
	maxBatches int,
	maxDuration time.Duration,
) error {
	if err := parentContext.Err(); err != nil {
		return fmt.Errorf("common representation cleanup canceled before startup: %w", err)
	}
	if storageOptions == nil {
		return fmt.Errorf("storage options are required")
	}
	if storageOptions.Database != storage.Postgres {
		return fmt.Errorf("common representation cleanup requires PostgreSQL; configured database is %q", storageOptions.Database)
	}
	resumeCursor, err := uuid.Parse(startAfter)
	if err != nil {
		return fmt.Errorf("--start-after must be a valid resource id: %w", err)
	}
	options, err := newCommonRepCleanupOptions(chunkRows, batchSize, batchDelay, maxBatches, maxDuration)
	if err != nil {
		return fmt.Errorf("invalid common representation cleanup execution bounds: %w", err)
	}
	jobContext, cancel := context.WithTimeout(parentContext, options.MaxDuration())
	defer cancel()
	policy, err := newCommonRepCleanupPolicy(keepVersions, resourceType)
	if err != nil {
		return err
	}
	if storageOptions.MaxSerializationRetries <= 0 {
		return fmt.Errorf("storage.max-serialization-retries must be positive")
	}
	if err := jobContext.Err(); err != nil {
		return fmt.Errorf("common representation cleanup canceled before database startup: %w", err)
	}

	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "common_representations_cleanup"))
	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := openCommonRepCleanupDatabase(jobContext, storageConfig, logHelper)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get common representation cleanup database handle: %w", err)
	}
	defer sqlDB.Close() //nolint:errcheck
	if err := jobContext.Err(); err != nil {
		return fmt.Errorf("common representation cleanup canceled during database startup: %w", err)
	}

	metrics := &metricscollector.MetricsCollector{}
	if err := metrics.New(otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")); err != nil {
		return fmt.Errorf("initialize common representation cleanup transaction metrics: %w", err)
	}
	transactionManager := data.NewGormTransactionManager(metrics, storageOptions.MaxSerializationRetries)
	transactions := newCommonRepCleanupTransaction(db, transactionManager)
	usecase, err := newCommonRepCleanup(transactions)
	if err != nil {
		return err
	}

	onChunk := func(progress commonRepChunkProgress) {
		logHelper.Infow("msg", "common representation cleanup chunk scanned",
			"chunk", progress.chunkNumber,
			"chunk_roots_scanned", progress.rootsScanned,
			"chunk_prunable_rows", progress.prunableRows,
			"cursor", progress.cursor.String(),
		)
	}

	if !execute {
		report, previewErr := usecase.Preview(jobContext, policy, options, resumeCursor, onChunk)
		if previewErr != nil {
			return previewErr
		}
		logCommonRepCleanupReport(logHelper, report, policy, options, false)
		logHelper.Info("Common representation cleanup preview completed; no database mutations were made")
		return nil
	}

	logHelper.Warn("Common representation cleanup execution explicitly enabled; committed batches cannot be undone by cancellation")
	result, executeErr := usecase.Execute(jobContext, policy, options, resumeCursor, func(batchNumber int, impact commonRepCleanupImpact) {
		logHelper.Infow("msg", "common representation cleanup batch committed",
			"batch", batchNumber,
			"common_representations_deleted", impact.Representations(),
			"resource_roots_impacted", impact.ResourceRootsImpacted(),
		)
	}, onChunk)
	logCommonRepCleanupReport(logHelper, result.Report(), policy, options, true)
	logCommonRepCleanupResult(logHelper, result)
	return executeErr
}

func logCommonRepCleanupReport(
	logHelper *log.Helper,
	report commonRepCleanupReport,
	policy commonRepCleanupPolicy,
	options commonRepCleanupOptions,
	executeRequested bool,
) {
	resourceTypeFilter := "*"
	if resourceType, hasFilter := policy.ResourceType(); hasFilter {
		resourceTypeFilter = resourceType.String()
	}
	logHelper.Infow("msg", "common representation cleanup pass report",
		"execution_requested", executeRequested,
		"keep_versions", report.KeepVersions(),
		"resource_type_filter", resourceTypeFilter,
		"chunk_rows", options.ChunkRows(),
		"batch_size", options.BatchSize(),
		"maximum_batches", options.MaxBatches(),
		"batch_delay", options.BatchDelay().String(),
		"maximum_duration", options.MaxDuration().String(),
		"scan_started_at_utc", report.StartedAt().Format(time.RFC3339Nano),
		"scan_finished_at_utc", report.FinishedAt().Format(time.RFC3339Nano),
		"chunks_processed", report.ChunksProcessed(),
		"roots_scanned", report.RootsScanned(),
		"prunable_common_representations", report.PrunableRows(),
		"prunable_resource_roots", report.PrunableRoots(),
		"roots_without_active_reporter_out_of_scope", report.RootsWithoutActiveReporter(),
		"excluded_inconsistent_pointer_roots", report.ExcludedInconsistentRoots(),
		"last_cursor", report.LastCursor().String(),
		"pass_complete", report.PassComplete(),
	)
}

func logCommonRepCleanupResult(logHelper *log.Helper, result commonRepCleanupResult) {
	impact := result.Impact()
	logHelper.Infow("msg", "common representation cleanup execution result",
		"committed_batches", result.CommittedBatches(),
		"batch_limit_reached", result.BatchLimitReached(),
		"common_representations_deleted", impact.Representations(),
		"resource_roots_impacted", impact.ResourceRootsImpacted(),
		"parked_root_count", len(result.ParkedRoots()),
	)
	for _, parkedRoot := range result.ParkedRoots() {
		logHelper.Warnw("msg", "common representation cleanup root parked after repeated serialization conflicts; rerun the job to retry it",
			"resource_id", parkedRoot.String(),
		)
	}
}
