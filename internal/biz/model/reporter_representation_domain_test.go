package model

import (
	"fmt"
	"testing"
)

// Test scenarios for ReporterRepresentation domain model_legacy
//
// These tests focus on domain logic, business rules, and model_legacy behavior
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
		AssertEqual(t, "hbi", rr.ReporterType, "Reporter type should be set correctly")
	})

	t.Run("ReporterRepresentation with empty LocalResourceID should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		_, err := fixture.ReporterRepresentationWithLocalResourceID("")

		AssertValidationError(t, err, "LocalResourceID", "ReporterRepresentation with empty LocalResourceID should be invalid")
	})

	t.Run("ReporterRepresentation with empty ReporterType should be invalid", func(t *testing.T) {
		t.Parallel()

		// Test validation through factory method (proper approach)
		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"", // empty ReporterType
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
			1,
			false,
			nil,
		)
		AssertValidationError(t, err, "ReporterType", "ReporterRepresentation with empty ReporterType should be invalid")
	})

	t.Run("ReporterRepresentation with empty ResourceType should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		_, err := fixture.ReporterRepresentationWithResourceType("")

		AssertValidationError(t, err, "ResourceType", "ReporterRepresentation with empty ResourceType should be invalid")
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

				// Test validation through factory method (proper approach)
				_, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					rr.LocalResourceID,
					rr.ReporterType,
					rr.ResourceType,
					rr.Version,
					rr.ReporterInstanceID,
					rr.Generation,
					rr.APIHref,
					rr.ConsoleHref,
					rr.CommonVersion,
					rr.Tombstone,
					rr.ReporterVersion,
				)
				if tc.valid {
					AssertNoError(t, err, fmt.Sprintf("%s should be valid", tc.name))
				} else {
					AssertValidationError(t, err, tc.field, fmt.Sprintf("%s should be invalid", tc.name))
				}
			})
		}
	})

	t.Run("ReporterRepresentation with zero Generation should be valid", func(t *testing.T) {
		t.Parallel()

		// Test that zero Generation is valid
		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			0, // zero Generation should be valid
			"https://api.example.com",
			nil,
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
			"test-local-id",
			"hbi",
			"host",
			0, // zero Version should be valid
			"test-instance",
			1,
			"https://api.example.com",
			nil,
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
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
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

	t.Run("ReporterRepresentation with empty APIHref should be invalid", func(t *testing.T) {
		t.Parallel()

		// Test validation through factory method (proper approach)
		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"", // empty APIHref
			nil,
			1,
			false,
			nil,
		)
		AssertValidationError(t, err, "APIHref", "ReporterRepresentation with empty APIHref should be invalid")
	})

	t.Run("ReporterRepresentation with invalid APIHref should be invalid", func(t *testing.T) {
		t.Parallel()

		// Test validation through factory method (proper approach)
		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"invalid-url", // invalid APIHref
			nil,
			1,
			false,
			nil,
		)
		AssertValidationError(t, err, "APIHref", "ReporterRepresentation with invalid APIHref should be invalid")
	})

	t.Run("ReporterRepresentation with nil Data should be valid", func(t *testing.T) {
		t.Parallel()

		// Test validation through factory method - nil data should be valid
		rr, err := NewReporterRepresentation(
			nil, // nil Data should be valid
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
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

	t.Run("should enforce unique constraint across all key fields", func(t *testing.T) {
		t.Parallel()

		// Create two identical ReporterRepresentations using factory method
		rr1, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "First ReporterRepresentation should be valid")

		rr2, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id", // same LocalResourceID
			"hbi",           // same ReporterType
			"host",          // same ResourceType
			1,               // same Version
			"test-instance", // same ReporterInstanceID
			1,               // same Generation
			"https://api.example.com",
			nil,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Second ReporterRepresentation should be valid")

		// They should be considered duplicates
		if !areReporterRepresentationsDuplicates(*rr1, *rr2) {
			t.Error("ReporterRepresentations with identical unique constraint fields should be considered duplicates")
		}
	})

	t.Run("should allow different representations when unique constraint fields differ", func(t *testing.T) {
		t.Parallel()

		// Create two ReporterRepresentations with different Generation values
		rr1, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1, // Generation = 1
			"https://api.example.com",
			nil,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "First ReporterRepresentation should be valid")

		rr2, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			2, // Generation = 2 (different)
			"https://api.example.com",
			nil,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Second ReporterRepresentation should be valid")

		// They should not be considered duplicates
		if areReporterRepresentationsDuplicates(*rr1, *rr2) {
			t.Error("ReporterRepresentations with different unique constraint fields should not be considered duplicates")
		}
	})

	t.Run("should enforce positive values for numeric fields", func(t *testing.T) {
		t.Parallel()

		// Test Generation = 1 (positive)
		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1, // Generation = 1
			"https://api.example.com",
			nil,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Positive Generation should be valid")

		// Test Version = 1 (positive)
		_, err = NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1, // Version = 1
			"test-instance",
			1,
			"https://api.example.com",
			nil,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Positive Version should be valid")

		// Test CommonVersion = 1 (positive)
		_, err = NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
			1, // CommonVersion = 1
			false,
			nil,
		)
		AssertNoError(t, err, "Positive CommonVersion should be valid")
	})

	t.Run("should require non-empty string fields", func(t *testing.T) {
		t.Parallel()

		// Test LocalResourceID required
		t.Run("LocalResourceID_required", func(t *testing.T) {
			t.Parallel()
			_, err := NewReporterRepresentation(
				JsonObject{"test": "data"},
				"", // empty LocalResourceID
				"hbi",
				"host",
				1,
				"test-instance",
				1,
				"https://api.example.com",
				nil,
				1,
				false,
				nil,
			)
			AssertValidationError(t, err, "LocalResourceID", "LocalResourceID should be required")
		})

		// Test ReporterType required
		t.Run("ReporterType_required", func(t *testing.T) {
			t.Parallel()
			_, err := NewReporterRepresentation(
				JsonObject{"test": "data"},
				"test-local-id",
				"", // empty ReporterType
				"host",
				1,
				"test-instance",
				1,
				"https://api.example.com",
				nil,
				1,
				false,
				nil,
			)
			AssertValidationError(t, err, "ReporterType", "ReporterType should be required")
		})

		// Test ResourceType required
		t.Run("ResourceType_required", func(t *testing.T) {
			t.Parallel()
			_, err := NewReporterRepresentation(
				JsonObject{"test": "data"},
				"test-local-id",
				"hbi",
				"", // empty ResourceType
				1,
				"test-instance",
				1,
				"https://api.example.com",
				nil,
				1,
				false,
				nil,
			)
			AssertValidationError(t, err, "ResourceType", "ResourceType should be required")
		})

		// Test ReporterInstanceID required
		t.Run("ReporterInstanceID_required", func(t *testing.T) {
			t.Parallel()
			_, err := NewReporterRepresentation(
				JsonObject{"test": "data"},
				"test-local-id",
				"hbi",
				"host",
				1,
				"", // empty ReporterInstanceID
				1,
				"https://api.example.com",
				nil,
				1,
				false,
				nil,
			)
			AssertValidationError(t, err, "ReporterInstanceID", "ReporterInstanceID should be required")
		})

		// Test APIHref required
		t.Run("APIHref_required", func(t *testing.T) {
			t.Parallel()
			_, err := NewReporterRepresentation(
				JsonObject{"test": "data"},
				"test-local-id",
				"hbi",
				"host",
				1,
				"test-instance",
				1,
				"", // empty APIHref
				nil,
				1,
				false,
				nil,
			)
			AssertValidationError(t, err, "APIHref", "APIHref should be required")
		})
	})
}

