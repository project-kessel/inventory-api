package model

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"

	"reflect"
	"testing"
)

// Helper function to check if a CommonRepresentation is valid
// This is used in infrastructure tests that need to verify existing objects
func isValidCommonRepresentation(cr *CommonRepresentation) bool {
	return cr != nil && cr.ResourceId != uuid.Nil && cr.Version > 0 && cr.ReportedByReporterType != "" && cr.ReportedByReporterInstance != ""
}

func TestCommonRepresentation_Infrastructure_Structure(t *testing.T) {
	t.Parallel()

	t.Run("should properly embed Representation", func(t *testing.T) {
		t.Parallel()

		cr := &CommonRepresentation{}

		// Check if CommonRepresentation embeds Representation
		crType := reflect.TypeOf(cr).Elem()
		field, found := crType.FieldByName("Representation")
		if !found {
			t.Error("CommonRepresentation should embed Representation")
			return
		}

		if field.Type != reflect.TypeOf(Representation{}) {
			t.Errorf("Expected Representation type, got %v", field.Type)
		}

		// Verify anonymous embedding
		if !field.Anonymous {
			t.Error("Representation should be anonymously embedded")
		}
	})

	t.Run("should have all required fields with correct types", func(t *testing.T) {
		t.Parallel()

		cr := &CommonRepresentation{}

		// Test field types
		AssertFieldType(t, cr, "ResourceId", reflect.TypeOf(uuid.UUID{}))
		AssertFieldType(t, cr, "Version", reflect.TypeOf(uint(0)))
		AssertFieldType(t, cr, "ReportedByReporterType", reflect.TypeOf(""))
		AssertFieldType(t, cr, "ReportedByReporterInstance", reflect.TypeOf(""))
		AssertFieldType(t, cr, "Data", reflect.TypeOf(internal.JsonObject{}))
	})

	t.Run("should have correct GORM tags for primary key", func(t *testing.T) {
		t.Parallel()

		cr := &CommonRepresentation{}

		// Check primary key fields have correct GORM tags
		AssertGORMTag(t, cr, "ResourceId", "type:text;primaryKey")
		AssertGORMTag(t, cr, "Version", "type:bigint;primaryKey;check:version >= 0")
	})

	t.Run("should have correct GORM size constraints", func(t *testing.T) {
		t.Parallel()

		cr := &CommonRepresentation{}

		// Verify size constraints match constants
		AssertGORMTag(t, cr, "ReportedByReporterType", "size:128")
		AssertGORMTag(t, cr, "ReportedByReporterInstance", "size:128")
	})

	t.Run("should have correct non-nullable field types", func(t *testing.T) {
		t.Parallel()

		cr := &CommonRepresentation{}

		// All fields in CommonRepresentation should be non-nullable
		AssertFieldType(t, cr, "ResourceId", reflect.TypeOf(uuid.UUID{}))
		AssertFieldType(t, cr, "Version", reflect.TypeOf(uint(0)))
		AssertFieldType(t, cr, "ReportedByReporterType", reflect.TypeOf(""))
		AssertFieldType(t, cr, "ReportedByReporterInstance", reflect.TypeOf(""))
		AssertFieldType(t, cr, "Data", reflect.TypeOf(internal.JsonObject{}))
	})
}

