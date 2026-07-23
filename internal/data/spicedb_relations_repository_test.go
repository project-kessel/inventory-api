package data

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var container *LocalSpiceDbContainer

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	flag.Parse()

	// Only set up SpiceDB container if not in short mode
	// Individual tests will skip themselves in short mode
	if !testing.Short() {
		var err error
		logger := log.With(log.NewStdLogger(os.Stdout),
			"ts", log.DefaultTimestamp,
			"caller", log.DefaultCaller,
			"trace.id", tracing.TraceID(),
			"span.id", tracing.SpanID(),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		container, err = CreateContainer(ctx, &ContainerOptions{Logger: logger})

		if err != nil {
			fmt.Printf("Error initializing Docker container: %s", err)
			return -1
		}
		defer container.Close()
	}

	return m.Run()
}

// requireSpiceDBIntegration skips the test if running in short mode or if container is not available
func requireSpiceDBIntegration(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping SpiceDB integration test in short mode")
	}
	if container == nil {
		t.Fatal("SpiceDB container not initialized")
	}
}

// uniqueID generates a unique identifier for the test by combining the base ID with the test name
func uniqueID(t *testing.T, baseID string) string {
	t.Helper()
	// Use a short hash of test name to keep IDs readable
	testName := strings.ReplaceAll(t.Name(), "/", "_")
	return fmt.Sprintf("%s_%s", baseID, testName)
}

func TestCreateRelationship(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	preExisting := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", uniqueID(t, "bob_club"), model.NewConsistencyUnspecified())
	assert.False(t, preExisting)

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, nil)
	assert.NoError(t, err)

	container.WaitForQuantizationInterval()

	exists := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", uniqueID(t, "bob_club"), model.NewConsistencyUnspecified())
	assert.True(t, exists)
}

func TestCreateRelationshipWithConsistencyToken(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	preExisting := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", uniqueID(t, "bob_club"), model.NewConsistencyUnspecified())
	assert.False(t, preExisting)

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
	}

	resp, err := spiceDbRepo.CreateTuples(ctx, tuples, false, nil)
	assert.NoError(t, err)

	exists := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", uniqueID(t, "bob_club"),
		model.NewConsistencyAtLeastAsFresh(resp.ConsistencyToken()))
	assert.True(t, exists)
}

func TestCreateRelationshipWithSubjectRelation(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	preExisting := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", uniqueID(t, "bob_club"), model.NewConsistencyUnspecified())
	assert.False(t, preExisting)

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "role_binding", "fan_binding", "granted", "rbac", "role", "fan", ""),
		createRelationship("rbac", "role_binding", "fan_binding", "subject", "rbac", "group", uniqueID(t, "bob_club"), "member"),
		createRelationship("rbac", "role", "fan", "view_widget", "rbac", "principal", "*", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, nil)
	assert.NoError(t, err)

	container.WaitForQuantizationInterval()

	exists := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", uniqueID(t, "bob_club"), model.NewConsistencyUnspecified())
	assert.True(t, exists)

	exists = CheckForRelationship(spiceDbRepo, uniqueID(t, "bob_club"), "rbac", "group", "member", "subject", "rbac", "role_binding", "fan_binding", model.NewConsistencyUnspecified())
	assert.True(t, exists)

	// zed permission check rbac/role_binding:fan_binding subject rbac/principal:bob
	// bob is a subject of fan_binding
	runSpiceDBCheck(t, ctx, spiceDbRepo, "principal", "rbac", "bob", "subject", "role_binding", "rbac", "fan_binding", true)

	// zed permission check rbac/role_binding:fan_binding subject rbac/principal:alice
	// alice is NOT a subject of fan_binding
	runSpiceDBCheck(t, ctx, spiceDbRepo, "principal", "rbac", "alice", "subject", "role_binding", "rbac", "fan_binding", false)

	// zed permission check rbac/role_binding:fan_binding view_widget rbac/principal:bob
	// bob has view_widget permission
	runSpiceDBCheck(t, ctx, spiceDbRepo, "principal", "rbac", "bob", "view_widget", "role_binding", "rbac", "fan_binding", true)

	// zed permission check rbac/role_binding:fan_binding subject rbac/principal:alice
	// alice does NOT have view_widget permission
	runSpiceDBCheck(t, ctx, spiceDbRepo, "principal", "rbac", "alice", "view_widget", "role_binding", "rbac", "fan_binding", false)

	// zed permission check rbac/role_binding:fan_binding t_granted rbac/role:fan
	// check that role binding is tied to correct role
	runSpiceDBCheck(t, ctx, spiceDbRepo, "role", "rbac", "fan", "granted", "role_binding", "rbac", "fan_binding", true)

	// zed permission check rbac/role_binding:fan_binding t_granted rbac/role:fake_fan
	// check for non-existent role not tied to role binding
	runSpiceDBCheck(t, ctx, spiceDbRepo, "role", "rbac", "fake_fan", "granted", "role_binding", "rbac", "fan_binding", false)
}

func TestSecondCreateRelationshipFailsWithUpsertFalse(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	preExisting := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", uniqueID(t, "bob_club"), model.NewConsistencyUnspecified())
	assert.False(t, preExisting)

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, nil)
	assert.NoError(t, err)

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, nil)
	assert.Error(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Convert(err).Code())

	container.WaitForQuantizationInterval()

	exists := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", uniqueID(t, "bob_club"), model.NewConsistencyUnspecified())
	assert.True(t, exists)
}

