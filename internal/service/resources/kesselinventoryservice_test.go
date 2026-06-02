package resources_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"testing"

	krlog "github.com/go-kratos/kratos/v2/log"
	kratosTransport "github.com/go-kratos/kratos/v2/transport"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	usecase "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/mocks"
	svc "github.com/project-kessel/inventory-api/internal/service/resources"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func testTupleFilterForHelper(namespace, resourceType, resourceID, relation, subjectNamespace, subjectType, subjectID string) model.TupleFilter {
	rn := model.DeserializeReporterType(namespace)
	ot := model.DeserializeResourceType(resourceType)
	oid := model.DeserializeLocalResourceId(resourceID)
	rel := model.DeserializeRelation(relation)
	subRN := model.DeserializeReporterType(subjectNamespace)
	st := model.DeserializeResourceType(subjectType)
	sid := model.DeserializeLocalResourceId(subjectID)
	return model.NewTupleFilter().
		WithReporterType(rn).
		WithObjectType(ot).
		WithObjectId(oid).
		WithRelation(rel).
		WithSubject(model.NewTupleSubjectFilter().
			WithReporterType(subRN).
			WithSubjectType(st).
			WithSubjectId(sid))
}

type testSelfSubjectStrategy struct{}

func (testSelfSubjectStrategy) SubjectFromAuthorizationContext(authzContext authnapi.AuthzContext) (model.SubjectReference, error) {
	if !authzContext.IsAuthenticated() || authzContext.Subject.SubjectId == "" {
		return model.SubjectReference{}, fmt.Errorf("subject claims not found")
	}
	subjectID := string(authzContext.Subject.SubjectId)
	return buildTestSubjectReference(subjectID)
}

func resourceRefFromKey(key model.ReporterResourceKey) model.ResourceReference {
	reporter := model.NewReporterReference(key.ReporterType(), nil)
	return model.NewResourceReference(key.ResourceType(), key.LocalResourceId(), &reporter)
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
	return model.NewSubjectReferenceWithoutRelation(resourceRefFromKey(key)), nil
}

func newTestSelfSubjectStrategy() usecase.SelfSubjectStrategy {
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
	Repo           model.ResourceRepository
	SchemaRepo     model.SchemaRepository
	Relations      model.RelationsRepository
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
		usecaseCfg = usecase.NewUsecaseConfig()
	}

	// Default to PermissiveMetaAuthorizer for tests unless explicitly overridden
	meta := cfg.MetaAuthorizer
	if meta == nil {
		meta = &PermissiveMetaAuthorizer{}
	}

	// Default to SimpleRelationsRepository when Relations is nil
	authzImpl := cfg.Relations
	if authzImpl == nil {
		authzImpl = data.NewSimpleRelationsRepository()
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

func TestToLookupObjectsCommand(t *testing.T) {
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

	result, err := svc.ToLookupObjectsCommand(input)
	require.NoError(t, err)

	// Verify the command fields
	assert.Equal(t, "host", result.ObjectType.ResourceType().Serialize())
	require.NotNil(t, result.ObjectType.ReporterType())
	assert.Equal(t, "hbi", result.ObjectType.ReporterType().Serialize())
	assert.Equal(t, "view", result.Relation.Serialize())
	assert.Equal(t, "res-id", result.Subject.Resource().ResourceId().Serialize())
	assert.Equal(t, "principal", result.Subject.Resource().ResourceType().Serialize())
	assert.Equal(t, "rbac", result.Subject.Resource().Reporter().ReporterType().Serialize())
	require.NotNil(t, result.Pagination)
	assert.Equal(t, uint32(50), result.Pagination.Limit)
}

func TestToLookupObjectsCommand_WithConsistencyToken(t *testing.T) {
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

	result, err := svc.ToLookupObjectsCommand(input)
	require.NoError(t, err)

	assert.Equal(t, model.ConsistencyAtLeastAsFresh, model.ConsistencyTypeOf(result.Consistency), "expected at-least-as-fresh when token provided")
	atLeastAsFreshToken := model.ConsistencyAtLeastAsFreshToken(result.Consistency)
	require.NotNil(t, atLeastAsFreshToken)
	assert.Equal(t, token, atLeastAsFreshToken.String(), "command should use the request consistency token")
}

func TestToLookupObjectsCommand_NoPagination(t *testing.T) {
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

	result, err := svc.ToLookupObjectsCommand(input)
	require.NoError(t, err)

	assert.Equal(t, "host", result.ObjectType.ResourceType().Serialize())
	require.NotNil(t, result.ObjectType.ReporterType())
	assert.Equal(t, "hbi", result.ObjectType.ReporterType().Serialize())
	assert.Equal(t, "view", result.Relation.Serialize())
	assert.Equal(t, "res-id", result.Subject.Resource().ResourceId().Serialize())
	assert.Equal(t, "principal", result.Subject.Resource().ResourceType().Serialize())
	assert.Equal(t, "rbac", result.Subject.Resource().Reporter().ReporterType().Serialize())
	// When pagination is not specified, both fields should be nil
	assert.Nil(t, result.Pagination)
}

func TestToLookupSubjectsCommand_NoPagination(t *testing.T) {
	reporterType := "hbi"
	input := &pb.StreamedListSubjectsRequest{
		Resource: &pb.ResourceReference{
			ResourceId:   "resource-1",
			ResourceType: "host",
			Reporter: &pb.ReporterReference{
				Type: "hbi",
			},
		},
		Relation: "view",
		SubjectType: &pb.RepresentationType{
			ResourceType: "principal",
			ReporterType: &reporterType,
		},
		// Pagination intentionally omitted
	}

	result, err := svc.ToLookupSubjectsCommand(input)
	require.NoError(t, err)

	assert.Equal(t, "resource-1", result.Resource.ResourceId().Serialize())
	assert.Equal(t, "host", result.Resource.ResourceType().Serialize())
	require.NotNil(t, result.Resource.Reporter())
	assert.Equal(t, "hbi", result.Resource.Reporter().ReporterType().Serialize())
	assert.Equal(t, "view", result.Relation.Serialize())
	assert.Equal(t, "principal", result.SubjectType.ResourceType().Serialize())
	require.NotNil(t, result.SubjectType.ReporterType())
	assert.Equal(t, "hbi", result.SubjectType.ReporterType().Serialize())
	// When pagination is not specified, both fields should be nil
	assert.Nil(t, result.Pagination)
}

func TestIsValidatedRepresentationType(t *testing.T) {
	assert.True(t, IsValidType("hbi"))

	// constructors now normalize
	rt, err := model.NewReporterType("HBI")
	require.NoError(t, err)
	assert.True(t, IsValidType(rt.String()))

	// strange characters
	assert.False(t, IsValidType("h?!!!"))
}

func TestNormalizeRepresentationType(t *testing.T) {
	rt, err := model.NewReporterType("HBI")
	require.NoError(t, err)
	assert.Equal(t, "hbi", rt.String())
}

var typePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func IsValidType(val string) bool {
	return typePattern.MatchString(val)
}

func TestToLookupObjectsResponse(t *testing.T) {
	rep := model.NewReporterReference(model.DeserializeReporterType("reporter-x"), nil)
	input := model.NewLookupObjectsItem(
		model.NewResourceReference(
			model.DeserializeResourceType("type-y"),
			model.DeserializeLocalResourceId("abc123"),
			&rep,
		),
		model.DeserializeContinuationToken("next-page-token"),
	)

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

	result := svc.ToLookupObjectsResponse(input)
	assert.Equal(t, expected, result)
}

func TestInventoryService_CheckSelf_AuthzDecisions(t *testing.T) {
	cases := []struct {
		name           string
		subjectID      string
		grantSubjectID string
		wantAllowed    pb.Allowed
	}{
		{"allowed - user-123", "user-123", "user-123", pb.Allowed_ALLOWED_TRUE},
		{"allowed - testuser subject match", "testuser", "testuser", pb.Allowed_ALLOWED_TRUE},
		{"denied - no grant", "user-123", "", pb.Allowed_ALLOWED_FALSE},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := &authnapi.Claims{
				SubjectId: authnapi.SubjectId(tc.subjectID),
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
				simpleAuthz := data.NewSimpleRelationsRepository()
				if tc.grantSubjectID != "" {
					simpleAuthz.Grant(tc.grantSubjectID, "view", "hbi", "host", "dd1b73b9-3e33-4264-968c-e3ce55b9afec")
				}
				return TestServerConfig{
						Usecase:       newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz}),
						Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
					}, func(t *testing.T, tr *Transport) {
						ctx := context.Background()
						res := tr.Invoke(ctx, withBody(protoReq, CheckSelf, httpEndpoint("POST /api/kessel/v1beta2/checkself")))
						resp := Extract(t, res, expectSuccess(func() *pb.CheckSelfResponse { return &pb.CheckSelfResponse{} }))
						assert.Equal(t, tc.wantAllowed, resp.Allowed)
						assert.NotEmpty(t, resp.ConsistencyToken.GetToken())
					}
			})
		})
	}
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

