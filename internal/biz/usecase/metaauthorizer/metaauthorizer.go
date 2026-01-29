package metaauthorizer

import (
	"context"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// AuthzContext carries authentication/transport context into authorization decisions.
// Alias to authn/api to keep a single source of truth.
type AuthzContext = authnapi.AuthzContext

// MetaObject is a sealed interface representing objects for meta-authorization.
//
// Meta authorization objects are not simply inventory resources,
// as not all inventory operations may be against inventory resources.
//
// Implementations are restricted to this package via the private method.
// This pattern allows adding new object types in the future.
type MetaObject interface {
	metaObject() // private method seals the interface
}

// MetaAuthorizer provides a simplified authorization check interface for usecases.
type MetaAuthorizer interface {
	Check(ctx context.Context, object MetaObject, relation Relation, authzCtx AuthzContext) (bool, error)
}

// InventoryResource represents a specific inventory resource instance for meta-authorization.
// Uses strongly-typed model types for its fields.
type InventoryResource struct {
	reporterType    model.ReporterType
	resourceType    model.ResourceType
	localResourceId model.LocalResourceId
}

// NewInventoryResource creates a new InventoryResource for meta-authorization.
func NewInventoryResource(reporterType model.ReporterType, resourceType model.ResourceType, localResourceId model.LocalResourceId) InventoryResource {
	return InventoryResource{
		reporterType:    reporterType,
		resourceType:    resourceType,
		localResourceId: localResourceId,
	}
}

// NewInventoryResourceFromKey creates an InventoryResource from a ReporterResourceKey.
func NewInventoryResourceFromKey(key model.ReporterResourceKey) InventoryResource {
	return InventoryResource{
		reporterType:    key.ReporterType(),
		resourceType:    key.ResourceType(),
		localResourceId: key.LocalResourceId(),
	}
}

// ReporterType returns the reporter type of the resource.
func (ir InventoryResource) ReporterType() model.ReporterType { return ir.reporterType }

// ResourceType returns the resource type.
func (ir InventoryResource) ResourceType() model.ResourceType { return ir.resourceType }

// LocalResourceId returns the local resource identifier.
func (ir InventoryResource) LocalResourceId() model.LocalResourceId { return ir.localResourceId }

// metaObject seals the interface - only types in this package can implement MetaObject.
func (InventoryResource) metaObject() {}

// ResourceTypeRef represents a resource type reference for meta-authorization.
// Used when authorizing operations that apply to a type of resource rather than
// a specific resource instance (e.g., LookupResources).
type ResourceTypeRef struct {
	reporterType model.ReporterType
	resourceType model.ResourceType
}

// NewResourceTypeRef creates a new ResourceTypeRef for meta-authorization.
func NewResourceTypeRef(reporterType model.ReporterType, resourceType model.ResourceType) ResourceTypeRef {
	return ResourceTypeRef{
		reporterType: reporterType,
		resourceType: resourceType,
	}
}

// ReporterType returns the reporter type.
func (rt ResourceTypeRef) ReporterType() model.ReporterType { return rt.reporterType }

// ResourceType returns the resource type.
func (rt ResourceTypeRef) ResourceType() model.ResourceType { return rt.resourceType }

// metaObject seals the interface - only types in this package can implement MetaObject.
func (ResourceTypeRef) metaObject() {}
