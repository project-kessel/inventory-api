package testutil

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	"github.com/project-kessel/inventory-api/internal/testutil/containerdebug"
	"github.com/project-kessel/inventory-api/internal/testutil/testlifecycle"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"gorm.io/gorm"
)

// TestMain resolves shared database retention after all test cleanups finish.
func TestMain(m *testing.M) {
	var server *postgresServer
	os.Exit(testlifecycle.Run(m,
		func() error { server = postgresShared; return nil },
		TerminateSharedPostgres,
		func() error { return verifyPostgresDebugCleanup(server) },
	))
}

// verifyPostgresDebugCleanup checks teardown after m.Run, when it is safe
// to finalize per-test retention. The parent requires the marker so intentional
// child failures cannot mask a failed teardown assertion.
func verifyPostgresDebugCleanup(server *postgresServer) error {
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.admin.PingContext(ctx); err == nil || !strings.Contains(err.Error(), "database is closed") {
			return fmt.Errorf("maintenance pool must be closed after cleanup: %v", err)
		}
	}
	if err := TerminateSharedPostgres(); err != nil {
		return fmt.Errorf("shared cleanup must remain idempotent: %w", err)
	}
	if os.Getenv("TEST_POSTGRES_DEBUG_CHILD") == "1" {
		fmt.Println("postgres-debug: maintenance pool closed and cleanup idempotent")
	}
	return nil
}

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
	for _, mode := range []string{"retain", "setup-failure", "drop-failure", "keep-passing", "no-debug"} {
		t.Run(mode, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "literal[")
			require.NoError(t, os.Mkdir(directory, 0700))
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
				cleanupPostgresDebugChild(t, directory, mode == "retain" || mode == "setup-failure")
			})
			output, err := command.CombinedOutput()
			require.NoError(t, ctx.Err(), "subprocess timed out")
			require.Contains(t, string(output), "postgres-debug: maintenance pool closed and cleanup idempotent")
			if mode == "retain" || mode == "setup-failure" || mode == "drop-failure" {
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
			files, err := postgresDebugRegistryFiles(directory)
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
			passingFixtures := 0
			var maintenance *sql.DB
			if mode == "retain" {
				dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", container.Username, container.Password, container.Endpoint, postgresMaintenanceDB)
				maintenance, err = sql.Open("pgx", dsn)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, maintenance.Close()) })
			}
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
				if mode == "drop-failure" {
					passingFixtures++
					require.Contains(t, string(output), "drop test database "+record.Database+": sql: database is closed")
				}
				if mode != "retain" {
					continue
				}
				if strings.Contains(record.Test, "/passing") {
					passingFixtures++
					var exists bool
					require.NoError(t, maintenance.QueryRowContext(ctx,
						"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname=$1)", record.Database).Scan(&exists))
					require.False(t, exists, "passing database must be dropped even when another test fails")
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
			require.Equal(t, mode == "retain" || mode == "setup-failure", retained)
			if mode == "drop-failure" {
				require.Equal(t, 2, passingFixtures, "all database cleanup errors must be reported")
				require.Contains(t, string(output), "--- PASS: TestPostgresDebugLifecycle", "only shared cleanup should fail")
			}
			if mode == "setup-failure" {
				_, err := os.Stat(filepath.Join(directory, "container-id"))
				require.True(t, os.IsNotExist(err), "cleanup must recover the ID exclusively from the registry")
				require.Contains(t, string(output), "intentional setup failure before writing container ID")
			}
			if mode == "retain" {
				require.Equal(t, 3, failedFixtures, "test, late cleanup, and initialization failures must retain data")
				require.Equal(t, 2, passingFixtures)
				require.Contains(t, string(output), "intentional late cleanup failure")
				require.Contains(t, string(output), "podman rm -f")
			}
		})
	}
}

func postgresDebugRegistryFiles(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		matched, err := filepath.Match("credentials.*.jsonl", entry.Name())
		if err != nil {
			return nil, err
		}
		if matched && !entry.IsDir() {
			paths = append(paths, filepath.Join(directory, entry.Name()))
		}
	}
	return paths, nil
}

// postgresDebugContainerIDs keeps every known ID, even if another discovery
// source is unreadable, so a partial setup failure cannot prevent cleanup.
func postgresDebugContainerIDs(t *testing.T, directory string) []string {
	t.Helper()
	var ids []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		for _, existing := range ids {
			if existing == id {
				return
			}
		}
		ids = append(ids, id)
	}
	data, err := os.ReadFile(filepath.Join(directory, "container-id"))
	if err == nil {
		add(string(data))
	} else if !os.IsNotExist(err) {
		t.Errorf("read child container ID: %v", err)
	}
	files, err := postgresDebugRegistryFiles(directory)
	if err != nil {
		t.Errorf("find child registries: %v", err)
	}
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			t.Errorf("open child registry: %v", err)
			continue
		}
		decoder := json.NewDecoder(file)
		for {
			var record postgresDebugRecord
			if err := decoder.Decode(&record); err != nil {
				if err != io.EOF {
					t.Errorf("decode child registry: %v", err)
				}
				break
			}
			if record.Kind == "postgres" {
				add(record.ContainerID)
			}
		}
		if err := file.Close(); err != nil {
			t.Errorf("close child registry: %v", err)
		}
	}
	return ids
}

