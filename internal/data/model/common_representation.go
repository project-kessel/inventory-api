package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
)

// CommonRepresentation is an immutable value object representing common resource
// It follows DDD principles where value objects are immutable and should be created
// through factory methods that enforce validation rules.
// Note: Fields are exported for GORM compatibility but should not be modified directly.
type CommonRepresentation struct {
	Representation
	ResourceId                 uuid.UUID `gorm:"type:text;column:id;primaryKey"`
	Version                    uint      `gorm:"type:bigint;column:version;primaryKey;check:version >= 0"`
	ReportedByReporterType     string    `gorm:"size:128;column:reported_by_reporter_type"`
	ReportedByReporterInstance string    `gorm:"size:128;column:reported_by_reporter_instance"`
	CreatedAt                  time.Time
}

// NewCommonRepresentation creates a CommonRepresentation
func NewCommonRepresentation(
	resourceId uuid.UUID,
	data internal.JsonObject,
	version uint,
	reportedByReporterType string,
	reportedByReporterInstance string,
) (*CommonRepresentation, error) {
	cr := &CommonRepresentation{
		Representation: Representation{
			Data: data,
		},
		ResourceId:                 resourceId,
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
	return aggregateErrors(
		validateUUIDRequired("ResourceId", cr.ResourceId),
		validateMinValueUint("Version", cr.Version, MinVersionValue),
		validateStringRequired("ReportedByReporterType", cr.ReportedByReporterType),
		validateMaxLength("ReportedByReporterType", cr.ReportedByReporterType, MaxReporterTypeLength),
		validateStringRequired("ReportedByReporterInstance", cr.ReportedByReporterInstance),
		validateMaxLength("ReportedByReporterInstance", cr.ReportedByReporterInstance, MaxReporterInstanceIDLength),
		// Data can be nil - this is a valid state
	)
}
