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

	reporter, err := NewReporter(reporterTypeVal, reporterInstanceIdVal)
	if err != nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation invalid reporter: %w", err)
	}

	version := NewVersion(versionVal)

	return CommonRepresentation{
		Representation: Representation{data: data},
		resourceId:     resourceId,
		version:        version,
		reporter:       reporter,
	}, nil
}

func (cr CommonRepresentation) Serialize() (*datamodel.CommonRepresentation, error) {
	return datamodel.NewCommonRepresentation(
		uuid.UUID(cr.resourceId),
		internal.JsonObject(cr.data),
		uint(cr.version),
		cr.reporter.reporterType.String(),
		cr.reporter.reporterInstanceId.String(),
	)
}
