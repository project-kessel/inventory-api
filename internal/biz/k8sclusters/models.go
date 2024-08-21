package k8sclusters

import (
	"github.com/project-kessel/inventory-api/internal/biz/common"
)

type K8SCluster struct {
	// Kessel Asset Inventory generated identifier.
	ID int64 `gorm:"primaryKey"`

	// required for gorm to associate the metadata table
	MetadataID int64           `json:"-"`
	Metadata   common.Metadata `json:"metadata"`

	// this should be a gorm json type.
	// see https://github.com/go-gorm/datatypes
	ResourceData *K8SClusterDetail `json:"resource_data"`
}

type Node struct {
	// The name of the node (this can contain private info)
	Name string `json:"name"`
	// CPU Capacity of the node defined in CPU units, e.g. \"0.5\"
	Cpu string `json:"cpu"`
	// Memory Capacity of the node defined as MiB, e.g. \"50Mi\"
	Memory string `json:"memory"`
	// Map of string keys and string values that can be used to organize and
	// categorize (scope and select) resources
	Labels []NodeLabel `json:"labels"`
}

type NodeLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type K8SClusterDetail struct {
	// The OCP cluster ID or ARN etc for *KS
	ExternalClusterId string `json:"external_cluster_id"`
	ClusterStatus     string `json:"cluster_status"`
	// The version of kubernetes
	KubeVersion string `json:"kube_version"`
	KubeVendor  string `json:"kube_vendor"`
	// The version of the productized kubernetes distribution
	VendorVersion string `json:"vendor_version"`
	CloudPlatform string `json:"cloud_platform"`
	Nodes         []Node `json:"nodes"`
}
