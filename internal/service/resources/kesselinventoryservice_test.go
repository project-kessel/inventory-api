package resources_test

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"testing"

	krlog "github.com/go-kratos/kratos/v2/log"
	kratosTransport "github.com/go-kratos/kratos/v2/transport"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/schema"
	"github.com/project-kessel/inventory-api/internal/biz/schema/validation"
	relationsV1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authz"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	usecase "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/mocks"
	svc "github.com/project-kessel/inventory-api/internal/service/resources"
	"github.com/project-kessel/inventory-api/internal/subject/selfsubject"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type testSelfSubjectStrategy struct{}

func (testSelfSubjectStrategy) SubjectFromAuthorizationContext(authzContext authnapi.AuthzContext) (model.SubjectReference, error) {
	if !authzContext.IsAuthenticated() || authzContext.Subject.SubjectId == "" {
		return model.SubjectReference{}, fmt.Errorf("subject claims not found")
	}
	subjectID := string(authzContext.Subject.SubjectId)
	return buildTestSubjectReference(subjectID)
}

// buildTestSubjectReference creates a model.SubjectReference for testing.
func buildTestSubjectReference(subjectID string) (model.SubjectReference, error) {
	localResourceId, err := model.NewLocalResourceId(subjectID)
	if err != nil {
		return model.SubjectReference{}, err
	}
	resourceType, err := model.NewResourceType("principal")
	if err != nil {
		return model.SubjectReference{}, err
	}
	reporterType, err := model.NewReporterType("rbac")
	if err != nil {
		return model.SubjectReference{}, err
	}
	key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, model.ReporterInstanceId(""))
	if err != nil {
		return model.SubjectReference{}, err
	}
	return model.NewSubjectReferenceWithoutRelation(key), nil
}

func newTestSelfSubjectStrategy() selfsubject.SelfSubjectStrategy {
	return testSelfSubjectStrategy{}
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

// DenyAuthenticator is an authenticator that always denies requests.
// Use this to test unauthenticated behavior without causing nil pointer panics.
type DenyAuthenticator struct{}

// Authenticate implements authnapi.Authenticator and always returns Deny.
func (d *DenyAuthenticator) Authenticate(_ context.Context, _ kratosTransport.Transporter) (*authnapi.Claims, authnapi.Decision) {
	return nil, authnapi.Deny
}

// PermissiveMetaAuthorizer is a MetaAuthorizer that allows all operations for testing.
// Use this when testing service logic without protocol-based authorization restrictions.
type PermissiveMetaAuthorizer struct{}

// Check implements metaauthorizer.MetaAuthorizer and always returns true.
func (p *PermissiveMetaAuthorizer) Check(_ context.Context, _ metaauthorizer.MetaObject, _ metaauthorizer.Relation, _ authnapi.AuthzContext) (bool, error) {
	return true, nil
}

// DenyingMetaAuthorizer is a MetaAuthorizer that denies all operations for testing.
// Use this to test that meta-authorization denial errors are properly mapped.
type DenyingMetaAuthorizer struct{}

// Check implements metaauthorizer.MetaAuthorizer and always returns false.
func (d *DenyingMetaAuthorizer) Check(_ context.Context, _ metaauthorizer.MetaObject, _ metaauthorizer.Relation, _ authnapi.AuthzContext) (bool, error) {
	return false, nil
}

// newTestServer is a convenience wrapper around newTestGRPCServer for tests
// that have not yet been migrated to the dual-protocol framework.
func newTestServer(t *testing.T, cfg TestServerConfig) pb.KesselInventoryServiceClient {
	t.Helper()
	return newTestGRPCServer(t, cfg)
}

// testUsecaseConfig holds optional overrides for constructing a test Usecase.
// All fields have sensible defaults when left as zero values.
type testUsecaseConfig struct {
	Repo           data.ResourceRepository
	SchemaRepo     schema.Repository
	Authz          authzapi.Authorizer
	Namespace      string
	Config         *usecase.UsecaseConfig
	MetaAuthorizer metaauthorizer.MetaAuthorizer
}

// newTestUsecase constructs a Usecase with test defaults.
// Override specific fields via testUsecaseConfig; unset fields use defaults.
func newTestUsecase(t *testing.T, cfg testUsecaseConfig) *usecase.Usecase {
	repo := cfg.Repo
	if repo == nil {
		repo = data.NewFakeResourceRepository()
	}

	schemaRepo := cfg.SchemaRepo
	if schemaRepo == nil {
		schemaRepo = newFakeSchemaRepository(t)
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

	// Default to SimpleAuthorizer when Authz is nil
	authzImpl := cfg.Authz
	if authzImpl == nil {
		authzImpl = authz.NewSimpleAuthorizer()
	}

	return usecase.New(
		repo,
		schemaRepo,
		authzImpl,
		namespace,
		krlog.NewStdLogger(io.Discard),
		nil, // ListenManager
		nil, // waitForNotifBreaker
		usecaseCfg,
		metricscollector.NewFakeMetricsCollector(),
		meta,
		newTestSelfSubjectStrategy(),
	)
}

func TestInventoryService_ReportResource_MissingReporterType(t *testing.T) {
	claims := &authnapi.Claims{SubjectId: authnapi.SubjectId("tester"), AuthType: authnapi.AuthTypeXRhIdentity}

	protoReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "v1",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-123"),
					"hostname":     structpb.NewStringValue("example-host"),
				},
			},
			Reporter: &structpb.Struct{},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res, requireErrorContaining(codes.InvalidArgument, "reporter_type"))
			}
	})
}

func TestInventoryService_ReportResource_MissingReporterInstanceId(t *testing.T) {
	claims := &authnapi.Claims{SubjectId: authnapi.SubjectId("tester"), AuthType: authnapi.AuthTypeXRhIdentity}

	protoReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "v1",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-123"),
					"hostname":     structpb.NewStringValue("example-host"),
				},
			},
			Reporter: &structpb.Struct{},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res, requireErrorContaining(codes.InvalidArgument, "reporter_instance_id"))
			}
	})
}

func TestResponseFromResource(t *testing.T) {
	resp := svc.ResponseFromResource()

	assert.NotNil(t, resp)
	assert.IsType(t, &pb.ReportResourceResponse{}, resp)
	assert.Equal(t, &pb.ReportResourceResponse{}, resp)
}

func TestInventoryService_DeleteResource_NoIdentity(t *testing.T) {
	instanceID := "instance-001"
	protoReq := &pb.DeleteResourceRequest{
		Reference: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "resource-123",
			Reporter: &pb.ReporterReference{
				Type:       "hbi",
				InstanceId: &instanceID,
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &DenyAuthenticator{},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, DeleteResource, httpEndpoint("DELETE /api/kessel/v1beta2/resources")))
				Assert(t, res, requireError(codes.Unauthenticated))
			}
	})
}

func TestToLookupResourcesCommand(t *testing.T) {
	permission := "view"
	reporterType := "hbi"
	input := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host",
			ReporterType: &reporterType,
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

	result, err := svc.ToLookupResourcesCommand(input)
	require.NoError(t, err)

	// Verify the command fields
	assert.Equal(t, "host", result.ResourceType.Serialize())
	assert.Equal(t, "hbi", result.ReporterType.Serialize())
	assert.Equal(t, "view", result.Relation.Serialize())
	assert.Equal(t, "res-id", result.Subject.Subject().LocalResourceId().Serialize())
	assert.Equal(t, "principal", result.Subject.Subject().ResourceType().Serialize())
	assert.Equal(t, "rbac", result.Subject.Subject().ReporterType().Serialize())
	assert.Equal(t, uint32(50), result.Limit)
}

func TestToLookupResourcesCommand_WithConsistencyToken(t *testing.T) {
	permission := "view"
	reporterType := "hbi"
	token := "test-consistency-token"
	input := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host",
			ReporterType: &reporterType,
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
		Consistency: &pb.Consistency{
			Requirement: &pb.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &pb.ConsistencyToken{Token: token},
			},
		},
	}

	result, err := svc.ToLookupResourcesCommand(input)
	require.NoError(t, err)

	assert.False(t, result.Consistency.MinimizeLatency(), "expected at-least-as-fresh when token provided")
	assert.Equal(t, token, result.Consistency.AtLeastAsFresh().String(), "command should use the request consistency token")
}

func TestToLookupResourcesCommand_NoPagination(t *testing.T) {
	permission := "view"
	reporterType := "hbi"
	input := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host",
			ReporterType: &reporterType,
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
		// Pagination intentionally omitted
	}

	result, err := svc.ToLookupResourcesCommand(input)
	require.NoError(t, err)

	assert.Equal(t, "host", result.ResourceType.Serialize())
	assert.Equal(t, "hbi", result.ReporterType.Serialize())
	assert.Equal(t, "view", result.Relation.Serialize())
	assert.Equal(t, "res-id", result.Subject.Subject().LocalResourceId().Serialize())
	assert.Equal(t, "principal", result.Subject.Subject().ResourceType().Serialize())
	assert.Equal(t, "rbac", result.Subject.Subject().ReporterType().Serialize())
	assert.Equal(t, uint32(1000), result.Limit)
	assert.Equal(t, "", result.Continuation)
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
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
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
					return sub.Subject.Id == "user-123" &&
						sub.Subject.Type.Name == "principal" &&
						sub.Subject.Type.Namespace == "rbac"
				}),
			).
			Return(relationsV1beta1.CheckResponse_ALLOWED_TRUE, &relationsV1beta1.ConsistencyToken{}, nil).
			Once()
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Authz: mockAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelf, httpEndpoint("POST /api/kessel/v1beta2/checkself")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckSelfResponse { return &pb.CheckSelfResponse{} }))
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Allowed)
				mockAuthz.AssertExpectations(t)
			}
	})
}

