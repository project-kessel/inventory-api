package model

import (
	"fmt"
)

// Representations encapsulates common and reporter representations with their respective versions
// for a specific point in time (e.g., current or previous).
// At least one version must be present (indicating that stream advanced).
// A version can exist without data (tombstone case - nil/empty data with version).
// Non-empty data requires a version.
type Representations struct {
	commonData                    Representation
	commonVersion                 *Version
	reporterData                  Representation
	reporterRepresentationVersion *Version
}

// NewRepresentations creates a Representations with optional common and reporter data.
// At least one version must be provided (indicating that stream advanced).
// Tombstones are represented as nil/empty data with a version.
// Non-empty data requires a version.
func NewRepresentations(
	commonData Representation,
	commonVersion *Version,
	reporterData Representation,
	reporterRepresentationVersion *Version,
) (*Representations, error) {
	// Validate that at least one version is present (at least one stream advanced)
	if commonVersion == nil && reporterRepresentationVersion == nil {
		return nil, fmt.Errorf("at least one version must be present")
	}

	// Validate that non-empty data requires a version
	// Note: A version can exist without data (tombstone case - empty/nil data with version)
	if len(commonData) > 0 && commonVersion == nil {
		return nil, fmt.Errorf("common data requires common version")
	}

	if len(reporterData) > 0 && reporterRepresentationVersion == nil {
		return nil, fmt.Errorf("reporter data requires reporter version")
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

// ReporterData returns the reporter representation data, or nil if not present.
func (r *Representations) ReporterData() Representation {
	return r.reporterData
}

// ReporterVersion returns a pointer to the reporter representation version, or nil if not present.
func (r *Representations) ReporterVersion() *Version {
	return r.reporterRepresentationVersion
}

// HasReporter returns true if reporter representation is present.
func (r *Representations) HasReporter() bool {
	return len(r.reporterData) > 0 && r.reporterRepresentationVersion != nil
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
