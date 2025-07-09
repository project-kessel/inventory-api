package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Test scenarios for ReporterRepresentation domain model
// These tests focus on domain logic, business rules, and model behavior
// rather than database operations or infrastructure concerns.

func TestReporterRepresentation_TableName(t *testing.T) {
	t.Parallel()

	fixture := NewTestFixture(t)
	rr := fixture.ValidReporterRepresentation()

	AssertTableName(t, rr, "reporter_representation")
}

func TestReporterRepresentation_Structure(t *testing.T) {
	t.Parallel()

	t.Run("should properly embed Representation", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{}

		// Check if Representation is embedded
		rrType := reflect.TypeOf(rr)
		found := false
		for i := 0; i < rrType.NumField(); i++ {
			field := rrType.Field(i)
			if field.Type == reflect.TypeOf(Representation{}) && field.Anonymous {
				found = true
				break
			}
		}

		if !found {
			t.Error("ReporterRepresentation should embed Representation")
		}
	})

	t.Run("should have all required fields with correct types", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{}
		rrType := reflect.TypeOf(rr)

		expectedFields := map[string]reflect.Type{
			"LocalResourceID":    reflect.TypeOf(""),
			"ReporterType":       reflect.TypeOf(""),
			"ResourceType":       reflect.TypeOf(""),
			"Version":            reflect.TypeOf(uint(0)),
			"ReporterInstanceID": reflect.TypeOf(""),
			"Generation":         reflect.TypeOf(uint(0)),
			"APIHref":            reflect.TypeOf(""),
			"ConsoleHref":        reflect.TypeOf((*string)(nil)),
			"CommonVersion":      reflect.TypeOf(uint(0)),
			"Tombstone":          reflect.TypeOf(false),
			"ReporterVersion":    reflect.TypeOf((*string)(nil)),
		}

		for fieldName, expectedType := range expectedFields {
			field, found := rrType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}
			if field.Type != expectedType {
				t.Errorf("Field %s has type %v, expected %v", fieldName, field.Type, expectedType)
			}
		}
	})

	t.Run("should have proper boolean field", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{}
		rrType := reflect.TypeOf(rr)

		field, found := rrType.FieldByName("Tombstone")
		if !found {
			t.Error("Tombstone field not found")
		}
		if field.Type != reflect.TypeOf(false) {
			t.Error("Tombstone should be a boolean field")
		}
	})

	t.Run("should have correct GORM tags for unique index", func(t *testing.T) {
		t.Parallel()

		rrType := reflect.TypeOf(ReporterRepresentation{})

		// Check all fields that should have the unique index tag
		expectedIndexFields := []string{
			"LocalResourceID", "ReporterType", "ResourceType",
			"Version", "ReporterInstanceID", "Generation",
		}

		for _, fieldName := range expectedIndexFields {
			field, found := rrType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}

			tag := field.Tag.Get("gorm")
			if !strings.Contains(tag, "index:reporter_rep_unique_idx,unique") {
				t.Errorf("Field %s should have unique index tag, got: %s", fieldName, tag)
			}
		}
	})

	t.Run("should have correct GORM size constraints", func(t *testing.T) {
		t.Parallel()

		rrType := reflect.TypeOf(ReporterRepresentation{})

		// Check size constraints
		sizeConstraints := map[string]string{
			"LocalResourceID":    "size:128",
			"ReporterType":       "size:128",
			"ResourceType":       "size:128",
			"ReporterInstanceID": "size:128",
			"APIHref":            "size:512",
			"ConsoleHref":        "size:512",
			"ReporterVersion":    "size:128",
		}

		for fieldName, expectedSize := range sizeConstraints {
			field, found := rrType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}

			gormTag := field.Tag.Get("gorm")
			if !strings.Contains(gormTag, expectedSize) {
				t.Errorf("Field %s should have %s constraint, got: %s", fieldName, expectedSize, gormTag)
			}
		}
	})
}

