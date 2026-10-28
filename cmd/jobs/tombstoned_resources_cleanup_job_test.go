package jobs

import (
	"testing"

	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/require"
)

// TestRunJobRegistersTombstonedResourcesCleanupAsPreviewByDefault verifies that
// the one-time command is reachable and does not execute deletions by default.
func TestRunJobRegistersTombstonedResourcesCleanupAsPreviewByDefault(t *testing.T) {
	command := NewRunJobCommand(storage.NewOptions(), common.LoggerOptions{})
	cleanupCommand, _, err := command.Find([]string{"tombstoned-resources-cleanup-job"})
	require.NoError(t, err)
	require.Equal(t, "tombstoned-resources-cleanup-job", cleanupCommand.Name())
	execute, err := cleanupCommand.Flags().GetBool("execute")
	require.NoError(t, err)
	require.False(t, execute)
	boundaryVerified, err := cleanupCommand.Flags().GetBool("consumer-boundary-verified")
	require.NoError(t, err)
	require.False(t, boundaryVerified)
	tombstoneDays, err := cleanupCommand.Flags().GetInt("tombstone-days")
	require.NoError(t, err)
	require.Equal(t, 30, tombstoneDays)
	maxBatches, err := cleanupCommand.Flags().GetInt("max-batches")
	require.NoError(t, err)
	require.Zero(t, maxBatches)
}