func TestInventoryService_CheckSelfBulk_AuthzDecisions(t *testing.T) {
	type grantSpec struct {
		subjectID, relation, resourceID string
	}
	type wantPair struct {
		allowed    pb.Allowed
		resourceID string
		relation   string
	}
	cases := []struct {
		name      string
		grants    []grantSpec
		wantPairs []wantPair
	}{
		{
			"all allowed",
			[]grantSpec{
				{"user-123", "view", "resource-1"},
				{"user-123", "edit", "resource-2"},
			},
			[]wantPair{
				{pb.Allowed_ALLOWED_TRUE, "resource-1", "view"},
				{pb.Allowed_ALLOWED_TRUE, "resource-2", "edit"},
			},
		},
		{
			"mixed - first allowed, second denied",
			[]grantSpec{
				{"user-123", "view", "resource-1"},
			},
			[]wantPair{
				{pb.Allowed_ALLOWED_TRUE, "resource-1", "view"},
				{pb.Allowed_ALLOWED_FALSE, "resource-2", "edit"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
				simpleAuthz := data.NewSimpleRelationsRepository()
				for _, g := range tc.grants {
					simpleAuthz.Grant(g.subjectID, g.relation, "hbi", "host", g.resourceID)
				}
				return TestServerConfig{
						Usecase:       newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz}),
						Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
					}, func(t *testing.T, tr *Transport) {
						ctx := context.Background()
						res := tr.Invoke(ctx, withBody(protoReq, CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
						resp := Extract(t, res, expectSuccess(func() *pb.CheckSelfBulkResponse { return &pb.CheckSelfBulkResponse{} }))
						require.Len(t, resp.Pairs, len(tc.wantPairs))
						for i, want := range tc.wantPairs {
							assert.Equal(t, want.allowed, resp.Pairs[i].GetItem().Allowed, "pair %d allowed", i)
							assert.Equal(t, want.resourceID, resp.Pairs[i].Request.Object.ResourceId, "pair %d resourceID", i)
							assert.Equal(t, want.relation, resp.Pairs[i].Request.Relation, "pair %d relation", i)
						}
						assert.NotEmpty(t, resp.ConsistencyToken.GetToken())
					}
			})
		})
	}
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
		// Edge case: CheckBulk returns more responses than requests (2 responses for 1 request)
		// This should cause an Internal error since response length doesn't match request length
		mockRelations := &mocks.MockRelationsRepository{}

		key1, _ := model.NewReporterResourceKey(
			model.DeserializeLocalResourceId("resource-1"),
			model.DeserializeResourceType("host"),
			model.DeserializeReporterType("hbi"),
			model.DeserializeReporterInstanceId(""),
		)
		key2, _ := model.NewReporterResourceKey(
			model.DeserializeLocalResourceId("resource-2"),
			model.DeserializeResourceType("host"),
			model.DeserializeReporterType("hbi"),
			model.DeserializeReporterInstanceId(""),
		)
		subj, _ := buildTestSubjectReference("test-user")
		rel1 := model.NewRelationship(resourceRefFromKey(key1), model.DeserializeRelation("view"), subj)
		rel2 := model.NewRelationship(resourceRefFromKey(key2), model.DeserializeRelation("view"), subj)
		mockRelations.
			On("CheckBulk", mock.Anything, mock.Anything, mock.Anything).
			Return(model.NewCheckBulkResult(
				[]model.CheckBulkResultPair{
					model.NewCheckBulkResultPair(rel1, model.NewCheckBulkResultItem(true, nil, 0)),
					model.NewCheckBulkResultPair(rel2, model.NewCheckBulkResultItem(true, nil, 0)),
				},
				"",
			), nil).
			Once()

		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Relations: mockRelations}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
				Assert(t, res, requireError(codes.Internal).And(func(t *testing.T) { mockRelations.AssertExpectations(t) }))
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
		simpleAuthz := data.NewSimpleRelationsRepository()
		simpleAuthz.Grant("subject-456", "view", "hbi", "host", "resource-abc")
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, Check, httpEndpoint("POST /api/kessel/v1beta2/check")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckResponse { return &pb.CheckResponse{} }))
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Allowed)
				assert.NotNil(t, resp.ConsistencyToken)
				assert.NotEmpty(t, resp.ConsistencyToken.GetToken())
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
				assert.NotNil(t, resp.ConsistencyToken)
				assert.NotEmpty(t, resp.ConsistencyToken.GetToken())
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
		simpleAuthz := data.NewSimpleRelationsRepository()
		simpleAuthz.Grant("subject-789", "edit", "hbi", "host", "resource-xyz")
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz}),
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
		simpleAuthz := data.NewSimpleRelationsRepository()
		simpleAuthz.Grant("subject-a", "view", "hbi", "host", "resource-1")
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz}),
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

