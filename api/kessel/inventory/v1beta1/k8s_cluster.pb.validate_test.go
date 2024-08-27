package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestK8_clusterValidatesSuccessfullyWithValidData(t *testing.T) {
	k8cluster := K8SCluster{
		ReporterData: &ReporterData{
			ReporterType:    2,
			LocalResourceId: "some-id",
		},
		ResourceData: &K8SClusterDetail{
			ExternalClusterId: "some-cluster-id",
			ClusterStatus:     2,
			KubeVendor:        5,
			KubeVersion:       "v1.30.1",
			VendorVersion:     "4.16",
			CloudPlatform:     6,
			Nodes: []*K8SClusterDetailNodesInner{
				{
					Name:   "ip-10-0-0-1.ec2.internal",
					Cpu:    "7500m",
					Memory: "30973224Ki",
					Labels: []*ResourceLabel{
						{
							Key:   "node.openshift.io/os_id",
							Value: "rhcos",
						},
					},
				},
			},
		},
	}
	err := k8cluster.ValidateAll()
	assert.NoError(t, err)
}

func TestK8ClusterValidationFailsWithMissingReporterData(t *testing.T) {
	k8cluster := K8SCluster{
		ResourceData: &K8SClusterDetail{
			ExternalClusterId: "some-cluster-id",
			ClusterStatus:     2,
			KubeVendor:        5,
			KubeVersion:       "v1.30.1",
			VendorVersion:     "4.16",
			CloudPlatform:     6,
			Nodes: []*K8SClusterDetailNodesInner{
				{
					Name:   "ip-10-0-0-1.ec2.internal",
					Cpu:    "7500m",
					Memory: "30973224Ki",
					Labels: []*ResourceLabel{
						{
							Key:   "node.openshift.io/os_id",
							Value: "rhcos",
						},
					},
				},
			},
		},
	}
	err := k8cluster.ValidateAll()
	assert.ErrorContains(t, err, "K8SCluster.ReporterData")
}
func TestK8ClusterValidationFailsWithMissingResourceData(t *testing.T) {
	k8cluster := K8SCluster{
		ReporterData: &ReporterData{
			ReporterType:    2,
			LocalResourceId: "some-id",
		},
	}
	err := k8cluster.ValidateAll()
	assert.ErrorContains(t, err, "K8SCluster.ResourceData")
}

func TestK8ClusterValidationFailsWithMissingAllData(t *testing.T) {
	k8cluster := K8SCluster{}

	err := k8cluster.ValidateAll()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "K8SCluster.ReporterData")
	assert.Contains(t, err.Error(), "K8SCluster.ResourceData")
}
