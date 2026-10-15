//go:build integration

package jobs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	resourceuc "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCommonRepCleanupPostgres(t *testing.T) {
	db, _ := setupCommonRepCleanupPostgres(t)

	t.Run("per-resource retention prunes only versions older than the retained window", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		// Active root with an explicit pointer: versions 0..4, current 4, keeps 3 and 4.
		pointerRootID := uuid.New()
		seedCommonRepRoot(t, db, pointerRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, pointerRootID, "pointer-root", "host", "hbi", false).Error)

		// Active root with a NULL pointer: current is MAX(version).
		nullPointerRootID := uuid.New()
		seedCommonRepRoot(t, db, nullPointerRootID, "host", nil, 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, nullPointerRootID, "null-pointer-root", "host", "hbi", false).Error)

		// Shared root with two active reporters: pruned like any other active root.
		sharedRootID := uuid.New()
		seedCommonRepRoot(t, db, sharedRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, sharedRootID, "shared-root-hbi", "host", "hbi", false).Error)
		require.NoError(t, insertCommonRepReporter(t, db, sharedRootID, "shared-root-ocm", "host", "ocm", false).Error)

		// Root with fewer versions than the retention window keeps everything.
		freshRootID := uuid.New()
		seedCommonRepRoot(t, db, freshRootID, "host", uintPointer(1), 0, 1)
		require.NoError(t, insertCommonRepReporter(t, db, freshRootID, "fresh-root", "host", "hbi", false).Error)

		// Root whose pointer does not resolve to an existing row is excluded, not guessed.
		inconsistentRootID := uuid.New()
		seedCommonRepRoot(t, db, inconsistentRootID, "host", uintPointer(4), 0, 1, 2, 3)
		require.NoError(t, insertCommonRepReporter(t, db, inconsistentRootID, "inconsistent-root", "host", "hbi", false).Error)

		// Root with only tombstoned reporters is out of scope: the tombstone
		// cleanup workstream owns it and no row may be touched here.
		tombstonedOnlyRootID := uuid.New()
		seedCommonRepRoot(t, db, tombstonedOnlyRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, tombstonedOnlyRootID, "tombstoned-only-root", "host", "hbi", true).Error)

		// Active root with no common history at all: not a candidate, never a failure.
		noHistoryRootID := uuid.New()
		seedCommonRepRoot(t, db, noHistoryRootID, "host", nil)
		require.NoError(t, insertCommonRepReporter(t, db, noHistoryRootID, "no-history-root", "host", "hbi", false).Error)

		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)

		report, err := usecase.Preview(context.Background(), policy, options)
		require.NoError(t, err)
		assert.Equal(t, int64(6), report.ScopedActiveRoots())
		assert.Equal(t, int64(1), report.RootsWithoutActiveReporter(), "the tombstoned-only root is out of scope")
		assert.Equal(t, int64(1), report.ExcludedInconsistentRoots())
		assert.Equal(t, int64(9), report.TotalPrunable().Representations(), "three roots with three prunable versions each")
		assert.Equal(t, int64(3), report.TotalPrunable().ResourceRootsImpacted())
		assert.Equal(t, int64(9), report.BoundedPrunable().Representations())

		// Preview must not mutate anything.
		assertCommonRepVersions(t, db, pointerRootID, []uint{0, 1, 2, 3, 4})
		assertCommonRepVersions(t, db, tombstonedOnlyRootID, []uint{0, 1, 2, 3, 4})

		// Read-only transactions must refuse deletion.
		readOnlyTransaction := newCommonRepCleanupTransaction(db, data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 3))
		readOnlyErr := readOnlyTransaction.WithinSerializableCommonRepCleanup(context.Background(), "common_rep_cleanup_read_only_enforcement_test", true, 30*time.Second, 5*time.Second, func(repository commonRepCleanupRepository) error {
			_, err := repository.DeleteCommonHistoryBatch(context.Background(), policy, 10)
			return err
		})
		require.ErrorContains(t, readOnlyErr, "read-only transaction")

		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, 1, result.CommittedBatches())
		assert.Equal(t, int64(9), result.Impact().Representations())
		assert.Equal(t, int64(3), result.Impact().ResourceRootsImpacted())

		assertCommonRepVersions(t, db, pointerRootID, []uint{3, 4}, "only the retained window survives")
		assertCommonRepVersions(t, db, nullPointerRootID, []uint{3, 4}, "a NULL pointer falls back to MAX(version)")
		assertCommonRepVersions(t, db, sharedRootID, []uint{3, 4})
		assertCommonRepVersions(t, db, freshRootID, []uint{0, 1})
		assertCommonRepVersions(t, db, inconsistentRootID, []uint{0, 1, 2, 3}, "an inconsistent pointer excludes the whole root")
		assertCommonRepVersions(t, db, tombstonedOnlyRootID, []uint{0, 1, 2, 3, 4}, "out-of-scope roots are untouched")
		assertCommonRepVersions(t, db, noHistoryRootID, nil)

		// Roots and reporter rows are never deleted.
		assertCommonRepRootCount(t, db, pointerRootID, 1)
		assertCommonRepReporterRowCount(t, db, sharedRootID, 2, "both shared-root reporters survive")
		var tombstonedReporterCount int64
		require.NoError(t, db.Table("reporter_resources").Where("resource_id = ?", tombstonedOnlyRootID).Count(&tombstonedReporterCount).Error)
		assert.Equal(t, int64(1), tombstonedReporterCount)
	})

	t.Run("keep-versions input widens the retained window", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(3, "")
		require.NoError(t, err)

		rootID := uuid.New()
		seedCommonRepRoot(t, db, rootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, rootID, "keep-three-root", "host", "hbi", false).Error)

		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.Impact().Representations())
		assertCommonRepVersions(t, db, rootID, []uint{2, 3, 4})
	})

	t.Run("resource type filter scopes the selection", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "cluster")
		require.NoError(t, err)

		clusterRootID := uuid.New()
		seedCommonRepRoot(t, db, clusterRootID, "cluster", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, clusterRootID, "cluster-root", "cluster", "hbi", false).Error)
		hostRootID := uuid.New()
		seedCommonRepRoot(t, db, hostRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, hostRootID, "host-root", "host", "hbi", false).Error)

		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)

		report, err := usecase.Preview(context.Background(), policy, options)
		require.NoError(t, err)
		assert.Equal(t, int64(1), report.ScopedActiveRoots())
		assert.Equal(t, int64(3), report.TotalPrunable().Representations())

		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.Impact().Representations())
		assertCommonRepVersions(t, db, clusterRootID, []uint{3, 4})
		assertCommonRepVersions(t, db, hostRootID, []uint{0, 1, 2, 3, 4}, "the unfiltered type is untouched")
	})

	t.Run("bounded batches commit exact accounting and rerun remaining eligibility", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		firstRootID := uuid.New()
		seedCommonRepRoot(t, db, firstRootID, "host", uintPointer(9), 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		require.NoError(t, insertCommonRepReporter(t, db, firstRootID, "batch-first-root", "host", "hbi", false).Error)
		secondRootID := uuid.New()
		seedCommonRepRoot(t, db, secondRootID, "host", uintPointer(9), 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		require.NoError(t, insertCommonRepReporter(t, db, secondRootID, "batch-second-root", "host", "hbi", false).Error)

		usecase := newTestCommonRepCleanupUsecase(t, db)
		boundedOptions, err := newCommonRepCleanupOptions(3, 0, 1, time.Minute)
		require.NoError(t, err)
		report, err := usecase.Preview(context.Background(), policy, boundedOptions)
		require.NoError(t, err)
		assert.Equal(t, int64(16), report.TotalPrunable().Representations())
		assert.Equal(t, int64(3), report.BoundedPrunable().Representations(), "the bounded projection covers one batch prefix")

		firstRun, err := usecase.Execute(context.Background(), policy, boundedOptions, nil)
		require.NoError(t, err)
		assert.Equal(t, 1, firstRun.CommittedBatches())
		assert.True(t, firstRun.BatchLimitReached())
		assert.Equal(t, int64(3), firstRun.Impact().Representations())

		rerunOptions, err := newCommonRepCleanupOptions(3, 0, 5, time.Minute)
		require.NoError(t, err)
		secondRun, err := usecase.Execute(context.Background(), policy, rerunOptions, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(13), secondRun.Impact().Representations(), "the rerun removes the remaining eligibility")
		assertCommonRepVersions(t, db, firstRootID, []uint{8, 9})
		assertCommonRepVersions(t, db, secondRootID, []uint{8, 9})
	})

	t.Run("batches target resources with the most versions first", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)

		// The low-version root is seeded first; only version-count ordering can
		// guarantee the high-version root is selected ahead of it.
		lowRootID := uuid.New()
		seedCommonRepRoot(t, db, lowRootID, "host", uintPointer(4), 0, 1, 2, 3, 4)
		require.NoError(t, insertCommonRepReporter(t, db, lowRootID, "low-version-root", "host", "hbi", false).Error)

		highRootID := uuid.New()
		seedCommonRepRoot(t, db, highRootID, "host", uintPointer(9), 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		require.NoError(t, insertCommonRepReporter(t, db, highRootID, "high-version-root", "host", "hbi", false).Error)

		usecase := newTestCommonRepCleanupUsecase(t, db)
		options, err := newCommonRepCleanupOptions(3, 0, 1, time.Minute)
		require.NoError(t, err)

		report, err := usecase.Preview(context.Background(), policy, options)
		require.NoError(t, err)
		assert.Equal(t, int64(11), report.TotalPrunable().Representations())
		assert.Equal(t, int64(3), report.BoundedPrunable().Representations())
		assert.Equal(t, int64(1), report.BoundedPrunable().ResourceRootsImpacted(), "the bounded prefix must target only the high-version root")

		result, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.Impact().Representations())
		assert.Equal(t, int64(1), result.Impact().ResourceRootsImpacted())
		assertCommonRepVersions(t, db, highRootID, []uint{3, 4, 5, 6, 7, 8, 9}, "the high-version root loses its oldest three versions")
		assertCommonRepVersions(t, db, lowRootID, []uint{0, 1, 2, 3, 4}, "the low-version root stays untouched while higher-version resources remain")
	})

	t.Run("API revival resumes version numbering and consumer reads stay available", func(t *testing.T) {
		isolateCommonRepCleanupPostgresSubtest(t, db)
		const reporterType = "commonrep"
		resourceUsecase, resourceRepository := commonRepAPIUsecase(t, db, reporterType)

		// Create the resource through the real API so the base state is API-valid.
		reportErr := resourceUsecase.ReportResource(
			commonRepAPIAuthzContext(),
			commonRepAPIReportCommand(t, reporterType, "revival-root", "report-id-initial"),
		)
		require.NoError(t, reportErr)

		var rootIDText string
		require.NoError(t, db.Raw("SELECT resource_id FROM reporter_resources WHERE local_resource_id = ?", "revival-root").Scan(&rootIDText).Error)
		rootID, err := uuid.Parse(rootIDText)
		require.NoError(t, err)
		// Advance history to five versions with the pointer at the current version.
		for version := uint(1); version <= 4; version++ {
			require.NoError(t, insertCommonRepRow(t, db, rootID, version, fmt.Sprintf("revival-txid-%d", version)).Error)
		}
		require.NoError(t, db.Exec("UPDATE resource SET common_version = 4 WHERE id = ?", rootID).Error)

		policy, err := newCommonRepCleanupPolicy(2, "")
		require.NoError(t, err)
		options, err := newCommonRepCleanupOptions(100, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := newTestCommonRepCleanupUsecase(t, db)
		firstPrune, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), firstPrune.Impact().Representations())
		assertCommonRepVersions(t, db, rootID, []uint{3, 4})

		// A new report through the API must resume numbering above the retained maximum.
		reportErr = resourceUsecase.ReportResource(
			commonRepAPIAuthzContext(),
			commonRepAPIReportCommand(t, reporterType, "revival-root", "report-id-after-prune"),
		)
		require.NoError(t, reportErr)

		key := commonRepReporterKey(t, reporterType, "revival-root")
		versionFive := bizmodel.NewVersion(5)
		current, previous, readErr := resourceRepository.FindCurrentAndPreviousVersionedRepresentations(nil, key, &versionFive, bizmodel.OperationTypeUpdated)
		require.NoError(t, readErr, "the consumer update read of v and v-1 must succeed against pruned state")
		require.NotNil(t, current)
		require.NotNil(t, previous)
		require.NotNil(t, current.CommonVersion())
		require.NotNil(t, previous.CommonVersion())
		assert.Equal(t, uint(5), current.CommonVersion().Uint())
		assert.Equal(t, uint(4), previous.CommonVersion().Uint())

		// Pruning again removes the newly superseded version while keeping v and v-1.
		secondPrune, err := usecase.Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(1), secondPrune.Impact().Representations(), "only the newly superseded version 3 is pruned")
		assertCommonRepVersions(t, db, rootID, []uint{4, 5})
	})
}