func TestInventoryService_CheckSelf_Allowed_XRhIdentity_SubjectIdMatch(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("testuser"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
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
					return sub.Subject.Id == "testuser" &&
						sub.Subject.Type.Name == "principal" &&
						sub.Subject.Type.Namespace == "rbac"
				}),
			).
			Return(relationsV1beta1.CheckResponse_ALLOWED_TRUE, &relationsV1beta1.ConsistencyToken{}, nil).
			Once()
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Authz: mockAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelf, httpEndpoint("POST /api/kessel/v1beta2/checkself")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckSelfResponse { return &pb.CheckSelfResponse{} }))
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Allowed)
				mockAuthz.AssertExpectations(t)
			}
	})
}

func TestInventoryService_CheckSelf_Denied(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
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
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Authz: mockAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelf, httpEndpoint("POST /api/kessel/v1beta2/checkself")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckSelfResponse { return &pb.CheckSelfResponse{} }))
				assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Allowed)
				mockAuthz.AssertExpectations(t)
			}
	})
}

func TestInventoryService_CheckSelf_NoIdentity(t *testing.T) {
	protoReq := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &DenyAuthenticator{},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelf, httpEndpoint("POST /api/kessel/v1beta2/checkself")))
				Assert(t, res, requireError(codes.Unauthenticated))
			}
	})
}

func TestInventoryService_CheckSelfBulk_Allowed_XRhIdentity(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckSelfBulkRequest{
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

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		mockAuthz := &mocks.MockAuthz{}
		mockAuthz.
			On("CheckBulk",
				mock.Anything,
				mock.MatchedBy(func(req *relationsV1beta1.CheckBulkRequest) bool {
					if len(req.Items) != 2 {
						return false
					}
					s1 := req.Items[0].Subject
					s2 := req.Items[1].Subject
					return s1.Subject.Id == "user-123" &&
						s1.Subject.Type.Name == "principal" &&
						s1.Subject.Type.Namespace == "rbac" &&
						s2.Subject.Id == "user-123" &&
						s2.Subject.Type.Name == "principal" &&
						s2.Subject.Type.Namespace == "rbac"
				}),
			).
			Return(&relationsV1beta1.CheckBulkResponse{
				Pairs: []*relationsV1beta1.CheckBulkResponsePair{
					{
						Request: &relationsV1beta1.CheckBulkRequestItem{
							Resource: &relationsV1beta1.ObjectReference{
								Type: &relationsV1beta1.ObjectType{Namespace: "hbi", Name: "host"},
								Id:   "resource-1",
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
								Type: &relationsV1beta1.ObjectType{Namespace: "hbi", Name: "host"},
								Id:   "resource-2",
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
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Authz: mockAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckSelfBulkResponse { return &pb.CheckSelfBulkResponse{} }))
				require.Len(t, resp.Pairs, 2)
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed)
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[1].GetItem().Allowed)
				assert.NotNil(t, resp.ConsistencyToken)
				assert.Equal(t, "test-token", resp.ConsistencyToken.GetToken())
				mockAuthz.AssertExpectations(t)
			}
	})
}

func TestInventoryService_CheckSelfBulk_MixedResults(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckSelfBulkRequest{
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

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		mockAuthz := &mocks.MockAuthz{}
		mockAuthz.
			On("CheckBulk", mock.Anything, mock.Anything).
			Return(&relationsV1beta1.CheckBulkResponse{
				Pairs: []*relationsV1beta1.CheckBulkResponsePair{
					{
						Request: &relationsV1beta1.CheckBulkRequestItem{
							Resource: &relationsV1beta1.ObjectReference{
								Type: &relationsV1beta1.ObjectType{Namespace: "hbi", Name: "host"},
								Id:   "resource-1",
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
								Type: &relationsV1beta1.ObjectType{Namespace: "hbi", Name: "host"},
								Id:   "resource-2",
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
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Authz: mockAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckSelfBulkResponse { return &pb.CheckSelfBulkResponse{} }))
				require.Len(t, resp.Pairs, 2)
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed)
				assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Pairs[1].GetItem().Allowed)
				assert.Equal(t, "resource-1", resp.Pairs[0].Request.Object.ResourceId)
				assert.Equal(t, "view", resp.Pairs[0].Request.Relation)
				assert.Equal(t, "resource-2", resp.Pairs[1].Request.Object.ResourceId)
				assert.Equal(t, "edit", resp.Pairs[1].Request.Relation)
				mockAuthz.AssertExpectations(t)
			}
	})
}

func TestInventoryService_CheckSelfBulk_ResponseLengthMismatch(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckSelfBulkRequest{
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

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		mockAuthz := &mocks.MockAuthz{}
		mockAuthz.
			On("CheckBulk", mock.Anything, mock.Anything).
			Return(&relationsV1beta1.CheckBulkResponse{
				Pairs: []*relationsV1beta1.CheckBulkResponsePair{
					{
						Request: &relationsV1beta1.CheckBulkRequestItem{
							Resource: &relationsV1beta1.ObjectReference{
								Type: &relationsV1beta1.ObjectType{Namespace: "hbi", Name: "host"},
								Id:   "resource-1",
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
								Type: &relationsV1beta1.ObjectType{Namespace: "hbi", Name: "host"},
								Id:   "resource-2",
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
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Authz: mockAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
				Assert(t, res, requireError(codes.Internal).And(func(t *testing.T) { mockAuthz.AssertExpectations(t) }))
			}
	})
}

func TestInventoryService_CheckSelfBulk_NoIdentity(t *testing.T) {
	protoReq := &pb.CheckSelfBulkRequest{
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

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &DenyAuthenticator{},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
				Assert(t, res, requireError(codes.Unauthenticated))
			}
	})
}

func TestInventoryService_Check_NoIdentity(t *testing.T) {
	protoReq := &pb.CheckRequest{
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

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &DenyAuthenticator{},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, Check, httpEndpoint("POST /api/kessel/v1beta2/check")))
				Assert(t, res, requireError(codes.Unauthenticated))
			}
	})
}

func TestInventoryService_CheckSelf_OIDC_Identity(t *testing.T) {
	// SimpleMetaAuthorizer denies check_self on gRPC (check_self is excluded from gRPC),
	// and denies OIDC on HTTP. So OIDC + check_self = denied on both protocols.
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("12345"),
		AuthType:  authnapi.AuthTypeOIDC,
	}

	protoReq := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase: newTestUsecase(t, testUsecaseConfig{
					MetaAuthorizer: metaauthorizer.NewSimpleMetaAuthorizer(),
				}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelf, httpEndpoint("POST /api/kessel/v1beta2/checkself")))
				Assert(t, res, requireError(codes.PermissionDenied))
			}
	})
}

// --- Success Path Tests Using SimpleAuthorizer ---

func TestInventoryService_Check_Allowed(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "resource-abc",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-456",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		simpleAuthz := authz.NewSimpleAuthorizer()
		simpleAuthz.Grant("subject-456", "view", "hbi", "host", "resource-abc")
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Authz: simpleAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, Check, httpEndpoint("POST /api/kessel/v1beta2/check")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckResponse { return &pb.CheckResponse{} }))
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Allowed)
			}
	})
}

func TestInventoryService_Check_Denied(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "resource-abc",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-456",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, Check, httpEndpoint("POST /api/kessel/v1beta2/check")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckResponse { return &pb.CheckResponse{} }))
				assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Allowed)
			}
	})
}

func TestInventoryService_Check_MetaAuthzDenied(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "resource-123",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-456",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{MetaAuthorizer: &DenyingMetaAuthorizer{}}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, Check, httpEndpoint("POST /api/kessel/v1beta2/check")))
				Assert(t, res, requireError(codes.PermissionDenied))
			}
	})
}

func TestInventoryService_CheckForUpdate_Allowed(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckForUpdateRequest{
		Relation: "edit",
		Object: &pb.ResourceReference{
			ResourceId:   "resource-xyz",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-789",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		simpleAuthz := authz.NewSimpleAuthorizer()
		simpleAuthz.Grant("subject-789", "edit", "hbi", "host", "resource-xyz")
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Authz: simpleAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdate, httpEndpoint("POST /api/kessel/v1beta2/checkforupdate")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckForUpdateResponse { return &pb.CheckForUpdateResponse{} }))
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Allowed)
			}
	})
}

func TestInventoryService_CheckForUpdate_Denied(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckForUpdateRequest{
		Relation: "edit",
		Object: &pb.ResourceReference{
			ResourceId:   "resource-xyz",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-789",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdate, httpEndpoint("POST /api/kessel/v1beta2/checkforupdate")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckForUpdateResponse { return &pb.CheckForUpdateResponse{} }))
				assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Allowed)
			}
	})
}

func TestInventoryService_CheckForUpdate_MetaAuthzDenied(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckForUpdateRequest{
		Relation: "edit",
		Object: &pb.ResourceReference{
			ResourceId:   "resource-123",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-456",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{MetaAuthorizer: &DenyingMetaAuthorizer{}}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdate, httpEndpoint("POST /api/kessel/v1beta2/checkforupdate")))
				Assert(t, res, requireError(codes.PermissionDenied))
			}
	})
}

