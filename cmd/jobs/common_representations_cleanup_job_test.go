package jobs

import (
	"testing"

	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunJobRegistersCommonRepresentationsCleanupAsPreviewByDefault(t *testing.T) {
	command := NewRunJobCommand(storage.NewOptions(), nil, common.LoggerOptions{})
	cleanupCommand, _, err := command.Find([]string{"common-representations-cleanup-job"})
	require.NoError(t, err)

	previewFlag, err := cleanupCommand.Flags().GetBool("execute")
	require.NoError(t, err)
	assert.False(t, previewFlag, "the common representation cleanup command must preview by default")

	keepVersions, err := cleanupCommand.Flags().GetInt("keep-versions")
	require.NoError(t, err)
	assert.Equal(t, 2, keepVersions, "the default retention must keep the current version and its predecessor")
}

func TestCommonRepCleanupCommandRejectsSingleVersionRetention(t *testing.T) {
	require.Error(t, validateCommonRepCleanupInvocation(1), "keep-versions below two must be rejected")
	require.Error(t, validateCommonRepCleanupInvocation(0))
	require.NoError(t, validateCommonRepCleanupInvocation(2))
	require.NoError(t, validateCommonRepCleanupInvocation(3))
}
