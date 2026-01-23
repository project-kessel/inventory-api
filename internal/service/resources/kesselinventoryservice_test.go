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
	"github.com/stretchr/testify/mock"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	usecase "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/project-kessel/inventory-api/internal/mocks"
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
		nil,
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
		nil,
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
		nil,
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

func TestInventoryService_CheckSelf_Allowed_XRhIdentity(t *testing.T) {
	// Test CheckSelf with x-rh-identity header (using UserID)
	id := &authnapi.Identity{
		Principal: "testuser",
		UserID:    "user-123",
		Type:      "User",
		AuthType:  "x-rh-identity",
	}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	mockRepo := data.NewFakeResourceRepository()
	mockAuthz := &mocks.MockAuthz{}
	mockAuthz.
		On("Check",
			mock.Anything,
			"hbi",
			"view",
			mock.Anything,
			"host",
			"dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			mock.MatchedBy(func(sub *relationsV1beta1.SubjectReference) bool {
				// Verify subject is derived from identity (UserID for x-rh-identity)
				return sub.Subject.Id == "user-123" &&
					sub.Subject.Type.Name == "principal" &&
					sub.Subject.Type.Namespace == "rbac"
			}),
		).
		Return(relationsV1beta1.CheckResponse_ALLOWED_TRUE, &relationsV1beta1.ConsistencyToken{}, nil).
		Once()

	cfg := &usecase.UsecaseConfig{}
	uc := usecase.New(
		mockRepo,
		nil,                                // LegacyReporterResourceRepository
		nil,                                // inventoryResourceRepository
		data.NewInMemorySchemaRepository(), // schema repository
		mockAuthz,                          // Authz
		nil,                                // Eventer
		"rbac",                             // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
	)

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckSelf(ctx, req)

	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Allowed)
	}

	mockAuthz.AssertExpectations(t)
}

func TestInventoryService_CheckSelf_Allowed_XRhIdentity_NoUserID(t *testing.T) {
	// Test CheckSelf with x-rh-identity header (fallback to Principal when UserID not available)
	id := &authnapi.Identity{
		Principal: "testuser",
		UserID:    "", // No UserID, should use Principal
		Type:      "User",
		AuthType:  "x-rh-identity",
	}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	mockRepo := data.NewFakeResourceRepository()
	mockAuthz := &mocks.MockAuthz{}
	mockAuthz.
		On("Check",
			mock.Anything,
			"hbi",
			"view",
			mock.Anything,
			"host",
			"dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			mock.MatchedBy(func(sub *relationsV1beta1.SubjectReference) bool {
				// Verify subject uses Principal when UserID not available
				return sub.Subject.Id == "testuser" &&
					sub.Subject.Type.Name == "principal" &&
					sub.Subject.Type.Namespace == "rbac"
			}),
		).
		Return(relationsV1beta1.CheckResponse_ALLOWED_TRUE, &relationsV1beta1.ConsistencyToken{}, nil).
		Once()

	cfg := &usecase.UsecaseConfig{}
	uc := usecase.New(
		mockRepo,
		nil,                                // LegacyReporterResourceRepository
		nil,                                // inventoryResourceRepository
		data.NewInMemorySchemaRepository(), // schema repository
		mockAuthz,                          // Authz
		nil,                                // Eventer
		"rbac",                             // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
	)

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckSelf(ctx, req)

	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Allowed)
	}

	mockAuthz.AssertExpectations(t)
}

func TestInventoryService_CheckSelf_Denied(t *testing.T) {
	id := &authnapi.Identity{
		Principal: "testuser",
		UserID:    "user-123",
		Type:      "User",
		AuthType:  "x-rh-identity",
	}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	mockRepo := data.NewFakeResourceRepository()
	mockAuthz := &mocks.MockAuthz{}
	mockAuthz.
		On("Check",
			mock.Anything,
			"hbi",
			"view",
			mock.Anything,
			"host",
			"dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			mock.Anything,
		).
		Return(relationsV1beta1.CheckResponse_ALLOWED_FALSE, &relationsV1beta1.ConsistencyToken{}, nil).
		Once()

	cfg := &usecase.UsecaseConfig{}
	uc := usecase.New(
		mockRepo,
		nil,                                // LegacyReporterResourceRepository
		nil,                                // inventoryResourceRepository
		data.NewInMemorySchemaRepository(), // schema repository
		mockAuthz,                          // Authz
		nil,                                // Eventer
		"rbac",                             // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
	)

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckSelf(ctx, req)

	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Allowed)
	}

	mockAuthz.AssertExpectations(t)
}

