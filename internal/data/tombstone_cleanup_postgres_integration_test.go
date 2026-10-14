//go:build integration

package data_test

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	mobycontainer "github.com/moby/moby/api/types/container"
	"github.com/project-kessel/inventory-api/internal"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	cleanupuc "github.com/project-kessel/inventory-api/internal/biz/usecase/cleanup"
	resourceuc "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/data"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestTombstoneCleanupOperatorIdentitiesPostgres(t *testing.T) {
	db, postgresContainer := setupTombstoneCleanupPostgres(t)
	evaluationTime := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	policy, err := bizmodel.NewTombstoneCleanupPolicy(evaluationTime, 7, " HBI ")
	require.NoError(t, err)
	cutoff := policy.Cutoff()

	firstRootID := uuid.New()
	tombstoneCleanupSeedRoot(t, db, firstRootID, "host", 1)
	firstReporterID := tombstoneCleanupInsertReporter(t, db, firstRootID, "operator-first", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-2*time.Microsecond), []tombstoneCleanupHistory{
		{version: 1, tombstone: true, transactionID: "operator-delete-first"},
	})
	sharedRootID := uuid.New()
	tombstoneCleanupSeedRoot(t, db, sharedRootID, "host", 1, 2)
	sharedReporterID := tombstoneCleanupInsertReporter(t, db, sharedRootID, "operator-shared", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Microsecond), []tombstoneCleanupHistory{
		{version: 1, tombstone: true, transactionID: "operator-delete-shared"},
	})
	tombstoneCleanupInsertReporter(t, db, sharedRootID, "operator-shared-active", "host", "hbi", "cleanup-instance", false, 1, 0, evaluationTime, nil)
	filteredOutRootID := uuid.New()
	tombstoneCleanupSeedRoot(t, db, filteredOutRootID, "host", 1)
	filteredOutReporterID := tombstoneCleanupInsertReporter(t, db, filteredOutRootID, "operator-filtered-out", "host", "ocm", "cleanup-instance", true, 1, 0, cutoff.Add(-3*time.Microsecond), []tombstoneCleanupHistory{
		{version: 1, tombstone: true, transactionID: "operator-delete-filtered-out"},
	})
	invalidRootID := uuid.New()
	tombstoneCleanupSeedRoot(t, db, invalidRootID, "host", 1)
	invalidReporterID := tombstoneCleanupInsertReporter(t, db, invalidRootID, "", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
		{version: 1, tombstone: true, transactionID: "operator-invalid-key"},
	})
	missingRootReporterID := tombstoneCleanupInsertReporter(t, db, uuid.New(), "operator-missing-root", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
		{version: 1, tombstone: true, transactionID: "operator-missing-root"},
	})
	mismatchedRootID := uuid.New()
	tombstoneCleanupSeedRoot(t, db, mismatchedRootID, "cluster", 1)
	mismatchedReporterID := tombstoneCleanupInsertReporter(t, db, mismatchedRootID, "operator-root-mismatch", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
		{version: 1, tombstone: true, transactionID: "operator-root-mismatch"},
	})
	missingTerminalRootID := uuid.New()
	tombstoneCleanupSeedRoot(t, db, missingTerminalRootID, "host", 1)
	missingTerminalReporterID := tombstoneCleanupInsertReporter(t, db, missingTerminalRootID, "operator-missing-terminal", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), nil)
	inconsistentRootID := uuid.New()
	tombstoneCleanupSeedRoot(t, db, inconsistentRootID, "host", 1)
	inconsistentReporterID := tombstoneCleanupInsertReporter(t, db, inconsistentRootID, "operator-inconsistent-terminal", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
		{version: 1, tombstone: false, transactionID: "operator-inconsistent-terminal"},
	})
	newerHistoryRootID := uuid.New()
	tombstoneCleanupSeedRoot(t, db, newerHistoryRootID, "host", 1)
	newerHistoryReporterID := tombstoneCleanupInsertReporter(t, db, newerHistoryRootID, "operator-newer-history", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
		{version: 1, tombstone: true, transactionID: "operator-newer-history"},
		{version: 2, tombstone: false, transactionID: "operator-after-pointer"},
	})
	recentRootID := uuid.New()
	tombstoneCleanupSeedRoot(t, db, recentRootID, "host", 1)
	recentReporterID := tombstoneCleanupInsertReporter(t, db, recentRootID, "operator-at-cutoff", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff, []tombstoneCleanupHistory{
		{version: 1, tombstone: true, transactionID: "operator-at-cutoff"},
	})
	unknownRootID := uuid.New()
	tombstoneCleanupSeedRoot(t, db, unknownRootID, "host", 1)
	unknownReporterID := tombstoneCleanupInsertReporter(t, db, unknownRootID, "operator-null-time", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
		{version: 1, tombstone: true, transactionID: "operator-null-time"},
	})
	require.NoError(t, db.Exec("UPDATE reporter_resources SET updated_at = NULL WHERE id = ?", unknownReporterID).Error)

	options, err := cleanupuc.NewOptions(1, 0, 1, time.Minute)
	require.NoError(t, err)
	usecase := tombstoneCleanupUsecase(t, db)
	report, err := usecase.Preview(context.Background(), policy, options)
	require.NoError(t, err)
	assert.Equal(t, int64(8), report.AgeCandidates())
	assert.Equal(t, int64(1), report.UnknownTimestampRows())
	assert.Equal(t, int64(2), report.TotalEligible().ReporterResources())
	assert.Equal(t, int64(1), report.TotalEligible().ResourcesRemovable())
	assert.Equal(t, int64(1), report.Exclusions().InvalidReporterKey())
	assert.Equal(t, int64(1), report.Exclusions().MissingRoot())
	assert.Equal(t, int64(1), report.Exclusions().RootTypeMismatch())
	assert.Equal(t, int64(1), report.Exclusions().MissingTerminalRepresentation())
	assert.Equal(t, int64(1), report.Exclusions().InconsistentTerminalRepresentation())
	assert.Equal(t, int64(1), report.Exclusions().NewerHistory())

	identityRows := runTombstoneCleanupIdentitySQL(t, postgresContainer, report, "hbi")
	summary := findTombstoneCleanupIdentityRow(t, identityRows, "summary")
	assertTombstoneCleanupIdentitySummaryMatchesReport(t, summary, report)
	assert.Equal(t, "hbi", summary["reporter_type_filter"])

	rowsByReporterID := make(map[string]map[string]string)
	for _, row := range identityRows {
		if row["record_type"] == "age_candidate" || row["record_type"] == "unknown_timestamp" {
			rowsByReporterID[row["reporter_resource_id"]] = row
		}
	}
	require.Len(t, rowsByReporterID, 9)
	first := rowsByReporterID[firstReporterID.String()]
	require.NotNil(t, first)
	assert.Equal(t, firstRootID.String(), first["resource_id"])
	assert.Equal(t, "1", first["representation_version"])
	assert.Equal(t, "0", first["generation"])
	assert.Equal(t, "1", first["eligible_position"])
	assert.Equal(t, "true", first["within_bounded_prefix"])
	firstAgeSeconds, err := strconv.ParseFloat(first["age_seconds"], 64)
	require.NoError(t, err)
	assert.InDelta(t, 7*24*60*60+0.000002, firstAgeSeconds, 0.0000001)

	shared := rowsByReporterID[sharedReporterID.String()]
	require.NotNil(t, shared)
	assert.Equal(t, sharedRootID.String(), shared["resource_id"])
	assert.Equal(t, "2", shared["eligible_position"])
	assert.Equal(t, "false", shared["within_bounded_prefix"])
	assert.Empty(t, shared["exclusion_reason"])

	invalid := rowsByReporterID[invalidReporterID.String()]
	require.NotNil(t, invalid)
	assert.Equal(t, invalidRootID.String(), invalid["resource_id"])
	assert.Equal(t, "invalid_reporter_key", invalid["exclusion_reason"])
	assert.Empty(t, invalid["eligible_position"])
	assert.Equal(t, "false", invalid["within_bounded_prefix"])
	for reporterID, reason := range map[uuid.UUID]string{
		missingRootReporterID:     "missing_root",
		mismatchedReporterID:      "root_type_mismatch",
		missingTerminalReporterID: "missing_terminal_representation",
		inconsistentReporterID:    "inconsistent_terminal_representation",
		newerHistoryReporterID:    "newer_history",
	} {
		row := rowsByReporterID[reporterID.String()]
		require.NotNil(t, row)
		assert.Equal(t, reason, row["exclusion_reason"])
		assert.Empty(t, row["eligible_position"])
		assert.Equal(t, "false", row["within_bounded_prefix"])
	}

	unknown := rowsByReporterID[unknownReporterID.String()]
	require.NotNil(t, unknown)
	assert.Equal(t, "unknown_timestamp", unknown["record_type"])
	assert.Empty(t, unknown["updated_at"])
	assert.Equal(t, "missing_updated_at", unknown["exclusion_reason"])
	assert.Equal(t, "false", unknown["within_bounded_prefix"])
	_, filteredOutIncluded := rowsByReporterID[filteredOutReporterID.String()]
	assert.False(t, filteredOutIncluded, "normalized reporter filter must apply before prefix ranking")
	_, recentIncluded := rowsByReporterID[recentReporterID.String()]
	assert.False(t, recentIncluded, "equality at the cutoff is retained")
	for _, column := range []string{"data", "ktn", "transaction_id", "local_resource_id", "reporter_instance_id"} {
		_, included := summary[column]
		assert.False(t, included, "identity artifact must not disclose %s", column)
	}

	unfilteredPolicy, err := bizmodel.NewTombstoneCleanupPolicy(evaluationTime, 7, "")
	require.NoError(t, err)
	unfilteredReport, err := usecase.Preview(context.Background(), unfilteredPolicy, options)
	require.NoError(t, err)
	unfilteredRows := runTombstoneCleanupIdentitySQL(t, postgresContainer, unfilteredReport, "")
	unfilteredSummary := findTombstoneCleanupIdentityRow(t, unfilteredRows, "summary")
	assertTombstoneCleanupIdentitySummaryMatchesReport(t, unfilteredSummary, unfilteredReport)
	assert.Empty(t, unfilteredSummary["reporter_type_filter"])
	unfilteredRowsByReporterID := make(map[string]map[string]string)
	for _, row := range unfilteredRows {
		if row["record_type"] == "age_candidate" || row["record_type"] == "unknown_timestamp" {
			unfilteredRowsByReporterID[row["reporter_resource_id"]] = row
		}
	}
	assert.Equal(t, int64(9), unfilteredReport.AgeCandidates())
	assert.Equal(t, int64(3), unfilteredReport.TotalEligible().ReporterResources())
	filteredOut := unfilteredRowsByReporterID[filteredOutReporterID.String()]
	require.NotNil(t, filteredOut)
	assert.Equal(t, "1", filteredOut["eligible_position"], "the unfiltered candidate joins the same deterministic prefix")
	assert.Equal(t, "true", filteredOut["within_bounded_prefix"])
	assert.Equal(t, "2", unfilteredRowsByReporterID[firstReporterID.String()]["eligible_position"])
	assert.Equal(t, "3", unfilteredRowsByReporterID[sharedReporterID.String()]["eligible_position"])
}

