package authz

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/project-kessel/inventory-api/internal/authz/allow"
	"github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/authz/spicedb"
)

func New(ctx context.Context, config CompletedConfig, logger *log.Helper) (api.Authorizer, error) {
	switch config.Authz {
	case AllowAll:
		return allow.New(logger), nil
	case SpiceDB:
		return spicedb.NewSpiceDbRepository(config.SpiceDB, logger)
	default:
		return nil, fmt.Errorf("unrecognized authz.impl: %s (valid options: allow-all, spicedb)", config.Authz)
	}
}

func CheckAuthorizer(config CompletedConfig) string {
	var authType string
	switch config.Authz {
	case AllowAll:
		authType = "AllowAll"
	case SpiceDB:
		authType = "SpiceDB"
	default:
		authType = "Unknown"
	}
	return authType
}
