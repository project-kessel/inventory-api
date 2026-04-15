package data

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/config/relations"
)

func NewRelationsRepository(ctx context.Context, config relations.CompletedConfig, logger *log.Helper) (model.RelationsRepository, error) {
	switch config.Authz {
	case relations.AllowAll:
		return NewAllowAllRelationsRepository(logger), nil
	case relations.Kessel:
		return NewGRPCRelationsRepository(ctx, config.Kessel, logger)
	default:
		return nil, fmt.Errorf("unrecognized relations implementation: %s", config.Authz)
	}
}