func TestInventoryService_CheckBulk_MixedResults(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckBulkRequest{
		Items: []*pb.CheckBulkRequestItem{
			{
				Object: &pb.ResourceReference{
					ResourceId:   "resource-1",
					ResourceType: "host",
					Reporter:     &pb.ReporterReference{Type: "hbi"},
				},
				Subject: &pb.SubjectReference{
					Resource: &pb.ResourceReference{
						ResourceId:   "subject-a",
						ResourceType: "principal",
						Reporter:     &pb.ReporterReference{Type: "rbac"},
					},
				},
				Relation: "view",
			},
			{
				Object: &pb.ResourceReference{
					ResourceId:   "resource-2",
					ResourceType: "host",
					Reporter:     &pb.ReporterReference{Type: "hbi"},
				},
				Subject: &pb.SubjectReference{
					Resource: &pb.ResourceReference{
						ResourceId:   "subject-b",
						ResourceType: "principal",
						Reporter:     &pb.ReporterReference{Type: "rbac"},
					},
				},
				Relation: "edit",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		simpleAuthz := authz.NewSimpleAuthorizer()
		simpleAuthz.Grant("subject-a", "view", "hbi", "host", "resource-1")
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Authz: simpleAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckBulk, httpEndpoint("POST /api/kessel/v1beta2/checkbulk")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckBulkResponse { return &pb.CheckBulkResponse{} }))
				require.Len(t, resp.Pairs, 2)
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed)
				assert.Equal(t, "resource-1", resp.Pairs[0].Request.Object.ResourceId)
				assert.Equal(t, "view", resp.Pairs[0].Request.Relation)
				assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Pairs[1].GetItem().Allowed)
				assert.Equal(t, "resource-2", resp.Pairs[1].Request.Object.ResourceId)
				assert.Equal(t, "edit", resp.Pairs[1].Request.Relation)
				assert.NotNil(t, resp.ConsistencyToken)
			}
	})
}

func TestInventoryService_ReportResource_Success(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "my-host-123",
				ApiHref:         "https://api.example.com/hosts/my-host-123",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue("example-host.example.com"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"reporter_field": structpb.NewStringValue("reporter-value"),
				},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				resp := Extract(t, res, expectSuccess(func() *pb.ReportResourceResponse { return &pb.ReportResourceResponse{} }))
				assert.NotNil(t, resp)
			}
	})
}

func TestInventoryService_ReportResource_NoIdentity(t *testing.T) {
	protoReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "my-host-123",
				ApiHref:         "https://api.example.com/hosts/my-host-123",
			},
			Common:   &structpb.Struct{},
			Reporter: &structpb.Struct{},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &DenyAuthenticator{},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res, requireError(codes.Unauthenticated))
			}
	})
}

func TestInventoryService_DeleteResource_Success(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	reportReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "host-to-delete",
				ApiHref:         "https://api.example.com/hosts/host-to-delete",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue("delete-me.example.com"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"reporter_field": structpb.NewStringValue("reporter-value"),
				},
			},
		},
	}
	instanceID := "instance-001"
	deleteReq := &pb.DeleteResourceRequest{
		Reference: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-to-delete",
			Reporter: &pb.ReporterReference{
				Type:       "hbi",
				InstanceId: &instanceID,
			},
		},
	}
	uc := newTestUsecase(t, testUsecaseConfig{})

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res1 := tr.Invoke(ctx, withBody(reportReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res1, requireSuccess())
				res2 := tr.Invoke(ctx, withBody(deleteReq, DeleteResource, httpEndpoint("DELETE /api/kessel/v1beta2/resources")))
				Assert(t, res2, requireSuccess())
			}
	})
}

func TestInventoryService_StreamedListObjects_Success(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-abc"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	// Set up SimpleAuthorizer with tuples that grant subject-xyz view on two hosts
	simpleAuthz := authz.NewSimpleAuthorizer()
	simpleAuthz.Grant("subject-xyz", "view", "hbi", "host", "host-1")
	simpleAuthz.Grant("subject-xyz", "view", "hbi", "host", "host-2")

	uc := newTestUsecase(t, testUsecaseConfig{Authz: simpleAuthz})
	client := newTestServer(t, TestServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})

	reporterType := "hbi"
	req := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ReporterType: &reporterType,
			ResourceType: "host",
		},
		Relation: "view",
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-xyz",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	stream, err := client.StreamedListObjects(context.Background(), req)
	require.NoError(t, err)

	// Collect all streamed results
	var resourceIDs []string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		resourceIDs = append(resourceIDs, resp.Object.ResourceId)
	}

	// Should receive 2 resources
	assert.Len(t, resourceIDs, 2)
	assert.Contains(t, resourceIDs, "host-1")
	assert.Contains(t, resourceIDs, "host-2")
}

// --- Update Path Tests ---

func TestInventoryService_ReportResource_Update(t *testing.T) {
	// Test that reporting the same resource again triggers an update path
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	createReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "updatable-host",
				ApiHref:         "https://api.example.com/hosts/updatable-host",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue("original-hostname.example.com"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"version": structpb.NewStringValue("1.0"),
				},
			},
		},
	}
	updateReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "updatable-host", // Same key
				ApiHref:         "https://api.example.com/hosts/updatable-host",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue("updated-hostname.example.com"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"version": structpb.NewStringValue("2.0"),
				},
			},
		},
	}
	uc := newTestUsecase(t, testUsecaseConfig{})

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res1 := tr.Invoke(ctx, withBody(createReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res1, requireSuccess())
				res2 := tr.Invoke(ctx, withBody(updateReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res2, requireSuccess())
			}
	})
}

func TestInventoryService_ReportResource_Update_FieldsEffective(t *testing.T) {
	// Test that reporting the same resource again with different field values
	// actually persists the updated values.
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	originalConsoleHref := "https://console.example.com/hosts/original"
	createReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "update-effective-host",
				ApiHref:         "https://api.example.com/v1/hosts/original",
				ConsoleHref:     &originalConsoleHref,
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-original"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"version": structpb.NewStringValue("1.0"),
				},
			},
		},
	}
	updatedConsoleHref := "https://console.example.com/hosts/updated"
	updateReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "update-effective-host",
				ApiHref:         "https://api.example.com/v2/hosts/updated",
				ConsoleHref:     &updatedConsoleHref,
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-updated"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"version": structpb.NewStringValue("2.0"),
				},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		repo, db := newSQLiteTestRepo(t)
		uc := newTestUsecase(t, testUsecaseConfig{Repo: repo})
		key := buildReporterResourceKey(t, "update-effective-host", "host", "hbi", "instance-001")
		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res1 := tr.Invoke(ctx, withBody(createReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res1, requireSuccess())

				resource1, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err)
				require.NotNil(t, resource1)
				assert.Equal(t, "https://api.example.com/v1/hosts/original", resource1.ReporterResources()[0].ApiHref().String())
				assert.Equal(t, originalConsoleHref, resource1.ReporterResources()[0].ConsoleHref().String())

				reps1, err := repo.FindLatestRepresentations(db, key)
				require.NoError(t, err)
				assert.Equal(t, "ws-original", reps1.CommonData()["workspace_id"])

				res2 := tr.Invoke(ctx, withBody(updateReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res2, requireSuccess())

				resource2, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err)
				require.NotNil(t, resource2)
				rr := resource2.ReporterResources()[0]
				assert.Equal(t, "https://api.example.com/v2/hosts/updated", rr.ApiHref().String(),
					"api_href should reflect the updated value")
				assert.Equal(t, updatedConsoleHref, rr.ConsoleHref().String(),
					"console_href should reflect the updated value")
				reps2, err := repo.FindLatestRepresentations(db, key)
				require.NoError(t, err)
				assert.Equal(t, "ws-updated", reps2.CommonData()["workspace_id"],
					"common data should reflect the updated value")
			}
	})
}

func TestInventoryService_ReportResource_Update_FieldsEffective(t *testing.T) {
	// Test that reporting the same resource again with different field values
	// actually persists the updated values.
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	repo, db := newSQLiteTestRepo(t)
	uc := newTestUsecase(t, testUsecaseConfig{Repo: repo})
	client := newTestServer(t, testServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})

	originalConsoleHref := "https://console.example.com/hosts/original"

	// First report - create
	createReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "update-effective-host",
				ApiHref:         "https://api.example.com/v1/hosts/original",
				ConsoleHref:     &originalConsoleHref,
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-original"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"version": structpb.NewStringValue("1.0"),
				},
			},
		},
	}

	resp1, err := client.ReportResource(context.Background(), createReq)
	require.NoError(t, err)
	require.NotNil(t, resp1)

	key := buildReporterResourceKey(t, "update-effective-host", "host", "hbi", "instance-001")

	// Verify initial state
	resource1, err := repo.FindResourceByKeys(db, key)
	require.NoError(t, err)
	require.NotNil(t, resource1)
	assert.Equal(t, "https://api.example.com/v1/hosts/original", resource1.ReporterResources()[0].ApiHref().String())
	assert.Equal(t, originalConsoleHref, resource1.ReporterResources()[0].ConsoleHref().String())

	reps1, err := repo.FindLatestRepresentations(db, key)
	require.NoError(t, err)
	assert.Equal(t, "ws-original", reps1.CommonData()["workspace_id"])

	// Second report - update with changed values
	updatedConsoleHref := "https://console.example.com/hosts/updated"
	updateReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "update-effective-host",
				ApiHref:         "https://api.example.com/v2/hosts/updated",
				ConsoleHref:     &updatedConsoleHref,
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-updated"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"version": structpb.NewStringValue("2.0"),
				},
			},
		},
	}

	resp2, err := client.ReportResource(context.Background(), updateReq)
	require.NoError(t, err)
	require.NotNil(t, resp2)

	// Verify updated state
	resource2, err := repo.FindResourceByKeys(db, key)
	require.NoError(t, err)
	require.NotNil(t, resource2)

	rr := resource2.ReporterResources()[0]
	assert.Equal(t, "https://api.example.com/v2/hosts/updated", rr.ApiHref().String(),
		"api_href should reflect the updated value")
	assert.Equal(t, updatedConsoleHref, rr.ConsoleHref().String(),
		"console_href should reflect the updated value")

	reps2, err := repo.FindLatestRepresentations(db, key)
	require.NoError(t, err)
	assert.Equal(t, "ws-updated", reps2.CommonData()["workspace_id"],
		"common data should reflect the updated value")
}

