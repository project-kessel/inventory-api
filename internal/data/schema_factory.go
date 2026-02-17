package data

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	schemaConfig "github.com/project-kessel/inventory-api/internal/config/schema"
	inmemoryConfig "github.com/project-kessel/inventory-api/internal/config/schema/inmemory"
)

func NewSchemaRepository(ctx context.Context, c schemaConfig.CompletedConfig, logger *log.Helper) (bizmodel.SchemaRepository, error) {
	switch c.Repository {
	case schemaConfig.InMemoryRepository:
		switch c.InMemory.Type {
		case inmemoryConfig.EmptyRepository:
			logger.Infof("Using empty in-memory schema repository")
			return NewInMemorySchemaRepository(), nil
		case inmemoryConfig.JSONRepository:
			logger.Infof("Using json in-memory schema repository from path %q", c.InMemory.Path)
			return NewInMemorySchemaRepositoryFromJsonFile(ctx, c.InMemory.Path, bizmodel.NewJsonSchemaValidatorFromString)
		case inmemoryConfig.DirRepository:
			logger.Infof("Using dir in-memory schema repository from path %q", c.InMemory.Path)
			return NewInMemorySchemaRepositoryFromDir(ctx, c.InMemory.Path, bizmodel.NewJsonSchemaValidatorFromString)
		default:
			return nil, fmt.Errorf("invalid repository type: %s/%s", c.Repository, c.InMemory.Type)
		}
	}

	return nil, fmt.Errorf("invalid repository type: %s", c.Repository)
}
