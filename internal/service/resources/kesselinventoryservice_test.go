package resources_test

import (
	"context"
	"fmt"
	"regexp"

	"io"
	"testing"

	krlog "github.com/go-kratos/kratos/v2/log"
	kratosTransport "github.com/go-kratos/kratos/v2/transport"
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
	"github.com/project-kessel/inventory-api/internal/subject/selfsubject"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type testSelfSubjectStrategy struct{}

func (testSelfSubjectStrategy) SubjectFromAuthorizationContext(authzContext authnapi.AuthzContext) (string, error) {
	if !authzContext.IsAuthenticated() || authzContext.Claims.SubjectId == "" {
		return "", fmt.Errorf("subject claims not found")
	}
	return string(authzContext.Claims.SubjectId), nil
}

func newTestResolver() *selfsubject.Resolver {
	return selfsubject.NewResolver(testSelfSubjectStrategy{})
}

func TestInventoryService_ReportResource_MissingReporterType(t *testing.T) {
	claims := &authnapi.Claims{SubjectId: authnapi.SubjectId("tester"), AuthType: authnapi.AuthTypeXRhIdentity}
	ctx := ctxWithClaimsAndTransport(claims, kratosTransport.KindGRPC)

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
		nil, // Authz
		nil, // Eventer
		"",  // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		nil, // Config
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestResolver(),
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
	claims := &authnapi.Claims{SubjectId: authnapi.SubjectId("tester"), AuthType: authnapi.AuthTypeXRhIdentity}
	ctx := ctxWithClaimsAndTransport(claims, kratosTransport.KindGRPC)

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
		nil, // Authz
		nil, // Eventer
		"",  // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		nil, // Config
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestResolver(),
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
	claims := &authnapi.Claims{SubjectId: authnapi.SubjectId("sarah"), AuthType: authnapi.AuthTypeXRhIdentity}
	ctx := ctxWithClaimsAndTransport(claims, kratosTransport.KindGRPC)

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
		nil, // Authz
		nil, // Eventer
		"",  // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		nil, // Config
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestResolver(),
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
	assert.Contains(t, err.Error(), "failed to get claims")

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

type testTransport struct {
	kind kratosTransport.Kind
}

func (t testTransport) Kind() kratosTransport.Kind { return t.kind }
func (testTransport) Endpoint() string             { return "" }
func (testTransport) Operation() string            { return "" }
func (testTransport) RequestHeader() kratosTransport.Header {
	return nil
}
func (testTransport) ReplyHeader() kratosTransport.Header { return nil }

func ctxWithClaimsAndTransport(claims *authnapi.Claims, kind kratosTransport.Kind) context.Context {
	ctx := context.WithValue(context.Background(), middleware.ClaimsRequestKey, claims)
	ctx = kratosTransport.NewServerContext(ctx, testTransport{kind: kind})
	return middleware.EnsureAuthzContext(ctx, claims)
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
	// Test CheckSelf with x-rh-identity claims (using SubjectId)
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	ctx := ctxWithClaimsAndTransport(claims, kratosTransport.KindHTTP)

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
				// Verify subject is derived from claims (SubjectId for x-rh-identity)
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
		mockAuthz, // Authz
		nil,       // Eventer
		"rbac",    // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestResolver(),
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
	// Test CheckSelf with x-rh-identity claims (SubjectId fallback value)
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("testuser"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	ctx := ctxWithClaimsAndTransport(claims, kratosTransport.KindHTTP)

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
				// Verify subject uses claims.SubjectId
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
		mockAuthz, // Authz
		nil,       // Eventer
		"rbac",    // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestResolver(),
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
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	ctx := ctxWithClaimsAndTransport(claims, kratosTransport.KindHTTP)

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
		mockAuthz, // Authz
		nil,       // Eventer
		"rbac",    // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestResolver(),
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
	assert.Contains(t, err.Error(), "authz context missing")

	// Check that it returns the correct gRPC status code
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, grpcStatus.Code())
}

func TestInventoryService_CheckSelfBulk_Allowed_XRhIdentity(t *testing.T) {
	// Test CheckSelfBulk with x-rh-identity claims
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	ctx := ctxWithClaimsAndTransport(claims, kratosTransport.KindHTTP)

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
				// Verify all items use the same subject (derived from claims)
				if len(req.Items) != 2 {
					return false
				}
				// Check that both items have the same subject (SubjectId from x-rh-identity)
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
		mockAuthz, // Authz
		nil,       // Eventer
		"rbac",    // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestResolver(),
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
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	ctx := ctxWithClaimsAndTransport(claims, kratosTransport.KindHTTP)

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
		mockAuthz, // Authz
		nil,       // Eventer
		"rbac",    // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestResolver(),
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

func TestInventoryService_CheckSelfBulk_ResponseLengthMismatch(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	ctx := ctxWithClaimsAndTransport(claims, kratosTransport.KindHTTP)

	req := &pb.CheckSelfBulkRequest{
		Items: []*pb.CheckSelfBulkRequestItem{
			{
				Relation: "view",
				Object: &pb.ResourceReference{
					ResourceId:   "resource-1",
					ResourceType: "host",
					Reporter:     &pb.ReporterReference{Type: "hbi"},
				},
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
						Relation: "view",
					},
					Response: &relationsV1beta1.CheckBulkResponsePair_Item{
						Item: &relationsV1beta1.CheckBulkResponseItem{
							Allowed: relationsV1beta1.CheckBulkResponseItem_ALLOWED_TRUE,
						},
					},
				},
			},
		}, nil).
		Once()

	cfg := &usecase.UsecaseConfig{}
	uc := usecase.New(
		data.NewFakeResourceRepository(),
		mockAuthz, // Authz
		nil,       // Eventer
		"rbac",    // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestResolver(),
	)

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckSelfBulk(ctx, req)

	assert.Nil(t, resp)
	assert.Error(t, err)
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, grpcStatus.Code())
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
	assert.Contains(t, err.Error(), "authz context missing")

	// Check that it returns the correct gRPC status code
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, grpcStatus.Code())
}

func TestInventoryService_CheckSelf_OIDC_Identity(t *testing.T) {
	// Test CheckSelf with OIDC claims (SubjectId in "domain/subject" format)
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("redhat.com/12345"),
		AuthType:  authnapi.AuthTypeOIDC,
	}
	ctx := ctxWithClaimsAndTransport(claims, kratosTransport.KindHTTP)

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
		mockAuthz, // Authz
		nil,       // Eventer
		"rbac",    // Namespace
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		cfg,
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestResolver(),
	)

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckSelf(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, grpcStatus.Code())
}
