package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/google/uuid"

	pbrelation "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
	pbresource "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	pbresourcev1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
)

func ReporterResourceIdFromPb(resourceType, reporterId string, reporter *pbresource.ReporterData) model_legacy.ReporterResourceId {
	return model_legacy.ReporterResourceId{
		LocalResourceId: reporter.LocalResourceId,
		ResourceType:    resourceType,
		ReporterId:      reporterId,
		ReporterType:    reporter.ReporterType.String(),
	}
}

func ResourceFromPbv1beta1(resourceType, reporterId string, resourceData model_legacy.JsonObject, metadata *pbresource.Metadata, reporter *pbresource.ReporterData) *model_legacy.Resource {
	return &model_legacy.Resource{
		ID:                 uuid.UUID{},
		ResourceData:       resourceData,
		ResourceType:       resourceType,
		WorkspaceId:        metadata.WorkspaceId,
		OrgId:              metadata.OrgId,
		ReporterResourceId: reporter.LocalResourceId,
		ReporterId:         reporter.ReporterType.String(),
		Reporter: model_legacy.ResourceReporter{
			Reporter: model_legacy.Reporter{
				ReporterId:      reporterId,
				ReporterType:    reporter.ReporterType.String(),
				ReporterVersion: reporter.ReporterVersion,
			},
			LocalResourceId: reporter.LocalResourceId,
		},
		ConsoleHref: reporter.ConsoleHref,
		ApiHref:     reporter.ApiHref,
		Labels:      labelsFromPb(metadata.Labels),
	}
}

func ResourceFromPb(resourceType, reporterType string, reporterInstanceId string, reporterId string, resourceData model_legacy.JsonObject, workspaceId string, resourceRep *pbresourcev1beta2.ResourceRepresentations, inventoryId *uuid.UUID) *model_legacy.Resource {
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

func ToJsonObject(in interface{}) (model_legacy.JsonObject, error) {
	if in == nil {
		return nil, nil
	}

	bytes, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	resourceData := model_legacy.JsonObject{}
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
		return "", fmt.Errorf("reporterType is required but was empty")
	}
	return reporterType, nil
}

func ExtractReporterInstanceID(reporterInstanceId string) (string, error) {
	if reporterInstanceId == "" {
		return "", fmt.Errorf("reporterInstanceId is required but was empty")
	}
	return reporterInstanceId, nil
}

func labelsFromPb(pbLabels []*pbresource.ResourceLabel) model_legacy.Labels {
	labels := model_legacy.Labels{}
	for _, pbLabel := range pbLabels {
		labels = append(labels, model_legacy.Label{
			Key:   pbLabel.Key,
			Value: pbLabel.Value,
		})
	}
	return labels
}

func ReporterRelationshipIdFromPb(relationshipType, reporterId string, reporter *pbrelation.ReporterData) (model_legacy.ReporterRelationshipId, error) {
	res := strings.Split(relationshipType, "_")

	if len(res) != 3 {
		return model_legacy.ReporterRelationshipId{}, errors.New("invalid relationship type, not in the expected format subject_relation_object ")
	}

	subjectType := conform(res[0])
	objectType := conform(res[2])

	return model_legacy.ReporterRelationshipId{
		ReporterId:       reporterId,
		ReporterType:     reporter.ReporterType.String(),
		RelationshipType: relationshipType,
		SubjectId: model_legacy.ReporterResourceId{
			LocalResourceId: reporter.SubjectLocalResourceId,
			ResourceType:    subjectType,
			ReporterId:      reporterId,
			ReporterType:    reporter.ReporterType.String(),
		},
		ObjectId: model_legacy.ReporterResourceId{
			LocalResourceId: reporter.ObjectLocalResourceId,
			ResourceType:    objectType,
			ReporterId:      reporterId,
			ReporterType:    reporter.ReporterType.String(),
		},
	}, nil
}

func RelationshipFromPb(relationshipType, reporterId string, relationshipData model_legacy.JsonObject, metadata *pbrelation.Metadata, reporter *pbrelation.ReporterData) (*model_legacy.Relationship, error) {
	res := strings.Split(relationshipType, "_")

	if len(res) != 3 {
		return nil, errors.New("invalid relationship type, not in the expected format subject_relation_object ")
	}

	subjectType := conform(res[0])
	objectType := conform(res[2])

	return &model_legacy.Relationship{
		ID:               uuid.UUID{},
		RelationshipData: relationshipData,
		RelationshipType: relationshipType,
		SubjectId:        uuid.UUID{},
		ObjectId:         uuid.UUID{},
		OrgId:            metadata.OrgId,
		Reporter: model_legacy.RelationshipReporter{
			Reporter: model_legacy.Reporter{
				ReporterId:      reporterId,
				ReporterType:    reporter.ReporterType.String(),
				ReporterVersion: reporter.ReporterVersion,
			},
			SubjectLocalResourceId: reporter.SubjectLocalResourceId,
			SubjectResourceType:    subjectType,
			ObjectLocalResourceId:  reporter.ObjectLocalResourceId,
			ObjectResourceType:     objectType,
		},
	}, nil
}

// Conform converts any hyphens in resource types to underscores to conform with SpiceDB validation requirements
func conform(resource string) string {
	return strings.ReplaceAll(resource, "-", "_")
}
