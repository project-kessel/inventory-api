package jobs

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	cleanupuc "github.com/project-kessel/inventory-api/internal/biz/usecase/cleanup"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
)

const (
	tombstoneCleanupDefaultBatchSize   = 1000
	tombstoneCleanupDefaultBatchDelay  = 500 * time.Millisecond
	tombstoneCleanupDefaultMaxBatches  = 1
	tombstoneCleanupDefaultMaxDuration = 5 * time.Minute
)

// NewTombstonedResourcesCleanupJobCommand creates a dry-run-by-default tombstone cleanup command.
func NewTombstonedResourcesCleanupJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var (
		execute                    bool
		acknowledgeDrainedConsumer bool
		tombstoneDays              int
		reporterType               string
		batchSize                  int
		batchDelay                 time.Duration
		maxBatches                 int
		maxDuration                time.Duration
	)

	command := &cobra.Command{
		Use:   "tombstoned-resources-cleanup-job",
		Short: "Preview or remove structurally valid old tombstoned resources",
		Long: `Previews old tombstoned reporter resources by default. Destructive execution
requires --execute and the explicit --acknowledge-drained-consumer attestation;
the attestation does not establish consumer drain or application quiescence.
Use only after verifying the operational preconditions in the tombstone cleanup runbook.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			signalContext, stopSignals := tombstoneCleanupSignalContext(command.Context())
			defer stopSignals()
			if err := validateTombstoneCleanupInvocation(execute, acknowledgeDrainedConsumer, tombstoneDays); err != nil {
				return err
			}
			return runTombstonedResourcesCleanupJob(
				signalContext,
				storageOptions,
				loggerOptions,
				execute,
				tombstoneDays,
				reporterType,
				batchSize,
				batchDelay,
				maxBatches,
				maxDuration,
			)
		},
	}

	flags := command.Flags()
	flags.BoolVar(&execute, "execute", false, "Commit bounded tombstone cleanup batches; default is read-only preview")
	flags.BoolVar(&acknowledgeDrainedConsumer, "acknowledge-drained-consumer", false, "Attest that delete processing is complete and relevant application operations are quiesced; this flag does not verify either condition")
	flags.IntVar(&tombstoneDays, "tombstone-days", 0, "Required positive age threshold in UTC days; tombstones exactly at the cutoff are retained")
	flags.StringVar(&reporterType, "reporter-type", "", "Optional reporter type filter")
	flags.IntVar(&batchSize, "batch-size", tombstoneCleanupDefaultBatchSize, "Maximum reporter metadata rows per transaction (default 1000)")
	flags.DurationVar(&batchDelay, "batch-delay", tombstoneCleanupDefaultBatchDelay, "Interruptible delay between committed batches (default 500ms)")
	flags.IntVar(&maxBatches, "max-batches", tombstoneCleanupDefaultMaxBatches, "Maximum number of destructive batches in one invocation (default 1)")
	flags.DurationVar(&maxDuration, "max-duration", tombstoneCleanupDefaultMaxDuration, "Maximum total invocation duration, including batch delays (default 5m)")
	_ = command.MarkFlagRequired("tombstone-days")

	return command
}

func tombstoneCleanupSignalContext(parentContext context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parentContext, os.Interrupt, syscall.SIGTERM)
}

func validateTombstoneCleanupInvocation(execute, acknowledgeDrainedConsumer bool, tombstoneDays int) error {
	if tombstoneDays <= 0 {
		return fmt.Errorf("--tombstone-days must be positive")
	}
	if execute && !acknowledgeDrainedConsumer {
		return fmt.Errorf("--execute requires --acknowledge-drained-consumer after verifying the operational preconditions")
	}
	return nil
}

func runTombstonedResourcesCleanupJob(
	parentContext context.Context,
	storageOptions *storage.Options,
	loggerOptions common.LoggerOptions,
	execute bool,
	tombstoneDays int,
	reporterType string,
	batchSize int,
	batchDelay time.Duration,
	maxBatches int,
	maxDuration time.Duration,
) error {
	if err := parentContext.Err(); err != nil {
		return fmt.Errorf("tombstone cleanup canceled before startup: %w", err)
	}
	if storageOptions == nil {
		return fmt.Errorf("storage options are required")
	}
	if storageOptions.Database != storage.Postgres {
		return fmt.Errorf("tombstone cleanup requires PostgreSQL; configured database is %q", storageOptions.Database)
	}
	options, err := cleanupuc.NewOptions(batchSize, batchDelay, maxBatches, maxDuration)
	if err != nil {
		return fmt.Errorf("invalid tombstone cleanup execution bounds: %w", err)
	}
	jobContext, cancel := context.WithTimeout(parentContext, options.MaxDuration())
	defer cancel()
	policy, err := model.NewTombstoneCleanupPolicy(time.Now().UTC(), tombstoneDays, reporterType)
	if err != nil {
		return err
	}
	if storageOptions.MaxSerializationRetries <= 0 {
		return fmt.Errorf("storage.max-serialization-retries must be positive")
	}
	if err := jobContext.Err(); err != nil {
		return fmt.Errorf("tombstone cleanup canceled before database startup: %w", err)
	}

	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "tombstoned_resources_cleanup"))
	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.NewWithContext(jobContext, storageConfig, logHelper)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get tombstone cleanup database handle: %w", err)
	}
	defer sqlDB.Close() //nolint:errcheck
	if err := jobContext.Err(); err != nil {
		return fmt.Errorf("tombstone cleanup canceled during database startup: %w", err)
	}

	metrics := &metricscollector.MetricsCollector{}
	if err := metrics.New(otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")); err != nil {
		return fmt.Errorf("initialize tombstone cleanup transaction metrics: %w", err)
	}
	transactionManager := data.NewGormTransactionManager(metrics, storageOptions.MaxSerializationRetries)
	transactions := data.NewTombstoneCleanupTransaction(db, transactionManager)
	usecase, err := cleanupuc.New(transactions)
	if err != nil {
		return err
	}

	report, err := usecase.Preview(jobContext, policy, options)
	if err != nil {
		return err
	}
	logTombstoneCleanupPreview(logHelper, report, policy, options, tombstoneDays, execute)
	if !execute {
		logHelper.Info("Tombstone cleanup preview completed; no database mutations were made")
		return nil
	}

	logHelper.Warn("Tombstone cleanup execution explicitly enabled; committed batches cannot be undone by cancellation")
	result, executeErr := usecase.Execute(jobContext, policy, options, func(batchNumber int, impact model.TombstoneCleanupImpact) {
		logHelper.Infow("msg", "tombstone cleanup batch committed",
			"batch", batchNumber,
			"reporter_entities", impact.ReporterEntities(),
			"reporter_resources_deleted", impact.ReporterResources(),
			"reporter_representations_deleted", impact.ReporterRepresentations(),
			"common_representations_deleted", impact.CommonRepresentations(),
			"resource_roots_impacted", impact.ResourceRootsImpacted(),
			"resources_removed", impact.ResourcesRemovable(),
		)
	})
	logTombstoneCleanupResult(logHelper, result)
	return executeErr
}

func logTombstoneCleanupPreview(
	logHelper *log.Helper,
	report model.TombstoneCleanupReport,
	policy model.TombstoneCleanupPolicy,
	options cleanupuc.Options,
	tombstoneDays int,
	executeRequested bool,
) {
	total := report.TotalEligible()
	bounded := report.BoundedImpact()
	exclusions := report.Exclusions()
	reporterTypeFilter := "*"
	if reporterType, hasFilter := policy.ReporterType(); hasFilter {
		reporterTypeFilter = reporterType.String()
	}
	logHelper.Infow("msg", "tombstone cleanup preview",
		"execution_requested", executeRequested,
		"tombstone_days", tombstoneDays,
		"reporter_type_filter", reporterTypeFilter,
		"batch_size", options.BatchSize(),
		"maximum_batches", options.MaxBatches(),
		"batch_delay", options.BatchDelay().String(),
		"maximum_duration", options.MaxDuration().String(),
		"evaluated_at_utc", report.EvaluatedAt().Format(time.RFC3339Nano),
		"database_snapshot_at_utc", report.SnapshotAt().Format(time.RFC3339Nano),
		"strict_cutoff_utc", report.Cutoff().Format(time.RFC3339Nano),
		"age_candidates", report.AgeCandidates(),
		"unknown_timestamp_rows", report.UnknownTimestampRows(),
		"eligible_reporter_entities", total.ReporterEntities(),
		"eligible_reporter_resources", total.ReporterResources(),
		"eligible_reporter_representations", total.ReporterRepresentations(),
		"eligible_common_representations", total.CommonRepresentations(),
		"eligible_resource_roots_impacted", total.ResourceRootsImpacted(),
		"eligible_resources_removable", total.ResourcesRemovable(),
		"bounded_reporter_row_limit", report.BoundedRowLimit(),
		"bounded_reporter_entities", bounded.ReporterEntities(),
		"bounded_reporter_resources", bounded.ReporterResources(),
		"bounded_reporter_representations", bounded.ReporterRepresentations(),
		"bounded_common_representations", bounded.CommonRepresentations(),
		"bounded_resource_roots_impacted", bounded.ResourceRootsImpacted(),
		"bounded_resources_removable", bounded.ResourcesRemovable(),
		"excluded_missing_root", exclusions.MissingRoot(),
		"excluded_root_type_mismatch", exclusions.RootTypeMismatch(),
		"excluded_invalid_reporter_key", exclusions.InvalidReporterKey(),
		"excluded_missing_terminal_representation", exclusions.MissingTerminalRepresentation(),
		"excluded_inconsistent_terminal_representation", exclusions.InconsistentTerminalRepresentation(),
		"excluded_newer_history", exclusions.NewerHistory(),
	)
}

func logTombstoneCleanupResult(logHelper *log.Helper, result cleanupuc.Result) {
	impact := result.Impact()
	logHelper.Infow("msg", "tombstone cleanup execution progress",
		"committed_batches", result.CommittedBatches(),
		"batch_limit_reached", result.BatchLimitReached(),
		"reporter_entities", impact.ReporterEntities(),
		"reporter_resources_deleted", impact.ReporterResources(),
		"reporter_representations_deleted", impact.ReporterRepresentations(),
		"common_representations_deleted", impact.CommonRepresentations(),
		"resource_roots_impacted", impact.ResourceRootsImpacted(),
		"resources_removed", impact.ResourcesRemovable(),
	)
}
