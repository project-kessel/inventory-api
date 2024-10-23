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
	SubjectId        uint64 `gorm:"index;not null"`
	ObjectId         uint64 `gorm:"index;not null"`
	Reporter         RelationshipReporter
	CreatedAt        *time.Time
	UpdatedAt        *time.Time

	// Used to create FKs
	Subject Resource `gorm:"foreignKey:SubjectId"`
	Object  Resource `gorm:"foreignKey:ObjectId"`
}

type RelationshipReporter struct {
	Reporter
	SubjectLocalResourceId string `json:"subject_local_resource_id"`
	SubjectResourceType    string `json:"subject_resource_type"`
	ObjectLocalResourceId  string `json:"object_local_resource_id"`
	ObjectResourceType     string `json:"object_resource_type"`
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
