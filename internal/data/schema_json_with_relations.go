package data

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// JsonSchemaWithRelations is a Schema implementation that validates data using
// JSON Schema and calculates tuples from a configurable table of relation
// definitions.
type JsonSchemaWithRelations struct {
	jsonSchema string
	relations  []model.RelationDef
}

// NewJsonSchemaWithRelations creates a Schema that validates against jsonSchema
// and computes tuples from the given relation definitions.
func NewJsonSchemaWithRelations(jsonSchema string, relations []model.RelationDef) model.Schema {
	return JsonSchemaWithRelations{jsonSchema: jsonSchema, relations: relations}
}

func (s JsonSchemaWithRelations) Validate(data interface{}) (bool, error) {
	return validateJsonSchema(s.jsonSchema, data)
}

func (s JsonSchemaWithRelations) CalculateTuples(
	currentRepresentation, previousRepresentation *model.Representations,
	key model.ReporterResourceKey,
) (model.TuplesToReplicate, error) {
	return model.CalculateTuplesFromRelationDefs(s.relations, currentRepresentation, previousRepresentation, key)
}