func TestSecondCreateRelationshipSucceedsWithUpsertTrue(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	preExisting := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", uniqueID(t, "bob_club"), model.NewConsistencyUnspecified())
	assert.False(t, preExisting)

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, nil)
	assert.NoError(t, err)

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	assert.NoError(t, err)

	container.WaitForQuantizationInterval()

	exists := CheckForRelationship(spiceDbRepo, "bob", "rbac", "principal", "", "member", "rbac", "group", uniqueID(t, "bob_club"), model.NewConsistencyUnspecified())
	assert.True(t, exists)
}

func TestIsBackendAvailable(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	spiceDbrepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbrepo.Close()

	healthResult, err := spiceDbrepo.Health(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, healthResult)
}

func TestIsBackendUnavailable(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	config := &SpiceDBConfig{
		Endpoint:        "-1",
		Token:           "foobar",
		UseTLS:          true,
		FullyConsistent: false,
	}
	spiceDBRepo, _, err := NewSpiceDBRelationsRepository(config, log.GetLogger())
	assert.NoError(t, err)

	_, err = spiceDBRepo.Health(context.Background())
	assert.Error(t, err)
}

func TestEffectiveLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		pagination   *model.Pagination
		defaultLimit uint32
		expected     uint32
	}{
		{
			name:         "nil pagination uses default",
			pagination:   nil,
			defaultLimit: 1000,
			expected:     1000,
		},
		{
			name:         "zero limit uses default",
			pagination:   model.NewPagination(0, nil),
			defaultLimit: 1000,
			expected:     1000,
		},
		{
			name:         "client limit smaller than default is used",
			pagination:   model.NewPagination(100, nil),
			defaultLimit: 1000,
			expected:     100,
		},
		{
			name:         "client limit equal to default is used",
			pagination:   model.NewPagination(1000, nil),
			defaultLimit: 1000,
			expected:     1000,
		},
		{
			name:         "client limit larger than default uses default",
			pagination:   model.NewPagination(5000, nil),
			defaultLimit: 1000,
			expected:     1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := effectiveLimit(tt.pagination, tt.defaultLimit)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDoesNotCreateRelationshipWithSlashInSubjectType(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	badSubjectType := "special/user"

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", badSubjectType, "bob", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, nil)
	assert.Error(t, err)
}

func TestDoesNotCreateRelationshipWithSlashInObjectType(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	badResourceType := "my/group"

	tuples := []model.RelationsTuple{
		createRelationship("rbac", badResourceType, uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, nil)
	assert.Error(t, err)
}

func TestCreateRelationshipFailsWithBadSubjectType(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	badSubjectType := "not_a_user"

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", badSubjectType, "bob", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, nil)
	assert.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Convert(err).Code())
	assert.Contains(t, err.Error(),
		fmt.Sprintf("object definition `%s/%s` not found", "rbac", badSubjectType))
}

func TestCreateRelationshipFailsWithBadObjectType(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	badObjectType := "not_an_object"

	tuples := []model.RelationsTuple{
		createRelationship("rbac", badObjectType, uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, nil)
	assert.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Convert(err).Code())
	assert.Contains(t, err.Error(),
		fmt.Sprintf("object definition `%s/%s` not found", "rbac", badObjectType))
}

func TestSupportedNsTypeTupleFilterCombinationsInReadRelationships(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	// Test 1: Has resource type but missing resource namespace - should error
	filter := model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithObjectType(model.DeserializeResourceType("group")).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")).
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	_, err = spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	assert.Error(t, err)

	// Test 2: Has resource namespace but missing resource type - should error
	filter = model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")).
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	_, err = spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	assert.Error(t, err)

	// Test 3: Has subject type but missing subject namespace - should error
	filter = model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	_, err = spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	assert.Error(t, err)

	// Test 4: Has subject namespace but missing subject type - should error
	filter = model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")).
			WithReporterType(model.DeserializeReporterType("rbac")))

	_, err = spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	assert.Error(t, err)

	// Test 5: All required fields present - should succeed
	filter = model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")).
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	_, err = spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	assert.NoError(t, err)

	// Test 6: Optional resource type missing (only ID and relation) - should succeed
	filter = model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")).
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	_, err = spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	assert.NoError(t, err)

	// Test 7: Optional subject type and namespace missing - should succeed
	filter = model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")))

	_, err = spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	assert.NoError(t, err)

	// Test 8: Minimal filter (only ID and relation) - should succeed
	filter = model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")))

	_, err = spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	assert.NoError(t, err)

	// Test 9: With full type info but without subject ID - should succeed
	filter = model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")))

	_, err = spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	assert.NoError(t, err)
}

func TestWriteAndReadBackRelationships(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	container.WaitForQuantizationInterval()

	filter := model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")).
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	stream, err := spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	if !assert.NoError(t, err) {
		return
	}

	readrels := readTuplesStreamToSlice(stream)
	assert.Equal(t, 1, len(readrels))
}

func TestWriteReadBackDeleteAndReadBackRelationships(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	container.WaitForQuantizationInterval()

	filter := model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")).
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	stream, err := spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	if !assert.NoError(t, err) {
		return
	}

	readrels := readTuplesStreamToSlice(stream)
	assert.Equal(t, 1, len(readrels))

	_, err = spiceDbRepo.DeleteTuples(ctx, filter, nil)
	if !assert.NoError(t, err) {
		return
	}

	container.WaitForQuantizationInterval()

	stream, err = spiceDbRepo.ReadTuples(ctx, filter, nil, model.NewConsistencyUnspecified())
	if !assert.NoError(t, err) {
		return
	}

	readrels = readTuplesStreamToSlice(stream)
	assert.Equal(t, 0, len(readrels))
}

func TestWriteReadBackDeleteAndReadBackRelationships_WithConsistencyToken(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
	}

	respCreate, err := spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	filter := model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId(uniqueID(t, "bob_club"))).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")).
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	stream, err := spiceDbRepo.ReadTuples(ctx, filter, nil,
		model.NewConsistencyAtLeastAsFresh(respCreate.ConsistencyToken()))
	if !assert.NoError(t, err) {
		return
	}

	readrels := readTuplesStreamToSlice(stream)
	assert.Equal(t, 1, len(readrels))

	respDelete, err := spiceDbRepo.DeleteTuples(ctx, filter, nil)
	if !assert.NoError(t, err) {
		return
	}

	stream, err = spiceDbRepo.ReadTuples(ctx, filter, nil,
		model.NewConsistencyAtLeastAsFresh(respDelete.ConsistencyToken()))
	if !assert.NoError(t, err) {
		return
	}

	readrels = readTuplesStreamToSlice(stream)
	assert.Equal(t, 0, len(readrels))
}

