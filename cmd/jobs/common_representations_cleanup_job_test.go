package jobs

import (
	"context"
	"testing"

	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunJobRegistersCommonRepresentationsCleanupAsPreviewByDefault(t *testing.T) {
	command := NewRunJobCommand(storage.NewOptions(), common.LoggerOptions{})
	cleanupCommand, _, err := command.Find([]string{"common-representations-cleanup-job"})
	require.NoError(t, err)

	previewFlag, err := cleanupCommand.Flags().GetBool("execute")
	require.NoError(t, err)
	assert.False(t, previewFlag, "the common representation cleanup command must preview by default")

	keepVersions, err := cleanupCommand.Flags().GetInt("keep-versions")
	require.NoError(t, err)
	assert.Equal(t, 2, keepVersions, "the default retention must keep the current version and its predecessor")
}

func TestCommonRepCleanupCommandRejectsSingleVersionRetentionAndRequiresAttestation(t *testing.T) {
	require.Error(t, validateCommonRepCleanupInvocation(false, false, 1), "keep-versions below two must be rejected")
	require.Error(t, validateCommonRepCleanupInvocation(true, false, 2), "execution requires the drained-consumer attestation")
	require.NoError(t, validateCommonRepCleanupInvocation(true, true, 2))
	require.NoError(t, validateCommonRepCleanupInvocation(false, false, 3))
}

func TestCommonRepCleanupCommandDoesNotOpenStorageBeforeValidationGates(t *testing.T) {
	err := runCommonRepresentationsCleanupJob(
		context.Background(),
		nil,
		common.LoggerOptions{},
		true,
		1,
		"",
		commonRepCleanupDefaultBatchSize,
		commonRepCleanupDefaultBatchDelay,
		commonRepCleanupDefaultMaxBatches,
		commonRepCleanupDefaultMaxDuration,
	)
	require.ErrorContains(t, err, "storage options are required")
}

func TestCommonRepCleanupCommandHonorsCanceledParentContextBeforeStorageOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runCommonRepresentationsCleanupJob(
		ctx,
		storage.NewOptions(),
		common.LoggerOptions{},
		false,
		2,
		"",
		commonRepCleanupDefaultBatchSize,
		commonRepCleanupDefaultBatchDelay,
		commonRepCleanupDefaultMaxBatches,
		commonRepCleanupDefaultMaxDuration,
	)
	require.ErrorContains(t, err, "canceled before startup")
}

func TestCommonRepCleanupSignalContextHandlesInterruptAndTermination(t *testing.T) {
	signalContext, stopSignals := commonRepCleanupSignalContext(context.Background())
	defer stopSignals()
	require.NoError(t, signalContext.Err())
}
