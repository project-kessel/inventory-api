package model

import (
	"fmt"
	"strings"

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
	resourceId ResourceId,
	data Representation,
	version Version,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
) (CommonRepresentation, error) {
	if resourceId.UUID() == uuid.Nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid resource ID: ResourceId cannot be nil")
	}

	if strings.TrimSpace(string(reporterType)) == "" {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid reporter type: ReporterType cannot be empty")
	}

	if strings.TrimSpace(string(reporterInstanceId)) == "" {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid reporter instance ID: ReporterInstanceId cannot be empty")
	}

	if len(data.Data()) == 0 {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation requires non-empty data")
	}

	if data.Data() == nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation requires non-empty data")
	}

	reporter := NewReporterId(reporterType, reporterInstanceId)

	return CommonRepresentation{
		Representation: data,
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
