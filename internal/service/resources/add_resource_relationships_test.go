package resources_test

import (
	"context"
	"io"
	"testing"

	krlog "github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	addctl "github.com/project-kessel/inventory-api/internal/biz/usecase/relationships"
	usecase "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// newAddRelationshipsTestServer builds a TestServerConfig whose resources and
// add-relationships use cases share the same fake repository and schema
// repository, so resources reported through ReportResource are visible to
// AddResourceRelationships. The host common schema requires workspace_id so
// that auto-create validation behavior is observable through the endpoint.
func newAddRelationshipsTestServer(t *testing.T) (TestServerConfig, model.ResourceRepository) {
	t.Helper()

	repo := data.NewFakeResourceRepository()
	schemaRepo := data.NewInMemorySchemaRepository()

	hostType, err := model.NewResourceType("host")
	require.NoError(t, err)
	hbiType, err := model.NewReporterType("hbi")
	require.NoError(t, err)

	hostCommonSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"workspace_id": { "type": "string" }
		},
		"required": ["workspace_id"]
	}`)
	hostSchema, err := model.NewResourceSchemaRepresentation(hostType, hostCommonSchema)
	require.NoError(t, err)
	require.NoError(t, schemaRepo.CreateResourceSchema(context.Background(), hostSchema))

	hbiReporterSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {},
		"required": []
	}`)
	hostHbi, err := model.NewReporterSchemaRepresentation(hostType, hbiType, hbiReporterSchema)
	require.NoError(t, err)
	require.NoError(t, schemaRepo.CreateReporterSchema(context.Background(), hostHbi))

	logger := krlog.NewStdLogger(io.Discard)
	mc := metricscollector.NewFakeMetricsCollector()
	cfg := usecase.NewUsecaseConfig()

	resourcesUsecase := usecase.New(
		repo,
		schemaRepo,
		data.NewSimpleRelationsRepository(),
		"rbac",
		logger,
		nil, // listen manager
		nil, // circuit breaker
		cfg,
		mc,
		&PermissiveMetaAuthorizer{},
		nil,
	)

	addUsecase := addctl.NewAddResourceRelationshipsUseCase(
		repo,
		schemaRepo,
		nil, // listen manager
		nil, // circuit breaker
		cfg,
		mc,
		&PermissiveMetaAuthorizer{},
		logger,
	)

	return TestServerConfig{
		Usecase:                 resourcesUsecase,
		AddRelationshipsUsecase: addUsecase,
		Authenticator: &StubAuthenticator{
			Claims:   &authnapi.Claims{SubjectId: authnapi.SubjectId("tester"), AuthType: authnapi.AuthTypeXRhIdentity},
			Decision: authnapi.Allow,
		},
	}, repo
}

// mustStruct converts the given map to a protobuf Struct, failing the test on
// invalid input.
func mustStruct(t *testing.T, m map[string]interface{}) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	require.NoError(t, err)
	return s
}

// hostKey builds the reporter resource key used by the host/hbi test data.
func hostKey(t *testing.T, localResourceId string) model.ReporterResourceKey {
	t.Helper()
	localId, err := model.NewLocalResourceId(localResourceId)
	require.NoError(t, err)
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("instance-1")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(localId, resourceType, reporterType, reporterInstanceId)
	require.NoError(t, err)
	return key
}

// strPtr returns a pointer to the given string.
func strPtr(s string) *string {
	return &s
}

// TestInventoryService_AddResourceRelationships_MergesIntoExisting validates
// that named relationship values are merged additively into an existing
// resource over both gRPC and HTTP.
func TestInventoryService_AddResourceRelationships_MergesIntoExisting(t *testing.T) {
	cfg, repo := newAddRelationshipsTestServer(t)

	// Seed a host with a reporter relationship and a common workspace_id.
	seed := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "host-1",
				ApiHref:         "https://api.example.com/host-1",
			},
			Reporter: mustStruct(t, map[string]interface{}{
				"service_preferences": []interface{}{"service-a"},
			}),
			Common: mustStruct(t, map[string]interface{}{
				"workspace_id": "ws-1",
			}),
		},
	}

	req := &pb.AddResourceRelationshipsRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-1",
		Reporter: mustStruct(t, map[string]interface{}{
			"service_preferences": []interface{}{"service-b"},
		}),
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			res := tr.Invoke(ctx, withBody(seed, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
			Extract(t, res, expectSuccess(func() *pb.ReportResourceResponse { return &pb.ReportResourceResponse{} }))

			res = tr.Invoke(ctx, withBody(req, AddResourceRelationships, httpEndpoint("POST /api/kessel/v1beta2/resources/relationships")))
			resp := Extract(t, res, expectSuccess(func() *pb.AddResourceRelationshipsResponse { return &pb.AddResourceRelationshipsResponse{} }))
			assert.True(t, proto.Equal(&pb.AddResourceRelationshipsResponse{}, resp))

			// The merged reporter representation contains both values and the
			// common representation is untouched.
			reps, err := repo.FindLatestRepresentations(nil, hostKey(t, "host-1"))
			require.NoError(t, err)
			assert.ElementsMatch(t,
				[]interface{}{"service-a", "service-b"},
				reps.ReporterData()["service_preferences"].([]interface{}))
			assert.Equal(t, "ws-1", reps.CommonData()["workspace_id"])
		}
	})
}

