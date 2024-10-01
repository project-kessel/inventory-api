package hosts

import "github.com/project-kessel/inventory-api/internal/biz/common"

type Host struct {
	// Kessel Asset Inventory generated identifier.
	ID int64 `gorm:"primaryKey"`

	// required for gorm to associate the metadata table
	MetadataID int64
	Metadata   common.Metadata
}
