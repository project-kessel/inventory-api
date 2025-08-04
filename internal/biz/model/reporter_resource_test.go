package model

import (
	"strings"
	"testing"
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

func assertInvalidReporterResource(t *testing.T, reporterResource ReporterResource, err error, expectedErrorSubstring string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
	}
	if reporterResource != (ReporterResource{}) {
		t.Error("Expected empty ReporterResource for invalid input, got non-empty")
	}
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error about %s, got %v", expectedErrorSubstring, err)
	}
}

func TestReporterResource_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewReporterResourceTestFixture()

	t.Run("should create reporter resource with valid inputs", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertValidReporterResource(t, reporterResource, err, "valid inputs")
	})

	t.Run("should create reporter resource with empty console href", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.EmptyConsoleHref,
		)

		assertValidReporterResource(t, reporterResource, err, "empty console href")
	})

	t.Run("should reject nil ID", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.NilId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "ReporterResource invalid ID")
	})

	t.Run("should reject empty local resource ID", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.EmptyLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "ReporterResource invalid key")
	})

	t.Run("should reject whitespace-only local resource ID", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.WhitespaceLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "LocalResourceId cannot be empty")
	})

	t.Run("should reject empty resource type", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.EmptyResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "ResourceType cannot be empty")
	})

	t.Run("should reject whitespace-only resource type", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.WhitespaceResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "ResourceType cannot be empty")
	})

	t.Run("should reject empty reporter type", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.EmptyReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "ReportedByReporterType cannot be empty")
	})

	t.Run("should reject whitespace-only reporter type", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.WhitespaceReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "ReportedByReporterType cannot be empty")
	})

	t.Run("should reject empty reporter instance ID", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.EmptyReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "ReporterInstanceId cannot be empty")
	})

	t.Run("should reject whitespace-only reporter instance ID", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.WhitespaceReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "ReporterInstanceId cannot be empty")
	})

	t.Run("should reject nil resource ID", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.NilResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "ResourceId cannot be empty")
	})

	t.Run("should reject empty API href", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.EmptyApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "ApiHref cannot be empty")
	})

	t.Run("should reject whitespace-only API href", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.WhitespaceApiHref,
			fixture.ValidConsoleHref,
		)

		assertInvalidReporterResource(t, reporterResource, err, "ApiHref cannot be empty")
	})

	t.Run("should accept local resource ID in UUID format", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceIdUUID,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		assertValidReporterResource(t, reporterResource, err, "UUID format local resource ID")
	})

	t.Run("should accept local resource ID in string format", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceIdString,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
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
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		id := reporterResource.Id()
		if id.UUID() != fixture.ValidId {
			t.Errorf("Expected ID %v, got %v", fixture.ValidId, id.UUID())
		}
	})
}

func TestReporterResource_Key(t *testing.T) {
	t.Parallel()
	fixture := NewReporterResourceTestFixture()

	t.Run("should return the correct reporter resource key", func(t *testing.T) {
		t.Parallel()

		reporterResource, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		key := reporterResource.Key()
		if key.LocalResourceId() != fixture.ValidLocalResourceId {
			t.Errorf("Expected LocalResourceId %s, got %s", fixture.ValidLocalResourceId, key.LocalResourceId())
		}
		if key.ResourceType() != fixture.ValidResourceType {
			t.Errorf("Expected ResourceType %s, got %s", fixture.ValidResourceType, key.ResourceType())
		}
		if key.ReporterType() != fixture.ValidReporterType {
			t.Errorf("Expected ReporterType %s, got %s", fixture.ValidReporterType, key.ReporterType())
		}
		if key.ReporterInstanceId() != fixture.ValidReporterInstanceId {
			t.Errorf("Expected ReporterInstanceId %s, got %s", fixture.ValidReporterInstanceId, key.ReporterInstanceId())
		}
	})
}

