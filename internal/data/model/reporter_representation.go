package model

import (
	"time"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

// ReporterRepresentation captures the **reporter-specific view** of a resource.  Each reporter maintains
// its own version & generation counters which evolve independently of the `CommonRepresentation`.  The
// struct purposefully embeds `Representation` so the JSON `Data` blob remains first-class.
type ReporterRepresentation struct {
	Representation

	ReporterResourceID string  `gorm:"size:128;primaryKey"`
	Version            uint    `gorm:"type:bigint;primaryKey;check:version >= 0"`
	Generation         uint    `gorm:"type:bigint;primaryKey;check:generation >= 0"`
	ReporterVersion    *string `gorm:"size:128"`
	CommonVersion      uint    `gorm:"type:bigint;check:common_version >= 0"`
	Tombstone          bool    `gorm:"not null"`
	CreatedAt          time.Time

	ReporterResource ReporterResource `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:ReporterResourceID;references:ID"`
}

// NewReporterRepresentation is the ONLY factory for creating a ReporterRepresentation. It guarantees that
// every instance is fully validated (IDs present, counters non-negative, optional fields length-checked)
// before it enters the system.
func NewReporterRepresentation(
	data internal.JsonObject,
	reporterResourceID string,
	version uint,
	generation uint,
	commonVersion uint,
	tombstone bool,
	reporterVersion *string,
) (*ReporterRepresentation, error) {
	rr := &ReporterRepresentation{
		Representation: Representation{
			Data: data,
		},
		ReporterResourceID: reporterResourceID,
		Version:            version,
		Generation:         generation,
		CommonVersion:      commonVersion,
		Tombstone:          tombstone,
		ReporterVersion:    reporterVersion,
	}

	if err := validateReporterRepresentation(rr); err != nil {
		return nil, err
	}

	return rr, nil
}

func validateReporterRepresentation(rr *ReporterRepresentation) error {
	return bizmodel.AggregateErrors(
		bizmodel.ValidateStringRequired("ReporterResourceID", rr.ReporterResourceID),
		bizmodel.ValidateMinValueUint("Version", rr.Version, MinVersionValue),
		bizmodel.ValidateMinValueUint("Generation", rr.Generation, MinGenerationValue),
		bizmodel.ValidateMinValueUint("CommonVersion", rr.CommonVersion, MinCommonVersion),
		bizmodel.ValidateOptionalString("ReporterVersion", rr.ReporterVersion, MaxReporterVersionLength),
	)
}