func TestSpiceDbRepository_CheckPermission_WithConsistencyToken(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "workspace", uniqueID(t, "test"), "user_grant", "rbac", "role_binding", "rb_test", ""),
		createRelationship("rbac", "role_binding", "rb_test", "granted", "rbac", "role", "rl1", ""),
		createRelationship("rbac", "role_binding", "rb_test", "subject", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "role", "rl1", "view_widget", "rbac", "principal", "*", ""),
	}

	relationshipResp, err := spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	subject := createSubjectReference("rbac", "principal", "bob")
	resource := createResourceReference("rbac", "workspace", uniqueID(t, "test"))

	// no wait, immediately read after write.
	// zed permission check rbac/workspace:test view_widget rbac/principal:bob --explain
	rel := model.NewRelationship(resource, model.DeserializeRelation("view_widget"), subject)
	resp, err := spiceDbRepo.Check(ctx, rel,
		model.NewConsistencyAtLeastAsFresh(relationshipResp.ConsistencyToken()))
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, resp.Allowed())
	assert.NotEmpty(t, resp.ConsistencyToken())
}

func TestSpiceDbRepository_CheckForUpdatePermission(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "workspace", uniqueID(t, "test"), "user_grant", "rbac", "role_binding", "rb_test", ""),
		createRelationship("rbac", "role_binding", "rb_test", "granted", "rbac", "role", "rl1", ""),
		createRelationship("rbac", "role_binding", "rb_test", "subject", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "role", "rl1", "view_widget", "rbac", "principal", "*", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	subject := createSubjectReference("rbac", "principal", "bob")
	resource := createResourceReference("rbac", "workspace", uniqueID(t, "test"))

	// no wait, immediately read after write.
	// zed permission check rbac/workspace:test view_widget rbac/principal:bob --explain
	rel := model.NewRelationship(resource, model.DeserializeRelation("view_widget"), subject)
	resp, err := spiceDbRepo.CheckForUpdate(ctx, rel)
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, resp.Allowed())
	assert.NotEmpty(t, resp.ConsistencyToken())
}

func TestSpiceDbRepository_CheckPermission(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "workspace", uniqueID(t, "test"), "user_grant", "rbac", "role_binding", "rb_test", ""),
		createRelationship("rbac", "role_binding", "rb_test", "granted", "rbac", "role", "rl1", ""),
		createRelationship("rbac", "role_binding", "rb_test", "subject", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "role", "rl1", "view_widget", "rbac", "principal", "*", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	container.WaitForQuantizationInterval()

	subject := createSubjectReference("rbac", "principal", "bob")
	resource := createResourceReference("rbac", "workspace", uniqueID(t, "test"))

	// zed permission check rbac/workspace:test view_widget rbac/principal:bob --explain
	rel := model.NewRelationship(resource, model.DeserializeRelation("view_widget"), subject)
	resp, err := spiceDbRepo.Check(ctx, rel, model.NewConsistencyUnspecified())
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, resp.Allowed())
	assert.NotEmpty(t, resp.ConsistencyToken())

	// Remove rbac/role_binding:rb_test#subject@rbac/principal:bob
	filter := model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId("rb_test")).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("role_binding")).
		WithRelation(model.DeserializeRelation("subject")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")).
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	_, err = spiceDbRepo.DeleteTuples(ctx, filter, nil)
	if !assert.NoError(t, err) {
		return
	}

	// zed permission check rbac/workspace:test view_widget rbac/principal:bob --explain
	resp2, err := spiceDbRepo.Check(ctx, rel, model.NewConsistencyUnspecified())
	if !assert.NoError(t, err) {
		return
	}

	assert.False(t, resp2.Allowed())
	assert.NotEmpty(t, resp2.ConsistencyToken())
}

