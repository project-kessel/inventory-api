//go:build test

package model

import (
	"strings"
	"testing"
)

func TestResourceEvent_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewResourceEventTestFixture()

	t.Run("should create resource event with valid inputs", func(t *testing.T) {
		t.Parallel()

		event, err := NewResourceEvent(
			fixture.ValidResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidReporterData,
			fixture.ValidReporterResourceID,
			fixture.ValidReporterVersion,
			fixture.ValidReporterGeneration,
			fixture.ValidCommonData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersionStr,
		)

		assertValidResourceEvent(t, event, err, "valid inputs")
	})

	t.Run("should create resource event with nil reporter version", func(t *testing.T) {
		t.Parallel()

		event, err := NewResourceEvent(
			fixture.ValidResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidReporterData,
			fixture.ValidReporterResourceID,
			fixture.ValidReporterVersion,
			fixture.ValidReporterGeneration,
			fixture.ValidCommonData,
			fixture.ValidCommonVersion,
			fixture.NilReporterVersionStr,
		)

		assertValidResourceEvent(t, event, err, "nil reporter version")
	})

	t.Run("should create resource event with different values", func(t *testing.T) {
		t.Parallel()

		event, err := NewResourceEvent(
			fixture.AnotherResourceId,
			fixture.AnotherResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.AnotherReporterData,
			fixture.ValidReporterResourceID,
			fixture.ValidReporterVersion,
			fixture.ValidReporterGeneration,
			fixture.AnotherCommonData,
			fixture.ValidCommonVersion,
			fixture.NilReporterVersionStr,
		)

		assertValidResourceEvent(t, event, err, "different values")
	})

	t.Run("should reject invalid resource id", func(t *testing.T) {
		t.Parallel()

		_, err := NewResourceEvent(
			fixture.InvalidResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidReporterData,
			fixture.ValidReporterResourceID,
			fixture.ValidReporterVersion,
			fixture.ValidReporterGeneration,
			fixture.ValidCommonData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersionStr,
		)

		assertInvalidResourceEvent(t, err, "ResourceEvent invalid resource ID")
	})

	t.Run("should reject empty resource type", func(t *testing.T) {
		t.Parallel()

		_, err := NewResourceEvent(
			fixture.ValidResourceId,
			fixture.EmptyResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidReporterData,
			fixture.ValidReporterResourceID,
			fixture.ValidReporterVersion,
			fixture.ValidReporterGeneration,
			fixture.ValidCommonData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersionStr,
		)

		assertInvalidResourceEvent(t, err, "ResourceEvent invalid resource type")
	})

	t.Run("should reject whitespace-only resource type", func(t *testing.T) {
		t.Parallel()

		_, err := NewResourceEvent(
			fixture.ValidResourceId,
			fixture.WhitespaceResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidReporterData,
			fixture.ValidReporterResourceID,
			fixture.ValidReporterVersion,
			fixture.ValidReporterGeneration,
			fixture.ValidCommonData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersionStr,
		)

		assertInvalidResourceEvent(t, err, "ResourceEvent invalid resource type")
	})

	t.Run("should reject empty reporter type", func(t *testing.T) {
		t.Parallel()

		_, err := NewResourceEvent(
			fixture.ValidResourceId,
			fixture.ValidResourceType,
			fixture.EmptyReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidReporterData,
			fixture.ValidReporterResourceID,
			fixture.ValidReporterVersion,
			fixture.ValidReporterGeneration,
			fixture.ValidCommonData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersionStr,
		)

		assertInvalidResourceEvent(t, err, "ResourceEvent invalid reporter")
	})

	t.Run("should reject empty reporter instance id", func(t *testing.T) {
		t.Parallel()

		_, err := NewResourceEvent(
			fixture.ValidResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.EmptyReporterInstanceId,
			fixture.ValidReporterData,
			fixture.ValidReporterResourceID,
			fixture.ValidReporterVersion,
			fixture.ValidReporterGeneration,
			fixture.ValidCommonData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersionStr,
		)

		assertInvalidResourceEvent(t, err, "ResourceEvent invalid reporter")
	})

	t.Run("should reject empty reporter data", func(t *testing.T) {
		t.Parallel()

		_, err := NewResourceEvent(
			fixture.ValidResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.EmptyReporterData,
			fixture.ValidReporterResourceID,
			fixture.ValidReporterVersion,
			fixture.ValidReporterGeneration,
			fixture.ValidCommonData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersionStr,
		)

		assertInvalidResourceEvent(t, err, "ResourceEvent invalid reporter representation")
	})

	t.Run("should reject empty reporter resource id", func(t *testing.T) {
		t.Parallel()

		_, err := NewResourceEvent(
			fixture.ValidResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidReporterData,
			fixture.EmptyReporterResourceID,
			fixture.ValidReporterVersion,
			fixture.ValidReporterGeneration,
			fixture.ValidCommonData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersionStr,
		)

		assertInvalidResourceEvent(t, err, "ResourceEvent invalid reporter representation")
	})

	t.Run("should reject empty common data", func(t *testing.T) {
		t.Parallel()

		_, err := NewResourceEvent(
			fixture.ValidResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidReporterData,
			fixture.ValidReporterResourceID,
			fixture.ValidReporterVersion,
			fixture.ValidReporterGeneration,
			fixture.EmptyCommonData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersionStr,
		)

		assertInvalidResourceEvent(t, err, "ResourceEvent invalid common representation")
	})
}

func assertValidResourceEvent(t *testing.T, event ResourceEvent, err error, context string) {
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
	// Check that reporterId is properly initialized by checking its underlying fields
	if event.reporterId.reporterType.String() == "" {
		t.Errorf("Expected valid reporter type for %s", context)
	}
	if event.reporterId.reporterInstanceId.String() == "" {
		t.Errorf("Expected valid reporter instance ID for %s", context)
	}
}

func assertInvalidResourceEvent(t *testing.T, err error, expectedErrorSubstring string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
	}
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error containing %s, got %v", expectedErrorSubstring, err)
	}
}
