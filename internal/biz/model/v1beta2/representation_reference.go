package v1beta2

import "github.com/google/uuid"

type RepresentationReference struct {
	ResourceID uuid.UUID `gorm:"type:uuid;column:resource_id;index:unique_rep_ref_idx,unique;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`

	LocalResourceID       string `gorm:"column:local_resource_id;index:search_rep_ref_idx"`
	ReporterType          string `gorm:"column:reporter_type;index:unique_rep_ref_idx,unique"`
	ResourceType          string `gorm:"column:resource_type;index:unique_rep_ref_idx,unique"`
	ReporterInstanceID    string `gorm:"column:reporter_instance_id;index:unique_rep_ref_idx,unique"`
	RepresentationVersion int    `gorm:"column:representation_version;index:unique_rep_ref_idx,unique"`
	Generation            int    `gorm:"column:generation;index:unique_rep_ref_idx,unique"`
	Tombstone             bool   `gorm:"column:tombstone"`
}

func (RepresentationReference) TableName() string {
	return "representation_references"
}
