package model

// RepresentationType represents the type of representation
type RepresentationType struct {
	ResourceType string `gorm:"size:128;column:resource_type;index:reporter_rep_unique_idx,unique"`
	ReporterType string `gorm:"size:128;column:reporter_type;index:reporter_rep_unique_idx,unique"`
}
