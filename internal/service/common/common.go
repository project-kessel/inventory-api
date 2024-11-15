package common

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	pbrelation "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
	pbresource "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"strings"
)

func ReporterResourceIdFromPb(resourceType, reporterId string, reporter *pbresource.ReporterData) model.ReporterResourceId {
	return model.ReporterResourceId{
		LocalResourceId: reporter.LocalResourceId,
		ResourceType:    resourceType,
		ReporterId:      reporterId,
		ReporterType:    reporter.ReporterType.String(),
	}
}

func ResourceFromPb(resourceType, reporterId string, resourceData model.JsonObject, metadata *pbresource.Metadata, reporter *pbresource.ReporterData) *model.Resource {
	return &model.Resource{
		ID:           uuid.UUID{},
		ResourceData: resourceData,
		ResourceType: resourceType,
		WorkspaceId:  metadata.WorkspaceId,
		OrgId:        metadata.OrgId,
		Reporter: model.ResourceReporter{
			Reporter: model.Reporter{
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

func ToJsonObject(in interface{}) (model.JsonObject, error) {
	if in == nil {
		return nil, nil
	}

	bytes, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	resourceData := model.JsonObject{}
	err = json.Unmarshal(bytes, &resourceData)
	if err != nil {
		return nil, err
	}

	return resourceData, err
}

func labelsFromPb(pbLabels []*pbresource.ResourceLabel) model.Labels {
	labels := model.Labels{}
	for _, pbLabel := range pbLabels {
		labels = append(labels, model.Label{
			Key:   pbLabel.Key,
			Value: pbLabel.Value,
		})
	}
	return labels
}

func ReporterRelationshipIdFromPb(relationshipType, reporterId string, reporter *pbrelation.ReporterData) (model.ReporterRelationshipId, error) {
	res := strings.Split(relationshipType, "_")

	if len(res) != 3 {
		return model.ReporterRelationshipId{}, errors.New("invalid relationship type, not in the expected format subject_relation_object ")
	}

	subjectType := conform(res[0])
	objectType := conform(res[2])

	return model.ReporterRelationshipId{
		ReporterId:       reporterId,
		ReporterType:     reporter.ReporterType.String(),
		RelationshipType: relationshipType,
		SubjectId: model.ReporterResourceId{
			LocalResourceId: reporter.SubjectLocalResourceId,
			ResourceType:    subjectType,
			ReporterId:      reporterId,
			ReporterType:    reporter.ReporterType.String(),
		},
		ObjectId: model.ReporterResourceId{
			LocalResourceId: reporter.ObjectLocalResourceId,
			ResourceType:    objectType,
			ReporterId:      reporterId,
			ReporterType:    reporter.ReporterType.String(),
		},
	}, nil
}

func RelationshipFromPb(relationshipType, reporterId string, relationshipData model.JsonObject, metadata *pbrelation.Metadata, reporter *pbrelation.ReporterData) (*model.Relationship, error) {
	res := strings.Split(relationshipType, "_")

	if len(res) != 3 {
		return nil, errors.New("invalid relationship type, not in the expected format subject_relation_object ")
	}

	subjectType := conform(res[0])
	objectType := conform(res[2])

	return &model.Relationship{
		ID:               uuid.UUID{},
		RelationshipData: relationshipData,
		RelationshipType: relationshipType,
		SubjectId:        uuid.UUID{},
		ObjectId:         uuid.UUID{},
		OrgId:            metadata.OrgId,
		Reporter: model.RelationshipReporter{
			Reporter: model.Reporter{
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
