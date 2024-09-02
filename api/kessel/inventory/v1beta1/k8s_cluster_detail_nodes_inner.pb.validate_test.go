package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestK8SClusterDetailNodesInnerValidatesSuccessfullyWithValidData(t *testing.T) {
	node := &K8SClusterDetailNodesInner{
		Name:   "ip-10-0-0-1.ec2.internal",
		Cpu:    "7500m",
		Memory: "30973224Ki",
		Labels: []*ResourceLabel{
			{
				Key:   "node.openshift.io/os_id",
				Value: "rhcos",
			},
		},
	}
	err := node.ValidateAll()
	assert.NoError(t, err)
}

func TestK8SClusterDetailNodesInnerValidationFailsWithEmptyName(t *testing.T) {
	node := &K8SClusterDetailNodesInner{
		Name:   "",
		Cpu:    "7500m",
		Memory: "30973224Ki",
		Labels: []*ResourceLabel{
			{
				Key:   "node.openshift.io/os_id",
				Value: "rhcos",
			},
		},
	}
	err := node.ValidateAll()
	assert.ErrorContains(t, err, "invalid K8SClusterDetailNodesInner.Name: value length must be at least 1 runes")
}

func TestK8SClusterDetailNodesInnerValidationFailsWithEmptyCpu(t *testing.T) {
	node := &K8SClusterDetailNodesInner{
		Name:   "ip-10-0-0-1.ec2.internal",
		Cpu:    "",
		Memory: "30973224Ki",
		Labels: []*ResourceLabel{
			{
				Key:   "node.openshift.io/os_id",
				Value: "rhcos",
			},
		},
	}
	err := node.ValidateAll()
	assert.ErrorContains(t, err, "invalid K8SClusterDetailNodesInner.Cpu: value length must be at least 1 runes")
}

func TestK8SClusterDetailNodesInnerValidationFailsWithEmptyMemory(t *testing.T) {
	node := &K8SClusterDetailNodesInner{
		Name:   "ip-10-0-0-1.ec2.internal",
		Cpu:    "7500m",
		Memory: "",
		Labels: []*ResourceLabel{
			{
				Key:   "node.openshift.io/os_id",
				Value: "rhcos",
			},
		},
	}
	err := node.ValidateAll()
	assert.ErrorContains(t, err, "invalid K8SClusterDetailNodesInner.Memory: value length must be at least 1 runes")
}

func TestK8SClusterDetailNodesInnerValidationFailsWithNilLabel(t *testing.T) {
	node := &K8SClusterDetailNodesInner{
		Name:   "ip-10-0-0-1.ec2.internal",
		Cpu:    "7500m",
		Memory: "30973224Ki",
		Labels: []*ResourceLabel{
			nil,
		},
	}
	err := node.ValidateAll()
	assert.ErrorContains(t, err, "invalid K8SClusterDetailNodesInner.Labels[0]: value is required")
}

func TestK8SClusterDetailNodesInnerValidationFailsWithInvalidLabel(t *testing.T) {
	node := &K8SClusterDetailNodesInner{
		Name:   "ip-10-0-0-1.ec2.internal",
		Cpu:    "7500m",
		Memory: "30973224Ki",
		Labels: []*ResourceLabel{
			{
				Key:   "",
				Value: "rhcos",
			},
		},
	}
	err := node.ValidateAll()
	assert.ErrorContains(t, err, "invalid K8SClusterDetailNodesInner.Labels[0]: embedded message failed validation")
}

var multipleErrorMessage = "invalid K8SClusterDetailNodesInner.Name: value length must be at least 1 runes; " +
	"invalid K8SClusterDetailNodesInner.Cpu: value length must be at least 1 runes; " +
	"invalid K8SClusterDetailNodesInner.Memory: value length must be at least 1 runes; " +
	"invalid K8SClusterDetailNodesInner.Labels[0]: value is required"

func TestK8SClusterDetailNodesInnerValidationWithMultipleErrors(t *testing.T) {
	node := &K8SClusterDetailNodesInner{
		Name:   "",
		Cpu:    "",
		Memory: "",
		Labels: []*ResourceLabel{nil},
	}

	err := node.ValidateAll()
	assert.ErrorContains(t, err, multipleErrorMessage)

}
