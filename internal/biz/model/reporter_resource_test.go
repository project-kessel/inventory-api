package model

import (
	"strings"
	"testing"
	"time"
)

func assertValidReporterResource(t *testing.T, reporterResource ReporterResource, err error, testCase string) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error for %s, got %v", testCase, err)
	}
	if reporterResource == (ReporterResource{}) {
		t.Errorf("Expected valid ReporterResource for %s, got empty struct", testCase)
	}
}

// consoleHrefPtr returns a pointer for *ConsoleHref parameters (optional field).
func consoleHrefPtr(ch ConsoleHref) *ConsoleHref { return &ch }

func TestReporterResource_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewReporterResourceTestFixture()

	t.Run("should create reporter resource with valid inputs", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidIdType(),
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidResourceIdType(),
			fixture.ValidApiHrefType(),
			consoleHrefPtr(fixture.ValidConsoleHrefType()),
		)

		assertValidReporterResource(t, reporterResource, err, "valid inputs")
	})

	t.Run("should create reporter resource with empty console href", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidIdType(),
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidResourceIdType(),
			fixture.ValidApiHrefType(),
			nil, // empty console href
		)

		assertValidReporterResource(t, reporterResource, err, "empty console href")
	})

	t.Run("should accept local resource ID in UUID format", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidIdType(),
			fixture.ValidLocalResourceIdUUIDType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidResourceIdType(),
			fixture.ValidApiHrefType(),
			consoleHrefPtr(fixture.ValidConsoleHrefType()),
		)

		assertValidReporterResource(t, reporterResource, err, "UUID format local resource ID")
	})

	t.Run("should accept local resource ID in string format", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidIdType(),
			fixture.ValidLocalResourceIdStringType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidResourceIdType(),
			fixture.ValidApiHrefType(),
			consoleHrefPtr(fixture.ValidConsoleHrefType()),
		)

		assertValidReporterResource(t, reporterResource, err, "string format local resource ID")
	})
}

func TestReporterResource_Id(t *testing.T) {
	t.Parallel()
	fixture := NewReporterResourceTestFixture()

	t.Run("should return the correct reporter resource ID", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidIdType(),
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidResourceIdType(),
			fixture.ValidApiHrefType(),
			consoleHrefPtr(fixture.ValidConsoleHrefType()),
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		id := reporterResource.Id()
		if id.UUID() != fixture.ValidId {
			t.Errorf("Expected ID %v, got %v", fixture.ValidIdType(), id.UUID())
		}
	})
}

func TestReporterResource_Key(t *testing.T) {
	t.Parallel()
	fixture := NewReporterResourceTestFixture()

	t.Run("should return the correct reporter resource key", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidIdType(),
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidResourceIdType(),
			fixture.ValidApiHrefType(),
			consoleHrefPtr(fixture.ValidConsoleHrefType()),
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		key := reporterResource.Key()
		if key.LocalResourceId().String() != fixture.ValidLocalResourceId {
			t.Errorf("Expected LocalResourceId %s, got %s", fixture.ValidLocalResourceIdType(), key.LocalResourceId())
		}
		if key.ResourceType().String() != fixture.ValidResourceType {
			t.Errorf("Expected ResourceType %s, got %s", fixture.ValidResourceTypeType(), key.ResourceType())
		}
		if key.ReporterType().String() != fixture.ValidReporterType {
			t.Errorf("Expected ReporterType %s, got %s", fixture.ValidReporterTypeType(), key.ReporterType())
		}
		if key.ReporterInstanceId().String() != fixture.ValidReporterInstanceId {
			t.Errorf("Expected ReporterInstanceId %s, got %s", fixture.ValidReporterInstanceIdType(), key.ReporterInstanceId())
		}
	})
}

