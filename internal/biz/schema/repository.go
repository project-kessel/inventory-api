package schema

import (
	"context"

	"github.com/project-kessel/inventory-api/internal/biz/schema/validation"
)

type schemaRepositoryError string

func (e schemaRepositoryError) Error() string {
	return string(e)
}

const (
	ResourceSchemaNotFound = schemaRepositoryError("resource not found")
	ReporterSchemaNotFound = schemaRepositoryError("reporter not found")
)

type ResourceRepresentation struct {
	ResourceType     string
	ValidationSchema validation.Schema
}
type ReporterRepresentation struct {
	ResourceType     string
	ReporterType     string
	ValidationSchema validation.Schema
}
type Repository interface {
	// GetResourceSchemas returns all the resourceTypes that have a ResourceRepresentation.
	GetResourceSchemas(ctx context.Context) ([]string, error)
	// CreateResourceSchema adds the ResourceRepresentation into the repository.
	CreateResourceSchema(ctx context.Context, resource ResourceRepresentation) error
	// GetResourceSchema returns the resource schema for the resourceType. Returns ResourceSchemaNotFound if the resource schema does not exist.
	GetResourceSchema(ctx context.Context, resourceType string) (ResourceRepresentation, error)
	// UpdateResourceSchema updates the ResourceRepresentation for the resourceType. Returns ResourceSchemaNotFound if the resource schema does not exist.
	UpdateResourceSchema(ctx context.Context, resource ResourceRepresentation) error
	// DeleteResourceSchema deletes the ResourceRepresentation for the resourceType. Returns ResourceSchemaNotFound if the resource schema does not exist.
	DeleteResourceSchema(ctx context.Context, resourceType string) error
	// GetReporterSchemas returns all the reporterTypes for resourceType. Returns ResourceSchemaNotFound if the resourceType does not exist.
	GetReporterSchemas(ctx context.Context, resourceType string) ([]string, error)
	// CreateReporterSchema adds the ReporterRepresentation into the repository. Returns ResourceSchemaNotFound if the resourceType does not exist.
	CreateReporterSchema(ctx context.Context, resourceReporter ReporterRepresentation) error
	// GetReporterSchema returns the ReporterRepresentation for the resourceType and reporterType. Returns ResourceSchemaNotFound if the resource schema does not exist and ReporterSchemaNotFound if the reporter schema does not exist for that resource.
	GetReporterSchema(ctx context.Context, resourceType string, reporterType string) (ReporterRepresentation, error)
	// UpdateReporterSchema updates the ReporterRepresentation for the resourceType and reporterType. Returns ResourceSchemaNotFound if the resource schema does not exist and ReporterSchemaNotFound if the reporter schema does not exist for that resource.
	UpdateReporterSchema(ctx context.Context, resourceReporter ReporterRepresentation) error
	// DeleteReporterSchema deletes the ReporterRepresentation for the resourceType and reporterType. Returns ResourceSchemaNotFound if the resource schema does not exist and ReporterSchemaNotFound if the reporter schema does not exist for that resource.
	DeleteReporterSchema(ctx context.Context, resourceType string, reporterType string) error
}
