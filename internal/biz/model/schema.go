package model

import "fmt"

// ValidationSchema defines the domain contract for resource schema validation.
type ValidationSchema interface {
	Validate(data interface{}) (bool, error)
}

// ValidationSchemaFromString is a factory function type that creates a ValidationSchema
// from a string representation (typically a JSON schema definition).
type ValidationSchemaFromString func(string) ValidationSchema

// ResourceSchemaRepresentation holds a resource schema with its validation logic.
type ResourceSchemaRepresentation struct {
	resourceType     ResourceType
	validationSchema ValidationSchema
}

func NewResourceSchemaRepresentation(resourceType ResourceType, validationSchema ValidationSchema) (ResourceSchemaRepresentation, error) {
	if resourceType == "" {
		return ResourceSchemaRepresentation{}, fmt.Errorf("resource type is required")
	}
	return ResourceSchemaRepresentation{
		resourceType:     resourceType,
		validationSchema: validationSchema,
	}, nil
}

func (r ResourceSchemaRepresentation) ResourceType() ResourceType         { return r.resourceType }
func (r ResourceSchemaRepresentation) ValidationSchema() ValidationSchema { return r.validationSchema }

// ReporterSchemaRepresentation holds a reporter-specific schema.
type ReporterSchemaRepresentation struct {
	resourceType     ResourceType
	reporterType     ReporterType
	validationSchema ValidationSchema
}

func NewReporterSchemaRepresentation(resourceType ResourceType, reporterType ReporterType, validationSchema ValidationSchema) (ReporterSchemaRepresentation, error) {
	if resourceType == "" {
		return ReporterSchemaRepresentation{}, fmt.Errorf("resource type is required")
	}
	if reporterType == "" {
		return ReporterSchemaRepresentation{}, fmt.Errorf("reporter type is required")
	}
	return ReporterSchemaRepresentation{
		resourceType:     resourceType,
		reporterType:     reporterType,
		validationSchema: validationSchema,
	}, nil
}

func (r ReporterSchemaRepresentation) ResourceType() ResourceType         { return r.resourceType }
func (r ReporterSchemaRepresentation) ReporterType() ReporterType         { return r.reporterType }
func (r ReporterSchemaRepresentation) ValidationSchema() ValidationSchema { return r.validationSchema }