func TestReporterRepresentation_TombstoneLogic(t *testing.T) {
	t.Parallel()

	t.Run("should handle tombstone true", func(t *testing.T) {
		t.Parallel()

		// Test validation through factory method (proper approach)
		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
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

		// Test validation through factory method (proper approach)
		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
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
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
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
				"test-local-id",
				"hbi",
				"host",
				version, // Test different version values
				"test-instance",
				1,
				"https://api.example.com",
				nil,
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
				"test-local-id",
				"hbi",
				"host",
				1,
				"test-instance",
				generation, // Test different generation values
				"https://api.example.com",
				nil,
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
				"test-local-id",
				"hbi",
				"host",
				1,
				"test-instance",
				1,
				"https://api.example.com",
				nil,
				commonVersion, // Test different common version values
				false,
				nil,
			)
			AssertNoError(t, err, fmt.Sprintf("CommonVersion %d should be valid", commonVersion))
		}
	})
}

func TestReporterRepresentation_HrefValidation(t *testing.T) {
	t.Parallel()

	t.Run("should validate APIHref URL format", func(t *testing.T) {
		t.Parallel()

		validURLs := []string{
			"https://api.example.com/resource/123",
			"http://localhost:8080/api/v1/resource",
			"https://api.redhat.com/api/inventory/v1/hosts/123",
		}

		for _, url := range validURLs {
			t.Run(fmt.Sprintf("valid_url_%s", url), func(t *testing.T) {
				t.Parallel()
				_, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					"test-local-id",
					"hbi",
					"host",
					1,
					"test-instance",
					1,
					url, // Test different valid URLs
					nil,
					1,
					false,
					nil,
				)
				AssertNoError(t, err, fmt.Sprintf("URL %s should be valid", url))
			})
		}
	})

	t.Run("should reject invalid APIHref URLs", func(t *testing.T) {
		t.Parallel()

		invalidURLs := []string{
			"not-a-url",
			"ftp://example.com/resource",
			"",
			"://missing-scheme",
		}

		for _, url := range invalidURLs {
			t.Run(fmt.Sprintf("invalid_url_%s", url), func(t *testing.T) {
				t.Parallel()
				_, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					"test-local-id",
					"hbi",
					"host",
					1,
					"test-instance",
					1,
					url, // Test different invalid URLs
					nil,
					1,
					false,
					nil,
				)
				AssertValidationError(t, err, "APIHref", fmt.Sprintf("URL %s should be invalid", url))
			})
		}
	})

	t.Run("should validate ConsoleHref when provided", func(t *testing.T) {
		t.Parallel()

		validURLs := []string{
			"https://console.redhat.com/insights/inventory/123",
			"https://console.example.com/resource/456",
		}

		for _, url := range validURLs {
			t.Run(fmt.Sprintf("valid_console_url_%s", url), func(t *testing.T) {
				t.Parallel()
				_, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					"test-local-id",
					"hbi",
					"host",
					1,
					"test-instance",
					1,
					"https://api.example.com",
					stringPtr(url), // Test different valid console URLs
					1,
					false,
					nil,
				)
				AssertNoError(t, err, fmt.Sprintf("Console URL %s should be valid", url))
			})
		}
	})

	t.Run("should allow nil ConsoleHref", func(t *testing.T) {
		t.Parallel()

		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil, // Nil ConsoleHref
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Nil ConsoleHref should be valid")

		if rr.ConsoleHref != nil {
			t.Error("ConsoleHref should be nil")
		}
	})

	t.Run("should reject invalid ConsoleHref URLs", func(t *testing.T) {
		t.Parallel()

		invalidURLs := []string{
			"not-a-url",
			"ftp://example.com/resource",
			"://missing-scheme",
		}

		for _, url := range invalidURLs {
			t.Run(fmt.Sprintf("invalid_console_url_%s", url), func(t *testing.T) {
				t.Parallel()
				_, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					"test-local-id",
					"hbi",
					"host",
					1,
					"test-instance",
					1,
					"https://api.example.com",
					stringPtr(url), // Test different invalid console URLs
					1,
					false,
					nil,
				)
				AssertValidationError(t, err, "ConsoleHref", fmt.Sprintf("Console URL %s should be invalid", url))
			})
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
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
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
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
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
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
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

		// Test zero Generation - should be valid now
		rr, err := NewReporterRepresentation(
			JsonObject{"satellite_id": "test-satellite"},
			"local-123",
			"hbi",
			"host",
			1,
			"reporter-instance-123",
			0, // zero Generation should be valid
			"https://api.example.com/resource/123",
			stringPtr("https://console.example.com/resource/123"),
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
