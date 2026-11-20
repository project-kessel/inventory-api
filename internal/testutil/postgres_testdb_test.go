package testutil

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	"github.com/project-kessel/inventory-api/internal/testutil/containerdebug"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"gorm.io/gorm"
)

// TestPostgresDebugLifecycle exercises retention across a real process exit,
// including the failed fixture's committed data and successful fixture cleanup.
func TestPostgresDebugLifecycle(t *testing.T) {
	if !UsingPostgres() {
		t.Skip("container retention requires the PostgreSQL backend")
	}
	if os.Getenv("TEST_POSTGRES_DEBUG_CHILD") == "1" {
		runPostgresDebugChild(t)
		return
	}
	require.NoError(t, containerdebug.Configure())
	for _, mode := range []string{"retain", "keep-passing", "no-debug"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			executable, err := os.Executable()
			require.NoError(t, err)
			command := exec.CommandContext(ctx, executable, "-test.run=^TestPostgresDebugLifecycle$", "-test.v")
			// Override rather than inherit debug settings: Configure is process-global.
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "TEST_CONTAINERS_") && !strings.HasPrefix(entry, "TEST_POSTGRES_DEBUG_") {
					command.Env = append(command.Env, entry)
				}
			}
			command.Env = append(command.Env, "TEST_POSTGRES_DEBUG_CHILD=1", "TEST_POSTGRES_DEBUG_MODE="+mode,
				"TEST_POSTGRES_DEBUG_DIRECTORY="+directory)
			if mode != "no-debug" {
				command.Env = append(command.Env, "TEST_CONTAINERS_KEEP_ON_FAILURE=true",
					"TEST_CONTAINERS_DEBUG_FILE="+filepath.Join(directory, "credentials"))
			}
			// Recover the ID even if an assertion or subprocess setup fails, so a
			// retained container is always removed by the parent, not by Ryuk.
			t.Cleanup(func() {
				data, err := os.ReadFile(filepath.Join(directory, "container-id"))
				if os.IsNotExist(err) {
					return
				}
				require.NoError(t, err)
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cleanupCancel()
				runtime, err := testcontainers.NewDockerClientWithOpts(cleanupCtx)
				require.NoError(t, err)
				defer func() { require.NoError(t, runtime.Close()) }()
				id := string(data)
				_, err = runtime.ContainerInspect(cleanupCtx, id, client.ContainerInspectOptions{})
				if mode == "retain" {
					_, removeErr := runtime.ContainerRemove(cleanupCtx, id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
					require.NoError(t, removeErr)
					require.NoError(t, err, "failed test container must survive subprocess exit")
				} else {
					// Remove unexpected leftovers before reporting the regression.
					if err == nil {
						_, removeErr := runtime.ContainerRemove(cleanupCtx, id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
						require.NoError(t, removeErr)
					}
					require.True(t, errdefs.IsNotFound(err), "successful test container must be removed")
				}
			})
			output, err := command.CombinedOutput()
			require.NoError(t, ctx.Err(), "subprocess timed out")
			if mode == "retain" {
				var exitError *exec.ExitError
				require.ErrorAs(t, err, &exitError, "%s", output)
				require.Equal(t, 1, exitError.ExitCode(), "%s", output)
			} else {
				require.NoError(t, err, "%s", output)
			}
			if mode == "no-debug" {
				require.NotContains(t, string(output), "containerdebug:")
				return
			}
			files, err := filepath.Glob(filepath.Join(directory, "credentials.*.jsonl"))
			require.NoError(t, err)
			require.Len(t, files, 1)
			info, err := os.Stat(files[0])
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			data, err := os.ReadFile(files[0])
			require.NoError(t, err)
			var records []postgresDebugRecord
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				var record postgresDebugRecord
				require.NoError(t, json.Unmarshal([]byte(line), &record))
				records = append(records, record)
			}
			require.NotEmpty(t, records)
			container := records[0]
			require.Equal(t, "container", container.Type)
			require.Equal(t, "postgres", container.Kind)
			require.NotEmpty(t, container.Password)
			// Use a boolean assertion so assertion failures cannot print the secret.
			require.False(t, strings.Contains(string(output), container.Password), "stdout must not contain the password")
			retained := false
			failedFixtures := 0
			for _, record := range records[1:] {
				require.Equal(t, container.ContainerID, record.ContainerID)
				if record.Type == "retained" {
					retained = true
					continue
				}
				require.Equal(t, "fixture", record.Type)
				require.Equal(t, "postgres", record.Kind)
				require.NotEmpty(t, record.Database)
				require.NotEmpty(t, record.Test)
				if mode != "retain" || strings.Contains(record.Test, "/passing") {
					continue
				}
				failedFixtures++
				// Credentials come exclusively from the owner-only debug file.
				dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", container.Username, container.Password, container.Endpoint, record.Database)
				db, err := sql.Open("pgx", dsn)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, db.Close()) })
				var value string
				require.NoError(t, db.QueryRowContext(ctx, "SELECT value FROM retained_row").Scan(&value))
				require.Equal(t, "survives process exit", value)
			}
			require.Equal(t, mode == "retain", retained)
			if mode == "retain" {
				require.Equal(t, 2, failedFixtures, "both test and initialization failures must retain data")
				require.Contains(t, string(output), "podman rm -f")
			}
		})
	}
}

