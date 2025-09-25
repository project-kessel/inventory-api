package model

import (
	"fmt"
	"log"
	"time"
)

// Create Entities with unexported fields for encapsulation
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

// Factory methods
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

// Model Behavior
func (rr *ReporterResource) Update(
	apiHref ApiHref,
	consoleHref ConsoleHref,
) {
	rr.apiHref = apiHref
	rr.consoleHref = consoleHref
	rr.representationVersion = rr.representationVersion.Increment()
	if rr.tombstone.Serialize() == true {
		rr.tombstone = false
		rr.generation = rr.generation.Increment()
		rr.representationVersion = initialReporterRepresentationVersion
	}
}

func (rr *ReporterResource) Delete() {
	rr.representationVersion = rr.representationVersion.Increment()
	rr.tombstone = true
}

// Add getters only where needed
func (rrk ReporterResourceKey) LocalResourceId() LocalResourceId {
	return rrk.localResourceID
}

func (rrk ReporterResourceKey) ResourceType() ResourceType {
	return rrk.resourceType
}

func (rrk ReporterResourceKey) ReporterType() ReporterType {
	return rrk.reporter.reporterType
}

func (rrk ReporterResourceKey) ReporterInstanceId() ReporterInstanceId {
	return rrk.reporter.reporterInstanceId
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

// Serialization + Deserialization functions, direct initialization without validation, convert to snapshots so we can bypass New validation
func (rr ReporterResource) Serialize() ReporterResourceSnapshot {
	keySnapshot := ReporterResourceKeySnapshot{
		LocalResourceID:    rr.localResourceID.Serialize(),
		ReporterType:       rr.reporter.reporterType.Serialize(),
		ResourceType:       rr.resourceType.Serialize(),
		ReporterInstanceID: rr.reporter.reporterInstanceId.Serialize(),
	}

	return ReporterResourceSnapshot{
		ID:                    rr.Id().Serialize(),
		ReporterResourceKey:   keySnapshot,
		ResourceID:            rr.resourceID.Serialize(),
		APIHref:               rr.apiHref.Serialize(),
		ConsoleHref:           rr.consoleHref.Serialize(),
		RepresentationVersion: rr.representationVersion.Serialize(),
		Generation:            rr.generation.Serialize(),
		Tombstone:             rr.tombstone.Serialize(),
		CreatedAt:             time.Now(), // TODO: Add proper timestamp from domain entity if available
		UpdatedAt:             time.Now(), // TODO: Add proper timestamp from domain entity if available
	}
}

func DeserializeReporterResource(snapshot ReporterResourceSnapshot) ReporterResource {

	log.Printf("----------------------------------")
	log.Printf("ReporterResourceSnapshot : %+v, ", snapshot)
	return ReporterResource{
		id:                    DeserializeReporterResourceId(snapshot.ID),
		ReporterResourceKey:   DeserializeReporterResourceKey(snapshot.ReporterResourceKey.LocalResourceID, snapshot.ReporterResourceKey.ResourceType, snapshot.ReporterResourceKey.ReporterType, snapshot.ReporterResourceKey.ReporterInstanceID),
		resourceID:            DeserializeResourceId(snapshot.ResourceID),
		apiHref:               DeserializeApiHref(snapshot.APIHref),
		consoleHref:           DeserializeConsoleHref(snapshot.ConsoleHref),
		representationVersion: DeserializeVersion(snapshot.RepresentationVersion),
		generation:            DeserializeGeneration(snapshot.Generation),
		tombstone:             DeserializeTombstone(snapshot.Tombstone),
	}
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
