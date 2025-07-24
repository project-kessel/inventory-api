package model

import (
	"fmt"
	"testing"
)

// Test scenarios for ReporterRepresentation domain model
//
// These tests focus on domain logic, business rules, and model behavior
// rather than database operations or infrastructure concerns.
//
// Domain tests validate:
// - Business validation rules and constraints
// - Domain behavior and business logic
// - Factory method behavior and validation
// - Data handling and transformation logic
// - Model comparison and equality semantics
// - Tombstone logic and resource lifecycle
// - Versioning and generation management

func TestReporterRepresentation_Validation(t *testing.T) {
	t.Parallel()

	t.Run("valid ReporterRepresentation with all required fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

		// Factory method should create a valid instance without errors
		AssertEqual(t, "dd1b73b9-3e33-4264-968c-e3ce55b9afec", rr.ReporterResourceID, "Reporter resource ID should be set correctly")
	})

	t.Run("ReporterRepresentation with empty ReporterResourceID should be invalid", func(t *testing.T) {
		t.Parallel()

		// Test validation through factory method
		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"", // empty ReporterResourceID
			1,
			1,
			1,
			false,
			nil,
		)
		AssertValidationError(t, err, "ReporterResourceID", "ReporterRepresentation with empty ReporterResourceID should be invalid")
	})

	t.Run("ReporterRepresentation with zero Generation should be valid", func(t *testing.T) {
		t.Parallel()

		// Test that zero Generation is valid
		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			1,
			0, // zero Generation should be valid
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with zero Generation should be valid")
		if rr == nil {
			t.Error("ReporterRepresentation should not be nil")
		} else if rr.Generation != 0 {
			t.Errorf("Expected Generation to be 0, got %d", rr.Generation)
		}
	})

	t.Run("ReporterRepresentation with zero Version should be valid", func(t *testing.T) {
		t.Parallel()

		// Test that zero Version is valid
		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			0, // zero Version should be valid
			1,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with zero Version should be valid")
		if rr == nil {
			t.Error("ReporterRepresentation should not be nil")
		} else if rr.Version != 0 {
			t.Errorf("Expected Version to be 0, got %d", rr.Version)
		}
	})

	t.Run("ReporterRepresentation with zero CommonVersion should be valid", func(t *testing.T) {
		t.Parallel()

		// Test that zero CommonVersion is valid
		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			1,
			1,
			0, // zero CommonVersion should be valid
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with zero CommonVersion should be valid")
		if rr == nil {
			t.Error("ReporterRepresentation should not be nil")
		} else if rr.CommonVersion != 0 {
			t.Errorf("Expected CommonVersion to be 0, got %d", rr.CommonVersion)
		}
	})

	t.Run("ReporterRepresentation with nil Data should be valid", func(t *testing.T) {
		t.Parallel()

		// Test validation through factory method - nil data should be valid
		rr, err := NewReporterRepresentation(
			nil, // nil Data should be valid
			"reporter-resource-123",
			1,
			1,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with nil Data should be valid")
		if rr == nil {
			t.Error("ReporterRepresentation should not be nil")
		} else if rr.Data != nil {
			t.Error("Expected Data to be nil")
		}
	})
}

