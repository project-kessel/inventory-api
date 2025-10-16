package schemas

import (
	"context"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/schemas/api"
	"github.com/project-kessel/inventory-api/internal/schemas/in_memory"
)

func New(ctx context.Context, c CompletedConfig, logger *log.Helper) (api.SchemaService, error) {
	repository, err := newRepository(ctx, c, logger)
	if err != nil {
		return nil, err
	}

	return NewSchemaService(repository), nil
}

func newRepository(ctx context.Context, c CompletedConfig, logger *log.Helper) (api.SchemaRepository, error) {
	switch c.Repository {
	case InMemoryRepository:
		switch c.InMemory.Type {
		case in_memory.EmptyRepository:
			return in_memory.New(ctx), nil
		case in_memory.JSONRepository:
			return in_memory.NewFromJsonFile(ctx, c.InMemory.Path)
		case in_memory.DirRepository:
			return in_memory.NewFromDir(ctx, c.InMemory.Path)
		}
	}

	return nil, fmt.Errorf("invalid repository type: %s", c.Repository)
}
