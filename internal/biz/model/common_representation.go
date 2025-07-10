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
	Representation
	ResourceId                 uuid.UUID `gorm:"type:text;column:id;primaryKey"`
	ResourceType               string    `gorm:"size:128;column:resource_type"`
	Version                    uint      `gorm:"type:bigint;column:version;primaryKey;check:version >= 0"`
	ReportedByReporterType     string    `gorm:"size:128;column:reported_by_reporter_type"`
	ReportedByReporterInstance string    `gorm:"size:128;column:reported_by_reporter_instance"`
}

func (CommonRepresentation) TableName() string {
	return CommonRepresentationTableName
}

// NewCommonRepresentation creates a CommonRepresentation
func NewCommonRepresentation(
	resourceId uuid.UUID,
	data JsonObject,
	resourceType string,
	version uint,
	reportedByReporterType string,
	reportedByReporterInstance string,
) (*CommonRepresentation, error) {
	cr := &CommonRepresentation{
		Representation: Representation{
			Data: data,
		},
		ResourceId:                 resourceId,
		ResourceType:               resourceType,
		Version:                    version,
		ReportedByReporterType:     reportedByReporterType,
		ReportedByReporterInstance: reportedByReporterInstance,
	}

	// Validate the instance
	if err := validateCommonRepresentation(cr); err != nil {
		return nil, err
	}

	return cr, nil
}

// validateCommonRepresentation validates a CommonRepresentation instance
// This function is used internally by factory methods to ensure consistency
func validateCommonRepresentation(cr *CommonRepresentation) error {
	if cr.ResourceId == uuid.Nil {
		return ValidationError{Field: "ResourceId", Message: "cannot be empty"}
	}
	if cr.ResourceType == "" {
		return ValidationError{Field: "ResourceType", Message: "cannot be empty"}
	}
	if len(cr.ResourceType) > MaxResourceTypeLength {
		return ValidationError{Field: "ResourceType", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxResourceTypeLength)}
	}
	if cr.Version < MinVersionValue {
		return ValidationError{Field: "Version", Message: fmt.Sprintf("must be >= %d", MinVersionValue)}
	}
	if cr.ReportedByReporterType == "" {
		return ValidationError{Field: "ReportedByReporterType", Message: "cannot be empty"}
	}
	if len(cr.ReportedByReporterType) > MaxReporterTypeLength {
		return ValidationError{Field: "ReportedByReporterType", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxReporterTypeLength)}
	}
	if cr.ReportedByReporterInstance == "" {
		return ValidationError{Field: "ReportedByReporterInstance", Message: "cannot be empty"}
	}
	if len(cr.ReportedByReporterInstance) > MaxReporterInstanceIDLength {
		return ValidationError{Field: "ReportedByReporterInstance", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxReporterInstanceIDLength)}
	}
	// Data can be nil - this is a valid state
	return nil
}
