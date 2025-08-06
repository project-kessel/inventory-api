package model

import (
	"fmt"
)

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
	reporterResourceIDVal string,
	version uint,
	generation uint,
	commonVersion uint,
	reporterVersionVal *string,
) (ReporterDataRepresentation, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: ReporterDataRepresentation data", ErrInvalidData)
	}

	reporterResourceID, err := NewReporterResourceIdFromString(reporterResourceIDVal)
	if err != nil {
		return nil, err
	}

	var reporterVersion *ReporterVersion
	if reporterVersionVal != nil {
		rv, err := NewReporterVersion(*reporterVersionVal)
		if err != nil {
			return nil, err
		}
		reporterVersion = &rv
	}

	return ReporterRepresentation{
		Representation: Representation{
			data: data,
		},
		reporterResourceID: reporterResourceID,
		version:            NewVersion(version),
		generation:         NewGeneration(generation),
		commonVersion:      NewVersion(commonVersion),
		reporterVersion:    reporterVersion,
		tombstone:          NewTombstone(false),
	}, nil
}

func NewReporterDeleteRepresentation(
	reporterResourceIDVal string,
	version uint,
	generation uint,
	commonVersion uint,
	reporterVersionVal *string,
) (ReporterDeleteRepresentation, error) {
	reporterResourceID, err := NewReporterResourceIdFromString(reporterResourceIDVal)
	if err != nil {
		return nil, err
	}

	var reporterVersion *ReporterVersion
	if reporterVersionVal != nil {
		rv, err := NewReporterVersion(*reporterVersionVal)
		if err != nil {
			return nil, err
		}
		reporterVersion = &rv
	}

	return ReporterRepresentation{
		Representation: Representation{
			data: nil,
		},
		reporterResourceID: reporterResourceID,
		version:            NewVersion(version),
		generation:         NewGeneration(generation),
		commonVersion:      NewVersion(commonVersion),
		reporterVersion:    reporterVersion,
		tombstone:          NewTombstone(true),
	}, nil
}
