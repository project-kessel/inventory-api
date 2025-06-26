package v1beta2

type ReporterRepresentation struct {
	BaseRepresentation

	LocalResourceID    string `gorm:"column:local_resource_id;index:unique_reporter_rep_idx,unique"`
	ReporterType       string `gorm:"size:128;column:reporter_type;index:unique_reporter_rep_idx,unique"`
	ResourceType       string `gorm:"size:128;column:resource_type;index:unique_reporter_rep_idx,unique"`
	Version            int    `gorm:"column:version;index:unique_reporter_rep_idx,unique"`
	ReporterInstanceID string `gorm:"size:256;column:reporter_instance_id;unique_reporter_rep_idx,unique"`
	Generation         int    `gorm:"column:generation;unique_reporter_rep_idx,unique"`
	APIHref            string `gorm:"size:256;column:api_href"`
	ConsoleHref        string `gorm:"size:256;column:console_href"`
	CommonVersion      int    `gorm:"column:common_version"`
	Tombstone          bool   `gorm:"column:tombstone"`
	ReporterVersion    string `gorm:"column:reporter_version"`
}

func (ReporterRepresentation) TableName() string {
	return "reporter_representation"
}
