package data

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

const featureNamespace = "features"

func mustRelationDef(fieldName, relationName, subjectNamespace, subjectResourceType string, multiValued bool) model.RelationDef {
	rd, err := model.NewRelationDef(fieldName, relationName, subjectNamespace, subjectResourceType, multiValued)
	if err != nil {
		panic(err)
	}
	return rd
}

// billingAccountRelations are the Features billing account relations used when
// reporter-derived tuples are enabled: the billing account tracks the services
// that report through it.
var billingAccountRelations = []model.RelationDef{
	mustRelationDef("services", "services", featureNamespace, "service", true),
}

// NewFeaturesBillingAccountSchemaFromString creates the reporter-aware Features
// billing account schema.
func NewFeaturesBillingAccountSchemaFromString(jsonSchema string) model.Schema {
	return NewJsonSchemaWithRelations(jsonSchema, billingAccountRelations)
}

// workspaceRelations are the Features workspace relations carried by the
// reporter representation.
var workspaceRelations = []model.RelationDef{
	mustRelationDef("direct_billing_account", "direct_billing_account", featureNamespace, "billing_account", false),
	mustRelationDef("direct_service_preferences", "direct_service_preferences", featureNamespace, "service", true),
}

// workspaceWildcardRelations are boolean reporter fields backed by a typed
// wildcard tuple (features/service:*). true writes the tuple; false and
// omission both mean the tuple is absent.
var workspaceWildcardRelations = []string{
	"desire_all_services",
	"ignore_inherited_desired_services",
	"ignore_inherited_paid_services",
}

// featuresWorkspaceSchema adds the boolean wildcard relations to the
// table-driven workspace relations.
type featuresWorkspaceSchema struct {
	JsonSchemaWithRelations
}

// NewFeaturesWorkspaceSchemaFromString creates the reporter-aware Features
// workspace schema.
func NewFeaturesWorkspaceSchemaFromString(jsonSchema string) model.Schema {
	return featuresWorkspaceSchema{JsonSchemaWithRelations{jsonSchema: jsonSchema, relations: workspaceRelations}}
}

// CalculateTuples diffs the wildcard relations the same way as the other
// relations: a tuple is created when a field becomes true and deleted when it
// stops being true.
func (s featuresWorkspaceSchema) CalculateTuples(
	current, previous *model.Representations,
	key model.ReporterResourceKey,
) (model.TuplesToReplicate, error) {
	result, err := s.JsonSchemaWithRelations.CalculateTuples(current, previous, key)
	if err != nil {
		return model.TuplesToReplicate{}, err
	}

	var creates, deletes []model.RelationsTuple
	if result.HasTuplesToCreate() {
		creates = *result.TuplesToCreate()
	}
	if result.HasTuplesToDelete() {
		deletes = *result.TuplesToDelete()
	}

	for _, relation := range workspaceWildcardRelations {
		c, d := model.DiffRelationValues(key, relation, featureNamespace, "service",
			wildcardValues(current, relation), wildcardValues(previous, relation))
		creates = append(creates, c...)
		deletes = append(deletes, d...)
	}

	return model.NewTuplesToReplicate(creates, deletes)
}

func wildcardValues(r *model.Representations, field string) []string {
	if r.ReporterBoolField(field) {
		return []string{"*"}
	}
	return nil
}

// The legacy definitions below reproduce the tuple model that shipped before
// reporter-derived tuples. They are used while
// consumer.load-reporter-representation is disabled, so that enabling the flag
// is the only thing that changes tuple output.

// legacyServiceRelations are the Features service relations emitted when
// reporter-derived tuples are disabled.
var legacyServiceRelations = []model.RelationDef{
	mustRelationDef("allowed_workspaces", "allowed_workspaces", model.RbacNamespace, "workspace", true),
	mustRelationDef("billing_account", "billing_account", featureNamespace, "billing_account", true),
	mustRelationDef("parent", "parent", featureNamespace, "service", false),
}

// NewLegacyFeaturesServiceSchemaFromString creates the pre-reporter-tuples
// Features service schema.
func NewLegacyFeaturesServiceSchemaFromString(jsonSchema string) model.Schema {
	return NewJsonSchemaWithRelations(jsonSchema, legacyServiceRelations)
}

// legacyBillingAccountRelations are the Features billing account relations
// emitted when reporter-derived tuples are disabled.
var legacyBillingAccountRelations = []model.RelationDef{
	mustRelationDef("workspaces", "workspace", model.RbacNamespace, "workspace", true),
}

// NewLegacyFeaturesBillingAccountSchemaFromString creates the
// pre-reporter-tuples Features billing account schema.
func NewLegacyFeaturesBillingAccountSchemaFromString(jsonSchema string) model.Schema {
	return NewJsonSchemaWithRelations(jsonSchema, legacyBillingAccountRelations)
}
