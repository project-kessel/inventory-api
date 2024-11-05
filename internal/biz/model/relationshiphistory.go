package model

import (
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type RelationshipHistory struct {
	ID               uuid.UUID `gorm:"type:uuid;primarykey"`
	OrgId            string    `gorm:"index"`
	RelationshipData JsonObject
	RelationshipType string
	SubjectId        uuid.UUID `gorm:"type:uuid;index"`
	ObjectId         uuid.UUID `gorm:"type:uuid;index"`
	Reporter         RelationshipReporter
	Timestamp        *time.Time `gorm:"autoCreateTime"`

	RelationshipId uuid.UUID     `gorm:"type:uuid;index"`
	OperationType  OperationType `gorm:"index"`
}

func (*RelationshipHistory) TableName() string {
	return "relationship_history"
}

func (r *RelationshipHistory) BeforeCreate(db *gorm.DB) error {
	var err error
	if r.ID == uuid.Nil {
		r.ID, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("failed to generate uuid: %w", err)
		}
	}
	return nil
}
