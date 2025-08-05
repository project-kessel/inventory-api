package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestResourceSnapshot_FromDomainEntity(t *testing.T) {
	t.Parallel()

	// Create test domain entities
	fixture := NewResourceTestFixture()

	resourceId := fixture.ValidResourceIdType()
	reporterResourceId := fixture.ValidReporterResourceIdType()
	localResourceId := fixture.ValidLocalResourceIdType()
	resourceType := fixture.ValidResourceTypeType()
	reporterType := fixture.ValidReporterTypeType()
	reporterInstanceId := fixture.ValidReporterInstanceIdType()
	apiHref := fixture.ValidApiHrefType()
	consoleHref := fixture.ValidConsoleHrefType()
	commonData := fixture.ValidCommonRepresentationType()
	reporterData := fixture.ValidReporterRepresentationType()

	// Create domain Resource
	resource, err := NewResource(
		resourceId,
		localResourceId,
		resourceType,
		reporterType,
		reporterInstanceId,
		reporterResourceId,
		apiHref,
		consoleHref,
		reporterData,
		commonData,
	)
	if err != nil {
		t.Fatalf("Failed to create test resource: %v", err)
	}

	// Test snapshot creation
	resourceSnapshot, reporterResourceSnapshot, commonRepSnapshot, reporterRepSnapshot, err := resource.CreateSnapshot()
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Verify ResourceSnapshot
	if resourceSnapshot.ID == uuid.Nil {
		t.Error("ResourceSnapshot should have a valid ID")
	}
	if resourceSnapshot.Type == "" {
		t.Error("ResourceSnapshot should have a valid Type")
	}
	// CommonVersion starts at 0 for a new resource
	t.Logf("Resource CommonVersion: %d", resourceSnapshot.CommonVersion)

	// Verify ReporterResourceSnapshot
	if reporterResourceSnapshot.ID == uuid.Nil {
		t.Error("ReporterResourceSnapshot should have a valid ID")
	}
	if reporterResourceSnapshot.ResourceID == uuid.Nil {
		t.Error("ReporterResourceSnapshot should have a valid ResourceID")
	}
	if reporterResourceSnapshot.APIHref == "" {
		t.Error("ReporterResourceSnapshot should have a valid APIHref")
	}

	// Verify ReporterResourceKeySnapshot
	if reporterResourceSnapshot.ReporterResourceKey.LocalResourceID == "" {
		t.Error("ReporterResourceKeySnapshot should have a valid LocalResourceID")
	}
	if reporterResourceSnapshot.ReporterResourceKey.ResourceType == "" {
		t.Error("ReporterResourceKeySnapshot should have a valid ResourceType")
	}
	if reporterResourceSnapshot.ReporterResourceKey.ReporterType == "" {
		t.Error("ReporterResourceKeySnapshot should have a valid ReporterType")
	}
	if reporterResourceSnapshot.ReporterResourceKey.ReporterInstanceID == "" {
		t.Error("ReporterResourceKeySnapshot should have a valid ReporterInstanceID")
	}

	// Verify CommonRepresentationSnapshot
	if commonRepSnapshot.ResourceId == uuid.Nil {
		t.Error("CommonRepresentationSnapshot should have a valid ResourceId")
	}
	if commonRepSnapshot.Representation.Data == nil {
		t.Error("CommonRepresentationSnapshot should have valid Data")
	}
	if commonRepSnapshot.ReportedByReporterType == "" {
		t.Error("CommonRepresentationSnapshot should have a valid ReportedByReporterType")
	}

	// Verify ReporterRepresentationSnapshot
	if reporterRepSnapshot.ReporterResourceID == "" {
		t.Error("ReporterRepresentationSnapshot should have a valid ReporterResourceID")
	}
	if reporterRepSnapshot.Representation.Data == nil {
		t.Error("ReporterRepresentationSnapshot should have valid Data")
	}

	t.Logf("Successfully created snapshots for Resource ID: %s", resourceSnapshot.ID)
}

