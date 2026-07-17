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

// ResourceTypeSchemaFactory is a factory function that creates a Schema
// with awareness of the resource type.
type ResourceTypeSchemaFactory func(resourceType ResourceType, jsonSchema string) Schema

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

// RelationDef describes how a field in a resource representation maps to a
// relation tuple.  fieldName is the JSON key in the representation data;
// relationName is the relation written to SpiceDB; subjectNamespace and
// subjectResourceType identify the subject side of the tuple; multiValued
// indicates whether the field holds an array (true) or a scalar (false).
type RelationDef struct {
	fieldName           string
	relationName        string
	subjectNamespace    string
	subjectResourceType string
	multiValued         bool
}

func NewRelationDef(fieldName, relationName, subjectNamespace, subjectResourceType string, multiValued bool) (RelationDef, error) {
	if fieldName == "" {
		return RelationDef{}, fmt.Errorf("field name is required")
	}
	if relationName == "" {
		return RelationDef{}, fmt.Errorf("relation name is required")
	}
	if subjectNamespace == "" {
		return RelationDef{}, fmt.Errorf("subject namespace is required")
	}
	if subjectResourceType == "" {
		return RelationDef{}, fmt.Errorf("subject resource type is required")
	}
	return RelationDef{
		fieldName:           fieldName,
		relationName:        relationName,
		subjectNamespace:    subjectNamespace,
		subjectResourceType: subjectResourceType,
		multiValued:         multiValued,
	}, nil
}

func (r RelationDef) FieldName() string           { return r.fieldName }
func (r RelationDef) RelationName() string        { return r.relationName }
func (r RelationDef) SubjectNamespace() string    { return r.subjectNamespace }
func (r RelationDef) SubjectResourceType() string { return r.subjectResourceType }
func (r RelationDef) MultiValued() bool           { return r.multiValued }

// CalculateTuplesFromRelationDefs computes create/delete tuple sets by
// diffing current vs previous representation values for each relation
// definition.
func CalculateTuplesFromRelationDefs(
	relations []RelationDef,
	current, previous *Representations,
	key ReporterResourceKey,
) (TuplesToReplicate, error) {
	var allCreates, allDeletes []RelationsTuple

	for _, rel := range relations {
		var currentValues, previousValues []string
		if rel.multiValued {
			currentValues = current.StringSliceField(rel.fieldName)
			previousValues = previous.StringSliceField(rel.fieldName)
		} else {
			if v := current.StringField(rel.fieldName); v != "" {
				currentValues = []string{v}
			}
			if v := previous.StringField(rel.fieldName); v != "" {
				previousValues = []string{v}
			}
		}

		creates, deletes := DiffRelationValues(
			key, rel.relationName, rel.subjectNamespace, rel.subjectResourceType,
			currentValues, previousValues,
		)
		allCreates = append(allCreates, creates...)
		allDeletes = append(allDeletes, deletes...)
	}

	return NewTuplesToReplicate(allCreates, allDeletes)
}
