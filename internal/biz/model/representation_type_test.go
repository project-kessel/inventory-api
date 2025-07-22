package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Tests for RepresentationType
// This struct contains ResourceType and ReporterType fields

func TestRepresentationType_Structure(t *testing.T) {
	t.Parallel()

	t.Run("should have all required fields with correct types", func(t *testing.T) {
		t.Parallel()

		repType := RepresentationType{}
		repTypeType := reflect.TypeOf(repType)

		expectedFields := map[string]reflect.Type{
			"ResourceType": reflect.TypeOf(""),
			"ReporterType": reflect.TypeOf(""),
		}

		for fieldName, expectedType := range expectedFields {
			field, found := repTypeType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}
			if field.Type != expectedType {
				t.Errorf("Field %s has type %v, expected %v", fieldName, field.Type, expectedType)
			}
		}
	})

	t.Run("should have correct GORM size constraints", func(t *testing.T) {
		t.Parallel()

		repTypeType := reflect.TypeOf(RepresentationType{})

		sizeConstraints := map[string]string{
			"ResourceType": "size:128",
			"ReporterType": "size:128",
		}

		for fieldName, expectedSize := range sizeConstraints {
			field, found := repTypeType.FieldByName(fieldName)
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

	t.Run("should have correct GORM index constraints", func(t *testing.T) {
		t.Parallel()

		repTypeType := reflect.TypeOf(RepresentationType{})

		// Both fields should have unique index tag
		expectedIndexFields := []string{"ResourceType", "ReporterType"}

		for _, fieldName := range expectedIndexFields {
			field, found := repTypeType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}

			gormTag := field.Tag.Get("gorm")
			if !strings.Contains(gormTag, "index:reporter_rep_unique_idx,unique") {
				t.Errorf("Field %s should have unique index tag, got: %s", fieldName, gormTag)
			}
		}
	})

	t.Run("should have correct non-nullable field types", func(t *testing.T) {
		t.Parallel()

		repTypeType := reflect.TypeOf(RepresentationType{})

		stringFields := []string{"ResourceType", "ReporterType"}

		for _, fieldName := range stringFields {
			field, found := repTypeType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}

			if field.Type.Kind() != reflect.String {
				t.Errorf("Field %s should be a string type, got: %v", fieldName, field.Type)
			}
		}
	})
}

func TestRepresentationType_Validation(t *testing.T) {
	t.Parallel()

	t.Run("should validate through ReporterRepresentation factory", func(t *testing.T) {
		t.Parallel()

		// Test valid RepresentationType through factory method
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
			false,
			nil,
		)
		AssertNoError(t, err, "Valid RepresentationType should be created successfully")

		// Verify RepresentationType in metadata
		if rr.Metadata == nil {
			t.Error("Metadata should not be nil")
			return
		}

		AssertEqual(t, "host", rr.Metadata.ResourceType, "ResourceType should match in metadata")
		AssertEqual(t, "hbi", rr.Metadata.ReporterType, "ReporterType should match in metadata")

		// Verify RepresentationType in representation (not persisted)
		AssertEqual(t, "host", rr.RepresentationType.ResourceType, "ResourceType should match in representation")
		AssertEqual(t, "hbi", rr.RepresentationType.ReporterType, "ReporterType should match in representation")
	})

	t.Run("should handle different resource types", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name         string
			resourceType string
			reporterType string
		}{
			{"host_hbi", "host", "hbi"},
			{"k8s_cluster_acm", "k8s_cluster", "acm"},
			{"k8s_cluster_acs", "k8s_cluster", "acs"},
			{"k8s_cluster_ocm", "k8s_cluster", "ocm"},
			{"k8s_policy_acm", "k8s_policy", "acm"},
			{"notifications_integration_notifications", "notifications_integration", "notifications"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				rr, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					"test-local-id",
					tc.reporterType,
					tc.resourceType,
					1,
					"test-instance",
					1,
					"https://api.example.com",
					nil,
					1,
					false,
					nil,
				)
				AssertNoError(t, err, "Valid resource/reporter type combination should be accepted")

				AssertEqual(t, tc.resourceType, rr.Metadata.ResourceType, "ResourceType should match")
				AssertEqual(t, tc.reporterType, rr.Metadata.ReporterType, "ReporterType should match")
				AssertEqual(t, tc.resourceType, rr.RepresentationType.ResourceType, "ResourceType should match in representation")
				AssertEqual(t, tc.reporterType, rr.RepresentationType.ReporterType, "ReporterType should match in representation")
			})
		}
	})

	t.Run("should reject empty resource type", func(t *testing.T) {
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
		AssertValidationError(t, err, "ResourceType", "Empty ResourceType should be rejected")
	})

	t.Run("should reject empty reporter type", func(t *testing.T) {
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
		AssertValidationError(t, err, "ReporterType", "Empty ReporterType should be rejected")
	})
}

