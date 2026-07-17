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

var serviceRelations = []model.RelationDef{
	mustRelationDef("allowed_workspaces", "allowed_workspaces", model.RbacNamespace, "workspace", true),
	mustRelationDef("billing_account", "billing_account", featureNamespace, "billing_account", true),
	mustRelationDef("parent", "parent", featureNamespace, "service", false),
}

func NewFeaturesServiceSchemaFromString(jsonSchema string) model.Schema {
	return NewJsonSchemaWithRelations(jsonSchema, serviceRelations)
}

var billingAccountRelations = []model.RelationDef{
	mustRelationDef("workspaces", "workspace", model.RbacNamespace, "workspace", true),
}

func NewFeaturesBillingAccountSchemaFromString(jsonSchema string) model.Schema {
	return NewJsonSchemaWithRelations(jsonSchema, billingAccountRelations)
}
