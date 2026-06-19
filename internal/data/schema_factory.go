package data

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// DefaultSchemaFactory wraps NewJsonSchemaWithWorkspacesFromString as a
// ResourceTypeSchemaFactory. It ignores the resource type and always
// creates a JsonSchemaWithWorkspaces (workspace-only tuple logic).
func DefaultSchemaFactory(_ model.ResourceType, jsonSchema string) model.Schema {
	return NewJsonSchemaWithWorkspacesFromString(jsonSchema)
}

// NewFeaturesAwareSchemaFactory returns a ResourceTypeSchemaFactory that
// dispatches to per-resource-type Schema implementations for Features
// service types, falling back to JsonSchemaWithWorkspaces for everything else.
func NewFeaturesAwareSchemaFactory() model.ResourceTypeSchemaFactory {
	serviceType := model.DeserializeResourceType("service")
	billingAccountType := model.DeserializeResourceType("billing_account")

	return func(resourceType model.ResourceType, jsonSchema string) model.Schema {
		switch resourceType {
		case serviceType:
			return NewFeaturesServiceSchemaFromString(jsonSchema)
		case billingAccountType:
			return NewFeaturesBillingAccountSchemaFromString(jsonSchema)
		default:
			return NewJsonSchemaWithWorkspacesFromString(jsonSchema)
		}
	}
}