func TestTombstoneCleanupPostgres(t *testing.T) {
	db, _ := setupTombstoneCleanupPostgres(t)

	t.Run("preview protects structural anomalies and delete removes only safe touched roots", func(t *testing.T) {
		isolateTombstoneCleanupPostgresSubtest(t, db)
		evaluationTime := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
		policy, err := bizmodel.NewTombstoneCleanupPolicy(evaluationTime, 7, "")
		require.NoError(t, err)
		cutoff := policy.Cutoff()

		eligibleRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, eligibleRootID, "host", 1, 2)
		eligibleReporterID := tombstoneCleanupInsertReporter(t, db, eligibleRootID, "eligible-alone", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-2*time.Microsecond), []tombstoneCleanupHistory{
			{version: 0, transactionID: "prior-history-eligible"},
			{version: 1, tombstone: true, transactionID: "delete-id-expiry-eligible"},
		})

		sharedRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, sharedRootID, "host", 1, 2)
		sharedTombstoneID := tombstoneCleanupInsertReporter(t, db, sharedRootID, "eligible-shared", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Microsecond), []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: "delete-id-expiry-shared"},
		})
		sharedActiveID := tombstoneCleanupInsertReporter(t, db, sharedRootID, "active-shared", "host", "ocm", "cleanup-instance", false, 1, 0, evaluationTime.Add(-24*time.Hour), nil)

		boundaryRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, boundaryRootID, "host", 1)
		boundaryReporterID := tombstoneCleanupInsertReporter(t, db, boundaryRootID, "at-cutoff", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff, []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: "delete-id-at-cutoff"},
		})
		recentRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, recentRootID, "host", 1)
		recentReporterID := tombstoneCleanupInsertReporter(t, db, recentRootID, "recent", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(time.Microsecond), []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: "delete-id-recent"},
		})
		activeRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, activeRootID, "host", 1)
		activeReporterID := tombstoneCleanupInsertReporter(t, db, activeRootID, "active-old", "host", "hbi", "cleanup-instance", false, 1, 0, cutoff.Add(-time.Hour), nil)
		orphanRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, orphanRootID, "host", 1)

		missingRootResourceID := uuid.New()
		missingRootReporterID := tombstoneCleanupInsertReporter(t, db, missingRootResourceID, "missing-root", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: "delete-id-missing-root"},
		})
		mismatchedRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, mismatchedRootID, "cluster", 1)
		mismatchedReporterID := tombstoneCleanupInsertReporter(t, db, mismatchedRootID, "mismatched-root", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: "delete-id-type-mismatch"},
		})
		invalidKeyRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, invalidKeyRootID, "host", 1)
		invalidKeyReporterID := tombstoneCleanupInsertReporter(t, db, invalidKeyRootID, "", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: "delete-id-invalid-key"},
		})
		missingTerminalRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, missingTerminalRootID, "host", 1)
		missingTerminalReporterID := tombstoneCleanupInsertReporter(t, db, missingTerminalRootID, "missing-terminal", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), nil)
		inconsistentRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, inconsistentRootID, "host", 1)
		inconsistentReporterID := tombstoneCleanupInsertReporter(t, db, inconsistentRootID, "inconsistent-terminal", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
			{version: 1, tombstone: false, transactionID: "live-current-for-tombstone-metadata"},
		})
		newerHistoryRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, newerHistoryRootID, "host", 1)
		newerHistoryReporterID := tombstoneCleanupInsertReporter(t, db, newerHistoryRootID, "newer-history", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: "delete-id-with-newer-history"},
			{version: 2, tombstone: false, transactionID: "later-history"},
		})
		unknownTimestampRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, unknownTimestampRootID, "host", 1)
		unknownTimestampReporterID := tombstoneCleanupInsertReporter(t, db, unknownTimestampRootID, "unknown-timestamp", "host", "hbi", "cleanup-instance", true, 1, 0, cutoff.Add(-time.Hour), []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: "delete-id-unknown-timestamp"},
		})
		require.NoError(t, db.Exec("UPDATE reporter_resources SET updated_at = NULL WHERE id = ?", unknownTimestampReporterID).Error)

		options, err := cleanupuc.NewOptions(1, 0, 1, time.Minute)
		require.NoError(t, err)
		usecase := tombstoneCleanupUsecase(t, db)
		report, err := usecase.Preview(context.Background(), policy, options)
		require.NoError(t, err)
		assert.Equal(t, int64(8), report.AgeCandidates())
		assert.Equal(t, int64(1), report.UnknownTimestampRows())
		assert.Equal(t, int64(2), report.TotalEligible().ReporterEntities())
		assert.Equal(t, int64(2), report.TotalEligible().ReporterResources())
		assert.Equal(t, int64(3), report.TotalEligible().ReporterRepresentations())
		assert.Equal(t, int64(2), report.TotalEligible().CommonRepresentations())
		assert.Equal(t, int64(2), report.TotalEligible().ResourceRootsImpacted())
		assert.Equal(t, int64(1), report.TotalEligible().ResourcesRemovable())
		assert.Equal(t, int64(1), report.BoundedImpact().ReporterResources())
		assert.Equal(t, int64(1), report.BoundedImpact().ResourceRootsImpacted())
		assert.Equal(t, int64(1), report.Exclusions().MissingRoot())
		assert.Equal(t, int64(1), report.Exclusions().RootTypeMismatch())
		assert.Equal(t, int64(1), report.Exclusions().InvalidReporterKey())
		assert.Equal(t, int64(1), report.Exclusions().MissingTerminalRepresentation())
		assert.Equal(t, int64(1), report.Exclusions().InconsistentTerminalRepresentation())
		assert.Equal(t, int64(1), report.Exclusions().NewerHistory())

		readOnlyTransaction := data.NewTombstoneCleanupTransaction(db, data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 3))
		readOnlyErr := readOnlyTransaction.WithinSerializableTombstoneCleanup(context.Background(), "tombstone_cleanup_read_only_enforcement_test", true, 30*time.Second, 5*time.Second, func(repository bizmodel.TombstoneCleanupRepository) error {
			_, err := repository.DeleteTombstoneBatch(context.Background(), policy, 1)
			return err
		})
		require.ErrorContains(t, readOnlyErr, "read-only transaction")
		assertTombstoneCleanupReporterCount(t, db, eligibleReporterID, 1)
		assertTombstoneCleanupHistoryCount(t, db, eligibleReporterID, 2)
		assertTombstoneCleanupRootCount(t, db, eligibleRootID, 1)
		assertTombstoneCleanupCommonCount(t, db, eligibleRootID, 2)

		executeOptions, err := cleanupuc.NewOptions(2, 0, 1, time.Minute)
		require.NoError(t, err)
		result, err := usecase.Execute(context.Background(), policy, executeOptions, nil)
		require.NoError(t, err)
		assert.Equal(t, 1, result.CommittedBatches())
		assert.True(t, result.BatchLimitReached())
		assert.Equal(t, int64(2), result.Impact().ReporterResources())
		assert.Equal(t, int64(3), result.Impact().ReporterRepresentations())
		assert.Equal(t, int64(2), result.Impact().CommonRepresentations())
		assert.Equal(t, int64(2), result.Impact().ResourceRootsImpacted())
		assert.Equal(t, int64(1), result.Impact().ResourcesRemovable())

		assertTombstoneCleanupResourceSnapshot(t, db, []tombstoneCleanupResourceState{
			{ID: sharedRootID, Type: "host"},
			{ID: boundaryRootID, Type: "host"},
			{ID: recentRootID, Type: "host"},
			{ID: activeRootID, Type: "host"},
			{ID: orphanRootID, Type: "host"},
			{ID: mismatchedRootID, Type: "cluster"},
			{ID: invalidKeyRootID, Type: "host"},
			{ID: missingTerminalRootID, Type: "host"},
			{ID: inconsistentRootID, Type: "host"},
			{ID: newerHistoryRootID, Type: "host"},
			{ID: unknownTimestampRootID, Type: "host"},
		})
		assertTombstoneCleanupReporterResourceSnapshot(t, db, []tombstoneCleanupReporterResourceState{
			{ID: sharedActiveID, ResourceID: sharedRootID, LocalResourceID: "active-shared", ReporterType: "ocm", ResourceType: "host", ReporterInstanceID: "cleanup-instance", RepresentationVersion: 1, Generation: 0, Tombstone: false},
			{ID: boundaryReporterID, ResourceID: boundaryRootID, LocalResourceID: "at-cutoff", ReporterType: "hbi", ResourceType: "host", ReporterInstanceID: "cleanup-instance", RepresentationVersion: 1, Generation: 0, Tombstone: true},
			{ID: recentReporterID, ResourceID: recentRootID, LocalResourceID: "recent", ReporterType: "hbi", ResourceType: "host", ReporterInstanceID: "cleanup-instance", RepresentationVersion: 1, Generation: 0, Tombstone: true},
			{ID: activeReporterID, ResourceID: activeRootID, LocalResourceID: "active-old", ReporterType: "hbi", ResourceType: "host", ReporterInstanceID: "cleanup-instance", RepresentationVersion: 1, Generation: 0, Tombstone: false},
			{ID: missingRootReporterID, ResourceID: missingRootResourceID, LocalResourceID: "missing-root", ReporterType: "hbi", ResourceType: "host", ReporterInstanceID: "cleanup-instance", RepresentationVersion: 1, Generation: 0, Tombstone: true},
			{ID: mismatchedReporterID, ResourceID: mismatchedRootID, LocalResourceID: "mismatched-root", ReporterType: "hbi", ResourceType: "host", ReporterInstanceID: "cleanup-instance", RepresentationVersion: 1, Generation: 0, Tombstone: true},
			{ID: invalidKeyReporterID, ResourceID: invalidKeyRootID, LocalResourceID: "", ReporterType: "hbi", ResourceType: "host", ReporterInstanceID: "cleanup-instance", RepresentationVersion: 1, Generation: 0, Tombstone: true},
			{ID: missingTerminalReporterID, ResourceID: missingTerminalRootID, LocalResourceID: "missing-terminal", ReporterType: "hbi", ResourceType: "host", ReporterInstanceID: "cleanup-instance", RepresentationVersion: 1, Generation: 0, Tombstone: true},
			{ID: inconsistentReporterID, ResourceID: inconsistentRootID, LocalResourceID: "inconsistent-terminal", ReporterType: "hbi", ResourceType: "host", ReporterInstanceID: "cleanup-instance", RepresentationVersion: 1, Generation: 0, Tombstone: true},
			{ID: newerHistoryReporterID, ResourceID: newerHistoryRootID, LocalResourceID: "newer-history", ReporterType: "hbi", ResourceType: "host", ReporterInstanceID: "cleanup-instance", RepresentationVersion: 1, Generation: 0, Tombstone: true},
			{ID: unknownTimestampReporterID, ResourceID: unknownTimestampRootID, LocalResourceID: "unknown-timestamp", ReporterType: "hbi", ResourceType: "host", ReporterInstanceID: "cleanup-instance", RepresentationVersion: 1, Generation: 0, Tombstone: true},
		})
		assertTombstoneCleanupReporterHistorySnapshot(t, db, []tombstoneCleanupReporterHistoryState{
			{ReporterResourceID: boundaryReporterID, Version: 1, Generation: 0, Tombstone: true, TransactionID: "delete-id-at-cutoff"},
			{ReporterResourceID: recentReporterID, Version: 1, Generation: 0, Tombstone: true, TransactionID: "delete-id-recent"},
			{ReporterResourceID: missingRootReporterID, Version: 1, Generation: 0, Tombstone: true, TransactionID: "delete-id-missing-root"},
			{ReporterResourceID: mismatchedReporterID, Version: 1, Generation: 0, Tombstone: true, TransactionID: "delete-id-type-mismatch"},
			{ReporterResourceID: invalidKeyReporterID, Version: 1, Generation: 0, Tombstone: true, TransactionID: "delete-id-invalid-key"},
			{ReporterResourceID: inconsistentReporterID, Version: 1, Generation: 0, Tombstone: false, TransactionID: "live-current-for-tombstone-metadata"},
			{ReporterResourceID: newerHistoryReporterID, Version: 1, Generation: 0, Tombstone: true, TransactionID: "delete-id-with-newer-history"},
			{ReporterResourceID: newerHistoryReporterID, Version: 2, Generation: 0, Tombstone: false, TransactionID: "later-history"},
			{ReporterResourceID: unknownTimestampReporterID, Version: 1, Generation: 0, Tombstone: true, TransactionID: "delete-id-unknown-timestamp"},
		})
		assertTombstoneCleanupCommonHistorySnapshot(t, db, []tombstoneCleanupCommonHistoryState{
			{ResourceID: sharedRootID, Version: 1, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", sharedRootID, 1)},
			{ResourceID: sharedRootID, Version: 2, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", sharedRootID, 2)},
			{ResourceID: boundaryRootID, Version: 1, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", boundaryRootID, 1)},
			{ResourceID: recentRootID, Version: 1, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", recentRootID, 1)},
			{ResourceID: activeRootID, Version: 1, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", activeRootID, 1)},
			{ResourceID: orphanRootID, Version: 1, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", orphanRootID, 1)},
			{ResourceID: mismatchedRootID, Version: 1, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", mismatchedRootID, 1)},
			{ResourceID: invalidKeyRootID, Version: 1, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", invalidKeyRootID, 1)},
			{ResourceID: missingTerminalRootID, Version: 1, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", missingTerminalRootID, 1)},
			{ResourceID: inconsistentRootID, Version: 1, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", inconsistentRootID, 1)},
			{ResourceID: newerHistoryRootID, Version: 1, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", newerHistoryRootID, 1)},
			{ResourceID: unknownTimestampRootID, Version: 1, TransactionID: fmt.Sprintf("cleanup-common-%s-%d", unknownTimestampRootID, 1)},
		})

		assertTombstoneCleanupReporterCount(t, db, eligibleReporterID, 0)
		assertTombstoneCleanupReporterCount(t, db, sharedTombstoneID, 0)
		assertTombstoneCleanupReporterCount(t, db, sharedActiveID, 1)
		assertTombstoneCleanupReporterCount(t, db, boundaryReporterID, 1)
		assertTombstoneCleanupReporterCount(t, db, recentReporterID, 1)
		assertTombstoneCleanupReporterCount(t, db, activeReporterID, 1)
		assertTombstoneCleanupReporterCount(t, db, missingRootReporterID, 1)
		assertTombstoneCleanupReporterCount(t, db, mismatchedReporterID, 1)
		assertTombstoneCleanupReporterCount(t, db, invalidKeyReporterID, 1)
		assertTombstoneCleanupReporterCount(t, db, missingTerminalReporterID, 1)
		assertTombstoneCleanupReporterCount(t, db, inconsistentReporterID, 1)
		assertTombstoneCleanupReporterCount(t, db, newerHistoryReporterID, 1)
		assertTombstoneCleanupReporterCount(t, db, unknownTimestampReporterID, 1)
		assertTombstoneCleanupRootCount(t, db, eligibleRootID, 0)
		assertTombstoneCleanupRootCount(t, db, sharedRootID, 1)
		assertTombstoneCleanupRootCount(t, db, orphanRootID, 1)
		assertTombstoneCleanupRootCount(t, db, unknownTimestampRootID, 1)
		assertTombstoneCleanupCommonCount(t, db, eligibleRootID, 0)
		assertTombstoneCleanupCommonCount(t, db, sharedRootID, 2)
		assertTombstoneCleanupCommonCount(t, db, orphanRootID, 1)
		assertTombstoneCleanupCommonCount(t, db, unknownTimestampRootID, 1)
		assertTombstoneCleanupHistoryTransactionCount(t, db, "delete-id-expiry-eligible", 0)
		assertTombstoneCleanupHistoryTransactionCount(t, db, "delete-id-expiry-shared", 0)
		postRunReport, err := usecase.Preview(context.Background(), policy, executeOptions)
		require.NoError(t, err)
		assert.Equal(t, int64(6), postRunReport.AgeCandidates())
		assert.Equal(t, int64(1), postRunReport.UnknownTimestampRows())
		assert.Zero(t, postRunReport.TotalEligible().ReporterResources())
		assert.Equal(t, int64(1), postRunReport.Exclusions().MissingRoot())
		assert.Equal(t, int64(1), postRunReport.Exclusions().RootTypeMismatch())
		assert.Equal(t, int64(1), postRunReport.Exclusions().InvalidReporterKey())
		assert.Equal(t, int64(1), postRunReport.Exclusions().MissingTerminalRepresentation())
		assert.Equal(t, int64(1), postRunReport.Exclusions().InconsistentTerminalRepresentation())
		assert.Equal(t, int64(1), postRunReport.Exclusions().NewerHistory())
	})

	t.Run("cancellation commits once, fresh preview allows safe rerun, and shared roots count once", func(t *testing.T) {
		isolateTombstoneCleanupPostgresSubtest(t, db)
		evaluationTime := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
		const reporterType = "batchtest"
		policy, err := bizmodel.NewTombstoneCleanupPolicy(evaluationTime, 7, reporterType)
		require.NoError(t, err)
		rootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, rootID, "host", 0, 1)
		reporterIDs := make([]uuid.UUID, 0, 3)
		for index := 0; index < 3; index++ {
			reporterID := tombstoneCleanupInsertReporter(t, db, rootID, fmt.Sprintf("shared-candidate-%d", index), "host", reporterType, "cleanup-instance", true, 1, 0, policy.Cutoff().Add(-time.Duration(index+1)*time.Minute), []tombstoneCleanupHistory{
				{version: 1, tombstone: true, transactionID: fmt.Sprintf("delete-id-shared-%d", index)},
			})
			reporterIDs = append(reporterIDs, reporterID)
		}
		usecase := tombstoneCleanupUsecase(t, db)

		cancelOptions, err := cleanupuc.NewOptions(1, time.Hour, 3, 2*time.Minute)
		require.NoError(t, err)
		cancelContext, cancel := context.WithCancel(context.Background())
		firstRun, firstRunErr := usecase.Execute(cancelContext, policy, cancelOptions, func(batchNumber int, _ bizmodel.TombstoneCleanupImpact) {
			if batchNumber == 1 {
				cancel()
			}
		})
		cancel()
		require.ErrorIs(t, firstRunErr, context.Canceled)
		assert.Equal(t, 1, firstRun.CommittedBatches())
		assert.False(t, firstRun.BatchLimitReached())
		assert.Equal(t, int64(1), firstRun.Impact().ReporterResources())
		assert.Equal(t, int64(1), firstRun.Impact().ReporterRepresentations())
		assert.Zero(t, firstRun.Impact().CommonRepresentations())
		assert.Equal(t, int64(1), firstRun.Impact().ResourceRootsImpacted())
		assert.Zero(t, firstRun.Impact().ResourcesRemovable())
		assertTombstoneCleanupRootCount(t, db, rootID, 1)
		for index, reporterID := range reporterIDs {
			if index == 2 { // This row has the oldest authoritative updated_at and must be selected first.
				assertTombstoneCleanupReporterCount(t, db, reporterID, 0)
			} else {
				assertTombstoneCleanupReporterCount(t, db, reporterID, 1)
			}
		}

		rerunOptions, err := cleanupuc.NewOptions(1, 0, 2, time.Minute)
		require.NoError(t, err)
		freshReport, err := usecase.Preview(context.Background(), policy, rerunOptions)
		require.NoError(t, err)
		assert.Equal(t, int64(2), freshReport.AgeCandidates())
		assert.Equal(t, int64(2), freshReport.TotalEligible().ReporterResources())

		rerun, err := usecase.Execute(context.Background(), policy, rerunOptions, nil)
		require.NoError(t, err)
		assert.Equal(t, 2, rerun.CommittedBatches())
		assert.Equal(t, int64(2), rerun.Impact().ReporterResources())
		assert.Equal(t, int64(2), rerun.Impact().ReporterRepresentations())
		assert.Equal(t, int64(2), rerun.Impact().CommonRepresentations())
		assert.Equal(t, int64(1), rerun.Impact().ResourceRootsImpacted(), "same root spans both committed batches")
		assert.Equal(t, int64(1), rerun.Impact().ResourcesRemovable())
		assertTombstoneCleanupRootCount(t, db, rootID, 0)
		assertTombstoneCleanupCommonCount(t, db, rootID, 0)
		for _, reporterID := range reporterIDs {
			assertTombstoneCleanupReporterCount(t, db, reporterID, 0)
		}
	})

	t.Run("API revival after selection uses the next generation and is preserved", func(t *testing.T) {
		isolateTombstoneCleanupPostgresSubtest(t, db)
		evaluationTime := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
		const reporterType = "revivalcase"
		policy, err := bizmodel.NewTombstoneCleanupPolicy(evaluationTime, 7, reporterType)
		require.NoError(t, err)
		rootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, rootID, "host", 1)
		reporterID := tombstoneCleanupInsertReporter(t, db, rootID, "revived-after-selection", "host", reporterType, "cleanup-instance", true, 1, 0, policy.Cutoff().Add(-time.Hour), []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: "delete-id-before-revival"},
		})

		gateLogger := &tombstoneCleanupPauseLogger{
			Interface: gormlogger.Default.LogMode(gormlogger.Silent),
			entered:   make(chan struct{}),
			release:   make(chan struct{}),
		}
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(gateLogger.release) }) }
		defer release()
		gatedDB := db.Session(&gorm.Session{Logger: gateLogger})
		transactions := data.NewTombstoneCleanupTransaction(gatedDB, data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 5))
		completed := make(chan struct {
			impact bizmodel.TombstoneCleanupImpact
			err    error
		}, 1)
		go func() {
			var finalImpact bizmodel.TombstoneCleanupImpact
			err := transactions.WithinSerializableTombstoneCleanup(context.Background(), "tombstone_cleanup_concurrent_revival", false, 30*time.Second, 5*time.Second, func(repository bizmodel.TombstoneCleanupRepository) error {
				finalImpact = bizmodel.TombstoneCleanupImpact{}
				impact, err := repository.DeleteTombstoneBatch(context.Background(), policy, 1)
				if err != nil {
					return err
				}
				finalImpact = impact
				return nil
			})
			completed <- struct {
				impact bizmodel.TombstoneCleanupImpact
				err    error
			}{impact: finalImpact, err: err}
		}()

		select {
		case <-gateLogger.entered:
		case <-time.After(time.Minute):
			t.Fatal("cleanup did not reach the post-selection history count")
		}
		resourceUsecase, _ := tombstoneCleanupAPIUsecase(t, db, reporterType)
		revivalErr := resourceUsecase.ReportResource(
			tombstoneCleanupAPIAuthzContext(),
			tombstoneCleanupAPIReportCommand(t, reporterType, "revived-after-selection", "report-id-after-revival"),
		)
		release()
		require.NoError(t, revivalErr)

		select {
		case result := <-completed:
			if result.err == nil {
				assert.Zero(t, result.impact.ReporterResources())
				assert.Zero(t, result.impact.ReporterRepresentations())
			} else {
				var postgresError *pgconn.PgError
				serializationConflict := errors.As(result.err, &postgresError) && postgresError.Code == "40001"
				eligibilityChanged := strings.Contains(result.err.Error(), "selected tombstone batch changed before deletion")
				require.True(t, serializationConflict || eligibilityChanged, "unexpected cleanup error: %v", result.err)
			}
		case <-time.After(time.Minute):
			t.Fatal("cleanup transaction did not finish after revival")
		}

		assertTombstoneCleanupReporterCount(t, db, reporterID, 1)
		assertTombstoneCleanupRootCount(t, db, rootID, 1)
		var reporter datamodel.ReporterResource
		require.NoError(t, db.Where("id = ?", reporterID).First(&reporter).Error)
		assert.False(t, reporter.Tombstone)
		assert.Equal(t, uint(1), reporter.Generation, "the actual API revival starts a new generation")
		assert.Equal(t, uint(0), reporter.RepresentationVersion, "the new generation starts at version zero")
		assertTombstoneCleanupHistoryCount(t, db, reporterID, 2)
		var revivedHistory datamodel.ReporterRepresentation
		require.NoError(t, db.Where("reporter_resource_id = ? AND generation = ? AND version = ?", reporterID, 1, 0).First(&revivedHistory).Error)
		assert.False(t, revivedHistory.Tombstone)
		assert.Equal(t, "report-id-after-revival", revivedHistory.TransactionId)
		assertTombstoneCleanupCommonCount(t, db, rootID, 2)
	})

	t.Run("cleanup before a later API report expires the last dedupe copy and creates a new aggregate", func(t *testing.T) {
		isolateTombstoneCleanupPostgresSubtest(t, db)
		evaluationTime := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
		const reporterType = "expirycase"
		const expiredTransactionID = "delete-id-expired-last-copy"
		policy, err := bizmodel.NewTombstoneCleanupPolicy(evaluationTime, 7, reporterType)
		require.NoError(t, err)
		oldRootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, oldRootID, "host", 1)
		oldReporterID := tombstoneCleanupInsertReporter(t, db, oldRootID, "reused-after-cleanup", "host", reporterType, "cleanup-instance", true, 1, 0, policy.Cutoff().Add(-time.Hour), []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: expiredTransactionID},
		})

		resourceUsecase, resourceRepository := tombstoneCleanupAPIUsecase(t, db, reporterType)
		processed, err := resourceRepository.HasTransactionIdBeenProcessed(db, bizmodel.NewTransactionId(expiredTransactionID))
		require.NoError(t, err)
		assert.True(t, processed, "the tombstone history is initially the last transaction-ID copy")

		options, err := cleanupuc.NewOptions(1, 0, 1, time.Minute)
		require.NoError(t, err)
		cleanupResult, err := tombstoneCleanupUsecase(t, db).Execute(context.Background(), policy, options, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(1), cleanupResult.Impact().ReporterResources())
		assert.Equal(t, int64(1), cleanupResult.Impact().ReporterRepresentations())
		assert.Equal(t, int64(1), cleanupResult.Impact().CommonRepresentations())
		assert.Equal(t, int64(1), cleanupResult.Impact().ResourcesRemovable())
		assertTombstoneCleanupReporterCount(t, db, oldReporterID, 0)
		assertTombstoneCleanupRootCount(t, db, oldRootID, 0)
		processed, err = resourceRepository.HasTransactionIdBeenProcessed(db, bizmodel.NewTransactionId(expiredTransactionID))
		require.NoError(t, err)
		assert.False(t, processed, "cleanup removed the last dedupe-history copy")

		reportErr := resourceUsecase.ReportResource(
			tombstoneCleanupAPIAuthzContext(),
			tombstoneCleanupAPIReportCommand(t, reporterType, "reused-after-cleanup", expiredTransactionID),
		)
		require.NoError(t, reportErr)
		var createdReporter datamodel.ReporterResource
		require.NoError(t, db.Where("local_resource_id = ? AND reporter_type = ? AND reporter_instance_id = ?", "reused-after-cleanup", reporterType, "cleanup-instance").First(&createdReporter).Error)
		assert.NotEqual(t, oldReporterID, createdReporter.ID)
		assert.NotEqual(t, oldRootID, createdReporter.ResourceID)
		assert.False(t, createdReporter.Tombstone)
		assert.Zero(t, createdReporter.Generation)
		assert.Zero(t, createdReporter.RepresentationVersion)
		assertTombstoneCleanupRootCount(t, db, createdReporter.ResourceID, 1)
		assertTombstoneCleanupHistoryCount(t, db, createdReporter.ID, 1)
		processed, err = resourceRepository.HasTransactionIdBeenProcessed(db, bizmodel.NewTransactionId(expiredTransactionID))
		require.NoError(t, err)
		assert.True(t, processed, "the later API report creates the newly accepted dedupe copy")
	})

	t.Run("commit-time PostgreSQL 40001 retries work and reports the committed batch once", func(t *testing.T) {
		isolateTombstoneCleanupPostgresSubtest(t, db)
		const (
			reporterType = "commitretry"
			sequenceName = "tombstone_cleanup_commit_retry_sequence"
			functionName = "tombstone_cleanup_commit_retry_failure"
			triggerName  = "tombstone_cleanup_commit_retry_trigger"
		)
		t.Cleanup(func() {
			if err := db.Exec("DROP TRIGGER IF EXISTS " + triggerName + " ON reporter_resources").Error; err != nil {
				t.Errorf("drop test-only deferred trigger: %v", err)
			}
			if err := db.Exec("DROP FUNCTION IF EXISTS " + functionName + "()").Error; err != nil {
				t.Errorf("drop test-only trigger function: %v", err)
			}
			if err := db.Exec("DROP SEQUENCE IF EXISTS " + sequenceName).Error; err != nil {
				t.Errorf("drop test-only retry sequence: %v", err)
			}
		})
		require.NoError(t, db.Exec("CREATE SEQUENCE "+sequenceName+" START 1").Error)
		require.NoError(t, db.Exec(`CREATE FUNCTION `+functionName+`() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF nextval('`+sequenceName+`') = 1 THEN
				RAISE EXCEPTION 'injected deferred serialization failure' USING ERRCODE = '40001';
			END IF;
			RETURN OLD;
		END;
		$$`).Error)
		require.NoError(t, db.Exec("CREATE CONSTRAINT TRIGGER "+triggerName+" AFTER DELETE ON reporter_resources DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION "+functionName+"()").Error)

		evaluationTime := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
		policy, err := bizmodel.NewTombstoneCleanupPolicy(evaluationTime, 7, reporterType)
		require.NoError(t, err)
		rootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, rootID, "host", 1)
		reporterID := tombstoneCleanupInsertReporter(t, db, rootID, "commit-retry-row", "host", reporterType, "cleanup-instance", true, 1, 0, policy.Cutoff().Add(-time.Hour), []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: "delete-id-commit-retry"},
		})
		options, err := cleanupuc.NewOptions(1, 0, 1, time.Minute)
		require.NoError(t, err)
		var committedNotifications int
		result, err := tombstoneCleanupUsecase(t, db).Execute(context.Background(), policy, options, func(int, bizmodel.TombstoneCleanupImpact) {
			committedNotifications++
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.CommittedBatches())
		assert.Equal(t, 1, committedNotifications, "the aborted first attempt must not notify as committed")
		assert.Equal(t, int64(1), result.Impact().ReporterResources())
		assert.Equal(t, int64(1), result.Impact().ReporterRepresentations())
		assert.Equal(t, int64(1), result.Impact().ResourcesRemovable())
		var retryAttempts int64
		require.NoError(t, db.Raw("SELECT last_value FROM "+sequenceName).Scan(&retryAttempts).Error)
		assert.Equal(t, int64(2), retryAttempts, "the real PostgreSQL commit error caused one retry")
		assert.Equal(t, 1, metricscollector.GetSerializationFailureCount())
		assertTombstoneCleanupReporterCount(t, db, reporterID, 0)
		assertTombstoneCleanupRootCount(t, db, rootID, 0)
	})

	t.Run("failed batch transaction rolls back metadata history and root deletion", func(t *testing.T) {
		isolateTombstoneCleanupPostgresSubtest(t, db)
		evaluationTime := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
		const reporterType = "rollbackcase"
		policy, err := bizmodel.NewTombstoneCleanupPolicy(evaluationTime, 7, reporterType)
		require.NoError(t, err)
		rootID := uuid.New()
		tombstoneCleanupSeedRoot(t, db, rootID, "host", 1)
		reporterID := tombstoneCleanupInsertReporter(t, db, rootID, "rollback-row", "host", reporterType, "cleanup-instance", true, 1, 0, policy.Cutoff().Add(-time.Hour), []tombstoneCleanupHistory{
			{version: 1, tombstone: true, transactionID: "delete-id-rollback"},
		})
		rollbackErr := errors.New("force rollback after cleanup work")
		transactions := data.NewTombstoneCleanupTransaction(db, data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 3))
		err = transactions.WithinSerializableTombstoneCleanup(context.Background(), "tombstone_cleanup_rollback_test", false, 30*time.Second, 5*time.Second, func(repository bizmodel.TombstoneCleanupRepository) error {
			_, err := repository.DeleteTombstoneBatch(context.Background(), policy, 1)
			if err != nil {
				return err
			}
			return rollbackErr
		})
		require.ErrorIs(t, err, rollbackErr)
		assertTombstoneCleanupReporterCount(t, db, reporterID, 1)
		assertTombstoneCleanupHistoryCount(t, db, reporterID, 1)
		assertTombstoneCleanupRootCount(t, db, rootID, 1)
		assertTombstoneCleanupCommonCount(t, db, rootID, 1)
	})
}

