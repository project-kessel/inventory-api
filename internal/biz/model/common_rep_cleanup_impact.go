package model

import (
	"fmt"
	"slices"

	"github.com/google/uuid"
)

// CommonRepCleanupImpact describes one batch's deleted rows and their roots.
// Root IDs stay internal to accounting and are never logged.
type CommonRepCleanupImpact struct {
	representations int64
	resourceRootIDs []ResourceId
}

// NewCommonRepCleanupImpact constructs immutable batch accounting.
func NewCommonRepCleanupImpact(representations int64, resourceRootIDs []ResourceId) (CommonRepCleanupImpact, error) {
	if representations < 0 || int64(len(resourceRootIDs)) > representations || (representations > 0 && len(resourceRootIDs) == 0) {
		return CommonRepCleanupImpact{}, fmt.Errorf("invalid common representation cleanup impact")
	}
	for _, resourceID := range resourceRootIDs {
		if resourceID.UUID() == uuid.Nil {
			return CommonRepCleanupImpact{}, fmt.Errorf("cleanup impact requires valid root identities")
		}
	}
	return CommonRepCleanupImpact{representations: representations, resourceRootIDs: slices.Clone(resourceRootIDs)}, nil
}

// Representations returns deleted rows in the batch.
func (impact CommonRepCleanupImpact) Representations() int64 { return impact.representations }

// ResourceRootIDs returns a copy of the roots used for distinct-root accounting.
func (impact CommonRepCleanupImpact) ResourceRootIDs() []ResourceId {
	return slices.Clone(impact.resourceRootIDs)
}
