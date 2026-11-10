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

// newRemoveHandlerTestServer builds a TestServerConfig whose resources and
// remove-relationships use cases share the same fake repository and schema
// repository, so resources reported through ReportResource are visible to
// RemoveResourceRelationships. The host common schema requires workspace_id so
// that removal validation behavior is observable through the endpoint.
func newRemoveHandlerTestServer(t *testing.T) (TestServerConfig, model.ResourceRepository) {
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

	removeUsecase := addctl.NewRemoveResourceRelationshipsUseCase(
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
		Usecase:                    resourcesUsecase,
		RemoveRelationshipsUsecase: removeUsecase,
		Authenticator: &StubAuthenticator{
			Claims:   &authnapi.Claims{SubjectId: authnapi.SubjectId("tester"), AuthType: authnapi.AuthTypeXRhIdentity},
			Decision: authnapi.Allow,
		},
	}, repo
}

// removeHandlerHostKey builds the reporter resource key used by the host/hbi
// remove-handler test data.
func removeHandlerHostKey(t *testing.T, localResourceId string) model.ReporterResourceKey {
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

// removeHandlerStrPtr returns a pointer to the given string.
func removeHandlerStrPtr(s string) *string {
	return &s
}

// TestInventoryService_RemoveResourceRelationships_RemovesFromExisting
// validates that only the named relationship values are removed from an
// existing resource over both gRPC and HTTP.
func TestInventoryService_RemoveResourceRelationships_RemovesFromExisting(t *testing.T) {
	cfg, repo := newRemoveHandlerTestServer(t)

	// Seed a host with two reporter relationships and a common workspace_id.
	seed := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "host-1",
				ApiHref:         "https://api.example.com/host-1",
			},
			Reporter: removeHandlerMustStruct(t, map[string]interface{}{
				"service_preferences": []interface{}{"service-a", "service-b"},
			}),
			Common: removeHandlerMustStruct(t, map[string]interface{}{
				"workspace_id": "ws-1",
			}),
		},
	}

	req := &pb.RemoveResourceRelationshipsRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-1",
		Reporter: removeHandlerMustStruct(t, map[string]interface{}{
			"service_preferences": []interface{}{"service-b"},
		}),
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			res := tr.Invoke(ctx, withBody(seed, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
			Extract(t, res, expectSuccess(func() *pb.ReportResourceResponse { return &pb.ReportResourceResponse{} }))

			res = tr.Invoke(ctx, withBody(req, RemoveResourceRelationships, httpEndpoint("DELETE /api/kessel/v1beta2/resources/relationships")))
			resp := Extract(t, res, expectSuccess(func() *pb.RemoveResourceRelationshipsResponse { return &pb.RemoveResourceRelationshipsResponse{} }))
			assert.True(t, proto.Equal(&pb.RemoveResourceRelationshipsResponse{}, resp))

			// Only the removed value is gone; the common representation is untouched.
			reps, err := repo.FindLatestRepresentations(nil, removeHandlerHostKey(t, "host-1"))
			require.NoError(t, err)
			assert.ElementsMatch(t,
				[]interface{}{"service-a"},
				reps.ReporterData()["service_preferences"].([]interface{}))
			assert.Equal(t, "ws-1", reps.CommonData()["workspace_id"])
		}
	})
}

// TestInventoryService_RemoveResourceRelationships_NonExistentValueIsIdempotent
// validates that removing values that are not present is an idempotent no-op:
// the request succeeds without altering the stored representation.
func TestInventoryService_RemoveResourceRelationships_NonExistentValueIsIdempotent(t *testing.T) {
	cfg, repo := newRemoveHandlerTestServer(t)

	seed := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "host-1",
				ApiHref:         "https://api.example.com/host-1",
			},
			Reporter: removeHandlerMustStruct(t, map[string]interface{}{
				"service_preferences": []interface{}{"service-a"},
			}),
			Common: removeHandlerMustStruct(t, map[string]interface{}{
				"workspace_id": "ws-1",
			}),
		},
	}

	req := &pb.RemoveResourceRelationshipsRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-1",
		Reporter: removeHandlerMustStruct(t, map[string]interface{}{
			"service_preferences": []interface{}{"service-z"},
		}),
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			res := tr.Invoke(ctx, withBody(seed, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
			Extract(t, res, expectSuccess(func() *pb.ReportResourceResponse { return &pb.ReportResourceResponse{} }))

			res = tr.Invoke(ctx, withBody(req, RemoveResourceRelationships, httpEndpoint("DELETE /api/kessel/v1beta2/resources/relationships")))
			Extract(t, res, expectSuccess(func() *pb.RemoveResourceRelationshipsResponse { return &pb.RemoveResourceRelationshipsResponse{} }))

			reps, err := repo.FindLatestRepresentations(nil, removeHandlerHostKey(t, "host-1"))
			require.NoError(t, err)
			assert.ElementsMatch(t,
				[]interface{}{"service-a"},
				reps.ReporterData()["service_preferences"].([]interface{}))
		}
	})
}

