package v1beta2

import (
	"context"
)

// CommonRepresentationRepository interface for common representation operations
type CommonRepresentationRepository interface {
	Create(ctx context.Context, commonRep *CommonRepresentation) (*CommonRepresentation, error)
}

// ReporterRepresentationRepository interface for reporter representation operations
type ReporterRepresentationRepository interface {
	Create(ctx context.Context, reporterRep *ReporterRepresentation) (*ReporterRepresentation, error)
}

// ResourceWithReferencesRepository interface for aggregate operations
type ResourceWithReferencesRepository interface {
	Create(ctx context.Context, resourceWithRefs *ResourceWithReferences) (*ResourceWithReferences, error)
	FindAllReferencesByReporterRepresentationId(ctx context.Context, reporterId ReporterRepresentationId) ([]*RepresentationReference, error)
}