func TestInventoryService_CheckSelf_NoIdentity(t *testing.T) {
	ctx := context.Background() // no identity

	req := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	uc := &usecase.Usecase{}
	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckSelf(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to get identity")

	// Check that it returns the correct gRPC status code
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, grpcStatus.Code())
}

func TestInventoryService_CheckSelfBulk_Allowed_XRhIdentity(t *testing.T) {
	// Test CheckSelfBulk with x-rh-identity header
	id := &authnapi.Identity{
		Principal: "testuser",
		UserID:    "user-123",
		Type:      "User",
		AuthType:  "x-rh-identity",
	}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.CheckSelfBulkRequest{
		Items: []*pb.CheckSelfBulkRequestItem{
			{
				Object: &pb.ResourceReference{
					ResourceId:   "resource-1",
					ResourceType: "host",
					Reporter:     &pb.ReporterReference{Type: "hbi"},
				},
				Relation: "view",
			},
			{
				Object: &pb.ResourceReference{
					ResourceId:   "resource-2",
					ResourceType: "host",
					Reporter:     &pb.ReporterReference{Type: "hbi"},
				},
				Relation: "edit",
			},
		},
	}

	mockAuthz := &mocks.MockAuthz{}
	mockAuthz.
		On("CheckBulk",
			mock.Anything,
			mock.MatchedBy(func(req *relationsV1beta1.CheckBulkRequest) bool {
				// Verify all items use the same subject (derived from identity)
				if len(req.Items) != 2 {
					return false
				}
				// Check that both items have the same subject (UserID from x-rh-identity)
				subject1 := req.Items[0].Subject
				subject2 := req.Items[1].Subject
				return subject1.Subject.Id == "user-123" &&
					subject1.Subject.Type.Name == "principal" &&
					subject1.Subject.Type.Namespace == "rbac" &&
					subject2.Subject.Id == "user-123" &&
					subject2.Subject.Type.Name == "principal" &&
					subject2.Subject.Type.Namespace == "rbac"
			}),
		).
		Return(&relationsV1beta1.CheckBulkResponse{
			Pairs: []*relationsV1beta1.CheckBulkResponsePair{
				{
					Request: &relationsV1beta1.CheckBulkRequestItem{
						Resource: &relationsV1beta1.ObjectReference{
							Type: &relationsV1beta1.ObjectType{
								Namespace: "hbi",
								Name:      "host",
							},
							Id: "resource-1",
						},
						Relation: "view",
					},
					Response: &relationsV1beta1.CheckBulkResponsePair_Item{
						Item: &relationsV1beta1.CheckBulkResponseItem{
							Allowed: relationsV1beta1.CheckBulkResponseItem_ALLOWED_TRUE,
						},
					},
				},
				{
					Request: &relationsV1beta1.CheckBulkRequestItem{
						Resource: &relationsV1beta1.ObjectReference{
							Type: &relationsV1beta1.ObjectType{
								Namespace: "hbi",
								Name:      "host",
							},
							Id: "resource-2",
						},
						Relation: "edit",
					},
					Response: &relationsV1beta1.CheckBulkResponsePair_Item{
						Item: &relationsV1beta1.CheckBulkResponseItem{
							Allowed: relationsV1beta1.CheckBulkResponseItem_ALLOWED_TRUE,
						},
					},
				},
			},
			ConsistencyToken: &relationsV1beta1.ConsistencyToken{Token: "test-token"},
		}, nil).
		Once()

	cfg := &usecase.UsecaseConfig{}
	uc := usecase.New(
		data.NewFakeResourceRepository(),
		nil,                                // LegacyReporterResourceRepository
		nil,                                // inventoryResourceRepository
		data.NewInMemorySchemaRepository(), // schema repository
		mockAuthz,                          // Authz
		nil,                                // Eventer
		"rbac",                             // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
	)

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckSelfBulk(ctx, req)

	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Len(t, resp.Pairs, 2)
		assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed)
		assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[1].GetItem().Allowed)
		// Verify consistency token is set
		assert.NotNil(t, resp.ConsistencyToken)
		assert.Equal(t, "test-token", resp.ConsistencyToken.GetToken())
	}

	mockAuthz.AssertExpectations(t)
}

