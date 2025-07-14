package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Infrastructure tests for ReporterRepresentation domain model
// These tests focus on database schema, field structure validation, edge cases, and serialization

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
			"Metadata":        reflect.TypeOf((*ReporterRepresentationMetadata)(nil)),
			"Version":         reflect.TypeOf(uint(0)),
			"Generation":      reflect.TypeOf(uint(0)),
			"CommonVersion":   reflect.TypeOf(uint(0)),
			"Tombstone":       reflect.TypeOf(false),
			"ReporterVersion": reflect.TypeOf((*string)(nil)),
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

		// Check fields that should have the unique index tag (now only versioned fields)
		expectedIndexFields := []string{
			"Version", "Generation",
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

		// Check size constraints for remaining fields
		sizeConstraints := map[string]string{
			"ReporterVersion": "size:128",
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

	t.Run("should have correct nullable field types", func(t *testing.T) {
		t.Parallel()

		rrType := reflect.TypeOf(ReporterRepresentation{})

		// Check nullable fields (now only ReporterVersion in main struct)
		nullableFields := []string{"ReporterVersion"}

		for _, fieldName := range nullableFields {
			field, found := rrType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}

			// Check if it's a pointer type
			if field.Type.Kind() != reflect.Ptr {
				t.Errorf("Field %s should be a pointer type for nullable field", fieldName)
			}

			// Check if it's a pointer to string
			if field.Type.Elem().Kind() != reflect.String {
				t.Errorf("Field %s should be a pointer to string", fieldName)
			}
		}
	})

	t.Run("should have correct non_nullable field types", func(t *testing.T) {
		t.Parallel()

		rrType := reflect.TypeOf(ReporterRepresentation{})

		// Check uint fields
		uintFields := []string{"Version", "Generation", "CommonVersion"}

		for _, fieldName := range uintFields {
			field, found := rrType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}

			// Check if it's a uint type
			if field.Type.Kind() != reflect.Uint {
				t.Errorf("Field %s should be a uint type, got: %v", fieldName, field.Type)
			}
		}

		// Check boolean field
		field, found := rrType.FieldByName("Tombstone")
		if !found {
			t.Error("Tombstone field not found")
		} else if field.Type.Kind() != reflect.Bool {
			t.Errorf("Tombstone should be a bool type, got: %v", field.Type)
		}
	})

	t.Run("should have metadata field with correct GORM tag", func(t *testing.T) {
		t.Parallel()

		rrType := reflect.TypeOf(ReporterRepresentation{})
		field, found := rrType.FieldByName("Metadata")
		if !found {
			t.Error("Metadata field not found")
			return
		}

		gormTag := field.Tag.Get("gorm")
		if gormTag != "-" {
			t.Errorf("Metadata field should have gorm:\"-\" tag, got: %s", gormTag)
		}
	})
}

func TestReporterRepresentation_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle empty string values for nullable fields", func(t *testing.T) {
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
			stringPtr(""), // Empty ConsoleHref
			1,
			false,
			stringPtr(""), // Empty ReporterVersion
		)
		AssertNoError(t, err, "ReporterRepresentation with empty string values for nullable fields should be valid")
	})

	t.Run("should handle nil values for nullable fields", func(t *testing.T) {
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
			nil, // Nil ConsoleHref
			1,
			false,
			nil, // Nil ReporterVersion
		)
		AssertNoError(t, err, "ReporterRepresentation with nil values for nullable fields should be valid")
	})

	t.Run("should handle complex nested JSON data", func(t *testing.T) {
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
			complexData,
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
			JsonObject{},
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

	t.Run("should handle version boundary values", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name    string
			version uint
			valid   bool
		}{
			{"version_1", 1, true},
			{"version_max_uint32", 4294967295, true},
			{"version_large", 1000000, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				_, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					"test-local-id",
					"hbi",
					"host",
					tc.version,
					"test-instance",
					1,
					"https://api.example.com",
					nil,
					1,
					false,
					nil,
				)
				if tc.valid {
					AssertNoError(t, err, "Version should be valid")
				} else {
					AssertError(t, err, "Version should be invalid")
				}
			})
		}
	})

	t.Run("should handle generation boundary values", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name       string
			generation uint
			valid      bool
		}{
			{"generation_1", 1, true},
			{"generation_max_uint32", 4294967295, true},
			{"generation_large", 1000000, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				_, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					"test-local-id",
					"hbi",
					"host",
					1,
					"test-instance",
					tc.generation,
					"https://api.example.com",
					nil,
					1,
					false,
					nil,
				)
				if tc.valid {
					AssertNoError(t, err, "Generation should be valid")
				} else {
					AssertError(t, err, "Generation should be invalid")
				}
			})
		}
	})

	t.Run("should handle tombstone flag variations", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name      string
			tombstone bool
		}{
			{"tombstone_true", true},
			{"tombstone_false", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
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
					nil,
					1,
					tc.tombstone,
					nil,
				)
				AssertNoError(t, err, "Tombstone flag should be valid")

				if rr.Tombstone != tc.tombstone {
					t.Errorf("Expected tombstone %v, got %v", tc.tombstone, rr.Tombstone)
				}
			})
		}
	})

	t.Run("should handle long URL values", func(t *testing.T) {
		t.Parallel()

		// Create a long but valid URL
		longURL := "https://api.example.com/v1/resources/very-long-resource-name-that-might-be-generated-by-system"
		for i := 0; i < 10; i++ {
			longURL += "/nested/path/segment"
		}
		longURL += "?query=parameter&another=value"

		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			longURL,
			stringPtr(longURL),
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with long URL values should be valid")
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

		// Compare key fields - now accessing through metadata
		AssertEqual(t, original.Metadata.LocalResourceID, unmarshaled.Metadata.LocalResourceID, "LocalResourceID should match after JSON round-trip")
		AssertEqual(t, original.Metadata.ReporterType, unmarshaled.Metadata.ReporterType, "ReporterType should match after JSON round-trip")
		AssertEqual(t, original.Metadata.ResourceType, unmarshaled.Metadata.ResourceType, "ResourceType should match after JSON round-trip")
		AssertEqual(t, original.Version, unmarshaled.Version, "Version should match after JSON round-trip")
		AssertEqual(t, original.Generation, unmarshaled.Generation, "Generation should match after JSON round-trip")
		AssertEqual(t, original.Tombstone, unmarshaled.Tombstone, "Tombstone should match after JSON round-trip")
	})

	t.Run("should handle null values in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.Metadata.ConsoleHref = nil
		rr.ReporterVersion = nil

		// Test JSON marshaling with null values
		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with null values to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with null values from JSON")

		// Check that null values are preserved
		if unmarshaled.Metadata.ConsoleHref != nil {
			t.Error("ConsoleHref should be nil after JSON round-trip")
		}
		if unmarshaled.ReporterVersion != nil {
			t.Error("ReporterVersion should be nil after JSON round-trip")
		}
	})

	t.Run("should handle empty string values in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		emptyString := ""
		rr.Metadata.ConsoleHref = &emptyString
		rr.ReporterVersion = &emptyString

		// Test JSON marshaling with empty string values
		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with empty string values to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with empty string values from JSON")

		// Check that empty string values are preserved
		if unmarshaled.Metadata.ConsoleHref == nil || *unmarshaled.Metadata.ConsoleHref != "" {
			t.Error("ConsoleHref should be empty string after JSON round-trip")
		}
		if unmarshaled.ReporterVersion == nil || *unmarshaled.ReporterVersion != "" {
			t.Error("ReporterVersion should be empty string after JSON round-trip")
		}
	})
}