func TestSpiceDbRepository_NewEnemyProblem_Success(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "workspace", uniqueID(t, "test"), "user_grant", "rbac", "role_binding", "rb_test", ""),
		createRelationship("rbac", "role_binding", "rb_test", "granted", "rbac", "role", "rl1", ""),
		createRelationship("rbac", "role_binding", "rb_test", "subject", "rbac", "principal", "u1", ""),
		createRelationship("rbac", "role_binding", "rb_test", "subject", "rbac", "principal", "u2", ""),
		createRelationship("rbac", "role", "rl1", "view_widget", "rbac", "principal", "*", ""),
	}

	relationshipResp, err := spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	resource := createResourceReference("rbac", "workspace", uniqueID(t, "test"))
	consistency := model.NewConsistencyAtLeastAsFresh(relationshipResp.ConsistencyToken())

	// u1
	u1Subject := createSubjectReference("rbac", "principal", "u1")
	u1Rel := model.NewRelationship(resource, model.DeserializeRelation("view_widget"), u1Subject)

	// no wait, immediately read after write.
	// zed permission check rbac/workspace:test view_widget rbac/principal:u1 --explain
	resp, err := spiceDbRepo.Check(ctx, u1Rel, consistency)
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, resp.Allowed())
	assert.NotEmpty(t, resp.ConsistencyToken())

	// u2
	u2Subject := createSubjectReference("rbac", "principal", "u2")
	u2Rel := model.NewRelationship(resource, model.DeserializeRelation("view_widget"), u2Subject)

	// zed permission check rbac/workspace:test view_widget rbac/principal:u2 --explain
	resp, err = spiceDbRepo.Check(ctx, u2Rel, consistency)
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, resp.Allowed())
	assert.NotEmpty(t, resp.ConsistencyToken())

	// remove access from u1, keep access for u2.
	filter := model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId("rb_test")).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("role_binding")).
		WithRelation(model.DeserializeRelation("subject")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("u1")).
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	respDelete, err := spiceDbRepo.DeleteTuples(ctx, filter, nil)
	if !assert.NoError(t, err) {
		return
	}

	// ensure u1 no longer has access, while u2 still does.
	deleteConsistency := model.NewConsistencyAtLeastAsFresh(respDelete.ConsistencyToken())

	// zed permission check rbac/workspace:test view_widget rbac/principal:u1 --explain
	resp, err = spiceDbRepo.Check(ctx, u1Rel, deleteConsistency)
	if !assert.NoError(t, err) {
		return
	}

	assert.False(t, resp.Allowed())
	assert.NotEmpty(t, resp.ConsistencyToken())

	// zed permission check rbac/workspace:test view_widget rbac/principal:u2 --explain
	resp, err = spiceDbRepo.Check(ctx, u2Rel, deleteConsistency)
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, resp.Allowed())
	assert.NotEmpty(t, resp.ConsistencyToken())
}

func TestSpiceDbRepository_CheckPermission_MinimizeLatency(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "workspace", uniqueID(t, "test"), "user_grant", "rbac", "role_binding", "rb_test", ""),
		createRelationship("rbac", "role_binding", "rb_test", "granted", "rbac", "role", "rl1", ""),
		createRelationship("rbac", "role_binding", "rb_test", "subject", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "role", "rl1", "view_widget", "rbac", "principal", "*", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	container.WaitForQuantizationInterval()

	subject := createSubjectReference("rbac", "principal", "bob")
	resource := createResourceReference("rbac", "workspace", uniqueID(t, "test"))

	// Test with minimize_latency = True.

	// zed permission check rbac/workspace:test view_widget rbac/principal:bob --explain
	rel := model.NewRelationship(resource, model.DeserializeRelation("view_widget"), subject)
	resp, err := spiceDbRepo.Check(ctx, rel, model.NewConsistencyMinimizeLatency())
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, resp.Allowed())
	assert.NotEmpty(t, resp.ConsistencyToken())

	// Remove rbac/role_binding:rb_test#subject@rbac/principal:bob
	filter := model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId("rb_test")).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("role_binding")).
		WithRelation(model.DeserializeRelation("subject")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("bob")).
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	_, err = spiceDbRepo.DeleteTuples(ctx, filter, nil)
	if !assert.NoError(t, err) {
		return
	}
}

func TestSpiceDbRepository_CheckBulk(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", uniqueID(t, "bob_club"), "member", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "workspace", uniqueID(t, "test"), "user_grant", "rbac", "role_binding", "rb_test", ""),
		createRelationship("rbac", "role_binding", "rb_test", "granted", "rbac", "role", "rl1", ""),
		createRelationship("rbac", "role_binding", "rb_test", "subject", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "role", "rl1", "view_widget", "rbac", "principal", "*", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	container.WaitForQuantizationInterval()

	resource := createResourceReference("rbac", "workspace", uniqueID(t, "test"))
	bobSubject := createSubjectReference("rbac", "principal", "bob")
	aliceSubject := createSubjectReference("rbac", "principal", "alice")

	rels := []model.Relationship{
		model.NewRelationship(resource, model.DeserializeRelation("view_widget"), bobSubject),
		model.NewRelationship(resource, model.DeserializeRelation("view_widget"), aliceSubject),
	}

	resp, err := spiceDbRepo.CheckBulk(ctx, rels, model.NewConsistencyUnspecified())
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, len(rels), len(resp.Pairs())) {
		return
	}

	results := map[string]bool{}
	for _, p := range resp.Pairs() {
		subjId := p.Request().Subject().Resource().ResourceId().String()
		results[subjId] = p.Result().Allowed()
	}
	assert.True(t, results["bob"])
	assert.False(t, results["alice"])
}

func TestSpiceDbRepository_CheckForUpdateBulk(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	// Unique IDs so this test does not collide with CheckBulk or other tests
	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	tuples := []model.RelationsTuple{
		createRelationship("rbac", "workspace", "checkforupdatebulk_workspace", "user_grant", "rbac", "role_binding", "checkforupdatebulk_role_binding", ""),
		createRelationship("rbac", "role_binding", "checkforupdatebulk_role_binding", "granted", "rbac", "role", "checkforupdatebulk_role", ""),
		createRelationship("rbac", "role_binding", "checkforupdatebulk_role_binding", "subject", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "role", "checkforupdatebulk_role", "view_widget", "rbac", "principal", "*", ""),
	}

	_, err = spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	// CheckForUpdateBulk is strongly consistent; no quantization wait needed.

	resource := createResourceReference("rbac", "workspace", "checkforupdatebulk_workspace")
	bobSubject := createSubjectReference("rbac", "principal", "bob")
	aliceSubject := createSubjectReference("rbac", "principal", "alice")

	rels := []model.Relationship{
		model.NewRelationship(resource, model.DeserializeRelation("view_widget"), bobSubject),
		model.NewRelationship(resource, model.DeserializeRelation("view_widget"), aliceSubject),
	}

	resp, err := spiceDbRepo.CheckForUpdateBulk(ctx, rels)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, len(rels), len(resp.Pairs())) {
		return
	}

	results := map[string]bool{}
	for _, p := range resp.Pairs() {
		subjId := p.Request().Subject().Resource().ResourceId().String()
		results[subjId] = p.Result().Allowed()
	}
	assert.True(t, results["bob"])
	assert.False(t, results["alice"])

	// Response includes consistency_token like CheckBulkResponse.
	assert.NotEmpty(t, resp.ConsistencyToken())
}