// TestInventoryService_AddResourceRelationships_AutoCreatesResource validates
// that a request naming a resource that does not yet exist creates it with the
// provided relationship fields as its initial representation.
func TestInventoryService_AddResourceRelationships_AutoCreatesResource(t *testing.T) {
	cfg, repo := newAddRelationshipsTestServer(t)

	req := &pb.AddResourceRelationshipsRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-new",
		Reporter: mustStruct(t, map[string]interface{}{
			"service_preferences": []interface{}{"service-a"},
		}),
		Common: mustStruct(t, map[string]interface{}{
			"workspace_id": "ws-1",
		}),
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			res := tr.Invoke(ctx, withBody(req, AddResourceRelationships, httpEndpoint("POST /api/kessel/v1beta2/resources/relationships")))
			resp := Extract(t, res, expectSuccess(func() *pb.AddResourceRelationshipsResponse { return &pb.AddResourceRelationshipsResponse{} }))
			assert.True(t, proto.Equal(&pb.AddResourceRelationshipsResponse{}, resp))

			// The resource was created with the provided relationships.
			found, err := repo.FindResourceByKeys(nil, hostKey(t, "host-new"))
			require.NoError(t, err)
			require.NotNil(t, found)

			reps, err := repo.FindLatestRepresentations(nil, hostKey(t, "host-new"))
			require.NoError(t, err)
			assert.Equal(t, []interface{}{"service-a"}, reps.ReporterData()["service_preferences"].([]interface{}))
			assert.Equal(t, "ws-1", reps.CommonData()["workspace_id"])
		}
	})
}

// TestInventoryService_AddResourceRelationships_AutoCreateMissingRequiredCommon
// validates that auto-creation is rejected with INVALID_ARGUMENT when the
// schema requires common fields the request does not provide.
func TestInventoryService_AddResourceRelationships_AutoCreateMissingRequiredCommon(t *testing.T) {
	cfg, _ := newAddRelationshipsTestServer(t)

	// host requires workspace_id in the common representation; the add only
	// provides reporter fields, so the auto-create must be rejected.
	req := &pb.AddResourceRelationshipsRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-bad",
		Reporter: mustStruct(t, map[string]interface{}{
			"service_preferences": []interface{}{"service-a"},
		}),
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			res := tr.Invoke(ctx, withBody(req, AddResourceRelationships, httpEndpoint("POST /api/kessel/v1beta2/resources/relationships")))
			Assert(t, res, requireError(codes.InvalidArgument))
		}
	})
}

// TestInventoryService_AddResourceRelationships_BothRepresentationsAbsent
// validates that a request carrying neither a reporter nor a common
// representation is rejected with INVALID_ARGUMENT.
func TestInventoryService_AddResourceRelationships_BothRepresentationsAbsent(t *testing.T) {
	cfg, _ := newAddRelationshipsTestServer(t)

	req := &pb.AddResourceRelationshipsRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-1",
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			// Rejected by the protovalidation CEL rule before reaching the handler.
			res := tr.Invoke(ctx, withBody(req, AddResourceRelationships, httpEndpoint("POST /api/kessel/v1beta2/resources/relationships")))
			Assert(t, res, requireError(codes.InvalidArgument))
		}
	})
}

// TestInventoryService_AddResourceRelationships_TombstonedResource validates
// that adding relationships to a tombstoned resource returns NOT_FOUND over
// both transports.
func TestInventoryService_AddResourceRelationships_TombstonedResource(t *testing.T) {
	cfg, _ := newAddRelationshipsTestServer(t)

	seed := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "host-1",
				ApiHref:         "https://api.example.com/host-1",
			},
			Common: mustStruct(t, map[string]interface{}{
				"workspace_id": "ws-1",
			}),
		},
	}

	deleteReq := &pb.DeleteResourceRequest{
		Reference: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-1",
			Reporter: &pb.ReporterReference{
				Type:       "hbi",
				InstanceId: strPtr("instance-1"),
			},
		},
	}

	req := &pb.AddResourceRelationshipsRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-1",
		Reporter: mustStruct(t, map[string]interface{}{
			"service_preferences": []interface{}{"service-a"},
		}),
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			res := tr.Invoke(ctx, withBody(seed, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
			Extract(t, res, expectSuccess(func() *pb.ReportResourceResponse { return &pb.ReportResourceResponse{} }))

			res = tr.Invoke(ctx, withBody(deleteReq, DeleteResource, httpEndpoint("DELETE /api/kessel/v1beta2/resources")))
			Extract(t, res, expectSuccess(func() *pb.DeleteResourceResponse { return &pb.DeleteResourceResponse{} }))

			res = tr.Invoke(ctx, withBody(req, AddResourceRelationships, httpEndpoint("POST /api/kessel/v1beta2/resources/relationships")))
			Assert(t, res, requireError(codes.NotFound))
		}
	})
}

// TestInventoryService_AddResourceRelationships_MissingReporterType validates
// that a request without a reporter type is rejected with INVALID_ARGUMENT.
func TestInventoryService_AddResourceRelationships_MissingReporterType(t *testing.T) {
	cfg, _ := newAddRelationshipsTestServer(t)

	req := &pb.AddResourceRelationshipsRequest{
		Type:               "host",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-1",
		Reporter: mustStruct(t, map[string]interface{}{
			"service_preferences": []interface{}{"service-a"},
		}),
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			res := tr.Invoke(ctx, withBody(req, AddResourceRelationships, httpEndpoint("POST /api/kessel/v1beta2/resources/relationships")))
			Assert(t, res, requireError(codes.InvalidArgument))
		}
	})
}
