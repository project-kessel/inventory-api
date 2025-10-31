package schema

import (
	"context"
	"fmt"

	"github.com/project-kessel/inventory-api/internal/schema/validation"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/schema/api"
	"github.com/project-kessel/inventory-api/internal/schema/in_memory"
)

func New(ctx context.Context, c CompletedConfig, logger *log.Helper) (*SchemaService, error) {
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
			return in_memory.New(), nil
		case in_memory.JSONRepository:
			return in_memory.NewFromJsonFile(ctx, c.InMemory.Path, validation.NewJsonSchemaValidatorFromString)
		case in_memory.DirRepository:
			return in_memory.NewFromDir(ctx, c.InMemory.Path, validation.NewJsonSchemaValidatorFromString)
		}
	}

	return nil, fmt.Errorf("invalid repository type: %s", c.Repository)
}