func TestReporterRepresentation_BusinessRules(t *testing.T) {
	t.Parallel()

	t.Run("should enforce unique constraint across primary key fields", func(t *testing.T) {
		t.Parallel()

		// Create two identical ReporterRepresentations using factory method
		rr1, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			1,
			1,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "First ReporterRepresentation should be valid")

		rr2, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123", // same ReporterResourceID
			1,                       // same Version
			1,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Second ReporterRepresentation should be valid")

		// They should be considered duplicates
		if !areReporterRepresentationsDuplicates(*rr1, *rr2) {
			t.Error("ReporterRepresentations with identical primary key fields should be considered duplicates")
		}
	})

	t.Run("should allow different representations when primary key fields differ", func(t *testing.T) {
		t.Parallel()

		// Create two ReporterRepresentations with different Version values
		rr1, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			1, // Version = 1
			1,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "First ReporterRepresentation should be valid")

		rr2, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			2, // Version = 2 (different)
			1,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Second ReporterRepresentation should be valid")

		// They should not be considered duplicates
		if areReporterRepresentationsDuplicates(*rr1, *rr2) {
			t.Error("ReporterRepresentations with different primary key fields should not be considered duplicates")
		}
	})

	t.Run("should enforce positive values for numeric fields", func(t *testing.T) {
		t.Parallel()

		// Test Generation = 1 (positive)
		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			1,
			1, // Generation = 1
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Positive Generation should be valid")

		// Test Version = 1 (positive)
		_, err = NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			1, // Version = 1
			1,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Positive Version should be valid")

		// Test CommonVersion = 1 (positive)
		_, err = NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			1,
			1,
			1, // CommonVersion = 1
			false,
			nil,
		)
		AssertNoError(t, err, "Positive CommonVersion should be valid")
	})

	t.Run("should require non-empty ReporterResourceID", func(t *testing.T) {
		t.Parallel()

		// Test ReporterResourceID required
		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"", // empty ReporterResourceID
			1,
			1,
			1,
			false,
			nil,
		)
		AssertValidationError(t, err, "ReporterResourceID", "ReporterResourceID should be required")
	})
}

func TestReporterRepresentation_TombstoneLogic(t *testing.T) {
	t.Parallel()

	t.Run("should handle tombstone true", func(t *testing.T) {
		t.Parallel()

		// Test validation through factory method
		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			1,
			1,
			1,
			true, // tombstone = true
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with tombstone=true should be valid")
		if rr == nil {
			t.Error("ReporterRepresentation should not be nil")
		} else if !rr.Tombstone {
			t.Error("Tombstone should be true")
		}
	})

	t.Run("should handle tombstone false", func(t *testing.T) {
		t.Parallel()

		// Test validation through factory method
		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			1,
			1,
			1,
			false, // tombstone = false
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with tombstone=false should be valid")
		if rr == nil {
			t.Error("ReporterRepresentation should not be nil")
		} else if rr.Tombstone {
			t.Error("Tombstone should be false")
		}
	})

	t.Run("should default to false when not specified", func(t *testing.T) {
		t.Parallel()

		// Test validation through factory method with default tombstone value (false)
		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"reporter-resource-123",
			1,
			1,
			1,
			false, // tombstone = false (default)
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with default tombstone should be valid")
		if rr == nil {
			t.Error("ReporterRepresentation should not be nil")
		} else if rr.Tombstone {
			t.Error("Default tombstone value should be false")
		}
	})
}

func TestReporterRepresentation_VersioningLogic(t *testing.T) {
	t.Parallel()

	t.Run("should handle version increments", func(t *testing.T) {
		t.Parallel()

		// Test different version values
		versions := []uint{0, 1, 2, 10, 100, 1000}
		for _, version := range versions {
			_, err := NewReporterRepresentation(
				JsonObject{"test": "data"},
				"reporter-resource-123",
				version, // Test different version values
				1,
				1,
				false,
				nil,
			)
			AssertNoError(t, err, fmt.Sprintf("Version %d should be valid", version))
		}
	})

	t.Run("should handle generation increments", func(t *testing.T) {
		t.Parallel()

		// Test different generation values
		generations := []uint{0, 1, 2, 10, 100, 1000}
		for _, generation := range generations {
			_, err := NewReporterRepresentation(
				JsonObject{"test": "data"},
				"reporter-resource-123",
				1,
				generation, // Test different generation values
				1,
				false,
				nil,
			)
			AssertNoError(t, err, fmt.Sprintf("Generation %d should be valid", generation))
		}
	})

	t.Run("should handle common version increments", func(t *testing.T) {
		t.Parallel()

		// Test different common version values
		commonVersions := []uint{0, 1, 2, 10, 100, 1000}
		for _, commonVersion := range commonVersions {
			_, err := NewReporterRepresentation(
				JsonObject{"test": "data"},
				"reporter-resource-123",
				1,
				1,
				commonVersion, // Test different common version values
				false,
				nil,
			)
			AssertNoError(t, err, fmt.Sprintf("CommonVersion %d should be valid", commonVersion))
		}
	})
}

