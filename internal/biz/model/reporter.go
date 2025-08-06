package model

type ReporterId struct {
	reporterType       ReporterType
	reporterInstanceId ReporterInstanceId
}

func NewReporterId(reporterType ReporterType, reporterInstanceId ReporterInstanceId) (ReporterId, error) {
	return ReporterId{
		reporterType:       reporterType,
		reporterInstanceId: reporterInstanceId,
	}, nil
}