func TestInventoryService_CheckSelfBulk_MixedResults(t *testing.T) {
	id := &authnapi.Identity{
		Principal: "testuser",
		UserID:    "user-123",
		Type:      "User",
		AuthType:  "x-rh-identity",
	}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.CheckSelfBulkRequest{
		Items: []*pb.CheckSelfBulkRequestItem{
			{
				Object: &pb.ResourceReference{
					ResourceId:   "resource-1",
					ResourceType: "host",
					Reporter:     &pb.ReporterReference{Type: "hbi"},
				},
				Relation: "view",
			},
			{
				Object: &pb.ResourceReference{
					ResourceId:   "resource-2",
					ResourceType: "host",
					Reporter:     &pb.ReporterReference{Type: "hbi"},
				},
				Relation: "edit",
			},
		},
	}

	mockAuthz := &mocks.MockAuthz{}
	mockAuthz.
		On("CheckBulk", mock.Anything, mock.Anything).
		Return(&relationsV1beta1.CheckBulkResponse{
			Pairs: []*relationsV1beta1.CheckBulkResponsePair{
				{
					Request: &relationsV1beta1.CheckBulkRequestItem{
						Resource: &relationsV1beta1.ObjectReference{
							Type: &relationsV1beta1.ObjectType{
								Namespace: "hbi",
								Name:      "host",
							},
							Id: "resource-1",
						},
						Relation: "view",
					},
					Response: &relationsV1beta1.CheckBulkResponsePair_Item{
						Item: &relationsV1beta1.CheckBulkResponseItem{
							Allowed: relationsV1beta1.CheckBulkResponseItem_ALLOWED_TRUE,
						},
					},
				},
				{
					Request: &relationsV1beta1.CheckBulkRequestItem{
						Resource: &relationsV1beta1.ObjectReference{
							Type: &relationsV1beta1.ObjectType{
								Namespace: "hbi",
								Name:      "host",
							},
							Id: "resource-2",
						},
						Relation: "edit",
					},
					Response: &relationsV1beta1.CheckBulkResponsePair_Item{
						Item: &relationsV1beta1.CheckBulkResponseItem{
							Allowed: relationsV1beta1.CheckBulkResponseItem_ALLOWED_FALSE,
						},
					},
				},
			},
			ConsistencyToken: &relationsV1beta1.ConsistencyToken{Token: "test-token"},
		}, nil).
		Once()

	cfg := &usecase.UsecaseConfig{}
	uc := usecase.New(
		data.NewFakeResourceRepository(),
		nil,                                // LegacyReporterResourceRepository
		nil,                                // inventoryResourceRepository
		data.NewInMemorySchemaRepository(), // schema repository
		mockAuthz,                          // Authz
		nil,                                // Eventer
		"rbac",                             // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
	)

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckSelfBulk(ctx, req)

	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Len(t, resp.Pairs, 2)
		assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed)
		assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Pairs[1].GetItem().Allowed)
		// Verify request items are mapped back correctly (no subject in CheckSelfBulkRequestItem)
		assert.Equal(t, "resource-1", resp.Pairs[0].Request.Object.ResourceId)
		assert.Equal(t, "view", resp.Pairs[0].Request.Relation)
		assert.Equal(t, "resource-2", resp.Pairs[1].Request.Object.ResourceId)
		assert.Equal(t, "edit", resp.Pairs[1].Request.Relation)
	}

	mockAuthz.AssertExpectations(t)
}

func TestInventoryService_CheckSelfBulk_NoIdentity(t *testing.T) {
	ctx := context.Background() // no identity

	req := &pb.CheckSelfBulkRequest{
		Items: []*pb.CheckSelfBulkRequestItem{
			{
				Object: &pb.ResourceReference{
					ResourceId:   "resource-1",
					ResourceType: "host",
					Reporter:     &pb.ReporterReference{Type: "hbi"},
				},
				Relation: "view",
			},
		},
	}

	uc := &usecase.Usecase{}
	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckSelfBulk(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to get identity")

	// Check that it returns the correct gRPC status code
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, grpcStatus.Code())
}

func TestInventoryService_CheckSelf_OIDC_Identity(t *testing.T) {
	// Test CheckSelf with OIDC identity (Principal in "domain/subject" format)
	id := &authnapi.Identity{
		Principal: "redhat.com/12345",
		AuthType:  "oidc",
		Type:      "",
	}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	mockRepo := data.NewFakeResourceRepository()
	mockAuthz := &mocks.MockAuthz{}

	cfg := &usecase.UsecaseConfig{}
	uc := usecase.New(
		mockRepo,
		nil,                                // LegacyReporterResourceRepository
		nil,                                // inventoryResourceRepository
		data.NewInMemorySchemaRepository(), // schema repository
		mockAuthz,                          // Authz
		nil,                                // Eventer
		"rbac",                             // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
	)

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckSelf(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}