func TestInventoryService_ReportResource_Update_DifferentReporterInstance(t *testing.T) {
	// Test that reporting same local_resource_id from a different reporter instance
	// creates a new resource (different reporter key)
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	req1 := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "shared-local-id",
				ApiHref:         "https://api.example.com/hosts/shared-local-id",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue("host1.example.com"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"source": structpb.NewStringValue("instance-001"),
				},
			},
		},
	}
	req2 := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-002",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "shared-local-id", // Same local ID, different instance
				ApiHref:         "https://api.example.com/hosts/shared-local-id",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue("host2.example.com"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"source": structpb.NewStringValue("instance-002"),
				},
			},
		},
	}
	uc := newTestUsecase(t, testUsecaseConfig{})

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res1 := tr.Invoke(ctx, withBody(req1, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res1, requireSuccess())
				res2 := tr.Invoke(ctx, withBody(req2, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res2, requireSuccess())
			}
	})
}

// --- Consistency Token Tests ---
// Note: In the v1beta2 API, CheckBulkRequest and CheckSelfBulkRequest support consistency tokens.
// CheckRequest and CheckForUpdateRequest do not have consistency token fields.

func TestInventoryService_CheckBulk_ConsistencyToken(t *testing.T) {
	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		claims := &authnapi.Claims{
			SubjectId: authnapi.SubjectId("user-123"),
			AuthType:  authnapi.AuthTypeXRhIdentity,
		}

		simpleAuthz := authz.NewSimpleAuthorizer()
		// Grant both permissions at initial version -> v3
		simpleAuthz.Grant("subject-a", "view", "hbi", "host", "resource-1")
		simpleAuthz.Grant("subject-b", "edit", "hbi", "host", "resource-2")
		snapshotVersion := simpleAuthz.RetainCurrentSnapshot() // Retain at v3

		// Remove one permission -> v4
		namespace := "hbi"
		resourceType := "host"
		resourceID := "resource-1"
		relation := "view"
		subjectNamespace := "rbac"
		subjectType := "principal"
		subjectID := "subject-a"

		_, _ = simpleAuthz.DeleteTuples(context.Background(), &relationsV1beta1.DeleteTuplesRequest{
			Filter: &relationsV1beta1.RelationTupleFilter{
				ResourceNamespace: &namespace,
				ResourceType:      &resourceType,
				ResourceId:        &resourceID,
				Relation:          &relation,
				SubjectFilter: &relationsV1beta1.SubjectFilter{
					SubjectNamespace: &subjectNamespace,
					SubjectType:      &subjectType,
					SubjectId:        &subjectID,
				},
			},
		})
		currentVersion := simpleAuthz.Version()

		uc := newTestUsecase(t, testUsecaseConfig{Authz: simpleAuthz})
		cfg := TestServerConfig{
			Usecase:       uc,
			Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
		}

		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			// CheckBulk without token -> uses oldest available (snapshot at v3) -> both allowed
			reqNoToken := &pb.CheckBulkRequest{
				Items: []*pb.CheckBulkRequestItem{
					{
						Object: &pb.ResourceReference{
							ResourceId:   "resource-1",
							ResourceType: "host",
							Reporter:     &pb.ReporterReference{Type: "hbi"},
						},
						Subject: &pb.SubjectReference{
							Resource: &pb.ResourceReference{
								ResourceId:   "subject-a",
								ResourceType: "principal",
								Reporter:     &pb.ReporterReference{Type: "rbac"},
							},
						},
						Relation: "view",
					},
					{
						Object: &pb.ResourceReference{
							ResourceId:   "resource-2",
							ResourceType: "host",
							Reporter:     &pb.ReporterReference{Type: "hbi"},
						},
						Subject: &pb.SubjectReference{
							Resource: &pb.ResourceReference{
								ResourceId:   "subject-b",
								ResourceType: "principal",
								Reporter:     &pb.ReporterReference{Type: "rbac"},
							},
						},
						Relation: "edit",
					},
				},
			}
			res := tr.Invoke(ctx, withBody(reqNoToken, CheckBulk, httpEndpoint("POST /api/kessel/v1beta2/checkbulk")))
			resp := Extract(t, res, expectSuccess(func() *pb.CheckBulkResponse { return &pb.CheckBulkResponse{} }))
			assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed, "no token uses oldest (snapshot), first allowed")
			assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[1].GetItem().Allowed, "no token uses oldest (snapshot), second allowed")

			// CheckBulk with token >= current version -> uses current (v4) -> first denied, second allowed
			currentTokenStr := fmt.Sprintf("%d", currentVersion)
			reqWithCurrentToken := &pb.CheckBulkRequest{
				Consistency: &pb.Consistency{
					Requirement: &pb.Consistency_AtLeastAsFresh{
						AtLeastAsFresh: &pb.ConsistencyToken{
							Token: currentTokenStr,
						},
					},
				},
				Items: []*pb.CheckBulkRequestItem{
					{
						Object: &pb.ResourceReference{
							ResourceId:   "resource-1",
							ResourceType: "host",
							Reporter:     &pb.ReporterReference{Type: "hbi"},
						},
						Subject: &pb.SubjectReference{
							Resource: &pb.ResourceReference{
								ResourceId:   "subject-a",
								ResourceType: "principal",
								Reporter:     &pb.ReporterReference{Type: "rbac"},
							},
						},
						Relation: "view",
					},
					{
						Object: &pb.ResourceReference{
							ResourceId:   "resource-2",
							ResourceType: "host",
							Reporter:     &pb.ReporterReference{Type: "hbi"},
						},
						Subject: &pb.SubjectReference{
							Resource: &pb.ResourceReference{
								ResourceId:   "subject-b",
								ResourceType: "principal",
								Reporter:     &pb.ReporterReference{Type: "rbac"},
							},
						},
						Relation: "edit",
					},
				},
			}
			res = tr.Invoke(ctx, withBody(reqWithCurrentToken, CheckBulk, httpEndpoint("POST /api/kessel/v1beta2/checkbulk")))
			resp = Extract(t, res, expectSuccess(func() *pb.CheckBulkResponse { return &pb.CheckBulkResponse{} }))
			assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Pairs[0].GetItem().Allowed, "current token uses latest, first denied")
			assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[1].GetItem().Allowed, "current token uses latest, second still allowed")

			// CheckBulk with old token (snapshot version) -> uses snapshot -> both allowed
			tokenStr := fmt.Sprintf("%d", snapshotVersion)
			reqWithToken := &pb.CheckBulkRequest{
				Consistency: &pb.Consistency{
					Requirement: &pb.Consistency_AtLeastAsFresh{
						AtLeastAsFresh: &pb.ConsistencyToken{
							Token: tokenStr,
						},
					},
				},
				Items: []*pb.CheckBulkRequestItem{
					{
						Object: &pb.ResourceReference{
							ResourceId:   "resource-1",
							ResourceType: "host",
							Reporter:     &pb.ReporterReference{Type: "hbi"},
						},
						Subject: &pb.SubjectReference{
							Resource: &pb.ResourceReference{
								ResourceId:   "subject-a",
								ResourceType: "principal",
								Reporter:     &pb.ReporterReference{Type: "rbac"},
							},
						},
						Relation: "view",
					},
					{
						Object: &pb.ResourceReference{
							ResourceId:   "resource-2",
							ResourceType: "host",
							Reporter:     &pb.ReporterReference{Type: "hbi"},
						},
						Subject: &pb.SubjectReference{
							Resource: &pb.ResourceReference{
								ResourceId:   "subject-b",
								ResourceType: "principal",
								Reporter:     &pb.ReporterReference{Type: "rbac"},
							},
						},
						Relation: "edit",
					},
				},
			}
			res = tr.Invoke(ctx, withBody(reqWithToken, CheckBulk, httpEndpoint("POST /api/kessel/v1beta2/checkbulk")))
			resp = Extract(t, res, expectSuccess(func() *pb.CheckBulkResponse { return &pb.CheckBulkResponse{} }))
			assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed, "old token uses snapshot, first allowed")
			assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[1].GetItem().Allowed, "old token uses snapshot, second allowed")
		}
	})
}

