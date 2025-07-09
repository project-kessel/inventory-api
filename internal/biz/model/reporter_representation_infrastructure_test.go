package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Infrastructure tests for ReporterRepresentation domain model
// These tests focus on database schema, field structure validation, edge cases, and serialization

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

	t.Run("should have correct nullable field types", func(t *testing.T) {
		t.Parallel()

		rrType := reflect.TypeOf(ReporterRepresentation{})

		// Check nullable fields
		nullableFields := []string{"ConsoleHref", "ReporterVersion"}

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

		// Check non-nullable string fields
		nonNullableStringFields := []string{
			"LocalResourceID", "ReporterType", "ResourceType",
			"ReporterInstanceID", "APIHref",
		}

		for _, fieldName := range nonNullableStringFields {
			field, found := rrType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}

			// Check if it's a string type (not pointer)
			if field.Type.Kind() != reflect.String {
				t.Errorf("Field %s should be a string type, got: %v", fieldName, field.Type)
			}
		}

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

	t.Run("should handle empty string values for nullable fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.ConsoleHref = stringPtr("")
		rr.ReporterVersion = stringPtr("")

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with empty string values for nullable fields should be valid")
	})

	t.Run("should handle nil values for nullable fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.ConsoleHref = nil
		rr.ReporterVersion = nil

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with nil values for nullable fields should be valid")
	})

	t.Run("should handle maximum length string values", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

		// Create strings at maximum allowed length
		maxLen128 := ""
		for i := 0; i < 128; i++ {
			maxLen128 += "a"
		}

		maxLen512 := ""
		for i := 0; i < 512; i++ {
			maxLen512 += "b"
		}

		rr.LocalResourceID = maxLen128
		rr.ReporterType = maxLen128
		rr.ResourceType = maxLen128
		rr.ReporterInstanceID = maxLen128
		rr.APIHref = maxLen512
		rr.ConsoleHref = &maxLen512
		rr.ReporterVersion = &maxLen128

		AssertNoError(t, ValidateReporterRepresentation(rr), "ReporterRepresentation with maximum length string values should be valid")
	})

	t.Run("should handle complex nested JSON data", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

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

	t.Run("should handle version boundary values", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)

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
				rr := fixture.ValidReporterRepresentation()
				rr.Version = tc.version

				err := ValidateReporterRepresentation(rr)
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

		fixture := NewTestFixture(t)

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
				rr := fixture.ValidReporterRepresentation()
				rr.Generation = tc.generation

				err := ValidateReporterRepresentation(rr)
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

		fixture := NewTestFixture(t)

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
				rr := fixture.ValidReporterRepresentation()
				rr.Tombstone = tc.tombstone

				err := ValidateReporterRepresentation(rr)
				AssertNoError(t, err, "Tombstone flag should be valid")

				if rr.Tombstone != tc.tombstone {
					t.Errorf("Expected tombstone %v, got %v", tc.tombstone, rr.Tombstone)
				}
			})
		}
	})

	t.Run("should handle long URL values", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()

		// Create a long but valid URL
		longURL := "https://api.example.com/v1/resources/very-long-resource-name-that-might-be-generated-by-system"
		for i := 0; i < 10; i++ {
			longURL += "/nested/path/segment"
		}
		longURL += "?query=parameter&another=value"

		rr.APIHref = longURL
		rr.ConsoleHref = &longURL

		err := ValidateReporterRepresentation(rr)
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

		// Compare key fields
		AssertEqual(t, original.LocalResourceID, unmarshaled.LocalResourceID, "LocalResourceID should match after JSON round-trip")
		AssertEqual(t, original.ReporterType, unmarshaled.ReporterType, "ReporterType should match after JSON round-trip")
		AssertEqual(t, original.ResourceType, unmarshaled.ResourceType, "ResourceType should match after JSON round-trip")
		AssertEqual(t, original.Version, unmarshaled.Version, "Version should match after JSON round-trip")
		AssertEqual(t, original.Generation, unmarshaled.Generation, "Generation should match after JSON round-trip")
		AssertEqual(t, original.Tombstone, unmarshaled.Tombstone, "Tombstone should match after JSON round-trip")
	})

	t.Run("should handle null values in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.ConsoleHref = nil
		rr.ReporterVersion = nil

		// Test JSON marshaling with null values
		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with null values to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with null values from JSON")

		// Check that null values are preserved
		if unmarshaled.ConsoleHref != nil {
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
		rr.ConsoleHref = &emptyString
		rr.ReporterVersion = &emptyString

		// Test JSON marshaling with empty string values
		jsonData, err := json.Marshal(rr)
		AssertNoError(t, err, "Should be able to marshal ReporterRepresentation with empty string values to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal ReporterRepresentation with empty string values from JSON")

		// Check that empty string values are preserved
		if unmarshaled.ConsoleHref == nil || *unmarshaled.ConsoleHref != "" {
			t.Error("ConsoleHref should be empty string after JSON round-trip")
		}
		if unmarshaled.ReporterVersion == nil || *unmarshaled.ReporterVersion != "" {
			t.Error("ReporterVersion should be empty string after JSON round-trip")
		}
	})

	t.Run("should handle unicode characters in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.LocalResourceID = "测试-resource-🌟"
		rr.ReporterType = "测试-reporter"
		rr.Data = JsonObject{
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
		AssertEqual(t, rr.LocalResourceID, unmarshaled.LocalResourceID, "Unicode LocalResourceID should match after JSON round-trip")
		AssertEqual(t, rr.ReporterType, unmarshaled.ReporterType, "Unicode ReporterType should match after JSON round-trip")

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
		rr.LocalResourceID = "resource-with-special-chars-!@#$%^&*()"
		rr.Data = JsonObject{
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
		AssertEqual(t, rr.LocalResourceID, unmarshaled.LocalResourceID, "Special character LocalResourceID should match after JSON round-trip")

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

		complexData := JsonObject{
			"metadata": JsonObject{
				"labels": JsonObject{
					"app":         "test-app",
					"environment": "staging",
				},
				"annotations": JsonObject{
					"deployment.kubernetes.io/revision": "1",
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
		rr.CommonVersion = 4294967295

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
		AssertEqual(t, uint(4294967295), unmarshaled.CommonVersion, "Large CommonVersion should match after JSON round-trip")
	})

	t.Run("should handle empty data object in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		rr := fixture.ValidReporterRepresentation()
		rr.Data = JsonObject{}

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
}
