package tuples

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	"github.com/project-kessel/inventory-api/internal/data"
)

// Test helpers

func testAuthzContext() context.Context {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("test-user"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	return authnapi.NewAuthzContext(context.Background(), authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject:  claims,
	})
}

func createTestTuple() model.RelationsTuple {
	resourceId, _ := model.NewLocalResourceId("resource-1")
	subjectId, _ := model.NewLocalResourceId("subject-1")
	return model.NewRelationsTuple(
		model.NewRelationsResource(resourceId, model.NewRelationsObjectType("workspace", "rbac")),
		model.DeserializeRelation("member"),
		model.NewRelationsSubject(
			model.NewRelationsResource(subjectId, model.NewRelationsObjectType("principal", "rbac")),
			nil,
		),
	)
}

// Fakes

type recordingMetaAuthorizer struct {
	allowed   bool
	err       error
	relations []metaauthorizer.Relation
	calls     int
}

func (r *recordingMetaAuthorizer) Check(_ context.Context, _ metaauthorizer.MetaObject, relation metaauthorizer.Relation, _ authnapi.AuthzContext) (bool, error) {
	r.calls++
	r.relations = append(r.relations, relation)
	return r.allowed, r.err
}

// CreateTuples tests

func TestCreateTuples_Success(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
		Upsert: true,
	}

	result, err := uc.CreateTuples(ctx, cmd)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, meta.calls)
}

func TestCreateTuples_UsesCreateTuplesRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	_, _ = uc.CreateTuples(ctx, cmd)

	require.Len(t, meta.relations, 1)
	assert.Equal(t, metaauthorizer.RelationCreateTuples, meta.relations[0])
}

func TestCreateTuples_MetaAuthzDenied(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: false}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	_, err := uc.CreateTuples(ctx, cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestCreateTuples_MetaAuthzContextMissing(t *testing.T) {
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	_, err := uc.CreateTuples(context.Background(), cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthzContextMissing)
}

// DeleteTuples tests

func TestDeleteTuples_Success(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	namespace := "rbac"
	cmd := DeleteTuplesCommand{
		Filter: model.TupleFilter{
			ResourceNamespace: &namespace,
		},
	}

	result, err := uc.DeleteTuples(ctx, cmd)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, meta.calls)
}

func TestDeleteTuples_UsesDeleteTuplesRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	namespace := "rbac"
	cmd := DeleteTuplesCommand{
		Filter: model.TupleFilter{
			ResourceNamespace: &namespace,
		},
	}

	_, _ = uc.DeleteTuples(ctx, cmd)

	require.Len(t, meta.relations, 1)
	assert.Equal(t, metaauthorizer.RelationDeleteTuples, meta.relations[0])
}

func TestDeleteTuples_MetaAuthzDenied(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: false}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	namespace := "rbac"
	cmd := DeleteTuplesCommand{
		Filter: model.TupleFilter{
			ResourceNamespace: &namespace,
		},
	}

	_, err := uc.DeleteTuples(ctx, cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestDeleteTuples_MetaAuthzContextMissing(t *testing.T) {
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	namespace := "rbac"
	cmd := DeleteTuplesCommand{
		Filter: model.TupleFilter{
			ResourceNamespace: &namespace,
		},
	}

	_, err := uc.DeleteTuples(context.Background(), cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthzContextMissing)
}

// ReadTuples tests

func TestReadTuples_Success(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := ReadTuplesCommand{
		Filter:      model.TupleFilter{},
		Consistency: model.NewConsistencyMinimizeLatency(),
	}

	stream, err := uc.ReadTuples(ctx, cmd)

	require.NoError(t, err)
	assert.NotNil(t, stream)
	assert.Equal(t, 1, meta.calls)
}

func TestReadTuples_UsesReadTuplesRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := ReadTuplesCommand{
		Filter:      model.TupleFilter{},
		Consistency: model.NewConsistencyMinimizeLatency(),
	}

	_, _ = uc.ReadTuples(ctx, cmd)

	require.Len(t, meta.relations, 1)
	assert.Equal(t, metaauthorizer.RelationReadTuples, meta.relations[0])
}

func TestReadTuples_MetaAuthzDenied(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: false}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := ReadTuplesCommand{
		Filter:      model.TupleFilter{},
		Consistency: model.NewConsistencyMinimizeLatency(),
	}

	_, err := uc.ReadTuples(ctx, cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestReadTuples_MetaAuthzContextMissing(t *testing.T) {
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := ReadTuplesCommand{
		Filter:      model.TupleFilter{},
		Consistency: model.NewConsistencyMinimizeLatency(),
	}

	_, err := uc.ReadTuples(context.Background(), cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthzContextMissing)
}

// AcquireLock tests

func TestAcquireLock_Success(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := AcquireLockCommand{
		LockId: "lock-123",
	}

	result, err := uc.AcquireLock(ctx, cmd)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, meta.calls)
}

func TestAcquireLock_UsesAcquireLockRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := AcquireLockCommand{
		LockId: "lock-123",
	}

	_, _ = uc.AcquireLock(ctx, cmd)

	require.Len(t, meta.relations, 1)
	assert.Equal(t, metaauthorizer.RelationAcquireLock, meta.relations[0])
}