func TestInventoryService_CheckSelfBulk_ConsistencyToken(t *testing.T) {
	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		// The testSelfSubjectStrategy converts claims.SubjectId to rbac/principal/{subjectId}
		// So we use subject IDs that match what we grant in the authorizer.
		claims := &authnapi.Claims{
			SubjectId: authnapi.SubjectId("subject-a"),
			AuthType:  authnapi.AuthTypeXRhIdentity,
		}

		simpleAuthz := authz.NewSimpleAuthorizer()
		// Grant permission at initial version -> v3
		// The self subject strategy maps "subject-a" to rbac/principal/subject-a
		simpleAuthz.Grant("subject-a", "view", "hbi", "host", "resource-1")
		simpleAuthz.Grant("subject-a", "edit", "hbi", "host", "resource-2")
		snapshotVersion := simpleAuthz.RetainCurrentSnapshot() // Retain at v3

		// Remove one permission -> v4
		namespace := "hbi"
		resourceType := "host"
		resourceID := "resource-1"
		relation := "view"
		subjectNamespace := "rbac"
		subjectType := "principal"
		subjectID := "subject-a"

		_, _ = simpleAuthz.DeleteTuples(context.Background(), &relationsV1beta1.DeleteTuplesRequest{
			Filter: &relationsV1beta1.RelationTupleFilter{
				ResourceNamespace: &namespace,
				ResourceType:      &resourceType,
				ResourceId:        &resourceID,
				Relation:          &relation,
				SubjectFilter: &relationsV1beta1.SubjectFilter{
					SubjectNamespace: &subjectNamespace,
					SubjectType:      &subjectType,
					SubjectId:        &subjectID,
				},
			},
		})
		currentVersion := simpleAuthz.Version()

		uc := newTestUsecase(t, testUsecaseConfig{Authz: simpleAuthz})
		cfg := TestServerConfig{
			Usecase:       uc,
			Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
		}

		return cfg, func(t *testing.T, tr *Transport) {
			ctx := context.Background()

			// CheckSelfBulk without token -> uses oldest available (snapshot at v3) -> both allowed
			reqNoToken := &pb.CheckSelfBulkRequest{
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
			res := tr.Invoke(ctx, withBody(reqNoToken, CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
			resp := Extract(t, res, expectSuccess(func() *pb.CheckSelfBulkResponse { return &pb.CheckSelfBulkResponse{} }))
			assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed, "no token uses oldest (snapshot), first allowed")
			assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[1].GetItem().Allowed, "no token uses oldest (snapshot), second allowed")

			// CheckSelfBulk with token >= current version -> uses current (v4) -> first denied, second allowed
			currentTokenStr := fmt.Sprintf("%d", currentVersion)
			reqWithCurrentToken := &pb.CheckSelfBulkRequest{
				Consistency: &pb.Consistency{
					Requirement: &pb.Consistency_AtLeastAsFresh{
						AtLeastAsFresh: &pb.ConsistencyToken{
							Token: currentTokenStr,
						},
					},
				},
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
			res = tr.Invoke(ctx, withBody(reqWithCurrentToken, CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
			resp = Extract(t, res, expectSuccess(func() *pb.CheckSelfBulkResponse { return &pb.CheckSelfBulkResponse{} }))
			assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Pairs[0].GetItem().Allowed, "current token uses latest, first denied")
			assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[1].GetItem().Allowed, "current token uses latest, second still allowed")

			// CheckSelfBulk with old token (snapshot version) -> uses snapshot -> both allowed
			tokenStr := fmt.Sprintf("%d", snapshotVersion)
			reqWithToken := &pb.CheckSelfBulkRequest{
				Consistency: &pb.Consistency{
					Requirement: &pb.Consistency_AtLeastAsFresh{
						AtLeastAsFresh: &pb.ConsistencyToken{
							Token: tokenStr,
						},
					},
				},
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
			res = tr.Invoke(ctx, withBody(reqWithToken, CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
			resp = Extract(t, res, expectSuccess(func() *pb.CheckSelfBulkResponse { return &pb.CheckSelfBulkResponse{} }))
			assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed, "old token uses snapshot, first allowed")
			assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[1].GetItem().Allowed, "old token uses snapshot, second allowed")
		}
	})
}

// Note: CheckForUpdateResponse in v1beta2 does not include a consistency token.

// --- ReporterInstanceId Tests ---

func TestInventoryService_DeleteResource_WithReporterInstanceId(t *testing.T) {
	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		claims := &authnapi.Claims{
			SubjectId: authnapi.SubjectId("reporter-service"),
			AuthType:  authnapi.AuthTypeXRhIdentity,
		}
		uc := newTestUsecase(t, testUsecaseConfig{})

		// First, report a resource with specific instance ID
		reportReq := &pb.ReportResourceRequest{
			Type:               "host",
			ReporterType:       "hbi",
			ReporterInstanceId: "instance-specific",
			Representations: &pb.ResourceRepresentations{
				Metadata: &pb.RepresentationMetadata{
					LocalResourceId: "host-with-instance",
					ApiHref:         "https://api.example.com/hosts/host-with-instance",
				},
				Common: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"hostname": structpb.NewStringValue("test-host.example.com"),
					},
				},
				Reporter: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"source": structpb.NewStringValue("instance-specific"),
					},
				},
			},
		}

		// Delete with matching instance ID
		instanceID := "instance-specific"
		deleteReq := &pb.DeleteResourceRequest{
			Reference: &pb.ResourceReference{
				ResourceType: "host",
				ResourceId:   "host-with-instance",
				Reporter: &pb.ReporterReference{
					Type:       "hbi",
					InstanceId: &instanceID,
				},
			},
		}

		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res1 := tr.Invoke(ctx, withBody(reportReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res1, requireSuccess())
				res2 := tr.Invoke(ctx, withBody(deleteReq, DeleteResource, httpEndpoint("DELETE /api/kessel/v1beta2/resources")))
				Assert(t, res2, requireSuccess())
			}
	})
}

func TestInventoryService_DeleteResource_WithoutReporterInstanceId(t *testing.T) {
	// Test that deleting without reporter instance ID uses empty string for instance,
	// which creates a different reporter key than the original resource.
	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		claims := &authnapi.Claims{
			SubjectId: authnapi.SubjectId("reporter-service"),
			AuthType:  authnapi.AuthTypeXRhIdentity,
		}
		uc := newTestUsecase(t, testUsecaseConfig{})

		// First, report a resource with specific instance ID
		reportReq := &pb.ReportResourceRequest{
			Type:               "host",
			ReporterType:       "hbi",
			ReporterInstanceId: "instance-for-delete",
			Representations: &pb.ResourceRepresentations{
				Metadata: &pb.RepresentationMetadata{
					LocalResourceId: "host-for-delete-test",
					ApiHref:         "https://api.example.com/hosts/host-for-delete-test",
				},
				Common: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"hostname": structpb.NewStringValue("delete-test.example.com"),
					},
				},
				Reporter: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"source": structpb.NewStringValue("instance-for-delete"),
					},
				},
			},
		}

		// Delete without instance ID (nil)
		// FIXME: THIS IS BROKEN.
		// Note: The current implementation uses empty string for instance ID when nil is provided,
		// which creates a different reporter key. However, the FakeResourceRepository
		// implementation seems to match by local_resource_id alone, allowing the delete.
		// In a real implementation with a proper database, this might behave differently.
		deleteReq := &pb.DeleteResourceRequest{
			Reference: &pb.ResourceReference{
				ResourceType: "host",
				ResourceId:   "host-for-delete-test",
				Reporter: &pb.ReporterReference{
					Type:       "hbi",
					InstanceId: nil, // No instance ID
				},
			},
		}

		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res1 := tr.Invoke(ctx, withBody(reportReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res1, requireSuccess())
				res2 := tr.Invoke(ctx, withBody(deleteReq, DeleteResource, httpEndpoint("DELETE /api/kessel/v1beta2/resources")))
				// The delete succeeds with the FakeResourceRepository, which matches on local_resource_id
				Assert(t, res2, requireSuccess())
			}
	})
}

// --- Check with InstanceId variations ---

func TestInventoryService_Check_ReporterWithInstanceId(t *testing.T) {
	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		claims := &authnapi.Claims{
			SubjectId: authnapi.SubjectId("user-123"),
			AuthType:  authnapi.AuthTypeXRhIdentity,
		}
		instanceId := "instance-001"
		protoReq := &pb.CheckRequest{
			Relation: "view",
			Object: &pb.ResourceReference{
				ResourceId:   "resource-with-instance",
				ResourceType: "host",
				Reporter: &pb.ReporterReference{
					Type:       "hbi",
					InstanceId: &instanceId,
				},
			},
			Subject: &pb.SubjectReference{
				Resource: &pb.ResourceReference{
					ResourceId:   "subject-456",
					ResourceType: "principal",
					Reporter:     &pb.ReporterReference{Type: "rbac"},
				},
			},
		}
		simpleAuthz := authz.NewSimpleAuthorizer()
		simpleAuthz.Grant("subject-456", "view", "hbi", "host", "resource-with-instance")
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Authz: simpleAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, Check, httpEndpoint("POST /api/kessel/v1beta2/check")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckResponse { return &pb.CheckResponse{} }))
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Allowed)
			}
	})
}

// --- StreamedListObjects with NoIdentity ---

func TestInventoryService_StreamedListObjects_NoIdentity(t *testing.T) {
	simpleAuthz := authz.NewSimpleAuthorizer()
	simpleAuthz.Grant("subject-xyz", "view", "hbi", "host", "host-1")

	uc := newTestUsecase(t, testUsecaseConfig{Authz: simpleAuthz})
	// Use DenyAuthenticator to simulate unauthenticated streaming request
	client := newTestServer(t, TestServerConfig{
		Usecase:       uc,
		Authenticator: &DenyAuthenticator{},
	})

	reporterType := "hbi"
	req := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ReporterType: &reporterType,
			ResourceType: "host",
		},
		Relation: "view",
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-xyz",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	stream, err := client.StreamedListObjects(context.Background(), req)
	require.NoError(t, err) // Stream creation succeeds

	// First Recv should return the auth error
	_, err = stream.Recv()
	assert.Error(t, err)
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, grpcStatus.Code())
}

func TestInventoryService_StreamedListObjects_MetaAuthzDenied(t *testing.T) {
	// Test that meta-authorization denial is properly mapped to PermissionDenied
	// for streaming RPCs.
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	reporterType := "hbi"
	req := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ReporterType: &reporterType,
			ResourceType: "host",
		},
		Relation: "view",
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-xyz",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	uc := newTestUsecase(t, testUsecaseConfig{
		MetaAuthorizer: &DenyingMetaAuthorizer{},
	})
	client := newTestServer(t, TestServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})

	stream, err := client.StreamedListObjects(context.Background(), req)
	require.NoError(t, err) // Stream creation succeeds

	// First Recv should return the mapped error
	_, err = stream.Recv()
	assert.Error(t, err)
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, grpcStatus.Code())
}

// --- CheckForUpdate with NoIdentity ---

func TestInventoryService_CheckForUpdate_NoIdentity(t *testing.T) {
	protoReq := &pb.CheckForUpdateRequest{
		Relation: "edit",
		Object: &pb.ResourceReference{
			ResourceId:   "resource-xyz",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-789",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &DenyAuthenticator{},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdate, httpEndpoint("POST /api/kessel/v1beta2/checkforupdate")))
				Assert(t, res, requireError(codes.Unauthenticated))
			}
	})
}