type tombstoneCleanupResourceState struct {
	ID   uuid.UUID `gorm:"column:id"`
	Type string    `gorm:"column:type"`
}

type tombstoneCleanupReporterResourceState struct {
	ID                    uuid.UUID `gorm:"column:id"`
	ResourceID            uuid.UUID `gorm:"column:resource_id"`
	LocalResourceID       string    `gorm:"column:local_resource_id"`
	ReporterType          string    `gorm:"column:reporter_type"`
	ResourceType          string    `gorm:"column:resource_type"`
	ReporterInstanceID    string    `gorm:"column:reporter_instance_id"`
	RepresentationVersion uint      `gorm:"column:representation_version"`
	Generation            uint      `gorm:"column:generation"`
	Tombstone             bool      `gorm:"column:tombstone"`
}

type tombstoneCleanupReporterHistoryState struct {
	ReporterResourceID uuid.UUID `gorm:"column:reporter_resource_id"`
	Version            uint      `gorm:"column:version"`
	Generation         uint      `gorm:"column:generation"`
	Tombstone          bool      `gorm:"column:tombstone"`
	TransactionID      string    `gorm:"column:transaction_id"`
}

type tombstoneCleanupCommonHistoryState struct {
	ResourceID    uuid.UUID `gorm:"column:resource_id"`
	Version       uint      `gorm:"column:version"`
	TransactionID string    `gorm:"column:transaction_id"`
}

