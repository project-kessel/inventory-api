package provider

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
)

// NewSchemaRepository creates a new SchemaRepository from options.
func NewSchemaRepository(ctx context.Context, opts *SchemaOptions, logger *log.Helper) (model.SchemaRepository, error) {
	if errs := opts.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("schema validation failed: %v", errs)
	}

	switch opts.Repository {
	case SchemaRepositoryInMemory:
		return newInMemorySchemaRepository(ctx, opts.InMemory, logger)
	default:
		return nil, fmt.Errorf("invalid repository type: %s", opts.Repository)
	}
}

// newInMemorySchemaRepository creates an in-memory schema repository.
func newInMemorySchemaRepository(ctx context.Context, opts *InMemorySchemaOptions, logger *log.Helper) (model.SchemaRepository, error) {
	switch opts.Type {
	case SchemaTypeEmpty:
		return data.NewInMemorySchemaRepository(), nil
	case SchemaTypeJSON:
		return data.NewInMemorySchemaRepositoryFromJsonFile(ctx, opts.Path, data.NewJsonSchemaWithWorkspacesFromString)
	case SchemaTypeDir:
		return data.NewInMemorySchemaRepositoryFromDir(ctx, opts.Path, data.NewJsonSchemaWithWorkspacesFromString)
	default:
		return nil, fmt.Errorf("invalid in-memory repository type: %s", opts.Type)
	}
}
