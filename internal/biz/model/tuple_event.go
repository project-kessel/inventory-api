package model

type TupleEvent struct {
	commonVersion       Version
	reporterResourceKey ReporterResourceKey
}

func NewTupleEvent(version Version, reporterResourceKey ReporterResourceKey) (TupleEvent, error) {
	return TupleEvent{
		commonVersion:       version,
		reporterResourceKey: reporterResourceKey,
	}, nil
}

func (te TupleEvent) Version() Version {
	return te.commonVersion
}

func (te TupleEvent) ReporterResourceKey() ReporterResourceKey {
	return te.reporterResourceKey
}
