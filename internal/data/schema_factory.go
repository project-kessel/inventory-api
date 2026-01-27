package data

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	schemaConfig "github.com/project-kessel/inventory-api/internal/config/schema"
	inmemoryConfig "github.com/project-kessel/inventory-api/internal/config/schema/inmemory"
)

func NewSchemaRepository(ctx context.Context, c schemaConfig.CompletedConfig, logger *log.Helper) (model.SchemaRepository, error) {
	switch c.Repository {
	case schemaConfig.InMemoryRepository:
		switch c.InMemory.Type {
		case inmemoryConfig.EmptyRepository:
			return NewInMemorySchemaRepository(), nil
		case inmemoryConfig.JSONRepository:
			return NewInMemorySchemaRepositoryFromJsonFile(ctx, c.InMemory.Path, NewJsonSchemaWithWorkspacesFromString)
		case inmemoryConfig.DirRepository:
			return NewInMemorySchemaRepositoryFromDir(ctx, c.InMemory.Path, NewJsonSchemaWithWorkspacesFromString)
		default:
			return nil, fmt.Errorf("invalid repository type: %s/%s", c.Repository, c.InMemory.Type)
		}
	}

	return nil, fmt.Errorf("invalid repository type: %s", c.Repository)
}
