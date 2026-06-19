package data

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

const (
	featureNamespace = "features"
	rbacNamespace    = "rbac"
)

type relationDef struct {
	fieldName           string
	relationName        string
	subjectNamespace    string
	subjectResourceType string
	multiValued         bool
}

// relationDefsSchema validates data using JSON Schema and calculates tuples
// from a table of relation definitions. Hardcoded per-resource-type instances
// will be replaced by a generic Starlark-driven schema.
type relationDefsSchema struct {
	jsonSchema string
	relations  []relationDef
}

func (s relationDefsSchema) Validate(data interface{}) (bool, error) {
	return validateJsonSchema(s.jsonSchema, data)
}

func (s relationDefsSchema) CalculateTuples(
	currentRepresentation, previousRepresentation *model.Representations,
	key model.ReporterResourceKey,
) (model.TuplesToReplicate, error) {
	var allCreates, allDeletes []model.RelationsTuple

	for _, rel := range s.relations {
		var currentValues, previousValues []string
		if rel.multiValued {
			currentValues = currentRepresentation.StringSliceField(rel.fieldName)
			previousValues = previousRepresentation.StringSliceField(rel.fieldName)
		} else {
			if v := currentRepresentation.StringField(rel.fieldName); v != "" {
				currentValues = []string{v}
			}
			if v := previousRepresentation.StringField(rel.fieldName); v != "" {
				previousValues = []string{v}
			}
		}

		creates, deletes := model.DiffRelationValues(
			key, rel.relationName, rel.subjectNamespace, rel.subjectResourceType,
			currentValues, previousValues,
		)
		allCreates = append(allCreates, creates...)
		allDeletes = append(allDeletes, deletes...)
	}

	return model.NewTuplesToReplicate(allCreates, allDeletes)
}

var serviceRelations = []relationDef{
	{"allowed_workspace_ids", "allowed_workspaces", rbacNamespace, "workspace", true},
	{"billing_account_ids", "billing_account", featureNamespace, "billing_account", true},
	{"parent_service_id", "parent", featureNamespace, "service", false},
}

func NewFeaturesServiceSchemaFromString(jsonSchema string) model.Schema {
	return relationDefsSchema{jsonSchema: jsonSchema, relations: serviceRelations}
}