type postgresDebugRecord struct {
	Type        string `json:"type"`
	Kind        string `json:"kind"`
	ContainerID string `json:"container_id"`
	Endpoint    string `json:"endpoint"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Test        string `json:"test"`
	Database    string `json:"database"`
}

// runPostgresDebugChild intentionally fails only in the isolated subprocess.
func runPostgresDebugChild(t *testing.T) {
	directory := os.Getenv("TEST_POSTGRES_DEBUG_DIRECTORY")
	mode := os.Getenv("TEST_POSTGRES_DEBUG_MODE")
	t.Cleanup(func() {
		server := postgresShared
		require.NotNil(t, server)
		require.NoError(t, TerminateSharedPostgres())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.ErrorContains(t, server.admin.PingContext(ctx), "database is closed", "maintenance pool must be closed")
		require.NoError(t, TerminateSharedPostgres(), "shared cleanup remains idempotent")
	})
	var passedDatabase string
	passing := func(t *testing.T) {
		dsn := PostgresTestDSN(t, nil)
		server := postgresShared
		require.NoError(t, os.WriteFile(filepath.Join(directory, "container-id"), []byte(server.container.GetContainerID()), 0600))
		db, err := sql.Open("pgx", dsn)
		require.NoError(t, err)
		defer func() { require.NoError(t, db.Close()) }()
		require.NoError(t, db.QueryRow("SELECT current_database()").Scan(&passedDatabase))
	}
	t.Run("passing", passing)
	var exists bool
	require.NoError(t, postgresShared.admin.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname=$1)", passedDatabase).Scan(&exists))
	require.False(t, exists, "passing database must be dropped even with keep enabled")
	if mode != "retain" {
		return
	}
	t.Run("failed", func(t *testing.T) {
		db := NewPostgresTestDB(t, nil, nil)
		require.NoError(t, db.Exec("CREATE TABLE retained_row (value text); INSERT INTO retained_row VALUES ('survives process exit')").Error)
		t.Fatal("intentional failure to verify retention")
	})
	t.Run("initialization", func(t *testing.T) {
		PostgresTestDSN(t, func(db *gorm.DB) error {
			if err := db.Exec("CREATE TABLE retained_row (value text); INSERT INTO retained_row VALUES ('survives process exit')").Error; err != nil {
				return err
			}
			return fmt.Errorf("intentional initialization failure")
		})
	})
	// A previous failure retains the container, not later successful fixtures.
	t.Run("passing-after-failure", passing)
	require.NoError(t, postgresShared.admin.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname=$1)", passedDatabase).Scan(&exists))
	require.False(t, exists, "passing database must be dropped after another test fails")
}

// TestTerminateSharedPostgres_Unused verifies that repeated cleanup without
// database setup succeeds without starting a container.
func TestTerminateSharedPostgres_Unused(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_LIFECYCLE_CHILD") == t.Name() {
		require.NoError(t, TerminateSharedPostgres())
		require.NoError(t, TerminateSharedPostgres())
		return
	}

	output, err := runPostgresLifecycleTest(t)
	require.NoError(t, err, "%s", output)
}

// TestSharedPostgresServer_AfterTermination verifies that setup after shutdown
// fails with a useful message instead of returning a nil server.
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

// TestPostgresDatabaseIdentifiers verifies that creation, template cloning, and
// deletion preserve quoted identifiers without executing SQL-looking name text.
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

// TestPostgresServerTerminate_ReportsFailureAndAllowsRetry verifies that failed
// cleanup identifies the container and retains enough state to retry removal.
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