// --- CheckBulk with NoIdentity ---

func TestInventoryService_CheckBulk_NoIdentity(t *testing.T) {
	protoReq := &pb.CheckBulkRequest{
		Items: []*pb.CheckBulkRequestItem{
			{
				Object: &pb.ResourceReference{
					ResourceId:   "resource-1",
					ResourceType: "host",
					Reporter:     &pb.ReporterReference{Type: "hbi"},
				},
				Subject: &pb.SubjectReference{
					Resource: &pb.ResourceReference{
						ResourceId:   "subject-a",
						ResourceType: "principal",
						Reporter:     &pb.ReporterReference{Type: "rbac"},
					},
				},
				Relation: "view",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &DenyAuthenticator{},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckBulk, httpEndpoint("POST /api/kessel/v1beta2/checkbulk")))
				Assert(t, res, requireError(codes.Unauthenticated))
			}
	})
}

// =============================================================================
// ERROR SCENARIO TESTS
// =============================================================================
// These tests document the current error handling behavior.

// --- DeleteResource Error Scenarios ---

func TestInventoryService_DeleteResource_ResourceNotFound(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	instanceID := "instance-001"
	protoReq := &pb.DeleteResourceRequest{
		Reference: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "nonexistent-resource",
			Reporter: &pb.ReporterReference{
				Type:       "hbi",
				InstanceId: &instanceID,
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, DeleteResource, httpEndpoint("DELETE /api/kessel/v1beta2/resources")))
				Assert(t, res, requireError(codes.NotFound))
			}
	})
}

func TestInventoryService_DeleteResource_InvalidReference_EmptyResourceId(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	instanceID := "instance-001"
	protoReq := &pb.DeleteResourceRequest{
		Reference: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "",
			Reporter: &pb.ReporterReference{
				Type:       "hbi",
				InstanceId: &instanceID,
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, DeleteResource, httpEndpoint("DELETE /api/kessel/v1beta2/resources")))
				Assert(t, res, requireError(codes.InvalidArgument))
			}
	})
}

func TestInventoryService_DeleteResource_InvalidReference_EmptyResourceType(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	instanceID := "instance-001"
	protoReq := &pb.DeleteResourceRequest{
		Reference: &pb.ResourceReference{
			ResourceType: "",
			ResourceId:   "resource-123",
			Reporter: &pb.ReporterReference{
				Type:       "hbi",
				InstanceId: &instanceID,
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, DeleteResource, httpEndpoint("DELETE /api/kessel/v1beta2/resources")))
				Assert(t, res, requireError(codes.InvalidArgument))
			}
	})
}

func TestInventoryService_DeleteResource_InvalidReference_EmptyReporterType(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	instanceID := "instance-001"
	protoReq := &pb.DeleteResourceRequest{
		Reference: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "resource-123",
			Reporter: &pb.ReporterReference{
				Type:       "",
				InstanceId: &instanceID,
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, DeleteResource, httpEndpoint("DELETE /api/kessel/v1beta2/resources")))
				Assert(t, res, requireError(codes.InvalidArgument))
			}
	})
}

// --- Check Error Scenarios ---

func TestInventoryService_Check_InvalidReference_EmptyResourceId(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-456",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, Check, httpEndpoint("POST /api/kessel/v1beta2/check")))
				Assert(t, res, requireError(codes.InvalidArgument))
			}
	})
}

// --- CheckForUpdate Error Scenarios ---

func TestInventoryService_CheckForUpdate_InvalidReference_EmptyResourceId(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckForUpdateRequest{
		Relation: "edit",
		Object: &pb.ResourceReference{
			ResourceId:   "",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "subject-789",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdate, httpEndpoint("POST /api/kessel/v1beta2/checkforupdate")))
				Assert(t, res, requireError(codes.InvalidArgument))
			}
	})
}

// --- CheckSelf Error Scenarios ---

func TestInventoryService_CheckSelf_InvalidReference_EmptyResourceId(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelf, httpEndpoint("POST /api/kessel/v1beta2/checkself")))
				Assert(t, res, requireError(codes.InvalidArgument))
			}
	})
}

func TestInventoryService_CheckSelf_MetaAuthzDenied(t *testing.T) {
	// SimpleMetaAuthorizer denies check_self on gRPC regardless of auth type,
	// and denies OIDC on HTTP. So OIDC + check_self is denied on both protocols.
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeOIDC,
	}

	protoReq := &pb.CheckSelfRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "resource-123",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{MetaAuthorizer: metaauthorizer.NewSimpleMetaAuthorizer()}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelf, httpEndpoint("POST /api/kessel/v1beta2/checkself")))
				Assert(t, res, requireError(codes.PermissionDenied))
			}
	})
}

// --- CheckSelfBulk Error Scenarios ---

func TestInventoryService_CheckSelfBulk_EmptyItems(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckSelfBulkRequest{
		Items: []*pb.CheckSelfBulkRequestItem{},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
				Assert(t, res, requireErrorContaining(codes.InvalidArgument, "items"))
			}
	})
}

func TestInventoryService_CheckSelfBulk_MetaAuthzDenied(t *testing.T) {
	// SimpleMetaAuthorizer denies check_self_bulk on gRPC (it's "check_self" relation),
	// and denies OIDC on HTTP. So OIDC + check_self_bulk is denied on both protocols.
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeOIDC,
	}

	protoReq := &pb.CheckSelfBulkRequest{
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

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{MetaAuthorizer: metaauthorizer.NewSimpleMetaAuthorizer()}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
				Assert(t, res, requireError(codes.PermissionDenied))
			}
	})
}

// --- CheckBulk Error Scenarios ---

func TestInventoryService_CheckBulk_MetaAuthzProtocolBehavior(t *testing.T) {
	// SimpleMetaAuthorizer protocol-aware behavior:
	// - gRPC: allow ALL relations EXCEPT "check_self" -> CheckBulk ALLOWED
	// - HTTP + OIDC: deny (only x-rh-identity + check_self is allowed) -> PermissionDenied
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeOIDC,
	}

	protoReq := &pb.CheckBulkRequest{
		Items: []*pb.CheckBulkRequestItem{
			{
				Object: &pb.ResourceReference{
					ResourceId:   "resource-1",
					ResourceType: "host",
					Reporter:     &pb.ReporterReference{Type: "hbi"},
				},
				Subject: &pb.SubjectReference{
					Resource: &pb.ResourceReference{
						ResourceId:   "subject-a",
						ResourceType: "principal",
						Reporter:     &pb.ReporterReference{Type: "rbac"},
					},
				},
				Relation: "view",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		simpleAuthz := authz.NewSimpleAuthorizer()
		simpleAuthz.Grant("subject-a", "view", "hbi", "host", "resource-1")
		return TestServerConfig{
				Usecase: newTestUsecase(t, testUsecaseConfig{
					Authz:          simpleAuthz,
					MetaAuthorizer: metaauthorizer.NewSimpleMetaAuthorizer(),
				}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckBulk, httpEndpoint("POST /api/kessel/v1beta2/checkbulk")))
				Assert(t, res, Expectation{
					GRPC: func(t *testing.T, resp proto.Message, err error) {
						require.NoError(t, err)
						r := resp.(*pb.CheckBulkResponse)
						assert.Equal(t, pb.Allowed_ALLOWED_TRUE, r.Pairs[0].GetItem().Allowed)
					},
					HTTP: func(t *testing.T, statusCode int, _ []byte) {
						assert.Equal(t, 403, statusCode)
					},
				})
			}
	})
}

func TestInventoryService_CheckBulk_MetaAuthzDenied(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckBulkRequest{
		Items: []*pb.CheckBulkRequestItem{
			{
				Object: &pb.ResourceReference{
					ResourceId:   "resource-1",
					ResourceType: "host",
					Reporter:     &pb.ReporterReference{Type: "hbi"},
				},
				Subject: &pb.SubjectReference{
					Resource: &pb.ResourceReference{
						ResourceId:   "subject-a",
						ResourceType: "principal",
						Reporter:     &pb.ReporterReference{Type: "rbac"},
					},
				},
				Relation: "view",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{MetaAuthorizer: &DenyingMetaAuthorizer{}}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckBulk, httpEndpoint("POST /api/kessel/v1beta2/checkbulk")))
				Assert(t, res, requireError(codes.PermissionDenied))
			}
	})
}

func TestInventoryService_ReportResource_MetaAuthzProtocolBehavior(t *testing.T) {
	// SimpleMetaAuthorizer protocol-aware behavior:
	// - gRPC: allow ALL relations EXCEPT "check_self" -> ReportResource ALLOWED
	// - HTTP + OIDC: deny (only x-rh-identity + check_self is allowed) -> PermissionDenied
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeOIDC,
	}

	protoReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "my-host-123",
				ApiHref:         "https://api.example.com/hosts/my-host-123",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue("example.com"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"field": structpb.NewStringValue("value"),
				},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase: newTestUsecase(t, testUsecaseConfig{
					MetaAuthorizer: metaauthorizer.NewSimpleMetaAuthorizer(),
				}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res, Expectation{
					GRPC: func(t *testing.T, resp proto.Message, err error) {
						require.NoError(t, err)
						assert.NotNil(t, resp)
					},
					HTTP: func(t *testing.T, statusCode int, _ []byte) {
						assert.Equal(t, 403, statusCode)
					},
				})
			}
	})
}

