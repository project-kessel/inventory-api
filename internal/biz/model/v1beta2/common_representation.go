package v1beta2

type CommonRepresentation struct {
	BaseRepresentation
	LocalResourceID string `gorm:"column:local_resource_id;primaryKey"`
	ReporterType    string `gorm:"size:128;column:reporter_type"`
	ResourceType    string `gorm:"size:128;column:resource_type"`
	Version         int    `gorm:"column:version;primaryKey"`
	ReportedBy      string `gorm:"column:reported_by"`
}

func (CommonRepresentation) TableName() string {
	return "common_representation"
}
