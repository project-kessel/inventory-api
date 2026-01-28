package resources_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"testing"

	krlog "github.com/go-kratos/kratos/v2/log"
	kratosMiddleware "github.com/go-kratos/kratos/v2/middleware"
	kratosTransport "github.com/go-kratos/kratos/v2/transport"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	relationsV1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
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

// StubAuthenticator is a configurable authenticator for testing.
// It returns the configured Claims and Decision for any authentication request.
type StubAuthenticator struct {
	Claims   *authnapi.Claims
	Decision authnapi.Decision
}

// Authenticate implements authnapi.Authenticator.
func (s *StubAuthenticator) Authenticate(_ context.Context, _ kratosTransport.Transporter) (*authnapi.Claims, authnapi.Decision) {
	return s.Claims, s.Decision
}

// PermissiveMetaAuthorizer is a MetaAuthorizer that allows all operations for testing.
// Use this when testing service logic without protocol-based authorization restrictions.
type PermissiveMetaAuthorizer struct{}

// Check implements metaauthorizer.MetaAuthorizer and always returns true.
func (p *PermissiveMetaAuthorizer) Check(_ context.Context, _ model.RelationsResource, _ metaauthorizer.Relation, _ authnapi.AuthzContext) (bool, error) {
	return true, nil
}

const bufSize = 1024 * 1024

// testServerConfig holds configuration for creating a test gRPC server.
type testServerConfig struct {
	Usecase       *usecase.Usecase
	Authenticator authnapi.Authenticator // nil = no auth middleware
}

// newTestServer creates a gRPC server with the KesselInventoryService using
// an in-memory bufconn transport. Cleanup is registered via t.Cleanup() automatically.
// The server uses the production Authentication middleware with the provided authenticator.
func newTestServer(t *testing.T, cfg testServerConfig) pb.KesselInventoryServiceClient {
	t.Helper()

	lis := bufconn.Listen(bufSize)

	// Build middleware chain - reuse production Authentication middleware
	var middlewares []kratosMiddleware.Middleware
	if cfg.Authenticator != nil {
		middlewares = append(middlewares, middleware.Authentication(cfg.Authenticator))
	}

	// Create Kratos gRPC server with bufconn listener
	// We need to provide an explicit endpoint since bufconn doesn't have a real address
	testEndpoint := &url.URL{Scheme: "grpc", Host: "bufconn"}
	srv := kgrpc.NewServer(
		kgrpc.Listener(lis),
		kgrpc.Endpoint(testEndpoint),
		kgrpc.Middleware(middlewares...),
	)

	// Register service
	service := svc.NewKesselInventoryServiceV1beta2(cfg.Usecase)
	pb.RegisterKesselInventoryServiceServer(srv, service)

	// Start server in background
	go func() {
		if err := srv.Start(context.Background()); err != nil {
			t.Logf("Server exited: %v", err)
		}
	}()

	// Create client via bufconn dialer
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close()
		srv.Stop(context.Background())
	})

	return pb.NewKesselInventoryServiceClient(conn)
}

// testUsecaseConfig holds optional overrides for constructing a test Usecase.
// All fields have sensible defaults when left as zero values.
type testUsecaseConfig struct {
	Repo           data.ResourceRepository
	Authz          authzapi.Authorizer
	Namespace      string
	Config         *usecase.UsecaseConfig
	MetaAuthorizer metaauthorizer.MetaAuthorizer
}

// newTestUsecase constructs a Usecase with test defaults.
// Override specific fields via testUsecaseConfig; unset fields use defaults.
func newTestUsecase(cfg testUsecaseConfig) *usecase.Usecase {
	repo := cfg.Repo
	if repo == nil {
		repo = data.NewFakeResourceRepository()
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "rbac"
	}

	usecaseCfg := cfg.Config
	if usecaseCfg == nil {
		usecaseCfg = &usecase.UsecaseConfig{}
	}

	// Default to PermissiveMetaAuthorizer for tests unless explicitly overridden
	meta := cfg.MetaAuthorizer
	if meta == nil {
		meta = &PermissiveMetaAuthorizer{}
	}

	return usecase.New(
		repo,
		cfg.Authz,
		nil, // Eventer
		namespace,
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		usecaseCfg,
		metricscollector.NewFakeMetricsCollector(),
		meta,
		newTestResolver(),
	)
}

