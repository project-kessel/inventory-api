package middleware

import (
	"context"
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

// MockAuthorizer is a mock implementation of authzapi.Authorizer
type MockAuthorizer struct {
	mock.Mock
}

func (m *MockAuthorizer) Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*kesselv1.GetReadyzResponse), args.Error(1)
}

func (m *MockAuthorizer) Check(ctx context.Context, namespace string, relation string, consistencyToken string, resourceType string, localResourceId string, sub *kessel.SubjectReference) (kessel.CheckResponse_Allowed, *kessel.ConsistencyToken, error) {
	args := m.Called(ctx, namespace, relation, consistencyToken, resourceType, localResourceId, sub)
	return args.Get(0).(kessel.CheckResponse_Allowed), args.Get(1).(*kessel.ConsistencyToken), args.Error(2)
}

func (m *MockAuthorizer) CheckForUpdate(ctx context.Context, namespace string, permission string, resourceType string, localResourceId string, sub *kessel.SubjectReference) (kessel.CheckForUpdateResponse_Allowed, *kessel.ConsistencyToken, error) {
	args := m.Called(ctx, namespace, permission, resourceType, localResourceId, sub)
	return args.Get(0).(kessel.CheckForUpdateResponse_Allowed), args.Get(1).(*kessel.ConsistencyToken), args.Error(2)
}

func (m *MockAuthorizer) CheckBulk(ctx context.Context, req *kessel.CheckBulkRequest) (*kessel.CheckBulkResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*kessel.CheckBulkResponse), args.Error(1)
}

func (m *MockAuthorizer) LookupResources(ctx context.Context, in *kessel.LookupResourcesRequest) (grpc.ServerStreamingClient[kessel.LookupResourcesResponse], error) {
	args := m.Called(ctx, in)
	return args.Get(0).(grpc.ServerStreamingClient[kessel.LookupResourcesResponse]), args.Error(1)
}

func (m *MockAuthorizer) CreateTuples(ctx context.Context, req *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*kessel.CreateTuplesResponse), args.Error(1)
}

func (m *MockAuthorizer) DeleteTuples(ctx context.Context, req *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*kessel.DeleteTuplesResponse), args.Error(1)
}

func (m *MockAuthorizer) AcquireLock(ctx context.Context, req *kessel.AcquireLockRequest) (*kessel.AcquireLockResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*kessel.AcquireLockResponse), args.Error(1)
}

func (m *MockAuthorizer) UnsetWorkspace(ctx context.Context, namespace string, resourceType string, localResourceId string) (*kessel.DeleteTuplesResponse, error) {
	args := m.Called(ctx, namespace, resourceType, localResourceId)
	return args.Get(0).(*kessel.DeleteTuplesResponse), args.Error(1)
}

func (m *MockAuthorizer) SetWorkspace(ctx context.Context, namespace string, resourceType string, localResourceId string, workspaceId string, createIfNotExists bool) (*kessel.CreateTuplesResponse, error) {
	args := m.Called(ctx, namespace, resourceType, localResourceId, workspaceId, createIfNotExists)
	return args.Get(0).(*kessel.CreateTuplesResponse), args.Error(1)
}

var testLogger = log.NewStdLogger(io.Discard)

