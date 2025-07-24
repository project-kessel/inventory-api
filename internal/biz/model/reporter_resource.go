package model

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ReporterResourceKey represents the natural key that identifies a resource as reported by a specific reporter.
// This tuple must be unique across the table.
type ReporterResourceKey struct {
	LocalResourceID    string `gorm:"column:local_resource_id;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:1;not null;"`
	ReporterType       string `gorm:"size:128;column:reporter_type;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:2;not null;"`
	ResourceType       string `gorm:"size:128;column:resource_type;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:3;not null;"`
	ReporterInstanceID string `gorm:"size:256;column:reporter_instance_id;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:4;not null;"`
}

// ReporterResource represents the metadata that identifies a resource as reported by a specific reporter.
// It combines a surrogate UUID primary key with the natural composite key and latest state information.
type ReporterResource struct {
	// Surrogate Id for ReporterResourceKey
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`
	// Actual Id
	ReporterResourceKey

	// Fields that do not need versioning, only latest state matters
	ResourceID  uuid.UUID `gorm:"type:uuid;column:resource_id;not null;"`
	APIHref     string    `gorm:"size:256;column:api_href"`
	ConsoleHref string    `gorm:"size:256;column:console_href"`

	// Normalized Latest values
	RepresentationVersion int  `gorm:"column:representation_version;index:reporter_resource_key_idx,unique;not null;"`
	Generation            int  `gorm:"column:generation;index:reporter_resource_key_idx,unique;not null;"`
	Tombstone             bool `gorm:"column:tombstone;not null;"`
}

// TableName returns the table name for ReporterResource
func (ReporterResource) TableName() string {
	return ReporterResourceTableName
}

// NewReporterResource validates inputs and returns an immutable ReporterResource value.
func NewReporterResource(
	id uuid.UUID,
	localResourceID string,
	reporterType string,
	resourceType string,
	reporterInstanceID string,
	resourceID uuid.UUID,
	apiHref string,
	consoleHref string,
	representationVersion int,
	generation int,
	tombstone bool,
) (*ReporterResource, error) {
	rr := &ReporterResource{
		ID: id,
		ReporterResourceKey: ReporterResourceKey{
			LocalResourceID:    localResourceID,
			ReporterType:       reporterType,
			ResourceType:       resourceType,
			ReporterInstanceID: reporterInstanceID,
		},
		ResourceID:            resourceID,
		APIHref:               apiHref,
		ConsoleHref:           consoleHref,
		RepresentationVersion: representationVersion,
		Generation:            generation,
		Tombstone:             tombstone,
	}

	if err := validateReporterResource(rr); err != nil {
		return nil, err
	}
	return rr, nil
}

func validateReporterResource(r *ReporterResource) error {
	if r.ID == uuid.Nil {
		return ValidationError{Field: "ID", Message: "cannot be empty"}
	}

	if strings.TrimSpace(r.LocalResourceID) == "" {
		return ValidationError{Field: "LocalResourceID", Message: "cannot be empty"}
	}
	if len(r.LocalResourceID) > MaxLocalResourceIDLength {
		return ValidationError{Field: "LocalResourceID", Message: fmt.Sprintf("exceeds %d chars", MaxLocalResourceIDLength)}
	}

	if strings.TrimSpace(r.ReporterType) == "" {
		return ValidationError{Field: "ReporterType", Message: "cannot be empty"}
	}
	if len(r.ReporterType) > MaxReporterTypeLength {
		return ValidationError{Field: "ReporterType", Message: fmt.Sprintf("exceeds %d chars", MaxReporterTypeLength)}
	}

	if strings.TrimSpace(r.ResourceType) == "" {
		return ValidationError{Field: "ResourceType", Message: "cannot be empty"}
	}
	if len(r.ResourceType) > MaxResourceTypeLength {
		return ValidationError{Field: "ResourceType", Message: fmt.Sprintf("exceeds %d chars", MaxResourceTypeLength)}
	}

	if strings.TrimSpace(r.ReporterInstanceID) == "" {
		return ValidationError{Field: "ReporterInstanceID", Message: "cannot be empty"}
	}
	if len(r.ReporterInstanceID) > MaxReporterInstanceIDLength {
		return ValidationError{Field: "ReporterInstanceID", Message: fmt.Sprintf("exceeds %d chars", MaxReporterInstanceIDLength)}
	}

	if r.ResourceID == uuid.Nil {
		return ValidationError{Field: "ResourceID", Message: "cannot be empty"}
	}

	if r.Generation < MinGenerationValue {
		return ValidationError{Field: "Generation", Message: "must be >= 0"}
	}

	if r.RepresentationVersion < 0 {
		return ValidationError{Field: "RepresentationVersion", Message: "must be >= 0"}
	}

	if len(r.APIHref) > 0 {
		if len(r.APIHref) > MaxAPIHrefLength {
			return ValidationError{Field: "APIHref", Message: fmt.Sprintf("exceeds %d chars", MaxAPIHrefLength)}
		}
		if err := validateURL(r.APIHref); err != nil {
			return ValidationError{Field: "APIHref", Message: err.Error()}
		}
	}

	if len(r.ConsoleHref) > 0 {
		if len(r.ConsoleHref) > MaxConsoleHrefLength {
			return ValidationError{Field: "ConsoleHref", Message: fmt.Sprintf("exceeds %d chars", MaxConsoleHrefLength)}
		}
		if err := validateURL(r.ConsoleHref); err != nil {
			return ValidationError{Field: "ConsoleHref", Message: err.Error()}
		}
	}

	return nil
}
