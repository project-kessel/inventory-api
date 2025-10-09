package schemas

import (
	"context"
)

type SchemaType struct {
	ResourceType string
	Type         string
	ReporterType string
}

type SchemaService interface {
	ShallowValidate(ctx context.Context, resourceType string, commonRepresentation map[string]interface{}, reporterRepresentation map[string]interface{}) error

	// CalculateTuples(context.Context, string, CommonRepresentation, ReporterRepresentation) ([]model.TuplesToReplicate, error)
	// TODO: Add full validation
}

type SchemaRepository interface {
	Create(ctx context.Context, schemaType SchemaType, content string) error
	Get(ctx context.Context, schemaType SchemaType) (string, error)
	Update(ctx context.Context, schemaType SchemaType, content string) error
	Delete(ctx context.Context, schemaType SchemaType) error
}
