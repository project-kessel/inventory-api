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
// Phase 2: Tuple calculation uses relations from the schema.
// Phase 4a: Supports reporter-specific relations extracted from reporter data.
type UnifiedSchemaImpl struct {
	// Schema is the embedded JSON Schema object for validation.
	// Passed directly to gojsonschema - no conversion needed.
	schema map[string]interface{}

	// Relations define the common tuples to create/delete.
	// These are extracted from the common representation.
	relations []model.RelationDefinition

	// reporterRelations maps reporter type to reporter-specific relations (Phase 4a).
	// These are extracted from the reporter representation.
	// Map key is the reporter type string (e.g., "hbi", "ocm").
	reporterRelations map[string][]model.RelationDefinition

	// schemaLoader is cached for performance.
	schemaLoader gojsonschema.JSONLoader
}

// NewUnifiedSchemaImpl creates a new UnifiedSchemaImpl from a schema and relations.
func NewUnifiedSchemaImpl(schema map[string]interface{}, relations []model.RelationDefinition) *UnifiedSchemaImpl {
	return &UnifiedSchemaImpl{
		schema:            schema,
		relations:         relations,
		reporterRelations: make(map[string][]model.RelationDefinition),
		schemaLoader:      gojsonschema.NewGoLoader(schema),
	}
}

// NewUnifiedSchemaImplWithReporterRelations creates a new UnifiedSchemaImpl with reporter relations.
// This is used when creating reporter-specific schemas that have their own relations (Phase 4a).
func NewUnifiedSchemaImplWithReporterRelations(
	schema map[string]interface{},
	commonRelations []model.RelationDefinition,
	reporterType string,
	reporterRelations []model.RelationDefinition,
) *UnifiedSchemaImpl {
	reporterRelationsMap := make(map[string][]model.RelationDefinition)
	if len(reporterRelations) > 0 {
		reporterRelationsMap[reporterType] = reporterRelations
	}

	return &UnifiedSchemaImpl{
		schema:            schema,
		relations:         commonRelations,
		reporterRelations: reporterRelationsMap,
		schemaLoader:      gojsonschema.NewGoLoader(schema),
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
		if relation.Cardinality == "many" {
			// Handle many cardinality - extract array and create tuples for each element
			createTuples, deleteTuples, err := processManyCardinality(currentRepresentation, previousRepresentation, key, relation, extractFieldArray)
			if err != nil {
				return model.TuplesToReplicate{}, fmt.Errorf("failed to process many cardinality for relation %q: %w", relation.Name, err)
			}
			tuplesToCreate = append(tuplesToCreate, createTuples...)
			tuplesToDelete = append(tuplesToDelete, deleteTuples...)
		} else {
			// Handle one cardinality (default behavior)
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
	}

	// Process reporter-specific relations (Phase 4a)
	// These are extracted from the reporter representation instead of common
	reporterType := key.ReporterType().String()
	if reporterRelations, exists := s.reporterRelations[reporterType]; exists {
		for _, relation := range reporterRelations {
			if relation.Cardinality == "many" {
				// Handle many cardinality for reporter relations
				createTuples, deleteTuples, err := processManyCardinality(currentRepresentation, previousRepresentation, key, relation, extractReporterFieldArray)
				if err != nil {
					return model.TuplesToReplicate{}, fmt.Errorf("failed to process many cardinality for reporter relation %q: %w", relation.Name, err)
				}
				tuplesToCreate = append(tuplesToCreate, createTuples...)
				tuplesToDelete = append(tuplesToDelete, deleteTuples...)
			} else {
				// Handle one cardinality (default behavior)
				// Extract field values from reporter representation
				currentValue := extractReporterFieldValue(currentRepresentation, relation.Field)
				previousValue := extractReporterFieldValue(previousRepresentation, relation.Field)

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
						return model.TuplesToReplicate{}, fmt.Errorf("failed to build reporter tuple for relation %q: %w", relation.Name, err)
					}
					tuplesToCreate = append(tuplesToCreate, tuple)
				}

				// Delete tuple for old value
				if previousValue != "" {
					tuple, err := buildRelationTuple(key, relation, previousValue)
					if err != nil {
						return model.TuplesToReplicate{}, fmt.Errorf("failed to build delete reporter tuple for relation %q: %w", relation.Name, err)
					}
					tuplesToDelete = append(tuplesToDelete, tuple)
				}
			}
		}
	}

	return model.NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
}