func TestInventoryService_StreamedListObjects_NilRequest(t *testing.T) {
	// ToLookupResourcesCommand returns error for nil request
	// This is validated by protovalidate before reaching the handler,
	// so this tests the internal function behavior only.
	_, err := svc.ToLookupResourcesCommand(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")
}

// newSQLiteTestRepo creates a real GORM repository backed by an in-memory SQLite
// database with all migrations applied. Returns the repository and the underlying
// *gorm.DB for use in assertions.
func newSQLiteTestRepo(t *testing.T) (data.ResourceRepository, *gorm.DB) {
	t.Helper()
	db := testutil.NewSQLiteTestDB(t, &gorm.Config{TranslateError: true})
	err := data.Migrate(db, nil)
	require.NoError(t, err)
	mc := metricscollector.NewFakeMetricsCollector()
	tm := data.NewGormTransactionManager(mc, 3)
	repo := data.NewResourceRepository(db, tm)
	return repo, db
}

// buildReporterResourceKey is a test helper that constructs a model.ReporterResourceKey
// from plain string values matching a ReportResourceRequest.
func buildReporterResourceKey(t *testing.T, localResourceId, resourceType, reporterType, reporterInstanceId string) model.ReporterResourceKey {
	t.Helper()
	lid, err := model.NewLocalResourceId(localResourceId)
	require.NoError(t, err)
	rt, err := model.NewResourceType(resourceType)
	require.NoError(t, err)
	rpt, err := model.NewReporterType(reporterType)
	require.NoError(t, err)
	rid, err := model.NewReporterInstanceId(reporterInstanceId)
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(lid, rt, rpt, rid)
	require.NoError(t, err)
	return key
}

// --- ReportResource: Optional Metadata Fields ---

func TestInventoryService_ReportResource_AllOptionalMetadataFields(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	txId := "txn-all-optional-metadata"
	consoleHref := "https://console.example.com/hosts/my-host-123"
	reporterVersion := "1.2.3"

	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "host-all-optional",
				ApiHref:         "https://api.example.com/hosts/host-all-optional",
				ConsoleHref:     &consoleHref,
				ReporterVersion: &reporterVersion,
				IdempotencyKey: &pb.RepresentationMetadata_TransactionId{
					TransactionId: txId,
				},
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-all-optional"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"reporter_field": structpb.NewStringValue("reporter-value"),
				},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		repo, db := newSQLiteTestRepo(t)
		uc := newTestUsecase(t, testUsecaseConfig{Repo: repo})
		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(req, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res, requireSuccess())

				key := buildReporterResourceKey(t, "host-all-optional", "host", "hbi", "instance-001")
				resource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err)
				require.NotNil(t, resource)

				rr := resource.ReporterResources()[0]
				assert.Equal(t, "host-all-optional", rr.Key().LocalResourceId().String())
				assert.Equal(t, "host", rr.Key().ResourceType().String())
				assert.Equal(t, "hbi", rr.Key().ReporterType().String())
				assert.Equal(t, "instance-001", rr.Key().ReporterInstanceId().String())
				assert.Equal(t, "https://api.example.com/hosts/host-all-optional", rr.ApiHref().String())
				assert.Equal(t, consoleHref, rr.ConsoleHref().String())

				reps, err := repo.FindLatestRepresentations(db, key)
				require.NoError(t, err)
				require.NotNil(t, reps)
				assert.Equal(t, "ws-all-optional", string(reps.CommonData()["workspace_id"].(string)))

				processed, err := repo.HasTransactionIdBeenProcessed(db, txId)
				require.NoError(t, err)
				assert.True(t, processed, "transaction_id should be recorded as processed")
			}
	})
}

// --- ReportResource: Nil/Empty Optional Struct Combinations ---

// The model layer requires both common and reporter representation data to be
// non-empty. Sending nil or empty structs produces an error. These tests verify
// the error paths.

// TODO: This is actually not correct behavior.
// These should be optional, and it depends on schema.
func TestInventoryService_ReportResource_NilOrEmptyRepresentationStructs(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	cases := []struct {
		name            string
		localResourceId string
		common          *structpb.Struct
		reporter        *structpb.Struct
		expectMsg       string
	}{
		{
			name:            "both nil",
			localResourceId: "host-both-nil",
			common:          nil,
			reporter:        nil,
			expectMsg:       "invalid reporter representation: representation required",
		},
		{
			name:            "common set, reporter nil",
			localResourceId: "host-common-only",
			common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-common-only"),
				},
			},
			reporter:  nil,
			expectMsg: "invalid reporter representation: representation required",
		},
		{
			name:            "common nil, reporter set",
			localResourceId: "host-reporter-only",
			common:          nil,
			reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"reporter_field": structpb.NewStringValue("val"),
				},
			},
			expectMsg: "invalid common representation: representation required",
		},
		{
			name:            "both empty structs",
			localResourceId: "host-both-empty",
			common:          &structpb.Struct{},
			reporter:        &structpb.Struct{},
			expectMsg:       "invalid reporter representation: representation data cannot be empty",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
				repo, _ := newSQLiteTestRepo(t)
				uc := newTestUsecase(t, testUsecaseConfig{Repo: repo})
				req := &pb.ReportResourceRequest{
					Type:               "host",
					ReporterType:       "hbi",
					ReporterInstanceId: "instance-001",
					Representations: &pb.ResourceRepresentations{
						Metadata: &pb.RepresentationMetadata{
							LocalResourceId: tc.localResourceId,
							ApiHref:         "https://api.example.com/hosts/" + tc.localResourceId,
						},
						Common:   tc.common,
						Reporter: tc.reporter,
					},
				}
				return TestServerConfig{
						Usecase:       uc,
						Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
					}, func(t *testing.T, tr *Transport) {
						ctx := context.Background()
						res := tr.Invoke(ctx, withBody(req, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res, requireErrorContaining(codes.InvalidArgument, tc.expectMsg))
					}
			})
		})
	}
}

// --- ReportResource: Error Message Format Tests ---
// These tests verify the exact error message format returned by the service layer.
// They serve as a contract for API consumers and must be updated if error formats change.

// TestInventoryService_ReportResource_ErrorFormats tests error messages
// from the protovalidate middleware, which catches empty/invalid fields before they
// reach the service layer.
func TestInventoryService_ReportResource_ErrorFormats(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	validCommon := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"workspace_id": structpb.NewStringValue("ws-123"),
		},
	}
	validReporter := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"satellite_id": structpb.NewStringValue("sat-123"),
		},
	}

	cases := []struct {
		name              string
		resourceType      string
		reporterType      string
		reporterInstance  string
		localResourceId   string
		apiHref           string
		expectCode        codes.Code
		expectMsgContains string
	}{
		{
			name:              "empty local_resource_id",
			resourceType:      "host",
			reporterType:      "hbi",
			reporterInstance:  "instance-001",
			localResourceId:   "",
			apiHref:           "https://api.example.com/hosts/test",
			expectCode:        codes.InvalidArgument,
			expectMsgContains: "local_resource_id",
		},
		{
			name:              "empty resource_type",
			resourceType:      "",
			reporterType:      "hbi",
			reporterInstance:  "instance-001",
			localResourceId:   "test-host",
			apiHref:           "https://api.example.com/hosts/test",
			expectCode:        codes.InvalidArgument,
			expectMsgContains: "type",
		},
		{
			name:              "empty reporter_type",
			resourceType:      "host",
			reporterType:      "",
			reporterInstance:  "instance-001",
			localResourceId:   "test-host",
			apiHref:           "https://api.example.com/hosts/test",
			expectCode:        codes.InvalidArgument,
			expectMsgContains: "reporter_type",
		},
		{
			name:              "empty reporter_instance_id",
			resourceType:      "host",
			reporterType:      "hbi",
			reporterInstance:  "",
			localResourceId:   "test-host",
			apiHref:           "https://api.example.com/hosts/test",
			expectCode:        codes.InvalidArgument,
			expectMsgContains: "reporter_instance_id",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
				uc := newTestUsecase(t, testUsecaseConfig{})
				req := &pb.ReportResourceRequest{
					Type:               tc.resourceType,
					ReporterType:       tc.reporterType,
					ReporterInstanceId: tc.reporterInstance,
					Representations: &pb.ResourceRepresentations{
						Metadata: &pb.RepresentationMetadata{
							LocalResourceId: tc.localResourceId,
							ApiHref:         tc.apiHref,
						},
						Common:   validCommon,
						Reporter: validReporter,
					},
				}
				return TestServerConfig{
						Usecase:       uc,
						Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
					}, func(t *testing.T, tr *Transport) {
						ctx := context.Background()
						res := tr.Invoke(ctx, withBody(req, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res, requireErrorContaining(tc.expectCode, tc.expectMsgContains))
					}
			})
		})
	}
}

func TestInventoryService_ReportResource_ValidationErrorFormats(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	cases := []struct {
		name              string
		resourceType      string
		reporterType      string
		common            *structpb.Struct
		reporter          *structpb.Struct
		expectCode        codes.Code
		expectMsgContains string
	}{
		{
			name:         "unknown reporter for resource type",
			resourceType: "host",
			reporterType: "unknown_reporter",
			common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-123"),
				},
			},
			reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"data": structpb.NewStringValue("value"),
				},
			},
			expectCode:        codes.InvalidArgument,
			expectMsgContains: "failed validation for report resource",
		},
		{
			name:         "nil reporter representation",
			resourceType: "host",
			reporterType: "hbi",
			common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-123"),
				},
			},
			reporter:          nil,
			expectCode:        codes.InvalidArgument,
			expectMsgContains: "invalid reporter representation: representation required",
		},
		{
			name:         "nil common representation",
			resourceType: "host",
			reporterType: "hbi",
			common:       nil,
			reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"satellite_id": structpb.NewStringValue("sat-123"),
				},
			},
			expectCode:        codes.InvalidArgument,
			expectMsgContains: "invalid common representation: representation required",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
				repo, _ := newSQLiteTestRepo(t)
				uc := newTestUsecase(t, testUsecaseConfig{Repo: repo})
				req := &pb.ReportResourceRequest{
					Type:               tc.resourceType,
					ReporterType:       tc.reporterType,
					ReporterInstanceId: "instance-001",
					Representations: &pb.ResourceRepresentations{
						Metadata: &pb.RepresentationMetadata{
							LocalResourceId: "test-host",
							ApiHref:         "https://api.example.com/hosts/test",
						},
						Common:   tc.common,
						Reporter: tc.reporter,
					},
				}
				return TestServerConfig{
						Usecase:       uc,
						Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
					}, func(t *testing.T, tr *Transport) {
						ctx := context.Background()
						res := tr.Invoke(ctx, withBody(req, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res, requireErrorContaining(tc.expectCode, tc.expectMsgContains))
					}
			})
		})
	}
}

