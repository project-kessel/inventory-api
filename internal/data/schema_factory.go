package data

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// DefaultSchemaFactory wraps NewJsonSchemaWithWorkspacesFromString as a
// ResourceTypeSchemaFactory. It ignores the resource type and reporter flag,
// and always creates a JsonSchemaWithWorkspaces (workspace-only tuple logic).
func DefaultSchemaFactory(_ model.ResourceType, _ bool, jsonSchema string) model.Schema {
	return NewJsonSchemaWithWorkspacesFromString(jsonSchema)
}

var (
	workspaceType      = model.DeserializeResourceType("workspace")
	billingAccountType = model.DeserializeResourceType("billing_account")
)

// FeaturesAwareSchemaFactory dispatches to per-resource-type Schema
// implementations for Features reporter schemas.
// For common schemas, it falls back to JsonSchemaWithWorkspaces (default workspace logic).
func FeaturesAwareSchemaFactory(resourceType model.ResourceType, isReporter bool, jsonSchema string) model.Schema {
	// Only use Features-specific schemas for reporter schemas
	if isReporter {
		switch resourceType {
		case workspaceType:
			return NewFeaturesWorkspaceSchemaFromString(jsonSchema)
		case billingAccountType:
			return NewFeaturesBillingAccountSchemaFromString(jsonSchema)
		}
	}

	// For common schemas or non-Features types, use default workspace schema
	return NewJsonSchemaWithWorkspacesFromString(jsonSchema)
}