func TestReporterResource_Update(t *testing.T) {
	t.Parallel()
	fixture := NewReporterResourceTestFixture()

	t.Run("should update apiHref and consoleHref successfully", func(t *testing.T) {
		t.Parallel()

		original, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		newApiHref := "https://api.example.com/updated"
		newConsoleHref := "https://console.example.com/updated"

		updated, err := original.Update(newApiHref, newConsoleHref)

		if err != nil {
			t.Fatalf("Expected no error updating ReporterResource, got %v", err)
		}

		if updated.apiHref.String() != newApiHref {
			t.Errorf("Expected updated apiHref %s, got %s", newApiHref, updated.apiHref.String())
		}
		if updated.consoleHref.String() != newConsoleHref {
			t.Errorf("Expected updated consoleHref %s, got %s", newConsoleHref, updated.consoleHref.String())
		}
	})

	t.Run("should increment representation version", func(t *testing.T) {
		t.Parallel()

		original, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		originalVersion := original.representationVersion.Uint()

		updated, err := original.Update("https://api.example.com/updated", "https://console.example.com/updated")

		if err != nil {
			t.Fatalf("Expected no error updating ReporterResource, got %v", err)
		}

		expectedVersion := originalVersion + 1
		if updated.representationVersion.Uint() != expectedVersion {
			t.Errorf("Expected representation version %d, got %d", expectedVersion, updated.representationVersion.Uint())
		}
	})

	t.Run("should preserve other fields unchanged", func(t *testing.T) {
		t.Parallel()

		original, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		updated, err := original.Update("https://api.example.com/updated", "https://console.example.com/updated")

		if err != nil {
			t.Fatalf("Expected no error updating ReporterResource, got %v", err)
		}

		if updated.id != original.id {
			t.Errorf("Expected ID to remain unchanged")
		}
		if updated.ReporterResourceKey != original.ReporterResourceKey {
			t.Errorf("Expected ReporterResourceKey to remain unchanged")
		}
		if updated.resourceID != original.resourceID {
			t.Errorf("Expected resourceID to remain unchanged")
		}
		if updated.generation != original.generation {
			t.Errorf("Expected generation to remain unchanged")
		}
		if updated.tombstone != original.tombstone {
			t.Errorf("Expected tombstone to remain unchanged")
		}
	})

	t.Run("should handle empty consoleHref", func(t *testing.T) {
		t.Parallel()

		original, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		newApiHref := "https://api.example.com/updated"
		emptyConsoleHref := ""

		updated, err := original.Update(newApiHref, emptyConsoleHref)

		if err != nil {
			t.Fatalf("Expected no error updating ReporterResource, got %v", err)
		}

		if updated.apiHref.String() != newApiHref {
			t.Errorf("Expected updated apiHref %s, got %s", newApiHref, updated.apiHref.String())
		}
		if updated.consoleHref.String() != "" {
			t.Errorf("Expected empty consoleHref, got %s", updated.consoleHref.String())
		}
	})

	t.Run("should return error for invalid apiHref", func(t *testing.T) {
		t.Parallel()

		original, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		invalidApiHref := ""

		_, err = original.Update(invalidApiHref, "https://console.example.com/updated")

		if err == nil {
			t.Error("Expected error for invalid apiHref, got none")
		}
		if !strings.Contains(err.Error(), "API href") {
			t.Errorf("Expected error about API href, got %v", err)
		}
	})

	t.Run("should return error for invalid consoleHref", func(t *testing.T) {
		t.Parallel()

		original, err := NewReporterResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
		)

		if err != nil {
			t.Fatalf("Expected no error creating ReporterResource, got %v", err)
		}

		invalidConsoleHref := "   "

		_, err = original.Update("https://api.example.com/updated", invalidConsoleHref)

		if err == nil {
			t.Error("Expected error for invalid consoleHref, got none")
		}
		if !strings.Contains(err.Error(), "console href") {
			t.Errorf("Expected error about console href, got %v", err)
		}
	})
}
