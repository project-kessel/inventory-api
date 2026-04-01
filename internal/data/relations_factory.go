package data

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// NewRelationsRepository creates a RelationsRepository implementation based on the config.
func NewRelationsRepository(ctx context.Context, config RelationsCompletedConfig, logger *log.Helper) (model.RelationsRepository, error) {
	switch config.Impl {
	case RelationsImplAllowAll:
		return newAllowAllRelationsRepository(logger), nil
	case RelationsImplSpiceDB:
		return newSpicedbRelationsRepository(ctx, config, logger)
	default:
		return nil, fmt.Errorf("unrecognized relations.impl: %s", config.Impl)
	}
}

// CheckRelationsImpl returns a human-readable name for the active relations implementation.
func CheckRelationsImpl(config RelationsCompletedConfig) string {
	switch config.Impl {
	case RelationsImplAllowAll:
		return "AllowAll"
	case RelationsImplSpiceDB:
		return "SpiceDB"
	default:
		return "Unknown"
	}
}