func TestReporterRepresentation_Validation(t *testing.T) {
	t.Parallel()

	t.Run("valid ReporterRepresentation with all required fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

		AssertNoError(t, ValidateReporterRepresentation(rr), "Valid ReporterRepresentation should not have validation errors")
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
			limit int
		}{
			{"LocalResourceID too long", "LocalResourceID", strings.Repeat("a", 129), 128},
			{"ReporterType too long", "ReporterType", strings.Repeat("a", 129), 128},
			{"ResourceType too long", "ResourceType", strings.Repeat("a", 129), 128},
			{"ReporterInstanceID too long", "ReporterInstanceID", strings.Repeat("a", 129), 128},
			{"APIHref too long", "APIHref", strings.Repeat("a", 513), 512},
			{"ConsoleHref too long", "ConsoleHref", strings.Repeat("a", 513), 512},
			{"ReporterVersion too long", "ReporterVersion", strings.Repeat("a", 129), 128},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				rr := fixture.ValidReporterRepresentation()

				// Use reflection to set the field value
				rv := reflect.ValueOf(rr).Elem()
				field := rv.FieldByName(tc.field)
				if !field.IsValid() {
					t.Fatalf("Field %s not found", tc.field)
				}

				// Handle pointer fields differently
				if field.Kind() == reflect.Ptr {
					// For pointer fields, create a new string value and set the pointer
					stringValue := tc.value
					field.Set(reflect.ValueOf(&stringValue))
				} else {
					field.SetString(tc.value)
				}

				err := ValidateReporterRepresentation(rr)
				if err == nil {
					t.Errorf("Expected validation error for %s exceeding %d characters", tc.field, tc.limit)
				}

				expectedMsg := fmt.Sprintf("exceeds maximum length of %d characters", tc.limit)
				if !strings.Contains(err.Error(), expectedMsg) {
					t.Errorf("Expected error message to contain '%s', got: %s", expectedMsg, err.Error())
				}
			})
		}
	})

	t.Run("ReporterRepresentation with whitespace-only fields should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)

		testCases := []struct {
			name  string
			field string
			value string
		}{
			{"whitespace-only LocalResourceID", "LocalResourceID", "   "},
			{"whitespace-only ReporterType", "ReporterType", "   "},
			{"whitespace-only ResourceType", "ResourceType", "   "},
			{"whitespace-only ReporterInstanceID", "ReporterInstanceID", "   "},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				rr := fixture.ValidReporterRepresentation()

				// Use reflection to set the field value
				rrValue := reflect.ValueOf(rr).Elem()
				field := rrValue.FieldByName(tc.field)
				if field.IsValid() && field.CanSet() {
					field.SetString(tc.value)
				}

				AssertError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with whitespace-only "+tc.field+" should be invalid")
			})
		}
	})
}