func TestCommonRepresentation_Infrastructure_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle unicode characters", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.UnicodeCommonRepresentation()

		// Factory method should create a valid instance with unicode characters
		AssertEqual(t, "测试-reporter", cr.ReportedByReporterType, "Unicode reporter type should be preserved")
	})

	t.Run("should handle special characters in string fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.SpecialCharsCommonRepresentation()

		// Factory method should create a valid instance with special characters
		AssertEqual(t, "special-†‡•-reporter", cr.ReportedByReporterType, "Special characters in reporter type should be preserved")
	})

	t.Run("should handle large integer values", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.MaximalCommonRepresentation()

		// Factory method should create a valid instance with large integer values
		AssertEqual(t, uint(4294967295), cr.Version, "Large integer version should be preserved")
	})

	t.Run("should handle maximum length string values", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.ValidCommonRepresentation()

		// Create strings at maximum allowed length
		maxLen128 := ""
		for i := 0; i < 128; i++ {
			maxLen128 += "a"
		}

		cr.ReportedByReporterType = maxLen128
		cr.ReportedByReporterInstance = maxLen128

		if !isValidCommonRepresentation(cr) {
			t.Error("CommonRepresentation with maximum length string values should be valid")
		}
	})

	t.Run("should handle complex nested JSON data", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.ValidCommonRepresentation()

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

		cr.Data = complexData

		if !isValidCommonRepresentation(cr) {
			t.Error("CommonRepresentation with complex nested JSON should be valid")
		}
	})

	t.Run("should handle empty JSON object", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.ValidCommonRepresentation()
		cr.Data = internal.JsonObject{}

		if !isValidCommonRepresentation(cr) {
			t.Error("CommonRepresentation with empty JSON object should be valid")
		}
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
				cr := fixture.ValidCommonRepresentation()
				cr.Version = tc.version

				isValid := isValidCommonRepresentation(cr)
				if tc.valid {
					if !isValid {
						t.Error("Version should be valid")
					}
				} else {
					if isValid {
						t.Error("Version should be invalid")
					}
				}
			})
		}
	})

	t.Run("should handle ResourceId boundary values", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)

		testCases := []struct {
			name       string
			resourceId uuid.UUID
			valid      bool
		}{
			{"valid_uuid", uuid.New(), true},
			{"nil_uuid", uuid.Nil, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				cr := fixture.ValidCommonRepresentation()
				cr.ResourceId = tc.resourceId

				isValid := isValidCommonRepresentation(cr)
				if tc.valid {
					if !isValid {
						t.Error("ResourceId should be valid")
					}
				} else {
					if isValid {
						t.Error("ResourceId should be invalid")
					}
				}
			})
		}
	})
}

