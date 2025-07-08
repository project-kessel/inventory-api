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
	BaseRepresentation

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
	return "reporter_representation"
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
		BaseRepresentation: BaseRepresentation{
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

// newReporterRepresentationForTest creates a ReporterRepresentation for testing with all parameters
// This is unexported and only available to tests via export_test.go

// ValidateReporterRepresentation validates a ReporterRepresentation instance
// This function is used by both factory methods and tests to ensure consistency
func ValidateReporterRepresentation(rr *ReporterRepresentation) error {
	if rr.LocalResourceID == "" || strings.TrimSpace(rr.LocalResourceID) == "" {
		return ValidationError{Field: "LocalResourceID", Message: "cannot be empty"}
	}
	if len(rr.LocalResourceID) > 128 {
		return ValidationError{Field: "LocalResourceID", Message: "exceeds maximum length of 128 characters"}
	}
	if rr.ReporterType == "" || strings.TrimSpace(rr.ReporterType) == "" {
		return ValidationError{Field: "ReporterType", Message: "cannot be empty"}
	}
	if len(rr.ReporterType) > 128 {
		return ValidationError{Field: "ReporterType", Message: "exceeds maximum length of 128 characters"}
	}
	if rr.ResourceType == "" || strings.TrimSpace(rr.ResourceType) == "" {
		return ValidationError{Field: "ResourceType", Message: "cannot be empty"}
	}
	if len(rr.ResourceType) > 128 {
		return ValidationError{Field: "ResourceType", Message: "exceeds maximum length of 128 characters"}
	}
	if rr.Version == 0 {
		return ValidationError{Field: "Version", Message: "must be a positive value"}
	}
	if rr.ReporterInstanceID == "" || strings.TrimSpace(rr.ReporterInstanceID) == "" {
		return ValidationError{Field: "ReporterInstanceID", Message: "cannot be empty"}
	}
	if len(rr.ReporterInstanceID) > 128 {
		return ValidationError{Field: "ReporterInstanceID", Message: "exceeds maximum length of 128 characters"}
	}
	if rr.Generation == 0 {
		return ValidationError{Field: "Generation", Message: "must be a positive value"}
	}
	if rr.APIHref != "" {
		if len(rr.APIHref) > 512 {
			return ValidationError{Field: "APIHref", Message: "exceeds maximum length of 512 characters"}
		}
		if err := validateURL(rr.APIHref); err != nil {
			return ValidationError{Field: "APIHref", Message: err.Error()}
		}
	}
	if rr.ConsoleHref != nil && *rr.ConsoleHref != "" {
		if len(*rr.ConsoleHref) > 512 {
			return ValidationError{Field: "ConsoleHref", Message: "exceeds maximum length of 512 characters"}
		}
		if err := validateURL(*rr.ConsoleHref); err != nil {
			return ValidationError{Field: "ConsoleHref", Message: err.Error()}
		}
	}
	if rr.CommonVersion == 0 {
		return ValidationError{Field: "CommonVersion", Message: "must be positive"}
	}
	if rr.ReporterVersion != nil && len(*rr.ReporterVersion) > 128 {
		return ValidationError{Field: "ReporterVersion", Message: "exceeds maximum length of 128 characters"}
	}
	if rr.Data == nil {
		return ValidationError{Field: "Data", Message: "cannot be nil"}
	}
	return nil
}
