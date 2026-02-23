package metaauthorizer

import (
	"context"
	"testing"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/stretchr/testify/assert"
)

func TestSimpleMetaAuthorizer_HTTP_XRhIdentity(t *testing.T) {
	authorizer := NewSimpleMetaAuthorizer()
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolHTTP,
		Subject:  &authnapi.Claims{AuthType: authnapi.AuthTypeXRhIdentity},
	}

	allowed, err := authorizer.Check(ctx, InventoryResource{}, RelationCheckSelf, authzCtx)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = authorizer.Check(ctx, InventoryResource{}, Relation("view"), authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestSimpleMetaAuthorizer_GRPC(t *testing.T) {
	authorizer := NewSimpleMetaAuthorizer()
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{Protocol: authnapi.ProtocolGRPC}

	allowed, err := authorizer.Check(ctx, InventoryResource{}, RelationCheckSelf, authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)

	allowed, err = authorizer.Check(ctx, InventoryResource{}, Relation("view"), authzCtx)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestSimpleMetaAuthorizer_HTTP_OIDC_Denies(t *testing.T) {
	authorizer := NewSimpleMetaAuthorizer()
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolHTTP,
		Subject:  &authnapi.Claims{AuthType: authnapi.AuthTypeOIDC},
	}

	allowed, err := authorizer.Check(ctx, InventoryResource{}, RelationCheckSelf, authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)

	allowed, err = authorizer.Check(ctx, InventoryResource{}, Relation("view"), authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestSimpleMetaAuthorizer_GRPC_OverridesXRhIdentity(t *testing.T) {
	authorizer := NewSimpleMetaAuthorizer()
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject:  &authnapi.Claims{AuthType: authnapi.AuthTypeXRhIdentity},
	}

	allowed, err := authorizer.Check(ctx, InventoryResource{}, RelationCheckSelf, authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestSimpleMetaAuthorizer_UnknownProtocol_Denies(t *testing.T) {
	authorizer := NewSimpleMetaAuthorizer()
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{
		Protocol: authnapi.ProtocolUnknown,
		Subject:  &authnapi.Claims{AuthType: authnapi.AuthTypeXRhIdentity},
	}

	allowed, err := authorizer.Check(ctx, InventoryResource{}, RelationCheckSelf, authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)
}
