package relationstest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/testutil"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMain(m *testing.M) {
	code := m.Run()
	if err := TerminateSharedSpiceDB(); err != nil {
		fmt.Fprintf(os.Stderr, "SpiceDB test cleanup failed: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func TestNewSkipsForSQLiteWithoutProbingRuntime(t *testing.T) {
	// Keep this test non-parallel: restore the process-wide probe before the
	// parallel integration tests start.
	t.Setenv(testutil.TestDBBackendEnvVar, testutil.BackendSQLite)
	original := containerRuntimeHealth
	var probed bool
	containerRuntimeHealth = func() error {
		probed = true
		return errors.New("runtime must not be probed for SQLite")
	}
	t.Cleanup(func() { containerRuntimeHealth = original })

	var skipped bool
	t.Run("SQLite", func(t *testing.T) {
		defer func() { skipped = t.Skipped() }()
		New(t)
		t.Fatal("New must skip before attempting to start SpiceDB")
	})
	require.True(t, skipped, "SQLite intentionally excludes SpiceDB")
	require.False(t, probed, "SQLite must not depend on runtime availability")
}

func TestNewFailsWithoutRuntimeOutsideSQLite(t *testing.T) {
	const childEnv = "INVENTORY_TEST_REQUIRE_RUNTIME_CHILD"
	if os.Getenv(childEnv) == "1" {
		containerRuntimeHealth = func() error { return errors.New("no reachable container runtime") }
		New(t)
		t.Fatal("New must fail when the required runtime is unavailable")
		return
	}

	// A subprocess lets us verify FailNow and the test binary's nonzero exit,
	// rather than weakening the required-runtime path to make it testable.
	for _, backend := range []string{"", testutil.BackendPostgres} {
		t.Run("backend="+backend, func(t *testing.T) {
			t.Setenv(childEnv, "1")
			t.Setenv(testutil.TestDBBackendEnvVar, backend)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNewFailsWithoutRuntimeOutsideSQLite$", "-test.v")
			output, err := command.CombinedOutput()
			require.NoError(t, ctx.Err(), "runtime failure test timed out")
			require.Error(t, err, "missing required runtime must fail the test binary")
			require.Contains(t, string(output), "no reachable container runtime")
			require.Contains(t, string(output), "--- FAIL: TestNewFailsWithoutRuntimeOutsideSQLite")
			require.NotContains(t, string(output), "--- SKIP")
		})
	}
}

func TestConsumerWorkspaceTuplesAndFencing(t *testing.T) {
	t.Parallel()
	repo := New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lockID, err := model.NewLockId("consumer-partition-0")
	require.NoError(t, err)
	lock, err := repo.AcquireLock(ctx, lockID)
	require.NoError(t, err)
	fence := model.NewFencingCheck(lockID, lock.LockToken())
	workspace := resourceReference(t, "rbac", "workspace", "workspace-1")
	workspaceRelation, err := model.NewRelation("workspace")
	require.NoError(t, err)
	parentRelation, err := model.NewRelation("parent")
	require.NoError(t, err)
	tuples := []model.RelationsTuple{
		model.NewRelationsTuple(resourceReference(t, "notifications", "integration", "integration-1"), workspaceRelation, model.NewSubjectReferenceWithoutRelation(workspace)),
		model.NewRelationsTuple(resourceReference(t, "hbi", "host", "host-1"), workspaceRelation, model.NewSubjectReferenceWithoutRelation(workspace)),
		model.NewRelationsTuple(workspace, parentRelation, model.NewSubjectReferenceWithoutRelation(resourceReference(t, "rbac", "workspace", "root"))),
	}
	_, err = repo.CreateTuples(ctx, tuples, true, &fence)
	require.NoError(t, err)
	for _, tuple := range tuples {
		object := tuple.Object()
		filter := model.NewTupleFilter().WithReporterType(object.Reporter().ReporterType()).WithObjectType(object.ResourceType()).WithObjectId(object.ResourceId()).WithRelation(tuple.Relation())
		stream, err := repo.ReadTuples(ctx, filter, model.NewPagination(10, nil), nil)
		require.NoError(t, err)
		_, err = stream.Recv()
		require.NoError(t, err)
		_, err = stream.Recv()
		require.ErrorIs(t, err, io.EOF)
	}
	_, err = repo.AcquireLock(ctx, lockID)
	require.NoError(t, err)
	_, err = repo.CreateTuples(ctx, tuples, true, &fence)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "old lock tokens must be rejected by SpiceDB")
}

func TestNewIsolatesDatastores(t *testing.T) {
	t.Parallel()
	first := New(t)
	second := New(t)
	token := Grant(t, first, "alice", "view", "hbi", "host", "host-1")
	require.NotEmpty(t, token.Serialize())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	relationship := principalRelationship(t, "alice", "view", "hbi", "host", "host-1")
	allowed, err := first.Check(ctx, relationship, nil)
	require.NoError(t, err)
	require.True(t, allowed.Allowed())
	denied, err := second.Check(ctx, relationship, nil)
	require.NoError(t, err)
	require.False(t, denied.Allowed())
}

func TestRealComputedPermissions(t *testing.T) {
	t.Parallel()
	repo := New(t)
	// Editing implies viewing through the schema, without writing a view tuple.
	token := Grant(t, repo, "alice", "edit", "hbi", "host", "host-1")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, permission := range []string{"edit", "view"} {
		result, err := repo.Check(ctx, principalRelationship(t, "alice", permission, "hbi", "host", "host-1"), model.NewConsistencyAtLeastAsFresh(token))
		require.NoError(t, err)
		require.True(t, result.Allowed(), permission)
	}
	result, err := repo.Check(ctx, principalRelationship(t, "bob", "view", "hbi", "host", "host-1"), nil)
	require.NoError(t, err)
	require.False(t, result.Allowed())

	object := resourceReference(t, "hbi", "host", "host-1")
	filter := model.NewTupleFilter().WithReporterType(object.Reporter().ReporterType()).WithObjectType(object.ResourceType()).WithObjectId(object.ResourceId())
	_, err = repo.DeleteTuples(ctx, filter, nil)
	require.NoError(t, err)
	result, err = repo.Check(ctx, principalRelationship(t, "alice", "view", "hbi", "host", "host-1"), nil)
	require.NoError(t, err)
	require.False(t, result.Allowed(), "default reads must observe deletion without a sleep")
}
