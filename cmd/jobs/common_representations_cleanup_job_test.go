package jobs

import (
	"testing"

	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunJobRegistersCommonRepresentationsCleanupAsPreviewByDefault checks safe CLI defaults.
func TestRunJobRegistersCommonRepresentationsCleanupAsPreviewByDefault(t *testing.T) {
	command := NewRunJobCommand(storage.NewOptions(), common.LoggerOptions{})
	cleanupCommand, _, err := command.Find([]string{"common-representations-cleanup-job"})
	require.NoError(t, err)

	previewFlag, err := cleanupCommand.Flags().GetBool("execute")
	require.NoError(t, err)
	assert.False(t, previewFlag, "the common representation cleanup command must preview by default")
	writersPaused, err := cleanupCommand.Flags().GetBool("writers-paused")
	require.NoError(t, err)
	assert.False(t, writersPaused, "the command must not assume an external write fence")

	keepVersions, err := cleanupCommand.Flags().GetInt("keep-versions")
	require.NoError(t, err)
	assert.Equal(t, 2, keepVersions, "the default retention must keep the current version and its predecessor")
	maxBatches, err := cleanupCommand.Flags().GetInt("max-batches")
	require.NoError(t, err)
	assert.Zero(t, maxBatches, "execution must process the entire worklist by default")
}