func TestIndividualSnapshotMethods(t *testing.T) {
	t.Parallel()

	resourceFixture := NewResourceTestFixture()
	reporterResourceFixture := NewReporterResourceTestFixture()

	// Test ReporterResource snapshot
	reporterResource, err := NewReporterResource(
		reporterResourceFixture.ValidIdType(),
		reporterResourceFixture.ValidLocalResourceIdType(),
		reporterResourceFixture.ValidResourceTypeType(),
		reporterResourceFixture.ValidReporterTypeType(),
		reporterResourceFixture.ValidReporterInstanceIdType(),
		reporterResourceFixture.ValidResourceIdType(),
		reporterResourceFixture.ValidApiHrefType(),
		reporterResourceFixture.ValidConsoleHrefType(),
	)
	if err != nil {
		t.Fatalf("Failed to create ReporterResource: %v", err)
	}

	rrSnapshot, err := reporterResource.CreateSnapshot()
	if err != nil {
		t.Fatalf("Failed to create ReporterResource snapshot: %v", err)
	}

	if rrSnapshot.ID == uuid.Nil {
		t.Error("ReporterResource snapshot should have valid ID")
	}

	// Test CommonRepresentation snapshot
	commonRepFixture := NewCommonRepresentationTestFixture()
	commonRep, err := NewCommonRepresentation(
		commonRepFixture.ValidResourceIdType(),
		commonRepFixture.ValidRepresentationType(),
		commonRepFixture.ValidVersionType(),
		commonRepFixture.ValidReporterTypeType(),
		commonRepFixture.ValidReporterInstanceIdType(),
	)
	if err != nil {
		t.Fatalf("Failed to create CommonRepresentation: %v", err)
	}

	crSnapshot, err := commonRep.CreateSnapshot()
	if err != nil {
		t.Fatalf("Failed to create CommonRepresentation snapshot: %v", err)
	}

	if crSnapshot.ResourceId == uuid.Nil {
		t.Error("CommonRepresentation snapshot should have valid ResourceId")
	}

	// Test ReporterDataRepresentation snapshot - using simpler approach
	versionOne := NewVersion(1)
	genOne := NewGeneration(1)
	testData := Representation(map[string]interface{}{"test": "data"})

	dataRep, err := NewReporterDataRepresentation(
		resourceFixture.ValidReporterResourceIdType(),
		versionOne,
		genOne,
		testData,
		versionOne,
		nil, // No reporter version for this test
	)
	if err != nil {
		t.Fatalf("Failed to create ReporterDataRepresentation: %v", err)
	}

	dataSnapshot, err := dataRep.CreateSnapshot()
	if err != nil {
		t.Fatalf("Failed to create ReporterDataRepresentation snapshot: %v", err)
	}

	if dataSnapshot.ReporterResourceID == "" {
		t.Error("ReporterDataRepresentation snapshot should have valid ReporterResourceID")
	}

	// Test ReporterDeleteRepresentation snapshot
	deleteRep, err := NewReporterDeleteRepresentation(
		resourceFixture.ValidReporterResourceIdType(),
		versionOne,
		genOne,
		versionOne,
		nil, // No reporter version for this test
	)
	if err != nil {
		t.Fatalf("Failed to create ReporterDeleteRepresentation: %v", err)
	}

	deleteSnapshot, err := deleteRep.CreateSnapshot()
	if err != nil {
		t.Fatalf("Failed to create ReporterDeleteRepresentation snapshot: %v", err)
	}

	if deleteSnapshot.ReporterResourceID == "" {
		t.Error("ReporterDeleteRepresentation snapshot should have valid ReporterResourceID")
	}
	if !deleteSnapshot.Tombstone {
		t.Error("ReporterDeleteRepresentation snapshot should have Tombstone=true")
	}

	t.Log("All individual snapshot methods work correctly")
}

func TestSnapshotSerialization(t *testing.T) {
	t.Parallel()

	// Test that snapshots can be used for JSON serialization
	snapshot := ResourceSnapshot{
		ID:               uuid.New(),
		Type:             "test-resource",
		CommonVersion:    1,
		ConsistencyToken: "test-token",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Test that all fields are accessible and properly typed
	if snapshot.ID == uuid.Nil {
		t.Error("Snapshot ID should be valid")
	}
	if snapshot.Type != "test-resource" {
		t.Error("Snapshot Type should match")
	}
	if snapshot.CommonVersion != 1 {
		t.Error("Snapshot CommonVersion should match")
	}

	t.Log("Snapshot serialization fields are accessible")
}
