package model

import (
	"database/sql/driver"
	"encoding/json"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"time"
)

type Relationship struct {
	ID               uint64 `gorm:"primarykey"`
	OrgId            string `gorm:"index"`
	RelationshipData JsonObject
	RelationshipType string
	SubjectId        uint64 `gorm:"index"`
	ObjectId         uint64 `gorm:"index"`
	Reporter         RelationshipReporter
	CreatedAt        *time.Time
	UpdatedAt        *time.Time
}

type RelationshipReporter struct {
	Reporter
	SubjectLocalResourceId string `json:"subject_local_resource_id"`
	ObjectLocalResourceId  string `json:"object_local_resource_id"`
}

func (RelationshipReporter) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	return GormDBDataType(db, field)
}

func (data RelationshipReporter) Value() (driver.Value, error) {
	return json.Marshal(data)
}

func (data *RelationshipReporter) Scan(value interface{}) error {
	return Scan(value, data)
}
