package policies

import (
	"github.com/project-kessel/inventory-api/internal/biz/common"
)

type Policy struct {
	// Kessel Asset Inventory generated identifier.
	ID int64 `gorm:"primaryKey"`

	// required for gorm to associate the metadata table
	MetadataID int64           `json:"-"`
	Metadata   common.Metadata `json:"metadata"`

	// this should be a gorm json type.
	// see https://github.com/go-gorm/datatypes
	ResourceData *PolicyDetail `json:"resource_data"`
}

type PolicyDetail struct {
	Disabled bool   `json:"disabled"`
	Severity string `json:"severity"`
}
