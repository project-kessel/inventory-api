package v1beta2

import (
	"context"

	"github.com/google/uuid"
)

// CommonRepresentationRepository interface for common representation operations
type CommonRepresentationRepository interface {
	Create(ctx context.Context, commonRep *CommonRepresentation) (*CommonRepresentation, error)
}

// ReporterRepresentationRepository interface for reporter representation operations
type ReporterRepresentationRepository interface {
	Create(ctx context.Context, reporterRep *ReporterRepresentation) (*ReporterRepresentation, error)
}

// RepresentationVersionUpdateFilter defines criteria for updating representation versions
type RepresentationVersionUpdateFilter struct {
	ResourceID      uuid.UUID
	ReporterType    *string // Optional - if nil, updates all reporter types
	LocalResourceID *string // Optional - if nil, updates all local resource IDs for the reporter type
}

// ResourceWithReferencesRepository interface for aggregate operations
type ResourceWithReferencesRepository interface {
	Create(ctx context.Context, resourceWithRefs *ResourceWithReferences) (*ResourceWithReferences, error)
	FindAllReferencesByReporterRepresentationId(ctx context.Context, reporterId ReporterRepresentationId) ([]*RepresentationReference, error)
	UpdateConsistencyToken(ctx context.Context, resourceID uuid.UUID, token string) error
	UpdateRepresentationVersion(ctx context.Context, filter RepresentationVersionUpdateFilter, newVersion int) (int64, error)
	// Convenience methods for common update scenarios
	UpdateCommonRepresentationVersion(ctx context.Context, resourceID uuid.UUID, newVersion int) (int64, error)
	UpdateReporterRepresentationVersion(ctx context.Context, resourceID uuid.UUID, reporterType string, localResourceID string, newVersion int) (int64, error)
}
