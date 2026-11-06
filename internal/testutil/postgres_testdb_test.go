package testutil

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTerminateSharedPostgres_Unused(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_LIFECYCLE_CHILD") == t.Name() {
		require.NoError(t, TerminateSharedPostgres())
		require.NoError(t, TerminateSharedPostgres())
		return
	}

	output, err := runPostgresLifecycleTest(t)
	require.NoError(t, err, "%s", output)
}

func TestSharedPostgresServer_AfterTermination(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_LIFECYCLE_CHILD") == t.Name() {
		require.NoError(t, TerminateSharedPostgres())
		sharedPostgresServer(t)
		return
	}

	output, err := runPostgresLifecycleTest(t)
	var exitError *exec.ExitError
	require.ErrorAs(t, err, &exitError, "%s", output)
	require.Equal(t, 1, exitError.ExitCode(), "%s", output)
	require.Contains(t, string(output), "shared PostgreSQL test server is unavailable: TerminateSharedPostgres was called before this test")
	require.NotContains(t, string(output), "panic:")
}

// runPostgresLifecycleTest isolates irreversible changes to the shared server's
// sync.Once so lifecycle tests cannot affect other tests in this binary.
func runPostgresLifecycleTest(t *testing.T) ([]byte, error) {
	t.Helper()

	executable, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^"+t.Name()+"$", "-test.v")
	command.Env = append(os.Environ(), "TEST_POSTGRES_LIFECYCLE_CHILD="+t.Name())
	return command.CombinedOutput()
}

func TestPostgresDatabaseIdentifiers(t *testing.T) {
	if !UsingPostgres() {
		t.Skip("database identifier quoting requires the PostgreSQL backend")
	}

	server, err := startPostgresServer()
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		require.NoError(t, server.terminate(ctx))
	})

	// PostgreSQL escapes embedded quotes by doubling them, not with Go's
	// backslash escapes. SQL-looking content must remain part of the name.
	templateName := `template "quoted"\name`
	databaseName := `test"; SELECT 1; --\name`
	require.NoError(t, createDatabase(server, templateName, ""))
	require.NoError(t, createDatabase(server, databaseName, templateName))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, name := range []string{databaseName, templateName} {
		var exists bool
		require.NoError(t, server.admin.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists))
		require.True(t, exists, "database name must be preserved exactly: %s", name)

		dropDatabase(t, server, name)
		require.NoError(t, server.admin.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists))
		require.False(t, exists, "database must be dropped: %s", name)
	}
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
