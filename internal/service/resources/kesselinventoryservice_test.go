package resources_test

import (
	"context"
	"regexp"

	"io"
	"testing"

	krlog "github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	relationsV1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"

	"github.com/stretchr/testify/assert"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	usecase "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	svc "github.com/project-kessel/inventory-api/internal/service/resources"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRequestToResource_Success(t *testing.T) {
	invID := "9b8b5a02-4ac7-4d6c-a2c0-123456789abc"
	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "inst-1",
		InventoryId:        &invID,
		Representations: &pb.ResourceRepresentations{
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-123"),
				},
			},
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "v1",
			},
		},
	}
	identity := &authnapi.Identity{Principal: "reporter-1"}

	resource, err := svc.RequestToResource(req, identity)

	assert.NoError(t, err)
	assert.NotNil(t, resource)
	assert.Equal(t, "host", req.GetType())
	assert.Equal(t, "hbi", req.GetReporterType())
	assert.Equal(t, "inst-1", req.GetReporterInstanceId())
	assert.Equal(t, "9b8b5a02-4ac7-4d6c-a2c0-123456789abc", req.GetInventoryId())
}

func TestRequestToResource_Error_InvalidInventoryID(t *testing.T) {
	id := "bad-uuid"
	req := &pb.ReportResourceRequest{
		InventoryId: &id,
		Representations: &pb.ResourceRepresentations{
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws"),
				},
			},
		},
	}
	identity := &authnapi.Identity{Principal: "reporter-1"}

	resource, err := svc.RequestToResource(req, identity)

	assert.Error(t, err)
	assert.Nil(t, resource)
	assert.Contains(t, err.Error(), "invalid inventory ID")
}

func TestRequestToResource_Error_MissingReporterType(t *testing.T) {
	req := &pb.ReportResourceRequest{
		ReporterType:       "",
		ReporterInstanceId: "inst",
		Representations: &pb.ResourceRepresentations{
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws"),
				},
			},
		},
	}
	identity := &authnapi.Identity{Principal: "reporter-1"}

	resource, err := svc.RequestToResource(req, identity)

	assert.Error(t, err)
	assert.Nil(t, resource)
	assert.Contains(t, err.Error(), "cannot be empty: ReporterType")
}

func TestRequestToResource_Error_MissingReporterInstanceId(t *testing.T) {
	req := &pb.ReportResourceRequest{
		ReporterType:       "hbi",
		ReporterInstanceId: "",
		Representations: &pb.ResourceRepresentations{
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws"),
				},
			},
		},
	}
	identity := &authnapi.Identity{Principal: "reporter-1"}

	resource, err := svc.RequestToResource(req, identity)

	assert.Error(t, err)
	assert.Nil(t, resource)
	assert.Contains(t, err.Error(), "cannot be empty: ReporterInstanceId")
}

func TestInventoryService_ReportResource_MissingReporterType(t *testing.T) {
	id := &authnapi.Identity{Principal: "tester", Type: "reporterType"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "v1",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue("example-host"),
				},
			},
			Reporter: &structpb.Struct{},
		},
	}

	uc := usecase.New(
		data.NewFakeResourceRepository(),
		nil,                                // LegacyReporterResourceRepository
		nil,                                // inventoryResourceRepository
		data.NewInMemorySchemaRepository(), // schema repository
		nil,                                // Authz
		nil,                                // Eventer
		"",                                 // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		nil, // Config
		metricscollector.NewFakeMetricsCollector(),
	)
	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.ReportResource(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	// Check that it returns the correct gRPC status code
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, grpcStatus.Code())
}

func TestInventoryService_ReportResource_MissingReporterInstanceId(t *testing.T) {
	id := &authnapi.Identity{Principal: "tester", Type: "reporterType"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "v1",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue("example-host"),
				},
			},
			Reporter: &structpb.Struct{},
		},
	}

	uc := usecase.New(
		data.NewFakeResourceRepository(),
		nil,                                // LegacyReporterResourceRepository
		nil,                                // inventoryResourceRepository
		data.NewInMemorySchemaRepository(), // schema repository
		nil,                                // Authz
		nil,                                // Eventer
		"",                                 // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		nil, // Config
		metricscollector.NewFakeMetricsCollector(),
	)
	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.ReportResource(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "cannot be empty: ReporterInstanceId")

	// Check that it returns the correct gRPC status code
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, grpcStatus.Code())
}

