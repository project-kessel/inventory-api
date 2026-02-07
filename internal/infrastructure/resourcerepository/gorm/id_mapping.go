package gorm

import (
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"gorm.io/gorm"
)

// GetLastResourceId retrieves the last known resource ID for a given reporter resource.
func GetLastResourceId(DB *gorm.DB, reporterResourceId model_legacy.ReporterResourceId) (uuid.UUID, error) {
	localInventoryToResourceId := model_legacy.LocalInventoryToResource{}
	err := GetLastResourceIdQuery(DB, reporterResourceId).First(&localInventoryToResourceId).Error
	return localInventoryToResourceId.ResourceId, err
}

// GetLastResourceIdQuery builds a query to find the resource ID for a given reporter resource.
func GetLastResourceIdQuery(DB *gorm.DB, reporterResourceId model_legacy.ReporterResourceId) *gorm.DB {
	return DB.Table("local_inventory_to_resources").Select("resource_id").Where("local_resource_id = ? AND reporter_id = ? AND resource_type = ?", reporterResourceId.LocalResourceId, reporterResourceId.ReporterId, reporterResourceId.ResourceType)
}
