package k8spolicy

import "github.com/project-kessel/inventory-api/internal/biz/common"

type K8SPolicyIsPropagatedToK8SCluster struct {
	// Kessel Asset Inventory generated identifier.
	ID int64 `gorm:"primaryKey"`

	MetadataID int64
	Metadata   common.RelationshipMetadata

	Status string
}