func TestAcquireLock_MetaAuthzDenied(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: false}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := AcquireLockCommand{
		LockId: "lock-123",
	}

	_, err := uc.AcquireLock(ctx, cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestAcquireLock_MetaAuthzContextMissing(t *testing.T) {
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	cmd := AcquireLockCommand{
		LockId: "lock-123",
	}

	_, err := uc.AcquireLock(context.Background(), cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthzContextMissing)
}

// WhitelistMetaAuthorizer Integration Tests
//
// These tests verify the full authorization flow with WhitelistMetaAuthorizer
// at the usecase layer, testing different authentication scenarios.

func TestWhitelistMetaAuthorizer_Integration_RBACServiceAllowed(t *testing.T) {
	// Setup: RBAC service is in allowlist
	allowlist := []string{"rbac-service"}
	meta := metaauthorizer.NewWhitelistMetaAuthorizer(allowlist)
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	// Create context with RBAC service credentials
	ctx := createOIDCAuthzContext("rbac-service", "service-account-123", authnapi.ProtocolGRPC)

	// Execute: CreateTuples should succeed
	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	result, err := uc.CreateTuples(ctx, cmd)

	// Verify: Allowed
	require.NoError(t, err, "RBAC service should be allowed")
	assert.NotNil(t, result)
}

func TestWhitelistMetaAuthorizer_Integration_UnauthorizedServiceDenied(t *testing.T) {
	// Setup: Only RBAC service in allowlist
	allowlist := []string{"rbac-service"}
	meta := metaauthorizer.NewWhitelistMetaAuthorizer(allowlist)
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	// Create context with different service credentials
	ctx := createOIDCAuthzContext("unauthorized-service", "service-account-xyz", authnapi.ProtocolGRPC)

	// Execute: CreateTuples should fail
	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	_, err := uc.CreateTuples(ctx, cmd)

	// Verify: Denied
	require.Error(t, err, "unauthorized service should be denied")
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestWhitelistMetaAuthorizer_Integration_WildcardAllowsAll(t *testing.T) {
	// Setup: Wildcard in allowlist
	allowlist := []string{"*"}
	meta := metaauthorizer.NewWhitelistMetaAuthorizer(allowlist)
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	// Create context with any service
	ctx := createOIDCAuthzContext("any-service", "any-subject", authnapi.ProtocolGRPC)

	// Execute: CreateTuples should succeed
	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	result, err := uc.CreateTuples(ctx, cmd)

	// Verify: Allowed
	require.NoError(t, err, "wildcard should allow all")
	assert.NotNil(t, result)
}

func TestWhitelistMetaAuthorizer_Integration_EmptyAllowlistDeniesAll(t *testing.T) {
	// Setup: Empty allowlist (default secure behavior)
	allowlist := []string{}
	meta := metaauthorizer.NewWhitelistMetaAuthorizer(allowlist)
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	// Create context with RBAC service
	ctx := createOIDCAuthzContext("rbac-service", "service-account-123", authnapi.ProtocolGRPC)

	// Execute: CreateTuples should fail
	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	_, err := uc.CreateTuples(ctx, cmd)

	// Verify: Denied (fail-closed)
	require.Error(t, err, "empty allowlist should deny all")
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestWhitelistMetaAuthorizer_Integration_HTTPProtocolDenied(t *testing.T) {
	// Setup: Wildcard allowlist
	allowlist := []string{"*"}
	meta := metaauthorizer.NewWhitelistMetaAuthorizer(allowlist)
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	// Create context with HTTP protocol (should be rejected)
	ctx := createOIDCAuthzContext("rbac-service", "service-account-123", authnapi.ProtocolHTTP)

	// Execute: CreateTuples should fail
	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	_, err := uc.CreateTuples(ctx, cmd)

	// Verify: Denied (gRPC only)
	require.Error(t, err, "HTTP protocol should be denied (gRPC only)")
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestWhitelistMetaAuthorizer_Integration_XRhIdentityDenied(t *testing.T) {
	// Setup: Wildcard allowlist
	allowlist := []string{"*"}
	meta := metaauthorizer.NewWhitelistMetaAuthorizer(allowlist)
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	// Create context with x-rh-identity auth (should be rejected)
	ctx := createXRhIdentityAuthzContext("user@redhat.com")

	// Execute: CreateTuples should fail
	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	_, err := uc.CreateTuples(ctx, cmd)

	// Verify: Denied (OIDC only)
	require.Error(t, err, "x-rh-identity should be denied (OIDC only)")
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestWhitelistMetaAuthorizer_Integration_UnauthenticatedDenied(t *testing.T) {
	// Setup: Wildcard allowlist
	allowlist := []string{"*"}
	meta := metaauthorizer.NewWhitelistMetaAuthorizer(allowlist)
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	// Create context without authentication
	ctx := createUnauthenticatedAuthzContext()

	// Execute: CreateTuples should fail
	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	_, err := uc.CreateTuples(ctx, cmd)

	// Verify: Denied
	require.Error(t, err, "unauthenticated should be denied")
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestWhitelistMetaAuthorizer_Integration_EmptyClientID_Denies(t *testing.T) {
	// Setup: Allowlist with service name
	allowlist := []string{"service-account-fallback"}
	meta := metaauthorizer.NewWhitelistMetaAuthorizer(allowlist)
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	// Create context with SubjectId but no ClientID
	ctx := createOIDCAuthzContextWithoutClientID("service-account-fallback")

	// Execute: CreateTuples should fail (empty ClientID is denied)
	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	result, err := uc.CreateTuples(ctx, cmd)

	// Verify: Denied (empty ClientID is not allowed)
	require.Error(t, err, "should deny when ClientID is empty")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "meta authorization denied")
}

func TestWhitelistMetaAuthorizer_Integration_AllOperations(t *testing.T) {
	// Setup: RBAC service in allowlist
	allowlist := []string{"rbac-service"}
	meta := metaauthorizer.NewWhitelistMetaAuthorizer(allowlist)
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	// Create context with RBAC service credentials
	ctx := createOIDCAuthzContext("rbac-service", "service-account-123", authnapi.ProtocolGRPC)

	// Test all tuple operations
	t.Run("CreateTuples", func(t *testing.T) {
		cmd := CreateTuplesCommand{Tuples: []model.RelationsTuple{createTestTuple()}}
		_, err := uc.CreateTuples(ctx, cmd)
		require.NoError(t, err)
	})

	t.Run("DeleteTuples", func(t *testing.T) {
		namespace := "rbac"
		cmd := DeleteTuplesCommand{Filter: model.TupleFilter{ResourceNamespace: &namespace}}
		_, err := uc.DeleteTuples(ctx, cmd)
		require.NoError(t, err)
	})

	t.Run("ReadTuples", func(t *testing.T) {
		cmd := ReadTuplesCommand{
			Filter:      model.TupleFilter{},
			Consistency: model.NewConsistencyMinimizeLatency(),
		}
		_, err := uc.ReadTuples(ctx, cmd)
		require.NoError(t, err)
	})

	t.Run("AcquireLock", func(t *testing.T) {
		cmd := AcquireLockCommand{LockId: "lock-123"}
		_, err := uc.AcquireLock(ctx, cmd)
		require.NoError(t, err)
	})
}

func TestWhitelistMetaAuthorizer_Integration_MultipleServicesAllowlist(t *testing.T) {
	// Setup: Multiple services in allowlist
	allowlist := []string{"rbac-service", "another-service", "third-service"}
	meta := metaauthorizer.NewWhitelistMetaAuthorizer(allowlist)
	uc := New(&data.AllowAllRelationsRepository{}, meta, log.DefaultLogger)

	// Test each service is allowed
	services := []string{"rbac-service", "another-service", "third-service"}
	for _, service := range services {
		t.Run(service, func(t *testing.T) {
			ctx := createOIDCAuthzContext(service, "service-account-"+service, authnapi.ProtocolGRPC)
			cmd := CreateTuplesCommand{Tuples: []model.RelationsTuple{createTestTuple()}}
			_, err := uc.CreateTuples(ctx, cmd)
			require.NoError(t, err, "%s should be allowed", service)
		})
	}

	// Test unauthorized service is denied
	t.Run("unauthorized-service", func(t *testing.T) {
		ctx := createOIDCAuthzContext("unauthorized-service", "service-account-bad", authnapi.ProtocolGRPC)
		cmd := CreateTuplesCommand{Tuples: []model.RelationsTuple{createTestTuple()}}
		_, err := uc.CreateTuples(ctx, cmd)
		require.Error(t, err, "unauthorized service should be denied")
		assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
	})
}

// Test helper functions for creating different authentication contexts

func createOIDCAuthzContext(clientID, subjectID string, protocol authnapi.Protocol) context.Context {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId(subjectID),
		ClientID:  authnapi.ClientID(clientID),
		AuthType:  authnapi.AuthTypeOIDC,
		Issuer:    authnapi.Issuer("https://sso.redhat.com"),
	}
	return authnapi.NewAuthzContext(context.Background(), authnapi.AuthzContext{
		Protocol: protocol,
		Subject:  claims,
	})
}

func createOIDCAuthzContextWithoutClientID(subjectID string) context.Context {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId(subjectID),
		ClientID:  "", // No ClientID - tests SubjectId fallback
		AuthType:  authnapi.AuthTypeOIDC,
		Issuer:    authnapi.Issuer("https://sso.redhat.com"),
	}
	return authnapi.NewAuthzContext(context.Background(), authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject:  claims,
	})
}

func createXRhIdentityAuthzContext(userID string) context.Context {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId(userID),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	return authnapi.NewAuthzContext(context.Background(), authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject:  claims,
	})
}

func createUnauthenticatedAuthzContext() context.Context {
	claims := &authnapi.Claims{
		AuthType: authnapi.AuthTypeAllowUnauthenticated,
	}
	return authnapi.NewAuthzContext(context.Background(), authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject:  claims,
	})
}
