package k8spolicies

import (
	"database/sql/driver"
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

func (details *K8sPolicyDetail) Scan(value interface{}) error {
	*details = K8sPolicyDetail{}
	return common.Unmarshal(value, details)
}

func (details *K8sPolicyDetail) Value() (driver.Value, error) {
	return common.Marshal(details)
}
