package data

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/config/relations"
)

func NewRelationsRepository(ctx context.Context, config relations.CompletedConfig, logger log.Logger) (model.RelationsRepository, error) {
	helper := log.NewHelper(logger)
	switch config.Authz {
	case relations.AllowAll:
		return NewAllowAllRelationsRepository(helper), nil
	case relations.Kessel:
		return NewGRPCRelationsRepository(ctx, config.Kessel, helper)
	case relations.SpiceDB:
		repo, _, err := NewSpiceDBRelationsRepository(NewSpiceDBConfigFromCompleted(config.SpiceDB), logger)
		if err != nil {
			return nil, fmt.Errorf("error creating spicedb relations repository: %w", err)
		}
		return repo, nil
	default:
		return nil, fmt.Errorf("unrecognized relations implementation: %s", config.Authz)
	}
}