func TestRepresentationType_Serialization(t *testing.T) {
	t.Parallel()

	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		t.Parallel()

		repType := RepresentationType{
			ResourceType: "host",
			ReporterType: "hbi",
		}

		// Test JSON marshaling
		jsonData, err := json.Marshal(repType)
		AssertNoError(t, err, "Should be able to marshal RepresentationType to JSON")

		// Test JSON unmarshaling
		var unmarshaled RepresentationType
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal RepresentationType from JSON")

		// Compare fields
		AssertEqual(t, repType.ResourceType, unmarshaled.ResourceType, "ResourceType should match")
		AssertEqual(t, repType.ReporterType, unmarshaled.ReporterType, "ReporterType should match")
	})

	t.Run("should handle empty string values in JSON", func(t *testing.T) {
		t.Parallel()

		repType := RepresentationType{
			ResourceType: "",
			ReporterType: "",
		}

		jsonData, err := json.Marshal(repType)
		AssertNoError(t, err, "Should be able to marshal RepresentationType with empty strings to JSON")

		var unmarshaled RepresentationType
		err = json.Unmarshal(jsonData, &unmarshaled)
		AssertNoError(t, err, "Should be able to unmarshal RepresentationType with empty strings from JSON")

		AssertEqual(t, "", unmarshaled.ResourceType, "Empty ResourceType should be preserved")
		AssertEqual(t, "", unmarshaled.ReporterType, "Empty ReporterType should be preserved")
	})
}

func TestRepresentationType_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle unicode characters", func(t *testing.T) {
		t.Parallel()

		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"测试-reporter-🌟",
			"测试-resource-type",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Unicode characters should be valid")

		AssertEqual(t, "测试-resource-type", rr.Metadata.ResourceType, "Unicode ResourceType should match")
		AssertEqual(t, "测试-reporter-🌟", rr.Metadata.ReporterType, "Unicode ReporterType should match")
	})

	t.Run("should handle special characters", func(t *testing.T) {
		t.Parallel()

		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"reporter-type-with-special-chars-!@#$%^&*()",
			"resource-type-with-special-chars-!@#$%^&*()",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Special characters should be valid")

		AssertEqual(t, "resource-type-with-special-chars-!@#$%^&*()", rr.Metadata.ResourceType, "Special character ResourceType should match")
		AssertEqual(t, "reporter-type-with-special-chars-!@#$%^&*()", rr.Metadata.ReporterType, "Special character ReporterType should match")
	})

	t.Run("should handle long type names", func(t *testing.T) {
		t.Parallel()

		longResourceType := "very-long-resource-type-name-that-might-be-generated-by-system-with-many-segments"
		longReporterType := "very-long-reporter-type-name-that-might-be-generated-by-system-with-many-segments"

		rr, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			longReporterType,
			longResourceType,
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil,
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Long type names should be valid")

		AssertEqual(t, longResourceType, rr.Metadata.ResourceType, "Long ResourceType should match")
		AssertEqual(t, longReporterType, rr.Metadata.ReporterType, "Long ReporterType should match")
	})
}
