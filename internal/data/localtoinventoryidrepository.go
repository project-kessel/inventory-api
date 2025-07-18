package data

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
)

func GetLastResourceId(DB *gorm.DB, reporterResourceId model_legacy.ReporterResourceId) (uuid.UUID, error) {
	localInventoryToResourceId := model_legacy.LocalInventoryToResource{}
	err := GetLastResourceIdQuery(DB, reporterResourceId).First(&localInventoryToResourceId).Error
	return localInventoryToResourceId.ResourceId, err
}

func GetLastResourceIdQuery(DB *gorm.DB, reporterResourceId model_legacy.ReporterResourceId) *gorm.DB {
	return DB.Table("local_inventory_to_resources").Select("resource_id").Where("local_resource_id = ? AND reporter_id = ? AND resource_type = ?", reporterResourceId.LocalResourceId, reporterResourceId.ReporterId, reporterResourceId.ResourceType)
}