func setupCommonRepCleanupPostgres(t *testing.T) (*gorm.DB, testcontainers.Container) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	password := uuid.NewString()
	postgresContainer, err := testcontainers.Run(
		ctx,
		"postgres:17.8",
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER":     "common_rep_cleanup",
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       "common_rep_cleanup",
		}),
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(3*time.Minute)),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		if err := postgresContainer.Terminate(cleanupContext); err != nil {
			t.Errorf("terminate common representation cleanup PostgreSQL test container: %v", err)
		}
	})

	host, err := postgresContainer.Host(ctx)
	require.NoError(t, err)
	port, err := postgresContainer.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	dsn := fmt.Sprintf("host=%s port=%s user=common_rep_cleanup password=%s dbname=common_rep_cleanup sslmode=disable", host, port.Port(), password)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(16)
	require.NoError(t, data.Migrate(db, nil))
	// Migrations may bind their GORM session to a dedicated advisory-lock connection.
	require.NoError(t, sqlDB.Close())
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	sqlDB, err = db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(16)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, postgresContainer
}

func isolateCommonRepCleanupPostgresSubtest(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("TRUNCATE TABLE reporter_representations, reporter_resources, common_representations, resource CASCADE").Error)
}

func newTestCommonRepCleanupUsecase(t *testing.T, db *gorm.DB) *commonRepCleanupUsecase {
	t.Helper()
	transactionManager := data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 5)
	usecase, err := newCommonRepCleanup(newCommonRepCleanupTransaction(db, transactionManager))
	require.NoError(t, err)
	return usecase
}

