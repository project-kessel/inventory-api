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
// - URL validation and href handling
// - Tombstone logic and resource lifecycle
// - Versioning and generation management

func TestReporterRepresentation_Validation(t *testing.T) {
	t.Parallel()

	t.Run("valid ReporterRepresentation with all required fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

		// Factory method should create a valid instance without errors
		if rr == nil {
			t.Error("Valid ReporterRepresentation should not be nil")
		}
	})

	t.Run("ReporterRepresentation with empty LocalResourceID should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ReporterRepresentationWithLocalResourceID("")

		AssertValidationError(t, ValidateReporterRepresentation(rr), "LocalResourceID", "ReporterRepresentation with empty LocalResourceID should be invalid")
	})

	t.Run("ReporterRepresentation with empty ReporterType should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.ReporterType = ""

		AssertValidationError(t, ValidateReporterRepresentation(rr), "ReporterType", "ReporterRepresentation with empty ReporterType should be invalid")
	})

	t.Run("ReporterRepresentation with empty ResourceType should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.ResourceType = ""

		AssertValidationError(t, ValidateReporterRepresentation(rr), "ResourceType", "ReporterRepresentation with empty ResourceType should be invalid")
	})

	t.Run("ReporterRepresentation with field length constraints", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)

		testCases := []struct {
			name  string
			field string
			value string
			valid bool
		}{
			{"LocalResourceID too long", "LocalResourceID", "a" + fmt.Sprintf("%0128s", ""), false},
			{"ReporterType too long", "ReporterType", "a" + fmt.Sprintf("%0128s", ""), false},
			{"ResourceType too long", "ResourceType", "a" + fmt.Sprintf("%0128s", ""), false},
			{"ReporterInstanceID too long", "ReporterInstanceID", "a" + fmt.Sprintf("%0128s", ""), false},
			{"APIHref too long", "APIHref", "https://example.com/" + fmt.Sprintf("%0500s", ""), false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				rr := fixture.ValidReporterRepresentation()

				switch tc.field {
				case "LocalResourceID":
					rr.LocalResourceID = tc.value
				case "ReporterType":
					rr.ReporterType = tc.value
				case "ResourceType":
					rr.ResourceType = tc.value
				case "ReporterInstanceID":
					rr.ReporterInstanceID = tc.value
				case "APIHref":
					rr.APIHref = tc.value
				}

				err := ValidateReporterRepresentation(rr)
				if tc.valid {
					AssertNoError(t, err, fmt.Sprintf("%s should be valid", tc.name))
				} else {
					AssertValidationError(t, err, tc.field, fmt.Sprintf("%s should be invalid", tc.name))
				}
			})
		}
	})

	t.Run("ReporterRepresentation with zero Generation should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.Generation = 0

		AssertValidationError(t, ValidateReporterRepresentation(rr), "Generation", "ReporterRepresentation with zero Generation should be invalid")
	})

	t.Run("ReporterRepresentation with zero Version should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.Version = 0

		AssertValidationError(t, ValidateReporterRepresentation(rr), "Version", "ReporterRepresentation with zero Version should be invalid")
	})

	t.Run("ReporterRepresentation with zero CommonVersion should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.CommonVersion = 0

		AssertValidationError(t, ValidateReporterRepresentation(rr), "CommonVersion", "ReporterRepresentation with zero CommonVersion should be invalid")
	})

	t.Run("ReporterRepresentation with empty APIHref should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.APIHref = ""

		AssertValidationError(t, ValidateReporterRepresentation(rr), "APIHref", "ReporterRepresentation with empty APIHref should be invalid")
	})

	t.Run("ReporterRepresentation with invalid APIHref should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.APIHref = "invalid-url"

		AssertValidationError(t, ValidateReporterRepresentation(rr), "APIHref", "ReporterRepresentation with invalid APIHref should be invalid")
	})

	t.Run("ReporterRepresentation with nil Data should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.Data = nil

		AssertValidationError(t, ValidateReporterRepresentation(rr), "Data", "ReporterRepresentation with nil Data should be invalid")
	})
}

