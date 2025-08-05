package model

import (
	"fmt"
	"time"
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

func NewReporterResource(
	id ReporterResourceId,
	localResourceId LocalResourceId,
	resourceType ResourceType,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
	resourceId ResourceId,
	apiHref ApiHref,
	consoleHref ConsoleHref,
) (ReporterResource, error) {
	reporterResourceKey, err := NewReporterResourceKey(
		localResourceId,
		resourceType,
		reporterType,
		reporterInstanceId,
	)
	if err != nil {
		return ReporterResource{}, fmt.Errorf("ReporterResource invalid key: %w", err)
	}

	resource := ReporterResource{
		id:                    id,
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
	localResourceID LocalResourceId,
	resourceType ResourceType,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
) (ReporterResourceKey, error) {
	reporterId := NewReporterId(reporterType, reporterInstanceId)

	return ReporterResourceKey{
		localResourceID: localResourceID,
		resourceType:    resourceType,
		reporter:        reporterId,
	}, nil
}

func (rr *ReporterResource) Update(
	apiHref ApiHref,
	consoleHref ConsoleHref,
) {
	rr.apiHref = apiHref
	rr.consoleHref = consoleHref
	rr.representationVersion = rr.representationVersion.Increment()
}

// CreateSnapshot creates a snapshot of the ReporterResource
func (rr ReporterResource) CreateSnapshot() (ReporterResourceSnapshot, error) {
	return rr.Serialize(), nil
}

func (rrk ReporterResourceKey) LocalResourceId() string {
	return rrk.localResourceID.String()
}

func (rrk ReporterResourceKey) ResourceType() string {
	return rrk.resourceType.String()
}

func (rrk ReporterResourceKey) ReporterType() string {
	return rrk.reporter.reporterType.String()
}

func (rrk ReporterResourceKey) ReporterInstanceId() string {
	return rrk.reporter.reporterInstanceId.String()
}

func (rrk ReporterResourceKey) Serialize() (string, string, string, string) {
	reporterType, reporterInstanceId := rrk.reporter.Serialize()
	return rrk.localResourceID.Serialize(), rrk.resourceType.Serialize(), reporterType, reporterInstanceId
}

func DeserializeReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId string) ReporterResourceKey {
	return ReporterResourceKey{
		localResourceID: DeserializeLocalResourceId(localResourceId),
		resourceType:    DeserializeResourceType(resourceType),
		reporter:        DeserializeReporterId(reporterType, reporterInstanceId),
	}
}

func (rr ReporterResource) LocalResourceId() string {
	return rr.localResourceID.String()
}

func (rr ReporterResource) Id() ReporterResourceId {
	return rr.id
}

func (rr ReporterResource) Key() ReporterResourceKey {
	return rr.ReporterResourceKey
}

func (rr ReporterResource) Serialize() ReporterResourceSnapshot {
	// Create ReporterResourceKey snapshot
	keySnapshot := ReporterResourceKeySnapshot{
		LocalResourceID:    rr.localResourceID.String(),
		ReporterType:       rr.reporter.reporterType.String(),
		ResourceType:       rr.resourceType.String(),
		ReporterInstanceID: rr.reporter.reporterInstanceId.String(),
	}

	// Create ReporterResource snapshot - direct initialization without validation
	return ReporterResourceSnapshot{
		ID:                    rr.id.UUID(),
		ReporterResourceKey:   keySnapshot,
		ResourceID:            rr.resourceID.UUID(),
		APIHref:               rr.apiHref.String(),
		ConsoleHref:           rr.consoleHref.String(),
		RepresentationVersion: rr.representationVersion.Serialize(),
		Generation:            rr.generation.Serialize(),
		Tombstone:             rr.tombstone.Bool(),
		CreatedAt:             time.Now(), // TODO: Add proper timestamp from domain entity if available
		UpdatedAt:             time.Now(), // TODO: Add proper timestamp from domain entity if available
	}
}

func DeserializeReporterResource(snapshot ReporterResourceSnapshot) ReporterResource {
	// Create domain tiny types directly from snapshot values - no validation
	reporterResourceId := ReporterResourceId(snapshot.ID)
	domainResourceId := ResourceId(snapshot.ResourceID)
	localResourceID := LocalResourceId(snapshot.ReporterResourceKey.LocalResourceID)
	resourceType := ResourceType(snapshot.ReporterResourceKey.ResourceType)
	reporterType := ReporterType(snapshot.ReporterResourceKey.ReporterType)
	reporterInstanceId := ReporterInstanceId(snapshot.ReporterResourceKey.ReporterInstanceID)
	apiHref := ApiHref(snapshot.APIHref)
	consoleHref := ConsoleHref(snapshot.ConsoleHref)

	// Create reporter ID
	reporterId := ReporterId{
		reporterType:       reporterType,
		reporterInstanceId: reporterInstanceId,
	}

	// Create reporter resource key
	reporterResourceKey := ReporterResourceKey{
		localResourceID: localResourceID,
		resourceType:    resourceType,
		reporter:        reporterId,
	}

	return ReporterResource{
		id:                    reporterResourceId,
		ReporterResourceKey:   reporterResourceKey,
		resourceID:            domainResourceId,
		apiHref:               apiHref,
		consoleHref:           consoleHref,
		representationVersion: DeserializeVersion(snapshot.RepresentationVersion),
		generation:            DeserializeGeneration(snapshot.Generation),
		tombstone:             DeserializeTombstone(snapshot.Tombstone),
	}
}
