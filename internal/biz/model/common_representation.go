package model

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CommonRepresentation is an immutable value object representing common resource data.
// It follows DDD principles where value objects are immutable and should be created
// through factory methods that enforce validation rules.
// Note: Fields are exported for GORM compatibility but should not be modified directly.
type CommonRepresentation struct {
	BaseRepresentation
	ID                         uuid.UUID `gorm:"type:text;column:id;primary_key"`
	ResourceType               string    `gorm:"size:128;column:resource_type"`
	Version                    uint      `gorm:"type:bigint;column:version;primary_key;check:version > 0"`
	ReportedByReporterType     string    `gorm:"size:128;column:reported_by_reporter_type"`
	ReportedByReporterInstance string    `gorm:"size:128;column:reported_by_reporter_instance"`
}

func (CommonRepresentation) TableName() string {
	return "common_representation"
}

// Factory method for creating a new CommonRepresentation
// This enforces immutability by validating all inputs and creating a valid instance
func NewCommonRepresentation(
	data JsonObject,
	resourceType string,
	version uint,
	reportedByReporterType string,
	reportedByReporterInstance string,
) (*CommonRepresentation, error) {
	// Generate a new UUID for the instance
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}

	// Create instance with provided data
	cr := &CommonRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: data,
		},
		ID:                         id,
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

// newCommonRepresentationWithID creates a CommonRepresentation with a specific ID
// This is useful for testing or when you need to recreate an existing representation
// This function is unexported and only available to tests via export_test.go
func newCommonRepresentationWithID(
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
		ID:                         id,
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
	if cr.ID == uuid.Nil {
		return ValidationError{Field: "ID", Message: "cannot be empty"}
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

// BeforeCreate hook generates UUID programmatically for cross-database compatibility.
// This approach is used instead of database-specific defaults (like PostgreSQL's gen_random_uuid())
// to ensure the model works correctly with both SQLite (used in tests) and PostgreSQL (production).
// SQLite doesn't support UUID type or gen_random_uuid() function, so we handle UUID generation
// in Go code using GORM hooks, following the same pattern as other models in the codebase.
func (cr *CommonRepresentation) BeforeCreate(db *gorm.DB) error {
	var err error
	if cr.ID == uuid.Nil {
		cr.ID, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("failed to generate uuid: %w", err)
		}
	}
	return nil
}