func uintPointer(value uint) *uint { return &value }

// seedCommonRepRoot creates a resource root and its common representation history.
// A nil commonVersion leaves the pointer NULL, exercising the MAX(version) fallback.
func seedCommonRepRoot(t *testing.T, db *gorm.DB, resourceID uuid.UUID, resourceType string, commonVersion *uint, versions ...uint) {
	t.Helper()
	require.NoError(t, db.Exec("INSERT INTO resource (id, type, common_version) VALUES (?, ?, ?)", resourceID, resourceType, commonVersion).Error)
	for _, version := range versions {
		require.NoError(t, insertCommonRepRow(t, db, resourceID, version, fmt.Sprintf("seed-%s-%d", resourceID, version)).Error)
	}
}

func insertCommonRepRow(t *testing.T, db *gorm.DB, resourceID uuid.UUID, version uint, transactionID string) *gorm.DB {
	t.Helper()
	return db.Exec(
		`INSERT INTO common_representations (resource_id, version, data, reported_by_reporter_type, reported_by_reporter_instance, transaction_id, created_at)
		 VALUES (?, ?, '{"workspace_id": "common-rep-cleanup-test"}'::jsonb, 'hbi', 'cleanup-instance', ?, now())`,
		resourceID, version, transactionID,
	)
}

