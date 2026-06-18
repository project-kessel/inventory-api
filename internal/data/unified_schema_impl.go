package data

import (
	"fmt"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/xeipuuv/gojsonschema"
)

// UnifiedSchemaImpl implements model.Schema using embedded JSON Schema for validation
// and schema-defined relations for tuple calculation.
//
// Phase 1: Validation works, tuple calculation delegates to DefaultSchema.
// Phase 2: Tuple calculation will use relations from the schema.
type UnifiedSchemaImpl struct {
	// Schema is the embedded JSON Schema object for validation.
	// Passed directly to gojsonschema - no conversion needed.
	schema map[string]interface{}

	// Relations define the tuples to create/delete (Phase 2).
	// For Phase 1, this is populated but not used - we delegate to DefaultSchema.
	relations []model.RelationDefinition

	// schemaLoader is cached for performance.
	schemaLoader gojsonschema.JSONLoader
}

// NewUnifiedSchemaImpl creates a new UnifiedSchemaImpl from a schema and relations.
func NewUnifiedSchemaImpl(schema map[string]interface{}, relations []model.RelationDefinition) *UnifiedSchemaImpl {
	return &UnifiedSchemaImpl{
		schema:       schema,
		relations:    relations,
		schemaLoader: gojsonschema.NewGoLoader(schema),
	}
}

// Validate validates the given data against the embedded JSON Schema.
// The schema is used directly with gojsonschema - no conversion needed.
func (s *UnifiedSchemaImpl) Validate(data interface{}) (bool, error) {
	dataLoader := gojsonschema.NewGoLoader(data)
	result, err := gojsonschema.Validate(s.schemaLoader, dataLoader)

	if err != nil {
		return false, fmt.Errorf("validation error: %w", err)
	}

	if !result.Valid() {
		// Collect all validation errors
		var errMsgs []string
		for _, desc := range result.Errors() {
			errMsgs = append(errMsgs, desc.String())
		}
		return false, fmt.Errorf("validation failed: %s", joinErrors(errMsgs))
	}

	return true, nil
}

// CalculateTuples computes the relation tuples to replicate for a given resource.
// Uses the relations defined in the schema to dynamically generate tuples based on
// field values in the current and previous representations.
//
// For each relation:
// - Extract field value from current and previous representations
// - If values differ, create new tuple for current value and delete tuple for previous value
// - If values are the same, no tuples needed (already exists)
func (s *UnifiedSchemaImpl) CalculateTuples(
	currentRepresentation, previousRepresentation *model.Representations,
	key model.ReporterResourceKey,
) (model.TuplesToReplicate, error) {
	var tuplesToCreate, tuplesToDelete []model.RelationsTuple

	// Process each relation defined in the schema
	for _, relation := range s.relations {
		// Extract field values from representations
		currentValue := extractFieldValue(currentRepresentation, relation.Field)
		previousValue := extractFieldValue(previousRepresentation, relation.Field)

		// Skip if both values are empty (no relation exists)
		if currentValue == "" && previousValue == "" {
			continue
		}

		// If values are the same, tuple already exists - no action needed
		if currentValue == previousValue {
			continue
		}

		// Create tuple for new value
		if currentValue != "" {
			tuple, err := buildRelationTuple(key, relation, currentValue)
			if err != nil {
				return model.TuplesToReplicate{}, fmt.Errorf("failed to build tuple for relation %q: %w", relation.Name, err)
			}
			tuplesToCreate = append(tuplesToCreate, tuple)
		}

		// Delete tuple for old value
		if previousValue != "" {
			tuple, err := buildRelationTuple(key, relation, previousValue)
			if err != nil {
				return model.TuplesToReplicate{}, fmt.Errorf("failed to build delete tuple for relation %q: %w", relation.Name, err)
			}
			tuplesToDelete = append(tuplesToDelete, tuple)
		}
	}

	return model.NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
}

// extractFieldValue extracts a field value from representations.
// Phase 1: Only checks common data (where workspace_id lives).
// Phase 4: Will also check reporter data for reporter-specific relations.
func extractFieldValue(representations *model.Representations, fieldName string) string {
	if representations == nil {
		return ""
	}

	// Check common data
	if representations.HasCommon() {
		if value, ok := representations.CommonData()[fieldName]; ok {
			if strValue, ok := value.(string); ok && strValue != "" {
				return strValue
			}
		}
	}

	// Phase 4: Add reporter data support when reporter-specific relations are implemented
	// For now, all Phase 1 relations are in common data

	return ""
}

// buildRelationTuple builds a RelationsTuple from a relation definition and field value.
// Mimics the structure of NewWorkspaceRelationsTuple but uses dynamic relation metadata.
func buildRelationTuple(
	key model.ReporterResourceKey,
	relation model.RelationDefinition,
	fieldValue string,
) (model.RelationsTuple, error) {
	// Create the object (the resource being related)
	reporter := model.NewReporterReference(key.ReporterType(), nil)
	object := model.NewResourceReference(
		key.ResourceType(),
		key.LocalResourceId(),
		&reporter,
	)

	// Parse the target (e.g., "rbac/workspace" -> namespace="rbac", resourceType="workspace")
	targetNamespace, targetResourceType, err := parseTarget(relation.Target)
	if err != nil {
		return model.RelationsTuple{}, fmt.Errorf("invalid target %q: %w", relation.Target, err)
	}

	// Create the subject (the target resource)
	subjectId := model.DeserializeLocalResourceId(fieldValue)
	subjectReporterType := model.DeserializeReporterType(targetNamespace)
	subjectReporter := model.NewReporterReference(subjectReporterType, nil)
	subjectResource := model.NewResourceReference(
		model.DeserializeResourceType(targetResourceType),
		subjectId,
		&subjectReporter,
	)
	subject := model.NewSubjectReferenceWithoutRelation(subjectResource)

	// Create the relation
	relationName := model.DeserializeRelation(relation.Name)

	return model.NewRelationsTuple(object, relationName, subject), nil
}

// parseTarget parses a target string like "rbac/workspace" into namespace and resource type.
func parseTarget(target string) (namespace, resourceType string, err error) {
	parts := splitOnce(target, '/')
	if len(parts) != 2 {
		return "", "", fmt.Errorf("target must be in format 'namespace/resource_type', got %q", target)
	}
	return parts[0], parts[1], nil
}

// splitOnce splits a string on the first occurrence of sep.
func splitOnce(s string, sep rune) []string {
	for i, c := range s {
		if c == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// joinErrors joins error messages with semicolons.
func joinErrors(errors []string) string {
	if len(errors) == 0 {
		return ""
	}
	if len(errors) == 1 {
		return errors[0]
	}

	result := errors[0]
	for i := 1; i < len(errors); i++ {
		result += "; " + errors[i]
	}
	return result
}
