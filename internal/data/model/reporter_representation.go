package model

import (
	"time"

	"github.com/project-kessel/inventory-api/internal"
)

// ReporterRepresentation is an immutable value object representing reporter-specific resource
// It follows DDD principles where value objects are immutable and should be created
// through factory methods that enforce validation rules.
// Note: Fields are exported for GORM compatibility but should not be modified directly.
type ReporterRepresentation struct {
	Representation

	ReporterResourceID string  `gorm:"size:128;column:reporter_resource_id;primaryKey"`
	Version            uint    `gorm:"type:bigint;column:version;primaryKey;check:version >= 0"`
	Generation         uint    `gorm:"type:bigint;column:generation;primaryKey;check:generation >= 0"`
	ReporterVersion    *string `gorm:"size:128;column:reporter_version"`
	CommonVersion      uint    `gorm:"type:bigint;column:common_version;check:common_version >= 0"`
	Tombstone          bool    `gorm:"column:tombstone"`
	CreatedAt          time.Time

	// Foreign key constraint to ensure ReporterResourceID exists in ReporterResource table
	ReporterResource ReporterResource `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:ReporterResourceID;references:ID"`
}

// NewReporterRepresentation Factory method for creating a new ReporterRepresentation
// This enforces immutability by validating all inputs and creating a valid instance
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

	// Validate the instance
	if err := validateReporterRepresentation(rr); err != nil {
		return nil, err
	}

	return rr, nil
}

// validateReporterRepresentation validates a ReporterRepresentation instance
// This function is used internally by factory methods to ensure consistency
func validateReporterRepresentation(rr *ReporterRepresentation) error {
	return aggregateErrors(
		validateStringRequired("ReporterResourceID", rr.ReporterResourceID),
		validateMinValueUint("Version", rr.Version, MinVersionValue),
		validateMinValueUint("Generation", rr.Generation, MinGenerationValue),
		validateMinValueUint("CommonVersion", rr.CommonVersion, MinCommonVersion),
		validateOptionalString("ReporterVersion", rr.ReporterVersion, MaxReporterVersionLength),
		// Data can be nil - this is a valid state
	)
}