func TestSpiceDbRepository_CreateRelationships_WithFencing(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	// Acquire a lock to get a fencing token
	lockId, _ := model.NewLockId(uniqueID(t, "test-lock"))
	lockResp, err := spiceDbRepo.AcquireLock(ctx, lockId)
	assert.NoError(t, err)
	fencingToken := lockResp.LockToken()
	assert.NotEmpty(t, fencingToken)

	// Create a relationship with fencing
	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", "fencing_group", "member", "rbac", "principal", "fenced_bob", ""),
	}
	fencing := model.NewFencingCheck(lockId, fencingToken)
	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, &fencing)
	assert.NoError(t, err)

	container.WaitForQuantizationInterval()

	// Relationship should exist
	exists := CheckForRelationship(
		spiceDbRepo, "fenced_bob", "rbac", "principal", "", "member", "rbac", "group", "fencing_group", model.NewConsistencyUnspecified(),
	)
	assert.True(t, exists)

	// Try to create with an invalid fencing token
	badToken, _ := model.NewLockToken("invalid-token")
	badFencing := model.NewFencingCheck(lockId, badToken)
	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, &badFencing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error writing relationships to SpiceDB")

	// try to create with a non-existent lock id
	badLockId, _ := model.NewLockId("invalid-lock-id")
	badFencing2 := model.NewFencingCheck(badLockId, fencingToken)
	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, &badFencing2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error writing relationships to SpiceDB")
}

func TestSpiceDbRepository_DeleteRelationships_WithFencing(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	// Acquire a lock to get a fencing token
	lockId, _ := model.NewLockId(uniqueID(t, "test-lock"))
	lockResp, err := spiceDbRepo.AcquireLock(ctx, lockId)
	assert.NoError(t, err)
	fencingToken := lockResp.LockToken()
	assert.NotEmpty(t, fencingToken)

	// Create a relationship to delete
	tuples := []model.RelationsTuple{
		createRelationship("rbac", "group", "fencing_group_del", "member", "rbac", "principal", "fenced_bob_del", ""),
	}
	_, err = spiceDbRepo.CreateTuples(ctx, tuples, false, nil)
	assert.NoError(t, err)

	// Delete with correct fencing
	filter := model.NewTupleFilter().
		WithObjectId(model.DeserializeLocalResourceId("fencing_group_del")).
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithSubjectId(model.DeserializeLocalResourceId("fenced_bob_del")).
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))

	fencing := model.NewFencingCheck(lockId, fencingToken)
	_, err = spiceDbRepo.DeleteTuples(ctx, filter, &fencing)
	assert.NoError(t, err)

	container.WaitForQuantizationInterval()

	// Relationship should not exist
	exists := CheckForRelationship(
		spiceDbRepo, "fenced_bob_del", "rbac", "principal", "", "member", "rbac", "group", "fencing_group_del", model.NewConsistencyUnspecified(),
	)
	assert.False(t, exists)

	// Try to delete with an invalid fencing token
	badToken, _ := model.NewLockToken("invalid-token")
	badFencing := model.NewFencingCheck(lockId, badToken)
	_, err = spiceDbRepo.DeleteTuples(ctx, filter, &badFencing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error invoking DeleteRelationships in SpiceDB")

	// try to delete with a non-existent lock id
	badLockId, _ := model.NewLockId("invalid-lock-id")
	badFencing2 := model.NewFencingCheck(badLockId, fencingToken)
	_, err = spiceDbRepo.DeleteTuples(ctx, filter, &badFencing2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error invoking DeleteRelationships in SpiceDB")
}

func TestSpiceDbRepository_AcquireLock_NewLock(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	lockId, _ := model.NewLockId(uniqueID(t, "test-lock"))

	// Acquire a new lock
	resp, err := spiceDbRepo.AcquireLock(ctx, lockId)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.LockToken())
}

func TestSpiceDbRepository_AcquireLock_ReplaceExistingLock(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	assert.NoError(t, err)
	defer spiceDbRepo.Close()

	lockId, _ := model.NewLockId(uniqueID(t, "test-lock"))

	// Acquire initial lock
	resp1, err := spiceDbRepo.AcquireLock(ctx, lockId)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp1.LockToken())

	// Acquire lock again, forcefully replacing the existing lock
	resp2, err := spiceDbRepo.AcquireLock(ctx, lockId)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp2.LockToken())
	assert.NotEqual(t, resp1.LockToken(), resp2.LockToken())
}

func TestSpiceDbRepository_AcquireLock_EmptyIdentifier(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	// Try to acquire lock with an empty identifier (will fail at NewLockId)
	_, err := model.NewLockId("")
	assert.Error(t, err)
}