func assertTombstoneCleanupResourceSnapshot(t *testing.T, db *gorm.DB, expected []tombstoneCleanupResourceState) {
	t.Helper()
	var actual []tombstoneCleanupResourceState
	require.NoError(t, db.Table("resource").Select("id, type").Scan(&actual).Error)
	assert.ElementsMatch(t, expected, actual)
}

func assertTombstoneCleanupReporterResourceSnapshot(t *testing.T, db *gorm.DB, expected []tombstoneCleanupReporterResourceState) {
	t.Helper()
	var actual []tombstoneCleanupReporterResourceState
	require.NoError(t, db.Table("reporter_resources").Select("id, resource_id, local_resource_id, reporter_type, resource_type, reporter_instance_id, representation_version, generation, tombstone").Scan(&actual).Error)
	assert.ElementsMatch(t, expected, actual)
}

func assertTombstoneCleanupReporterHistorySnapshot(t *testing.T, db *gorm.DB, expected []tombstoneCleanupReporterHistoryState) {
	t.Helper()
	var actual []tombstoneCleanupReporterHistoryState
	require.NoError(t, db.Table("reporter_representations").Select("reporter_resource_id, version, generation, tombstone, transaction_id").Scan(&actual).Error)
	assert.ElementsMatch(t, expected, actual)
}

