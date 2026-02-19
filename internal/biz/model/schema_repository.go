package model

import (
	"context"
)

type schemaRepositoryError string

func (e schemaRepositoryError) Error() string {
	return string(e)
}

const (
	ResourceSchemaNotFound = schemaRepositoryError("resource not found")
	ReporterSchemaNotFound = schemaRepositoryError("reporter not found")
)

type ResourceSchema struct {
	ResourceType     string
	ValidationSchema ValidationSchema
}
type ReporterSchema struct {
	ResourceType     string
	ReporterType     string
	ValidationSchema ValidationSchema
}
type SchemaRepository interface {
	// GetResourceSchemas returns all the resourceTypes that have a ResourceSchema.
	GetResourceSchemas(ctx context.Context) ([]string, error)
	// CreateResourceSchema adds the ResourceSchema into the repository.
	CreateResourceSchema(ctx context.Context, resource ResourceSchema) error
	// GetResourceSchema returns the resource schema for the resourceType. Returns ResourceSchemaNotFound if the resource schema does not exist.
	GetResourceSchema(ctx context.Context, resourceType string) (ResourceSchema, error)
	// UpdateResourceSchema updates the ResourceSchema for the resourceType. Returns ResourceSchemaNotFound if the resource schema does not exist.
	UpdateResourceSchema(ctx context.Context, resource ResourceSchema) error
	// DeleteResourceSchema deletes the ResourceSchema for the resourceType. Returns ResourceSchemaNotFound if the resource schema does not exist.
	DeleteResourceSchema(ctx context.Context, resourceType string) error
	// GetReporterSchemas returns all the reporterTypes for resourceType. Returns ResourceSchemaNotFound if the resourceType does not exist.
	GetReporterSchemas(ctx context.Context, resourceType string) ([]string, error)
	// CreateReporterSchema adds the ReporterSchema into the repository. Returns ResourceSchemaNotFound if the resourceType does not exist.
	CreateReporterSchema(ctx context.Context, resourceReporter ReporterSchema) error
	// GetReporterSchema returns the ReporterSchema for the resourceType and reporterType. Returns ResourceSchemaNotFound if the resource schema does not exist and ReporterSchemaNotFound if the reporter schema does not exist for that resource.
	GetReporterSchema(ctx context.Context, resourceType string, reporterType string) (ReporterSchema, error)
	// UpdateReporterSchema updates the ReporterSchema for the resourceType and reporterType. Returns ResourceSchemaNotFound if the resource schema does not exist and ReporterSchemaNotFound if the reporter schema does not exist for that resource.
	UpdateReporterSchema(ctx context.Context, resourceReporter ReporterSchema) error
	// DeleteReporterSchema deletes the ReporterSchema for the resourceType and reporterType. Returns ResourceSchemaNotFound if the resource schema does not exist and ReporterSchemaNotFound if the reporter schema does not exist for that resource.
	DeleteReporterSchema(ctx context.Context, resourceType string, reporterType string) error
}
