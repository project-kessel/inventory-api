package resources

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReporterDataValid(t *testing.T) {
	reporter := ReporterData{
		ReporterType:    ReporterData_OCM,
		LocalResourceId: "my-id",
	}

	err := reporter.ValidateAll()

	assert.NoError(t, err)
}

func TestReporterDataInvalidReporterTypes(t *testing.T) {
	invalidReporterTypes := []int32{
		int32(ReporterData_REPORTER_TYPE_UNSPECIFIED),
		15,
		99,
	}

	for _, reporterType := range invalidReporterTypes {
		t.Run(strconv.Itoa(int(reporterType)), func(t *testing.T) {
			reporter := ReporterData{
				ReporterType:    ReporterData_ReporterType(reporterType),
				LocalResourceId: "my-id",
			}

			err := reporter.ValidateAll()
			assert.ErrorContains(t, err, "ReporterData.ReporterType")
		})
	}
}

func TestReporterDataEmpty(t *testing.T) {
	reporter := ReporterData{}

	err := reporter.ValidateAll()

	assert.ErrorContains(t, err, "ReporterData.ReporterType")
	assert.ErrorContains(t, err, "ReporterData.LocalResourceId")
}
