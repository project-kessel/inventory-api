package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
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
	return bizmodel.AggregateErrors(
		bizmodel.ValidateUUIDRequired("ResourceId", cr.ResourceId),
		bizmodel.ValidateMinValueUint("Version", cr.Version, MinVersionValue),
		bizmodel.ValidateStringRequired("ReportedByReporterType", cr.ReportedByReporterType),
		bizmodel.ValidateMaxLength("ReportedByReporterType", cr.ReportedByReporterType, MaxReporterTypeLength),
		bizmodel.ValidateStringRequired("ReportedByReporterInstance", cr.ReportedByReporterInstance),
		bizmodel.ValidateMaxLength("ReportedByReporterInstance", cr.ReportedByReporterInstance, MaxReporterInstanceIDLength),
	)
}

// SerializeToSnapshot converts GORM CommonRepresentation to snapshot type - direct initialization without validation
func (cr CommonRepresentation) SerializeToSnapshot() bizmodel.CommonRepresentationSnapshot {
	// Create representation snapshot
	representationSnapshot := bizmodel.RepresentationSnapshot{
		Data: cr.Data,
	}

	return bizmodel.CommonRepresentationSnapshot{
		Representation:             representationSnapshot,
		ResourceId:                 cr.ResourceId,
		Version:                    cr.Version,
		ReportedByReporterType:     cr.ReportedByReporterType,
		ReportedByReporterInstance: cr.ReportedByReporterInstance,
		CreatedAt:                  cr.CreatedAt,
	}
}

// DeserializeFromSnapshot creates GORM CommonRepresentation from snapshot - direct initialization without validation
func DeserializeCommonRepresentationFromSnapshot(snapshot bizmodel.CommonRepresentationSnapshot) CommonRepresentation {
	return CommonRepresentation{
		Representation: Representation{
			Data: snapshot.Representation.Data,
		},
		ResourceId:                 snapshot.ResourceId,
		Version:                    snapshot.Version,
		ReportedByReporterType:     snapshot.ReportedByReporterType,
		ReportedByReporterInstance: snapshot.ReportedByReporterInstance,
		CreatedAt:                  snapshot.CreatedAt,
	}
}
