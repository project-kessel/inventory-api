package model

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/errors"
)

func TestCommonRepresentation_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewCommonRepresentationTestFixture()

	t.Run("should create common representation with valid inputs", func(t *testing.T) {
		t.Parallel()

		_, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		if err != nil {
			t.Errorf("Expected no error for valid inputs, got %v", err)
		}
	})

	t.Run("should accept zero version", func(t *testing.T) {
		t.Parallel()

		_, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ZeroVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		if err != nil {
			t.Errorf("Expected no error for zero version, got %v", err)
		}
	})

	t.Run("should reject nil resource ID", func(t *testing.T) {
		t.Parallel()

		_, err := NewCommonRepresentation(
			fixture.NilResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		errors.AssertIs(t, err, ErrInvalidUUID)
	})

	t.Run("should reject nil data", func(t *testing.T) {
		t.Parallel()

		_, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.NilRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		errors.AssertIs(t, err, ErrInvalidData)
	})

	t.Run("should reject empty data object", func(t *testing.T) {
		t.Parallel()

		_, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.EmptyRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		errors.AssertIs(t, err, ErrInvalidData)
	})

	t.Run("should reject empty reporter type", func(t *testing.T) {
		t.Parallel()

		_, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.EmptyReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		errors.AssertIs(t, err, ErrEmpty)
	})

	t.Run("should reject whitespace-only reporter type", func(t *testing.T) {
		t.Parallel()

		_, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.WhitespaceReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)

		errors.AssertIs(t, err, ErrEmpty)
	})

	t.Run("should reject empty reporter instance", func(t *testing.T) {
		t.Parallel()

		_, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.EmptyReporterInstanceIdType(),
		)

		errors.AssertIs(t, err, ErrEmpty)
	})

	t.Run("should reject whitespace-only reporter instance", func(t *testing.T) {
		t.Parallel()

		_, err := NewCommonRepresentation(
			fixture.ValidResourceIdType(),
			fixture.ValidRepresentationType(),
			fixture.ValidVersionType(),
			fixture.ValidReporterTypeType(),
			fixture.WhitespaceReporterInstanceIdType(),
		)

		errors.AssertIs(t, err, ErrEmpty)
	})
}
