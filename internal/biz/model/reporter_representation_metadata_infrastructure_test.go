package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Infrastructure tests for ReporterRepresentationMetadata
// These tests focus on database schema, field structure validation, edge cases, and serialization

func TestReporterRepresentationMetadata_Structure(t *testing.T) {
	t.Parallel()

	t.Run("should have all required fields with correct types", func(t *testing.T) {
		t.Parallel()

		metadata := ReporterRepresentationMetadata{}
		metadataType := reflect.TypeOf(metadata)

		expectedFields := map[string]reflect.Type{
			"ReporterRepresentationKey": reflect.TypeOf(ReporterRepresentationKey{}),
			"APIHref":                   reflect.TypeOf(""),
			"ConsoleHref":               reflect.TypeOf((*string)(nil)),
		}

		for fieldName, expectedType := range expectedFields {
			field, found := metadataType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}
			if field.Type != expectedType {
				t.Errorf("Field %s has type %v, expected %v", fieldName, field.Type, expectedType)
			}
		}
	})

	t.Run("should embed ReporterRepresentationKey", func(t *testing.T) {
		t.Parallel()

		metadata := ReporterRepresentationMetadata{}
		metadataType := reflect.TypeOf(metadata)

		field, found := metadataType.FieldByName("ReporterRepresentationKey")
		if !found {
			t.Error("ReporterRepresentationMetadata should embed ReporterRepresentationKey")
			return
		}

		if !field.Anonymous {
			t.Error("ReporterRepresentationKey should be anonymously embedded")
		}
	})

	t.Run("should have correct GORM size constraints", func(t *testing.T) {
		t.Parallel()

		metadataType := reflect.TypeOf(ReporterRepresentationMetadata{})

		sizeConstraints := map[string]string{
			"APIHref":     "size:512",
			"ConsoleHref": "size:512",
		}

		for fieldName, expectedSize := range sizeConstraints {
			field, found := metadataType.FieldByName(fieldName)
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

		metadataType := reflect.TypeOf(ReporterRepresentationMetadata{})

		// ConsoleHref should be nullable
		field, found := metadataType.FieldByName("ConsoleHref")
		if !found {
			t.Error("ConsoleHref field not found")
			return
		}

		if field.Type.Kind() != reflect.Ptr {
			t.Error("ConsoleHref should be a pointer type for nullable field")
		}

		if field.Type.Elem().Kind() != reflect.String {
			t.Error("ConsoleHref should be a pointer to string")
		}
	})

	t.Run("should have correct non-nullable field types", func(t *testing.T) {
		t.Parallel()

		metadataType := reflect.TypeOf(ReporterRepresentationMetadata{})

		// APIHref should be non-nullable string
		field, found := metadataType.FieldByName("APIHref")
		if !found {
			t.Error("APIHref field not found")
			return
		}

		if field.Type.Kind() != reflect.String {
			t.Errorf("APIHref should be a string type, got: %v", field.Type)
		}
	})

	t.Run("should have correct column names in GORM tags", func(t *testing.T) {
		t.Parallel()

		metadataType := reflect.TypeOf(ReporterRepresentationMetadata{})

		columnMappings := map[string]string{
			"APIHref":     "api_href",
			"ConsoleHref": "console_href",
		}

		for fieldName, expectedColumn := range columnMappings {
			field, found := metadataType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}

			gormTag := field.Tag.Get("gorm")
			expectedColumnTag := "column:" + expectedColumn
			if !strings.Contains(gormTag, expectedColumnTag) {
				t.Errorf("Field %s should have %s column tag, got: %s", fieldName, expectedColumnTag, gormTag)
			}
		}
	})
}

