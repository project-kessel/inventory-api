package k8spolicies

import (
	"github.com/project-kessel/inventory-api/internal/biz/common"
)

type K8sPolicy struct {
	// Kessel Asset Inventory generated identifier.
	ID int64 `gorm:"primaryKey"`

	// required for gorm to associate the metadata table
	MetadataID int64           `json:"-"`
	Metadata   common.Metadata `json:"metadata"`

	// this should be a gorm json type.
	// see https://github.com/go-gorm/datatypes
	ResourceData *K8sPolicyDetail `json:"resource_data"`
}

type K8sPolicyDetail struct {
	Disabled bool   `json:"disabled"`
	Severity string `json:"severity"`
}
