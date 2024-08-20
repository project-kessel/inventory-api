package v1beta1

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNotificationIntegrationOptionalMetadata(t *testing.T) {
	notifintegration := NotificationsIntegration{
		ReporterData: &ReporterData{
			ReporterType:    ReporterData_REPORTER_TYPE_OCM,
			LocalResourceId: "foo",
		},
	}
	err := notifintegration.ValidateAll()

	assert.NoError(t, err)
}

func TestNotificationIntegrationMetadataIsValidatedIfFound(t *testing.T) {
	notifintegration := NotificationsIntegration{
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
	err := notifintegration.ValidateAll()

	assert.ErrorContains(t, err, "Integration.Metadata")
	assert.ErrorContains(t, err, "Metadata.Labels")
}

func TestNotificationIntegrationDataIsValidated(t *testing.T) {
	host := NotificationsIntegration{}
	err := host.ValidateAll()

	assert.ErrorContains(t, err, "Integration.ReporterData")
}