func TestReporterRepresentationMetadata_Serialization(t *testing.T) {
	t.Parallel()

	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		t.Parallel()

		metadata := ReporterRepresentationMetadata{
			ReporterRepresentationKey: ReporterRepresentationKey{
				RepresentationType: RepresentationType{
					ResourceType: "host",
					ReporterType: "hbi",
				},
				LocalResourceID:    "test-local-id",
				ReporterInstanceID: "test-instance",
			},
			APIHref:     "https://api.example.com",
			ConsoleHref: stringPtr("https://console.example.com"),
		}

		// Test JSON marshaling
		jsonData, err := json.Marshal(metadata)
		AssertNoError(t, err, "Should be able to marshal metadata to JSON")

		// Test JSON unmarshaling
		var unmarshaled ReporterRepresentationMetadata
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal metadata from JSON")

		// Compare fields
		AssertEqual(t, metadata.LocalResourceID, unmarshaled.LocalResourceID, "LocalResourceID should match")
		AssertEqual(t, metadata.ReporterType, unmarshaled.ReporterType, "ReporterType should match")
		AssertEqual(t, metadata.ResourceType, unmarshaled.ResourceType, "ResourceType should match")
		AssertEqual(t, metadata.ReporterInstanceID, unmarshaled.ReporterInstanceID, "ReporterInstanceID should match")
		AssertEqual(t, metadata.APIHref, unmarshaled.APIHref, "APIHref should match")
		AssertEqual(t, *metadata.ConsoleHref, *unmarshaled.ConsoleHref, "ConsoleHref should match")
	})

	t.Run("should handle nil ConsoleHref in JSON", func(t *testing.T) {
		t.Parallel()

		metadata := ReporterRepresentationMetadata{
			ReporterRepresentationKey: ReporterRepresentationKey{
				RepresentationType: RepresentationType{
					ResourceType: "host",
					ReporterType: "hbi",
				},
				LocalResourceID:    "test-local-id",
				ReporterInstanceID: "test-instance",
			},
			APIHref:     "https://api.example.com",
			ConsoleHref: nil,
		}

		jsonData, err := json.Marshal(metadata)
		AssertNoError(t, err, "Should be able to marshal metadata with nil ConsoleHref to JSON")

		var unmarshaled ReporterRepresentationMetadata
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal metadata with nil ConsoleHref from JSON")

		if unmarshaled.ConsoleHref != nil {
			t.Error("ConsoleHref should be nil after JSON round-trip")
		}
	})

	t.Run("should handle empty string ConsoleHref in JSON", func(t *testing.T) {
		t.Parallel()

		metadata := ReporterRepresentationMetadata{
			ReporterRepresentationKey: ReporterRepresentationKey{
				RepresentationType: RepresentationType{
					ResourceType: "host",
					ReporterType: "hbi",
				},
				LocalResourceID:    "test-local-id",
				ReporterInstanceID: "test-instance",
			},
			APIHref:     "https://api.example.com",
			ConsoleHref: stringPtr(""),
		}

		jsonData, err := json.Marshal(metadata)
		AssertNoError(t, err, "Should be able to marshal metadata with empty ConsoleHref to JSON")

		var unmarshaled ReporterRepresentationMetadata
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal metadata with empty ConsoleHref from JSON")

		if unmarshaled.ConsoleHref == nil || *unmarshaled.ConsoleHref != "" {
			t.Error("ConsoleHref should be empty string after JSON round-trip")
		}
	})

	t.Run("should handle complex nested structure in JSON", func(t *testing.T) {
		t.Parallel()

		metadata := ReporterRepresentationMetadata{
			ReporterRepresentationKey: ReporterRepresentationKey{
				RepresentationType: RepresentationType{
					ResourceType: "k8s_cluster",
					ReporterType: "acm",
				},
				LocalResourceID:    "cluster-123-with-special-chars-!@#$%^&*()",
				ReporterInstanceID: "instance-456-with-unicode-测试-🌟",
			},
			APIHref:     "https://api.example.com/v1/clusters/cluster-123-with-special-chars-!@#$%^&*()",
			ConsoleHref: stringPtr("https://console.example.com/clusters/cluster-123-with-special-chars-!@#$%^&*()"),
		}

		jsonData, err := json.Marshal(metadata)
		AssertNoError(t, err, "Should be able to marshal complex metadata to JSON")

		var unmarshaled ReporterRepresentationMetadata
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal complex metadata from JSON")

		// Verify all fields are preserved
		AssertEqual(t, metadata.LocalResourceID, unmarshaled.LocalResourceID, "Complex LocalResourceID should match")
		AssertEqual(t, metadata.ReporterInstanceID, unmarshaled.ReporterInstanceID, "Complex ReporterInstanceID should match")
		AssertEqual(t, metadata.ResourceType, unmarshaled.ResourceType, "ResourceType should match")
		AssertEqual(t, metadata.ReporterType, unmarshaled.ReporterType, "ReporterType should match")
		AssertEqual(t, metadata.APIHref, unmarshaled.APIHref, "Complex APIHref should match")
		AssertEqual(t, *metadata.ConsoleHref, *unmarshaled.ConsoleHref, "Complex ConsoleHref should match")
	})
}

