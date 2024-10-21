package data

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"gorm.io/gorm"
)

func GetLastResourceId(DB *gorm.DB, reporterResourceId model.ReporterResourceId) (uint64, error) {
	localInventoryToResourceId := model.LocalInventoryToResource{}
	err := DB.Order("created_at DESC").First(&localInventoryToResourceId, "local_resource_id = ? AND reporter_id = ? AND resource_type = ?", reporterResourceId.LocalResourceId, reporterResourceId.ReporterId, reporterResourceId.ResourceType).Error
	return localInventoryToResourceId.ResourceId, err
}
