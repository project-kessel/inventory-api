package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ReporterRepresentationMetadata represents the metadata fields for ReporterRepresentation
// This includes the composite key fields and other metadata fields
// (do not need to be versioned, they can change but only latest matters)
type ReporterRepresentationMetadata struct {
	ReporterRepresentationKey
	// Foreign key reference to ReporterRepresentation
	ReporterRepresentationID uuid.UUID `gorm:"type:text;column:reporter_representation_id;index"`
	// Other metadata fields
	APIHref     string  `gorm:"size:512;column:api_href"`
	ConsoleHref *string `gorm:"size:512;column:console_href"`
}

// BeforeCreate sets the foreign key reference if not already set
func (rrm *ReporterRepresentationMetadata) BeforeCreate(tx *gorm.DB) error {
	// The foreign key will be set by the application logic when creating the metadata
	// This hook is here for consistency and future extensibility
	return nil
}

// ReporterRepresentationKey represents the composite key fields for ReporterRepresentation
// Do not need to be versioned + composite ID + need foreign keys from rep_ref if we care to ensure that rep ref refers to a reporter rep that exists
type ReporterRepresentationKey struct {
	RepresentationType
	LocalResourceID    string `gorm:"size:128;column:local_resource_id;index:reporter_rep_unique_idx,unique"`
	ReporterInstanceID string `gorm:"size:128;column:reporter_instance_id;index:reporter_rep_unique_idx,unique"`
}
