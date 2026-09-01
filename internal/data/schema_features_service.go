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

func NewFeaturesWorkspaceSchemaFromString(jsonSchema string) model.Schema {
	return NewJsonSchemaWithRelations(jsonSchema, workspaceRelations)
}