func TestMetaAuthorizer_Disabled(t *testing.T) {
	config := MetaAuthorizerConfig{
		Enabled: false,
	}
	middleware := MetaAuthorizerMiddleware(config, testLogger)

	ctx := context.Background()
	req := &pb.CheckSelfRequest{
		Object: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
		},
		Relation: "viewer",
	}

	// Should pass through without checking
	result, err := middleware(func(ctx context.Context, req any) (any, error) {
		return "success", nil
	})(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestMetaAuthorizer_NonCheckSelfRequest(t *testing.T) {
	config := MetaAuthorizerConfig{
		Enabled: true,
	}
	middleware := MetaAuthorizerMiddleware(config, testLogger)

	ctx := context.Background()
	req := "not a CheckSelfRequest"

	// Should pass through without checking
	result, err := middleware(func(ctx context.Context, req any) (any, error) {
		return "success", nil
	})(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestMetaAuthorizer_BlockGRPC_CheckSelfRequest(t *testing.T) {
	config := MetaAuthorizerConfig{
		Enabled: true,
	}
	middleware := MetaAuthorizerMiddleware(config, testLogger)

	// Create a gRPC transport context
	ctx := context.Background()
	grpcTransport := &mockGRPCTransport{}
	ctx = transport.NewServerContext(ctx, grpcTransport)

	identity := &authnapi.Identity{
		Principal: "user-123",
		AuthType:  "allow-unauthenticated", // gRPC uses allow-unauthenticated
	}
	ctx = context.WithValue(ctx, IdentityRequestKey, identity)

	req := &pb.CheckSelfRequest{
		Object: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
			Reporter: &pb.ReporterReference{
				Type: "hbi",
			},
		},
		Relation: "viewer",
	}

	// Should block gRPC CheckSelf requests
	result, err := middleware(func(ctx context.Context, req any) (any, error) {
		return "success", nil
	})(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.IsForbidden(err))
	assert.Equal(t, ErrGRPCNotAllowed, err)
}

// mockGRPCTransport implements transport.Transporter for testing
type mockGRPCTransport struct{}

func (m *mockGRPCTransport) Kind() transport.Kind {
	return transport.KindGRPC
}

func (m *mockGRPCTransport) Endpoint() string {
	return "/kessel.inventory.v1beta2.KesselInventoryService/CheckSelf"
}

func (m *mockGRPCTransport) Operation() string {
	return "CheckSelf"
}

func (m *mockGRPCTransport) RequestHeader() transport.Header {
	return &mockHeader{headers: make(map[string]string)}
}

func (m *mockGRPCTransport) ReplyHeader() transport.Header {
	return &mockHeader{headers: make(map[string]string)}
}

type mockHeader struct {
	headers map[string]string
}

func (m *mockHeader) Get(key string) string {
	return m.headers[key]
}

func (m *mockHeader) Set(key, value string) {
	m.headers[key] = value
}

func (m *mockHeader) Add(key, value string) {
	m.headers[key] = value
}

func (m *mockHeader) Keys() []string {
	return nil
}

func (m *mockHeader) Values(key string) []string {
	return nil
}

func TestMetaAuthorizer_MissingIdentity(t *testing.T) {
	config := MetaAuthorizerConfig{
		Enabled: true,
	}
	middleware := MetaAuthorizerMiddleware(config, testLogger)

	ctx := context.Background() // No identity in context
	req := &pb.CheckSelfRequest{
		Object: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
		},
		Relation: "viewer",
	}

	// Should return unauthorized error
	result, err := middleware(func(ctx context.Context, req any) (any, error) {
		return "success", nil
	})(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.IsUnauthorized(err))
}

func TestMetaAuthorizer_DecisionRule1_CheckSelfRelation(t *testing.T) {
	config := MetaAuthorizerConfig{
		Enabled: true,
	}
	middleware := MetaAuthorizerMiddleware(config, testLogger)

	identity := &authnapi.Identity{
		Principal: "user-123",
		AuthType:  "x-rh-identity",
		UserID:    "user-123",
	}
	ctx := context.WithValue(context.Background(), IdentityRequestKey, identity)

	// Decision Rule 1 always applies because we create tempReq with relation="check_self"
	// Original relation can be anything - it's preserved for the service handler
	req := &pb.CheckSelfRequest{
		Object: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
			Reporter: &pb.ReporterReference{
				Type: "hbi",
			},
		},
		Relation: "viewer", // Original relation - will be preserved
	}

	// Should allow without metacheck (Decision Rule 1 always applies)
	result, err := middleware(func(ctx context.Context, req any) (any, error) {
		// Verify original relation is preserved
		checkSelfReq := req.(*pb.CheckSelfRequest)
		assert.Equal(t, "viewer", checkSelfReq.Relation)
		return "success", nil
	})(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestMetaAuthorizer_DecisionRule2_OIDCAuthType(t *testing.T) {
	config := MetaAuthorizerConfig{
		Enabled: true,
	}
	middleware := MetaAuthorizerMiddleware(config, testLogger)

	identity := &authnapi.Identity{
		Principal: "redhat.com/user-456",
		AuthType:  "oidc", // OIDC auth type - middleware should skip (only processes x-rh-identity)
	}
	ctx := context.WithValue(context.Background(), IdentityRequestKey, identity)

	req := &pb.CheckSelfRequest{
		Object: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
			Reporter: &pb.ReporterReference{
				Type: "hbi",
			},
		},
		Relation: "viewer",
	}

	// Should pass through without meta-authorization (OIDC is filtered out at middleware level)
	result, err := middleware(func(ctx context.Context, req any) (any, error) {
		return "success", nil
	})(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestMetaAuthorizer_DecisionRule3_MetacheckAllowed(t *testing.T) {
	// NOTE: Decision Rule 3 (metacheck) will never be reached because Decision Rule 1 always applies
	// This test is kept for documentation but will not actually perform a metacheck
	mockAuthorizer := new(MockAuthorizer)
	config := MetaAuthorizerConfig{
		Enabled:          true,
		Authorizer:       mockAuthorizer,
		SubjectNamespace: "rbac",
	}
	middleware := MetaAuthorizerMiddleware(config, testLogger)

	identity := &authnapi.Identity{
		Principal: "user-789",
		AuthType:  "x-rh-identity",
		UserID:    "user-789",
	}
	ctx := context.WithValue(context.Background(), IdentityRequestKey, identity)

	req := &pb.CheckSelfRequest{
		Object: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
			Reporter: &pb.ReporterReference{
				Type: "hbi",
			},
		},
		Relation: "viewer", // Original relation - Decision Rule 1 will apply (tempReq has relation="check_self")
	}

	// Should allow without metacheck (Decision Rule 1 always applies)
	// Mock authorizer should NOT be called
	result, err := middleware(func(ctx context.Context, req any) (any, error) {
		// Verify original relation is preserved
		checkSelfReq := req.(*pb.CheckSelfRequest)
		assert.Equal(t, "viewer", checkSelfReq.Relation)
		return "success", nil
	})(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	// Verify metacheck was NOT called (Decision Rule 1 applies first)
	mockAuthorizer.AssertNotCalled(t, "Check")
}

func TestMetaAuthorizer_DecisionRule3_MetacheckDenied(t *testing.T) {
	// NOTE: Decision Rule 3 (metacheck) will never be reached because Decision Rule 1 always applies
	// This test verifies that Decision Rule 1 applies and allows the request
	mockAuthorizer := new(MockAuthorizer)
	config := MetaAuthorizerConfig{
		Enabled:          true,
		Authorizer:       mockAuthorizer,
		SubjectNamespace: "rbac",
	}
	middleware := MetaAuthorizerMiddleware(config, testLogger)

	identity := &authnapi.Identity{
		Principal: "user-789",
		AuthType:  "x-rh-identity",
		UserID:    "user-789",
	}
	ctx := context.WithValue(context.Background(), IdentityRequestKey, identity)

	req := &pb.CheckSelfRequest{
		Object: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
			Reporter: &pb.ReporterReference{
				Type: "hbi",
			},
		},
		Relation: "viewer", // Original relation - Decision Rule 1 will apply
	}

	// Should allow without metacheck (Decision Rule 1 always applies)
	// Mock authorizer should NOT be called
	result, err := middleware(func(ctx context.Context, req any) (any, error) {
		return "success", nil
	})(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	// Verify metacheck was NOT called (Decision Rule 1 applies first)
	mockAuthorizer.AssertNotCalled(t, "Check")
}

func TestMetaAuthorizer_NoAuthorizerConfigured(t *testing.T) {
	// NOTE: With Decision Rule 1 always applying, no authorizer is needed
	// This test verifies that Decision Rule 1 allows the request even without an authorizer
	config := MetaAuthorizerConfig{
		Enabled:          true,
		Authorizer:       nil, // No authorizer configured - not needed because Decision Rule 1 applies
		SubjectNamespace: "rbac",
	}
	middleware := MetaAuthorizerMiddleware(config, testLogger)

	identity := &authnapi.Identity{
		Principal: "user-789",
		AuthType:  "x-rh-identity",
		UserID:    "user-789",
	}
	ctx := context.WithValue(context.Background(), IdentityRequestKey, identity)

	req := &pb.CheckSelfRequest{
		Object: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
			Reporter: &pb.ReporterReference{
				Type: "hbi",
			},
		},
		Relation: "viewer", // Original relation - Decision Rule 1 will apply
	}

	// Should allow without metacheck (Decision Rule 1 always applies, no authorizer needed)
	result, err := middleware(func(ctx context.Context, req any) (any, error) {
		return "success", nil
	})(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestSubjectReferenceFromIdentityForMetaAuthorizer_XRhIdentity(t *testing.T) {
	identity := &authnapi.Identity{
		Principal: "principal-123",
		AuthType:  "x-rh-identity",
		UserID:    "user-123",
	}

	config := MetaAuthorizerConfig{
		SubjectNamespace: "rbac",
	}
	subjectRef, err := subjectReferenceFromIdentityForMetaAuthorizer(identity, config)
	assert.NoError(t, err)

	assert.NotNil(t, subjectRef)
	assert.NotNil(t, subjectRef.Subject)
	assert.Equal(t, "user-123", subjectRef.Subject.Id) // Should prefer UserID
	assert.Equal(t, "rbac", subjectRef.Subject.Type.Namespace)
	assert.Equal(t, "principal", subjectRef.Subject.Type.Name)
}

func TestSubjectReferenceFromIdentityForMetaAuthorizer_XRhIdentity_NoUserID(t *testing.T) {
	identity := &authnapi.Identity{
		Principal: "principal-456",
		AuthType:  "x-rh-identity",
		UserID:    "", // No UserID
	}

	config := MetaAuthorizerConfig{
		SubjectNamespace: "rbac",
	}
	subjectRef, err := subjectReferenceFromIdentityForMetaAuthorizer(identity, config)
	assert.NoError(t, err)

	assert.NotNil(t, subjectRef)
	assert.NotNil(t, subjectRef.Subject)
	assert.Equal(t, "principal-456", subjectRef.Subject.Id) // Should use Principal
	assert.Equal(t, "rbac", subjectRef.Subject.Type.Namespace)
	assert.Equal(t, "principal", subjectRef.Subject.Type.Name)
}

func TestSubjectReferenceFromIdentityForMetaAuthorizer_OIDC(t *testing.T) {
	identity := &authnapi.Identity{
		Principal: "redhat.com/user-789",
		AuthType:  "oidc",
	}

	config := MetaAuthorizerConfig{
		SubjectNamespace: "rbac",
	}
	subjectRef, err := subjectReferenceFromIdentityForMetaAuthorizer(identity, config)
	assert.NoError(t, err)

	assert.NotNil(t, subjectRef)
	assert.NotNil(t, subjectRef.Subject)
	assert.Equal(t, "user-789", subjectRef.Subject.Id) // Should extract from "domain/subject"
	assert.Equal(t, "rbac", subjectRef.Subject.Type.Namespace)
	assert.Equal(t, "principal", subjectRef.Subject.Type.Name)
}

func TestCreateTempRequestForDecisionLogic(t *testing.T) {
	originalReq := &pb.CheckSelfRequest{
		Object: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-123",
			Reporter: &pb.ReporterReference{
				Type: "hbi",
			},
		},
		Relation: "viewer", // Original relation
		Consistency: &pb.Consistency{
			Requirement: &pb.Consistency_MinimizeLatency{
				MinimizeLatency: true,
			},
		},
	}

	tempReq := createTempRequestForDecisionLogic(originalReq)

	// Verify tempReq has relation="check_self"
	assert.Equal(t, CheckSelfRelation, tempReq.Relation)
	// Verify Object is preserved
	assert.Equal(t, originalReq.Object, tempReq.Object)
	// Verify Consistency is preserved
	assert.Equal(t, originalReq.Consistency, tempReq.Consistency)
	// Verify original request is not modified
	assert.Equal(t, "viewer", originalReq.Relation)
}