func assertTombstoneCleanupCommonHistorySnapshot(t *testing.T, db *gorm.DB, expected []tombstoneCleanupCommonHistoryState) {
	t.Helper()
	var actual []tombstoneCleanupCommonHistoryState
	require.NoError(t, db.Table("common_representations").Select("resource_id, version, transaction_id").Scan(&actual).Error)
	assert.ElementsMatch(t, expected, actual)
}

type tombstoneCleanupHistory struct {
	version       uint
	tombstone     bool
	transactionID string
}

func setupTombstoneCleanupPostgres(t *testing.T) (*gorm.DB, testcontainers.Container) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	password := uuid.NewString()
	postgresContainer, err := testcontainers.Run(
		ctx,
		"postgres:17.8",
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER":     "tombstone_cleanup",
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       "tombstone_cleanup",
		}),
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(3*time.Minute)),
		testcontainers.WithHostConfigModifier(func(hostConfig *mobycontainer.HostConfig) {
			hostConfig.ShmSize = 256 << 20
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		if err := postgresContainer.Terminate(cleanupContext); err != nil {
			t.Errorf("terminate tombstone cleanup PostgreSQL test container: %v", err)
		}
	})

	host, err := postgresContainer.Host(ctx)
	require.NoError(t, err)
	port, err := postgresContainer.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	dsn := fmt.Sprintf("host=%s port=%s user=tombstone_cleanup password=%s dbname=tombstone_cleanup sslmode=disable", host, port.Port(), password)
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

