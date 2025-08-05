package model

import (
	"fmt"

	"github.com/google/uuid"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
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
	dataReporterResource, err := rr.Serialize()
	if err != nil {
		return ReporterResourceSnapshot{}, err
	}

	return NewReporterResourceSnapshot(dataReporterResource), nil
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

func (rr ReporterResource) Serialize() (*datamodel.ReporterResource, error) {
	return datamodel.NewReporterResource(
		uuid.UUID(rr.id),
		rr.localResourceID.String(),
		rr.reporter.reporterType.String(),
		rr.resourceType.String(),
		rr.reporter.reporterInstanceId.String(),
		uuid.UUID(rr.resourceID),
		rr.apiHref.String(),
		rr.consoleHref.String(),
		uint(rr.representationVersion),
		uint(rr.generation),
		rr.tombstone.Bool(),
	)
}

func DeserializeReporterResource(
	id uuid.UUID,
	localResourceID string,
	reporterType string,
	resourceType string,
	reporterInstanceID string,
	resourceID uuid.UUID,
	apiHref string,
	consoleHref string,
	representationVersion uint,
	generation uint,
	tombstone bool,
) (*ReporterResource, error) {
	reporterResourceId := DeserializeReporterResourceId(id)
	reporterResourceKey := DeserializeReporterResourceKey(localResourceID, resourceType, reporterType, reporterInstanceID)
	domainResourceId := DeserializeResourceId(resourceID)
	domainApiHref := DeserializeApiHref(apiHref)
	domainConsoleHref := DeserializeConsoleHref(consoleHref)

	reporterResource := &ReporterResource{
		id:                    reporterResourceId,
		ReporterResourceKey:   reporterResourceKey,
		resourceID:            domainResourceId,
		apiHref:               domainApiHref,
		consoleHref:           domainConsoleHref,
		representationVersion: DeserializeVersion(representationVersion),
		generation:            DeserializeGeneration(generation),
		tombstone:             DeserializeTombstone(tombstone),
	}

	return reporterResource, nil
}
