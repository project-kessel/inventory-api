package model

type CommonRepresentation struct {
	BaseRepresentation
	ID                         string `gorm:"column:id;index:unique_common_rep_idx,unique"`
	ResourceType               string `gorm:"size:128;column:resource_type"`
	Version                    int    `gorm:"column:version;index:unique_common_rep_idx,unique"`
	ReportedByReporterType     string `gorm:"column:reported_by_reporter_type"`
	ReportedByReporterInstance string `gorm:"column:reported_by_reporter_instance"`
}

func (CommonRepresentation) TableName() string {
	return "common_representation"
}
