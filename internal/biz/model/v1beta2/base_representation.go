package v1beta2

import (
	"time"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type BaseRepresentation struct {
	Data      model.JsonObject `gorm:"column:data"`
	CreatedAt time.Time        `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time        `gorm:"column:updated_at;autoUpdateTime"`
}
