package authz

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/project-kessel/inventory-api/internal/authz/allow"
	"github.com/project-kessel/inventory-api/internal/authz/kessel"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

func New(ctx context.Context, config CompletedConfig, logger *log.Helper) (model.Authorizer, error) {
	switch config.Authz {
	case AllowAll:
		return allow.New(logger), nil
	case Kessel:
		return kessel.New(ctx, config.Kessel, logger)
	default:
		return nil, fmt.Errorf("unrecognized authz.impl: %s", config.Authz)
	}
}

func CheckAuthorizer(config CompletedConfig) string {
	var authType string
	switch config.Authz {
	case AllowAll:
		authType = "AllowAll"
	case Kessel:
		authType = "Kessel"
	default:
		authType = "Unknown"
	}
	return authType
}