func insertCommonRepReporter(t *testing.T, db *gorm.DB, resourceID uuid.UUID, localResourceID string, resourceType string, reporterType string, tombstone bool) *gorm.DB {
	t.Helper()
	return db.Exec(
		`INSERT INTO reporter_resources (id, local_resource_id, reporter_type, resource_type, reporter_instance_id, resource_id, api_href, representation_version, generation, tombstone, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'cleanup-instance', ?, ?, 0, 0, ?, now(), now())`,
		uuid.New(), localResourceID, reporterType, resourceType, resourceID, "https://common-rep-cleanup.example.test/"+localResourceID, tombstone,
	)
}

func assertCommonRepVersions(t *testing.T, db *gorm.DB, resourceID uuid.UUID, expected []uint, message ...interface{}) {
	t.Helper()
	var versions []uint
	require.NoError(t, db.Raw("SELECT version FROM common_representations WHERE resource_id = ? ORDER BY version ASC", resourceID).Scan(&versions).Error)
	if len(message) > 0 {
		assert.Equal(t, expected, versions, "%v", message)
	} else {
		assert.Equal(t, expected, versions)
	}
}

func assertCommonRepRootCount(t *testing.T, db *gorm.DB, resourceID uuid.UUID, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM resource WHERE id = ?", resourceID).Scan(&count).Error)
	assert.Equal(t, expected, count)
}

func assertCommonRepReporterRowCount(t *testing.T, db *gorm.DB, resourceID uuid.UUID, expected int64, message ...interface{}) {
	t.Helper()
	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM reporter_resources WHERE resource_id = ?", resourceID).Scan(&count).Error)
	if len(message) > 0 {
		assert.Equal(t, expected, count, "%v", message)
	} else {
		assert.Equal(t, expected, count)
	}
}

