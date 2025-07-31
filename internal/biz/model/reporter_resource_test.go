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

		assertInvalidReporterResource(t, reporterResource, err, "ReporterResourceId cannot be empty")
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

		assertInvalidReporterResource(t, reporterResource, err, "LocalResourceId cannot be empty")
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

		assertInvalidReporterResource(t, reporterResource, err, "ReportedByReporterInstance cannot be empty")
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

		assertInvalidReporterResource(t, reporterResource, err, "ReportedByReporterInstance cannot be empty")
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
