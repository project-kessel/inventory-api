package model

import "github.com/google/uuid"

type CommonRepresentation struct {
	BaseRepresentation
	ID                         uuid.UUID `gorm:"type:uuid;column:id;primary_key;default:gen_random_uuid()"`
	ResourceType               string    `gorm:"size:128;column:resource_type"`
	Version                    int       `gorm:"column:version;primary_key"`
	ReportedByReporterType     string    `gorm:"column:reported_by_reporter_type"`
	ReportedByReporterInstance string    `gorm:"column:reported_by_reporter_instance"`
}

func (CommonRepresentation) TableName() string {
	return "common_representation"
}
