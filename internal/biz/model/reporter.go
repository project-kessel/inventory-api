package model

import "fmt"

type Reporter struct {
	reporterType       ReporterType
	reporterInstanceId ReporterInstanceId
}

func NewReporter(reporterTypeVal, reporterInstanceIdVal string) (Reporter, error) {
	reporterType, err := NewReporterType(reporterTypeVal)
	if err != nil {
		return Reporter{}, fmt.Errorf("Reporter invalid type: %w", err)
	}

	reporterInstanceId, err := NewReporterInstanceId(reporterInstanceIdVal)
	if err != nil {
		return Reporter{}, fmt.Errorf("Reporter invalid instance ID: %w", err)
	}

	return Reporter{
		reporterType:       reporterType,
		reporterInstanceId: reporterInstanceId,
	}, nil
}
