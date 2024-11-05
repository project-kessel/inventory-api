package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"time"
)

type Relationship struct {
	ID               uuid.UUID `gorm:"type:uuid;primarykey"`
	OrgId            string    `gorm:"index"`
	RelationshipData JsonObject
	RelationshipType string
	SubjectId        uuid.UUID `gorm:"type:uuid;index;not null"`
	ObjectId         uuid.UUID `gorm:"type:uuid;index;not null"`
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

func (r *Relationship) BeforeCreate(db *gorm.DB) error {
	var err error
	if r.ID == uuid.Nil {
		r.ID, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("failed to generate resource uuid: %w", err)
		}
	}
	return nil
}