func commonRepReporterKey(t *testing.T, reporterType string, localResourceID string) bizmodel.ReporterResourceKey {
	t.Helper()
	localID, err := bizmodel.NewLocalResourceId(localResourceID)
	require.NoError(t, err)
	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	reporterTypeValue, err := bizmodel.NewReporterType(reporterType)
	require.NoError(t, err)
	reporterInstanceID, err := bizmodel.NewReporterInstanceId("cleanup-instance")
	require.NoError(t, err)
	key, err := bizmodel.NewReporterResourceKey(localID, resourceType, reporterTypeValue, reporterInstanceID)
	require.NoError(t, err)
	return key
}

func commonRepAPIUsecase(
	t *testing.T,
	db *gorm.DB,
	reporterType string,
) (*resourceuc.Usecase, bizmodel.ResourceRepository) {
	t.Helper()
	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	reporterTypeValue, err := bizmodel.NewReporterType(reporterType)
	require.NoError(t, err)
	schemaRepository := data.NewInMemorySchemaRepository()
	commonSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema":"http://json-schema.org/draft-07/schema#",
		"type":"object",
		"properties":{"workspace_id":{"type":"string"}},
		"required":["workspace_id"]
	}`)
	reporterSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema":"http://json-schema.org/draft-07/schema#",
		"type":"object",
		"properties":{"local_resource_id":{"type":"string"}},
		"required":[]
	}`)
	resourceSchemaRepresentation, err := bizmodel.NewResourceSchemaRepresentation(resourceType, commonSchema)
	require.NoError(t, err)
	require.NoError(t, schemaRepository.CreateResourceSchema(context.Background(), resourceSchemaRepresentation))
	reporterSchemaRepresentation, err := bizmodel.NewReporterSchemaRepresentation(resourceType, reporterTypeValue, reporterSchema)
	require.NoError(t, err)
	require.NoError(t, schemaRepository.CreateReporterSchema(context.Background(), reporterSchemaRepresentation))

	metrics := metricscollector.NewFakeMetricsCollector()
	transactionManager := data.NewGormTransactionManager(metrics, 5)
	resourceRepository := data.NewResourceRepository(db, transactionManager, data.SetOutboxPublisher(storage.OutboxModeNone))
	relationsRepository := &data.AllowAllRelationsRepository{Logger: log.NewHelper(log.DefaultLogger)}
	usecase := resourceuc.New(
		resourceRepository,
		schemaRepository,
		relationsRepository,
		"common-rep-cleanup-integration",
		log.DefaultLogger,
		nil,
		nil,
		resourceuc.NewUsecaseConfig(),
		metrics,
		nil,
		nil,
	)
	return usecase, resourceRepository
}

func commonRepAPIAuthzContext() context.Context {
	return authnapi.NewAuthzContext(context.Background(), authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject: &authnapi.Claims{
			SubjectId: authnapi.SubjectId("common-rep-cleanup-integration-user"),
			AuthType:  authnapi.AuthTypeXRhIdentity,
		},
	})
}

func commonRepAPIReportCommand(
	t *testing.T,
	reporterType string,
	localResourceID string,
	transactionID string,
) resourceuc.ReportResourceCommand {
	t.Helper()
	localID, err := bizmodel.NewLocalResourceId(localResourceID)
	require.NoError(t, err)
	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	reporterTypeValue, err := bizmodel.NewReporterType(reporterType)
	require.NoError(t, err)
	reporterInstanceID, err := bizmodel.NewReporterInstanceId("cleanup-instance")
	require.NoError(t, err)
	apiHref, err := bizmodel.NewApiHref("https://common-rep-cleanup.example.test/api/" + localResourceID)
	require.NoError(t, err)
	consoleHref, err := bizmodel.NewConsoleHref("https://common-rep-cleanup.example.test/console/" + localResourceID)
	require.NoError(t, err)
	reporterRepresentation, err := bizmodel.NewRepresentation(map[string]any{"local_resource_id": localResourceID})
	require.NoError(t, err)
	commonRepresentation, err := bizmodel.NewRepresentation(map[string]any{"workspace_id": "common-rep-cleanup-api-workspace"})
	require.NoError(t, err)
	transactionIDValue := bizmodel.NewTransactionId(transactionID)
	return resourceuc.ReportResourceCommand{
		LocalResourceId:        localID,
		ResourceType:           resourceType,
		ReporterType:           reporterTypeValue,
		ReporterInstanceId:     reporterInstanceID,
		ApiHref:                apiHref,
		ConsoleHref:            &consoleHref,
		TransactionId:          &transactionIDValue,
		ReporterRepresentation: &reporterRepresentation,
		CommonRepresentation:   &commonRepresentation,
		WriteVisibility:        resourceuc.WriteVisibilityMinimizeLatency,
	}
}
