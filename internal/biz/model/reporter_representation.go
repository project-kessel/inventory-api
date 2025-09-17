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

func NewReporterDataRepresentation(
	reporterResourceID ReporterResourceId,
	version Version,
	generation Generation,
	data Representation,
	commonVersion Version,
	reporterVersion *ReporterVersion,
) (ReporterDataRepresentation, error) {

	if reporterResourceID.UUID() == uuid.Nil {
		return ReporterDataRepresentation{}, fmt.Errorf("%w: ReporterResourceId", ErrInvalidUUID)
	}

	if len(data) == 0 {
		return ReporterDataRepresentation{}, fmt.Errorf("%w: ReporterDataRepresentation data", ErrInvalidData)
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
) (ReporterDeleteRepresentation, error) {
	if reporterResourceID.UUID() == uuid.Nil {
		return ReporterDeleteRepresentation{}, fmt.Errorf("%w: ReporterResourceId", ErrInvalidUUID)
	}

	return ReporterDeleteRepresentation{
		ReporterRepresentation: ReporterRepresentation{
			Representation:     Representation(nil),
			reporterResourceID: reporterResourceID,
			version:            version,
			generation:         generation,
			tombstone:          NewTombstone(true),
		},
	}, nil
}

func (r ReporterRepresentation) Data() internal.JsonObject {
	return r.Representation.Data()
}

func (r ReporterRepresentation) IsTombstone() bool {
	return r.tombstone.Bool()
}

func (rr ReporterRepresentation) Serialize() ReporterRepresentationSnapshot {
	var reporterVersionStr *string
	if rr.reporterVersion != nil {
		versionStr := rr.reporterVersion.Serialize()
		reporterVersionStr = &versionStr
	}

	// Create representation snapshot
	representationSnapshot := RepresentationSnapshot{
		Data: rr.Representation.Serialize(),
	}

	// Create ReporterRepresentation snapshot - direct initialization without validation
	return ReporterRepresentationSnapshot{
		Representation:     representationSnapshot,
		ReporterResourceID: rr.reporterResourceID.Serialize(),
		Version:            rr.version.Serialize(),
		Generation:         rr.generation.Serialize(),
		ReporterVersion:    reporterVersionStr,
		CommonVersion:      rr.commonVersion.Serialize(),
		Tombstone:          rr.tombstone.Serialize(),
		CreatedAt:          time.Now(), // TODO: Add proper timestamp from domain entity if available
	}
}

// DeserializeReporterDataRepresentation creates a ReporterRepresentation from snapshot - direct initialization without validation
func DeserializeReporterDataRepresentation(snapshot *ReporterRepresentationSnapshot) *ReporterDataRepresentation {
	if snapshot == nil {
		return nil
	}
	// Create domain tiny types directly from snapshot values
	reporterResourceId := DeserializeReporterResourceId(snapshot.ReporterResourceID)
	representation := DeserializeRepresentation(snapshot.Representation.Data)
	version := DeserializeVersion(snapshot.Version)
	generation := DeserializeGeneration(snapshot.Generation)
	commonVersion := DeserializeVersion(snapshot.CommonVersion)
	tombstone := DeserializeTombstone(snapshot.Tombstone)

	var reporterVersion *ReporterVersion
	if snapshot.ReporterVersion != nil {
		rv := DeserializeReporterVersion(*snapshot.ReporterVersion)
		reporterVersion = &rv
	}

	return &ReporterDataRepresentation{
		ReporterRepresentation{
			Representation:     representation,
			reporterResourceID: reporterResourceId,
			version:            version,
			generation:         generation,
			commonVersion:      commonVersion,
			reporterVersion:    reporterVersion,
			tombstone:          tombstone,
		},
	}
}

func (rr ReporterRepresentation) CommonVersion() Version {
	return rr.commonVersion
}
