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
// Tuple calculation merges values from both common and reporter representations.
func NewJsonSchemaWithRelations(jsonSchema string, relations []model.RelationDef) model.Schema {
	return JsonSchemaWithRelations{jsonSchema: jsonSchema, relations: relations}
}

func (s JsonSchemaWithRelations) Validate(data interface{}) (bool, error) {
	return validateJsonSchema(s.jsonSchema, data)
}

// CommonRelationFields returns an empty slice: the relation definitions
// this type carries are not reporter-scoped and are used only for tuple
// calculation, so strict field enforcement is not applied on this path
// (known limitation; full enforcement is unified schema only).
func (s JsonSchemaWithRelations) CommonRelationFields() []string {
	return nil
}

// ReporterRelationFields returns an empty slice: the relation definitions
// this type carries are not reporter-scoped and are used only for tuple
// calculation, so strict field enforcement is not applied on this path
// (known limitation; full enforcement is unified schema only).
func (s JsonSchemaWithRelations) ReporterRelationFields(_ model.ReporterType) []string {
	return nil
}

func (s JsonSchemaWithRelations) CalculateTuples(
	currentRepresentation, previousRepresentation *model.Representations,
	key model.ReporterResourceKey,
) (model.TuplesToReplicate, error) {
	return model.CalculateTuplesFromRelationDefs(s.relations, currentRepresentation, previousRepresentation, key)
}
