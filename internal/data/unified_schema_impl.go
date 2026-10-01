package data

import (
	"fmt"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/xeipuuv/gojsonschema"
)

// UnifiedSchemaImpl adapts an embedded unified JSON Schema to the existing
// model.Schema contract. Relation metadata is retained for tuple calculation.
// Tuple calculation currently preserves legacy behavior.
type UnifiedSchemaImpl struct {
	schemaLoader      gojsonschema.JSONLoader
	commonRelations   []UnifiedSchemaRelation
	reporterRelations []UnifiedSchemaRelation
}

// NewUnifiedSchemaImpl creates a schema adapter for a common or reporter
// representation. The relation definitions are retained for later consumers.
func NewUnifiedSchemaImpl(
	schema map[string]interface{},
	commonRelations []UnifiedSchemaRelation,
	reporterRelations []UnifiedSchemaRelation,
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

// CalculateTuples preserves the existing tuple behavior until the unified
// relation calculation is implemented.
func (s *UnifiedSchemaImpl) CalculateTuples(
	currentRepresentation, previousRepresentation *model.Representations,
	key model.ReporterResourceKey,
) (model.TuplesToReplicate, error) {
	return model.NewDefaultSchema().CalculateTuples(currentRepresentation, previousRepresentation, key)
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
