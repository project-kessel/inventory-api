package model

import (
	"time"

	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

// ReporterResourceKey represents the natural key that identifies **a single resource** as reported by a
// particular reporter instance. The combination of these four attributes must be unique. Keeping this as
// a dedicated embedded struct makes the composite-key explicit and reusable in both the database layer and
// higher-level domain/validation code.
type ReporterResourceKey struct {
	LocalResourceID    string `gorm:"size:256;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:1;not null"`
	ReporterType       string `gorm:"size:128;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:2;not null"`
	ResourceType       string `gorm:"size:128;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:3;not null"`
	ReporterInstanceID string `gorm:"size:256;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:4;not null"`
}

// ReporterResource is the *latest-state row* for a resource coming from a reporter. It combines an opaque
// surrogate UUID (`ID`) with the natural composite key (`ReporterResourceKey`).  Non-versioned fields
// (APIHref, ConsoleHref, Generation, Tombstone, …) always reflect the reporter’s most recent view. The
// struct is treated as an immutable value from a domain perspective – updates happen by inserting a new row
// via GORM where required, not by mutating an existing instance in-place.
type ReporterResource struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`
	ReporterResourceKey

	ResourceID  uuid.UUID `gorm:"type:uuid;not null"`
	APIHref     string    `gorm:"size:512;not null"`
	ConsoleHref string    `gorm:"size:512"`

	RepresentationVersion uint `gorm:"index:reporter_resource_key_idx,unique;not null"`
	Generation            uint `gorm:"index:reporter_resource_key_idx,unique;not null"`
	Tombstone             bool `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewReporterResource is the single constructor used across the code-base. It validates inputs, returning
// either a fully-populated *ReporterResource or a `ValidationError` aggregating all problems discovered in
// one pass.  This encourages call-sites to handle validation uniformly and avoids partially-initialised
// objects leaking into the domain.
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
	return bizmodel.AggregateErrors(
		bizmodel.ValidateUUIDRequired("ID", r.ID),
		bizmodel.ValidateStringRequired("LocalResourceID", r.LocalResourceID),
		bizmodel.ValidateMaxLength("LocalResourceID", r.LocalResourceID, MaxLocalResourceIDLength),
		bizmodel.ValidateStringRequired("ReporterType", r.ReporterType),
		bizmodel.ValidateMaxLength("ReporterType", r.ReporterType, MaxReporterTypeLength),
		bizmodel.ValidateStringRequired("ResourceType", r.ResourceType),
		bizmodel.ValidateMaxLength("ResourceType", r.ResourceType, MaxResourceTypeLength),
		bizmodel.ValidateStringRequired("ReporterInstanceID", r.ReporterInstanceID),
		bizmodel.ValidateMaxLength("ReporterInstanceID", r.ReporterInstanceID, MaxReporterInstanceIDLength),
		bizmodel.ValidateUUIDRequired("ResourceID", r.ResourceID),
		bizmodel.ValidateMinValue("Generation", r.Generation, MinGenerationValue),
		bizmodel.ValidateMinValue("RepresentationVersion", r.RepresentationVersion, 0),
		bizmodel.ValidateOptionalURL("APIHref", r.APIHref, MaxAPIHrefLength),
		bizmodel.ValidateOptionalURL("ConsoleHref", r.ConsoleHref, MaxConsoleHrefLength),
	)
}

// SerializeToSnapshot converts GORM ReporterResource to snapshot type - direct initialization without validation
func (rr ReporterResource) SerializeToSnapshot() bizmodel.ReporterResourceSnapshot {
	// Create ReporterResourceKey snapshot
	keySnapshot := bizmodel.ReporterResourceKeySnapshot{
		LocalResourceID:    rr.LocalResourceID,
		ReporterType:       rr.ReporterType,
		ResourceType:       rr.ResourceType,
		ReporterInstanceID: rr.ReporterInstanceID,
	}

	return bizmodel.ReporterResourceSnapshot{
		ID:                    rr.ID,
		ReporterResourceKey:   keySnapshot,
		ResourceID:            rr.ResourceID,
		APIHref:               rr.APIHref,
		ConsoleHref:           rr.ConsoleHref,
		RepresentationVersion: rr.RepresentationVersion,
		Generation:            rr.Generation,
		Tombstone:             rr.Tombstone,
		CreatedAt:             rr.CreatedAt,
		UpdatedAt:             rr.UpdatedAt,
	}
}

// DeserializeFromSnapshot creates GORM ReporterResource from snapshot - direct initialization without validation
func DeserializeReporterResourceFromSnapshot(snapshot bizmodel.ReporterResourceSnapshot) ReporterResource {
	return ReporterResource{
		ID: snapshot.ID,
		ReporterResourceKey: ReporterResourceKey{
			LocalResourceID:    snapshot.ReporterResourceKey.LocalResourceID,
			ReporterType:       snapshot.ReporterResourceKey.ReporterType,
			ResourceType:       snapshot.ReporterResourceKey.ResourceType,
			ReporterInstanceID: snapshot.ReporterResourceKey.ReporterInstanceID,
		},
		ResourceID:            snapshot.ResourceID,
		APIHref:               snapshot.APIHref,
		ConsoleHref:           snapshot.ConsoleHref,
		RepresentationVersion: snapshot.RepresentationVersion,
		Generation:            snapshot.Generation,
		Tombstone:             snapshot.Tombstone,
		CreatedAt:             snapshot.CreatedAt,
		UpdatedAt:             snapshot.UpdatedAt,
	}
}