func TestInventoryService_StreamedListObjects_StreamResults(t *testing.T) {
	type grantSpec struct {
		subjectID, resourceID string
	}
	cases := []struct {
		name    string
		grants  []grantSpec
		wantIDs []string
	}{
		{
			"success - two objects",
			[]grantSpec{{"subject-xyz", "host-1"}, {"subject-xyz", "host-2"}},
			[]string{"host-1", "host-2"},
		},
		{
			"empty - no grants",
			nil,
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := &authnapi.Claims{
				SubjectId: authnapi.SubjectId("user-abc"),
				AuthType:  authnapi.AuthTypeXRhIdentity,
			}

			simpleAuthz := data.NewSimpleRelationsRepository()
			for _, g := range tc.grants {
				simpleAuthz.Grant(g.subjectID, "view", "hbi", "host", g.resourceID)
			}

			uc := newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz})
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

			var resourceIDs []string
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				resourceIDs = append(resourceIDs, resp.Object.ResourceId)
			}

			slices.Sort(resourceIDs)
			wantSorted := slices.Clone(tc.wantIDs)
			slices.Sort(wantSorted)
			assert.Equal(t, wantSorted, resourceIDs)
		})
	}
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
		repo := newSQLiteTestRepo(t)
		uc := newTestUsecase(t, testUsecaseConfig{Repo: repo})
		key := buildReporterResourceKey(t, "update-effective-host", "host", "hbi", "instance-001")
		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res1 := tr.Invoke(ctx, withBody(createReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res1, requireSuccess())

				resource1, err := repoFindResourceByKeys(repo, key)
				require.NoError(t, err)
				require.NotNil(t, resource1)
				assert.Equal(t, "https://api.example.com/v1/hosts/original", resource1.ReporterResources()[0].ApiHref().String())
				assert.Equal(t, originalConsoleHref, resource1.ReporterResources()[0].ConsoleHref().String())

				reps1, err := repoFindLatestRepresentations(repo, key)
				require.NoError(t, err)
				assert.Equal(t, "ws-original", reps1.CommonData()["workspace_id"])

				res2 := tr.Invoke(ctx, withBody(updateReq, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res2, requireSuccess())

				resource2, err := repoFindResourceByKeys(repo, key)
				require.NoError(t, err)
				require.NotNil(t, resource2)
				rr := resource2.ReporterResources()[0]
				assert.Equal(t, "https://api.example.com/v2/hosts/updated", rr.ApiHref().String(),
					"api_href should reflect the updated value")
				assert.Equal(t, updatedConsoleHref, rr.ConsoleHref().String(),
					"console_href should reflect the updated value")
				reps2, err := repoFindLatestRepresentations(repo, key)
				require.NoError(t, err)
				assert.Equal(t, "ws-updated", reps2.CommonData()["workspace_id"],
					"common data should reflect the updated value")
			}
	})
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

		simpleAuthz := data.NewSimpleRelationsRepository()
		// Grant both permissions at initial version -> v3
		simpleAuthz.Grant("subject-a", "view", "hbi", "host", "resource-1")
		simpleAuthz.Grant("subject-b", "edit", "hbi", "host", "resource-2")
		snapshotVersion := simpleAuthz.RetainCurrentSnapshot() // Retain at v3

		// Remove one permission -> v4
		_, _ = simpleAuthz.DeleteTuples(context.Background(), testTupleFilterForHelper("hbi", "host", "resource-1", "view", "rbac", "principal", "subject-a"), nil)
		currentVersion := simpleAuthz.Version()

		uc := newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz})
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

		simpleAuthz := data.NewSimpleRelationsRepository()
		// Grant permission at initial version -> v3
		// The self subject strategy maps "subject-a" to rbac/principal/subject-a
		simpleAuthz.Grant("subject-a", "view", "hbi", "host", "resource-1")
		simpleAuthz.Grant("subject-a", "edit", "hbi", "host", "resource-2")
		snapshotVersion := simpleAuthz.RetainCurrentSnapshot() // Retain at v3

		// Remove one permission -> v4
		_, _ = simpleAuthz.DeleteTuples(context.Background(), testTupleFilterForHelper("hbi", "host", "resource-1", "view", "rbac", "principal", "subject-a"), nil)
		currentVersion := simpleAuthz.Version()

		uc := newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz})
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
		simpleAuthz := data.NewSimpleRelationsRepository()
		simpleAuthz.Grant("subject-456", "view", "hbi", "host", "resource-with-instance")
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz}),
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
	simpleAuthz := data.NewSimpleRelationsRepository()
	simpleAuthz.Grant("subject-xyz", "view", "hbi", "host", "host-1")

	uc := newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz})
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

func TestInventoryService_StreamedListObjects_ValidationRejectsInvalidRequest(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-abc"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	uc := newTestUsecase(t, testUsecaseConfig{})
	client := newTestServer(t, TestServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})

	stream, err := client.StreamedListObjects(context.Background(), &pb.StreamedListObjectsRequest{})
	require.NoError(t, err)

	_, err = stream.Recv()
	assert.Error(t, err)
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, grpcStatus.Code())
}

func TestInventoryService_StreamedListObjects_ValidationRejectsMissingRelation(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-abc"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	uc := newTestUsecase(t, testUsecaseConfig{})
	client := newTestServer(t, TestServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})

	req := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{ResourceType: "host"},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceType: "principal",
				ResourceId:   "subject-xyz",
			},
		},
	}

	stream, err := client.StreamedListObjects(context.Background(), req)
	require.NoError(t, err)

	_, err = stream.Recv()
	assert.Error(t, err)
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, grpcStatus.Code())
}

func TestInventoryService_StreamedListSubjects_ValidationRejectsInvalidRequest(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-abc"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	uc := newTestUsecase(t, testUsecaseConfig{})
	client := newTestServer(t, TestServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})

	stream, err := client.StreamedListSubjects(context.Background(), &pb.StreamedListSubjectsRequest{})
	require.NoError(t, err)

	_, err = stream.Recv()
	assert.Error(t, err)
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, grpcStatus.Code())
}

func TestInventoryService_StreamedListSubjects_ValidationRejectsMissingRelation(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-abc"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	uc := newTestUsecase(t, testUsecaseConfig{})
	client := newTestServer(t, TestServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})

	req := &pb.StreamedListSubjectsRequest{
		Resource: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-1",
		},
		SubjectType: &pb.RepresentationType{ResourceType: "principal"},
	}

	stream, err := client.StreamedListSubjects(context.Background(), req)
	require.NoError(t, err)

	_, err = stream.Recv()
	assert.Error(t, err)
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, grpcStatus.Code())
}

// --- Streaming proto-level validation (interceptor-dependent) ---
//
// These tests verify that protovalidate constraints (patterns, required fields)
// are enforced on streaming RPCs via the StreamValidationInterceptor. They use
// inputs that violate proto constraints but would pass handler-level validation,
// so they fail if the interceptor is removed.

func TestInventoryService_StreamedListObjects_ProtoPatternRejectsHyphen(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-abc"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	simpleAuthz := data.NewSimpleRelationsRepository()
	uc := newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz})
	client := newTestServer(t, TestServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})

	reporterType := "hbi"
	req := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host-type",
			ReporterType: &reporterType,
		},
		Relation: "view",
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceType: "principal",
				ResourceId:   "subject-xyz",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	stream, err := client.StreamedListObjects(context.Background(), req)
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err)
	grpcStatus, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, grpcStatus.Code())
	assert.Contains(t, grpcStatus.Message(), "does not match regex pattern")
}

func TestInventoryService_StreamedListSubjects_ProtoPatternRejectsHyphen(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-abc"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	simpleAuthz := data.NewSimpleRelationsRepository()
	uc := newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz})
	client := newTestServer(t, TestServerConfig{
		Usecase:       uc,
		Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
	})

	req := &pb.StreamedListSubjectsRequest{
		Resource: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-1",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Relation: "view",
		SubjectType: &pb.RepresentationType{
			ResourceType: "user-principal",
		},
	}

	stream, err := client.StreamedListSubjects(context.Background(), req)
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err)
	grpcStatus, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, grpcStatus.Code())
	assert.Contains(t, grpcStatus.Message(), "does not match regex pattern")
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
		simpleAuthz := data.NewSimpleRelationsRepository()
		simpleAuthz.Grant("subject-a", "view", "hbi", "host", "resource-1")
		return TestServerConfig{
				Usecase: newTestUsecase(t, testUsecaseConfig{
					Relations:      simpleAuthz,
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

func TestInventoryService_CheckForUpdateBulk_AllAllowed(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckForUpdateBulkRequest{
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
				Relation: "update",
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
				Relation: "update",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		// Edge case: Testing consistency token handling in CheckForUpdateBulk
		// SimpleRelationsRepository doesn't support consistency tokens, so we mock CheckForUpdateBulk
		mockRelations := &mocks.MockRelationsRepository{}

		key1, _ := model.NewReporterResourceKey(
			model.DeserializeLocalResourceId("resource-1"),
			model.DeserializeResourceType("host"),
			model.DeserializeReporterType("hbi"),
			model.DeserializeReporterInstanceId(""),
		)
		key2, _ := model.NewReporterResourceKey(
			model.DeserializeLocalResourceId("resource-2"),
			model.DeserializeResourceType("host"),
			model.DeserializeReporterType("hbi"),
			model.DeserializeReporterInstanceId(""),
		)
		subjA, _ := buildTestSubjectReference("subject-a")
		subjB, _ := buildTestSubjectReference("subject-b")
		req1 := model.NewRelationship(resourceRefFromKey(key1), model.DeserializeRelation("update"), subjA)
		req2 := model.NewRelationship(resourceRefFromKey(key2), model.DeserializeRelation("update"), subjB)
		mockRelations.
			On("CheckForUpdateBulk",
				mock.Anything,
				mock.MatchedBy(func(rels []model.Relationship) bool {
					if len(rels) != 2 {
						return false
					}
					s1 := rels[0].Subject().Resource()
					s2 := rels[1].Subject().Resource()
					return s1.ResourceId().String() == "subject-a" &&
						s1.ResourceType().String() == "principal" &&
						s1.Reporter().ReporterType().String() == "rbac" &&
						s2.ResourceId().String() == "subject-b" &&
						s2.ResourceType().String() == "principal" &&
						s2.Reporter().ReporterType().String() == "rbac"
				}),
			).
			Return(model.NewCheckBulkResult(
				[]model.CheckBulkResultPair{
					model.NewCheckBulkResultPair(req1, model.NewCheckBulkResultItem(true, nil, 0)),
					model.NewCheckBulkResultPair(req2, model.NewCheckBulkResultItem(true, nil, 0)),
				},
				model.DeserializeConsistencyToken("update-bulk-token"),
			), nil).
			Once()

		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Relations: mockRelations}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdateBulk, httpEndpoint("POST /api/kessel/v1beta2/checkforupdatebulk")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckForUpdateBulkResponse { return &pb.CheckForUpdateBulkResponse{} }))
				require.Len(t, resp.Pairs, 2)
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed)
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[1].GetItem().Allowed)
				assert.NotNil(t, resp.ConsistencyToken)
				assert.Equal(t, "update-bulk-token", resp.ConsistencyToken.GetToken())
				mockRelations.AssertExpectations(t)
			}
	})
}