func TestReporterRepresentationMetadata_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle maximum length values", func(t *testing.T) {
		t.Parallel()

		// Create maximum length strings based on GORM constraints
		maxLocalResourceID := strings.Repeat("a", 128)
		maxReporterType := strings.Repeat("b", 128)
		maxResourceType := strings.Repeat("c", 128)
		maxReporterInstanceID := strings.Repeat("d", 128)
		maxAPIHref := "https://api.example.com/" + strings.Repeat("e", 512-26)         // Account for URL prefix
		maxConsoleHref := "https://console.example.com/" + strings.Repeat("f", 512-30) // Account for URL prefix

		metadata := ReporterRepresentationMetadata{
			ReporterRepresentationKey: ReporterRepresentationKey{
				RepresentationType: RepresentationType{
					ResourceType: maxResourceType,
					ReporterType: maxReporterType,
				},
				LocalResourceID:    maxLocalResourceID,
				ReporterInstanceID: maxReporterInstanceID,
			},
			APIHref:     maxAPIHref,
			ConsoleHref: stringPtr(maxConsoleHref),
		}

		jsonData, err := json.Marshal(metadata)
		AssertNoError(t, err, "Should be able to marshal metadata with maximum length values to JSON")

		var unmarshaled ReporterRepresentationMetadata
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal metadata with maximum length values from JSON")

		// Verify all maximum length fields are preserved
		AssertEqual(t, maxLocalResourceID, unmarshaled.LocalResourceID, "Max LocalResourceID should match")
		AssertEqual(t, maxReporterType, unmarshaled.ReporterType, "Max ReporterType should match")
		AssertEqual(t, maxResourceType, unmarshaled.ResourceType, "Max ResourceType should match")
		AssertEqual(t, maxReporterInstanceID, unmarshaled.ReporterInstanceID, "Max ReporterInstanceID should match")
		AssertEqual(t, maxAPIHref, unmarshaled.APIHref, "Max APIHref should match")
		AssertEqual(t, maxConsoleHref, *unmarshaled.ConsoleHref, "Max ConsoleHref should match")
	})

	t.Run("should handle minimum valid values", func(t *testing.T) {
		t.Parallel()

		metadata := ReporterRepresentationMetadata{
			ReporterRepresentationKey: ReporterRepresentationKey{
				RepresentationType: RepresentationType{
					ResourceType: "a",
					ReporterType: "b",
				},
				LocalResourceID:    "c",
				ReporterInstanceID: "d",
			},
			APIHref:     "https://a.b",
			ConsoleHref: stringPtr("https://c.d"),
		}

		jsonData, err := json.Marshal(metadata)
		AssertNoError(t, err, "Should be able to marshal metadata with minimum values to JSON")

		var unmarshaled ReporterRepresentationMetadata
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal metadata with minimum values from JSON")

		// Verify all minimum fields are preserved
		AssertEqual(t, "a", unmarshaled.ResourceType, "Min ResourceType should match")
		AssertEqual(t, "b", unmarshaled.ReporterType, "Min ReporterType should match")
		AssertEqual(t, "c", unmarshaled.LocalResourceID, "Min LocalResourceID should match")
		AssertEqual(t, "d", unmarshaled.ReporterInstanceID, "Min ReporterInstanceID should match")
		AssertEqual(t, "https://a.b", unmarshaled.APIHref, "Min APIHref should match")
		AssertEqual(t, "https://c.d", *unmarshaled.ConsoleHref, "Min ConsoleHref should match")
	})

	t.Run("should handle special characters in field values", func(t *testing.T) {
		t.Parallel()

		specialChars := "!@#$%^&*()_+-=[]{}|;':\",./<>?"
		metadata := ReporterRepresentationMetadata{
			ReporterRepresentationKey: ReporterRepresentationKey{
				RepresentationType: RepresentationType{
					ResourceType: "resource-" + specialChars,
					ReporterType: "reporter-" + specialChars,
				},
				LocalResourceID:    "local-" + specialChars,
				ReporterInstanceID: "instance-" + specialChars,
			},
			APIHref:     "https://api.example.com/resource-" + specialChars,
			ConsoleHref: stringPtr("https://console.example.com/resource-" + specialChars),
		}

		jsonData, err := json.Marshal(metadata)
		AssertNoError(t, err, "Should be able to marshal metadata with special characters to JSON")

		var unmarshaled ReporterRepresentationMetadata
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal metadata with special characters from JSON")

		// Verify all special character fields are preserved
		AssertEqual(t, "resource-"+specialChars, unmarshaled.ResourceType, "Special char ResourceType should match")
		AssertEqual(t, "reporter-"+specialChars, unmarshaled.ReporterType, "Special char ReporterType should match")
		AssertEqual(t, "local-"+specialChars, unmarshaled.LocalResourceID, "Special char LocalResourceID should match")
		AssertEqual(t, "instance-"+specialChars, unmarshaled.ReporterInstanceID, "Special char ReporterInstanceID should match")
		AssertEqual(t, "https://api.example.com/resource-"+specialChars, unmarshaled.APIHref, "Special char APIHref should match")
		AssertEqual(t, "https://console.example.com/resource-"+specialChars, *unmarshaled.ConsoleHref, "Special char ConsoleHref should match")
	})

	t.Run("should handle zero values", func(t *testing.T) {
		t.Parallel()

		// Test with zero values (empty struct)
		metadata := ReporterRepresentationMetadata{}

		jsonData, err := json.Marshal(metadata)
		AssertNoError(t, err, "Should be able to marshal zero-value metadata to JSON")

		var unmarshaled ReporterRepresentationMetadata
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal zero-value metadata from JSON")

		// Verify zero values are preserved
		AssertEqual(t, "", unmarshaled.ResourceType, "Zero ResourceType should match")
		AssertEqual(t, "", unmarshaled.ReporterType, "Zero ReporterType should match")
		AssertEqual(t, "", unmarshaled.LocalResourceID, "Zero LocalResourceID should match")
		AssertEqual(t, "", unmarshaled.ReporterInstanceID, "Zero ReporterInstanceID should match")
		AssertEqual(t, "", unmarshaled.APIHref, "Zero APIHref should match")
		if unmarshaled.ConsoleHref != nil {
			t.Error("Zero ConsoleHref should be nil")
		}
	})
}
