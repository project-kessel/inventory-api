package model

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/errors"
)

func assertValidReporterDataRepresentation(t *testing.T, dataRep ReporterDataRepresentation, err error, testCase string) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error for %s, got %v", testCase, err)
	}
	// Check if the struct is valid by checking if it has valid data
	if dataRep.Data() == nil {
		t.Errorf("Expected valid ReporterDataRepresentation for %s, got struct with nil data", testCase)
		return
	}
	if dataRep.IsTombstone() {
		t.Errorf("Expected ReporterDataRepresentation tombstone to be false for %s, got true", testCase)
	}
}

func assertValidReporterDeleteRepresentation(t *testing.T, deleteRep ReporterDeleteRepresentation, err error, testCase string) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error for %s, got %v", testCase, err)
	}
	// Check if the struct is valid by checking if it has the correct tombstone state
	if !deleteRep.IsTombstone() {
		t.Errorf("Expected ReporterDeleteRepresentation tombstone to be true for %s, got false", testCase)
		return
	}
	if deleteRep.Data() != nil {
		t.Errorf("Expected ReporterDeleteRepresentation to have nil data for %s, got non-nil", testCase)
	}
}

func assertInvalidReporterDataRepresentation(t *testing.T, dataRep ReporterDataRepresentation, err error, expectedSentinel error) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
		return
	}
	// For invalid input, the constructor should return an empty struct with nil data
	if dataRep.Data() != nil {
		t.Error("Expected ReporterDataRepresentation with nil data for invalid input, got non-nil data")
	}
	errors.AssertIs(t, err, expectedSentinel)
}

func assertInvalidReporterDeleteRepresentation(t *testing.T, deleteRep ReporterDeleteRepresentation, err error, expectedSentinel error) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
		return
	}
	// For invalid input, the constructor should return an empty struct with nil data
	if deleteRep.Data() != nil {
		t.Error("Expected ReporterDeleteRepresentation with nil data for invalid input, got non-nil data")
	}
	errors.AssertIs(t, err, expectedSentinel)
}

func TestReporterDataRepresentation_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewReporterRepresentationTestFixture()

	t.Run("should create data representation with valid inputs", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidRepresentationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		assertValidReporterDataRepresentation(t, dataRep, err, "valid inputs")
	})

	t.Run("should create data representation with nil reporter version", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidRepresentationType(),
			fixture.ValidCommonVersionType(),
			fixture.NilReporterVersionType(),
		)

		assertValidReporterDataRepresentation(t, dataRep, err, "nil reporter version")
	})

	t.Run("should reject data representation with nil data", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.NilRepresentationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		assertInvalidReporterDataRepresentation(t, dataRep, err, ErrInvalidData)
	})

	t.Run("should reject data representation with empty data", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.EmptyRepresentationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		assertInvalidReporterDataRepresentation(t, dataRep, err, ErrInvalidData)
	})

	t.Run("should reject data representation with empty reporter resource ID", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.EmptyReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidRepresentationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		assertInvalidReporterDataRepresentation(t, dataRep, err, ErrInvalidUUID)
	})

	t.Run("should reject data representation with whitespace-only reporter resource ID", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.WhitespaceReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidRepresentationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		assertInvalidReporterDataRepresentation(t, dataRep, err, ErrInvalidUUID)
	})

	t.Run("should reject data representation with invalid reporter resource ID format", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.InvalidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidRepresentationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		assertInvalidReporterDataRepresentation(t, dataRep, err, ErrInvalidUUID)
	})
}

func TestReporterDeleteRepresentation_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewReporterRepresentationTestFixture()

	t.Run("should create delete representation with valid inputs", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		assertValidReporterDeleteRepresentation(t, deleteRep, err, "valid inputs")
	})

	t.Run("should create delete representation with nil reporter version", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidCommonVersionType(),
			fixture.NilReporterVersionType(),
		)

		assertValidReporterDeleteRepresentation(t, deleteRep, err, "nil reporter version")
	})

	t.Run("should reject delete representation with empty reporter resource ID", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.EmptyReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		assertInvalidReporterDeleteRepresentation(t, deleteRep, err, ErrInvalidUUID)
	})

	t.Run("should reject delete representation with whitespace-only reporter resource ID", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.WhitespaceReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		assertInvalidReporterDeleteRepresentation(t, deleteRep, err, ErrInvalidUUID)
	})

	t.Run("should reject delete representation with invalid reporter resource ID format", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.InvalidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		assertInvalidReporterDeleteRepresentation(t, deleteRep, err, ErrInvalidUUID)
	})
}

func TestReporterRepresentation_BusinessRules(t *testing.T) {
	t.Parallel()
	fixture := NewReporterRepresentationTestFixture()

	t.Run("should enforce tombstone false for data representation", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidRepresentationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if dataRep.IsTombstone() {
			t.Error("Expected ReporterDataRepresentation to have tombstone=false, got true")
		}
	})

	t.Run("should enforce tombstone true for delete representation", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if !deleteRep.IsTombstone() {
			t.Error("Expected ReporterDeleteRepresentation to have tombstone=true, got false")
		}
	})

	t.Run("should enforce nil data for delete representation", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if deleteRep.Data() != nil {
			t.Error("Expected ReporterDeleteRepresentation to have nil data, got non-nil")
		}
	})

	t.Run("should enforce non-nil data for data representation", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceIdType(),
			fixture.ValidVersionType(),
			fixture.ValidGenerationType(),
			fixture.ValidRepresentationType(),
			fixture.ValidCommonVersionType(),
			fixture.ValidReporterVersionType(),
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if dataRep.Data() == nil {
			t.Error("Expected ReporterDataRepresentation to have non-nil data, got nil")
		}
		if len(dataRep.Data()) == 0 {
			t.Error("Expected ReporterDataRepresentation to have non-empty data, got empty")
		}
	})

	t.Run("should accept zero values for version and generation", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceIdType(),
			0, // zero version
			0, // zero generation
			fixture.ValidRepresentationType(),
			0, // zero common version
			fixture.ValidReporterVersionType(),
		)

		if err != nil {
			t.Errorf("Expected no error with zero values, got %v", err)
		}
		if dataRep.Data() == nil {
			t.Error("Expected valid ReporterDataRepresentation with zero values, got struct with nil data")
		}
	})
}
