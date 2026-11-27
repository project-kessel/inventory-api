package relationstest

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/moby/moby/client"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/testutil"
	"github.com/project-kessel/inventory-api/internal/testutil/containerdebug"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

func spiceDBDebugRegistryFiles(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		matched, err := filepath.Match("connections.*.jsonl", entry.Name())
		if err != nil {
			return nil, err
		}
		if matched && !entry.IsDir() {
			paths = append(paths, filepath.Join(directory, entry.Name()))
		}
	}
	return paths, nil
}

// spiceDBDebugContainerIDs recovers registry IDs even when setup failed before
// the child could write container-id. Discovery errors must not discard IDs
// already found or prevent cleanup of the other registry files.
func spiceDBDebugContainerIDs(t *testing.T, directory string) []string {
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
	id, err := os.ReadFile(filepath.Join(directory, "container-id"))
	if err == nil {
		add(string(id))
	} else if !os.IsNotExist(err) {
		t.Errorf("read child container ID: %v", err)
	}
	paths, err := spiceDBDebugRegistryFiles(directory)
	if err != nil {
		t.Errorf("find child registries: %v", err)
	}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			t.Errorf("open child registry: %v", err)
			continue
		}
		decoder := json.NewDecoder(file)
		for {
			var record struct {
				Kind        string `json:"kind"`
				ContainerID string `json:"container_id"`
			}
			if err := decoder.Decode(&record); err != nil {
				if err != io.EOF {
					t.Errorf("decode child registry: %v", err)
				}
				break
			}
			if record.Kind == "spicedb" {
				add(record.ContainerID)
			}
		}
		if err := file.Close(); err != nil {
			t.Errorf("close child registry: %v", err)
		}
	}
	return ids
}

// cleanupSpiceDBDebugChild never lets lifecycle assertions gate removal. Each
// phase gets a fresh deadline, and the runtime independently verifies absence
// after removal rather than trusting a container handle or the registry.
func cleanupSpiceDBDebugChild(t *testing.T, directory string, wantRetained bool) {
	t.Helper()
	ids := spiceDBDebugContainerIDs(t, directory)
	if len(ids) == 0 {
		t.Error("child did not record a container ID in container-id or its registry")
		return
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

		// Attempt force removal even if inspection failed for an unknown reason.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, removeErr := runtime.ContainerRemove(cleanupCtx, id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		cleanupCancel()
		verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, verifyErr := runtime.ContainerInspect(verifyCtx, id, client.ContainerInspectOptions{})
		verifyCancel()

		if removeErr != nil && !errdefs.IsNotFound(removeErr) {
			t.Errorf("remove child container %s (remove manually): %v", id, removeErr)
		}
		if !errdefs.IsNotFound(verifyErr) {
			t.Errorf("child container %s removal not verified (remove manually): %v", id, verifyErr)
		}
		if wantRetained {
			if inspectErr != nil {
				t.Errorf("failed test container %s must survive process exit: %v", id, inspectErr)
			}
		} else if !errdefs.IsNotFound(inspectErr) {
			t.Errorf("container %s must be removed without a failure-retention request: %v", id, inspectErr)
		}
	}
}

func TestSpiceDBDebugContainerIDs(t *testing.T) {
	for _, withIDFile := range []bool{false, true} {
		name := "registry-only"
		if withIDFile {
			name = "with-id-file"
		}
		t.Run(name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "literal[")
			require.NoError(t, os.Mkdir(directory, 0700))
			if withIDFile {
				require.NoError(t, os.WriteFile(filepath.Join(directory, "container-id"), []byte("spicedb-id\n"), 0600))
			}
			registry := `{"type":"container","kind":"spicedb","container_id":"spicedb-id"}
{"type":"fixture","kind":"spicedb","container_id":"spicedb-id"}
{"type":"container","kind":"postgres","container_id":"other-id"}
`
			require.NoError(t, os.WriteFile(filepath.Join(directory, "connections.child.1.jsonl"), []byte(registry), 0600))
			require.Equal(t, []string{"spicedb-id"}, spiceDBDebugContainerIDs(t, directory))
		})
	}
}

