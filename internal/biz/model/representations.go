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
	commonVersion                 *Version
	reporterData                  Representation
	reporterRepresentationVersion *Version
}

// NewRepresentations creates a Representations with optional common and reporter data.
// At least one of common or reporter representation must be provided.
// If a representation is provided, its version must also be provided.
func NewRepresentations(
	commonData Representation,
	commonVersion *Version,
	reporterData Representation,
	reporterRepresentationVersion *Version,
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
func (r *Representations) CommonVersion() *Version {
	return r.commonVersion
}

// HasCommon returns true if common representation is present.
func (r *Representations) HasCommon() bool {
	return len(r.commonData) > 0 && r.commonVersion != nil
}

// WorkspaceID returns the workspace_id from the common representation data.
// Returns empty string if not present or if common representation is not available.
func (r *Representations) WorkspaceID() string {
	return r.StringField("workspace_id")
}

// StringField returns a single string value from the common representation.
// Returns empty string if not present, not a string, or if common representation is unavailable.
func (r *Representations) StringField(fieldName string) string {
	if r != nil && r.HasCommon() {
		if value, ok := r.commonData[fieldName].(string); ok {
			return value
		}
	}
	return ""
}

// StringSliceField returns a string slice from the common representation.
// Returns nil if not present, not an array, or if common representation is unavailable.
// Non-string elements within the array are silently skipped.
func (r *Representations) StringSliceField(fieldName string) []string {
	if r == nil || !r.HasCommon() {
		return nil
	}
	raw, ok := r.commonData[fieldName]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
