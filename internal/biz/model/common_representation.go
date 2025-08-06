package model

import (
	"fmt"

	"github.com/google/uuid"
)

type CommonRepresentation struct {
	Representation
	resourceId ResourceId
	version    Version
	reporter   ReporterId
}

func NewCommonRepresentation(
	resourceIdVal uuid.UUID,
	data JsonObject,
	versionVal uint,
	reportedByReporterType string,
	reportedByReporterInstance string,
) (CommonRepresentation, error) {
	if len(data) == 0 {
		return CommonRepresentation{}, fmt.Errorf("%w: CommonRepresentation data", ErrInvalidData)
	}

	resourceId, err := NewResourceId(resourceIdVal)
	if err != nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid resource ID: %w", err)
	}

	reporterType, err := NewReporterType(reportedByReporterType)
	if err != nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid reporter type: %w", err)
	}

	reporterInstanceId, err := NewReporterInstanceId(reportedByReporterInstance)
	if err != nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid reporter instance: %w", err)
	}

	reporter, err := NewReporterId(reporterType, reporterInstanceId)
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
