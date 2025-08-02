package model

import (
	"fmt"

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
	return r.data
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

func (rr ReporterRepresentation) Serialize() (*datamodel.ReporterRepresentation, error) {
	var reporterVersionStr *string
	if rr.reporterVersion != nil {
		versionStr := rr.reporterVersion.String()
		reporterVersionStr = &versionStr
	}

	return datamodel.NewReporterRepresentation(
		internal.JsonObject(rr.data),
		rr.reporterResourceID.String(),
		uint(rr.version),
		uint(rr.generation),
		uint(rr.commonVersion),
		rr.tombstone.Bool(),
		reporterVersionStr,
	)
}
