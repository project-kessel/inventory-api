package v1beta1

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMetadataIsOptional(t *testing.T) {
	host := RhelHost{
		ReporterData: &ReporterData{
			ReporterType:    ReporterData_REPORTER_TYPE_OCM,
			LocalResourceId: "foo",
		},
	}
	err := host.ValidateAll()

	assert.NoError(t, err)
}

func TestMetadataIsValidatedIfFound(t *testing.T) {
	host := RhelHost{
		ReporterData: &ReporterData{
			ReporterType:    ReporterData_REPORTER_TYPE_OCM,
			LocalResourceId: "foo",
		},
		Metadata: &Metadata{
			Labels: []*ResourceLabel{
				{},
			},
		},
	}
	err := host.ValidateAll()

	assert.ErrorContains(t, err, "RhelHost.Metadata")
	assert.ErrorContains(t, err, "Metadata.Labels")
}

func TestReporterDataIsValidated(t *testing.T) {
	host := RhelHost{}
	err := host.ValidateAll()

	assert.ErrorContains(t, err, "RhelHost.ReporterData")
}
