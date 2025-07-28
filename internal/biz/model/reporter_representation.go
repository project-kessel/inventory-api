package model

import "github.com/project-kessel/inventory-api/internal"

type ReporterRepresentation struct {
	Representation
	reporterResourceID string
	version            uint
	generation         uint
	reporterVersion    *string
	commonVersion      uint
	tombstone          bool
}

type ReporterDataRepresentation interface {
	Data() internal.JsonObject
	IsTombstone() bool
}

type ReporterDeleteRepresentation interface {
	IsTombstone() bool
	Data() internal.JsonObject
}

func (r ReporterRepresentation) Data() internal.JsonObject {
	if r.tombstone {
		return nil
	}
	return r.Representation.data
}

func (r ReporterRepresentation) IsTombstone() bool {
	return r.tombstone
}

func NewReporterDataRepresentation(
	data internal.JsonObject,
	reporterResourceID string,
	version uint,
	generation uint,
	commonVersion uint,
	reporterVersion *string,
) ReporterDataRepresentation {
	return ReporterRepresentation{
		Representation: Representation{
			data: data,
		},
		reporterResourceID: reporterResourceID,
		version:            version,
		generation:         generation,
		commonVersion:      commonVersion,
		reporterVersion:    reporterVersion,
		tombstone:          false,
	}
}

func NewReporterDeleteRepresentation(
	reporterResourceID string,
	version uint,
	generation uint,
	commonVersion uint,
	reporterVersion *string,
) ReporterDeleteRepresentation {
	return ReporterRepresentation{
		Representation: Representation{
			data: nil,
		},
		reporterResourceID: reporterResourceID,
		version:            version,
		generation:         generation,
		commonVersion:      commonVersion,
		reporterVersion:    reporterVersion,
		tombstone:          true,
	}
}
