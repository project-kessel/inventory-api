package common

import (
	"encoding/json"
	"fmt"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/google/uuid"
	pbresourcev1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
)

func ResourceFromPb(resourceType, reporterType string, reporterInstanceId string, reporterId string, resourceData internal.JsonObject, workspaceId string, resourceRep *pbresourcev1beta2.ResourceRepresentations, inventoryId *uuid.UUID) *model_legacy.Resource {
	return &model_legacy.Resource{
		ID:                 uuid.UUID{},
		InventoryId:        inventoryId,
		ResourceData:       resourceData,
		ResourceType:       resourceType,
		WorkspaceId:        workspaceId,
		ReporterResourceId: resourceRep.Metadata.LocalResourceId,
		ReporterId:         reporterId,
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstanceId,
		ReporterVersion:    resourceRep.Metadata.GetReporterVersion(),
		ConsoleHref:        resourceRep.Metadata.GetConsoleHref(),
		ApiHref:            resourceRep.Metadata.ApiHref,
	}
}

func ToJsonObject(in interface{}) (internal.JsonObject, error) {
	if in == nil {
		return nil, nil
	}

	bytes, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	resourceData := internal.JsonObject{}
	err = json.Unmarshal(bytes, &resourceData)
	if err != nil {
		return nil, err
	}

	return resourceData, err
}

// TODO: Figure out how to store workspaceId in schema
func ExtractWorkspaceId(commonRepresentation *structpb.Struct) (string, error) {
	var workspaceId string
	if commonRepresentation != nil {
		workspaceId = commonRepresentation.GetFields()["workspace_id"].GetStringValue()
		return workspaceId, nil
	}
	return workspaceId, nil
}

func ExtractInventoryId(inventoryIDStr string) (*uuid.UUID, error) {
	if inventoryIDStr != "" {
		inventoryID, err := uuid.Parse(inventoryIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid inventory ID: %w", err)
		}
		return &inventoryID, nil
	}
	return nil, nil
}

func ExtractReporterType(reporterType string) (string, error) {
	if reporterType == "" {
		return "", fmt.Errorf("%w: ReporterType", bizmodel.ErrEmpty)
	}
	return reporterType, nil
}

func ExtractReporterInstanceID(reporterInstanceId string) (string, error) {
	if reporterInstanceId == "" {
		return "", fmt.Errorf("%w: ReporterInstanceId", bizmodel.ErrEmpty)
	}
	return reporterInstanceId, nil
}
