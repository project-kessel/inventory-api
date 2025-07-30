package model

import (
	"github.com/google/uuid"
)

type CommonRepresentation struct {
	Representation
	resourceId ResourceId
	version    Version
	reporter   Reporter
}

func NewCommonRepresentation(
	resourceId uuid.UUID,
	data JsonObject,
	version uint,
	reportedByReporterType string,
	reportedByReporterInstance string,
) (*CommonRepresentation, error) {
	resourceIdType, err := NewResourceId(resourceId)
	if err != nil {
		return nil, err
	}

	reporterObj, err := NewReporter(reportedByReporterType, reportedByReporterInstance)
	if err != nil {
		return nil, err
	}

	versionType := NewVersion(version)

	return &CommonRepresentation{
		Representation: Representation{
			data: data,
		},
		resourceId: resourceIdType,
		version:    versionType,
		reporter:   reporterObj,
	}, nil
}
