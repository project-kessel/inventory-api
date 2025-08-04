package model

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
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
	Data() internal.JsonObject
}

type ReporterDeleteRepresentation interface {
	// ReporterDeleteRepresentation should not have data
}

func (r ReporterRepresentation) Data() internal.JsonObject {
	return r.Representation.Data()
}

func (r ReporterRepresentation) IsTombstone() bool {
	return r.tombstone.Bool()
}

func NewReporterDataRepresentation(
	reporterResourceIdVal string,
	version uint,
	generation uint,
	data internal.JsonObject,
	commonVersion uint,
	reporterVersionVal *string,
) (ReporterDataRepresentation, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("ReporterDataRepresentation requires non-empty data")
	}

	reporterResourceID, err := NewReporterResourceIdFromString(reporterResourceIdVal)
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
		Representation:     Representation(data),
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
		Representation:     Representation(nil),
		reporterResourceID: reporterResourceID,
		version:            NewVersion(version),
		generation:         NewGeneration(generation),
		commonVersion:      NewVersion(commonVersion),
		reporterVersion:    reporterVersion,
		tombstone:          NewTombstone(true),
	}, nil
}

func (rr ReporterRepresentation) Serialize() (*datamodel.ReporterRepresentation, error) {
	var reporterVersionStr *string
	if rr.reporterVersion != nil {
		versionStr := rr.reporterVersion.Serialize()
		reporterVersionStr = &versionStr
	}

	return datamodel.NewReporterRepresentation(
		rr.Representation.Serialize(),
		rr.reporterResourceID.String(),
		rr.version.Serialize(),
		rr.generation.Serialize(),
		rr.commonVersion.Serialize(),
		rr.tombstone.Serialize(),
		reporterVersionStr,
	)
}

func DeserializeReporterDataRepresentation(
	reporterResourceIdVal string,
	version uint,
	generation uint,
	data internal.JsonObject,
	commonVersion uint,
	reporterVersionVal *string,
) ReporterRepresentation {
	var reporterVersion *ReporterVersion
	if reporterVersionVal != nil {
		rv := DeserializeReporterVersion(*reporterVersionVal)
		reporterVersion = &rv
	}

	return ReporterRepresentation{
		Representation:     DeserializeRepresentation(data),
		reporterResourceID: DeserializeReporterResourceId(uuid.MustParse(reporterResourceIdVal)),
		version:            DeserializeVersion(version),
		generation:         DeserializeGeneration(generation),
		commonVersion:      DeserializeVersion(commonVersion),
		reporterVersion:    reporterVersion,
		tombstone:          DeserializeTombstone(false),
	}
}

func DeserializeReporterDeleteRepresentation(
	reporterResourceIDVal string,
	version uint,
	generation uint,
	commonVersion uint,
	reporterVersionVal *string,
) ReporterRepresentation {
	var reporterVersion *ReporterVersion
	if reporterVersionVal != nil {
		rv := DeserializeReporterVersion(*reporterVersionVal)
		reporterVersion = &rv
	}

	return ReporterRepresentation{
		Representation:     DeserializeRepresentation(nil),
		reporterResourceID: DeserializeReporterResourceId(uuid.MustParse(reporterResourceIDVal)),
		version:            DeserializeVersion(version),
		generation:         DeserializeGeneration(generation),
		commonVersion:      DeserializeVersion(commonVersion),
		reporterVersion:    reporterVersion,
		tombstone:          DeserializeTombstone(true),
	}
}