func tombstoneCleanupUsecase(t *testing.T, db *gorm.DB) *cleanupuc.Usecase {
	t.Helper()
	transactionManager := data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 5)
	usecase, err := cleanupuc.New(data.NewTombstoneCleanupTransaction(db, transactionManager))
	require.NoError(t, err)
	return usecase
}

func isolateTombstoneCleanupPostgresSubtest(t *testing.T, db *gorm.DB) {
	t.Helper()
	clear := func() error {
		return db.Exec("TRUNCATE TABLE reporter_representations, reporter_resources, common_representations, resource CASCADE").Error
	}
	require.NoError(t, clear())
	t.Cleanup(func() {
		if err := clear(); err != nil {
			t.Errorf("clear tombstone cleanup subtest fixtures: %v", err)
		}
	})
}

func tombstoneCleanupAPIUsecase(
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
		"tombstone-cleanup-integration",
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

func tombstoneCleanupAPIAuthzContext() context.Context {
	return authnapi.NewAuthzContext(context.Background(), authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject: &authnapi.Claims{
			SubjectId: authnapi.SubjectId("tombstone-cleanup-integration-user"),
			AuthType:  authnapi.AuthTypeXRhIdentity,
		},
	})
}

func tombstoneCleanupAPIReportCommand(
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
	apiHref, err := bizmodel.NewApiHref("https://cleanup.example.test/api/" + localResourceID)
	require.NoError(t, err)
	consoleHref, err := bizmodel.NewConsoleHref("https://cleanup.example.test/console/" + localResourceID)
	require.NoError(t, err)
	reporterRepresentation, err := bizmodel.NewRepresentation(map[string]any{"local_resource_id": localResourceID})
	require.NoError(t, err)
	commonRepresentation, err := bizmodel.NewRepresentation(map[string]any{"workspace_id": "cleanup-api-workspace"})
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

func tombstoneCleanupSeedRoot(t *testing.T, db *gorm.DB, resourceID uuid.UUID, resourceType string, commonVersions ...uint) {
	t.Helper()
	var commonVersion *uint
	if len(commonVersions) > 0 {
		latest := commonVersions[0]
		for _, version := range commonVersions[1:] {
			if version > latest {
				latest = version
			}
		}
		commonVersion = &latest
	}
	root := datamodel.Resource{ID: resourceID, Type: resourceType, CommonVersion: commonVersion}
	require.NoError(t, db.Create(&root).Error)
	for _, version := range commonVersions {
		commonHistory := datamodel.CommonRepresentation{
			Representation:             datamodel.Representation{Data: internal.JsonObject{"workspace_id": "cleanup-test"}},
			ResourceId:                 resourceID,
			Version:                    version,
			ReportedByReporterType:     "hbi",
			ReportedByReporterInstance: "cleanup-instance",
			TransactionId:              fmt.Sprintf("cleanup-common-%s-%d", resourceID, version),
		}
		require.NoError(t, db.Create(&commonHistory).Error)
	}
}

func tombstoneCleanupInsertReporter(
	t *testing.T,
	db *gorm.DB,
	resourceID uuid.UUID,
	localResourceID string,
	resourceType string,
	reporterType string,
	reporterInstanceID string,
	tombstone bool,
	representationVersion uint,
	generation uint,
	updatedAt time.Time,
	history []tombstoneCleanupHistory,
) uuid.UUID {
	t.Helper()
	reporterResourceID := uuid.New()
	reporterResource := datamodel.ReporterResource{
		ID: reporterResourceID,
		ReporterResourceKey: datamodel.ReporterResourceKey{
			LocalResourceID:    localResourceID,
			ReporterType:       reporterType,
			ResourceType:       resourceType,
			ReporterInstanceID: reporterInstanceID,
		},
		ResourceID:            resourceID,
		APIHref:               "https://cleanup.example.test/resources/" + reporterResourceID.String(),
		RepresentationVersion: representationVersion,
		Generation:            generation,
		Tombstone:             tombstone,
		CreatedAt:             updatedAt,
		UpdatedAt:             updatedAt,
	}
	require.NoError(t, db.Create(&reporterResource).Error)
	for _, row := range history {
		var data internal.JsonObject
		if !row.tombstone {
			data = internal.JsonObject{"workspace_id": "cleanup-test"}
		}
		representation := datamodel.ReporterRepresentation{
			Representation:     datamodel.Representation{Data: data},
			ReporterResourceID: reporterResourceID,
			Version:            row.version,
			Generation:         generation,
			TransactionId:      row.transactionID,
			Tombstone:          row.tombstone,
			CreatedAt:          updatedAt,
		}
		require.NoError(t, db.Create(&representation).Error)
	}
	return reporterResourceID
}

func assertTombstoneCleanupReporterCount(t *testing.T, db *gorm.DB, reporterResourceID uuid.UUID, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&datamodel.ReporterResource{}).Where("id = ?", reporterResourceID).Count(&count).Error)
	assert.Equal(t, expected, count)
}