func TestReporterRepresentation_BusinessRules(t *testing.T) {
	t.Parallel()

	t.Run("unique constraint should be enforced by all key fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr1 := fixture.ValidReporterRepresentation()
		rr2 := fixture.ValidReporterRepresentation()

		// Check that identical representations would be duplicates
		if !areReporterRepresentationsDuplicates(*rr1, *rr2) {
			t.Error("ReporterRepresentations with identical key fields should be considered duplicates")
		}
	})

	t.Run("different LocalResourceID should not be duplicates", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr1 := fixture.ValidReporterRepresentation()
		rr2 := fixture.ReporterRepresentationWithLocalResourceID("different-local-id")

		if areReporterRepresentationsDuplicates(*rr1, *rr2) {
			t.Error("ReporterRepresentations with different LocalResourceID should not be duplicates")
		}
	})

	t.Run("different Generation should not be duplicates", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr1 := fixture.ValidReporterRepresentation()
		rr2 := fixture.ValidReporterRepresentation()
		rr2.Generation = 2

		if areReporterRepresentationsDuplicates(*rr1, *rr2) {
			t.Error("ReporterRepresentations with different Generation should not be duplicates")
		}
	})

	t.Run("Generation should support incremental updates", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		originalGeneration := rr.Generation

		rr.Generation++

		if rr.Generation != originalGeneration+1 {
			t.Error("Generation should support incremental updates")
		}
	})

	t.Run("CommonVersion should link to CommonRepresentation", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.CommonVersion = 5

		if rr.CommonVersion != 5 {
			t.Error("CommonVersion should be settable to link to CommonRepresentation")
		}
	})

	t.Run("ReporterVersion should track reporter software version", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		validVersions := []string{"1.0.0", "2.1.3", "1.0.0-beta.1", "1.0.0+build.123"}

		for _, version := range validVersions {
			rr := fixture.ValidReporterRepresentation()
			rr.ReporterVersion = &version

			AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with valid ReporterVersion "+version+" should be valid")
		}
	})

	t.Run("ReporterVersion can be nil", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ReporterRepresentationWithNilReporterVersion()

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with nil ReporterVersion should be valid")

		if rr.ReporterVersion != nil {
			t.Error("ReporterVersion should be nil")
		}
	})

	t.Run("ReporterVersion can be empty string", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ReporterRepresentationWithReporterVersion(stringPtr(""))

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with empty ReporterVersion should be valid")

		if rr.ReporterVersion == nil || *rr.ReporterVersion != "" {
			t.Error("ReporterVersion should be empty string")
		}
	})

	t.Run("ReporterVersion exceeding length limit should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		longVersion := strings.Repeat("a", 129) // 129 characters, exceeding the 128 limit
		rr := fixture.ReporterRepresentationWithReporterVersion(stringPtr(longVersion))

		AssertValidationError(t, ValidateReporterRepresentation(rr), "ReporterVersion", "ReporterRepresentation with ReporterVersion exceeding length limit should be invalid")
	})
}

func TestReporterRepresentation_TombstoneLogic(t *testing.T) {
	t.Parallel()

	t.Run("Tombstone should default to false", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{}

		if rr.Tombstone != false {
			t.Error("Tombstone should default to false")
		}
	})

	t.Run("Tombstone can be set to true", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ReporterRepresentationWithTombstone(true)

		if !rr.Tombstone {
			t.Error("Tombstone should be settable to true")
		}

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with Tombstone=true should be valid")
	})

	t.Run("Tombstone can be set to false", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ReporterRepresentationWithTombstone(false)

		if rr.Tombstone {
			t.Error("Tombstone should be settable to false")
		}

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with Tombstone=false should be valid")
	})

	t.Run("Tombstone logic should support resource lifecycle", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr1 := fixture.ReporterRepresentationWithTombstone(false)
		rr2 := fixture.ReporterRepresentationWithTombstone(true)

		// Both should be valid
		AssertNoError(t, ValidateReporterRepresentation(rr1), "Active resource should be valid")
		AssertNoError(t, ValidateReporterRepresentation(rr2), "Tombstoned resource should be valid")
	})
}

func TestReporterRepresentation_VersioningLogic(t *testing.T) {
	t.Parallel()

	t.Run("Version should be positive", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.Version = 1

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with positive Version should be valid")
	})

	t.Run("CommonVersion should be positive", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.CommonVersion = 1

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with positive CommonVersion should be valid")
	})

	t.Run("Generation should be positive", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.Generation = 1

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with positive Generation should be valid")
	})
}

