package model

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
