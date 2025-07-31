package model

import (
	"fmt"

	"github.com/google/uuid"
)

type CommonRepresentation struct {
	Representation
	resourceId ResourceId
	version    Version
	reporter   Reporter
}

func NewCommonRepresentation(
	resourceIdVal uuid.UUID,
	data JsonObject,
	versionVal uint,
	reportedByReporterType string,
	reportedByReporterInstance string,
) (CommonRepresentation, error) {
	if len(data) == 0 {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation requires non-empty data")
	}

	resourceId, err := NewResourceId(resourceIdVal)
	if err != nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid resource ID: %w", err)
	}

	reporter, err := NewReporter(reportedByReporterType, reportedByReporterInstance)
	if err != nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid reporter: %w", err)
	}

	version := NewVersion(versionVal)

	return CommonRepresentation{
		Representation: Representation{
			data: data,
		},
		resourceId: resourceId,
		version:    version,
		reporter:   reporter,
	}, nil
}
