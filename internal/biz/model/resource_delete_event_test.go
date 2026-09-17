//go:build test

package model

import (
	"strings"
	"testing"
)

func TestResourceDeleteEvent_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewResourceEventTestFixture()

	t.Run("should create resource delete event with valid inputs", func(t *testing.T) {
		t.Parallel()

		deleteRepresentation, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidReporterVersionType(),
			fixture.ValidReporterGenerationType(),
		)
		if err != nil {
			t.Fatalf("Failed to create delete representation: %v", err)
		}

		localResourceId, err := NewLocalResourceId("valid-resource-id")
		if err != nil {
			t.Fatalf("Failed to create local resource ID: %v", err)
		}

		commonVersion := NewVersion(0)
		event, err := NewResourceDeleteEvent(
			fixture.ValidResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			localResourceId,
			deleteRepresentation,
			&commonVersion,
		)

		assertValidResourceDeleteEvent(t, event, err, "valid inputs")
	})

	t.Run("should create resource delete event with different values", func(t *testing.T) {
		t.Parallel()

		deleteRepresentation, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidReporterVersionType(),
			fixture.ValidReporterGenerationType(),
		)
		if err != nil {
			t.Fatalf("Failed to create delete representation: %v", err)
		}

		localResourceId, err := NewLocalResourceId("another-resource-id")
		if err != nil {
			t.Fatalf("Failed to create local resource ID: %v", err)
		}

		commonVersion := NewVersion(1)
		event, err := NewResourceDeleteEvent(
			fixture.AnotherResourceIdType(),
			fixture.AnotherResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			localResourceId,
			deleteRepresentation,
			&commonVersion,
		)

		assertValidResourceDeleteEvent(t, event, err, "different values")
	})
}

func assertValidResourceDeleteEvent(t *testing.T, event ResourceDeleteEvent, err error, context string) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error for %s, got %v", context, err)
	}
	if event.id.String() == "" {
		t.Errorf("Expected valid resource ID for %s", context)
	}
	if event.resourceType.String() == "" {
		t.Errorf("Expected valid resource type for %s", context)
	}
	if event.reporterId.reporterType.String() == "" {
		t.Errorf("Expected valid reporter type for %s", context)
	}
	if event.reporterId.reporterInstanceId.String() == "" {
		t.Errorf("Expected valid reporter instance ID for %s", context)
	}
}

func assertInvalidResourceDeleteEvent(t *testing.T, err error, expectedErrorSubstring string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
	}
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error containing %s, got %v", expectedErrorSubstring, err)
	}
}

func TestResourceDeleteEvent_CommonVersion(t *testing.T) {
	t.Parallel()
	fixture := NewResourceEventTestFixture()

	t.Run("should include common version when provided", func(t *testing.T) {
		t.Parallel()

		deleteRepresentation, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidReporterVersionType(),
			fixture.ValidReporterGenerationType(),
		)
		if err != nil {
			t.Fatalf("Failed to create delete representation: %v", err)
		}

		localResourceId, err := NewLocalResourceId("test-resource-123")
		if err != nil {
			t.Fatalf("Failed to create local resource ID: %v", err)
		}

		commonVersion := NewVersion(2)
		event, err := NewResourceDeleteEvent(
			fixture.ValidResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			localResourceId,
			deleteRepresentation,
			&commonVersion,
		)

		if err != nil {
			t.Fatalf("Failed to create delete event with common version: %v", err)
		}

		retrievedVersion := event.CurrentCommonVersion()
		if retrievedVersion == nil {
			t.Error("Expected common version to be present, got nil")
		}
		if retrievedVersion != nil && retrievedVersion.Uint() != 2 {
			t.Errorf("Expected common version 2, got %d", retrievedVersion.Uint())
		}
	})

	t.Run("should allow nil common version for reporter-only resources", func(t *testing.T) {
		t.Parallel()

		deleteRepresentation, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidReporterVersionType(),
			fixture.ValidReporterGenerationType(),
		)
		if err != nil {
			t.Fatalf("Failed to create delete representation: %v", err)
		}

		localResourceId, err := NewLocalResourceId("reporter-only-resource")
		if err != nil {
			t.Fatalf("Failed to create local resource ID: %v", err)
		}

		event, err := NewResourceDeleteEvent(
			fixture.ValidResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			localResourceId,
			deleteRepresentation,
			nil, // nil for reporter-only resources
		)

		if err != nil {
			t.Fatalf("Failed to create delete event with nil common version: %v", err)
		}

		retrievedVersion := event.CurrentCommonVersion()
		if retrievedVersion != nil {
			t.Errorf("Expected nil common version for reporter-only resource, got %v", retrievedVersion)
		}
	})
}
