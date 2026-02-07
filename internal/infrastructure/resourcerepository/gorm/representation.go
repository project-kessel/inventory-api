package gorm

import "github.com/project-kessel/inventory-api/internal"

// Representation is the base GORM model for JSON data blobs.
type Representation struct {
	Data internal.JsonObject `gorm:"type:jsonb;"`
}
