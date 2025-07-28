package model

import "github.com/project-kessel/inventory-api/internal"

type Representation struct {
	Data internal.JsonObject `gorm:"type:jsonb;column:data"`
}