func TestReporterResource_Update(t *testing.T) {
	t.Parallel()
	fixture := NewReporterResourceTestFixture()

	t.Run("should update apiHref and consoleHref successfully", func(t *testing.T) {
		t.Parallel()

		original, err := NewReporterResource(
			fixture.ValidIdType(),
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidResourceIdType(),
			fixture.ValidApiHrefType(),
			consoleHrefPtr(fixture.ValidConsoleHrefType()),
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		newApiHref, err := NewApiHref("https://api.example.com/updated")
		if err != nil {
			t.Fatalf("Failed to create API href: %v", err)
		}
		newConsoleHref, err := NewConsoleHref("https://console.example.com/updated")
		if err != nil {
			t.Fatalf("Failed to create console href: %v", err)
		}

		original.Update(newApiHref, &newConsoleHref)

		if original.apiHref.String() != newApiHref.String() {
			t.Errorf("Expected updated apiHref %s, got %s", newApiHref.String(), original.apiHref.String())
		}
		if original.ConsoleHref().String() != newConsoleHref.String() {
			t.Errorf("Expected updated consoleHref %s, got %s", newConsoleHref.String(), original.ConsoleHref().String())
		}
	})

	t.Run("should increment representation version", func(t *testing.T) {
		t.Parallel()

		original, err := NewReporterResource(
			fixture.ValidIdType(),
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidResourceIdType(),
			fixture.ValidApiHrefType(),
			consoleHrefPtr(fixture.ValidConsoleHrefType()),
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		originalVersion := original.representationVersion.Uint()

		newApiHref, err := NewApiHref("https://api.example.com/updated")
		if err != nil {
			t.Fatalf("Failed to create API href: %v", err)
		}
		newConsoleHref, err := NewConsoleHref("https://console.example.com/updated")
		if err != nil {
			t.Fatalf("Failed to create console href: %v", err)
		}

		original.Update(newApiHref, &newConsoleHref)

		expectedVersion := originalVersion + 1
		if original.representationVersion.Uint() != expectedVersion {
			t.Errorf("Expected representation version %d, got %d", expectedVersion, original.representationVersion.Uint())
		}
	})

	t.Run("should preserve other fields unchanged", func(t *testing.T) {
		t.Parallel()

		original, err := NewReporterResource(
			fixture.ValidIdType(),
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidResourceIdType(),
			fixture.ValidApiHrefType(),
			consoleHrefPtr(fixture.ValidConsoleHrefType()),
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		// Store original values to check they remain unchanged
		originalId := original.id
		originalKey := original.ReporterResourceKey
		originalResourceId := original.resourceID
		originalGeneration := original.generation
		originalTombstone := original.tombstone

		newApiHref, err := NewApiHref("https://api.example.com/updated")
		if err != nil {
			t.Fatalf("Failed to create API href: %v", err)
		}
		newConsoleHref, err := NewConsoleHref("https://console.example.com/updated")
		if err != nil {
			t.Fatalf("Failed to create console href: %v", err)
		}

		original.Update(newApiHref, &newConsoleHref)

		if original.id != originalId {
			t.Errorf("Expected ID to remain unchanged")
		}
		if original.ReporterResourceKey != originalKey {
			t.Errorf("Expected ReporterResourceKey to remain unchanged")
		}
		if original.resourceID != originalResourceId {
			t.Errorf("Expected resourceID to remain unchanged")
		}
		if original.generation != originalGeneration {
			t.Errorf("Expected generation to remain unchanged")
		}
		if original.tombstone != originalTombstone {
			t.Errorf("Expected tombstone to remain unchanged")
		}
	})

	t.Run("should reject empty consoleHref", func(t *testing.T) {
		t.Parallel()

		_, err := NewConsoleHref("")

		if err == nil {
			t.Error("Expected error for empty consoleHref, got none")
		}
	})

	t.Run("should validate apiHref before update", func(t *testing.T) {
		t.Parallel()

		invalidApiHref := ""

		_, err := NewApiHref(invalidApiHref)

		if err == nil {
			t.Error("Expected error for invalid apiHref, got none")
		}
		if !strings.Contains(err.Error(), "ApiHref") {
			t.Errorf("Expected error about ApiHref, got %v", err)
		}
	})

	t.Run("should reject whitespace consoleHref", func(t *testing.T) {
		t.Parallel()

		whitespaceConsoleHref := "   "

		_, err := NewConsoleHref(whitespaceConsoleHref)

		if err == nil {
			t.Error("Expected error for whitespace consoleHref, got none")
		}
	})
}

func TestReporterResource_TimestampsAreSetOnCreation(t *testing.T) {
	t.Parallel()
	fixture := NewReporterResourceTestFixture()

	rr, err := NewReporterResource(
		fixture.ValidIdType(),
		fixture.ValidLocalResourceIdType(),
		fixture.ValidResourceTypeType(),
		fixture.ValidReporterTypeType(),
		fixture.ValidReporterInstanceIdType(),
		fixture.ValidResourceIdType(),
		fixture.ValidApiHrefType(),
		consoleHrefPtr(fixture.ValidConsoleHrefType()),
	)

	if err != nil {
		t.Fatalf("NewReporterResource failed: %v", err)
	}

	if rr.CreatedAt().IsZero() || rr.UpdatedAt().IsZero() {
		t.Error("Expected timestamps to be set")
	}

	if rr.UpdatedAt().Sub(rr.CreatedAt()) > time.Millisecond {
		t.Error("Expected createdAt and updatedAt to be nearly identical on creation")
	}
}

func TestReporterResource_CreatedAtStaysConstantOnUpdate(t *testing.T) {
	t.Parallel()
	fixture := NewReporterResourceTestFixture()

	rr, _ := NewReporterResource(
		fixture.ValidIdType(),
		fixture.ValidLocalResourceIdType(),
		fixture.ValidResourceTypeType(),
		fixture.ValidReporterTypeType(),
		fixture.ValidReporterInstanceIdType(),
		fixture.ValidResourceIdType(),
		fixture.ValidApiHrefType(),
		consoleHrefPtr(fixture.ValidConsoleHrefType()),
	)

	originalCreatedAt := rr.CreatedAt()
	time.Sleep(10 * time.Millisecond)

	newApiHref, _ := NewApiHref("https://api.updated.com")
	newConsoleHref, _ := NewConsoleHref("https://console.updated.com")
	rr.Update(newApiHref, &newConsoleHref)

	if !rr.CreatedAt().Equal(originalCreatedAt) {
		t.Error("createdAt should not change on Update")
	}

	if rr.UpdatedAt().Sub(rr.CreatedAt()) < 10*time.Millisecond {
		t.Error("updatedAt should be different from createdAt after Update")
	}
}

func TestReporterResource_CreatedAtStaysConstantOnDelete(t *testing.T) {
	t.Parallel()
	fixture := NewReporterResourceTestFixture()

	rr, _ := NewReporterResource(
		fixture.ValidIdType(),
		fixture.ValidLocalResourceIdType(),
		fixture.ValidResourceTypeType(),
		fixture.ValidReporterTypeType(),
		fixture.ValidReporterInstanceIdType(),
		fixture.ValidResourceIdType(),
		fixture.ValidApiHrefType(),
		consoleHrefPtr(fixture.ValidConsoleHrefType()),
	)

	originalCreatedAt := rr.CreatedAt()
	time.Sleep(10 * time.Millisecond)
	rr.Delete()

	if !rr.CreatedAt().Equal(originalCreatedAt) {
		t.Error("createdAt should not change on Delete")
	}

	if rr.UpdatedAt().Sub(rr.CreatedAt()) < 10*time.Millisecond {
		t.Error("updatedAt should be different from createdAt after Delete")
	}
}
