package model

import (
	"fmt"
	"strings"
)

// ReporterRepresentation is an immutable value object representing reporter-specific resource data.
// It follows DDD principles where value objects are immutable and should be created
// through factory methods that enforce validation rules.
// Note: Fields are exported for GORM compatibility but should not be modified directly.
type ReporterRepresentation struct {
	Representation

	ReporterResourceID string  `gorm:"size:128;column:reporter_resource_id;primaryKey"`
	Version            uint    `gorm:"type:bigint;column:version;primaryKey;check:version >= 0"`
	Generation         uint    `gorm:"type:bigint;column:generation;check:generation >= 0"`
	ReporterVersion    *string `gorm:"size:128;column:reporter_version"`
	CommonVersion      uint    `gorm:"type:bigint;column:common_version;check:common_version >= 0"`
	Tombstone          bool    `gorm:"column:tombstone"`
}

func (ReporterRepresentation) TableName() string {
	return ReporterRepresentationTableName
}

// NewReporterRepresentation Factory method for creating a new ReporterRepresentation
// This enforces immutability by validating all inputs and creating a valid instance
func NewReporterRepresentation(
	data JsonObject,
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
	if rr.ReporterResourceID == "" || strings.TrimSpace(rr.ReporterResourceID) == "" {
		return ValidationError{Field: "ReporterResourceID", Message: "cannot be empty"}
	}
	if rr.Version < MinVersionValue {
		return ValidationError{Field: "Version", Message: fmt.Sprintf("must be >= %d", MinVersionValue)}
	}
	if rr.Generation < MinGenerationValue {
		return ValidationError{Field: "Generation", Message: fmt.Sprintf("must be >= %d", MinGenerationValue)}
	}
	if rr.CommonVersion < MinCommonVersion {
		return ValidationError{Field: "CommonVersion", Message: fmt.Sprintf("must be >= %d", MinCommonVersion)}
	}
	if rr.ReporterVersion != nil && len(*rr.ReporterVersion) > MaxReporterVersionLength {
		return ValidationError{Field: "ReporterVersion", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxReporterVersionLength)}
	}
	// Data can be nil - this is a valid state
	return nil
}
