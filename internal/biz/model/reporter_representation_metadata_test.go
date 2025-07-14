package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Tests for ReporterRepresentationMetadata
// This struct represents the metadata table with key fields and non-versioned data

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
}

func TestReporterRepresentationMetadata_Validation(t *testing.T) {
	t.Parallel()

	t.Run("should validate through ReporterRepresentation factory", func(t *testing.T) {
		t.Parallel()

		// Test valid metadata through factory method
		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			stringPtr("https://console.example.com"),
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Valid metadata should be created successfully")

		// Verify metadata structure
		if rr.Metadata == nil {
			t.Error("Metadata should not be nil")
			return
		}

		AssertEqual(t, "test-local-id", rr.Metadata.LocalResourceID, "LocalResourceID should match")
		AssertEqual(t, "hbi", rr.Metadata.ReporterType, "ReporterType should match")
		AssertEqual(t, "host", rr.Metadata.ResourceType, "ResourceType should match")
		AssertEqual(t, "test-instance", rr.Metadata.ReporterInstanceID, "ReporterInstanceID should match")
		AssertEqual(t, "https://api.example.com", rr.Metadata.APIHref, "APIHref should match")
		AssertEqual(t, "https://console.example.com", *rr.Metadata.ConsoleHref, "ConsoleHref should match")
	})

	t.Run("should handle nil ConsoleHref", func(t *testing.T) {
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
			nil, // nil ConsoleHref
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Nil ConsoleHref should be valid")

		if rr.Metadata.ConsoleHref != nil {
			t.Error("ConsoleHref should be nil")
		}
	})

	t.Run("should handle empty string ConsoleHref", func(t *testing.T) {
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
			stringPtr(""), // empty string ConsoleHref
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Empty string ConsoleHref should be valid")

		if rr.Metadata.ConsoleHref == nil || *rr.Metadata.ConsoleHref != "" {
			t.Error("ConsoleHref should be empty string")
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
}

func TestReporterRepresentationMetadata_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle long URL values", func(t *testing.T) {
		t.Parallel()

		// Create a long but valid URL
		longURL := "https://api.example.com/v1/resources/very-long-resource-name-that-might-be-generated-by-system"
		for i := 0; i < 10; i++ {
			longURL += "/nested/path/segment"
		}
		longURL += "?query=parameter&another=value"

		rr, err := NewReporterRepresentation(
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
		AssertNoError(t, err, "Long URL values should be valid")

		AssertEqual(t, longURL, rr.Metadata.APIHref, "Long APIHref should match")
		AssertEqual(t, longURL, *rr.Metadata.ConsoleHref, "Long ConsoleHref should match")
	})

	t.Run("should handle unicode characters in URLs", func(t *testing.T) {
		t.Parallel()

		unicodeURL := "https://api.example.com/资源/测试-🌟"

		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			unicodeURL,
			stringPtr(unicodeURL),
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Unicode URL values should be valid")

		AssertEqual(t, unicodeURL, rr.Metadata.APIHref, "Unicode APIHref should match")
		AssertEqual(t, unicodeURL, *rr.Metadata.ConsoleHref, "Unicode ConsoleHref should match")
	})
}
