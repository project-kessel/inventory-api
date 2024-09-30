package model

import "time"

type RelationshipHistory struct {
	ID               uint64 `gorm:"primarykey"`
	RelationshipData JsonObject
	RelationshipType string
	SubjectId        uint64
	SubjectResource  Resource `gorm:"foreignKey:SubjectId" json:"-"`
	ObjectId         uint64
	ObjectResource   Resource `gorm:"foreignKey:ObjectId" json:"-"`
	Reporter         RelationshipReporter
	CreatedAt        *time.Time
	// We don't need UpdatedAt in here. We won't update the history resource

	RelationshipId       uint64
	OriginalRelationship Relationship `gorm:"foreignKey:RelationshipId" json:"-"`
}

func (*RelationshipHistory) TableName() string {
	return "relationship_history"
}
