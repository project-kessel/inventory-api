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

		event, err := NewResourceDeleteEvent(
			fixture.ValidResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidReporterDataType(),
			deleteRepresentation,
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

		event, err := NewResourceDeleteEvent(
			fixture.AnotherResourceIdType(),
			fixture.AnotherResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.AnotherReporterDataType(),
			deleteRepresentation,
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
