package notificationsintegrations

import (
	"github.com/project-kessel/inventory-api/internal/biz/common"
)

type NotificationsIntegration struct {
	// Kessel Asset Inventory generated identifier.
	ID int64 `gorm:"primaryKey"`

	// required for gorm to associate the metadata table
	MetadataID int64           `json:"-"`
	Metadata   common.Metadata `json:"metadata"`
}
