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

type ReporterDataRepresentation struct {
	ReporterRepresentation
}

type ReporterDeleteRepresentation struct {
	ReporterRepresentation
}

func (r ReporterRepresentation) Data() internal.JsonObject {
	return r.Representation.Data()
}

func (r ReporterRepresentation) IsTombstone() bool {
	return r.tombstone.Bool()
}

func NewReporterDataRepresentation(
	reporterResourceID ReporterResourceId,
	version Version,
	generation Generation,
	data Representation,
	commonVersion Version,
	reporterVersion *ReporterVersion,
) (ReporterDataRepresentation, error) {
	if reporterResourceID.UUID() == uuid.Nil {
		return ReporterDataRepresentation{}, fmt.Errorf("ReporterResourceId cannot be empty")
	}

	if len(data.Data()) == 0 {
		return ReporterDataRepresentation{}, fmt.Errorf("ReporterDataRepresentation requires non-empty data")
	}

	return ReporterDataRepresentation{
		ReporterRepresentation: ReporterRepresentation{
			Representation:     data,
			reporterResourceID: reporterResourceID,
			version:            version,
			generation:         generation,
			commonVersion:      commonVersion,
			reporterVersion:    reporterVersion,
			tombstone:          NewTombstone(false),
		},
	}, nil
}

func NewReporterDeleteRepresentation(
	reporterResourceID ReporterResourceId,
	version Version,
	generation Generation,
	commonVersion Version,
	reporterVersion *ReporterVersion,
) (ReporterDeleteRepresentation, error) {
	if reporterResourceID.UUID() == uuid.Nil {
		return ReporterDeleteRepresentation{}, fmt.Errorf("ReporterResourceId cannot be empty")
	}

	return ReporterDeleteRepresentation{
		ReporterRepresentation: ReporterRepresentation{
			Representation:     Representation(nil),
			reporterResourceID: reporterResourceID,
			version:            version,
			generation:         generation,
			commonVersion:      commonVersion,
			reporterVersion:    reporterVersion,
			tombstone:          NewTombstone(true),
		},
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
) ReporterDataRepresentation {
	var reporterVersion *ReporterVersion
	if reporterVersionVal != nil {
		rv := DeserializeReporterVersion(*reporterVersionVal)
		reporterVersion = &rv
	}

	return ReporterDataRepresentation{
		ReporterRepresentation: ReporterRepresentation{
			Representation:     DeserializeRepresentation(data),
			reporterResourceID: DeserializeReporterResourceId(uuid.MustParse(reporterResourceIdVal)),
			version:            DeserializeVersion(version),
			generation:         DeserializeGeneration(generation),
			commonVersion:      DeserializeVersion(commonVersion),
			reporterVersion:    reporterVersion,
			tombstone:          DeserializeTombstone(false),
		},
	}
}

func DeserializeReporterDeleteRepresentation(
	reporterResourceIDVal string,
	version uint,
	generation uint,
	commonVersion uint,
	reporterVersionVal *string,
) ReporterDeleteRepresentation {
	var reporterVersion *ReporterVersion
	if reporterVersionVal != nil {
		rv := DeserializeReporterVersion(*reporterVersionVal)
		reporterVersion = &rv
	}

	return ReporterDeleteRepresentation{
		ReporterRepresentation: ReporterRepresentation{
			Representation:     DeserializeRepresentation(nil),
			reporterResourceID: DeserializeReporterResourceId(uuid.MustParse(reporterResourceIDVal)),
			version:            DeserializeVersion(version),
			generation:         DeserializeGeneration(generation),
			commonVersion:      DeserializeVersion(commonVersion),
			reporterVersion:    reporterVersion,
			tombstone:          DeserializeTombstone(true),
		},
	}
}

// CreateSnapshot creates a snapshot of the ReporterRepresentation
func (rr ReporterRepresentation) CreateSnapshot() (ReporterRepresentationSnapshot, error) {
	dataReporterRepresentation, err := rr.Serialize()
	if err != nil {
		return ReporterRepresentationSnapshot{}, err
	}

	return NewReporterRepresentationSnapshot(dataReporterRepresentation), nil
}

// CreateSnapshot creates a snapshot of the ReporterDataRepresentation
func (rdr ReporterDataRepresentation) CreateSnapshot() (ReporterRepresentationSnapshot, error) {
	return rdr.ReporterRepresentation.CreateSnapshot()
}

// CreateSnapshot creates a snapshot of the ReporterDeleteRepresentation
func (rdr ReporterDeleteRepresentation) CreateSnapshot() (ReporterRepresentationSnapshot, error) {
	return rdr.ReporterRepresentation.CreateSnapshot()
}
