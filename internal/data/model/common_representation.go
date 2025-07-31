package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
)

// CommonRepresentation stores the *authoritative canonical state* for a resource across all reporters.  It
// tracks which reporter most recently supplied the data (`ReportedByReporterType/Instance`) alongside the
// shared `Version` counter used for optimistic concurrency.
type CommonRepresentation struct {
	Representation
	ResourceId                 uuid.UUID `gorm:"type:text;primaryKey"`
	Version                    uint      `gorm:"type:bigint;primaryKey;check:version >= 0"`
	ReportedByReporterType     string    `gorm:"size:128"`
	ReportedByReporterInstance string    `gorm:"size:128"`
	CreatedAt                  time.Time
}

// NewCommonRepresentation creates a fully-validated instance. Any field-level issues are returned as a
// single aggregated `ValidationError`.
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

	if err := validateCommonRepresentation(cr); err != nil {
		return nil, err
	}

	return cr, nil
}

func validateCommonRepresentation(cr *CommonRepresentation) error {
	return aggregateErrors(
		validateUUIDRequired("ResourceId", cr.ResourceId),
		validateMinValueUint("Version", cr.Version, MinVersionValue),
		validateStringRequired("ReportedByReporterType", cr.ReportedByReporterType),
		validateMaxLength("ReportedByReporterType", cr.ReportedByReporterType, MaxReporterTypeLength),
		validateStringRequired("ReportedByReporterInstance", cr.ReportedByReporterInstance),
		validateMaxLength("ReportedByReporterInstance", cr.ReportedByReporterInstance, MaxReporterInstanceIDLength),
	)
}