// TestInventoryService_RemoveResourceRelationships_ResourceNotFound validates
// that removing relationships from a resource that does not exist returns
// NOT_FOUND over both transports.
func TestInventoryService_RemoveResourceRelationships_ResourceNotFound(t *testing.T) {
	cfg, _ := newRemoveHandlerTestServer(t)

	req := &pb.RemoveResourceRelationshipsRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-missing",
		Reporter: removeHandlerMustStruct(t, map[string]interface{}{
			"service_preferences": []interface{}{"service-a"},
		}),
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			res := tr.Invoke(ctx, withBody(req, RemoveResourceRelationships, httpEndpoint("DELETE /api/kessel/v1beta2/resources/relationships")))
			Assert(t, res, requireError(codes.NotFound))
		}
	})
}

// TestInventoryService_RemoveResourceRelationships_TombstonedResource
// validates that removing relationships from a tombstoned resource returns
// NOT_FOUND over both transports.
func TestInventoryService_RemoveResourceRelationships_TombstonedResource(t *testing.T) {
	cfg, _ := newRemoveHandlerTestServer(t)

	seed := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "host-1",
				ApiHref:         "https://api.example.com/host-1",
			},
			Common: removeHandlerMustStruct(t, map[string]interface{}{
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
				InstanceId: removeHandlerStrPtr("instance-1"),
			},
		},
	}

	req := &pb.RemoveResourceRelationshipsRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-1",
		Reporter: removeHandlerMustStruct(t, map[string]interface{}{
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

			res = tr.Invoke(ctx, withBody(req, RemoveResourceRelationships, httpEndpoint("DELETE /api/kessel/v1beta2/resources/relationships")))
			Assert(t, res, requireError(codes.NotFound))
		}
	})
}

// TestInventoryService_RemoveResourceRelationships_BothRepresentationsAbsent
// validates that a request carrying neither a reporter nor a common
// representation is rejected with INVALID_ARGUMENT.
func TestInventoryService_RemoveResourceRelationships_BothRepresentationsAbsent(t *testing.T) {
	cfg, _ := newRemoveHandlerTestServer(t)

	req := &pb.RemoveResourceRelationshipsRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-1",
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			// Rejected by the protovalidation CEL rule before reaching the handler.
			res := tr.Invoke(ctx, withBody(req, RemoveResourceRelationships, httpEndpoint("DELETE /api/kessel/v1beta2/resources/relationships")))
			Assert(t, res, requireError(codes.InvalidArgument))
		}
	})
}

// TestInventoryService_RemoveResourceRelationships_MissingReporterType
// validates that a request without a reporter type is rejected with
// INVALID_ARGUMENT.
func TestInventoryService_RemoveResourceRelationships_MissingReporterType(t *testing.T) {
	cfg, _ := newRemoveHandlerTestServer(t)

	req := &pb.RemoveResourceRelationshipsRequest{
		Type:               "host",
		ReporterInstanceId: "instance-1",
		LocalResourceId:    "host-1",
		Reporter: removeHandlerMustStruct(t, map[string]interface{}{
			"service_preferences": []interface{}{"service-a"},
		}),
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			res := tr.Invoke(ctx, withBody(req, RemoveResourceRelationships, httpEndpoint("DELETE /api/kessel/v1beta2/resources/relationships")))
			Assert(t, res, requireError(codes.InvalidArgument))
		}
	})
}

// removeHandlerMustStruct builds a structpb.Struct, failing the test on error.
func removeHandlerMustStruct(t *testing.T, m map[string]interface{}) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	require.NoError(t, err)
	return s
}
