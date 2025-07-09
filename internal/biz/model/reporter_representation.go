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

	LocalResourceID    string  `gorm:"size:128;column:local_resource_id;index:reporter_rep_unique_idx,unique"`
	ReporterType       string  `gorm:"size:128;column:reporter_type;index:reporter_rep_unique_idx,unique"`
	ResourceType       string  `gorm:"size:128;column:resource_type;index:reporter_rep_unique_idx,unique"`
	Version            uint    `gorm:"type:bigint;column:version;index:reporter_rep_unique_idx,unique;check:version > 0"`
	ReporterInstanceID string  `gorm:"size:128;column:reporter_instance_id;index:reporter_rep_unique_idx,unique"`
	Generation         uint    `gorm:"type:bigint;column:generation;index:reporter_rep_unique_idx,unique;check:generation > 0"`
	APIHref            string  `gorm:"size:512;column:api_href"`
	ConsoleHref        *string `gorm:"size:512;column:console_href"`
	CommonVersion      uint    `gorm:"type:bigint;column:common_version;check:common_version > 0"`
	Tombstone          bool    `gorm:"column:tombstone"`
	ReporterVersion    *string `gorm:"size:128;column:reporter_version"`
}

func (ReporterRepresentation) TableName() string {
	return ReporterRepresentationTableName
}

// Factory method for creating a new ReporterRepresentation
// This enforces immutability by validating all inputs and creating a valid instance
func NewReporterRepresentation(
	data JsonObject,
	localResourceID string,
	reporterType string,
	resourceType string,
	version uint,
	reporterInstanceID string,
	generation uint,
	apiHref string,
	consoleHref *string,
	commonVersion uint,
	tombstone bool,
	reporterVersion *string,
) (*ReporterRepresentation, error) {
	rr := &ReporterRepresentation{
		Representation: Representation{
			Data: data,
		},
		LocalResourceID:    localResourceID,
		ReporterType:       reporterType,
		ResourceType:       resourceType,
		Version:            version,
		ReporterInstanceID: reporterInstanceID,
		Generation:         generation,
		APIHref:            apiHref,
		ConsoleHref:        consoleHref,
		CommonVersion:      commonVersion,
		Tombstone:          tombstone,
		ReporterVersion:    reporterVersion,
	}

	// Validate the instance
	if err := ValidateReporterRepresentation(rr); err != nil {
		return nil, fmt.Errorf("invalid ReporterRepresentation: %w", err)
	}

	return rr, nil
}

// ValidateReporterRepresentation validates a ReporterRepresentation instance
// This function is used by both factory methods and tests to ensure consistency
func ValidateReporterRepresentation(rr *ReporterRepresentation) error {
	if rr.LocalResourceID == "" || strings.TrimSpace(rr.LocalResourceID) == "" {
		return ValidationError{Field: "LocalResourceID", Message: "cannot be empty"}
	}
	if len(rr.LocalResourceID) > MaxLocalResourceIDLength {
		return ValidationError{Field: "LocalResourceID", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxLocalResourceIDLength)}
	}
	if rr.ReporterType == "" || strings.TrimSpace(rr.ReporterType) == "" {
		return ValidationError{Field: "ReporterType", Message: "cannot be empty"}
	}
	if len(rr.ReporterType) > MaxReporterTypeLength {
		return ValidationError{Field: "ReporterType", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxReporterTypeLength)}
	}
	if rr.ResourceType == "" || strings.TrimSpace(rr.ResourceType) == "" {
		return ValidationError{Field: "ResourceType", Message: "cannot be empty"}
	}
	if len(rr.ResourceType) > MaxResourceTypeLength {
		return ValidationError{Field: "ResourceType", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxResourceTypeLength)}
	}
	if rr.Version < MinVersionValue {
		return ValidationError{Field: "Version", Message: fmt.Sprintf("must be >= %d", MinVersionValue)}
	}
	if rr.ReporterInstanceID == "" || strings.TrimSpace(rr.ReporterInstanceID) == "" {
		return ValidationError{Field: "ReporterInstanceID", Message: "cannot be empty"}
	}
	if len(rr.ReporterInstanceID) > MaxReporterInstanceIDLength {
		return ValidationError{Field: "ReporterInstanceID", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxReporterInstanceIDLength)}
	}
	if rr.Generation < MinGenerationValue {
		return ValidationError{Field: "Generation", Message: fmt.Sprintf("must be >= %d", MinGenerationValue)}
	}
	if rr.APIHref != "" {
		if len(rr.APIHref) > MaxAPIHrefLength {
			return ValidationError{Field: "APIHref", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxAPIHrefLength)}
		}
		if err := validateURL(rr.APIHref); err != nil {
			return ValidationError{Field: "APIHref", Message: err.Error()}
		}
	}
	if rr.ConsoleHref != nil && *rr.ConsoleHref != "" {
		if len(*rr.ConsoleHref) > MaxConsoleHrefLength {
			return ValidationError{Field: "ConsoleHref", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxConsoleHrefLength)}
		}
		if err := validateURL(*rr.ConsoleHref); err != nil {
			return ValidationError{Field: "ConsoleHref", Message: err.Error()}
		}
	}
	if rr.CommonVersion < MinCommonVersion {
		return ValidationError{Field: "CommonVersion", Message: fmt.Sprintf("must be >= %d", MinCommonVersion)}
	}
	if rr.ReporterVersion != nil && len(*rr.ReporterVersion) > MaxReporterVersionLength {
		return ValidationError{Field: "ReporterVersion", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxReporterVersionLength)}
	}
	if rr.Data == nil {
		return ValidationError{Field: "Data", Message: "cannot be nil"}
	}
	return nil
}