func TestSpiceDbRepository_LookupResources(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	// Create a permission structure with multiple widgets:
	// - alice has view access to widgets in workspace1
	// - charlie has view access to widgets in workspace2
	// - widget1, widget2, widget3 are in workspace1
	// - widget4 is in workspace2
	tuples := []model.RelationsTuple{
		// Workspace grants
		createRelationship("rbac", "workspace", uniqueID(t, "workspace1"), "user_grant", "rbac", "role_binding", "binding1", ""),
		createRelationship("rbac", "workspace", uniqueID(t, "workspace2"), "user_grant", "rbac", "role_binding", "binding2", ""),

		// Role binding to role
		createRelationship("rbac", "role_binding", "binding1", "granted", "rbac", "role", "viewer", ""),
		createRelationship("rbac", "role_binding", "binding2", "granted", "rbac", "role", "viewer", ""),

		// Role binding to subjects
		createRelationship("rbac", "role_binding", "binding1", "subject", "rbac", "principal", "alice", ""),
		createRelationship("rbac", "role_binding", "binding2", "subject", "rbac", "principal", "charlie", ""),

		// Role permissions
		createRelationship("rbac", "role", "viewer", "view_widget", "rbac", "principal", "*", ""),

		// Widgets in workspaces
		createRelationship("rbac", "widget", uniqueID(t, "widget1"), "workspace", "rbac", "workspace", uniqueID(t, "workspace1"), ""),
		createRelationship("rbac", "widget", uniqueID(t, "widget2"), "workspace", "rbac", "workspace", uniqueID(t, "workspace1"), ""),
		createRelationship("rbac", "widget", uniqueID(t, "widget3"), "workspace", "rbac", "workspace", uniqueID(t, "workspace1"), ""),
		createRelationship("rbac", "widget", uniqueID(t, "widget4"), "workspace", "rbac", "workspace", uniqueID(t, "workspace2"), ""),
	}

	relationshipResp, err := spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	// Test 1: LookupObjects to find all widgets that alice can view
	// alice should see widget1, widget2, widget3 (from workspace1)
	objectType := model.NewRepresentationTypeRequired(
		model.DeserializeResourceType("widget"),
		model.DeserializeReporterType("rbac"))
	aliceSubject := createSubjectReference("rbac", "principal", "alice")

	stream, err := spiceDbRepo.LookupObjects(
		ctx,
		objectType,
		model.DeserializeRelation("view"), // relation/permission
		aliceSubject,
		nil, // no pagination
		model.NewConsistencyAtLeastAsFresh(relationshipResp.ConsistencyToken()),
	)
	if !assert.NoError(t, err) {
		return
	}

	// Collect all resources from the stream
	foundResources := collectObjectIds(t, stream)
	// alice should see widget1, widget2, widget3 from workspace1
	assert.True(t, foundResources[uniqueID(t, "widget1")], "alice should have view permission on widget1")
	assert.True(t, foundResources[uniqueID(t, "widget2")], "alice should have view permission on widget2")
	assert.True(t, foundResources[uniqueID(t, "widget3")], "alice should have view permission on widget3")
	assert.False(t, foundResources[uniqueID(t, "widget4")], "alice should not have view permission on widget4")
	assert.Equal(t, 3, len(foundResources), "alice should find exactly 3 widgets with view permission")

	// Test 2: LookupObjects to find all widgets that charlie can view
	// charlie should only see widget4 (from workspace2)
	charlieSubject := createSubjectReference("rbac", "principal", "charlie")

	stream2, err := spiceDbRepo.LookupObjects(
		ctx,
		objectType,
		model.DeserializeRelation("view"),
		charlieSubject,
		model.NewPagination(10, nil), // specify a limit to test that it gets passed through
		model.NewConsistencyAtLeastAsFresh(relationshipResp.ConsistencyToken()),
	)
	if !assert.NoError(t, err) {
		return
	}

	foundResources2 := collectObjectIds(t, stream2)
	// charlie should only see widget4 from workspace2
	assert.False(t, foundResources2[uniqueID(t, "widget1")], "charlie should not have view permission on widget1")
	assert.False(t, foundResources2[uniqueID(t, "widget2")], "charlie should not have view permission on widget2")
	assert.False(t, foundResources2[uniqueID(t, "widget3")], "charlie should not have view permission on widget3")
	assert.True(t, foundResources2[uniqueID(t, "widget4")], "charlie should have view permission on widget4")
	assert.Equal(t, 1, len(foundResources2), "charlie should find exactly 1 widget with view permission")
}

