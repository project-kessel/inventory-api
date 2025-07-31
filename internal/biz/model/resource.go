package model

import (
	"fmt"

	"github.com/google/uuid"
)

const initialCommonVersion = 0

type Resource struct {
	id                ResourceId
	resourceType      ResourceType
	commonVersion     Version
	consistencyToken  ConsistencyToken //nolint:unused
	reporterResources []ReporterResource
}

func NewResource(id uuid.UUID, resourceType string, reporterResources []ReporterResource) (Resource, error) {
	if len(reporterResources) == 0 {
		return Resource{}, fmt.Errorf("Resource must have at least one ReporterResource")
	}

	resourceTypeObj, err := NewResourceType(resourceType)
	if err != nil {
		return Resource{}, fmt.Errorf("Resource invalid type: %w", err)
	}

	resourceId, err := NewResourceId(id)
	if err != nil {
		return Resource{}, fmt.Errorf("Resource invalid ID: %w", err)
	}

	resource := Resource{
		id:                resourceId,
		resourceType:      resourceTypeObj,
		commonVersion:     NewVersion(initialCommonVersion),
		reporterResources: reporterResources,
	}
	return resource, nil
}