func TestReporterRepresentation_BusinessRules(t *testing.T) {
	t.Parallel()

	t.Run("should enforce unique constraint across all key fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr1 := fixture.ValidReporterRepresentation()
		rr2 := fixture.ValidReporterRepresentation()

		// Make them identical across all unique constraint fields
		rr2.LocalResourceID = rr1.LocalResourceID
		rr2.ReporterType = rr1.ReporterType
		rr2.ResourceType = rr1.ResourceType
		rr2.Version = rr1.Version
		rr2.ReporterInstanceID = rr1.ReporterInstanceID
		rr2.Generation = rr1.Generation

		// Both should be valid individually
		AssertNoError(t, ValidateReporterRepresentation(rr1), "First ReporterRepresentation should be valid")
		AssertNoError(t, ValidateReporterRepresentation(rr2), "Second ReporterRepresentation should be valid")

		// They should be considered duplicates
		if !areReporterRepresentationsDuplicates(*rr1, *rr2) {
			t.Error("ReporterRepresentations with identical unique constraint fields should be considered duplicates")
		}
	})

	t.Run("should allow different representations when unique constraint fields differ", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr1 := fixture.ValidReporterRepresentation()
		rr2 := fixture.ValidReporterRepresentation()

		// Change one field in the unique constraint
		rr2.Generation = rr1.Generation + 1

		// Both should be valid
		AssertNoError(t, ValidateReporterRepresentation(rr1), "First ReporterRepresentation should be valid")
		AssertNoError(t, ValidateReporterRepresentation(rr2), "Second ReporterRepresentation should be valid")

		// They should not be considered duplicates
		if areReporterRepresentationsDuplicates(*rr1, *rr2) {
			t.Error("ReporterRepresentations with different unique constraint fields should not be considered duplicates")
		}
	})

	t.Run("should enforce positive values for numeric fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)

		// Test Generation
		rr := fixture.ValidReporterRepresentation()
		rr.Generation = 1
		AssertNoError(t, ValidateReporterRepresentation(rr), "Positive Generation should be valid")

		// Test Version
		rr = fixture.ValidReporterRepresentation()
		rr.Version = 1
		AssertNoError(t, ValidateReporterRepresentation(rr), "Positive Version should be valid")

		// Test CommonVersion
		rr = fixture.ValidReporterRepresentation()
		rr.CommonVersion = 1
		AssertNoError(t, ValidateReporterRepresentation(rr), "Positive CommonVersion should be valid")
	})

	t.Run("should require non-empty string fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)

		requiredStringFields := []string{
			"LocalResourceID", "ReporterType", "ResourceType",
			"ReporterInstanceID", "APIHref",
		}

		for _, fieldName := range requiredStringFields {
			t.Run(fmt.Sprintf("%s_required", fieldName), func(t *testing.T) {
				t.Parallel()
				rr := fixture.ValidReporterRepresentation()

				switch fieldName {
				case "LocalResourceID":
					rr.LocalResourceID = ""
				case "ReporterType":
					rr.ReporterType = ""
				case "ResourceType":
					rr.ResourceType = ""
				case "ReporterInstanceID":
					rr.ReporterInstanceID = ""
				case "APIHref":
					rr.APIHref = ""
				}

				AssertValidationError(t, ValidateReporterRepresentation(rr), fieldName, fmt.Sprintf("%s should be required", fieldName))
			})
		}
	})
}

