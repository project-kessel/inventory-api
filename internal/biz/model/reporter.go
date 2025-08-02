package model

import "fmt"

type ReporterId struct {
	reporterType       ReporterType
	reporterInstanceId ReporterInstanceId
}

func NewReporter(reporterTypeVal, reporterInstanceIdVal string) (ReporterId, error) {
	reporterType, err := NewReporterType(reporterTypeVal)
	if err != nil {
		return ReporterId{}, fmt.Errorf("ReporterId invalid type: %w", err)
	}

	reporterInstanceId, err := NewReporterInstanceId(reporterInstanceIdVal)
	if err != nil {
		return ReporterId{}, fmt.Errorf("ReporterId invalid instance ID: %w", err)
	}

	return ReporterId{
		reporterType:       reporterType,
		reporterInstanceId: reporterInstanceId,
	}, nil
}
