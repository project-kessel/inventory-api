package authz

import (
	"context"
	"fmt"

	"github.com/project-kessel/inventory-api/internal/authz/allow"
	"github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/authz/kessel"
)

func New(ctx context.Context, config CompletedConfig) (api.Authorizer, error) {

	switch config.Authz {
	case AllowAll:
		return allow.New(), nil
	case Kessel:
		return kessel.New(ctx, config.Kessel)
	default:
		return nil, fmt.Errorf("Unrecognized authz.impl: %s", config.Authz)
	}
}