func TestReporterRepresentation_HrefValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid APIHref should be accepted", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		validURLs := []string{
			"https://api.example.com/resource/123",
			"http://localhost:8080/api/v1/resource",
			"https://console.redhat.com/insights/inventory/123",
		}

		for _, url := range validURLs {
			rr := fixture.ReporterRepresentationWithAPIHref(url)
			AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with valid APIHref should be valid")
		}
	})

	t.Run("invalid APIHref should be rejected", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		invalidURLs := []string{
			"not-a-url",
			"ftp://example.com/resource",
			"javascript:alert('xss')",
		}

		for _, url := range invalidURLs {
			rr := fixture.ReporterRepresentationWithAPIHref(url)
			AssertValidationError(t, ValidateReporterRepresentation(rr), "APIHref", "ReporterRepresentation with invalid APIHref should be invalid")
		}
	})

	t.Run("valid ConsoleHref should be accepted", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		validURLs := []string{
			"https://console.example.com/resource/123",
			"http://localhost:3000/dashboard/resource",
			"https://console.redhat.com/insights/inventory/123",
		}

		for _, url := range validURLs {
			rr := fixture.ReporterRepresentationWithConsoleHref(url)
			AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with valid ConsoleHref should be valid")
		}
	})

	t.Run("empty href fields should be valid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.APIHref = ""
		rr.ConsoleHref = nil

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with empty href fields should be valid")
	})

	t.Run("ConsoleHref can be nil", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ReporterRepresentationWithNilConsoleHref()

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with nil ConsoleHref should be valid")

		if rr.ConsoleHref != nil {
			t.Error("ConsoleHref should be nil")
		}
	})

	t.Run("ConsoleHref can be empty string", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ReporterRepresentationWithConsoleHref("")

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with empty ConsoleHref should be valid")

		if rr.ConsoleHref != nil {
			t.Error("ConsoleHref should be nil when empty string is provided")
		}
	})

	t.Run("ConsoleHref exceeding length limit should be invalid", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		longHref := "https://example.com/" + strings.Repeat("a", 500) // Exceeding 512 characters
		rr := fixture.ReporterRepresentationWithConsoleHref(longHref)

		AssertValidationError(t, ValidateReporterRepresentation(rr), "ConsoleHref", "ReporterRepresentation with ConsoleHref exceeding length limit should be invalid")
	})
}

func TestReporterRepresentation_DataHandling(t *testing.T) {
	t.Parallel()

	t.Run("should handle JSON data correctly", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

		// Test that data is properly handled
		if rr.Data == nil {
			t.Error("Data should not be nil for valid ReporterRepresentation")
		}

		// Test that data can be accessed
		if _, ok := rr.Data["satellite_id"]; !ok {
			t.Error("Data should be accessible as JsonObject")
		}
	})

	t.Run("should handle complex nested JSON", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		complexData := JsonObject{
			"metadata": JsonObject{
				"labels": JsonObject{
					"app":     "test-app",
					"version": "1.0.0",
				},
				"annotations": JsonObject{
					"description": "Test resource",
				},
			},
			"spec": JsonObject{
				"replicas": 3,
				"image":    "nginx:latest",
			},
			"status": JsonObject{
				"ready":         true,
				"readyReplicas": 3,
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

func TestReporterRepresentation_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle unicode characters", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.LocalResourceID = "测试-resource-🌟"
		rr.ReporterType = "测试-reporter"
		rr.Data = JsonObject{
			"name":        "测试资源",
			"description": "包含Unicode字符的描述 🌟",
		}

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with unicode characters should be valid")
	})

	t.Run("should handle special characters in string fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.LocalResourceID = "resource-with-special-chars-!@#$%^&*()"
		rr.ReporterType = "special-reporter-type"
		rr.Data = JsonObject{
			"special_field": "Value with special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?",
		}

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with special characters should be valid")
	})

	t.Run("should handle large integer values", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.Version = 4294967295 // Max uint32
		rr.Generation = 4294967295
		rr.CommonVersion = 4294967295

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with large integer values should be valid")
	})
}

func TestReporterRepresentation_Serialization(t *testing.T) {
	t.Parallel()

	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		original := fixture.ValidReporterRepresentation()

		// Test JSON marshaling
		jsonData, err := json.Marshal(original)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation from JSON")

		// Compare key fields
		AssertEqual(t, original.LocalResourceID, unmarshaled.LocalResourceID, "LocalResourceID should match after JSON round-trip")
		AssertEqual(t, original.ReporterType, unmarshaled.ReporterType, "ReporterType should match after JSON round-trip")
		AssertEqual(t, original.ResourceType, unmarshaled.ResourceType, "ResourceType should match after JSON round-trip")
		AssertEqual(t, original.Version, unmarshaled.Version, "Version should match after JSON round-trip")
		AssertEqual(t, original.Generation, unmarshaled.Generation, "Generation should match after JSON round-trip")
		AssertEqual(t, original.Tombstone, unmarshaled.Tombstone, "Tombstone should match after JSON round-trip")
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
