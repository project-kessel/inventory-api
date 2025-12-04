package model

import (
	"fmt"
)

// Representations encapsulates common and reporter representations with their respective versions
// for a specific point in time (e.g., current or previous).
// At least one of common or reporter representation must be present, but not both can be nil.
// If a representation is present, its version must also be present (and vice versa).
type Representations struct {
	commonData                    Representation
	commonVersion                 *uint
	reporterData                  Representation
	reporterRepresentationVersion *uint
}

// NewRepresentations creates a Representations with optional common and reporter data.
// At least one of common or reporter representation must be provided.
// If a representation is provided, its version must also be provided.
func NewRepresentations(
	commonData Representation,
	commonVersion *uint,
	reporterData Representation,
	reporterRepresentationVersion *uint,
) (*Representations, error) {
	// Validate that at least one representation is present
	hasCommon := len(commonData) > 0 && commonVersion != nil
	hasReporter := len(reporterData) > 0 && reporterRepresentationVersion != nil

	if !hasCommon && !hasReporter {
		return nil, fmt.Errorf("at least one of common or reporter representation must be present")
	}

	// Validate that if common data is present, version must be present (and vice versa)
	if (len(commonData) > 0) != (commonVersion != nil) {
		return nil, fmt.Errorf("common data and common version must both be present or both be absent")
	}

	// Validate that if reporter data is present, version must be present (and vice versa)
	if (len(reporterData) > 0) != (reporterRepresentationVersion != nil) {
		return nil, fmt.Errorf("reporter data and reporter representation version must both be present or both be absent")
	}

	return &Representations{
		commonData:                    commonData,
		commonVersion:                 commonVersion,
		reporterData:                  reporterData,
		reporterRepresentationVersion: reporterRepresentationVersion,
	}, nil
}

// CommonData returns the common representation data, or nil if not present.
func (r *Representations) CommonData() Representation {
	return r.commonData
}

// CommonVersion returns a pointer to the common version, or nil if not present.
func (r *Representations) CommonVersion() *uint {
	return r.commonVersion
}

// HasCommon returns true if common representation is present.
func (r *Representations) HasCommon() bool {
	return len(r.commonData) > 0 && r.commonVersion != nil
}

// WorkspaceID returns the workspace_id from the common representation data.
// Returns empty string if not present or if common representation is not available.
func (r *Representations) WorkspaceID() string {
	if r != nil && r.HasCommon() {
		if workspaceID, ok := r.commonData["workspace_id"].(string); ok {
			return workspaceID
		}
	}
	return ""
}
