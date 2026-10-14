package jobs

import (
	"context"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunJobRegistersTombstoneCleanupAsPreviewByDefault(t *testing.T) {
	command := NewRunJobCommand(storage.NewOptions(), common.LoggerOptions{})
	cleanupCommand, _, err := command.Find([]string{"tombstoned-resources-cleanup-job"})
	require.NoError(t, err)
	require.NotNil(t, cleanupCommand)
	executeFlag := cleanupCommand.Flags().Lookup("execute")
	require.NotNil(t, executeFlag)
	assert.Equal(t, "false", executeFlag.DefValue)
	assert.False(t, executeFlag.Changed)
	assert.Equal(t, "1000", cleanupCommand.Flags().Lookup("batch-size").DefValue)
	assert.Equal(t, "1", cleanupCommand.Flags().Lookup("max-batches").DefValue)
	assert.Equal(t, "500ms", cleanupCommand.Flags().Lookup("batch-delay").DefValue)
	assert.Equal(t, "5m0s", cleanupCommand.Flags().Lookup("max-duration").DefValue)
}

func TestTombstoneCleanupCommandRequiresPositiveAgeAndExecutionAttestation(t *testing.T) {
	for _, test := range []struct {
		name      string
		arguments []string
		errorText string
	}{
		{
			name:      "age is required",
			arguments: nil,
			errorText: "required flag(s) \"tombstone-days\" not set",
		},
		{
			name:      "age must be positive",
			arguments: []string{"--tombstone-days=0"},
			errorText: "--tombstone-days must be positive",
		},
		{
			name:      "execute requires a consumer drain attestation",
			arguments: []string{"--tombstone-days=7", "--execute"},
			errorText: "--execute requires --acknowledge-drained-consumer",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := NewTombstonedResourcesCleanupJobCommand(nil, common.LoggerOptions{})
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			command.SetArgs(test.arguments)

			err := command.Execute()
			require.Error(t, err)
			assert.ErrorContains(t, err, test.errorText)
		})
	}
}

func TestTombstoneCleanupCommandDoesNotOpenStorageBeforeExecutionGate(t *testing.T) {
	command := NewTombstonedResourcesCleanupJobCommand(nil, common.LoggerOptions{})
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--tombstone-days=7", "--execute", "--acknowledge-drained-consumer"})

	err := command.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "storage options are required")
}

func TestTombstoneCleanupCommandHonorsCanceledParentContextBeforeStorageOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storageOptions := storage.NewOptions()
	storageOptions.Database = storage.Postgres
	command := NewTombstonedResourcesCleanupJobCommand(storageOptions, common.LoggerOptions{})
	command.SetContext(ctx)
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--tombstone-days=7"})

	err := command.Execute()
	require.ErrorIs(t, err, context.Canceled)
}

func TestTombstoneCleanupSignalContextHandlesInterruptAndTermination(t *testing.T) {
	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) {
			ctx, stop := tombstoneCleanupSignalContext(context.Background())
			defer stop()
			process, err := os.FindProcess(os.Getpid())
			require.NoError(t, err)
			require.NoError(t, process.Signal(signal))
			select {
			case <-ctx.Done():
				assert.ErrorIs(t, ctx.Err(), context.Canceled)
			case <-time.After(time.Second):
				t.Fatal("cleanup context did not cancel for signal", signal)
			}
		})
	}
}