func TestSpiceDbRepository_LookupSubjects(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	ctx := context.Background()
	spiceDbRepo, err := container.CreateSpiceDbRepository()
	if !assert.NoError(t, err) {
		defer spiceDbRepo.Close()
		return
	}

	// Create a permission structure:
	// - alice and bob are members of the group "admins"
	// - charlie is a member of the group "viewers"
	// - workspace "test" has user_grant role_binding "admin_binding" and "viewer_binding"
	// - "admin_binding" grants role "admin" to group "admins" members
	// - "viewer_binding" grants role "viewer" to group "viewers" members
	// - role "admin" has view_widget and use_widget permissions
	// - role "viewer" has view_widget permission only
	tuples := []model.RelationsTuple{
		// Group memberships
		createRelationship("rbac", "group", "admins", "member", "rbac", "principal", "alice", ""),
		createRelationship("rbac", "group", "admins", "member", "rbac", "principal", "bob", ""),
		createRelationship("rbac", "group", "viewers", "member", "rbac", "principal", "charlie", ""),

		// Workspace grants
		createRelationship("rbac", "workspace", uniqueID(t, "test"), "user_grant", "rbac", "role_binding", "admin_binding", ""),
		createRelationship("rbac", "workspace", uniqueID(t, "test"), "user_grant", "rbac", "role_binding", "viewer_binding", ""),

		// Role binding to role
		createRelationship("rbac", "role_binding", "admin_binding", "granted", "rbac", "role", "admin", ""),
		createRelationship("rbac", "role_binding", "viewer_binding", "granted", "rbac", "role", "viewer", ""),

		// Role binding to subjects (groups)
		createRelationship("rbac", "role_binding", "admin_binding", "subject", "rbac", "group", "admins", "member"),
		createRelationship("rbac", "role_binding", "viewer_binding", "subject", "rbac", "group", "viewers", "member"),

		// Role permissions
		createRelationship("rbac", "role", "admin", "view_widget", "rbac", "principal", "*", ""),
		createRelationship("rbac", "role", "admin", "use_widget", "rbac", "principal", "*", ""),
		createRelationship("rbac", "role", "viewer", "view_widget", "rbac", "principal", "*", ""),
	}

	relationshipResp, err := spiceDbRepo.CreateTuples(ctx, tuples, true, nil)
	if !assert.NoError(t, err) {
		return
	}

	// LookupSubjects to find all principals that have view_widget permission on workspace:test
	subjectType := model.NewRepresentationTypeRequired(
		model.DeserializeResourceType("principal"),
		model.DeserializeReporterType("rbac"))
	resource := createResourceReference("rbac", "workspace", uniqueID(t, "test"))

	stream, err := spiceDbRepo.LookupSubjects(
		ctx,
		resource,
		model.DeserializeRelation("view_widget"), // relation/permission
		subjectType,
		nil, // subject_relation (nil for direct principals)
		nil, // no pagination
		model.NewConsistencyAtLeastAsFresh(relationshipResp.ConsistencyToken()),
	)
	if !assert.NoError(t, err) {
		return
	}

	// Collect all subjects from the stream
	foundSubjects := collectSubjectIds(t, stream)
	// Verify that alice, bob, and charlie all have view_widget permission
	assert.True(t, foundSubjects["alice"], "alice should have view_widget permission")
	assert.True(t, foundSubjects["bob"], "bob should have view_widget permission")
	assert.True(t, foundSubjects["charlie"], "charlie should have view_widget permission")
	assert.Equal(t, 3, len(foundSubjects), "should find exactly 3 subjects with view_widget permission")

	// Now test LookupSubjects for use_widget permission (only alice and bob via admin role)
	stream2, err := spiceDbRepo.LookupSubjects(
		ctx,
		resource,
		model.DeserializeRelation("use_widget"),
		subjectType,
		nil,
		nil,
		model.NewConsistencyAtLeastAsFresh(relationshipResp.ConsistencyToken()),
	)
	if !assert.NoError(t, err) {
		return
	}

	foundSubjects2 := collectSubjectIds(t, stream2)
	// Verify that only alice and bob have use_widget permission (not charlie)
	assert.True(t, foundSubjects2["alice"], "alice should have use_widget permission")
	assert.True(t, foundSubjects2["bob"], "bob should have use_widget permission")
	assert.False(t, foundSubjects2["charlie"], "charlie should not have use_widget permission")
	assert.Equal(t, 2, len(foundSubjects2), "should find exactly 2 subjects with use_widget permission")
}

// Helper functions

func createRelationship(resourceNamespace string, resourceType string, resourceId string, relationship string, subjectNamespace string, subjectType string, subjectId string, subjectRelationship string) model.RelationsTuple {
	reporterRef := model.NewReporterReference(model.DeserializeReporterType(subjectNamespace), nil)
	subjectResource := model.NewResourceReference(
		model.DeserializeResourceType(subjectType),
		model.DeserializeLocalResourceId(subjectId),
		&reporterRef,
	)

	var subject model.SubjectReference
	if subjectRelationship != "" {
		rel := model.DeserializeRelation(subjectRelationship)
		subject = model.NewSubjectReference(subjectResource, &rel)
	} else {
		subject = model.NewSubjectReferenceWithoutRelation(subjectResource)
	}

	resourceReporterRef := model.NewReporterReference(model.DeserializeReporterType(resourceNamespace), nil)
	resource := model.NewResourceReference(
		model.DeserializeResourceType(resourceType),
		model.DeserializeLocalResourceId(resourceId),
		&resourceReporterRef,
	)

	return model.NewRelationsTuple(resource, model.DeserializeRelation(relationship), subject)
}

func runSpiceDBCheck(t *testing.T, ctx context.Context, spiceDbRepo *SpiceDBRelationsRepository, subjectType,
	subjectNamespace, subjectID, relation, resourceType, resourceNamespace, resourceID string,
	expectedAllowed bool) {
	subject := createSubjectReference(subjectNamespace, subjectType, subjectID)
	resource := createResourceReference(resourceNamespace, resourceType, resourceID)

	rel := model.NewRelationship(resource, model.DeserializeRelation(relation), subject)
	resp, err := spiceDbRepo.Check(ctx, rel, model.NewConsistencyUnspecified())
	assert.NoError(t, err)

	assert.Equal(t, expectedAllowed, resp.Allowed())
}

func createResourceReference(namespace, resourceType, id string) model.ResourceReference {
	reporterRef := model.NewReporterReference(model.DeserializeReporterType(namespace), nil)
	return model.NewResourceReference(
		model.DeserializeResourceType(resourceType),
		model.DeserializeLocalResourceId(id),
		&reporterRef,
	)
}

func createSubjectReference(namespace, subjectType, id string) model.SubjectReference {
	reporterRef := model.NewReporterReference(model.DeserializeReporterType(namespace), nil)
	resource := model.NewResourceReference(
		model.DeserializeResourceType(subjectType),
		model.DeserializeLocalResourceId(id),
		&reporterRef,
	)
	return model.NewSubjectReferenceWithoutRelation(resource)
}

func readTuplesStreamToSlice(stream model.ResultStream[model.ReadTuplesItem]) []model.ReadTuplesItem {
	s := make([]model.ReadTuplesItem, 0)
	for {
		item, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			panic(err)
		}
		s = append(s, item)
	}
	return s
}