func TestInventoryService_ReportResource_MissingReporterType(t *testing.T) {
	claims := &authnapi.Claims{SubjectId: authnapi.SubjectId("tester"), AuthType: authnapi.AuthTypeXRhIdentity}

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

	uc := newTestUsecase(testUsecaseConfig{})
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})
	resp, err := client.ReportResource(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	// Check that it returns the correct gRPC status code
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, grpcStatus.Code())
}

func TestInventoryService_ReportResource_MissingReporterInstanceId(t *testing.T) {
	claims := &authnapi.Claims{SubjectId: authnapi.SubjectId("tester"), AuthType: authnapi.AuthTypeXRhIdentity}

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

	uc := newTestUsecase(testUsecaseConfig{})
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})
	resp, err := client.ReportResource(context.Background(), req)

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

	uc := newTestUsecase(testUsecaseConfig{})
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})
	resp, err := client.ReportResource(context.Background(), req)

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
	req := &pb.DeleteResourceRequest{}

	uc := newTestUsecase(testUsecaseConfig{})
	// No authenticator configured - simulates unauthenticated request
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: nil,
	})
	resp, err := client.DeleteResource(context.Background(), req)

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

	req := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

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

	uc := newTestUsecase(testUsecaseConfig{Authz: mockAuthz})
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})
	resp, err := client.CheckSelf(context.Background(), req)

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

	req := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

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

	uc := newTestUsecase(testUsecaseConfig{Authz: mockAuthz})
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})
	resp, err := client.CheckSelf(context.Background(), req)

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

	req := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

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

	uc := newTestUsecase(testUsecaseConfig{Authz: mockAuthz})
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})
	resp, err := client.CheckSelf(context.Background(), req)

	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Allowed)
	}

	mockAuthz.AssertExpectations(t)
}

func TestInventoryService_CheckSelf_NoIdentity(t *testing.T) {
	req := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	uc := newTestUsecase(testUsecaseConfig{})
	// No authenticator configured - simulates unauthenticated request
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: nil,
	})
	resp, err := client.CheckSelf(context.Background(), req)

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

	uc := newTestUsecase(testUsecaseConfig{Authz: mockAuthz})
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})
	resp, err := client.CheckSelfBulk(context.Background(), req)

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

	uc := newTestUsecase(testUsecaseConfig{Authz: mockAuthz})
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})
	resp, err := client.CheckSelfBulk(context.Background(), req)

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

	uc := newTestUsecase(testUsecaseConfig{Authz: mockAuthz})
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})
	resp, err := client.CheckSelfBulk(context.Background(), req)

	assert.Nil(t, resp)
	assert.Error(t, err)
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, grpcStatus.Code())
	mockAuthz.AssertExpectations(t)
}

func TestInventoryService_CheckSelfBulk_NoIdentity(t *testing.T) {
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

	uc := newTestUsecase(testUsecaseConfig{})
	// No authenticator configured - simulates unauthenticated request
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: nil,
	})
	resp, err := client.CheckSelfBulk(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "authz context missing")

	// Check that it returns the correct gRPC status code
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, grpcStatus.Code())
}

func TestInventoryService_Check_NoIdentity(t *testing.T) {
	req := &pb.CheckRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "resource-123",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "user-456",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	uc := newTestUsecase(testUsecaseConfig{})
	// No authenticator configured - simulates unauthenticated request
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: nil,
	})
	resp, err := client.Check(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, grpcStatus.Code())
	assert.Contains(t, err.Error(), "failed to get claims")
}

func TestInventoryService_CheckSelf_OIDC_Identity(t *testing.T) {
	// Test CheckSelf with OIDC claims (SubjectId in "domain/subject" format)
	// This test verifies that OIDC identities are denied at the meta-authorization layer.
	// We use the real SimpleMetaAuthorizer (not the permissive one) to test this behavior.
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("12345"),
		AuthType:  authnapi.AuthTypeOIDC,
	}

	req := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	mockAuthz := &mocks.MockAuthz{}
	// Use SimpleMetaAuthorizer to test protocol/auth-type based authorization
	uc := newTestUsecase(testUsecaseConfig{
		Authz:          mockAuthz,
		MetaAuthorizer: metaauthorizer.NewSimpleMetaAuthorizer(),
	})
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})
	resp, err := client.CheckSelf(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, grpcStatus.Code())
}
