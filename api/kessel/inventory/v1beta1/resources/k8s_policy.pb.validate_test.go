package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestK8SPolicyValidatesSuccessfullyWithValidData(t *testing.T) {
	policy := K8SPolicy{
		Metadata: &Metadata{},
		ReporterData: &ReporterData{
			ReporterInstanceId: "reporter-id",
			ReporterType:       1, //ACM
			ConsoleHref:        "https://example.com/console",
			ApiHref:            "https://example.com/api",
			LocalResourceId:    "local-resource-id",
			ReporterVersion:    "2.12"},
		ResourceData: &K8SPolicyDetail{},
	}
	err := policy.ValidateAll()
	assert.NoError(t, err)
}

func TestK8SPolicyValidationFailsWithMissingReporterData(t *testing.T) {
	policy := K8SPolicy{
		Metadata: &Metadata{},

		ResourceData: &K8SPolicyDetail{},
	}
	err := policy.ValidateAll()
	assert.ErrorContains(t, err, "Policy.ReporterData")
}

func TestK8SPolicyValidationFailsWithMissingResourceData(t *testing.T) {
	policy := K8SPolicy{
		Metadata: &Metadata{},
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
func TestK8SPolicyValidationWithAllErrors(t *testing.T) {
	policy := K8SPolicy{}
	err := policy.ValidateAll()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Policy.ReporterData")
	assert.Contains(t, err.Error(), "Policy.ResourceData")
}
