package reporterrepcleanup

import (
	"bytes"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReporterRepCleanupCommandRejectsSingleVersionRetention protects the required predecessor.
func TestReporterRepCleanupCommandRejectsSingleVersionRetention(t *testing.T) {
	require.Error(t, validateReporterRepCleanupInvocation(1), "keep-versions below two must be rejected")
	require.Error(t, validateReporterRepCleanupInvocation(0))
	require.NoError(t, validateReporterRepCleanupInvocation(2))
	require.NoError(t, validateReporterRepCleanupInvocation(3))
}

// TestReporterRepCleanupExecutionRequiresWriteFenceAcknowledgment checks the execution gate.
func TestReporterRepCleanupExecutionRequiresWriteFenceAcknowledgment(t *testing.T) {
	command := NewCommand(storage.NewOptions(), common.LoggerOptions{})
	command.SetArgs([]string{"--execute"})
	require.ErrorContains(t, command.Execute(), "--execute requires --writers-paused")
}

// TestSelectionLogging reports partial batches and only the delays actually
// planned for an invocation, including empty and batch-limited worklists.
func TestSelectionLogging(t *testing.T) {
	for _, test := range []struct {
		name       string
		rows       int64
		maxBatches int
		batches    string
		seconds    string
	}{
		{"empty", 0, 0, "0", "0"},
		{"single batch", 1000, 0, "1", "0"},
		{"exact batches", 2000, 0, "2", "0.5"},
		{"partial final batch", 5235, 0, "6", "2.5"},
		{"batch cap", 5235, 2, "2", "0.5"},
		{"cap exceeds worklist", 1000, 5, "1", "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := log.NewHelper(log.NewStdLogger(&output))
			options := reporterRepCleanupOptions{batchSize: 1000, batchDelay: 500 * time.Millisecond, maxBatches: test.maxBatches}
			logReporterRepCleanupSelection(logger, options, test.rows, time.Second)
			assert.Contains(t, output.String(), "projected_batches="+test.batches+" ")
			assert.Contains(t, output.String(), "minimum_duration_seconds="+test.seconds+" ")
			assert.Contains(t, output.String(), "minimum_duration_basis=batch_delays_only_excludes_sql")
		})
	}
}

// TestCommandDefaultsAndValidation verifies preview defaults and rejects malformed invocations before opening storage.
func TestCommandDefaultsAndValidation(t *testing.T) {
	command := NewCommand(storage.NewOptions(), common.LoggerOptions{})
	for flag, expected := range map[string]string{"execute": "false", "writers-paused": "false", "keep-versions": "2", "batch-size": "1000", "batch-delay": "500ms", "max-batches": "0"} {
		require.Equal(t, expected, command.Flags().Lookup(flag).DefValue)
	}
	require.Nil(t, command.Flags().Lookup("resource-type"))
	require.Nil(t, command.Flags().Lookup("reporter-type"))
	for _, args := range [][]string{{"--batch-size=0"}, {"--batch-delay=-1s"}, {"--max-batches=-1"}, {"--keep-versions=1"}} {
		command := NewCommand(storage.NewOptions(), common.LoggerOptions{})
		command.SetArgs(args)
		require.Error(t, command.Execute())
	}
}
