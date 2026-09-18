package data

import (
	"context"
	"fmt"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// NewInMemorySchemaRepositoryFromUnifiedYAMLDir creates an in-memory
// repository from validated unified YAML artifacts. It does not alter the
// legacy directory or JSON repository constructors.
func NewInMemorySchemaRepositoryFromUnifiedYAMLDir(ctx context.Context, dir string) (*InMemorySchemaRepository, error) {
	schemas, err := LoadUnifiedSchemasFromDirectory(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to load unified schemas: %w", err)
	}

	repository := NewInMemorySchemaRepository()
	for _, schema := range schemas {
		resourceType, err := model.NewResourceType(schema.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid resource type %q: %w", schema.Name, err)
		}

		commonSchema := NewUnifiedSchemaImpl(schema.Common.Schema, schema.Common.Relations, nil)
		resourceSchema, err := model.NewResourceSchemaRepresentation(resourceType, commonSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to create resource schema for %q: %w", schema.Name, err)
		}
		if err := repository.CreateResourceSchema(ctx, resourceSchema); err != nil {
			return nil, fmt.Errorf("failed to register resource schema for %q: %w", schema.Name, err)
		}

		for _, reporter := range schema.Reporters {
			reporterType, err := model.NewReporterType(reporter.Name)
			if err != nil {
				return nil, fmt.Errorf("invalid reporter type %q for resource %q: %w", reporter.Name, schema.Name, err)
			}

			var reporterSchema model.Schema
			if reporter.Schema != nil {
				reporterSchema = NewUnifiedSchemaImpl(
					reporter.Schema,
					schema.Common.Relations,
					reporter.Relations,
				)
			}

			reporterRepresentation, err := model.NewReporterSchemaRepresentation(
				resourceType,
				reporterType,
				reporterSchema,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create reporter schema for %q:%q: %w", schema.Name, reporter.Name, err)
			}
			if err := repository.CreateReporterSchema(ctx, reporterRepresentation); err != nil {
				return nil, fmt.Errorf("failed to register reporter schema for %q:%q: %w", schema.Name, reporter.Name, err)
			}
		}
	}

	return repository, nil
}
