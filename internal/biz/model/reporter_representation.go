package model

type ReporterRepresentation struct {
	BaseRepresentation

	LocalResourceID    string `gorm:"column:local_resource_id;index:reporter_rep_unique_idx,unique"`
	ReporterType       string `gorm:"size:128;column:reporter_type;index:reporter_rep_unique_idx,unique"`
	ResourceType       string `gorm:"size:128;column:resource_type;index:reporter_rep_unique_idx,unique"`
	Version            int    `gorm:"column:version;index:reporter_rep_unique_idx,unique"`
	ReporterInstanceID string `gorm:"size:256;column:reporter_instance_id;index:reporter_rep_unique_idx,unique"`
	Generation         int    `gorm:"column:generation;index:reporter_rep_unique_idx,unique"`
	APIHref            string `gorm:"size:256;column:api_href"`
	ConsoleHref        string `gorm:"size:256;column:console_href"`
	CommonVersion      int    `gorm:"column:common_version"`
	Tombstone          bool   `gorm:"column:tombstone"`
	ReporterVersion    string `gorm:"column:reporter_version"`
}

func (ReporterRepresentation) TableName() string {
	return "reporter_representation"
}
