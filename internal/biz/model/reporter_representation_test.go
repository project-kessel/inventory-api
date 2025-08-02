package model

import (
	"strings"
	"testing"
)

func assertValidReporterDataRepresentation(t *testing.T, dataRep ReporterDataRepresentation, err error, testCase string) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error for %s, got %v", testCase, err)
	}
	if dataRep == nil {
		t.Errorf("Expected valid ReporterDataRepresentation for %s, got nil", testCase)
		return
	}
	if dataRep.(ReporterRepresentation).IsTombstone() {
		t.Errorf("Expected ReporterDataRepresentation tombstone to be false for %s, got true", testCase)
	}
	if dataRep.Data() == nil {
		t.Errorf("Expected ReporterDataRepresentation to have data for %s, got nil", testCase)
	}
}

func assertValidReporterDeleteRepresentation(t *testing.T, deleteRep ReporterDeleteRepresentation, err error, testCase string) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error for %s, got %v", testCase, err)
	}
	if deleteRep == nil {
		t.Errorf("Expected valid ReporterDeleteRepresentation for %s, got nil", testCase)
		return
	}
	if rep, ok := deleteRep.(ReporterRepresentation); ok {
		if !rep.IsTombstone() {
			t.Errorf("Expected ReporterDeleteRepresentation tombstone to be true for %s, got false", testCase)
		}
		if rep.Data() != nil {
			t.Errorf("Expected ReporterDeleteRepresentation to have nil data for %s, got non-nil", testCase)
		}
	}
}

func assertInvalidReporterDataRepresentation(t *testing.T, dataRep ReporterDataRepresentation, err error, expectedErrorSubstring string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
	}
	if dataRep != nil {
		t.Error("Expected nil ReporterDataRepresentation for invalid input, got non-nil")
	}
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error about %s, got %v", expectedErrorSubstring, err)
	}
}

func assertInvalidReporterDeleteRepresentation(t *testing.T, deleteRep ReporterDeleteRepresentation, err error, expectedErrorSubstring string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
	}
	if deleteRep != nil {
		t.Error("Expected nil ReporterDeleteRepresentation for invalid input, got non-nil")
	}
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error about %s, got %v", expectedErrorSubstring, err)
	}
}

func TestReporterDataRepresentation_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewReporterRepresentationTestFixture()

	t.Run("should create data representation with valid inputs", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		assertValidReporterDataRepresentation(t, dataRep, err, "valid inputs")
	})

	t.Run("should create data representation with nil reporter version", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidData,
			fixture.ValidCommonVersion,
			fixture.NilReporterVersion,
		)

		assertValidReporterDataRepresentation(t, dataRep, err, "nil reporter version")
	})

	t.Run("should reject data representation with nil data", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.NilData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		assertInvalidReporterDataRepresentation(t, dataRep, err, "ReporterDataRepresentation requires non-empty data")
	})

	t.Run("should reject data representation with empty data", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.EmptyData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		assertInvalidReporterDataRepresentation(t, dataRep, err, "ReporterDataRepresentation requires non-empty data")
	})

	t.Run("should reject data representation with empty reporter resource ID", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.EmptyReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		assertInvalidReporterDataRepresentation(t, dataRep, err, "ReporterResourceId cannot be empty")
	})

	t.Run("should reject data representation with whitespace-only reporter resource ID", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.WhitespaceReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		assertInvalidReporterDataRepresentation(t, dataRep, err, "ReporterResourceId cannot be empty")
	})

	t.Run("should reject data representation with invalid reporter resource ID format", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.InvalidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		assertInvalidReporterDataRepresentation(t, dataRep, err, "invalid UUID")
	})
}

func TestReporterDeleteRepresentation_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewReporterRepresentationTestFixture()

	t.Run("should create delete representation with valid inputs", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		assertValidReporterDeleteRepresentation(t, deleteRep, err, "valid inputs")
	})

	t.Run("should create delete representation with nil reporter version", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidCommonVersion,
			fixture.NilReporterVersion,
		)

		assertValidReporterDeleteRepresentation(t, deleteRep, err, "nil reporter version")
	})

	t.Run("should reject delete representation with empty reporter resource ID", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.EmptyReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		assertInvalidReporterDeleteRepresentation(t, deleteRep, err, "ReporterResourceId cannot be empty")
	})

	t.Run("should reject delete representation with whitespace-only reporter resource ID", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.WhitespaceReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		assertInvalidReporterDeleteRepresentation(t, deleteRep, err, "ReporterResourceId cannot be empty")
	})

	t.Run("should reject delete representation with invalid reporter resource ID format", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.InvalidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		assertInvalidReporterDeleteRepresentation(t, deleteRep, err, "invalid UUID")
	})
}

func TestReporterRepresentation_BusinessRules(t *testing.T) {
	t.Parallel()
	fixture := NewReporterRepresentationTestFixture()

	t.Run("should enforce tombstone false for data representation", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if dataRep.(ReporterRepresentation).IsTombstone() {
			t.Error("Expected ReporterDataRepresentation to have tombstone=false, got true")
		}
	})

	t.Run("should enforce tombstone true for delete representation", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if rep, ok := deleteRep.(ReporterRepresentation); ok {
			if !rep.IsTombstone() {
				t.Error("Expected ReporterDeleteRepresentation to have tombstone=true, got false")
			}
		} else {
			t.Error("Expected ReporterDeleteRepresentation to be castable to ReporterRepresentation")
		}
	})

	t.Run("should enforce nil data for delete representation", func(t *testing.T) {
		t.Parallel()

		deleteRep, err := NewReporterDeleteRepresentation(
			fixture.ValidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if rep, ok := deleteRep.(ReporterRepresentation); ok {
			if rep.Data() != nil {
				t.Error("Expected ReporterDeleteRepresentation to have nil data, got non-nil")
			}
		} else {
			t.Error("Expected ReporterDeleteRepresentation to be castable to ReporterRepresentation")
		}
	})

	t.Run("should enforce non-nil data for data representation", func(t *testing.T) {
		t.Parallel()

		dataRep, err := NewReporterDataRepresentation(
			fixture.ValidReporterResourceId,
			fixture.ValidVersion,
			fixture.ValidGeneration,
			fixture.ValidData,
			fixture.ValidCommonVersion,
			fixture.ValidReporterVersion,
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
			fixture.ValidReporterResourceId,
			0, // zero version
			0, // zero generation
			fixture.ValidData,
			0, // zero common version
			fixture.ValidReporterVersion,
		)

		if err != nil {
			t.Errorf("Expected no error with zero values, got %v", err)
		}
		if dataRep == nil {
			t.Error("Expected valid ReporterDataRepresentation with zero values, got nil")
		}
	})
}
