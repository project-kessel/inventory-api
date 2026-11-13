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

func TestSpiceDBDebugLifecycle(t *testing.T) {
	if !testutil.UsingPostgres() {
		t.Skip("SQLite explicitly excludes SpiceDB")
	}
	if os.Getenv("TEST_SPICEDB_DEBUG_CHILD") == "1" {
		repo := New(t)
		Grant(t, repo, "alice", "edit", "hbi", "host", "retained-host")
		require.NoError(t, os.WriteFile(filepath.Join(os.Getenv("TEST_SPICEDB_DEBUG_DIRECTORY"), "container-id"), []byte(shared.container.GetContainerID()), 0600))
		if mode := os.Getenv("TEST_SPICEDB_DEBUG_MODE"); mode == "retain" || mode == "debug-failing" {
			t.Fatal("intentional failure to verify container retention")
		}
		return
	}
	require.NoError(t, containerdebug.Configure())
	for _, mode := range []string{"retain", "keep-passing", "debug-failing", "no-debug"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
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
			if mode == "retain" || mode == "keep-passing" {
				command.Env = append(command.Env, "TEST_CONTAINERS_KEEP_ON_FAILURE=1")
			}
			// This cleanup also removes unexpected leftovers before reporting a failure.
			t.Cleanup(func() {
				id, err := os.ReadFile(filepath.Join(directory, "container-id"))
				if os.IsNotExist(err) {
					return
				}
				require.NoError(t, err)
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cleanupCancel()
				runtime, err := testcontainers.NewDockerClientWithOpts(cleanupCtx)
				require.NoError(t, err)
				defer runtime.Close()
				_, inspectErr := runtime.ContainerInspect(cleanupCtx, string(id), client.ContainerInspectOptions{})
				if inspectErr == nil {
					_, err = runtime.ContainerRemove(cleanupCtx, string(id), client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
					require.NoError(t, err)
				}
				if mode == "retain" {
					require.NoError(t, inspectErr, "failed test container must survive process exit")
				} else {
					require.True(t, errdefs.IsNotFound(inspectErr), "container must be removed without a failure-retention request")
				}
			})
			output, err := command.CombinedOutput()
			require.NoError(t, ctx.Err(), "subprocess timed out")
			if mode == "retain" || mode == "debug-failing" {
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
			paths, err := filepath.Glob(filepath.Join(directory, "connections.*.jsonl"))
			require.NoError(t, err)
			require.Len(t, paths, 1)
			info, err := os.Stat(paths[0])
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			file, err := os.Open(paths[0])
			require.NoError(t, err)
			defer file.Close()
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
			require.Equal(t, mode == "retain", retained)
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