func TestReporterRepresentation_TombstoneLogic(t *testing.T) {
	t.Parallel()

	t.Run("should handle tombstone true", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ReporterRepresentationWithTombstone(true)

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with tombstone=true should be valid")

		if !rr.Tombstone {
			t.Error("Tombstone should be true")
		}
	})

	t.Run("should handle tombstone false", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ReporterRepresentationWithTombstone(false)

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with tombstone=false should be valid")

		if rr.Tombstone {
			t.Error("Tombstone should be false")
		}
	})

	t.Run("should default to false when not specified", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

		// Default should be false
		if rr.Tombstone {
			t.Error("Default tombstone value should be false")
		}
	})
}

func TestReporterRepresentation_VersioningLogic(t *testing.T) {
	t.Parallel()

	t.Run("should handle version increments", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

		// Test different version values
		versions := []uint{1, 2, 10, 100, 1000}
		for _, version := range versions {
			rr.Version = version
			AssertNoError(t, ValidateReporterRepresentation(rr), fmt.Sprintf("Version %d should be valid", version))
		}
	})

	t.Run("should handle generation increments", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

		// Test different generation values
		generations := []uint{1, 2, 10, 100, 1000}
		for _, generation := range generations {
			rr.Generation = generation
			AssertNoError(t, ValidateReporterRepresentation(rr), fmt.Sprintf("Generation %d should be valid", generation))
		}
	})

	t.Run("should handle common version increments", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

		// Test different common version values
		commonVersions := []uint{1, 2, 10, 100, 1000}
		for _, commonVersion := range commonVersions {
			rr.CommonVersion = commonVersion
			AssertNoError(t, ValidateReporterRepresentation(rr), fmt.Sprintf("CommonVersion %d should be valid", commonVersion))
		}
	})
}

func TestReporterRepresentation_HrefValidation(t *testing.T) {
	t.Parallel()

	t.Run("should validate APIHref URL format", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)

		validURLs := []string{
			"https://api.example.com/resource/123",
			"http://localhost:8080/api/v1/resource",
			"https://api.redhat.com/api/inventory/v1/hosts/123",
		}

		for _, url := range validURLs {
			t.Run(fmt.Sprintf("valid_url_%s", url), func(t *testing.T) {
				t.Parallel()
				rr := fixture.ReporterRepresentationWithAPIHref(url)
				AssertNoError(t, ValidateReporterRepresentation(rr), fmt.Sprintf("URL %s should be valid", url))
			})
		}
	})

	t.Run("should reject invalid APIHref URLs", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)

		invalidURLs := []string{
			"not-a-url",
			"ftp://example.com/resource",
			"",
			"://missing-scheme",
		}

		for _, url := range invalidURLs {
			t.Run(fmt.Sprintf("invalid_url_%s", url), func(t *testing.T) {
				t.Parallel()
				rr := fixture.ReporterRepresentationWithAPIHref(url)
				AssertValidationError(t, ValidateReporterRepresentation(rr), "APIHref", fmt.Sprintf("URL %s should be invalid", url))
			})
		}
	})

	t.Run("should validate ConsoleHref when provided", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)

		validURLs := []string{
			"https://console.redhat.com/insights/inventory/123",
			"https://console.example.com/resource/456",
		}

		for _, url := range validURLs {
			t.Run(fmt.Sprintf("valid_console_url_%s", url), func(t *testing.T) {
				t.Parallel()
				rr := fixture.ReporterRepresentationWithConsoleHref(url)
				AssertNoError(t, ValidateReporterRepresentation(rr), fmt.Sprintf("Console URL %s should be valid", url))
			})
		}
	})

	t.Run("should allow nil ConsoleHref", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ReporterRepresentationWithNilConsoleHref()

		AssertNoError(t, ValidateReporterRepresentation(rr), "Nil ConsoleHref should be valid")

		if rr.ConsoleHref != nil {
			t.Error("ConsoleHref should be nil")
		}
	})

	t.Run("should reject invalid ConsoleHref URLs", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)

		invalidURLs := []string{
			"not-a-url",
			"ftp://example.com/resource",
			"://missing-scheme",
		}

		for _, url := range invalidURLs {
			t.Run(fmt.Sprintf("invalid_console_url_%s", url), func(t *testing.T) {
				t.Parallel()
				rr := fixture.ReporterRepresentationWithConsoleHref(url)
				AssertValidationError(t, ValidateReporterRepresentation(rr), "ConsoleHref", fmt.Sprintf("Console URL %s should be invalid", url))
			})
		}
	})
}

