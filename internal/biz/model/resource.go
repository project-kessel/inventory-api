package model

import (
	"fmt"
)

const initialCommonVersion = 0

type Resource struct {
	id                ResourceId
	resourceType      ResourceType
	commonVersion     Version
	consistencyToken  ConsistencyToken //nolint:unused
	reporterResources []ReporterResource
}

func NewResource(id ResourceId, resourceType ResourceType, reporterResources []ReporterResource) (Resource, error) {
	if len(reporterResources) == 0 {
		return Resource{}, fmt.Errorf("Resource must have at least one ReporterResource")
	}

	resource := Resource{
		id:                id,
		resourceType:      resourceType,
		commonVersion:     NewVersion(initialCommonVersion),
		reporterResources: reporterResources,
	}
	return resource, nil
}
