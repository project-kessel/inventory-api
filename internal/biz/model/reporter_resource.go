package model

import (
	"fmt"

	"github.com/google/uuid"
)

type ReporterResource struct {
	id ReporterResourceId
	ReporterResourceKey

	resourceID  ResourceId
	apiHref     ApiHref
	consoleHref ConsoleHref

	representationVersion Version
	generation            Generation
	tombstone             Tombstone
}

type ReporterResourceKey struct {
	localResourceID LocalResourceId
	resourceType    ResourceType
	reporter        ReporterId
}

func NewReporterResource(idVal uuid.UUID, localResourceIdVal string, resourceTypeVal string, reporterTypeVal string, reporterInstanceIdVal string, resourceIdVal uuid.UUID, apiHrefVal string, consoleHrefVal string) (ReporterResource, error) {
	reporterResourceId, err := NewReporterResourceId(idVal)
	if err != nil {
		return ReporterResource{}, fmt.Errorf("ReporterResource invalid ID: %w", err)
	}

	reporterResourceKey, err := NewReporterResourceKey(localResourceIdVal, resourceTypeVal, reporterTypeVal, reporterInstanceIdVal)
	if err != nil {
		return ReporterResource{}, fmt.Errorf("ReporterResource invalid key: %w", err)
	}

	resourceId, err := NewResourceId(resourceIdVal)
	if err != nil {
		return ReporterResource{}, fmt.Errorf("ReporterResource invalid resource ID: %w", err)
	}

	apiHref, err := NewApiHref(apiHrefVal)
	if err != nil {
		return ReporterResource{}, fmt.Errorf("ReporterResource invalid API href: %w", err)
	}

	var consoleHref ConsoleHref
	if consoleHrefVal != "" {
		consoleHref, err = NewConsoleHref(consoleHrefVal)
		if err != nil {
			return ReporterResource{}, fmt.Errorf("ReporterResource invalid console href: %w", err)
		}
	}

	resource := ReporterResource{
		id:                    reporterResourceId,
		ReporterResourceKey:   reporterResourceKey,
		resourceID:            resourceId,
		apiHref:               apiHref,
		consoleHref:           consoleHref,
		representationVersion: NewVersion(initialReporterRepresentationVersion),
		generation:            NewGeneration(initialGeneration),
		tombstone:             NewTombstone(initialTombstone),
	}
	return resource, nil
}

func NewReporterResourceKey(
	localResourceIDVal string,
	resourceTypeVal string,
	reporterTypeVal string,
	reporterInstanceIdVal string,
) (ReporterResourceKey, error) {
	localResourceID, err := NewLocalResourceId(localResourceIDVal)
	if err != nil {
		return ReporterResourceKey{}, fmt.Errorf("ReporterResourceKey invalid local resource ID: %w", err)
	}

	resourceType, err := NewResourceType(resourceTypeVal)
	if err != nil {
		return ReporterResourceKey{}, fmt.Errorf("ReporterResourceKey invalid resource type: %w", err)
	}

	reporterType, err := NewReporterType(reporterTypeVal)
	if err != nil {
		return ReporterResourceKey{}, fmt.Errorf("ReporterResourceKey invalid reporter type: %w", err)
	}

	reporterInstanceId, err := NewReporterInstanceId(reporterInstanceIdVal)
	if err != nil {
		return ReporterResourceKey{}, fmt.Errorf("ReporterResourceKey invalid reporter instance: %w", err)
	}

	reporter, err := NewReporterId(reporterType, reporterInstanceId)
	if err != nil {
		return ReporterResourceKey{}, fmt.Errorf("ReporterResourceKey invalid reporter: %w", err)
	}

	return ReporterResourceKey{
		localResourceID: localResourceID,
		resourceType:    resourceType,
		reporter:        reporter,
	}, nil
}
