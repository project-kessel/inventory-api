package model

// ReporterRepresentationMetadata represents the metadata fields for ReporterRepresentation
// This includes the composite key fields and other metadata fields
// (do not need to be versioned, they can change but only latest matters)
type ReporterRepresentationMetadata struct {
	ReporterRepresentationKey
	// Other metadata fields
	APIHref     string  `gorm:"size:512;column:api_href"`
	ConsoleHref *string `gorm:"size:512;column:console_href"`
}

// ReporterRepresentationKey represents the composite key fields for ReporterRepresentation
// Do not need to be versioned + composite ID + need foreign keys from rep_ref if we care to ensure that rep ref refers to a reporter rep that exists
type ReporterRepresentationKey struct {
	RepresentationType
	LocalResourceID    string `gorm:"size:128;column:local_resource_id;index:reporter_rep_unique_idx,unique"`
	ReporterInstanceID string `gorm:"size:128;column:reporter_instance_id;index:reporter_rep_unique_idx,unique"`
}
