package model

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
