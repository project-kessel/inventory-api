package v1beta2

type ReporterRepresentation struct {
	BaseRepresentation

	LocalResourceID    string `gorm:"column:local_resource_id;primaryKey"`
	ReporterType       string `gorm:"size:128;column:reporter_type;primaryKey"`
	ResourceType       string `gorm:"size:128;column:resource_type;primaryKey"`
	Version            int    `gorm:"column:version;primaryKey"`
	ReporterInstanceID string `gorm:"size:256;column:reporter_instance_id;primaryKey"`
	Generation         int    `gorm:"column:generation;primaryKey"`
	APIHref            string `gorm:"size:256;column:api_href"`
	ConsoleHref        string `gorm:"size:256;column:console_href"`
	CommonVersion      int    `gorm:"column:common_version"`
	Tombstone          bool   `gorm:"column:tombstone"`
	ReporterVersion    string `gorm:"column:reporter_version"`
}

func (ReporterRepresentation) TableName() string {
	return "reporter_representation"
}
