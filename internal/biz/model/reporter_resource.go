package model

import "github.com/google/uuid"

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
	reporter        Reporter
}

func NewReporterResource(idVal uuid.UUID, localResourceIdVal string, resourceTypeVal string, reporterTypeVal string, reporterInstanceIdVal string, resourceIdVal uuid.UUID, apiHrefVal string, consoleHrefVal string) (ReporterResource, error) {
	reporterResourceId, err := NewReporterResourceId(idVal)
	if err != nil {
		return ReporterResource{}, err
	}

	reporterResourceKey, err := NewReporterResourceKey(localResourceIdVal, resourceTypeVal, reporterTypeVal, reporterInstanceIdVal)
	if err != nil {
		return ReporterResource{}, err
	}

	resourceId, err := NewResourceId(resourceIdVal)
	if err != nil {
		return ReporterResource{}, err
	}

	apiHref, err := NewApiHref(apiHrefVal)
	if err != nil {
		return ReporterResource{}, err
	}

	consoleHref, err := NewConsoleHref(consoleHrefVal)
	if err != nil {
		return ReporterResource{}, err
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
		return ReporterResourceKey{}, err
	}

	resourceType, err := NewResourceType(resourceTypeVal)
	if err != nil {
		return ReporterResourceKey{}, err
	}

	reporter, err := NewReporter(reporterTypeVal, reporterInstanceIdVal)
	if err != nil {
		return ReporterResourceKey{}, err
	}

	return ReporterResourceKey{
		localResourceID: localResourceID,
		resourceType:    resourceType,
		reporter:        reporter,
	}, nil
}
