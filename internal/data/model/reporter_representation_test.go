package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
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
			"ReporterResourceID": reflect.TypeOf(uuid.UUID{}),
			"Version":            reflect.TypeOf(uint(0)),
			"Generation":         reflect.TypeOf(uint(0)),
			"CommonVersion":      reflect.TypeOf((*uint)(nil)),
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

	t.Run("should have correct GORM tags for primary key", func(t *testing.T) {
		t.Parallel()

		rrType := reflect.TypeOf(ReporterRepresentation{})

		// Check primary key fields
		primaryKeyFields := []string{"ReporterResourceID", "Version", "Generation"}

		for _, fieldName := range primaryKeyFields {
			field, found := rrType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}

			tag := field.Tag.Get("gorm")
			if !strings.Contains(tag, "primaryKey") {
				t.Errorf("Field %s should have primaryKey tag, got: %s", fieldName, tag)
			}
		}
	})

	t.Run("should have correct GORM size constraints", func(t *testing.T) {
		t.Parallel()

		rrType := reflect.TypeOf(ReporterRepresentation{})

		// Check size constraints
		sizeConstraints := map[string]string{
			"ReporterResourceID": "size:128",
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

	t.Run("should have correct nullable field types", func(t *testing.T) {
		t.Parallel()

		rrType := reflect.TypeOf(ReporterRepresentation{})

		// Check nullable fields
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

		// Check UUID fields
		uuidFields := []string{"ReporterResourceID"}

		for _, fieldName := range uuidFields {
			field, found := rrType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}

			// Check if it's a UUID type
			if field.Type != reflect.TypeOf(uuid.UUID{}) {
				t.Errorf("Field %s should be a UUID type, got: %v", fieldName, field.Type)
			}
		}

		// Check uint fields (CommonVersion is now nullable so excluded)
		uintFields := []string{"Version", "Generation"}

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
	})
}

func TestReporterRepresentation_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle unicode characters", func(t *testing.T) {
		t.Parallel()

		commonVersion := uint(1)
		_, err := NewReporterRepresentation(
			internal.JsonObject{
				"name":        "测试资源",
				"description": "包含Unicode字符的描述 🌟",
			},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440010"),
			1,
			1,
			&commonVersion,
			"test-transaction-id-unicode-test",
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with unicode characters should be valid")
	})

	t.Run("should handle special characters in string fields", func(t *testing.T) {
		t.Parallel()

		commonVersion := uint(1)
		_, err := NewReporterRepresentation(
			internal.JsonObject{
				"special_field": "Value with special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?",
			},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440011"),
			1,
			1,
			&commonVersion,
			"test-transaction-id-special-chars-test",
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with special characters should be valid")
	})

	t.Run("should handle large integer values", func(t *testing.T) {
		t.Parallel()

		commonVersion := uint(4294967295)
		_, err := NewReporterRepresentation(
			internal.JsonObject{"test": "data"},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440012"),
			4294967295, // Max uint32 Version
			4294967295, // Max uint32 Generation
			&commonVersion, // Max uint32 CommonVersion
			"test-transaction-id-large-integers",
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with large integer values should be valid")
	})

	t.Run("should handle empty string values for nullable fields", func(t *testing.T) {
		t.Parallel()

		commonVersion := uint(1)
		_, err := NewReporterRepresentation(
			internal.JsonObject{"test": "data"},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440012"),
			1,
			1,
			&commonVersion,
			"test-transaction-id-empty-string",
			false,
			internal.StringPtr(""), // Empty ReporterVersion
		)
		AssertNoError(t, err, "ReporterRepresentation with empty string values for nullable fields should be valid")
	})

	t.Run("should handle nil values for nullable fields", func(t *testing.T) {
		t.Parallel()

		commonVersion := uint(1)
		_, err := NewReporterRepresentation(
			internal.JsonObject{"test": "data"},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440012"),
			1,
			1,
			&commonVersion,
			"test-transaction-id-nil-values",
			false,
			nil, // Nil ReporterVersion
		)
		AssertNoError(t, err, "ReporterRepresentation with nil values for nullable fields should be valid")
	})

	t.Run("should handle complex nested JSON data", func(t *testing.T) {
		t.Parallel()

		complexData := internal.JsonObject{
			"metadata": internal.JsonObject{
				"labels": internal.JsonObject{
					"app":         "test-app",
					"environment": "staging",
					"team":        "platform",
				},
				"annotations": internal.JsonObject{
					"deployment.kubernetes.io/revision":                "1",
					"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test"}}`,
				},
			},
			"spec": internal.JsonObject{
				"containers": []interface{}{
					internal.JsonObject{
						"name":  "app",
						"image": "nginx:1.21",
						"ports": []interface{}{
							internal.JsonObject{"containerPort": 80},
							internal.JsonObject{"containerPort": 443},
						},
					},
				},
			},
			"status": internal.JsonObject{
				"phase": "Running",
				"conditions": []interface{}{
					internal.JsonObject{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		}

		commonVersion := uint(1)
		_, err := NewReporterRepresentation(
			complexData,
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440012"),
			1,
			1,
			&commonVersion,
			"test-transaction-id-complex-json",
			false,
			nil,
		)
		AssertNoError(t, err, "ReporterRepresentation with complex nested JSON should be valid")
	})

	t.Run("should handle empty JSON object", func(t *testing.T) {
		t.Parallel()

		commonVersion := uint(1)
		_, err := NewReporterRepresentation(
			internal.JsonObject{},
			uuid.MustParse("550e8400-e29b-41d4-a716-446655440012"),
			1,
			1,
			&commonVersion,
			"test-transaction-id-empty-json",
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
			{"version_0", 0, true},
			{"version_1", 1, true},
			{"version_max_uint32", 4294967295, true},
			{"version_large", 1000000, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				commonVersion := uint(1)
				_, err := NewReporterRepresentation(
					internal.JsonObject{"test": "data"},
					uuid.MustParse("550e8400-e29b-41d4-a716-446655440012"),
					tc.version,
					1,
					&commonVersion,
					"test-transaction-id-version-boundary",
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
			{"generation_0", 0, true},
			{"generation_1", 1, true},
			{"generation_max_uint32", 4294967295, true},
			{"generation_large", 1000000, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				commonVersion := uint(1)
				_, err := NewReporterRepresentation(
					internal.JsonObject{"test": "data"},
					uuid.MustParse("550e8400-e29b-41d4-a716-446655440012"),
					1,
					tc.generation,
					&commonVersion,
					"test-transaction-id-generation-boundary",
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
				commonVersion := uint(1)
				rr, err := NewReporterRepresentation(
					internal.JsonObject{"test": "data"},
					uuid.MustParse("550e8400-e29b-41d4-a716-446655440012"),
					1,
					1,
					&commonVersion,
					"test-transaction-id-tombstone-flag",
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
		AssertEqual(t, original.ReporterResourceID, unmarshaled.ReporterResourceID, "ReporterResourceID should match after JSON round-trip")
		AssertEqual(t, original.Version, unmarshaled.Version, "Version should match after JSON round-trip")
		AssertEqual(t, original.Generation, unmarshaled.Generation, "Generation should match after JSON round-trip")
		AssertEqual(t, original.Tombstone, unmarshaled.Tombstone, "Tombstone should match after JSON round-trip")
	})

	t.Run("should handle null values in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.ReporterVersion = nil

		// Test JSON marshaling with null values
		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with null values to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with null values from JSON")

		// Check that null values are preserved
		if unmarshaled.ReporterVersion != nil {
			t.Error("ReporterVersion should be nil after JSON round-trip")
		}
	})

	t.Run("should handle empty string values in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		emptyString := ""
		rr.ReporterVersion = &emptyString

		// Test JSON marshaling with empty string values
		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with empty string values to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with empty string values from JSON")

		// Check that empty string values are preserved
		if unmarshaled.ReporterVersion == nil || *unmarshaled.ReporterVersion != "" {
			t.Error("ReporterVersion should be empty string after JSON round-trip")
		}
	})

	t.Run("should handle unicode characters in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.ReporterResourceID = uuid.MustParse("550e8400-e29b-41d4-a716-446655440020")
		rr.Data = internal.JsonObject{
			"name":        "测试资源",
			"description": "包含Unicode字符的描述 🌟",
		}

		// Test JSON marshaling with unicode characters
		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with unicode characters to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with unicode characters from JSON")

		// Check that unicode characters are preserved
		AssertEqual(t, rr.ReporterResourceID, unmarshaled.ReporterResourceID, "Unicode ReporterResourceID should match after JSON round-trip")

		// Check unicode in data
		if nameField, ok := unmarshaled.Data["name"]; ok {
			if nameStr, ok := nameField.(string); ok {
				AssertEqual(t, "测试资源", nameStr, "Unicode name in data should match after JSON round-trip")
			}
		}
	})

	t.Run("should handle special characters in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.ReporterResourceID = uuid.MustParse("550e8400-e29b-41d4-a716-446655440021")
		rr.Data = internal.JsonObject{
			"special_field": "Value with special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?",
		}

		// Test JSON marshaling with special characters
		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with special characters to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with special characters from JSON")

		// Check that special characters are preserved
		AssertEqual(t, rr.ReporterResourceID, unmarshaled.ReporterResourceID, "Special character ReporterResourceID should match after JSON round-trip")

		// Check special characters in data
		if specialField, ok := unmarshaled.Data["special_field"]; ok {
			if specialStr, ok := specialField.(string); ok {
				expected := "Value with special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?"
				AssertEqual(t, expected, specialStr, "Special character field in data should match after JSON round-trip")
			}
		}
	})

	t.Run("should handle complex nested JSON data serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

		complexData := internal.JsonObject{
			"metadata": internal.JsonObject{
				"labels": internal.JsonObject{
					"app":         "test-app",
					"environment": "staging",
				},
				"annotations": internal.JsonObject{
					"deployment.kubernetes.io/revision": "1",
				},
			},
			"spec": internal.JsonObject{
				"containers": []interface{}{
					internal.JsonObject{
						"name":  "app",
						"image": "nginx:1.21",
						"ports": []interface{}{
							internal.JsonObject{"containerPort": 80},
							internal.JsonObject{"containerPort": 443},
						},
					},
				},
			},
		}

		rr.Data = complexData

		// Test JSON marshaling with complex nested data
		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with complex nested data to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with complex nested data from JSON")

		// Check that complex nested data structure is preserved
		if metadata, ok := unmarshaled.Data["metadata"]; ok {
			if metadataObj, ok := metadata.(map[string]interface{}); ok {
				if labels, ok := metadataObj["labels"]; ok {
					if labelsObj, ok := labels.(map[string]interface{}); ok {
						if app, ok := labelsObj["app"]; ok {
							AssertEqual(t, "test-app", app, "Nested app label should match after JSON round-trip")
						}
					}
				}
			}
		}
	})

	t.Run("should handle boolean values in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)

		// Test with tombstone = true
		rr := fixture.ValidReporterRepresentation()
		rr.Tombstone = true

		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with tombstone=true to JSON")

		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with tombstone=true from JSON")

		AssertEqual(t, true, unmarshaled.Tombstone, "Tombstone=true should match after JSON round-trip")

		// Test with tombstone = false
		rr.Tombstone = false

		jsonData, err = json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with tombstone=false to JSON")

		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with tombstone=false from JSON")

		AssertEqual(t, false, unmarshaled.Tombstone, "Tombstone=false should match after JSON round-trip")
	})

	t.Run("should handle large integer values in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.Version = 4294967295 // Max uint32
		rr.Generation = 4294967295
		commonVersion := uint(4294967295)
		rr.CommonVersion = &commonVersion

		// Test JSON marshaling with large integer values
		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with large integer values to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with large integer values from JSON")

		// Check that large integer values are preserved
		AssertEqual(t, uint(4294967295), unmarshaled.Version, "Large Version should match after JSON round-trip")
		AssertEqual(t, uint(4294967295), unmarshaled.Generation, "Large Generation should match after JSON round-trip")
		if unmarshaled.CommonVersion != nil {
			AssertEqual(t, uint(4294967295), *unmarshaled.CommonVersion, "Large CommonVersion should match after JSON round-trip")
		} else {
			t.Error("CommonVersion should not be nil after JSON round-trip")
		}
	})

	t.Run("should handle empty data object in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.Data = internal.JsonObject{}

		// Test JSON marshaling with empty data object
		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with empty data object to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with empty data object from JSON")

		// Check that empty data object is preserved
		if unmarshaled.Data == nil {
			t.Error("Data should not be nil after JSON round-trip")
		}
		if len(unmarshaled.Data) != 0 {
			t.Errorf("Data should be empty after JSON round-trip, got: %v", unmarshaled.Data)
		}
	})

	t.Run("should enforce unique TransactionID constraint", func(t *testing.T) {
		t.Parallel()

		// Test that different ReporterRepresentations can have different TransactionIDs
		fixture := NewTestFixture(t)

		// Create first representation
		rr1 := fixture.ValidReporterRepresentation()
		AssertEqual(t, "test-transaction-id-valid-reporter", rr1.TransactionId, "First representation should have correct TransactionID")

		// Create second representation with different TransactionID
		rr2 := fixture.ReporterRepresentationWithTombstone(false)
		AssertEqual(t, "test-transaction-id-with-tombstone", rr2.TransactionId, "Second representation should have different TransactionID")

		// Verify they have different TransactionIDs
		AssertNotEqual(t, rr1.TransactionId, rr2.TransactionId, "Representations should have different TransactionIDs")
	})
}
