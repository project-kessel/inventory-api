package model

import (
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type LocalInventoryToResource struct {
	// Our local resource id
	ResourceId uuid.UUID  `gorm:"type:uuid;primarykey"`
	CreatedAt  *time.Time `gorm:"index"`

	ReporterResourceId
}

type ReporterResourceId struct {
	// Id in reporter's side
	LocalResourceId string `gorm:"primarykey"`
	ResourceType    string `gorm:"primarykey"`

	// Reporter identification
	ReporterId   string `gorm:"primarykey"`
	ReporterType string `gorm:"primarykey"`
}

type ReporterRelationshipId struct {
	// Reporter identification
	ReporterId   string
	ReporterType string

	// Relationship data
	RelationshipType string

	ObjectId  ReporterResourceId
	SubjectId ReporterResourceId
}

func ReporterResourceIdFromResource(resource *Resource) ReporterResourceId {
	return ReporterResourceId{
		LocalResourceId: resource.Reporter.LocalResourceId,
		ResourceType:    resource.ResourceType,
		ReporterId:      resource.Reporter.ReporterId,
		ReporterType:    resource.Reporter.ReporterType,
	}
}

func ReporterRelationshipIdFromRelationship(relationship *Relationship) ReporterRelationshipId {
	return ReporterRelationshipId{
		ReporterId:       relationship.Reporter.ReporterId,
		ReporterType:     relationship.Reporter.ReporterType,
		RelationshipType: relationship.RelationshipType,
		SubjectId: ReporterResourceId{
			LocalResourceId: relationship.Reporter.SubjectLocalResourceId,
			ResourceType:    relationship.Reporter.SubjectResourceType,
			ReporterId:      relationship.Reporter.ReporterId,
			ReporterType:    relationship.Reporter.ReporterType,
		},
		ObjectId: ReporterResourceId{
			LocalResourceId: relationship.Reporter.ObjectLocalResourceId,
			ResourceType:    relationship.Reporter.ObjectResourceType,
			ReporterId:      relationship.Reporter.ReporterId,
			ReporterType:    relationship.Reporter.ReporterType,
		},
	}
}

func (r *LocalInventoryToResource) BeforeCreate(db *gorm.DB) error {
	var err error
	if r.ResourceId == uuid.Nil {
		r.ResourceId, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("failed to generate uuid: %w", err)
		}
	}
	return nil
}
