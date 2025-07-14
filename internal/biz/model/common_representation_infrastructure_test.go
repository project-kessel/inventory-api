package model

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

// Helper function to check if a CommonRepresentation is valid
// This is used in infrastructure tests that need to verify existing objects
func isValidCommonRepresentation(cr *CommonRepresentation) bool {
	return cr != nil && cr.ResourceId != uuid.Nil && cr.ResourceType != "" && cr.Version > 0 && cr.ReportedByReporterType != "" && cr.ReportedByReporterInstance != ""
}

// Helper function to check if a ReporterRepresentation is valid
// This is used in tests that need to verify existing objects
// Infrastructure tests for CommonRepresentation focus on:
// - Database schema validation (table names, GORM tags)
// - Field structure and types
// - Edge cases and boundary conditions
// - Data serialization and deserialization
// - Infrastructure-level constraints and validation

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
		AssertFieldType(t, cr, "ResourceType", reflect.TypeOf(""))
		AssertFieldType(t, cr, "ReportedByReporterType", reflect.TypeOf(""))
		AssertFieldType(t, cr, "ReportedByReporterInstance", reflect.TypeOf(""))
		AssertFieldType(t, cr, "Data", reflect.TypeOf(JsonObject{}))
	})

	t.Run("should have correct GORM tags for primary key", func(t *testing.T) {
		t.Parallel()

		cr := &CommonRepresentation{}

		// Check primary key fields have correct GORM tags
		AssertGORMTag(t, cr, "ResourceId", "type:text;column:id;primaryKey")
		AssertGORMTag(t, cr, "Version", "type:bigint;column:version;primaryKey;check:version >= 0")
	})

	t.Run("should have correct GORM size constraints", func(t *testing.T) {
		t.Parallel()

		cr := &CommonRepresentation{}

		// Verify size constraints match constants
		AssertGORMTag(t, cr, "ResourceType", "size:128;column:resource_type")
		AssertGORMTag(t, cr, "ReportedByReporterType", "size:128;column:reported_by_reporter_type")
		AssertGORMTag(t, cr, "ReportedByReporterInstance", "size:128;column:reported_by_reporter_instance")
	})

	t.Run("should have correct non-nullable field types", func(t *testing.T) {
		t.Parallel()

		cr := &CommonRepresentation{}

		// All fields in CommonRepresentation should be non-nullable
		AssertFieldType(t, cr, "ResourceId", reflect.TypeOf(uuid.UUID{}))
		AssertFieldType(t, cr, "Version", reflect.TypeOf(uint(0)))
		AssertFieldType(t, cr, "ResourceType", reflect.TypeOf(""))
		AssertFieldType(t, cr, "ReportedByReporterType", reflect.TypeOf(""))
		AssertFieldType(t, cr, "ReportedByReporterInstance", reflect.TypeOf(""))
		AssertFieldType(t, cr, "Data", reflect.TypeOf(JsonObject{}))
	})
}

func TestCommonRepresentation_Infrastructure_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle unicode characters", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.UnicodeCommonRepresentation()

		// Factory method should create a valid instance with unicode characters
		AssertEqual(t, "测试-resource-type", cr.ResourceType, "Unicode resource type should be preserved")
	})

	t.Run("should handle special characters in string fields", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.SpecialCharsCommonRepresentation()

		// Factory method should create a valid instance with special characters
		AssertEqual(t, "special-!@#$%^&*()-type", cr.ResourceType, "Special characters in resource type should be preserved")
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

		cr.ResourceType = maxLen128
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

		cr.Data = complexData

		if !isValidCommonRepresentation(cr) {
			t.Error("CommonRepresentation with complex nested JSON should be valid")
		}
	})

	t.Run("should handle empty JSON object", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.ValidCommonRepresentation()
		cr.Data = JsonObject{}

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
		AssertEqual(t, original.ResourceType, unmarshaled.ResourceType, "ResourceType should match after JSON round-trip")
		AssertEqual(t, original.ReportedByReporterType, unmarshaled.ReportedByReporterType, "ReportedByReporterType should match after JSON round-trip")
		AssertEqual(t, original.ReportedByReporterInstance, unmarshaled.ReportedByReporterInstance, "ReportedByReporterInstance should match after JSON round-trip")
	})

	t.Run("should handle empty string values in JSON serialization", func(t *testing.T) {
		t.Parallel()

		fixture := NewTestFixture(t)
		cr := fixture.ValidCommonRepresentation()
		cr.Data = JsonObject{"empty_field": ""}

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

		complexData := JsonObject{
			"nested": JsonObject{
				"array": []interface{}{
					JsonObject{"key": "value1"},
					JsonObject{"key": "value2"},
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
		cr.Data = JsonObject{}

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
			"host",
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
