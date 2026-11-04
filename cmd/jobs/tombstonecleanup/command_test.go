package tombstonecleanup

import (
	"bytes"
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/require"
)

// TestCommandRejectsInvalidControls checks each control fails with its own
// message before a database connection is attempted.
func TestCommandRejectsInvalidControls(t *testing.T) {
	for _, test := range []struct{ arg, message string }{
		{"--tombstone-days=0", "tombstone-days"},
		{"--tombstone-days=-1", "tombstone-days"},
		{"--rows-per-transaction=4", "rows-per-transaction"},
		{"--rows-per-transaction=50001", "rows-per-transaction"},
		{"--max-batches=-1", "max-batches"},
		{"--max-batches=9223372036854775807", "max-batches"},
		{"--batch-delay=-1s", "batch-delay"},
	} {
		t.Run(test.arg, func(t *testing.T) {
			command := NewCommand(storage.NewOptions(), common.LoggerOptions{})
			command.SetArgs([]string{test.arg})
			require.ErrorContains(t, command.Execute(), test.message)
		})
	}
}

// TestCommandAcceptsBoundaryControls reaches storage validation, past flag checks.
func TestCommandAcceptsBoundaryControls(t *testing.T) {
	for _, args := range [][]string{
		{"--execute"},
		{"--rows-per-transaction=5", "--max-batches=1", "--batch-delay=0s"},
		{"--rows-per-transaction=50000", "--tombstone-days=1"},
	} {
		command := NewCommand(storage.NewOptions(), common.LoggerOptions{})
		command.SetArgs(args)
		require.ErrorContains(t, command.Execute(), "requires PostgreSQL storage options", args)
	}
}

// TestFinalLogStatus distinguishes a failed invocation from a successful one.
func TestFinalLogStatus(t *testing.T) {
	for _, failure := range []error{nil, context.Canceled} {
		var output bytes.Buffer
		logResult(log.NewHelper(log.NewStdLogger(&output)), "tombstone cleanup result", true, result{batches: 1, deleted: counts{parents: 2}}, &failure)
		require.Contains(t, output.String(), "reporter_resources_deleted=2")
		if failure == nil {
			require.Contains(t, output.String(), "INFO")
			require.Contains(t, output.String(), "status=succeeded")
		} else {
			require.Contains(t, output.String(), "ERROR")
			require.Contains(t, output.String(), "status=failed")
			require.Contains(t, output.String(), "context canceled")
		}
	}
}