// extractFieldValue extracts a field value from common representations.
// This is used for common relations defined in the common schema.
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

	return ""
}

// extractReporterFieldValue extracts a field value from reporter representations (Phase 4a).
// This is used for reporter-specific relations defined in the reporter schema.
func extractReporterFieldValue(representations *model.Representations, fieldName string) string {
	if representations == nil {
		return ""
	}

	// Check reporter data
	if representations.HasReporter() {
		if value, ok := representations.ReporterData()[fieldName]; ok {
			if strValue, ok := value.(string); ok && strValue != "" {
				return strValue
			}
		}
	}

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

// extractFieldArray extracts an array field from common representations (Phase 4b).
// Returns a slice of string values from the array field.
func extractFieldArray(representations *model.Representations, fieldName string) []string {
	if representations == nil {
		return nil
	}

	if representations.HasCommon() {
		if value, ok := representations.CommonData()[fieldName]; ok {
			return arrayToStringSlice(value)
		}
	}

	return nil
}

// extractReporterFieldArray extracts an array field from reporter representations (Phase 4b).
// Returns a slice of string values from the array field.
func extractReporterFieldArray(representations *model.Representations, fieldName string) []string {
	if representations == nil {
		return nil
	}

	if representations.HasReporter() {
		if value, ok := representations.ReporterData()[fieldName]; ok {
			return arrayToStringSlice(value)
		}
	}

	return nil
}

// arrayToStringSlice converts an interface{} that should be an array to []string.
// Skips empty strings and non-string elements.
func arrayToStringSlice(value interface{}) []string {
	// Handle []interface{} (common from JSON unmarshaling)
	if arr, ok := value.([]interface{}); ok {
		var result []string
		for _, elem := range arr {
			if str, ok := elem.(string); ok && str != "" {
				result = append(result, str)
			}
		}
		return result
	}

	// Handle []string directly
	if arr, ok := value.([]string); ok {
		var result []string
		for _, str := range arr {
			if str != "" {
				result = append(result, str)
			}
		}
		return result
	}

	return nil
}

// arrayExtractor is a function type that extracts an array from representations.
type arrayExtractor func(*model.Representations, string) []string

// processManyCardinality handles relations with cardinality "many" (Phase 4b).
// It computes the diff between current and previous arrays and returns tuples to create/delete.
func processManyCardinality(
	currentRepresentation, previousRepresentation *model.Representations,
	key model.ReporterResourceKey,
	relation model.RelationDefinition,
	extractor arrayExtractor,
) ([]model.RelationsTuple, []model.RelationsTuple, error) {
	var tuplesToCreate, tuplesToDelete []model.RelationsTuple

	// Extract arrays
	currentArray := extractor(currentRepresentation, relation.Field)
	previousArray := extractor(previousRepresentation, relation.Field)

	// Build sets for efficient diff
	currentSet := make(map[string]bool)
	for _, value := range currentArray {
		currentSet[value] = true
	}

	previousSet := make(map[string]bool)
	for _, value := range previousArray {
		previousSet[value] = true
	}

	// Create tuples for elements in current but not in previous
	for value := range currentSet {
		if !previousSet[value] {
			tuple, err := buildRelationTuple(key, relation, value)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to build tuple for value %q: %w", value, err)
			}
			tuplesToCreate = append(tuplesToCreate, tuple)
		}
	}

	// Delete tuples for elements in previous but not in current
	for value := range previousSet {
		if !currentSet[value] {
			tuple, err := buildRelationTuple(key, relation, value)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to build delete tuple for value %q: %w", value, err)
			}
			tuplesToDelete = append(tuplesToDelete, tuple)
		}
	}

	return tuplesToCreate, tuplesToDelete, nil
}
