package data

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

const featureNamespace = "features"

const featuresServiceWildcard = "features/service:*"

const featuresServiceWildcardID = "*"

func mustRelationDef(fieldName, relationName, subjectNamespace, subjectResourceType string, multiValued bool) model.RelationDef {
	rd, err := model.NewRelationDef(fieldName, relationName, subjectNamespace, subjectResourceType, multiValued)
	if err != nil {
		panic(err)
	}
	return rd
}

var billingAccountRelations = []model.RelationDef{
	mustRelationDef("services", "services", featureNamespace, "service", true),
}

func NewFeaturesBillingAccountSchemaFromString(jsonSchema string) model.Schema {
	return NewJsonSchemaWithRelations(jsonSchema, billingAccountRelations)
}

var workspaceRelations = []model.RelationDef{
	mustRelationDef("direct_billing_account", "direct_billing_account", featureNamespace, "billing_account", false),
	mustRelationDef("direct_service_preferences", "direct_service_preferences", featureNamespace, "service", true),
}

var workspaceWildcardRelations = []string{
	"desire_all_services",
	"ignore_inherited_desired_services",
	"ignore_inherited_paid_services",
}

// featuresWorkspaceSchema delegates validation and direct relation tuple calculation to its wrapped schema, and handles service wildcard tuples.
type featuresWorkspaceSchema struct {
	schema JsonSchemaWithRelations
}

// NewFeaturesWorkspaceSchemaFromString creates a workspace schema from a JSON schema string.
func NewFeaturesWorkspaceSchemaFromString(jsonSchema string) model.Schema {
	return featuresWorkspaceSchema{
		schema: JsonSchemaWithRelations{
			jsonSchema: jsonSchema,
			relations:  workspaceRelations,
		},
	}
}

// Validate delegates workspace data validation to the wrapped JSON schema.
func (s featuresWorkspaceSchema) Validate(data interface{}) (bool, error) {
	return s.schema.Validate(data)
}

// CalculateTuples delegates direct relation tuple calculation to the wrapped schema and handles configured service wildcard fields.
// A change to or from the qualified `features/service:*` marker creates or deletes a `features/service` subject tuple with ID `*`.
func (s featuresWorkspaceSchema) CalculateTuples(
	currentRepresentation, previousRepresentation *model.Representations,
	key model.ReporterResourceKey,
) (model.TuplesToReplicate, error) {
	result, err := s.schema.CalculateTuples(currentRepresentation, previousRepresentation, key)
	if err != nil {
		return model.TuplesToReplicate{}, err
	}

	var tuplesToCreate, tuplesToDelete []model.RelationsTuple
	if result.HasTuplesToCreate() {
		tuplesToCreate = append(tuplesToCreate, *result.TuplesToCreate()...)
	}
	if result.HasTuplesToDelete() {
		tuplesToDelete = append(tuplesToDelete, *result.TuplesToDelete()...)
	}

	for _, relationName := range workspaceWildcardRelations {
		currentValue := workspaceWildcardRelationValue(currentRepresentation, relationName)
		previousValue := workspaceWildcardRelationValue(previousRepresentation, relationName)

		if currentValue == featuresServiceWildcard && previousValue != featuresServiceWildcard {
			tuplesToCreate = append(tuplesToCreate, model.NewRelationTupleForSubject(
				key, relationName, featureNamespace, "service", featuresServiceWildcardID,
			))
		}
		if previousValue == featuresServiceWildcard && currentValue != featuresServiceWildcard {
			tuplesToDelete = append(tuplesToDelete, model.NewRelationTupleForSubject(
				key, relationName, featureNamespace, "service", featuresServiceWildcardID,
			))
		}
	}

	return model.NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
}

// workspaceWildcardRelationValue returns the reporter value for fieldName when set, falling back to the common representation value.
func workspaceWildcardRelationValue(representation *model.Representations, fieldName string) string {
	if reporterValue := representation.ReporterStringField(fieldName); reporterValue != "" {
		return reporterValue
	}
	return representation.StringField(fieldName)
}
