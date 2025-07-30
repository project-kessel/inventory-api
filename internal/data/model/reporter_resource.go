package model

import (
	"time"

	"github.com/google/uuid"
)

// ReporterResourceKey represents the natural key that identifies a resource as reported by a specific reporter.
// This tuple must be unique across the table.
type ReporterResourceKey struct {
	LocalResourceID    string `gorm:"size:256;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:1;not null"`
	ReporterType       string `gorm:"size:128;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:2;not null"`
	ResourceType       string `gorm:"size:128;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:3;not null"`
	ReporterInstanceID string `gorm:"size:256;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:4;not null"`
}

// ReporterResource represents the metadata that identifies a resource as reported by a specific reporter.
// It combines a surrogate UUID primary key with the natural composite key and latest state information.
type ReporterResource struct {
	// Surrogate Id for ReporterResourceKey
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`
	// Actual Id
	ReporterResourceKey

	// Fields that do not need versioning, only latest state matters
	ResourceID  uuid.UUID `gorm:"type:uuid;not null"`
	APIHref     string    `gorm:"size:512;not null"`
	ConsoleHref string    `gorm:"size:512"`

	// Normalized Latest values
	RepresentationVersion uint `gorm:"index:reporter_resource_key_idx,unique;not null"`
	Generation            uint `gorm:"index:reporter_resource_key_idx,unique;not null"`
	Tombstone             bool `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
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
	representationVersion uint,
	generation uint,
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
	return aggregateErrors(
		validateUUIDRequired("ID", r.ID),
		validateStringRequired("LocalResourceID", r.LocalResourceID),
		validateMaxLength("LocalResourceID", r.LocalResourceID, MaxLocalResourceIDLength),
		validateStringRequired("ReporterType", r.ReporterType),
		validateMaxLength("ReporterType", r.ReporterType, MaxReporterTypeLength),
		validateStringRequired("ResourceType", r.ResourceType),
		validateMaxLength("ResourceType", r.ResourceType, MaxResourceTypeLength),
		validateStringRequired("ReporterInstanceID", r.ReporterInstanceID),
		validateMaxLength("ReporterInstanceID", r.ReporterInstanceID, MaxReporterInstanceIDLength),
		validateUUIDRequired("ResourceID", r.ResourceID),
		validateMinValue("Generation", r.Generation, MinGenerationValue),
		validateMinValue("RepresentationVersion", r.RepresentationVersion, 0),
		validateOptionalURL("APIHref", r.APIHref, MaxAPIHrefLength),
		validateOptionalURL("ConsoleHref", r.ConsoleHref, MaxConsoleHrefLength),
	)
}