func TestInventoryService_ReportResource_InvalidJsonObject(t *testing.T) {
	id := &authnapi.Identity{Principal: "sarah", Type: "rbac"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	badCommon := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"invalid": nil, // use intentionally unsupported types
		},
	}

	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Common: badCommon,
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "v1",
			},
			Reporter: &structpb.Struct{},
		},
	}

	uc := usecase.New(
		data.NewFakeResourceRepository(),
		nil,                                // LegacyReporterResourceRepository
		nil,                                // inventoryResourceRepository
		data.NewInMemorySchemaRepository(), // schema repository
		nil,                                // Authz
		nil,                                // Eventer
		"",                                 // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		nil, // Config
		metricscollector.NewFakeMetricsCollector(),
	)

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.ReportResource(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestResponseFromResource(t *testing.T) {
	resp := svc.ResponseFromResource()

	assert.NotNil(t, resp)
	assert.IsType(t, &pb.ReportResourceResponse{}, resp)
	assert.Equal(t, &pb.ReportResourceResponse{}, resp)
}

func TestInventoryService_DeleteResource_NoIdentity(t *testing.T) {
	ctx := context.Background() // no identity

	req := &pb.DeleteResourceRequest{}

	uc := &usecase.Usecase{}
	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.DeleteResource(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to get identity")

	// Check that it returns the correct gRPC status code
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, grpcStatus.Code())
}

func TestToLookupResourceRequest(t *testing.T) {
	permission := "view"
	input := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "hbi",
		},
		Relation: "view",
		Subject: &pb.SubjectReference{
			Relation: &permission,
			Resource: &pb.ResourceReference{
				ResourceId:   "res-id",
				ResourceType: "principal",
				Reporter: &pb.ReporterReference{
					Type: "rbac",
				},
			},
		},
		Pagination: &pb.RequestPagination{
			Limit: 50,
		},
	}

	expected := &relationsV1beta1.LookupResourcesRequest{
		ResourceType: &relationsV1beta1.ObjectType{
			Name: "hbi",
		},
		Relation: "view",
		Subject: &relationsV1beta1.SubjectReference{
			Relation: &permission,
			Subject: &relationsV1beta1.ObjectReference{
				Type: &relationsV1beta1.ObjectType{
					Name:      "principal",
					Namespace: "rbac",
				},
				Id: "res-id",
			},
		},
		Pagination: &relationsV1beta1.RequestPagination{
			Limit: 50,
		},
	}

	result, _ := svc.ToLookupResourceRequest(input)
	assert.Equal(t, expected, result)
}

func TestIsValidatedRepresentationType(t *testing.T) {

	assert.True(t, IsValidType("hbi"))

	// normalize then validate
	normalized := svc.NormalizeType("HBI")
	assert.True(t, IsValidType(normalized))
	// strange characters
	assert.False(t, IsValidType("h?!!!"))
}

func TestNormalizeRepresentationType(t *testing.T) {
	// normalize then validate
	normalized := svc.NormalizeType("HBI")
	assert.True(t, IsValidType(normalized))

	assert.Equal(t, "hbi", normalized)
}

var typePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func IsValidType(val string) bool {
	return typePattern.MatchString(val)
}

func TestToLookupResourceResponse(t *testing.T) {
	input := &relationsV1beta1.LookupResourcesResponse{
		Resource: &relationsV1beta1.ObjectReference{
			Type: &relationsV1beta1.ObjectType{
				Namespace: "reporter-x",
				Name:      "type-y",
			},
			Id: "abc123",
		},
		Pagination: &relationsV1beta1.ResponsePagination{
			ContinuationToken: "next-page-token",
		},
	}

	expected := &pb.StreamedListObjectsResponse{
		Object: &pb.ResourceReference{
			Reporter: &pb.ReporterReference{
				Type: "reporter-x",
			},
			ResourceId:   "abc123",
			ResourceType: "type-y",
		},
		Pagination: &pb.ResponsePagination{
			ContinuationToken: "next-page-token",
		},
	}

	result := svc.ToLookupResourceResponse(input)
	assert.Equal(t, expected, result)
}
