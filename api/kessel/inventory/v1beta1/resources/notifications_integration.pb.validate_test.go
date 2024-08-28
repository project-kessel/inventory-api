package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotificationIntegrationOptionalMetadata(t *testing.T) {
	notifintegration := NotificationsIntegration{
		ReporterData: &ReporterData{
			ReporterType:    ReporterData_OCM,
			LocalResourceId: "foo",
		},
	}
	err := notifintegration.ValidateAll()

	assert.NoError(t, err)
}

func TestNotificationIntegrationMetadataIsValidatedIfFound(t *testing.T) {
	notifintegration := NotificationsIntegration{
		ReporterData: &ReporterData{
			ReporterType:    ReporterData_OCM,
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