func TestSpiceDBDebugLifecycle(t *testing.T) {
	if !testutil.UsingPostgres() {
		t.Skip("SQLite explicitly excludes SpiceDB")
	}
	if os.Getenv("TEST_SPICEDB_DEBUG_CHILD") == "1" {
		repo := New(t)
		Grant(t, repo, "alice", "edit", "hbi", "host", "retained-host")
		t.Log("spicedb-debug: grant completed")
		mode := os.Getenv("TEST_SPICEDB_DEBUG_MODE")
		if mode != "retain-registry-only" {
			require.NoError(t, os.WriteFile(filepath.Join(os.Getenv("TEST_SPICEDB_DEBUG_DIRECTORY"), "container-id"), []byte(shared.container.GetContainerID()), 0600))
		}
		if mode == "retain" || mode == "retain-registry-only" || mode == "debug-failing" {
			t.Fatal("intentional failure to verify container retention")
		}
		return
	}
	require.NoError(t, containerdebug.Configure())
	for _, mode := range []string{"retain", "retain-registry-only", "keep-passing", "debug-failing", "no-debug"} {
		t.Run(mode, func(t *testing.T) {
			wantRetained := mode == "retain" || mode == "retain-registry-only"
			directory := filepath.Join(t.TempDir(), "literal[")
			require.NoError(t, os.Mkdir(directory, 0700))
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSpiceDBDebugLifecycle$", "-test.v")
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "TEST_CONTAINERS_") && !strings.HasPrefix(entry, "TEST_SPICEDB_DEBUG_") {
					command.Env = append(command.Env, entry)
				}
			}
			command.Env = append(command.Env, "TEST_SPICEDB_DEBUG_CHILD=1", "TEST_SPICEDB_DEBUG_MODE="+mode,
				"TEST_SPICEDB_DEBUG_DIRECTORY="+directory)
			if mode != "no-debug" {
				command.Env = append(command.Env, "TEST_CONTAINERS_DEBUG_FILE="+filepath.Join(directory, "connections"))
			}
			if wantRetained || mode == "keep-passing" {
				command.Env = append(command.Env, "TEST_CONTAINERS_KEEP_ON_FAILURE=1")
			}
			// Register before launching: even an early assertion failure must recover
			// recorded IDs and attempt removal with an independent context.
			t.Cleanup(func() { cleanupSpiceDBDebugChild(t, directory, wantRetained) })
			output, err := command.CombinedOutput()
			if mode == "retain-registry-only" {
				require.NoFileExists(t, filepath.Join(directory, "container-id"), "cleanup must recover this child's ID from its registry")
			}
			require.NoError(t, ctx.Err(), "subprocess timed out")
			require.Contains(t, string(output), "spicedb-debug: grant completed", "child must reach the real relationship write: %s", output)
			if wantRetained || mode == "debug-failing" {
				require.Contains(t, string(output), "intentional failure to verify container retention")
				var exitError *exec.ExitError
				require.ErrorAs(t, err, &exitError, "%s", output)
				require.Equal(t, 1, exitError.ExitCode())
			} else {
				require.NoError(t, err, "%s", output)
			}
			if mode == "no-debug" {
				require.NotContains(t, string(output), "containerdebug:")
				return
			}
			paths, err := spiceDBDebugRegistryFiles(directory)
			require.NoError(t, err)
			require.Len(t, paths, 1)
			info, err := os.Stat(paths[0])
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			file, err := os.Open(paths[0])
			require.NoError(t, err)
			defer func() { require.NoError(t, file.Close()) }()
			var endpoint, token string
			retained := false
			decoder := json.NewDecoder(file)
			for {
				var record struct {
					Type     string `json:"type"`
					Endpoint string `json:"endpoint"`
					Token    string `json:"token"`
					Test     string `json:"test"`
				}
				if err := decoder.Decode(&record); err == io.EOF {
					break
				} else {
					require.NoError(t, err)
				}
				switch record.Type {
				case "container":
					endpoint = record.Endpoint
				case "fixture":
					token = record.Token
					require.Equal(t, "TestSpiceDBDebugLifecycle", record.Test)
				case "retained":
					retained = true
				}
			}
			require.NotEmpty(t, endpoint)
			require.NotEmpty(t, token)
			require.False(t, strings.Contains(string(output), token), "stdout must not contain the datastore token")
			require.Equal(t, wantRetained, retained)
			if !retained {
				return
			}
			require.Contains(t, string(output), "podman rm -f")
			// Reconnect solely with the recorded credentials and inspect the exact
			// datastore's computed permission after the test process has exited.
			schemaPath := filepath.Join(directory, "schema.zed")
			require.NoError(t, os.WriteFile(schemaPath, schema, 0600))
			repo, cleanup, err := data.NewSpiceDBRelationsRepository(&data.SpiceDBConfig{
				Endpoint: endpoint, Token: token, SchemaFile: schemaPath, FullyConsistent: true,
			}, log.NewStdLogger(io.Discard))
			require.NoError(t, err)
			t.Cleanup(cleanup)
			result, err := repo.Check(ctx, principalRelationship(t, "alice", "view", "hbi", "host", "retained-host"), nil)
			require.NoError(t, err)
			require.True(t, result.Allowed(), "retained SpiceDB must preserve real relationship data")
		})
	}
}
