package jobs

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

const (
	DefaultTestDataResourcesCount     = 1000000 // 1M resources
	DefaultTestDataVersionsPerResource = 5       // 5 versions per resource
	DefaultTestDataBatchSize           = 1000    // Insert 1000 at a time
)

func NewTestDataGeneratorJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var resourcesCount int
	var versionsPerResource int
	var batchSize int
	var oldDataDays int
	var tombstonePercent int
	var oldTombstonePercent int

	cmd := &cobra.Command{
		Use:   "test-data-generator-job",
		Short: "Generate test data at scale for retention cleanup testing",
		Long: `Generate millions of test records with varied ages and tombstone states.

Creates realistic test data for validating retention cleanup jobs:
- Active resources with multiple versions spanning days/weeks
- Recently tombstoned resources (<30 days)
- Old tombstoned resources (>30 days)
- Resources with current versions that are old (edge case testing)

WARNING: This generates large amounts of data. Use in test/ephemeral environments only.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateTestData(storageOptions, loggerOptions, resourcesCount, versionsPerResource, batchSize, oldDataDays, tombstonePercent, oldTombstonePercent)
		},
	}

	cmd.Flags().IntVar(&resourcesCount, "resources", DefaultTestDataResourcesCount, "Number of resources to generate")
	cmd.Flags().IntVar(&versionsPerResource, "versions-per-resource", DefaultTestDataVersionsPerResource, "Average number of versions per resource")
	cmd.Flags().IntVar(&batchSize, "batch-size", DefaultTestDataBatchSize, "Batch size for inserts")
	cmd.Flags().IntVar(&oldDataDays, "old-data-days", 60, "Maximum age of oldest data in days")
	cmd.Flags().IntVar(&tombstonePercent, "tombstone-percent", 20, "Percentage of resources to tombstone (0-100)")
	cmd.Flags().IntVar(&oldTombstonePercent, "old-tombstone-percent", 50, "Percentage of tombstoned resources that are >30 days old (0-100)")

	return cmd
}

func generateTestData(storageOptions *storage.Options, loggerOptions common.LoggerOptions, resourcesCount int, versionsPerResource int, batchSize int, oldDataDays int, tombstonePercent int, oldTombstonePercent int) error {
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "test_data_generator"))

	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.New(storageConfig, logHelper)
	if err != nil {
		return err
	}

	logHelper.Infof("Starting test data generation")
	logHelper.Infof("Resources: %d", resourcesCount)
	logHelper.Infof("Versions per resource: ~%d", versionsPerResource)
	logHelper.Infof("Old data days: %d", oldDataDays)
	logHelper.Infof("Tombstone percent: %d%%", tombstonePercent)
	logHelper.Infof("Old tombstone percent: %d%%", oldTombstonePercent)

	totalRecords := resourcesCount * versionsPerResource
	logHelper.Infof("Estimated total records: ~%d", totalRecords)

	startTime := time.Now()

	// Generate resources in batches
	createdResources := 0
	createdReporterResources := 0
	createdReporterRepresentations := 0
	createdCommonRepresentations := 0

	for i := 0; i < resourcesCount; i += batchSize {
		end := i + batchSize
		if end > resourcesCount {
			end = resourcesCount
		}

		err := generateResourceBatch(db, logHelper, i, end, resourcesCount, versionsPerResource, oldDataDays, tombstonePercent, oldTombstonePercent,
			&createdResources, &createdReporterResources, &createdReporterRepresentations, &createdCommonRepresentations)
		if err != nil {
			return fmt.Errorf("failed to generate batch %d-%d: %w", i, end, err)
		}

		if (i+batchSize)%(batchSize*10) == 0 {
			elapsed := time.Since(startTime)
			rate := float64(createdResources) / elapsed.Seconds()
			remaining := resourcesCount - createdResources
			estimatedRemaining := time.Duration(float64(remaining)/rate) * time.Second

			logHelper.Infof("Progress: %d/%d resources (%.1f%%) - %.0f resources/sec - ETA: %s",
				createdResources, resourcesCount, float64(createdResources)/float64(resourcesCount)*100, rate, estimatedRemaining)
		}
	}

	elapsed := time.Since(startTime)
	logHelper.Infof("Test data generation completed in %s", elapsed)
	logHelper.Infof("Created:")
	logHelper.Infof("  Resources: %d", createdResources)
	logHelper.Infof("  Reporter resources: %d", createdReporterResources)
	logHelper.Infof("  Reporter representations: %d", createdReporterRepresentations)
	logHelper.Infof("  Common representations: %d", createdCommonRepresentations)
	logHelper.Infof("Rate: %.0f resources/sec", float64(createdResources)/elapsed.Seconds())

	return nil
}

func generateResourceBatch(db *gorm.DB, logHelper *log.Helper, start int, end int, resourcesCount int, versionsPerResource int, oldDataDays int,
	tombstonePercent int, oldTombstonePercent int,
	createdResources *int, createdReporterResources *int, createdReporterRepresentations *int, createdCommonRepresentations *int) error {

	now := time.Now()
	batchSize := end - start

	// Pre-allocate slices for batch inserts (using pointers to match New* return types)
	resources := make([]model.Resource, 0, batchSize)
	reporterResources := make([]*model.ReporterResource, 0, batchSize)
	reporterRepresentations := make([]*model.ReporterRepresentation, 0, batchSize*versionsPerResource)
	commonRepresentations := make([]*model.CommonRepresentation, 0, batchSize*versionsPerResource)

	// Build all records in memory first
	for i := start; i < end; i++ {
		// Create resource
		resourceID := uuid.New()
		resource := model.Resource{
			ID:   resourceID,
			Type: "host",
		}
		resources = append(resources, resource)

		// Determine tombstone state
		isTombstoned := (i*100)/resourcesCount < tombstonePercent
		var tombstoneAge time.Duration
		if isTombstoned {
			isOldTombstone := (i*100)/(resourcesCount*tombstonePercent/100) < oldTombstonePercent
			if isOldTombstone {
				// Tombstoned >30 days ago (30-60 days)
				tombstoneAge = time.Duration(30+i%30) * 24 * time.Hour
			} else {
				// Tombstoned <30 days ago (1-29 days)
				tombstoneAge = time.Duration(1+i%29) * 24 * time.Hour
			}
		}

		// Create reporter_resource
		reporterResourceID := uuid.New()
		consoleHref := fmt.Sprintf("https://console.example.com/hosts/%d", i)
		reporterResource, err := model.NewReporterResource(
			reporterResourceID,
			fmt.Sprintf("test-resource-%d", i), // localResourceID
			"hbi",                                // reporterType
			"host",                               // resourceType
			"test-instance",                      // reporterInstanceID
			resourceID,                           // resourceID
			fmt.Sprintf("https://api.example.com/hosts/%d", i), // apiHref
			&consoleHref, // consoleHref
			0,            // representationVersion
			0,            // generation
			isTombstoned, // tombstone
		)
		if err != nil {
			return fmt.Errorf("failed to create reporter_resource: %w", err)
		}

		// For tombstoned resources, we need to set the updated_at timestamp manually
		if isTombstoned {
			tombstonedAt := now.Add(-tombstoneAge)
			reporterResource.CreatedAt = tombstonedAt
			reporterResource.UpdatedAt = tombstonedAt
		}
		reporterResources = append(reporterResources, reporterResource)

		// Create multiple versions (representations) with varied ages
		numVersions := versionsPerResource + (i % 3) - 1 // Vary 4-6 versions
		if numVersions < 1 {
			numVersions = 1
		}

		for v := 0; v < numVersions; v++ {
			// Calculate age for this version
			// Latest version (v == numVersions-1) is recent
			// Older versions spread across oldDataDays
			var versionAge time.Duration
			if v == numVersions-1 {
				// Latest version: 0-2 days old
				versionAge = time.Duration(i%3) * 24 * time.Hour
			} else {
				// Older versions: spread across oldDataDays
				dayOffset := (oldDataDays * v) / numVersions
				versionAge = time.Duration(dayOffset) * 24 * time.Hour
			}

			versionTime := now.Add(-versionAge)

			// Create reporter_representation
			reporterRep, err := model.NewReporterRepresentation(
				internal.JsonObject{
					"satellite_id":           fmt.Sprintf("sat-%d", i),
					"subscription_manager_id": fmt.Sprintf("sub-%d", i),
					"insights_id":            fmt.Sprintf("insights-%d", i),
					"ansible_host":           fmt.Sprintf("host-%d", i),
				},
				reporterResourceID,
				uint(v),
				0,
				nil,
				"",
				isTombstoned,
				nil,
			)
			if err != nil {
				return fmt.Errorf("failed to create reporter representation: %w", err)
			}
			reporterRep.CreatedAt = versionTime
			reporterRepresentations = append(reporterRepresentations, reporterRep)

			// Create common_representation
			commonRep, err := model.NewCommonRepresentation(
				resourceID,
				internal.JsonObject{
					"workspace_id": fmt.Sprintf("workspace-%d", i%100),
					"org_id":       fmt.Sprintf("org-%d", i%10),
				},
				uint(v),
				"hbi",
				"test-instance",
				"",
			)
			if err != nil {
				return fmt.Errorf("failed to create common representation: %w", err)
			}
			commonRep.CreatedAt = versionTime
			commonRepresentations = append(commonRepresentations, commonRep)
		}
	}

	// Perform batch inserts in dependency order
	// 1. Insert resources first (no dependencies)
	if len(resources) > 0 {
		if err := db.CreateInBatches(resources, 1000).Error; err != nil {
			return fmt.Errorf("failed to batch insert resources: %w", err)
		}
		*createdResources += len(resources)
	}

	// 2. Insert reporter_resources (depends on resources)
	if len(reporterResources) > 0 {
		if err := db.CreateInBatches(reporterResources, 1000).Error; err != nil {
			return fmt.Errorf("failed to batch insert reporter_resources: %w", err)
		}
		*createdReporterResources += len(reporterResources)
	}

	// 3. Insert representations (depend on reporter_resources and resources)
	if len(reporterRepresentations) > 0 {
		if err := db.CreateInBatches(reporterRepresentations, 1000).Error; err != nil {
			return fmt.Errorf("failed to batch insert reporter_representations: %w", err)
		}
		*createdReporterRepresentations += len(reporterRepresentations)
	}

	if len(commonRepresentations) > 0 {
		if err := db.CreateInBatches(commonRepresentations, 1000).Error; err != nil {
			return fmt.Errorf("failed to batch insert common_representations: %w", err)
		}
		*createdCommonRepresentations += len(commonRepresentations)
	}

	return nil
}
