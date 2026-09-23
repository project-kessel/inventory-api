package data

import (
	"fmt"
	"strings"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/xeipuuv/gojsonschema"
)

// UnifiedSchemaImpl adapts an embedded unified JSON Schema to the existing
// model.Schema contract. Relation metadata is retained for tuple calculation.
// Tuple calculation currently preserves legacy behavior.
type UnifiedSchemaImpl struct {
	schemaLoader      gojsonschema.JSONLoader
	commonRelations   []UnifiedSchemaRelation
	reporterRelations map[string][]UnifiedSchemaRelation
}

// NewUnifiedSchemaImpl creates a schema adapter for a common or reporter
// representation. The relation definitions are retained for later consumers.
func NewUnifiedSchemaImpl(
	schema map[string]interface{},
	commonRelations []UnifiedSchemaRelation,
	reporterRelations map[string][]UnifiedSchemaRelation,
) *UnifiedSchemaImpl {
	return &UnifiedSchemaImpl{
		schemaLoader:      gojsonschema.NewGoLoader(schema),
		commonRelations:   commonRelations,
		reporterRelations: reporterRelations,
	}
}

// Validate validates representation data against the embedded JSON Schema.
func (s *UnifiedSchemaImpl) Validate(data interface{}) (bool, error) {
	result, err := gojsonschema.Validate(s.schemaLoader, gojsonschema.NewGoLoader(data))
	if err != nil {
		return false, fmt.Errorf("validation error: %w", err)
	}
	if result.Valid() {
		return true, nil
	}

	validationErrors := make([]string, 0, len(result.Errors()))
	for _, validationError := range result.Errors() {
		validationErrors = append(validationErrors, validationError.String())
	}
	return false, fmt.Errorf("validation failed: %s", joinValidationErrors(validationErrors))
}

// CalculateTuples calculates common and reporter-specific relation changes.
func (s *UnifiedSchemaImpl) CalculateTuples(
	currentRepresentation, previousRepresentation *model.Representations,
	key model.ReporterResourceKey,
) (model.TuplesToReplicate, error) {
	var tuplesToCreate, tuplesToDelete []model.RelationsTuple

	for _, relation := range s.commonRelations {
		creates, deletes, err := calculateUnifiedRelation(
			currentRepresentation,
			previousRepresentation,
			key,
			relation,
			extractCommonRelationValues,
		)
		if err != nil {
			return model.TuplesToReplicate{}, fmt.Errorf("failed to calculate relation %q: %w", relation.Name, err)
		}
		tuplesToCreate = append(tuplesToCreate, creates...)
		tuplesToDelete = append(tuplesToDelete, deletes...)
	}

	for _, relation := range s.reporterRelations[key.ReporterType().String()] {
		creates, deletes, err := calculateUnifiedRelation(
			currentRepresentation,
			previousRepresentation,
			key,
			relation,
			extractReporterRelationValues,
		)
		if err != nil {
			return model.TuplesToReplicate{}, fmt.Errorf("failed to calculate reporter relation %q: %w", relation.Name, err)
		}
		tuplesToCreate = append(tuplesToCreate, creates...)
		tuplesToDelete = append(tuplesToDelete, deletes...)
	}

	return model.NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
}

type unifiedRelationValueExtractor func(*model.Representations, string, string) []string

func calculateUnifiedRelation(
	currentRepresentation, previousRepresentation *model.Representations,
	key model.ReporterResourceKey,
	relation UnifiedSchemaRelation,
	extract unifiedRelationValueExtractor,
) ([]model.RelationsTuple, []model.RelationsTuple, error) {
	namespace, resourceType, err := parseUnifiedRelationTarget(relation.Target)
	if err != nil {
		return nil, nil, err
	}

	creates, deletes := model.DiffRelationValues(
		key,
		relation.Name,
		namespace,
		resourceType,
		extract(currentRepresentation, relation.Field, relation.Cardinality),
		extract(previousRepresentation, relation.Field, relation.Cardinality),
	)
	return creates, deletes, nil
}

func parseUnifiedRelationTarget(target string) (string, string, error) {
	parts := strings.Split(target, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid relation target %q", target)
	}
	return parts[0], parts[1], nil
}

func extractCommonRelationValues(representations *model.Representations, field, cardinality string) []string {
	if representations == nil || !representations.HasCommon() {
		return nil
	}
	return extractRelationValues(representations.CommonData()[field], cardinality)
}

func extractReporterRelationValues(representations *model.Representations, field, cardinality string) []string {
	if representations == nil || !representations.HasReporter() {
		return nil
	}
	return extractRelationValues(representations.ReporterData()[field], cardinality)
}

func extractRelationValues(value interface{}, cardinality string) []string {
	if cardinality == "one" {
		if value, ok := value.(string); ok && value != "" {
			return []string{value}
		}
		return nil
	}

	return extractStringSlice(value)
}

func extractStringSlice(value interface{}) []string {
	switch values := value.(type) {
	case []interface{}:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if value, ok := value.(string); ok && value != "" {
				result = append(result, value)
			}
		}
		return result
	case []string:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if value != "" {
				result = append(result, value)
			}
		}
		return result
	default:
		return nil
	}
}

func joinValidationErrors(errors []string) string {
	if len(errors) == 0 {
		return ""
	}
	if len(errors) == 1 {
		return errors[0]
	}

	result := errors[0]
	for _, validationError := range errors[1:] {
		result += "; " + validationError
	}
	return result
}
