package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTerminateSharedPostgres_Unused(t *testing.T) {
	require.NoError(t, TerminateSharedPostgres())
	require.NoError(t, TerminateSharedPostgres())
}

func TestPostgresServerTerminate_ReportsFailureAndAllowsRetry(t *testing.T) {
	if !UsingPostgres() {
		t.Skip("container cleanup requires the PostgreSQL backend")
	}
	// Use a private server: terminating the shared server during m.Run would
	// invalidate other tests' database handles and cached templates.
	server, err := startPostgresServer()
	require.NoError(t, err)
	terminated := false
	t.Cleanup(func() {
		if terminated {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		require.NoError(t, server.terminate(ctx))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = server.terminate(ctx)
	require.Error(t, err, "a failed termination must not be reported as success")
	require.ErrorContains(t, err, server.container.GetContainerID(), "identify the container for manual cleanup")

	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()
	require.NoError(t, server.terminate(cleanupCtx), "cleanup must be retryable after a failed attempt")
	terminated = true
	_, err = server.container.State(cleanupCtx)
	require.Error(t, err, "the container must have been removed, not merely stopped")
}