func TestInventoryService_CheckForUpdateBulk_MixedResults(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckForUpdateBulkRequest{
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
				Relation: "update",
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
				Relation: "update",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		simpleAuthz := data.NewSimpleRelationsRepository()
		simpleAuthz.Grant("subject-a", "update", "hbi", "host", "resource-1")
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdateBulk, httpEndpoint("POST /api/kessel/v1beta2/checkforupdatebulk")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckForUpdateBulkResponse { return &pb.CheckForUpdateBulkResponse{} }))
				require.Len(t, resp.Pairs, 2)
				assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Pairs[0].GetItem().Allowed)
				assert.Equal(t, "resource-1", resp.Pairs[0].Request.Object.ResourceId)
				assert.Equal(t, "update", resp.Pairs[0].Request.Relation)
				assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Pairs[1].GetItem().Allowed)
				assert.Equal(t, "resource-2", resp.Pairs[1].Request.Object.ResourceId)
				assert.Equal(t, "update", resp.Pairs[1].Request.Relation)
				assert.NotNil(t, resp.ConsistencyToken)
			}
	})
}

// --- CheckForUpdateBulk with NoIdentity ---

func TestInventoryService_CheckForUpdateBulk_NoIdentity(t *testing.T) {
	protoReq := &pb.CheckForUpdateBulkRequest{
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
				Relation: "update",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &DenyAuthenticator{},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdateBulk, httpEndpoint("POST /api/kessel/v1beta2/checkforupdatebulk")))
				Assert(t, res, requireError(codes.Unauthenticated))
			}
	})
}

// --- CheckForUpdateBulk Error Scenarios ---

func TestInventoryService_CheckForUpdateBulk_MetaAuthzProtocolBehavior(t *testing.T) {
	// SimpleMetaAuthorizer protocol-aware behavior:
	// - gRPC: allow ALL relations EXCEPT "check_self" -> CheckForUpdateBulk ALLOWED
	// - HTTP + OIDC: deny (only x-rh-identity + check_self is allowed) -> PermissionDenied
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeOIDC,
	}

	protoReq := &pb.CheckForUpdateBulkRequest{
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
				Relation: "update",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		simpleAuthz := data.NewSimpleRelationsRepository()
		simpleAuthz.Grant("subject-a", "update", "hbi", "host", "resource-1")
		return TestServerConfig{
				Usecase: newTestUsecase(t, testUsecaseConfig{
					Relations:      simpleAuthz,
					MetaAuthorizer: metaauthorizer.NewSimpleMetaAuthorizer(),
				}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdateBulk, httpEndpoint("POST /api/kessel/v1beta2/checkforupdatebulk")))
				Assert(t, res, Expectation{
					GRPC: func(t *testing.T, resp proto.Message, err error) {
						require.NoError(t, err)
						r := resp.(*pb.CheckForUpdateBulkResponse)
						assert.Equal(t, pb.Allowed_ALLOWED_TRUE, r.Pairs[0].GetItem().Allowed)
					},
					HTTP: func(t *testing.T, statusCode int, _ []byte) {
						assert.Equal(t, 403, statusCode)
					},
				})
			}
	})
}

func TestInventoryService_CheckForUpdateBulk_MetaAuthzDenied(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckForUpdateBulkRequest{
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
				Relation: "update",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{MetaAuthorizer: &DenyingMetaAuthorizer{}}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdateBulk, httpEndpoint("POST /api/kessel/v1beta2/checkforupdatebulk")))
				Assert(t, res, requireError(codes.PermissionDenied))
			}
	})
}

func TestInventoryService_CheckForUpdateBulk_InvalidReference_EmptyResourceId(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckForUpdateBulkRequest{
		Items: []*pb.CheckBulkRequestItem{
			{
				Object: &pb.ResourceReference{
					ResourceId:   "",
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
				Relation: "update",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdateBulk, httpEndpoint("POST /api/kessel/v1beta2/checkforupdatebulk")))
				Assert(t, res, requireError(codes.InvalidArgument))
			}
	})
}

func TestInventoryService_CheckForUpdateBulk_PairError(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckForUpdateBulkRequest{
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
				Relation: "update",
			},
		},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		mockRelations := &mocks.MockRelationsRepository{}

		key1, _ := model.NewReporterResourceKey(
			model.DeserializeLocalResourceId("resource-1"),
			model.DeserializeResourceType("host"),
			model.DeserializeReporterType("hbi"),
			model.DeserializeReporterInstanceId(""),
		)
		subjA, _ := buildTestSubjectReference("subject-a")
		errReq := model.NewRelationship(resourceRefFromKey(key1), model.DeserializeRelation("update"), subjA)
		mockRelations.
			On("CheckForUpdateBulk", mock.Anything, mock.Anything).
			Return(model.NewCheckBulkResult(
				[]model.CheckBulkResultPair{
					model.NewCheckBulkResultPair(errReq, model.NewCheckBulkResultItem(false, errors.New("denied by policy"), int32(codes.PermissionDenied))),
				},
				model.DeserializeConsistencyToken("error-token"),
			), nil).
			Once()
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{Relations: mockRelations}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdateBulk, httpEndpoint("POST /api/kessel/v1beta2/checkforupdatebulk")))
				resp := Extract(t, res, expectSuccess(func() *pb.CheckForUpdateBulkResponse { return &pb.CheckForUpdateBulkResponse{} }))
				require.Len(t, resp.Pairs, 1)
				assert.Nil(t, resp.Pairs[0].GetItem())
				pairErr := resp.Pairs[0].GetError()
				require.NotNil(t, pairErr, "expected per-pair error but got nil")
				assert.Equal(t, int32(codes.PermissionDenied), pairErr.GetCode())
				assert.Contains(t, pairErr.GetMessage(), "denied by policy")
				require.NotNil(t, resp.ConsistencyToken)
				assert.Equal(t, "error-token", resp.ConsistencyToken.GetToken())
				mockRelations.AssertExpectations(t)
			}
	})
}