func assertTombstoneCleanupRootCount(t *testing.T, db *gorm.DB, resourceID uuid.UUID, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&datamodel.Resource{}).Where("id = ?", resourceID).Count(&count).Error)
	assert.Equal(t, expected, count)
}

func assertTombstoneCleanupCommonCount(t *testing.T, db *gorm.DB, resourceID uuid.UUID, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&datamodel.CommonRepresentation{}).Where("resource_id = ?", resourceID).Count(&count).Error)
	assert.Equal(t, expected, count)
}

func assertTombstoneCleanupHistoryCount(t *testing.T, db *gorm.DB, reporterResourceID uuid.UUID, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&datamodel.ReporterRepresentation{}).Where("reporter_resource_id = ?", reporterResourceID).Count(&count).Error)
	assert.Equal(t, expected, count)
}

func assertTombstoneCleanupHistoryTransactionCount(t *testing.T, db *gorm.DB, transactionID string, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&datamodel.ReporterRepresentation{}).Where("transaction_id = ?", transactionID).Count(&count).Error)
	assert.Equal(t, expected, count)
}

func runTombstoneCleanupIdentitySQL(
	t *testing.T,
	postgresContainer testcontainers.Container,
	report bizmodel.TombstoneCleanupReport,
	reporterTypeFilter string,
) []map[string]string {
	t.Helper()
	assetPath, err := filepath.Abs(filepath.Join("..", "..", "docs", "runbooks", "tombstone-cleanup-identities.sql"))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	const containerAssetPath = "/tmp/tombstone-cleanup-identities.sql"
	const containerCSVPath = "/tmp/tombstone-cleanup-identities.csv"
	require.NoError(t, postgresContainer.CopyFileToContainer(ctx, assetPath, containerAssetPath, 0o600))
	exitCode, output, err := postgresContainer.Exec(ctx, []string{
		"psql", "-X", "-q", "--csv", "-P", "footer=off",
		"-v", "ON_ERROR_STOP=1",
		"-v", "evaluation_time_utc=" + report.EvaluatedAt().Format(time.RFC3339Nano),
		"-v", "cutoff_utc=" + report.Cutoff().Format(time.RFC3339Nano),
		"-v", "reporter_type_filter=" + reporterTypeFilter,
		"-v", "bounded_row_limit=" + strconv.FormatInt(report.BoundedRowLimit(), 10),
		"-U", "tombstone_cleanup", "-d", "tombstone_cleanup", "-o", containerCSVPath, "-f", containerAssetPath,
	})
	require.NoError(t, err)
	execOutput, err := io.ReadAll(output)
	require.NoError(t, err)
	require.Equal(t, 0, exitCode, string(execOutput))
	archiveReader, err := postgresContainer.CopyFileFromContainer(ctx, containerCSVPath)
	require.NoError(t, err)
	defer archiveReader.Close()
	csvBytes, err := io.ReadAll(archiveReader)
	require.NoError(t, err)
	parsedRows, err := csv.NewReader(strings.NewReader(string(csvBytes))).ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, parsedRows)
	headers := parsedRows[0]
	rows := make([]map[string]string, 0, len(parsedRows)-1)
	for _, values := range parsedRows[1:] {
		require.Len(t, values, len(headers))
		row := make(map[string]string, len(headers))
		for index, header := range headers {
			row[header] = values[index]
		}
		rows = append(rows, row)
	}
	return rows
}

