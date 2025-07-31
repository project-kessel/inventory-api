package model

import (
	"strings"
	"testing"
)

func assertValidCommonRepresentation(t *testing.T, commonRep *CommonRepresentation, err error, testCase string) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error for %s, got %v", testCase, err)
	}
	if commonRep == nil {
		t.Errorf("Expected valid CommonRepresentation for %s, got nil", testCase)
	}
}

func assertInvalidCommonRepresentation(t *testing.T, commonRep *CommonRepresentation, err error, expectedErrorSubstring string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
	}
	if commonRep != nil {
		t.Error("Expected nil CommonRepresentation for invalid input, got non-nil")
	}
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error about %s, got %v", expectedErrorSubstring, err)
	}
}

func TestCommonRepresentation_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewCommonRepresentationTestFixture()

	t.Run("should create common representation with valid inputs", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceId,
			fixture.ValidData,
			fixture.ValidVersion,
			fixture.ValidReportedByReporterType,
			fixture.ValidReportedByReporterInstance,
		)

		assertValidCommonRepresentation(t, commonRep, err, "valid inputs")
	})

	t.Run("should accept zero version", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceId,
			fixture.ValidData,
			fixture.ZeroVersion,
			fixture.ValidReportedByReporterType,
			fixture.ValidReportedByReporterInstance,
		)

		assertValidCommonRepresentation(t, commonRep, err, "zero version")
	})

	t.Run("should reject nil resource ID", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.NilResourceId,
			fixture.ValidData,
			fixture.ValidVersion,
			fixture.ValidReportedByReporterType,
			fixture.ValidReportedByReporterInstance,
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation invalid resource ID")
	})

	t.Run("should reject nil data", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceId,
			fixture.NilData,
			fixture.ValidVersion,
			fixture.ValidReportedByReporterType,
			fixture.ValidReportedByReporterInstance,
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation requires non-empty data")
	})

	t.Run("should reject empty data object", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceId,
			fixture.EmptyData,
			fixture.ValidVersion,
			fixture.ValidReportedByReporterType,
			fixture.ValidReportedByReporterInstance,
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation requires non-empty data")
	})

	t.Run("should reject empty reporter type", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceId,
			fixture.ValidData,
			fixture.ValidVersion,
			fixture.EmptyReporterType,
			fixture.ValidReportedByReporterInstance,
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation invalid reporter")
	})

	t.Run("should reject whitespace-only reporter type", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceId,
			fixture.ValidData,
			fixture.ValidVersion,
			fixture.WhitespaceReporterType,
			fixture.ValidReportedByReporterInstance,
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation invalid reporter")
	})

	t.Run("should reject empty reporter instance", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceId,
			fixture.ValidData,
			fixture.ValidVersion,
			fixture.ValidReportedByReporterType,
			fixture.EmptyReporterInstance,
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation invalid reporter")
	})

	t.Run("should reject whitespace-only reporter instance", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceId,
			fixture.ValidData,
			fixture.ValidVersion,
			fixture.ValidReportedByReporterType,
			fixture.WhitespaceReporterInstance,
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation invalid reporter")
	})
}
