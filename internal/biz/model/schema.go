package model

import "context"

// ValidationSchema defines the domain contract for resource schema validation.
type ValidationSchema interface {
	Validate(data interface{}) (bool, error)
}

// ValidationSchemaFromString is a factory function type that creates a ValidationSchema
// from a string representation (typically a JSON schema definition).
type ValidationSchemaFromString func(string) ValidationSchema

// ResourceSchemaRepresentation holds a resource schema with its validation logic.
type ResourceSchemaRepresentation struct {
	ResourceType     ResourceType
	ValidationSchema ValidationSchema
}

// ReporterSchemaRepresentation holds a reporter-specific schema.
type ReporterSchemaRepresentation struct {
	ResourceType     ResourceType
	ReporterType     ReporterType
	ValidationSchema ValidationSchema
}

// SchemaRepositoryError is a sentinel error type for schema repository operations.
type SchemaRepositoryError string

func (e SchemaRepositoryError) Error() string {
	return string(e)
}

const (
	ResourceSchemaNotFound = SchemaRepositoryError("resource not found")
	ReporterSchemaNotFound = SchemaRepositoryError("reporter not found")
)

// SchemaRepository defines the interface for managing resource and reporter schemas.
type SchemaRepository interface {
	// GetResourceSchemas returns all the resourceTypes that have a ResourceSchemaRepresentation.
	GetResourceSchemas(ctx context.Context) ([]ResourceType, error)
	// CreateResourceSchema adds the ResourceSchemaRepresentation into the repository.
	CreateResourceSchema(ctx context.Context, resource ResourceSchemaRepresentation) error
	// GetResourceSchema returns the resource schema for the resourceType.
	// Returns ResourceSchemaNotFound if the resource schema does not exist.
	GetResourceSchema(ctx context.Context, resourceType ResourceType) (ResourceSchemaRepresentation, error)
	// UpdateResourceSchema updates the ResourceSchemaRepresentation for the resourceType.
	// Returns ResourceSchemaNotFound if the resource schema does not exist.
	UpdateResourceSchema(ctx context.Context, resource ResourceSchemaRepresentation) error
	// DeleteResourceSchema deletes the ResourceSchemaRepresentation for the resourceType.
	// Returns ResourceSchemaNotFound if the resource schema does not exist.
	DeleteResourceSchema(ctx context.Context, resourceType ResourceType) error
	// GetReporterSchemas returns all the reporterTypes for resourceType.
	// Returns ResourceSchemaNotFound if the resourceType does not exist.
	GetReporterSchemas(ctx context.Context, resourceType ResourceType) ([]ReporterType, error)
	// CreateReporterSchema adds the ReporterSchemaRepresentation into the repository.
	// Returns ResourceSchemaNotFound if the resourceType does not exist.
	CreateReporterSchema(ctx context.Context, resourceReporter ReporterSchemaRepresentation) error
	// GetReporterSchema returns the ReporterSchemaRepresentation for the resourceType and reporterType.
	// Returns ResourceSchemaNotFound if the resource schema does not exist and
	// ReporterSchemaNotFound if the reporter schema does not exist for that resource.
	GetReporterSchema(ctx context.Context, resourceType ResourceType, reporterType ReporterType) (ReporterSchemaRepresentation, error)
	// UpdateReporterSchema updates the ReporterSchemaRepresentation for the resourceType and reporterType.
	// Returns ResourceSchemaNotFound if the resource schema does not exist and
	// ReporterSchemaNotFound if the reporter schema does not exist for that resource.
	UpdateReporterSchema(ctx context.Context, resourceReporter ReporterSchemaRepresentation) error
	// DeleteReporterSchema deletes the ReporterSchemaRepresentation for the resourceType and reporterType.
	// Returns ResourceSchemaNotFound if the resource schema does not exist and
	// ReporterSchemaNotFound if the reporter schema does not exist for that resource.
	DeleteReporterSchema(ctx context.Context, resourceType ResourceType, reporterType ReporterType) error
}
