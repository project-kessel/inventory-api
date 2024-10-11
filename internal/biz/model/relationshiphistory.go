package model

import "time"

type RelationshipHistory struct {
	ID               uint64 `gorm:"primarykey"`
	OrgId            string `gorm:"index"`
	RelationshipData JsonObject
	RelationshipType string
	SubjectId        uint64 `gorm:"index"`
	ObjectId         uint64 `gorm:"index"`
	Reporter         RelationshipReporter
	Timestamp        *time.Time `gorm:"autoCreateTime"`

	RelationshipId uint64        `gorm:"index"`
	OperationType  OperationType `gorm:"index"`
}

func (*RelationshipHistory) TableName() string {
	return "relationship_history"
}
