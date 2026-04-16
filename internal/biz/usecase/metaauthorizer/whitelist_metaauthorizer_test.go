package metaauthorizer

import (
	"context"
	"testing"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/stretchr/testify/assert"
)

func TestWhitelistMetaAuthorizer_ClientID_Allowed(t *testing.T) {
	authorizer := NewWhitelistMetaAuthorizer([]string{"rbac-service", "another-service"})
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject: &authnapi.Claims{
			SubjectId: "service-account-abc123",
			ClientID:  "rbac-service",
			AuthType:  authnapi.AuthTypeOIDC,
		},
	}

	allowed, err := authorizer.Check(ctx, NewTupleSystem(), RelationCreateTuples, authzCtx)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestWhitelistMetaAuthorizer_ClientID_Denied(t *testing.T) {
	authorizer := NewWhitelistMetaAuthorizer([]string{"rbac-service"})
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject: &authnapi.Claims{
			SubjectId: "service-account-xyz789",
			ClientID:  "unknown-service",
			AuthType:  authnapi.AuthTypeOIDC,
		},
	}

	allowed, err := authorizer.Check(ctx, NewTupleSystem(), RelationCreateTuples, authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestWhitelistMetaAuthorizer_EmptyClientID_Denies(t *testing.T) {
	authorizer := NewWhitelistMetaAuthorizer([]string{"service-account-fallback"})
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject: &authnapi.Claims{
			SubjectId: "service-account-fallback",
			ClientID:  "", // Empty ClientID should be denied
			AuthType:  authnapi.AuthTypeOIDC,
		},
	}

	allowed, err := authorizer.Check(ctx, NewTupleSystem(), RelationCreateTuples, authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed) // Empty ClientID is denied
}

func TestWhitelistMetaAuthorizer_Wildcard(t *testing.T) {
	authorizer := NewWhitelistMetaAuthorizer([]string{"*"})
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject: &authnapi.Claims{
			SubjectId: "any-subject",
			ClientID:  "any-client",
			AuthType:  authnapi.AuthTypeOIDC,
		},
	}

	allowed, err := authorizer.Check(ctx, NewTupleSystem(), RelationCreateTuples, authzCtx)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestWhitelistMetaAuthorizer_EmptyAllowlist_Denies(t *testing.T) {
	authorizer := NewWhitelistMetaAuthorizer([]string{})
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject: &authnapi.Claims{
			SubjectId: "service-account-abc",
			ClientID:  "rbac-service",
			AuthType:  authnapi.AuthTypeOIDC,
		},
	}

	allowed, err := authorizer.Check(ctx, NewTupleSystem(), RelationCreateTuples, authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestWhitelistMetaAuthorizer_Unauthenticated_Denies(t *testing.T) {
	authorizer := NewWhitelistMetaAuthorizer([]string{"*"})
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject: &authnapi.Claims{
			AuthType: authnapi.AuthTypeAllowUnauthenticated,
		},
	}

	allowed, err := authorizer.Check(ctx, NewTupleSystem(), RelationCreateTuples, authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestWhitelistMetaAuthorizer_HTTP_Denies(t *testing.T) {
	authorizer := NewWhitelistMetaAuthorizer([]string{"*"})
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolHTTP,
		Subject: &authnapi.Claims{
			ClientID: "rbac-service",
			AuthType: authnapi.AuthTypeOIDC,
		},
	}

	allowed, err := authorizer.Check(ctx, NewTupleSystem(), RelationCreateTuples, authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed, "should deny HTTP connections even with wildcard")
}

func TestWhitelistMetaAuthorizer_XRhIdentity_Denies(t *testing.T) {
	authorizer := NewWhitelistMetaAuthorizer([]string{"*"})
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject: &authnapi.Claims{
			SubjectId: "user@redhat.com",
			AuthType:  authnapi.AuthTypeXRhIdentity,
		},
	}

	allowed, err := authorizer.Check(ctx, NewTupleSystem(), RelationCreateTuples, authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestWhitelistMetaAuthorizer_RelationIndependent(t *testing.T) {
	// Verify that the authorizer doesn't filter based on relation
	authorizer := NewWhitelistMetaAuthorizer([]string{"rbac-service"})
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject: &authnapi.Claims{
			ClientID: "rbac-service",
			AuthType: authnapi.AuthTypeOIDC,
		},
	}

	// Should allow for all tuple relations
	for _, relation := range []Relation{
		RelationCreateTuples,
		RelationDeleteTuples,
		RelationReadTuples,
		RelationAcquireLock,
	} {
		allowed, err := authorizer.Check(ctx, NewTupleSystem(), relation, authzCtx)
		assert.NoError(t, err)
		assert.True(t, allowed, "should allow relation %s", relation)
	}
}