func TestReporterRepresentation_DataHandling(t *testing.T) {
	t.Parallel()

	t.Run("should handle valid JSON data", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

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

		rr.Data = testData

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with valid JSON data should be valid")
	})

	t.Run("should handle complex nested JSON", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
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

		rr := fixture.ValidReporterRepresentation()
		rr.Data = complexData

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with complex nested JSON should be valid")
	})

	t.Run("should handle empty JSON object", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.Data = JsonObject{}

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with empty JSON object should be valid")
	})
}

func TestReporterRepresentation_FactoryMethods(t *testing.T) {
	t.Run("should_create_valid_ReporterRepresentation_using_factory", func(t *testing.T) {
		rr, err := NewReporterRepresentation(
			JsonObject{"satellite_id": "test-satellite"},
			"local-123",
			"hbi",
			"host",
			1,
			"reporter-instance-123",
			1,
			"https://api.example.com/resource/123",
			stringPtr("https://console.example.com/resource/123"),
			1,
			false,
			stringPtr("1.0.0"),
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if rr.LocalResourceID != "local-123" {
			t.Errorf("Expected LocalResourceID 'local-123', got '%s'", rr.LocalResourceID)
		}

		if rr.ReporterType != "hbi" {
			t.Errorf("Expected ReporterType 'hbi', got '%s'", rr.ReporterType)
		}

		if rr.Tombstone != false {
			t.Errorf("Expected Tombstone false, got %v", rr.Tombstone)
		}
	})

	t.Run("should_enforce_validation_rules_in_factory", func(t *testing.T) {
		// Test empty LocalResourceID
		_, err := NewReporterRepresentation(
			JsonObject{"satellite_id": "test-satellite"},
			"", // empty LocalResourceID
			"hbi",
			"host",
			1,
			"reporter-instance-123",
			1,
			"https://api.example.com/resource/123",
			stringPtr("https://console.example.com/resource/123"),
			1,
			false,
			stringPtr("1.0.0"),
		)

		if err == nil {
			t.Error("Expected validation error for empty LocalResourceID")
		}

		// Test zero Generation
		_, err = NewReporterRepresentation(
			JsonObject{"satellite_id": "test-satellite"},
			"local-123",
			"hbi",
			"host",
			1,
			"reporter-instance-123",
			0, // zero Generation
			"https://api.example.com/resource/123",
			stringPtr("https://console.example.com/resource/123"),
			1,
			false,
			stringPtr("1.0.0"),
		)

		if err == nil {
			t.Error("Expected validation error for zero Generation")
		}

		// Test invalid URL
		_, err = NewReporterRepresentation(
			JsonObject{"satellite_id": "test-satellite"},
			"local-123",
			"hbi",
			"host",
			1,
			"reporter-instance-123",
			1,
			"invalid-url", // invalid URL
			stringPtr("https://console.example.com/resource/123"),
			1,
			false,
			stringPtr("1.0.0"),
		)

		if err == nil {
			t.Error("Expected validation error for invalid URL")
		}
	})
}

// Helper function to check if two ReporterRepresentations are duplicates
// based on their unique constraint fields
func areReporterRepresentationsDuplicates(rr1, rr2 ReporterRepresentation) bool {
	return rr1.LocalResourceID == rr2.LocalResourceID &&
		rr1.ReporterType == rr2.ReporterType &&
		rr1.ResourceType == rr2.ResourceType &&
		rr1.Version == rr2.Version &&
		rr1.ReporterInstanceID == rr2.ReporterInstanceID &&
		rr1.Generation == rr2.Generation
}
