package model

import (
	"fmt"

	"github.com/google/uuid"
)

const (
	// CommonRepExclusionNone identifies a root eligible for retention.
	CommonRepExclusionNone = ""
	// CommonRepExclusionNoActiveReporter excludes roots owned by tombstone cleanup.
	CommonRepExclusionNoActiveReporter = "no_active_reporter"
	// CommonRepExclusionInconsistentPointer excludes an unresolved current version.
	CommonRepExclusionInconsistentPointer = "inconsistent_pointer"
)

// CommonRepRootClassification describes one resource's retention eligibility.
type CommonRepRootClassification struct {
	resourceID   ResourceId
	exclusion    string
	prunableRows int64
}

// NewCommonRepRootClassification validates a root's classification.
func NewCommonRepRootClassification(resourceID ResourceId, exclusion string, prunableRows int64) (CommonRepRootClassification, error) {
	if resourceID.UUID() == uuid.Nil || prunableRows < 0 {
		return CommonRepRootClassification{}, fmt.Errorf("invalid cleanup root classification")
	}
	switch exclusion {
	case CommonRepExclusionNone:
	case CommonRepExclusionNoActiveReporter, CommonRepExclusionInconsistentPointer:
		if prunableRows != 0 {
			return CommonRepRootClassification{}, fmt.Errorf("excluded roots cannot have prunable rows")
		}
	default:
		return CommonRepRootClassification{}, fmt.Errorf("unknown cleanup exclusion %q", exclusion)
	}
	return CommonRepRootClassification{resourceID: resourceID, exclusion: exclusion, prunableRows: prunableRows}, nil
}

// ResourceID returns the classified root.
func (classification CommonRepRootClassification) ResourceID() ResourceId {
	return classification.resourceID
}

// Exclusion returns the exclusion reason, or the empty string for eligible roots.
func (classification CommonRepRootClassification) Exclusion() string { return classification.exclusion }

// PrunableRows returns the eligible rows observed by the scan.
func (classification CommonRepRootClassification) PrunableRows() int64 {
	return classification.prunableRows
}
