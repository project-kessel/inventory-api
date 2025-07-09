package model

import (
	"fmt"

	"github.com/google/uuid"
)

// CommonRepresentation is an immutable value object representing common resource data.
// It follows DDD principles where value objects are immutable and should be created
// through factory methods that enforce validation rules.
// Note: Fields are exported for GORM compatibility but should not be modified directly.
type CommonRepresentation struct {
	BaseRepresentation
	ResourceId                 uuid.UUID `gorm:"type:text;column:id;primary_key"`
	ResourceType               string    `gorm:"size:128;column:resource_type"`
	Version                    uint      `gorm:"type:bigint;column:version;primary_key;check:version > 0"`
	ReportedByReporterType     string    `gorm:"size:128;column:reported_by_reporter_type"`
	ReportedByReporterInstance string    `gorm:"size:128;column:reported_by_reporter_instance"`
}

func (CommonRepresentation) TableName() string {
	return "common_representation"
}

// NewCommonRepresentationWithID creates a CommonRepresentation with a specific ID
// This is useful when you need to recreate an existing representation where the ResourceId
// is not owned by the CommonRepresentation and is generated elsewhere
func NewCommonRepresentationWithID(
	id uuid.UUID,
	data JsonObject,
	resourceType string,
	version uint,
	reportedByReporterType string,
	reportedByReporterInstance string,
) (*CommonRepresentation, error) {
	cr := &CommonRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: data,
		},
		ResourceId:                 id,
		ResourceType:               resourceType,
		Version:                    version,
		ReportedByReporterType:     reportedByReporterType,
		ReportedByReporterInstance: reportedByReporterInstance,
	}

	// Validate the instance
	if err := ValidateCommonRepresentation(cr); err != nil {
		return nil, fmt.Errorf("invalid CommonRepresentation: %w", err)
	}

	return cr, nil
}

// ValidateCommonRepresentation validates a CommonRepresentation instance
// This function is used by both factory methods and tests to ensure consistency
func ValidateCommonRepresentation(cr *CommonRepresentation) error {
	if cr.ResourceId == uuid.Nil {
		return ValidationError{Field: "ResourceId", Message: "cannot be empty"}
	}
	if cr.ResourceType == "" {
		return ValidationError{Field: "ResourceType", Message: "cannot be empty"}
	}
	if cr.Version == 0 {
		return ValidationError{Field: "Version", Message: "must be positive"}
	}
	if cr.ReportedByReporterType == "" {
		return ValidationError{Field: "ReportedByReporterType", Message: "cannot be empty"}
	}
	if cr.ReportedByReporterInstance == "" {
		return ValidationError{Field: "ReportedByReporterInstance", Message: "cannot be empty"}
	}
	if cr.Data == nil {
		return ValidationError{Field: "Data", Message: "cannot be nil"}
	}
	return nil
}