func TestInventoryService_CheckForUpdateBulk_EmptyItems(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("user-123"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	protoReq := &pb.CheckForUpdateBulkRequest{
		Items: []*pb.CheckBulkRequestItem{},
	}

	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
		return TestServerConfig{
				Usecase:       newTestUsecase(t, testUsecaseConfig{}),
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(protoReq, CheckForUpdateBulk, httpEndpoint("POST /api/kessel/v1beta2/checkforupdatebulk")))
				Assert(t, res, requireErrorContaining(codes.InvalidArgument, "items"))
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
	// ToLookupObjectsCommand returns error for nil request
	// This is validated by protovalidate before reaching the handler,
	// so this tests the internal function behavior only.
	_, err := svc.ToLookupObjectsCommand(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")
}

// newSQLiteTestRepo creates a real GORM repository backed by an in-memory SQLite
// database with all migrations applied.
func newSQLiteTestRepo(t *testing.T) model.ResourceRepository {
	t.Helper()
	db := testutil.NewSQLiteTestDB(t, &gorm.Config{TranslateError: true})
	err := data.Migrate(db, nil)
	require.NoError(t, err)
	mc := metricscollector.NewFakeMetricsCollector()
	noopPublisher := data.OutboxPublisher(func(_ *gorm.DB, _ *model_legacy.OutboxEvent) error { return nil })
	return data.NewResourceRepository(data.GormResourceRepositoryConfig{
		DB:                      db,
		OutboxPublisher:         noopPublisher,
		MetricsCollector:        mc,
		MaxSerializationRetries: 3,
	})
}

func repoFindResourceByKeys(repo model.ResourceRepository, key model.ReporterResourceKey) (*model.Resource, error) {
	tx, err := repo.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	resource, err := tx.FindResourceByKeys(key)
	if err != nil {
		return nil, err
	}
	_ = tx.Commit()
	return resource, nil
}

func repoFindLatestRepresentations(repo model.ResourceRepository, key model.ReporterResourceKey) (*model.Representations, error) {
	tx, err := repo.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	reps, err := tx.FindLatestRepresentations(key)
	if err != nil {
		return nil, err
	}
	_ = tx.Commit()
	return reps, nil
}

func repoHasTransactionIdBeenProcessed(repo model.ResourceRepository, transactionId model.TransactionId) (bool, error) {
	tx, err := repo.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	processed, err := tx.HasTransactionIdBeenProcessed(transactionId)
	if err != nil {
		return false, err
	}
	_ = tx.Commit()
	return processed, nil
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
		repo := newSQLiteTestRepo(t)
		uc := newTestUsecase(t, testUsecaseConfig{Repo: repo})
		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(req, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res, requireSuccess())

				key := buildReporterResourceKey(t, "host-all-optional", "host", "hbi", "instance-001")
				resource, err := repoFindResourceByKeys(repo, key)
				require.NoError(t, err)
				require.NotNil(t, resource)

				rr := resource.ReporterResources()[0]
				assert.Equal(t, "host-all-optional", rr.Key().LocalResourceId().String())
				assert.Equal(t, "host", rr.Key().ResourceType().String())
				assert.Equal(t, "hbi", rr.Key().ReporterType().String())
				assert.Equal(t, "instance-001", rr.Key().ReporterInstanceId().String())
				assert.Equal(t, "https://api.example.com/hosts/host-all-optional", rr.ApiHref().String())
				assert.Equal(t, consoleHref, rr.ConsoleHref().String())

				reps, err := repoFindLatestRepresentations(repo, key)
				require.NoError(t, err)
				require.NotNil(t, reps)
				assert.Equal(t, "ws-all-optional", string(reps.CommonData()["workspace_id"].(string)))

				processed, err := repoHasTransactionIdBeenProcessed(repo, model.NewTransactionId(txId))
				require.NoError(t, err)
				assert.True(t, processed, "transaction_id should be recorded as processed")
			}
	})
}

// --- ReportResource: Nil/Empty Optional Struct Combinations ---

