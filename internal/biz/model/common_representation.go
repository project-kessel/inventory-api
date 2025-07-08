package model

import "github.com/google/uuid"

type CommonRepresentation struct {
	BaseRepresentation
	ID                         uuid.UUID `gorm:"type:uuid;column:id;primary_key;default:gen_random_uuid()"`
	ResourceType               string    `gorm:"size:128;column:resource_type"`
	Version                    uint      `gorm:"type:bigint;column:version;primary_key;check:version > 0"`
	ReportedByReporterType     string    `gorm:"size:128;column:reported_by_reporter_type"`
	ReportedByReporterInstance string    `gorm:"size:128;column:reported_by_reporter_instance"`
}

func (CommonRepresentation) TableName() string {
	return "common_representation"
}