// --- ReportResource: WriteVisibility Variations ---

func TestInventoryService_ReportResource_WriteVisibility(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	cases := []struct {
		name            string
		visibility      pb.WriteVisibility
		localResourceId string
	}{
		{
			name:            "UNSPECIFIED",
			visibility:      pb.WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED,
			localResourceId: "host-vis-unspecified",
		},
		{
			name:            "MINIMIZE_LATENCY",
			visibility:      pb.WriteVisibility_MINIMIZE_LATENCY,
			localResourceId: "host-vis-minimize",
		},
		{
			name:            "IMMEDIATE",
			visibility:      pb.WriteVisibility_IMMEDIATE,
			localResourceId: "host-vis-immediate",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
				repo, db := newSQLiteTestRepo(t)
				uc := newTestUsecase(t, testUsecaseConfig{Repo: repo})
				req := &pb.ReportResourceRequest{
					Type:               "host",
					ReporterType:       "hbi",
					ReporterInstanceId: "instance-001",
					WriteVisibility:    tc.visibility,
					Representations: &pb.ResourceRepresentations{
						Metadata: &pb.RepresentationMetadata{
							LocalResourceId: tc.localResourceId,
							ApiHref:         "https://api.example.com/hosts/" + tc.localResourceId,
						},
						Common: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"workspace_id": structpb.NewStringValue("ws-vis"),
							},
						},
						Reporter: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"reporter_key": structpb.NewStringValue("reporter-val"),
							},
						},
					},
				}
				return TestServerConfig{
						Usecase:       uc,
						Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
					}, func(t *testing.T, tr *Transport) {
						ctx := context.Background()
						res := tr.Invoke(ctx, withBody(req, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res, requireSuccess())

						key := buildReporterResourceKey(t, tc.localResourceId, "host", "hbi", "instance-001")
						resource, err := repo.FindResourceByKeys(db, key)
						require.NoError(t, err)
						require.NotNil(t, resource, "resource should be persisted regardless of write_visibility")
					}
			})
		})
	}
}

// --- ReportResource: inventory_id Set (Regression Guard) ---

func TestInventoryService_ReportResource_InventoryIdSet(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	inventoryId := "inv-12345"
	req := &pb.ReportResourceRequest{
		InventoryId:        &inventoryId,
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "host-with-inventory-id",
				ApiHref:         "https://api.example.com/hosts/host-with-inventory-id",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-inv-id"),
				},
			},
			Reporter: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"reporter_key": structpb.NewStringValue("reporter-val"),
				},
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		repo, db := newSQLiteTestRepo(t)
		uc := newTestUsecase(t, testUsecaseConfig{Repo: repo})
		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(req, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res, requireSuccess())

				key := buildReporterResourceKey(t, "host-with-inventory-id", "host", "hbi", "instance-001")
				resource, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err)
				require.NotNil(t, resource, "inventory_id should not interfere with persistence")
			}
	})
}

// --- ReportResource: Missing Required Fields (Validation) ---

func TestInventoryService_ReportResource_MissingRequiredFields(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	validReq := func() *pb.ReportResourceRequest {
		return &pb.ReportResourceRequest{
			Type:               "host",
			ReporterType:       "hbi",
			ReporterInstanceId: "instance-001",
			Representations: &pb.ResourceRepresentations{
				Metadata: &pb.RepresentationMetadata{
					LocalResourceId: "valid-host",
					ApiHref:         "https://api.example.com/hosts/valid-host",
				},
				Common:   &structpb.Struct{},
				Reporter: &structpb.Struct{},
			},
		}
	}

	cases := []struct {
		name      string
		mutate    func(r *pb.ReportResourceRequest)
		expectMsg string
	}{
		{
			name: "missing type",
			mutate: func(r *pb.ReportResourceRequest) {
				r.Type = ""
			},
			expectMsg: "type: value length must be at least 1 characters",
		},
		{
			name: "invalid type pattern",
			mutate: func(r *pb.ReportResourceRequest) {
				r.Type = "host!@#"
			},
			expectMsg: "type: value does not match regex pattern `^[A-Za-z0-9_-]+$`",
		},
		{
			name: "missing representations",
			mutate: func(r *pb.ReportResourceRequest) {
				r.Representations = nil
			},
			expectMsg: "representations: value is required",
		},
		{
			name: "missing metadata",
			mutate: func(r *pb.ReportResourceRequest) {
				r.Representations.Metadata = nil
			},
			expectMsg: "representations.metadata: value is required",
		},
		{
			name: "missing local_resource_id",
			mutate: func(r *pb.ReportResourceRequest) {
				r.Representations.Metadata.LocalResourceId = ""
			},
			expectMsg: "representations.metadata.local_resource_id: value length must be at least 1 characters",
		},
		{
			name: "missing api_href",
			mutate: func(r *pb.ReportResourceRequest) {
				r.Representations.Metadata.ApiHref = ""
			},
			expectMsg: "representations.metadata.api_href: value length must be at least 1 characters",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
				uc := newTestUsecase(t, testUsecaseConfig{})
				req := validReq()
				tc.mutate(req)
				return TestServerConfig{
						Usecase:       uc,
						Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
					}, func(t *testing.T, tr *Transport) {
						ctx := context.Background()
						res := tr.Invoke(ctx, withBody(req, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res, requireErrorContaining(codes.InvalidArgument, tc.expectMsg))
					}
			})
		})
	}
}

// --- ReportResource: Transaction ID Idempotency ---

func TestInventoryService_ReportResource_TransactionIdIdempotency(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	txId := "txn-idempotency-test"
	makeReq := func(apiHref string) *pb.ReportResourceRequest {
		return &pb.ReportResourceRequest{
			Type:               "host",
			ReporterType:       "hbi",
			ReporterInstanceId: "instance-001",
			Representations: &pb.ResourceRepresentations{
				Metadata: &pb.RepresentationMetadata{
					LocalResourceId: "host-idempotent",
					ApiHref:         apiHref,
					IdempotencyKey: &pb.RepresentationMetadata_TransactionId{
						TransactionId: txId,
					},
				},
				Common: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"workspace_id": structpb.NewStringValue("ws-idempotent"),
					},
				},
				Reporter: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"reporter_key": structpb.NewStringValue("reporter-val"),
					},
				},
			},
		}
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		repo, db := newSQLiteTestRepo(t)
		uc := newTestUsecase(t, testUsecaseConfig{Repo: repo})
		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				key := buildReporterResourceKey(t, "host-idempotent", "host", "hbi", "instance-001")

				// First report
				res1 := tr.Invoke(ctx, withBody(makeReq("https://api.example.com/v1"), ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res1, requireSuccess())

				processed, err := repo.HasTransactionIdBeenProcessed(db, txId)
				require.NoError(t, err)
				assert.True(t, processed, "transaction_id should be recorded after first report")

				resource1, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err)
				require.NotNil(t, resource1)
				apiHrefAfterFirst := resource1.ReporterResources()[0].ApiHref().String()

				// Second report with same transaction_id but different api_href
				res2 := tr.Invoke(ctx, withBody(makeReq("https://api.example.com/v2-should-be-ignored"), ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res2, requireSuccess())

				resource2, err := repo.FindResourceByKeys(db, key)
				require.NoError(t, err)
				require.NotNil(t, resource2)
				apiHrefAfterSecond := resource2.ReporterResources()[0].ApiHref().String()

				assert.Equal(t, apiHrefAfterFirst, apiHrefAfterSecond,
					"second report with same transaction_id should be a no-op; api_href should not change")
			}
	})
}

// --- ReportResource: MetaAuthorizer Denied ---

func TestInventoryService_ReportResource_MetaAuthzDenied(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "host-denied",
				ApiHref:         "https://api.example.com/hosts/host-denied",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{MetaAuthorizer: &DenyingMetaAuthorizer{}}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res, requireError(codes.PermissionDenied))
			}
	})
}

func newFakeSchemaRepository(t *testing.T) schema.Repository {
	schemaRepository := data.NewInMemorySchemaRepository()

	emptyValidationSchema := validation.NewJsonSchemaValidatorFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
		},
		"required": []
	}`)

	withWorkspaceValidationSchema := validation.NewJsonSchemaValidatorFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"workspace_id": { "type": "string" }
		}
	}`)

	err := schemaRepository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
		ResourceType:     "k8s_cluster",
		ValidationSchema: withWorkspaceValidationSchema,
	})
	assert.NoError(t, err)

	err = schemaRepository.CreateReporterSchema(context.Background(), schema.ReporterRepresentation{
		ResourceType:     "k8s_cluster",
		ReporterType:     "ocm",
		ValidationSchema: emptyValidationSchema,
	})
	assert.NoError(t, err)

	err = schemaRepository.CreateResourceSchema(context.Background(), schema.ResourceRepresentation{
		ResourceType:     "host",
		ValidationSchema: withWorkspaceValidationSchema,
	})
	assert.NoError(t, err)

	err = schemaRepository.CreateReporterSchema(context.Background(), schema.ReporterRepresentation{
		ResourceType:     "host",
		ReporterType:     "hbi",
		ValidationSchema: emptyValidationSchema,
	})
	assert.NoError(t, err)

	return schemaRepository
}
