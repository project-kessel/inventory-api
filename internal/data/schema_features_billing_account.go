package data

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

var billingAccountRelations = []relationDef{
	{"workspaces", "workspace", model.RbacNamespace, "workspace", true},
}

func NewFeaturesBillingAccountSchemaFromString(jsonSchema string) model.Schema {
	return relationDefsSchema{jsonSchema: jsonSchema, relations: billingAccountRelations}
}
