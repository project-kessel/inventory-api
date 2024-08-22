package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPolicyValidatesSuccessfullyWithValidData(t *testing.T) {
	policy := Policy{
		Metadata: &Metadata{
			Id:           12345,
			ResourceType: "policy",
			Workspace:    "workspace-name",
			Labels: []*ResourceLabel{
				{
					Key:   "apps.open-cluster-management.io/reconcile-rate",
					Value: "high",
				},
			},
		},
		ReporterData: &ReporterData{
			ReporterInstanceId: "reporter-id",
			ReporterType:       1, //ACM
			ConsoleHref:        "https://example.com/console",
			ApiHref:            "https://example.com/api",
			LocalResourceId:    "local-resource-id",
			ReporterVersion:    "2.12",
		},
		ResourceData: &PolicyDetail{
			Disabled: false,
			Severity: 1,
		},
	}
	err := policy.ValidateAll()
	assert.NoError(t, err)
}

func TestPolicyValidationFailsWithMissingReporterData(t *testing.T) {
	policy := Policy{
		Metadata: &Metadata{
			Id:           12345,
			ResourceType: "policy",
			Workspace:    "workspace-name",
			Labels: []*ResourceLabel{
				{
					Key:   "apps.open-cluster-management.io/reconcile-rate",
					Value: "high",
				},
			},
		},
		ResourceData: &PolicyDetail{
			Disabled: false,
			Severity: 1,
		},
	}
	err := policy.ValidateAll()
	assert.ErrorContains(t, err, "Policy.ReporterData")
}

func TestPolicyValidationFailsWithMissingResourceData(t *testing.T) {
	policy := Policy{
		Metadata: &Metadata{
			Id:           12345,
			ResourceType: "policy",
			Workspace:    "workspace-name",
			Labels: []*ResourceLabel{
				{
					Key:   "apps.open-cluster-management.io/reconcile-rate",
					Value: "high",
				},
			},
		},
		ReporterData: &ReporterData{
			ReporterInstanceId: "reporter-id",
			ReporterType:       1, //ACM
			ConsoleHref:        "https://example.com/console",
			ApiHref:            "https://example.com/api",
			LocalResourceId:    "local-resource-id",
			ReporterVersion:    "2.12",
		},
	}
	err := policy.ValidateAll()
	assert.ErrorContains(t, err, "Policy.ResourceData")
}

// Missing ReporterData and ResourceData, Policy MetaData is not required
func TestPolicyValidationWithAllErrors(t *testing.T) {
	policy := Policy{}
	err := policy.ValidateAll()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Policy.ReporterData")
	assert.Contains(t, err.Error(), "Policy.ResourceData")
}
