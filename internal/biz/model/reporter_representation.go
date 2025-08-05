package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
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

func (rr ReporterRepresentation) Serialize() ReporterRepresentationSnapshot {
	var reporterVersionStr *string
	if rr.reporterVersion != nil {
		versionStr := rr.reporterVersion.Serialize()
		reporterVersionStr = &versionStr
	}

	// Create representation snapshot
	representationSnapshot := RepresentationSnapshot{
		Data: rr.Representation.Data(),
	}

	// Create ReporterRepresentation snapshot - direct initialization without validation
	return ReporterRepresentationSnapshot{
		Representation:     representationSnapshot,
		ReporterResourceID: rr.reporterResourceID.String(),
		Version:            rr.version.Serialize(),
		Generation:         rr.generation.Serialize(),
		ReporterVersion:    reporterVersionStr,
		CommonVersion:      rr.commonVersion.Serialize(),
		Tombstone:          rr.tombstone.Bool(),
		CreatedAt:          time.Now(), // TODO: Add proper timestamp from domain entity if available
	}
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
	return rr.Serialize(), nil
}

// DeserializeReporterRepresentation creates a ReporterRepresentation from snapshot - direct initialization without validation
func DeserializeReporterRepresentation(snapshot ReporterRepresentationSnapshot) ReporterRepresentation {
	// Create domain tiny types directly from snapshot values
	reporterResourceId := ReporterResourceId(uuid.MustParse(snapshot.ReporterResourceID))
	representation := Representation(snapshot.Representation.Data)
	version := DeserializeVersion(snapshot.Version)
	generation := DeserializeGeneration(snapshot.Generation)
	commonVersion := DeserializeVersion(snapshot.CommonVersion)
	tombstone := DeserializeTombstone(snapshot.Tombstone)

	var reporterVersion *ReporterVersion
	if snapshot.ReporterVersion != nil {
		rv := DeserializeReporterVersion(*snapshot.ReporterVersion)
		reporterVersion = &rv
	}

	return ReporterRepresentation{
		Representation:     representation,
		reporterResourceID: reporterResourceId,
		version:            version,
		generation:         generation,
		commonVersion:      commonVersion,
		reporterVersion:    reporterVersion,
		tombstone:          tombstone,
	}
}

// CreateSnapshot creates a snapshot of the ReporterDataRepresentation
func (rdr ReporterDataRepresentation) CreateSnapshot() (ReporterRepresentationSnapshot, error) {
	return rdr.ReporterRepresentation.CreateSnapshot()
}

// CreateSnapshot creates a snapshot of the ReporterDeleteRepresentation
func (rdr ReporterDeleteRepresentation) CreateSnapshot() (ReporterRepresentationSnapshot, error) {
	return rdr.ReporterRepresentation.CreateSnapshot()
}
