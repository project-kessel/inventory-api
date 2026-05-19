package model

import "fmt"

// Schema defines the domain contract for resource schema validation and tuple calculation.
// Implementations encapsulate both the validation rules (e.g., JSON Schema) and the
// business logic for determining which tuples need replication when a resource changes.
type Schema interface {
	Validate(data interface{}) (bool, error)
	CalculateTuples(currentRepresentation, previousRepresentation *Representations, key ReporterResourceKey) (TuplesToReplicate, error)
}

// DefaultSchema provides the default tuple calculation behavior for resource types
// that do not have a registered JSON schema. It creates workspace-based tuples
// using the workspace_id from representations, with no validation constraints.
type DefaultSchema struct{}

var defaultSchemaInstance = DefaultSchema{}

func NewDefaultSchema() Schema {
	return defaultSchemaInstance
}

func (d DefaultSchema) Validate(_ interface{}) (bool, error) {
	return true, nil
}

func (d DefaultSchema) CalculateTuples(currentRepresentation, previousRepresentation *Representations, key ReporterResourceKey) (TuplesToReplicate, error) {
	currentWorkspaceID := ""
	if currentRepresentation != nil {
		currentWorkspaceID = currentRepresentation.WorkspaceID()
	}
	previousWorkspaceID := ""
	if previousRepresentation != nil {
		previousWorkspaceID = previousRepresentation.WorkspaceID()
	}

	if previousWorkspaceID != "" && previousWorkspaceID == currentWorkspaceID {
		return TuplesToReplicate{}, nil
	}

	var tuplesToCreate, tuplesToDelete []RelationsTuple

	if currentWorkspaceID != "" {
		tuplesToCreate = append(tuplesToCreate, NewWorkspaceRelationsTuple(currentWorkspaceID, key))
	}

	if previousWorkspaceID != "" {
		tuplesToDelete = append(tuplesToDelete, NewWorkspaceRelationsTuple(previousWorkspaceID, key))
	}

	return NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
}

// SchemaFromString is a factory function type that creates a Schema
// from a string representation (typically a JSON schema definition).
type SchemaFromString func(string) Schema

// ResourceSchemaRepresentation holds a resource schema with its validation and tuple logic.
type ResourceSchemaRepresentation struct {
	resourceType ResourceType
	schema       Schema
}

func NewResourceSchemaRepresentation(resourceType ResourceType, schema Schema) (ResourceSchemaRepresentation, error) {
	if resourceType == "" {
		return ResourceSchemaRepresentation{}, fmt.Errorf("resource type is required")
	}
	return ResourceSchemaRepresentation{
		resourceType: resourceType,
		schema:       schema,
	}, nil
}

func (r ResourceSchemaRepresentation) ResourceType() ResourceType { return r.resourceType }
func (r ResourceSchemaRepresentation) Schema() Schema             { return r.schema }

// ReporterSchemaRepresentation holds a reporter-specific schema.
type ReporterSchemaRepresentation struct {
	resourceType ResourceType
	reporterType ReporterType
	schema       Schema
}

func NewReporterSchemaRepresentation(resourceType ResourceType, reporterType ReporterType, schema Schema) (ReporterSchemaRepresentation, error) {
	if resourceType == "" {
		return ReporterSchemaRepresentation{}, fmt.Errorf("resource type is required")
	}
	if reporterType == "" {
		return ReporterSchemaRepresentation{}, fmt.Errorf("reporter type is required")
	}
	return ReporterSchemaRepresentation{
		resourceType: resourceType,
		reporterType: reporterType,
		schema:       schema,
	}, nil
}

func (r ReporterSchemaRepresentation) ResourceType() ResourceType { return r.resourceType }
func (r ReporterSchemaRepresentation) ReporterType() ReporterType { return r.reporterType }
func (r ReporterSchemaRepresentation) Schema() Schema             { return r.schema }