func collectSubjectIds(t *testing.T, stream model.ResultStream[model.LookupSubjectsItem]) map[string]bool {
	foundSubjects := make(map[string]bool)
	for {
		item, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("Error receiving subjects: %v", err)
		}
		foundSubjects[item.Subject().Resource().ResourceId().String()] = true
	}
	return foundSubjects
}

func collectObjectIds(t *testing.T, stream model.ResultStream[model.LookupObjectsItem]) map[string]bool {
	foundResources := make(map[string]bool)
	for {
		item, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("Error receiving resources: %v", err)
		}
		// Verify ResourceType is correct (not corrupted to be the same as ResourceId)
		assert.Equal(t, "widget", item.Object().ResourceType().String(),
			"ResourceType should be 'widget', not the resource ID")
		// Verify Reporter is set correctly
		assert.NotNil(t, item.Object().Reporter(),
			"Reporter should not be nil for widget resources")
		if item.Object().Reporter() != nil {
			assert.Equal(t, "rbac", item.Object().Reporter().ReporterType().String(),
				"ReporterType should be 'rbac'")
		}
		foundResources[item.Object().ResourceId().String()] = true
	}
	return foundResources
}

func TestFromSpicePair_WithError(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	// Build a SpiceDB pair that contains an error instead of an item
	pair := &v1.CheckBulkPermissionsPair{
		Request: &v1.CheckBulkPermissionsRequestItem{
			Resource: &v1.ObjectReference{
				ObjectType: "rbac/workspace",
				ObjectId:   "test",
			},
			Permission: "view_widget",
			Subject: &v1.SubjectReference{
				Object: &v1.ObjectReference{
					ObjectType: "rbac/principal",
					ObjectId:   "bob",
				},
			},
		},
		Response: &v1.CheckBulkPermissionsPair_Error{
			Error: &rpcstatus.Status{
				Code:    int32(codes.InvalidArgument),
				Message: "invalid request",
			},
		},
	}

	// Build the original request that would have been sent
	bobPrincipal := model.NewResourceReference(
		model.DeserializeResourceType("principal"),
		model.DeserializeLocalResourceId("bob"),
		func() *model.ReporterReference {
			r := model.NewReporterReference(model.DeserializeReporterType("rbac"), nil)
			return &r
		}(),
	)
	testWorkspace := model.NewResourceReference(
		model.DeserializeResourceType("workspace"),
		model.DeserializeLocalResourceId("test"),
		func() *model.ReporterReference {
			r := model.NewReporterReference(model.DeserializeReporterType("rbac"), nil)
			return &r
		}(),
	)
	originalRequest := model.NewRelationship(
		testWorkspace,
		model.DeserializeRelation("view_widget"),
		model.NewSubjectReferenceWithoutRelation(bobPrincipal),
	)

	got := spicePairToCheckBulkResultPair(pair, originalRequest)

	// When error is present, item should have error set
	item := got.Result()
	assert.False(t, item.Allowed())
	assert.NotNil(t, item.Err())
	assert.Equal(t, int32(codes.InvalidArgument), item.ErrorCode())

	// And the request should be preserved
	req := got.Request()
	assert.Equal(t, "rbac", req.Object().Reporter().ReporterType().String())
	assert.Equal(t, "workspace", req.Object().ResourceType().String())
	assert.Equal(t, "test", req.Object().ResourceId().String())
	assert.Equal(t, "view_widget", req.Relation().String())
	assert.Equal(t, "rbac", req.Subject().Resource().Reporter().ReporterType().String())
	assert.Equal(t, "principal", req.Subject().Resource().ResourceType().String())
	assert.Equal(t, "bob", req.Subject().Resource().ResourceId().String())
}

func TestTupleFilterToSpiceDBFilter_SubjectRelationNil(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	// Test case 1: subjectFilter.Relation is nil
	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithSubject(model.NewTupleSubjectFilter().
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")))
	// Note: no WithRelation call on subject filter, so Relation() returns nil

	result, err := tupleFilterToSpiceDBFilter(filter)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.OptionalSubjectFilter)
	// When Relation is nil, OptionalRelation should be nil
	assert.Nil(t, result.OptionalSubjectFilter.OptionalRelation)
}

func TestTupleFilterToSpiceDBFilter_SubjectRelationEmptyString(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	// Test case 2: subjectFilter.Relation is an empty string (via Deserialize)
	emptyRelation := model.DeserializeRelation("")
	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithSubject(model.NewTupleSubjectFilter().
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")).
			WithRelation(emptyRelation)) // empty string relation

	result, err := tupleFilterToSpiceDBFilter(filter)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.OptionalSubjectFilter)
	// When Relation is empty string, OptionalRelation should be set with RelationFilter containing empty string
	assert.NotNil(t, result.OptionalSubjectFilter.OptionalRelation)
	assert.Equal(t, "", result.OptionalSubjectFilter.OptionalRelation.Relation)
}

func TestTupleFilterToSpiceDBFilter_SubjectRelationWithValue(t *testing.T) {
	requireSpiceDBIntegration(t)

	t.Parallel()

	// Test case 3: subjectFilter.Relation has an actual value
	memberRelation := model.DeserializeRelation("member")
	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("group")).
		WithSubject(model.NewTupleSubjectFilter().
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")).
			WithRelation(memberRelation)) // actual value

	result, err := tupleFilterToSpiceDBFilter(filter)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.OptionalSubjectFilter)
	// When Relation has a value, OptionalRelation should be set with RelationFilter containing that value
	assert.NotNil(t, result.OptionalSubjectFilter.OptionalRelation)
	assert.Equal(t, "member", result.OptionalSubjectFilter.OptionalRelation.Relation)
}