func findTombstoneCleanupIdentityRow(t *testing.T, rows []map[string]string, recordType string) map[string]string {
	t.Helper()
	for _, row := range rows {
		if row["record_type"] == recordType {
			return row
		}
	}
	t.Fatalf("operator identity CSV has no %q row", recordType)
	return nil
}

func assertTombstoneCleanupIdentitySummaryMatchesReport(t *testing.T, summary map[string]string, report bizmodel.TombstoneCleanupReport) {
	t.Helper()
	total := report.TotalEligible()
	bounded := report.BoundedImpact()
	exclusions := report.Exclusions()
	expected := map[string]int64{
		"bounded_row_limit":                             report.BoundedRowLimit(),
		"age_candidates":                                report.AgeCandidates(),
		"unknown_timestamp_rows":                        report.UnknownTimestampRows(),
		"eligible_reporter_resources":                   total.ReporterResources(),
		"bounded_reporter_resources":                    bounded.ReporterResources(),
		"excluded_missing_root":                         exclusions.MissingRoot(),
		"excluded_root_type_mismatch":                   exclusions.RootTypeMismatch(),
		"excluded_invalid_reporter_key":                 exclusions.InvalidReporterKey(),
		"excluded_missing_terminal_representation":      exclusions.MissingTerminalRepresentation(),
		"excluded_inconsistent_terminal_representation": exclusions.InconsistentTerminalRepresentation(),
		"excluded_newer_history":                        exclusions.NewerHistory(),
	}
	for column, expectedCount := range expected {
		assert.Equal(t, strconv.FormatInt(expectedCount, 10), summary[column], "column=%s", column)
	}
	assert.NotEmpty(t, summary["evaluation_time_utc"])
	assert.NotEmpty(t, summary["cutoff_utc"])
}

type tombstoneCleanupPauseLogger struct {
	gormlogger.Interface
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (logger *tombstoneCleanupPauseLogger) Trace(ctx context.Context, begin time.Time, query func() (string, int64), err error) {
	statement, rowsAffected := query()
	normalizedStatement := strings.ToLower(strings.ReplaceAll(statement, `"`, ""))
	if strings.HasPrefix(strings.TrimSpace(normalizedStatement), "select count(*) from reporter_representations") {
		logger.once.Do(func() {
			close(logger.entered)
			<-logger.release
		})
	}
	logger.Interface.Trace(ctx, begin, func() (string, int64) { return statement, rowsAffected }, err)
}
