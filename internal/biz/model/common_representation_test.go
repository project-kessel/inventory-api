package model

import (
	"strings"
	"testing"
)

func assertValidCommonRepresentation(t *testing.T, commonRep CommonRepresentation, err error, testCase string) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error for %s, got %v", testCase, err)
	}
}

func assertInvalidCommonRepresentation(t *testing.T, commonRep CommonRepresentation, err error, expectedErrorSubstring string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
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
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		assertValidCommonRepresentation(t, commonRep, err, "valid inputs")
	})

	t.Run("should accept zero version", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ZeroVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		assertValidCommonRepresentation(t, commonRep, err, "zero version")
	})

	t.Run("should reject nil resource ID", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.NilResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation invalid resource ID")
	})

	t.Run("should reject nil data", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.NilRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation requires non-empty data")
	})

	t.Run("should reject empty data object", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.EmptyRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation requires non-empty data")
	})

	t.Run("should reject empty reporter type", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.EmptyReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation invalid reporter")
	})

	t.Run("should reject whitespace-only reporter type", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.WhitespaceReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation invalid reporter")
	})

	t.Run("should reject empty reporter instance", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.EmptyReporterInstanceIdType(),
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation invalid reporter")
	})

	t.Run("should reject whitespace-only reporter instance", func(t *testing.T) {
		t.Parallel()

		commonRep, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.WhitespaceReporterInstanceIdType(),
		)

		assertInvalidCommonRepresentation(t, commonRep, err, "CommonRepresentation invalid reporter")
	})
}
