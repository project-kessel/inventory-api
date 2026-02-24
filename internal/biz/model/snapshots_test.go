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
		&transactionId,
		reporterResourceId,
		apiHref,
		&consoleHref,
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
	ch := reporterResourceFixture.ValidConsoleHrefType()
	reporterResource, err := NewReporterResource(
		reporterResourceFixture.ValidIdType(),
		reporterResourceFixture.ValidLocalResourceIdType(),
		reporterResourceFixture.ValidResourceTypeType(),
		reporterResourceFixture.ValidReporterTypeType(),
		reporterResourceFixture.ValidReporterInstanceIdType(),
		reporterResourceFixture.ValidResourceIdType(),
		reporterResourceFixture.ValidApiHrefType(),
		&ch,
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
		versionOne,
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
	now := time.Now()
	snapshot := ResourceSnapshot{
		ID:               uuid.New(),
		Type:             "test-resource",
		CommonVersion:    1,
		ConsistencyToken: "test-token",
		CreatedAt:        now,
		UpdatedAt:        now,
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

// TestCommonRepresentationSnapshot_TransactionId_NilMeansNotSet documents that
// when TransactionId is not set, the snapshot uses nil (optional semantics).
func TestCommonRepresentationSnapshot_TransactionId_NilMeansNotSet(t *testing.T) {
	t.Parallel()

	// Build a snapshot with nil TransactionId (optional = not set)
	snapshot := CommonRepresentationSnapshot{
		Representation:             RepresentationSnapshot{Data: map[string]interface{}{"id": "test"}},
		ResourceId:                 uuid.New(),
		Version:                    1,
		ReportedByReporterType:     "test-reporter",
		ReportedByReporterInstance: "instance-1",
		TransactionId:              nil, // nil = not set (optional)
		CreatedAt:                  time.Time{},
	}
	// Round-trip: deserialize to domain, serialize back
	cr := DeserializeCommonRepresentation(&snapshot)
	roundTrip := cr.Serialize()
	if roundTrip.TransactionId != nil {
		t.Errorf("Expected nil TransactionId to round-trip as nil, got %v", roundTrip.TransactionId)
	}
}

// TestReporterRepresentationSnapshot_TransactionId_NilMeansNotSet documents the same
// optional convention for reporter representation snapshots (nil = not set).
func TestReporterRepresentationSnapshot_TransactionId_NilMeansNotSet(t *testing.T) {
	t.Parallel()

	snapshot := ReporterRepresentationSnapshot{
		Representation:     RepresentationSnapshot{Data: map[string]interface{}{"k": "v"}},
		ReporterResourceID: uuid.New(),
		Version:            1,
		Generation:         0,
		ReporterVersion:    nil,
		CommonVersion:      0,
		TransactionId:      nil, // nil = not set
		Tombstone:          false,
		CreatedAt:          time.Time{},
	}
	// Deserialize and serialize back
	rr := DeserializeReporterDataRepresentation(&snapshot)
	roundTrip := rr.Serialize()
	if roundTrip.TransactionId != nil {
		t.Errorf("Expected nil TransactionId to round-trip as nil, got %v", roundTrip.TransactionId)
	}
}

// TestResourceSnapshot_CreatedAtUpdatedAt_ZeroMeansNotSet documents that
// when timestamps are not set, the snapshot uses zero value (time.Time).
func TestResourceSnapshot_CreatedAtUpdatedAt_ZeroMeansNotSet(t *testing.T) {
	t.Parallel()

	snapshot := ResourceSnapshot{
		ID:               uuid.New(),
		Type:             "test",
		CommonVersion:    0,
		ConsistencyToken: "",
		CreatedAt:        time.Time{}, // zero = not set
		UpdatedAt:        time.Time{},
	}
	if !snapshot.CreatedAt.IsZero() {
		t.Error("CreatedAt zero means not set")
	}
	if !snapshot.UpdatedAt.IsZero() {
		t.Error("UpdatedAt zero means not set")
	}
}
