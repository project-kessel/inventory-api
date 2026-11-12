// Package relationstest provides isolated real SpiceDB repositories for tests.
// Each test binary shares a lazily started serve-testing container; each New
// uses a unique bearer token, which selects a separate in-memory datastore.
// TEST_DB=sqlite explicitly skips SpiceDB tests. Other backends require a
// healthy Docker or Podman provider; unavailability fails the test.
// Call TerminateSharedSpiceDB from TestMain after m.Run, checking its error.
package relationstest

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/testutil"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

//go:embed schema.zed
var schema []byte

var shared struct {
	sync.Mutex
	container *testcontainers.DockerContainer
	endpoint  string
}

var containerRuntimeHealth = sync.OnceValue(checkContainerRuntime)

// checkContainerRuntime provides a cached availability error before startup.
// A missing runtime is a failure unless SQLite explicitly excludes SpiceDB.
func checkContainerRuntime() (err error) {
	// Testcontainers can panic while discovering a missing runtime. Limit recovery
	// to this availability probe; never hide panics from starting or using SpiceDB.
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("discover container runtime: %v", recovered)
		}
	}()
	provider, err := testcontainers.ProviderDocker.GetProvider()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return provider.Health(ctx)
}

// New returns a real repository backed by a private serve-testing datastore.
// It skips the calling test only when TEST_DB=sqlite explicitly excludes SpiceDB.
// The schema is initialized once for that datastore before returning. Default
// reads are fully consistent; explicit consistency requests retain their meaning.
// Repository connections are closed by t.Cleanup, not the shared container.
func New(t testing.TB) *data.SpiceDBRelationsRepository {
	t.Helper()
	if testutil.TestDBBackend() == testutil.BackendSQLite {
		t.Skip("SpiceDB tests are explicitly disabled with TEST_DB=sqlite")
	}
	require.NoError(t, containerRuntimeHealth(), "SpiceDB tests require a running Docker or Podman runtime")
	endpoint, err := sharedEndpoint()
	require.NoError(t, err, "start shared SpiceDB")

	schemaPath := filepath.Join(t.TempDir(), "schema.zed")
	require.NoError(t, os.WriteFile(schemaPath, schema, 0600))
	repo, cleanup, err := data.NewSpiceDBRelationsRepository(&data.SpiceDBConfig{
		Endpoint:        endpoint,
		Token:           uuid.NewString(),
		SchemaFile:      schemaPath,
		FullyConsistent: true,
	}, log.NewStdLogger(io.Discard))
	require.NoError(t, err, "create isolated SpiceDB repository")
	t.Cleanup(cleanup)

	// A read initializes the schema without seeding any relationships. The
	// repository's initialization gate prevents subsequent schema rewrites.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = repo.Check(ctx, principalRelationship(t, "schema-probe", "view", "hbi", "host", "schema-probe"), nil)
	require.NoError(t, err, "initialize test SpiceDB schema")
	return repo
}

func sharedEndpoint() (string, error) {
	shared.Lock()
	defer shared.Unlock()
	if shared.container != nil {
		if shared.endpoint == "" {
			return "", fmt.Errorf("shared SpiceDB endpoint discovery previously failed")
		}
		return shared.endpoint, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := testcontainers.Run(ctx, data.SpicedbImage+":"+data.SpicedbVersion,
		testcontainers.WithCmd("serve-testing", "--skip-release-check=true"),
		testcontainers.WithExposedPorts("50051/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("50051/tcp").WithStartupTimeout(2*time.Minute)),
	)
	if err != nil {
		return "", fmt.Errorf("start SpiceDB: %w", err)
	}
	// Keep ownership even if endpoint discovery fails, so TestMain can clean up.
	shared.container = container
	host, err := container.Host(ctx)
	if err != nil {
		return "", fmt.Errorf("get SpiceDB host: %w", err)
	}
	port, err := container.MappedPort(ctx, "50051/tcp")
	if err != nil {
		return "", fmt.Errorf("get SpiceDB port: %w", err)
	}
	shared.endpoint = net.JoinHostPort(host, port.Port())
	return shared.endpoint, nil
}

// Grant writes a real relationship from the resource to rbac/principal subjectID
// and returns SpiceDB's write token. Pass an unprefixed relation (e.g. "view",
// "edit", "update"); the production adapter supplies its storage prefix.
// Computed permissions must be granted through their schema-defined relations.
func Grant(t testing.TB, repo model.RelationsRepository, subjectID, relation, reporterType, resourceType, resourceID string) model.ConsistencyToken {
	t.Helper()
	relationship := principalRelationship(t, subjectID, relation, reporterType, resourceType, resourceID)
	tuple := model.NewRelationsTuple(relationship.Object(), relationship.Relation(), relationship.Subject())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := repo.CreateTuples(ctx, []model.RelationsTuple{tuple}, true, nil)
	require.NoError(t, err, "grant real SpiceDB relationship")
	return result.ConsistencyToken()
}

func principalRelationship(t testing.TB, subjectID, relation, reporterType, resourceType, resourceID string) model.Relationship {
	t.Helper()
	relationValue, err := model.NewRelation(relation)
	require.NoError(t, err)
	return model.NewRelationship(resourceReference(t, reporterType, resourceType, resourceID), relationValue,
		model.NewSubjectReferenceWithoutRelation(resourceReference(t, "rbac", "principal", subjectID)))
}

func resourceReference(t testing.TB, reporterType, resourceType, resourceID string) model.ResourceReference {
	t.Helper()
	reporter, err := model.NewReporterType(reporterType)
	require.NoError(t, err)
	typeValue, err := model.NewResourceType(resourceType)
	require.NoError(t, err)
	id, err := model.NewLocalResourceId(resourceID)
	require.NoError(t, err)
	reporterReference := model.NewReporterReference(reporter, nil)
	return model.NewResourceReference(typeValue, id, &reporterReference)
}

// TerminateSharedSpiceDB stops this binary's shared container, if started.
// Call only after all tests and their repository cleanups have completed.
// It is safe to call repeatedly; failed termination can be retried.
func TerminateSharedSpiceDB() error {
	shared.Lock()
	defer shared.Unlock()
	if shared.container == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := shared.container.Terminate(ctx); err != nil {
		return fmt.Errorf("terminate shared SpiceDB: %w", err)
	}
	shared.container = nil
	shared.endpoint = ""
	return nil
}
