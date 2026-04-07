package model

import (
	"encoding/json"
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
	transactionId := fixture.ValidTransactionIdType()
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
		transactionId,
		reporterResourceId,
		apiHref,
		consoleHref,
		&reporterData,
		&commonData,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to create test resource: %v", err)
	}

	// Test snapshot creation
	resourceSnapshot, reporterResourceSnapshot, reporterRepSnapshot, commonRepSnapshot, err := resource.Serialize()
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
	if commonRepSnapshot == nil {
		t.Fatal("CommonRepresentationSnapshot should not be nil")
	}
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
	if reporterRepSnapshot == nil {
		t.Fatal("ReporterRepresentationSnapshot should not be nil")
	}
	if reporterRepSnapshot.ReporterResourceID == uuid.Nil {
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

	rrSnapshot := reporterResource.Serialize()

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
		commonRepFixture.ValidTransactionIdType(),
	)
	if err != nil {
		t.Fatalf("Failed to create CommonRepresentation: %v", err)
	}

	crSnapshot := commonRep.Serialize()

	if crSnapshot.ResourceId == uuid.Nil {
		t.Error("CommonRepresentation snapshot should have valid ResourceId")
	}

	// Test ReporterDataRepresentation snapshot - using simpler approach
	versionOne := NewVersion(1)
	genOne := NewGeneration(1)
	testData := Representation(map[string]interface{}{"test": "data"})
	transactionId := NewTransactionId("test-transaction-id")

	dataRep, err := NewReporterDataRepresentation(
		resourceFixture.ValidReporterResourceIdType(),
		versionOne,
		genOne,
		testData,
		&versionOne,
		nil, // No reporter version for this test
		transactionId,
	)
	if err != nil {
		t.Fatalf("Failed to create ReporterDataRepresentation: %v", err)
	}

	dataSnapshot := dataRep.Serialize()

	if dataSnapshot.ReporterResourceID == uuid.Nil {
		t.Error("ReporterDataRepresentation snapshot should have valid ReporterResourceID")
	}

	// Test ReporterDeleteRepresentation snapshot
	deleteRep, err := NewReporterDeleteRepresentation(
		resourceFixture.ValidReporterResourceIdType(),
		versionOne,
		genOne)
	if err != nil {
		t.Fatalf("Failed to create ReporterDeleteRepresentation: %v", err)
	}

	deleteSnapshot := deleteRep.Serialize()

	if deleteSnapshot.ReporterResourceID == uuid.Nil {
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
	commonVersion := uint(1)
	snapshot := ResourceSnapshot{
		ID:               uuid.New(),
		Type:             "test-resource",
		CommonVersion:    &commonVersion,
		ConsistencyToken: "test-token",
	}

	// Test that all fields are accessible and properly typed
	if snapshot.ID == uuid.Nil {
		t.Error("Snapshot ID should be valid")
	}
	if snapshot.Type != "test-resource" {
		t.Error("Snapshot Type should match")
	}
	if snapshot.CommonVersion == nil || *snapshot.CommonVersion != 1 {
		t.Error("Snapshot CommonVersion should match")
	}

	t.Log("Snapshot serialization fields are accessible")

	t.Run("json with nil CommonVersion", func(t *testing.T) {
		t.Parallel()
		snapshotWithNil := ResourceSnapshot{
			ID:               uuid.New(),
			Type:             "test-resource",
			CommonVersion:    nil,
			ConsistencyToken: "test-token",
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		// Ensure the snapshot can be created and CommonVersion is nil
		if snapshotWithNil.CommonVersion != nil {
			t.Errorf("expected CommonVersion to be nil, got: %v", *snapshotWithNil.CommonVersion)
		}

		// Ensure other fields are still valid
		if snapshotWithNil.ID == uuid.Nil {
			t.Error("Snapshot ID should be valid even when CommonVersion is nil")
		}
		if snapshotWithNil.Type != "test-resource" {
			t.Error("Snapshot Type should match even when CommonVersion is nil")
		}

		// Verify that json:"common_version,omitempty" causes the field to be absent
		// in the marshaled output and remains nil after a round-trip unmarshal.
		data, err := json.Marshal(snapshotWithNil)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var unmarshaled ResourceSnapshot
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		if unmarshaled.CommonVersion != nil {
			t.Errorf("expected unmarshaled CommonVersion to be nil, got: %v", *unmarshaled.CommonVersion)
		}
	})
}

func TestCommonRepresentationSnapshot_TransactionId_EmptyMeansNotSet(t *testing.T) {
	t.Parallel()

	snapshot := CommonRepresentationSnapshot{
		Representation:             RepresentationSnapshot{Data: map[string]interface{}{"id": "test"}},
		ResourceId:                 uuid.New(),
		Version:                    1,
		ReportedByReporterType:     "test-reporter",
		ReportedByReporterInstance: "instance-1",
		TransactionId:              "",
		CreatedAt:                  time.Time{},
	}
	cr := DeserializeCommonRepresentation(&snapshot)
	roundTrip := cr.Serialize()
	if roundTrip.TransactionId != "" {
		t.Errorf("Expected empty TransactionId to round-trip as empty, got %v", roundTrip.TransactionId)
	}
}

func TestReporterRepresentationSnapshot_TransactionId_EmptyMeansNotSet(t *testing.T) {
	t.Parallel()

	snapshot := ReporterRepresentationSnapshot{
		Representation:     RepresentationSnapshot{Data: map[string]interface{}{"k": "v"}},
		ReporterResourceID: uuid.New(),
		Version:            1,
		Generation:         0,
		ReporterVersion:    nil,
		CommonVersion:      0,
		TransactionId:      "",
		Tombstone:          false,
		CreatedAt:          time.Time{},
	}
	rr := DeserializeReporterDataRepresentation(&snapshot)
	roundTrip := rr.Serialize()
	if roundTrip.TransactionId != "" {
		t.Errorf("Expected empty TransactionId to round-trip as empty, got %v", roundTrip.TransactionId)
	}
}

func TestResourceSnapshot_CreatedAtUpdatedAt_ZeroMeansNotSet(t *testing.T) {
	t.Parallel()

	snapshot := ResourceSnapshot{
		ID:               uuid.New(),
		Type:             "test",
		CommonVersion:    0,
		ConsistencyToken: "",
		CreatedAt:        time.Time{},
		UpdatedAt:        time.Time{},
	}
	if !snapshot.CreatedAt.IsZero() {
		t.Error("CreatedAt zero means not set")
	}
	if !snapshot.UpdatedAt.IsZero() {
		t.Error("UpdatedAt zero means not set")
	}
}