func TestCommonRepresentation_Infrastructure_Serialization(t *testing.T) {
	t.Parallel()

	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		original := fixture.ValidCommonRepresentation()

		// Test JSON marshaling
		jsonData, err := json.Marshal(original)
		AssertNoError(t, err, "Should be able to marshal CommonRepresentation to JSON")

		// Test JSON unmarshaling
		var unmarshaled CommonRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal CommonRepresentation from JSON")

		// Compare key fields
		AssertEqual(t, original.ResourceId, unmarshaled.ResourceId, "ResourceId should match after JSON round-trip")
		AssertEqual(t, original.Version, unmarshaled.Version, "Version should match after JSON round-trip")
		AssertEqual(t, original.ReportedByReporterType, unmarshaled.ReportedByReporterType, "ReportedByReporterType should match after JSON round-trip")
		AssertEqual(t, original.ReportedByReporterInstance, unmarshaled.ReportedByReporterInstance, "ReportedByReporterInstance should match after JSON round-trip")
	})

	t.Run("should handle empty string values in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.ValidCommonRepresentation()
		cr.Data = internal.JsonObject{"empty_field": ""}

		jsonData, err := json.Marshal(cr)
		AssertNoError(t, err, "Should be able to marshal CommonRepresentation with empty string values")

		var unmarshaled CommonRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal CommonRepresentation with empty string values")

		AssertEqual(t, cr.Data["empty_field"], unmarshaled.Data["empty_field"], "Empty string values should be preserved")
	})

	t.Run("should handle unicode characters in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.UnicodeCommonRepresentation()

		jsonData, err := json.Marshal(cr)
		AssertNoError(t, err, "Should be able to marshal CommonRepresentation with unicode characters")

		var unmarshaled CommonRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal CommonRepresentation with unicode characters")

		AssertEqual(t, cr.ResourceId, unmarshaled.ResourceId, "Unicode characters should be preserved in ResourceId")
		AssertEqual(t, cr.ReportedByReporterType, unmarshaled.ReportedByReporterType, "Unicode characters should be preserved in ReportedByReporterType")
	})

	t.Run("should handle special characters in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.SpecialCharsCommonRepresentation()

		jsonData, err := json.Marshal(cr)
		AssertNoError(t, err, "Should be able to marshal CommonRepresentation with special characters")

		var unmarshaled CommonRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal CommonRepresentation with special characters")

		AssertEqual(t, cr.ResourceId, unmarshaled.ResourceId, "Special characters should be preserved in ResourceId")
		AssertEqual(t, cr.Data["special_field"], unmarshaled.Data["special_field"], "Special characters should be preserved in Data")
	})

	t.Run("should handle large integer values in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.ValidCommonRepresentation()
		cr.Version = 4294967295 // Max uint32

		jsonData, err := json.Marshal(cr)
		AssertNoError(t, err, "Should be able to marshal CommonRepresentation with large integer values")

		var unmarshaled CommonRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal CommonRepresentation with large integer values")

		AssertEqual(t, cr.Version, unmarshaled.Version, "Large integer values should be preserved")
	})

	t.Run("should handle complex nested JSON data serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.ValidCommonRepresentation()

		complexData := internal.JsonObject{
			"nested": internal.JsonObject{
				"array": []interface{}{
					internal.JsonObject{"key": "value1"},
					internal.JsonObject{"key": "value2"},
				},
				"number":  42,
				"boolean": true,
			},
		}
		cr.Data = complexData

		jsonData, err := json.Marshal(cr)
		AssertNoError(t, err, "Should be able to marshal CommonRepresentation with complex nested JSON")

		var unmarshaled CommonRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal CommonRepresentation with complex nested JSON")

		// Verify nested structure is preserved
		nested, ok := unmarshaled.Data["nested"].(map[string]interface{})
		if !ok {
			t.Error("Nested object should be preserved as map[string]interface{}")
			return
		}

		AssertEqual(t, float64(42), nested["number"], "Nested number should be preserved")
		AssertEqual(t, true, nested["boolean"], "Nested boolean should be preserved")
	})

	t.Run("should handle empty data object in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.ValidCommonRepresentation()
		cr.Data = internal.JsonObject{}

		jsonData, err := json.Marshal(cr)
		AssertNoError(t, err, "Should be able to marshal CommonRepresentation with empty data object")

		var unmarshaled CommonRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal CommonRepresentation with empty data object")

		if len(unmarshaled.Data) != 0 {
			t.Errorf("Expected empty data object, got %v", unmarshaled.Data)
		}
	})

	t.Run("should handle nil data in JSON serialization", func(t *testing.T) {
		t.Parallel()

		// Create a CommonRepresentation with nil data using factory method
		cr, err := NewCommonRepresentation(
			uuid.NewSHA1(uuid.NameSpaceOID, []byte("test")),
			nil, // nil Data should be valid
			1,
			"hbi",
			"test-instance",
		)
		AssertNoError(t, err, "Should be able to create CommonRepresentation with nil data")

		// Test JSON marshaling with nil data
		jsonData, err := json.Marshal(cr)
		AssertNoError(t, err, "Should be able to marshal CommonRepresentation with nil data to JSON")

		// Test JSON unmarshaling
		var unmarshaled CommonRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal CommonRepresentation with nil data from JSON")

		// Check that nil data is preserved as nil (not empty object)
		if unmarshaled.Data != nil {
			t.Errorf("Data should be nil after JSON round-trip, got: %v", unmarshaled.Data)
		}
	})

	t.Run("should handle UUID serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.ValidCommonRepresentation()
		originalUUID := uuid.New()
		cr.ResourceId = originalUUID

		jsonData, err := json.Marshal(cr)
		AssertNoError(t, err, "Should be able to marshal CommonRepresentation with UUID")

		var unmarshaled CommonRepresentation
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal CommonRepresentation with UUID")

		AssertEqual(t, originalUUID, unmarshaled.ResourceId, "UUID should be preserved during JSON round-trip")
	})
}
