package v1beta2

import "github.com/project-kessel/inventory-api/internal/biz/model"

type BaseRepresentation struct {
	Data model.JsonObject `gorm:"column:data"`
}
