package model

import (
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
)

type CommonRepresentation struct {
	Representation
	resourceId                 uuid.UUID
	version                    uint
	reportedByReporterType     string
	reportedByReporterInstance string
}

func NewCommonRepresentation(
	resourceId uuid.UUID,
	data internal.JsonObject,
	version uint,
	reportedByReporterType string,
	reportedByReporterInstance string,
) (CommonRepresentation, error) {
	return CommonRepresentation{
		Representation: Representation{
			data: data,
		},
		resourceId:                 resourceId,
		version:                    version,
		reportedByReporterType:     reportedByReporterType,
		reportedByReporterInstance: reportedByReporterInstance,
	}, nil
}
