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
		Identity: &authnapi.Identity{AuthType: "x-rh-identity"},
	}

	allowed, err := authorizer.Check(ctx, nil, nil, RelationCheckSelf, authzCtx)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = authorizer.Check(ctx, nil, nil, "view", authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestSimpleMetaAuthorizer_GRPC(t *testing.T) {
	authorizer := NewSimpleMetaAuthorizer()
	ctx := context.Background()
	authzCtx := authnapi.AuthzContext{Protocol: authnapi.ProtocolGRPC}

	allowed, err := authorizer.Check(ctx, nil, nil, RelationCheckSelf, authzCtx)
	assert.NoError(t, err)
	assert.False(t, allowed)

	allowed, err = authorizer.Check(ctx, nil, nil, "view", authzCtx)
	assert.NoError(t, err)
	assert.True(t, allowed)
}
