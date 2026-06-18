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
//
// Phase 1: Delegates to DefaultSchema (hardcoded workspace logic).
// Phase 2: Will use s.relations to generate tuples dynamically.
func (s *UnifiedSchemaImpl) CalculateTuples(
	currentRepresentation, previousRepresentation *model.Representations,
	key model.ReporterResourceKey,
) (model.TuplesToReplicate, error) {
	// Phase 1: Delegate to existing workspace logic
	// Phase 2: Will implement relation-based tuple calculation here
	return model.NewDefaultSchema().CalculateTuples(currentRepresentation, previousRepresentation, key)
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
