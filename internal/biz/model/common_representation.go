package model

import (
	"fmt"

	"github.com/project-kessel/inventory-api/internal"

	"github.com/google/uuid"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
)

type CommonRepresentation struct {
	Representation
	resourceId ResourceId
	version    Version
	reporter   ReporterId
}

func NewCommonRepresentation(
	resourceIdVal uuid.UUID,
	data internal.JsonObject,
	versionVal uint,
	reporterTypeVal string,
	reporterInstanceIdVal string,
) (CommonRepresentation, error) {
	if len(data) == 0 {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation requires non-empty data")
	}

	if data == nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation requires non-empty data")
	}

	resourceId, err := NewResourceId(resourceIdVal)
	if err != nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid resource ID: %w", err)
	}

	reporterType, err := NewReporterType(reporterTypeVal)
	if err != nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid reporter type: %w", err)
	}

	reporterInstanceId, err := NewReporterInstanceId(reporterInstanceIdVal)
	if err != nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid reporter instance ID: %w", err)
	}

	reporter := NewReporterId(reporterType, reporterInstanceId)

	version := NewVersion(versionVal)

	return CommonRepresentation{
		Representation: Representation(data),
		resourceId:     resourceId,
		version:        version,
		reporter:       reporter,
	}, nil
}

func (cr CommonRepresentation) Serialize() (*datamodel.CommonRepresentation, error) {
	reporterType, reporterInstanceId := cr.reporter.Serialize()
	return datamodel.NewCommonRepresentation(
		cr.resourceId.Serialize(),
		cr.Representation.Serialize(),
		cr.version.Serialize(),
		reporterType,
		reporterInstanceId,
	)
}

func DeserializeCommonRepresentation(
	resourceIdVal uuid.UUID,
	data internal.JsonObject,
	versionVal uint,
	reporterTypeVal string,
	reporterInstanceIdVal string,
) CommonRepresentation {
	return CommonRepresentation{
		Representation: DeserializeRepresentation(data),
		resourceId:     DeserializeResourceId(resourceIdVal),
		version:        DeserializeVersion(versionVal),
		reporter:       DeserializeReporterId(reporterTypeVal, reporterInstanceIdVal),
	}
}
