package model

import (
	"github.com/google/uuid"
)

const initialCommonVersion = 0

type Resource struct {
	id                uuid.UUID
	resourceType      string
	commonVersion     uint
	consistencyToken  string
	reporterResources []ReporterResource
}

func NewReporterResource(id uuid.UUID, resourceType string, reporterResource ReporterResource) (Resource, error) {
	resource := Resource{
		id:                id,
		resourceType:      resourceType,
		commonVersion:     initialCommonVersion,
		reporterResources: []ReporterResource{reporterResource},
	}
	return resource, nil
}
