package model

import "time"

type RelationshipHistory struct {
	ID               uint64 `gorm:"primarykey"`
	RelationshipData JsonObject
	RelationshipType string
	SubjectId        uint64 `gorm:"index"`
	ObjectId         uint64 `gorm:"index"`
	Reporter         RelationshipReporter
	CreatedAt        *time.Time
	// We don't need UpdatedAt in here. We won't update the history resource

	RelationshipId uint64 `gorm:"index"`
}

func (*RelationshipHistory) TableName() string {
	return "relationship_history"
}
