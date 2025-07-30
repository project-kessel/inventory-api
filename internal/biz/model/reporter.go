package model

type Reporter struct {
	reporterType       ReporterType
	reporterInstanceId ReporterInstanceId
}

func NewReporter(reporterType, reporterInstanceId string) (Reporter, error) {
	reporterTypeObj, err := NewReporterType(reporterType)
	if err != nil {
		return Reporter{}, err
	}

	reporterInstanceIdObj, err := NewReporterInstanceId(reporterInstanceId)
	if err != nil {
		return Reporter{}, err
	}

	return Reporter{
		reporterType:       reporterTypeObj,
		reporterInstanceId: reporterInstanceIdObj,
	}, nil
}
