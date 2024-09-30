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
	RelationshipData JsonObject
	RelationshipType string
	SubjectId        uint64
	SubjectResource  Resource `gorm:"foreignKey:SubjectId" json:"-"`
	ObjectId         uint64
	ObjectResource   Resource `gorm:"foreignKey:ObjectId" json:"-"`
	Reporter         RelationshipReporter
	CreatedAt        *time.Time
	UpdatedAt        *time.Time
}

type RelationshipReporter struct {
	Reporter
	SubjectLocalResourceId string
	ObjectLocalResourceId  string
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
