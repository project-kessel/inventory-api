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
			fixture.ValidResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidReporterDataType(),
			fixture.ValidReporterResourceIdType(),
			fixture.ValidReporterVersionType(),
			fixture.ValidReporterGenerationType(),
			fixture.ValidCommonDataType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionStrType(),
		)

		assertValidResourceEvent(t, event, err, "valid inputs")
	})

	t.Run("should create resource event with nil reporter version", func(t *testing.T) {
		t.Parallel()

		event, err := NewResourceEvent(
			fixture.ValidResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidReporterDataType(),
			fixture.ValidReporterResourceIdType(),
			fixture.ValidReporterVersionType(),
			fixture.ValidReporterGenerationType(),
			fixture.ValidCommonDataType(),
			fixture.ValidCommonVersionType(),
			fixture.NilReporterVersionStrType(),
		)

		assertValidResourceEvent(t, event, err, "nil reporter version")
	})

	t.Run("should create resource event with different values", func(t *testing.T) {
		t.Parallel()

		event, err := NewResourceEvent(
			fixture.AnotherResourceIdType(),
			fixture.AnotherResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.AnotherReporterDataType(),
			fixture.ValidReporterResourceIdType(),
			fixture.ValidReporterVersionType(),
			fixture.ValidReporterGenerationType(),
			fixture.AnotherCommonDataType(),
			fixture.ValidCommonVersionType(),
			fixture.NilReporterVersionStrType(),
		)

		assertValidResourceEvent(t, event, err, "different values")
	})

	// All tiny type validation tests have been moved to common_test.go where they belong.
	// ResourceEvent tests should only test business logic with valid tiny types.
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
