package model

type ReporterId struct {
	reporterType       ReporterType
	reporterInstanceId ReporterInstanceId
}

func NewReporterId(
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
) ReporterId {
	return ReporterId{
		reporterType:       reporterType,
		reporterInstanceId: reporterInstanceId,
	}
}

func (r ReporterId) ReporterType() string {
	return r.reporterType.String()
}

func (r ReporterId) ReporterInstanceId() string {
	return r.reporterInstanceId.String()
}

func (r ReporterId) Serialize() (string, string) {
	return r.reporterType.Serialize(), r.reporterInstanceId.Serialize()
}

func DeserializeReporterId(reporterType, reporterInstanceId string) ReporterId {
	return ReporterId{
		reporterType:       DeserializeReporterType(reporterType),
		reporterInstanceId: DeserializeReporterInstanceId(reporterInstanceId),
	}
}
