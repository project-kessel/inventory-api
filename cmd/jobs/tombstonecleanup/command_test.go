package tombstonecleanup

import (
	"testing"

	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/require"
)

// TestCommandDefaultsAndValidation protects preview defaults and rejects invalid
// batch controls before attempting a database connection.
func TestCommandDefaultsAndValidation(t *testing.T) {
	command := NewCommand(storage.NewOptions(), common.LoggerOptions{})
	for flag, expected := range map[string]string{
		"execute": "false", "tombstone-days": "30", "batch-size": "1000",
		"batch-delay": "500ms", "max-batches": "0",
	} {
		require.NotNil(t, command.Flags().Lookup(flag), flag)
		require.Equal(t, expected, command.Flags().Lookup(flag).DefValue, flag)
	}
	for _, arguments := range [][]string{
		{"--tombstone-days=-1"}, {"--tombstone-days=0"}, {"--batch-size=0"}, {"--batch-delay=-1s"}, {"--max-batches=-1"},
	} {
		command := NewCommand(storage.NewOptions(), common.LoggerOptions{})
		command.SetArgs(arguments)
		require.Error(t, command.Execute(), arguments)
	}
}

// TestExecutionRequiresConsumerBoundaryAcknowledgment prevents accidentally
// pruning data that an unverified replication backlog may still need.
func TestExecutionRequiresConsumerBoundaryAcknowledgment(t *testing.T) {
	command := NewCommand(storage.NewOptions(), common.LoggerOptions{})
	command.SetArgs([]string{"--execute"})
	require.ErrorContains(t, command.Execute(), "--execute requires --consumer-boundary-verified")
}