// cleanupPostgresDebugChild reports child retention independently of parent
// removal. Inspection failures must not gate removal or consume its deadline.
func cleanupPostgresDebugChild(t *testing.T, directory string, wantRetained bool) {
	t.Helper()
	ids := postgresDebugContainerIDs(t, directory)
	if len(ids) == 0 {
		return // setup failed before registering a container
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	runtime, err := testcontainers.NewDockerClientWithOpts(ctx)
	cancel()
	if err != nil {
		t.Errorf("connect for child cleanup; remove containers %v manually: %v", ids, err)
		return
	}
	defer func() {
		if err := runtime.Close(); err != nil {
			t.Errorf("close cleanup runtime client: %v", err)
		}
	}()
	for _, id := range ids {
		inspectCtx, inspectCancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, inspectErr := runtime.ContainerInspect(inspectCtx, id, client.ContainerInspectOptions{})
		inspectCancel()

		if !errdefs.IsNotFound(inspectErr) {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, removeErr := runtime.ContainerRemove(cleanupCtx, id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
			cleanupCancel()
			if removeErr != nil && !errdefs.IsNotFound(removeErr) {
				t.Errorf("remove child container %s (remove manually): %v", id, removeErr)
			}
			verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, verifyErr := runtime.ContainerInspect(verifyCtx, id, client.ContainerInspectOptions{})
			verifyCancel()
			if !errdefs.IsNotFound(verifyErr) {
				t.Errorf("child container %s removal not verified (remove manually): %v", id, verifyErr)
			}
		}
		if wantRetained {
			if inspectErr != nil {
				t.Errorf("failed test container %s must survive subprocess exit: %v", id, inspectErr)
			}
		} else if !errdefs.IsNotFound(inspectErr) {
			t.Errorf("successful test container %s must be removed: %v", id, inspectErr)
		}
	}
}

// TestPostgresDebugContainerIDs verifies literal-directory discovery, ID
// deduplication, and recovery of every PostgreSQL container from the registry.
func TestPostgresDebugContainerIDs(t *testing.T) {
	for _, withIDFile := range []bool{false, true} {
		name := "registry-only"
		if withIDFile {
			name = "with-id-file"
		}
		t.Run(name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "literal[")
			require.NoError(t, os.Mkdir(directory, 0700))
			if withIDFile {
				require.NoError(t, os.WriteFile(filepath.Join(directory, "container-id"), []byte("postgres-id\n"), 0600))
			}
			registry := `{"type":"container","kind":"postgres","container_id":"postgres-id"}
{"type":"fixture","kind":"postgres","container_id":"postgres-id"}
{"type":"container","kind":"spicedb","container_id":"other-id"}
{"type":"container","kind":"postgres","container_id":"second-postgres-id"}
`
			require.NoError(t, os.WriteFile(filepath.Join(directory, "credentials.child.1.jsonl"), []byte(registry), 0600))
			require.Equal(t, []string{"postgres-id", "second-postgres-id"}, postgresDebugContainerIDs(t, directory))
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
	if mode == "setup-failure" {
		sharedPostgresServer(t)
		t.Fatal("intentional setup failure before writing container ID")
	}
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
	if mode == "no-debug" {
		require.NoError(t, postgresShared.admin.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname=$1)", passedDatabase).Scan(&exists))
		require.False(t, exists, "off-mode cleanup must drop the database immediately")
	}
	if mode == "drop-failure" {
		t.Run("passing-before-drop-failure", passing)
		// Force real DROP failures after m.Run; teardown must report both and
		// still remove the container rather than returning on the first error.
		t.Cleanup(func() { require.NoError(t, postgresShared.admin.Close()) })
		return
	}
	if mode != "retain" {
		return
	}
	t.Run("failed", func(t *testing.T) {
		db := NewPostgresTestDB(t, nil, nil)
		require.NoError(t, db.Exec("CREATE TABLE retained_row (value text); INSERT INTO retained_row VALUES ('survives process exit')").Error)
		t.Fatal("intentional failure to verify retention")
	})
	t.Run("late-cleanup-failure", func(t *testing.T) {
		// Registered before the fixture, so this runs after its cleanup (LIFO).
		t.Cleanup(func() { t.Error("intentional late cleanup failure") })
		db := NewPostgresTestDB(t, nil, nil)
		require.NoError(t, db.Exec("CREATE TABLE retained_row (value text); INSERT INTO retained_row VALUES ('survives process exit')").Error)
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