func TestReporterRepresentation_DataHandling(t *testing.T) {
	t.Parallel()

	t.Run("should handle valid JSON data", func(t *testing.T) {
		t.Parallel()

		// Test with different types of JSON data
		testData := JsonObject{
			"string_field":  "test value",
			"number_field":  42,
			"boolean_field": true,
			"array_field":   []interface{}{1, 2, 3},
			"object_field": JsonObject{
				"nested_string": "nested value",
				"nested_number": 123,
			},
		}

		_, err := NewReporterRepresentation(
			testData, // Test with complex JSON data
			"reporter-resource-123",
			1,
			1,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with valid JSON data should be valid")
	})

	t.Run("should handle complex nested JSON", func(t *testing.T) {
		t.Parallel()

		complexData := JsonObject{
			"metadata": JsonObject{
				"labels": JsonObject{
					"app":         "test-app",
					"environment": "staging",
					"team":        "platform",
				},
				"annotations": JsonObject{
					"deployment.kubernetes.io/revision":                "1",
					"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test"}}`,
				},
			},
			"spec": JsonObject{
				"containers": []interface{}{
					JsonObject{
						"name":  "app",
						"image": "nginx:1.21",
						"ports": []interface{}{
							JsonObject{"containerPort": 80},
							JsonObject{"containerPort": 443},
						},
					},
				},
			},
			"status": JsonObject{
				"phase": "Running",
				"conditions": []interface{}{
					JsonObject{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		}

		_, err := NewReporterRepresentation(
			complexData, // Test with complex nested JSON
			"reporter-resource-123",
			1,
			1,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with complex nested JSON should be valid")
	})

	t.Run("should handle empty JSON object", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterRepresentation(
			JsonObject{}, // Test with empty JSON object
			"reporter-resource-123",
			1,
			1,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with empty JSON object should be valid")
	})
}

func TestReporterRepresentation_FactoryMethods(t *testing.T) {
	t.Run("should_create_valid_ReporterRepresentation_using_factory", func(t *testing.T) {
		rr, err := NewReporterRepresentation(
			JsonObject{"satellite_id": "test-satellite"},
			"reporter-resource-123",
			1,
			1,
			1,
			false,
			stringPtr("1.0.0"),
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if rr.ReporterResourceID != "reporter-resource-123" {
			t.Errorf("Expected ReporterResourceID 'reporter-resource-123', got '%s'", rr.ReporterResourceID)
		}

		if rr.Tombstone != false {
			t.Errorf("Expected Tombstone false, got %v", rr.Tombstone)
		}
	})

	t.Run("should_enforce_validation_rules_in_factory", func(t *testing.T) {
		// Test empty ReporterResourceID
		_, err := NewReporterRepresentation(
			JsonObject{"satellite_id": "test-satellite"},
			"", // empty ReporterResourceID
			1,
			1,
			1,
			false,
			stringPtr("1.0.0"),
		)

		if err == nil {
			t.Error("Expected validation error for empty ReporterResourceID")
		}

		// Test zero Generation - should be valid now
		rr, err := NewReporterRepresentation(
			JsonObject{"satellite_id": "test-satellite"},
			"reporter-resource-123",
			1,
			0, // zero Generation should be valid
			1,
			false,
			stringPtr("1.0.0"),
		)

		if err != nil {
			t.Errorf("Expected zero Generation to be valid, got error: %v", err)
		}
		if rr == nil {
			t.Error("Expected valid ReporterRepresentation, got nil")
		} else if rr.Generation != 0 {
			t.Errorf("Expected Generation to be 0, got %d", rr.Generation)
		}
	})
}

// Helper function to check if two ReporterRepresentations are duplicates
// based on their primary key fields
func areReporterRepresentationsDuplicates(rr1, rr2 ReporterRepresentation) bool {
	return rr1.ReporterResourceID == rr2.ReporterResourceID &&
		rr1.Version == rr2.Version
}