// Representation validation: both common and reporter must be non-nil and non-empty.
// Both nil, reporter-only, common-only, or both-empty return an error.
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
		expectMsg       string // empty string means the request is expected to succeed
	}{
		{
			name:            "both nil",
			localResourceId: "host-both-nil",
			common:          nil,
			reporter:        nil,
			expectMsg:       "at least one of reporterRepresentation or commonRepresentation must be provided",
		},
		{
			name:            "both empty structs",
			localResourceId: "host-both-empty",
			common:          &structpb.Struct{},
			reporter:        &structpb.Struct{},
			expectMsg:       "representation data cannot be empty",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
				repo := newSQLiteTestRepo(t)
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
				expectation := requireErrorContaining(codes.InvalidArgument, tc.expectMsg)
				return TestServerConfig{
						Usecase:       uc,
						Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
					}, func(t *testing.T, tr *Transport) {
						ctx := context.Background()
						res := tr.Invoke(ctx, withBody(req, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res, expectation)
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
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
				repo := newSQLiteTestRepo(t)
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
				expectation := requireErrorContaining(tc.expectCode, tc.expectMsgContains)
				return TestServerConfig{
						Usecase:       uc,
						Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
					}, func(t *testing.T, tr *Transport) {
						ctx := context.Background()
						res := tr.Invoke(ctx, withBody(req, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res, expectation)
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
				repo := newSQLiteTestRepo(t)
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
						resource, err := repoFindResourceByKeys(repo, key)
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
		repo := newSQLiteTestRepo(t)
		uc := newTestUsecase(t, testUsecaseConfig{Repo: repo})
		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				res := tr.Invoke(ctx, withBody(req, ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res, requireSuccess())

				key := buildReporterResourceKey(t, "host-with-inventory-id", "host", "hbi", "instance-001")
				resource, err := repoFindResourceByKeys(repo, key)
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
			expectMsg: "type: must be at least 1 characters",
		},
		{
			name: "invalid type pattern",
			mutate: func(r *pb.ReportResourceRequest) {
				r.Type = "host!@#"
			},
			expectMsg: "type: does not match regex pattern `^[A-Za-z0-9_-]+$`",
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
			expectMsg: "representations.metadata.local_resource_id: must be at least 1 characters",
		},
		{
			name: "missing api_href",
			mutate: func(r *pb.ReportResourceRequest) {
				r.Representations.Metadata.ApiHref = ""
			},
			expectMsg: "representations.metadata.api_href: must be at least 1 characters",
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
		repo := newSQLiteTestRepo(t)
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

				processed, err := repoHasTransactionIdBeenProcessed(repo, model.NewTransactionId(txId))
				require.NoError(t, err)
				assert.True(t, processed, "transaction_id should be recorded after first report")

				resource1, err := repoFindResourceByKeys(repo, key)
				require.NoError(t, err)
				require.NotNil(t, resource1)
				apiHrefAfterFirst := resource1.ReporterResources()[0].ApiHref().String()

				// Second report with same transaction_id but different api_href
				res2 := tr.Invoke(ctx, withBody(makeReq("https://api.example.com/v2-should-be-ignored"), ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res2, requireSuccess())

				resource2, err := repoFindResourceByKeys(repo, key)
				require.NoError(t, err)
				require.NotNil(t, resource2)
				apiHrefAfterSecond := resource2.ReporterResources()[0].ApiHref().String()

				assert.Equal(t, apiHrefAfterFirst, apiHrefAfterSecond,
					"second report with same transaction_id should be a no-op; api_href should not change")
			}
	})
}

func TestInventoryService_ReportResource_IdempotencyDisabled(t *testing.T) {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("reporter-service"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}

	txId := "txn-idempotency-disabled-test"
	makeReq := func(apiHref string) *pb.ReportResourceRequest {
		return &pb.ReportResourceRequest{
			Type:               "host",
			ReporterType:       "hbi",
			ReporterInstanceId: "instance-001",
			Representations: &pb.ResourceRepresentations{
				Metadata: &pb.RepresentationMetadata{
					LocalResourceId: "host-replay",
					ApiHref:         apiHref,
					IdempotencyKey: &pb.RepresentationMetadata_TransactionId{
						TransactionId: txId,
					},
				},
				Common: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"workspace_id": structpb.NewStringValue("ws-replay"),
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
		repo := newSQLiteTestRepo(t)
		idempotencyOff := usecase.NewUsecaseConfig()
		idempotencyOff.IdempotencyCheckEnabled = false
		uc := newTestUsecase(t, testUsecaseConfig{
			Repo:   repo,
			Config: idempotencyOff,
		})
		return TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			}, func(t *testing.T, tr *Transport) {
				ctx := context.Background()
				key := buildReporterResourceKey(t, "host-replay", "host", "hbi", "instance-001")

				// First report
				res1 := tr.Invoke(ctx, withBody(makeReq("https://api.example.com/v1"), ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res1, requireSuccess())

				resource1, err := repoFindResourceByKeys(repo, key)
				require.NoError(t, err)
				require.NotNil(t, resource1)
				apiHrefAfterFirst := resource1.ReporterResources()[0].ApiHref().String()
				assert.Equal(t, "https://api.example.com/v1", apiHrefAfterFirst)

				// Second report with same transaction_id but different api_href — should NOT be skipped
				res2 := tr.Invoke(ctx, withBody(makeReq("https://api.example.com/v2-replayed"), ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
				Assert(t, res2, requireSuccess())

				resource2, err := repoFindResourceByKeys(repo, key)
				require.NoError(t, err)
				require.NotNil(t, resource2)
				apiHrefAfterSecond := resource2.ReporterResources()[0].ApiHref().String()

				assert.Equal(t, "https://api.example.com/v2-replayed", apiHrefAfterSecond,
					"with idempotency disabled, replayed event should update the resource")
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

func TestInventoryService_StreamedListSubjects_StreamResults(t *testing.T) {
	cases := []struct {
		name            string
		grantSubjectIDs []string
		wantIDs         []string
	}{
		{
			"success - two subjects",
			[]string{"user-1", "user-2"},
			[]string{"user-1", "user-2"},
		},
		{
			"empty - no grants",
			nil,
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := &authnapi.Claims{
				SubjectId: authnapi.SubjectId("user-abc"),
				AuthType:  authnapi.AuthTypeXRhIdentity,
			}

			simpleAuthz := data.NewSimpleRelationsRepository()
			for _, subjectID := range tc.grantSubjectIDs {
				simpleAuthz.Grant(subjectID, "view", "hbi", "host", "host-1")
			}

			uc := newTestUsecase(t, testUsecaseConfig{Relations: simpleAuthz})
			client := newTestServer(t, TestServerConfig{
				Usecase:       uc,
				Authenticator: &StubAuthenticator{Claims: claims, Decision: authnapi.Allow},
			})

			reporterType := "hbi"
			subjectReporterType := "rbac"
			req := &pb.StreamedListSubjectsRequest{
				Resource: &pb.ResourceReference{
					ResourceType: "host",
					ResourceId:   "host-1",
					Reporter:     &pb.ReporterReference{Type: reporterType},
				},
				Relation: "view",
				SubjectType: &pb.RepresentationType{
					ResourceType: "principal",
					ReporterType: &subjectReporterType,
				},
			}

			stream, err := client.StreamedListSubjects(context.Background(), req)
			require.NoError(t, err)

			var subjectIDs []string
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				subjectIDs = append(subjectIDs, resp.Subject.Resource.ResourceId)
			}

			slices.Sort(subjectIDs)
			wantSorted := slices.Clone(tc.wantIDs)
			slices.Sort(wantSorted)
			assert.Equal(t, wantSorted, subjectIDs)
		})
	}
}

func newFakeSchemaRepository(t *testing.T) model.SchemaRepository {
	schemaRepository := data.NewInMemorySchemaRepository()

	emptyValidationSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
		},
		"required": []
	}`)

	withWorkspaceValidationSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"workspace_id": { "type": "string" }
		}
	}`)

	k8sCluster, err := model.NewResourceType("k8s_cluster")
	require.NoError(t, err)
	host, err := model.NewResourceType("host")
	require.NoError(t, err)
	ocm, err := model.NewReporterType("ocm")
	require.NoError(t, err)
	hbi, err := model.NewReporterType("hbi")
	require.NoError(t, err)

	k8sClusterSchema, err := model.NewResourceSchemaRepresentation(k8sCluster, withWorkspaceValidationSchema)
	require.NoError(t, err)
	err = schemaRepository.CreateResourceSchema(context.Background(), k8sClusterSchema)
	require.NoError(t, err)

	k8sClusterOcm, err := model.NewReporterSchemaRepresentation(k8sCluster, ocm, emptyValidationSchema)
	require.NoError(t, err)
	err = schemaRepository.CreateReporterSchema(context.Background(), k8sClusterOcm)
	require.NoError(t, err)

	hostSchema, err := model.NewResourceSchemaRepresentation(host, withWorkspaceValidationSchema)
	require.NoError(t, err)
	err = schemaRepository.CreateResourceSchema(context.Background(), hostSchema)
	require.NoError(t, err)

	hostHbi, err := model.NewReporterSchemaRepresentation(host, hbi, emptyValidationSchema)
	require.NoError(t, err)
	err = schemaRepository.CreateReporterSchema(context.Background(), hostHbi)
	require.NoError(t, err)

	return schemaRepository
}

func assertProtoEqual(t *testing.T, expected, actual proto.Message) {
	t.Helper()
	if !proto.Equal(expected, actual) {
		t.Errorf("proto mismatch\nwant: %v\n got: %v", expected, actual)
	}
}

// --- Case-insensitive test helpers ---

type caseVariant struct {
	name         string
	resourceType string
	reporterType string
}

var caseVariants = []caseVariant{
	{"lowercase types", "host", "hbi"},
	{"uppercase types", "HOST", "HBI"},
	{"mixed case", "Host", "Hbi"},
}

func makeResourceRef(resourceId, resourceType, reporterType string) *pb.ResourceReference {
	return &pb.ResourceReference{
		ResourceId:   resourceId,
		ResourceType: resourceType,
		Reporter:     &pb.ReporterReference{Type: reporterType},
	}
}

func makeSubjectRef(subjectId string) *pb.SubjectReference {
	return &pb.SubjectReference{
		Resource: makeResourceRef(subjectId, "principal", "rbac"),
	}
}

func makeAuthzConfig(t *testing.T, authz *data.SimpleRelationsRepository, subjectId string) TestServerConfig {
	t.Helper()
	return TestServerConfig{
		Usecase: newTestUsecase(t, testUsecaseConfig{Relations: authz}),
		Authenticator: &StubAuthenticator{
			Claims:   &authnapi.Claims{SubjectId: authnapi.SubjectId(subjectId), AuthType: authnapi.AuthTypeXRhIdentity},
			Decision: authnapi.Allow,
		},
	}
}

func makeReportConfig(t *testing.T) TestServerConfig {
	t.Helper()
	return TestServerConfig{
		Usecase: newTestUsecase(t, testUsecaseConfig{}),
		Authenticator: &StubAuthenticator{
			Claims:   &authnapi.Claims{SubjectId: authnapi.SubjectId("reporter-service"), AuthType: authnapi.AuthTypeXRhIdentity},
			Decision: authnapi.Allow,
		},
	}
}

func makeReportReq(resourceType, reporterType, instanceId, localResourceId, hostname string) *pb.ReportResourceRequest {
	return &pb.ReportResourceRequest{
		Type:               resourceType,
		ReporterType:       reporterType,
		ReporterInstanceId: instanceId,
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: localResourceId,
				ApiHref:         "https://api.example.com/hosts/" + localResourceId,
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue(hostname),
				},
			},
		},
	}
}

// --- Normalization tests ---

func TestInventoryService_CaseInsensitiveTypes(t *testing.T) {
	t.Parallel()

	t.Run("Report", func(t *testing.T) {
		t.Parallel()
		// Resource and reporter types are normalized to lowercase by the service layer.
		cases := []struct {
			name         string
			resourceType string
			reporterType string
		}{
			{"lowercase types", "host", "hbi"},
			{"uppercase resource type", "HOST", "hbi"},
			{"uppercase reporter type", "host", "HBI"},
			{"all uppercase", "HOST", "HBI"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
					return makeReportConfig(t), func(t *testing.T, tr *Transport) {
						res := tr.Invoke(context.Background(), withBody(
							makeReportReq(tc.resourceType, tc.reporterType, "instance-001", "my-host-123", "example-host"),
							ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res, requireSuccess())
					}
				})
			})
		}
	})

	t.Run("ReportUpdate", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name           string
			createType     string
			createReporter string
			updateType     string
			updateReporter string
		}{
			{"create uppercase, update lowercase", "HOST", "HBI", "host", "hbi"},
			{"create lowercase, update uppercase", "host", "hbi", "HOST", "HBI"},
			{"create mixed, update lowercase", "Host", "Hbi", "host", "hbi"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
					return makeReportConfig(t), func(t *testing.T, tr *Transport) {
						ctx := context.Background()
						res1 := tr.Invoke(ctx, withBody(
							makeReportReq(tc.createType, tc.createReporter, "instance-001", "my-host-123", "original-host"),
							ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res1, requireSuccess())

						res2 := tr.Invoke(ctx, withBody(
							makeReportReq(tc.updateType, tc.updateReporter, "instance-001", "my-host-123", "updated-host"),
							ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res2, requireSuccess())
					}
				})
			})
		}
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name           string
			reportType     string
			reportReporter string
			deleteType     string
			deleteReporter string
		}{
			{"report uppercase, delete lowercase", "HOST", "HBI", "host", "hbi"},
			{"report lowercase, delete uppercase", "host", "hbi", "HOST", "HBI"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
					instanceId := "instance-001"
					return makeReportConfig(t), func(t *testing.T, tr *Transport) {
						ctx := context.Background()
						res1 := tr.Invoke(ctx, withBody(
							makeReportReq(tc.reportType, tc.reportReporter, instanceId, "my-host-123", "example-host"),
							ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res1, requireSuccess())

						res2 := tr.Invoke(ctx, withBody(&pb.DeleteResourceRequest{
							Reference: &pb.ResourceReference{
								ResourceType: tc.deleteType,
								ResourceId:   "my-host-123",
								Reporter: &pb.ReporterReference{
									Type:       tc.deleteReporter,
									InstanceId: &instanceId,
								},
							},
						}, DeleteResource, httpEndpoint("DELETE /api/kessel/v1beta2/resources")))
						Assert(t, res2, requireSuccess())
					}
				})
			})
		}
	})

	t.Run("Check", func(t *testing.T) {
		t.Parallel()
		for _, tc := range caseVariants {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
					authz := data.NewSimpleRelationsRepository()
					authz.Grant("subject-456", "view", "hbi", "host", "resource-abc")
					return makeAuthzConfig(t, authz, "user-123"), func(t *testing.T, tr *Transport) {
						res := tr.Invoke(context.Background(), withBody(&pb.CheckRequest{
							Relation: "view",
							Object:   makeResourceRef("resource-abc", tc.resourceType, tc.reporterType),
							Subject:  makeSubjectRef("subject-456"),
						}, Check, httpEndpoint("POST /api/kessel/v1beta2/check")))
						resp := Extract(t, res, expectSuccess(func() *pb.CheckResponse { return &pb.CheckResponse{} }))
						require.NotEmpty(t, resp.ConsistencyToken.GetToken())
						assertProtoEqual(t, &pb.CheckResponse{
							Allowed:          pb.Allowed_ALLOWED_TRUE,
							ConsistencyToken: resp.ConsistencyToken,
						}, resp)
					}
				})
			})
		}
	})

	t.Run("CheckForUpdate", func(t *testing.T) {
		t.Parallel()
		for _, tc := range caseVariants {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
					authz := data.NewSimpleRelationsRepository()
					authz.Grant("subject-789", "edit", "hbi", "host", "resource-xyz")
					return makeAuthzConfig(t, authz, "user-123"), func(t *testing.T, tr *Transport) {
						res := tr.Invoke(context.Background(), withBody(&pb.CheckForUpdateRequest{
							Relation: "edit",
							Object:   makeResourceRef("resource-xyz", tc.resourceType, tc.reporterType),
							Subject:  makeSubjectRef("subject-789"),
						}, CheckForUpdate, httpEndpoint("POST /api/kessel/v1beta2/checkforupdate")))
						resp := Extract(t, res, expectSuccess(func() *pb.CheckForUpdateResponse { return &pb.CheckForUpdateResponse{} }))
						require.NotEmpty(t, resp.ConsistencyToken.GetToken())
						assertProtoEqual(t, &pb.CheckForUpdateResponse{
							Allowed:          pb.Allowed_ALLOWED_TRUE,
							ConsistencyToken: resp.ConsistencyToken,
						}, resp)
					}
				})
			})
		}
	})

	t.Run("CheckSelf", func(t *testing.T) {
		t.Parallel()
		for _, tc := range caseVariants {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
					authz := data.NewSimpleRelationsRepository()
					authz.Grant("user-123", "view", "hbi", "host", "resource-abc")
					return makeAuthzConfig(t, authz, "user-123"), func(t *testing.T, tr *Transport) {
						res := tr.Invoke(context.Background(), withBody(&pb.CheckSelfRequest{
							Relation: "view",
							Object:   makeResourceRef("resource-abc", tc.resourceType, tc.reporterType),
						}, CheckSelf, httpEndpoint("POST /api/kessel/v1beta2/checkself")))
						resp := Extract(t, res, expectSuccess(func() *pb.CheckSelfResponse { return &pb.CheckSelfResponse{} }))
						require.NotEmpty(t, resp.ConsistencyToken.GetToken())
						assertProtoEqual(t, &pb.CheckSelfResponse{
							Allowed:          pb.Allowed_ALLOWED_TRUE,
							ConsistencyToken: resp.ConsistencyToken,
						}, resp)
					}
				})
			})
		}
	})

	t.Run("CheckBulk", func(t *testing.T) {
		t.Parallel()
		for _, tc := range caseVariants {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
					authz := data.NewSimpleRelationsRepository()
					authz.Grant("subject-a", "view", "hbi", "host", "resource-1")
					items := []*pb.CheckBulkRequestItem{
						{Object: makeResourceRef("resource-1", tc.resourceType, tc.reporterType), Subject: makeSubjectRef("subject-a"), Relation: "view"},
						{Object: makeResourceRef("resource-2", tc.resourceType, tc.reporterType), Subject: makeSubjectRef("subject-b"), Relation: "edit"},
					}
					return makeAuthzConfig(t, authz, "user-123"), func(t *testing.T, tr *Transport) {
						res := tr.Invoke(context.Background(), withBody(&pb.CheckBulkRequest{Items: items},
							CheckBulk, httpEndpoint("POST /api/kessel/v1beta2/checkbulk")))
						resp := Extract(t, res, expectSuccess(func() *pb.CheckBulkResponse { return &pb.CheckBulkResponse{} }))
						require.NotEmpty(t, resp.ConsistencyToken.GetToken())
						assertProtoEqual(t, &pb.CheckBulkResponse{
							Pairs: []*pb.CheckBulkResponsePair{
								{Request: items[0], Response: &pb.CheckBulkResponsePair_Item{Item: &pb.CheckBulkResponseItem{Allowed: pb.Allowed_ALLOWED_TRUE}}},
								{Request: items[1], Response: &pb.CheckBulkResponsePair_Item{Item: &pb.CheckBulkResponseItem{Allowed: pb.Allowed_ALLOWED_FALSE}}},
							},
							ConsistencyToken: resp.ConsistencyToken,
						}, resp)
					}
				})
			})
		}
	})

	t.Run("CheckForUpdateBulk", func(t *testing.T) {
		t.Parallel()
		for _, tc := range caseVariants {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
					authz := data.NewSimpleRelationsRepository()
					authz.Grant("subject-a", "update", "hbi", "host", "resource-1")
					items := []*pb.CheckBulkRequestItem{
						{Object: makeResourceRef("resource-1", tc.resourceType, tc.reporterType), Subject: makeSubjectRef("subject-a"), Relation: "update"},
						{Object: makeResourceRef("resource-2", tc.resourceType, tc.reporterType), Subject: makeSubjectRef("subject-b"), Relation: "update"},
					}
					return makeAuthzConfig(t, authz, "user-123"), func(t *testing.T, tr *Transport) {
						res := tr.Invoke(context.Background(), withBody(&pb.CheckForUpdateBulkRequest{Items: items},
							CheckForUpdateBulk, httpEndpoint("POST /api/kessel/v1beta2/checkforupdatebulk")))
						resp := Extract(t, res, expectSuccess(func() *pb.CheckForUpdateBulkResponse { return &pb.CheckForUpdateBulkResponse{} }))
						require.NotEmpty(t, resp.ConsistencyToken.GetToken())
						assertProtoEqual(t, &pb.CheckForUpdateBulkResponse{
							Pairs: []*pb.CheckForUpdateBulkResponsePair{
								{Request: items[0], Response: &pb.CheckForUpdateBulkResponsePair_Item{Item: &pb.CheckForUpdateBulkResponseItem{Allowed: pb.Allowed_ALLOWED_TRUE}}},
								{Request: items[1], Response: &pb.CheckForUpdateBulkResponsePair_Item{Item: &pb.CheckForUpdateBulkResponseItem{Allowed: pb.Allowed_ALLOWED_FALSE}}},
							},
							ConsistencyToken: resp.ConsistencyToken,
						}, resp)
					}
				})
			})
		}
	})

	t.Run("CheckSelfBulk", func(t *testing.T) {
		t.Parallel()
		for _, tc := range caseVariants {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
					authz := data.NewSimpleRelationsRepository()
					authz.Grant("user-123", "view", "hbi", "host", "resource-1")
					items := []*pb.CheckSelfBulkRequestItem{
						{Object: makeResourceRef("resource-1", tc.resourceType, tc.reporterType), Relation: "view"},
						{Object: makeResourceRef("resource-2", tc.resourceType, tc.reporterType), Relation: "edit"},
					}
					return makeAuthzConfig(t, authz, "user-123"), func(t *testing.T, tr *Transport) {
						res := tr.Invoke(context.Background(), withBody(&pb.CheckSelfBulkRequest{Items: items},
							CheckSelfBulk, httpEndpoint("POST /api/kessel/v1beta2/checkselfbulk")))
						resp := Extract(t, res, expectSuccess(func() *pb.CheckSelfBulkResponse { return &pb.CheckSelfBulkResponse{} }))
						require.NotEmpty(t, resp.ConsistencyToken.GetToken())
						assertProtoEqual(t, &pb.CheckSelfBulkResponse{
							Pairs: []*pb.CheckSelfBulkResponsePair{
								{Request: items[0], Response: &pb.CheckSelfBulkResponsePair_Item{Item: &pb.CheckSelfBulkResponseItem{Allowed: pb.Allowed_ALLOWED_TRUE}}},
								{Request: items[1], Response: &pb.CheckSelfBulkResponsePair_Item{Item: &pb.CheckSelfBulkResponseItem{Allowed: pb.Allowed_ALLOWED_FALSE}}},
							},
							ConsistencyToken: resp.ConsistencyToken,
						}, resp)
					}
				})
			})
		}
	})

	t.Run("StreamedListObjects", func(t *testing.T) {
		t.Parallel()
		for _, tc := range caseVariants {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				authz := data.NewSimpleRelationsRepository()
				authz.Grant("subject-xyz", "view", "hbi", "host", "host-1")
				authz.Grant("subject-xyz", "view", "hbi", "host", "host-2")

				reporterType := tc.reporterType
				client := newTestServer(t, makeAuthzConfig(t, authz, "user-abc"))
				stream, err := client.StreamedListObjects(context.Background(), &pb.StreamedListObjectsRequest{
					ObjectType: &pb.RepresentationType{
						ReporterType: &reporterType,
						ResourceType: tc.resourceType,
					},
					Relation: "view",
					Subject:  makeSubjectRef("subject-xyz"),
				})
				require.NoError(t, err)

				var objects []*pb.ResourceReference
				for {
					resp, err := stream.Recv()
					if err == io.EOF {
						break
					}
					require.NoError(t, err)
					objects = append(objects, resp.Object)
				}

				slices.SortFunc(objects, func(a, b *pb.ResourceReference) int {
					if a.ResourceId < b.ResourceId {
						return -1
					}
					if a.ResourceId > b.ResourceId {
						return 1
					}
					return 0
				})
				require.Len(t, objects, 2)
				assertProtoEqual(t, makeResourceRef("host-1", "host", "hbi"), objects[0])
				assertProtoEqual(t, makeResourceRef("host-2", "host", "hbi"), objects[1])
			})
		}
	})

	t.Run("StreamedListSubjects", func(t *testing.T) {
		t.Parallel()
		for _, tc := range caseVariants {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				authz := data.NewSimpleRelationsRepository()
				authz.Grant("user-1", "view", "hbi", "host", "host-1")
				authz.Grant("user-2", "view", "hbi", "host", "host-1")

				reporterType := tc.reporterType
				subjectReporterType := "rbac"
				client := newTestServer(t, makeAuthzConfig(t, authz, "user-abc"))
				stream, err := client.StreamedListSubjects(context.Background(), &pb.StreamedListSubjectsRequest{
					Resource: &pb.ResourceReference{
						ResourceType: tc.resourceType,
						ResourceId:   "host-1",
						Reporter:     &pb.ReporterReference{Type: reporterType},
					},
					Relation: "view",
					SubjectType: &pb.RepresentationType{
						ResourceType: "principal",
						ReporterType: &subjectReporterType,
					},
				})
				require.NoError(t, err)

				var subjects []*pb.SubjectReference
				for {
					resp, err := stream.Recv()
					if err == io.EOF {
						break
					}
					require.NoError(t, err)
					subjects = append(subjects, resp.Subject)
				}

				slices.SortFunc(subjects, func(a, b *pb.SubjectReference) int {
					if a.Resource.ResourceId < b.Resource.ResourceId {
						return -1
					}
					if a.Resource.ResourceId > b.Resource.ResourceId {
						return 1
					}
					return 0
				})
				require.Len(t, subjects, 2)
				assertProtoEqual(t, makeSubjectRef("user-1"), subjects[0])
				assertProtoEqual(t, makeSubjectRef("user-2"), subjects[1])
			})
		}
	})

	t.Run("ReportEmptyType", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name         string
			resourceType string
			reporterType string
		}{
			{"empty resource type", "", "hbi"},
			{"empty reporter type", "host", ""},
			{"whitespace resource type", "   ", "hbi"},
			{"whitespace reporter type", "host", "   "},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
					return makeReportConfig(t), func(t *testing.T, tr *Transport) {
						res := tr.Invoke(context.Background(), withBody(
							makeReportReq(tc.resourceType, tc.reporterType, "instance-001", "my-host-123", "example-host"),
							ReportResource, httpEndpoint("POST /api/kessel/v1beta2/resources")))
						Assert(t, res, requireError(codes.InvalidArgument))
					}
				})
			})
		}
	})
}
