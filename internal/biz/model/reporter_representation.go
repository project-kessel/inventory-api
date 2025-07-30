package model

type ReporterRepresentation struct {
	Representation
	reporterResourceID ReporterResourceId
	version            Version
	generation         Generation
	reporterVersion    *ReporterVersion
	commonVersion      Version
	tombstone          Tombstone
}

type ReporterDataRepresentation interface {
	Data() JsonObject
	IsTombstone() bool
}

type ReporterDeleteRepresentation interface {
}

func (r ReporterRepresentation) Data() JsonObject {
	if r.tombstone.Bool() {
		return nil
	}
	return r.data
}

func (r ReporterRepresentation) IsTombstone() bool {
	return r.tombstone.Bool()
}

func NewReporterDataRepresentation(
	data JsonObject,
	reporterResourceID string,
	version uint,
	generation uint,
	commonVersion uint,
	reporterVersion *string,
) (ReporterDataRepresentation, error) {
	reporterResourceIDObj, err := NewReporterResourceIdFromString(reporterResourceID)
	if err != nil {
		return nil, err
	}

	reporterVersionObj, err := NewReporterVersionPtr(reporterVersion)
	if err != nil {
		return nil, err
	}

	return ReporterRepresentation{
		Representation: Representation{
			data: data,
		},
		reporterResourceID: reporterResourceIDObj,
		version:            NewVersion(version),
		generation:         NewGeneration(generation),
		commonVersion:      NewVersion(commonVersion),
		reporterVersion:    reporterVersionObj,
		tombstone:          NewTombstone(false),
	}, nil
}

func NewReporterDeleteRepresentation(
	reporterResourceID string,
	version uint,
	generation uint,
	commonVersion uint,
	reporterVersion *string,
) (ReporterDeleteRepresentation, error) {
	reporterResourceIDObj, err := NewReporterResourceIdFromString(reporterResourceID)
	if err != nil {
		return nil, err
	}

	reporterVersionObj, err := NewReporterVersionPtr(reporterVersion)
	if err != nil {
		return nil, err
	}

	return ReporterRepresentation{
		Representation: Representation{
			data: nil,
		},
		reporterResourceID: reporterResourceIDObj,
		version:            NewVersion(version),
		generation:         NewGeneration(generation),
		commonVersion:      NewVersion(commonVersion),
		reporterVersion:    reporterVersionObj,
		tombstone:          NewTombstone(true),
	}, nil
}
