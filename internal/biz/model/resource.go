package model

import (
	"github.com/google/uuid"
)

const initialCommonVersion = 0

type Resource struct {
	id                ResourceId
	resourceType      ResourceType
	commonVersion     Version
	consistencyToken  ConsistencyToken
	reporterResources []ReporterResource
}

func NewResource(id uuid.UUID, resourceType string, reporterResource ReporterResource) (Resource, error) {
	resourceTypeObj, err := NewResourceType(resourceType)
	if err != nil {
		return Resource{}, err
	}

	resourceId, err := NewResourceId(id)
	if err != nil {
		return Resource{}, err
	}

	defaultToken, err := NewConsistencyToken("initial")
	if err != nil {
		return Resource{}, err
	}

	resource := Resource{
		id:                resourceId,
		resourceType:      resourceTypeObj,
		commonVersion:     initialCommonVersion,
		consistencyToken:  defaultToken,
		reporterResources: []ReporterResource{reporterResource},
	}
	return resource, nil
}
