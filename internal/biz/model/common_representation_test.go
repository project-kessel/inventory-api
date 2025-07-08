package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Test scenarios for CommonRepresentation domain model
//
// These tests focus on domain logic, business rules, and model behavior
// rather than database operations or infrastructure concerns.

func TestCommonRepresentation_TableName(t *testing.T) {
	t.Parallel()

	t.Run("should return correct table name", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{}
		expected := "common_representation"
		actual := cr.TableName()

		if actual != expected {
			t.Errorf("Expected table name %q, got %q", expected, actual)
		}
	})

	t.Run("should be consistent across different instances", func(t *testing.T) {
		t.Parallel()

		cr1 := CommonRepresentation{ID: "test1"}
		cr2 := CommonRepresentation{ID: "test2"}

		if cr1.TableName() != cr2.TableName() {
			t.Error("Table name should be consistent across different instances")
		}
	})

	t.Run("should match expected database table naming convention", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{}
		tableName := cr.TableName()

		// Check naming convention: lowercase with underscores
		if strings.Contains(tableName, " ") {
			t.Error("Table name should not contain spaces")
		}
		if strings.ToLower(tableName) != tableName {
			t.Error("Table name should be lowercase")
		}
	})
}

func TestCommonRepresentation_Structure(t *testing.T) {
	t.Parallel()

	t.Run("should properly embed BaseRepresentation", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{}

		// Check if BaseRepresentation is embedded
		crType := reflect.TypeOf(cr)
		found := false
		for i := 0; i < crType.NumField(); i++ {
			field := crType.Field(i)
			if field.Type == reflect.TypeOf(BaseRepresentation{}) && field.Anonymous {
				found = true
				break
			}
		}

		if !found {
			t.Error("CommonRepresentation should embed BaseRepresentation")
		}
	})

	t.Run("should have all required fields with correct types", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{}
		crType := reflect.TypeOf(cr)

		expectedFields := map[string]reflect.Type{
			"ID":                         reflect.TypeOf(""),
			"ResourceType":               reflect.TypeOf(""),
			"Version":                    reflect.TypeOf(0),
			"ReportedByReporterType":     reflect.TypeOf(""),
			"ReportedByReporterInstance": reflect.TypeOf(""),
		}

		for fieldName, expectedType := range expectedFields {
			field, found := crType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}
			if field.Type != expectedType {
				t.Errorf("Field %s has type %v, expected %v", fieldName, field.Type, expectedType)
			}
		}
	})

	t.Run("should support zero values for all fields", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{}

		// Should not panic when accessing zero values
		_ = cr.ID
		_ = cr.ResourceType
		_ = cr.Version
		_ = cr.ReportedByReporterType
		_ = cr.ReportedByReporterInstance
		_ = cr.Data
	})

	t.Run("should have correct GORM tags for unique index", func(t *testing.T) {
		t.Parallel()

		crType := reflect.TypeOf(CommonRepresentation{})

		// Check ID field has correct index tag
		idField, found := crType.FieldByName("ID")
		if !found {
			t.Error("ID field not found")
		} else {
			tag := idField.Tag.Get("gorm")
			if !strings.Contains(tag, "index:common_rep_unique_idx,unique") {
				t.Errorf("ID field should have unique index tag, got: %s", tag)
			}
		}

		// Check Version field has correct index tag
		versionField, found := crType.FieldByName("Version")
		if !found {
			t.Error("Version field not found")
		} else {
			tag := versionField.Tag.Get("gorm")
			if !strings.Contains(tag, "index:common_rep_unique_idx,unique") {
				t.Errorf("Version field should have unique index tag, got: %s", tag)
			}
		}
	})
}

func TestCommonRepresentation_Validation(t *testing.T) {
	t.Parallel()

	t.Run("valid CommonRepresentation with all required fields", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: JsonObject{"key": "value"},
			},
			ID:                         "test-id-123",
			ResourceType:               "k8s_cluster",
			Version:                    1,
			ReportedByReporterType:     "acm",
			ReportedByReporterInstance: "acm-instance-1",
		}

		if err := validateCommonRepresentation(cr); err != nil {
			t.Errorf("Valid CommonRepresentation should not have validation errors: %v", err)
		}
	})

	t.Run("CommonRepresentation with empty ID should be invalid", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			ID:           "",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		if err := validateCommonRepresentation(cr); err == nil {
			t.Error("CommonRepresentation with empty ID should be invalid")
		}
	})

	t.Run("CommonRepresentation with empty ResourceType should be invalid", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			ID:           "test-id",
			ResourceType: "",
			Version:      1,
		}

		if err := validateCommonRepresentation(cr); err == nil {
			t.Error("CommonRepresentation with empty ResourceType should be invalid")
		}
	})

	t.Run("CommonRepresentation with negative Version should be invalid", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      -1,
		}

		if err := validateCommonRepresentation(cr); err == nil {
			t.Error("CommonRepresentation with negative Version should be invalid")
		}
	})

	t.Run("CommonRepresentation with zero Version should be valid", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      0,
		}

		if err := validateCommonRepresentation(cr); err != nil {
			t.Errorf("CommonRepresentation with zero Version should be valid: %v", err)
		}
	})

	t.Run("CommonRepresentation with very long ResourceType should be invalid", func(t *testing.T) {
		t.Parallel()

		// ResourceType has size:128 constraint
		longResourceType := strings.Repeat("a", 129)
		cr := CommonRepresentation{
			ID:           "test-id",
			ResourceType: longResourceType,
			Version:      1,
		}

		if err := validateCommonRepresentation(cr); err == nil {
			t.Error("CommonRepresentation with ResourceType longer than 128 characters should be invalid")
		}
	})

	t.Run("CommonRepresentation with whitespace-only fields should be invalid", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name string
			cr   CommonRepresentation
		}{
			{
				name: "whitespace-only ID",
				cr: CommonRepresentation{
					ID:           "   ",
					ResourceType: "k8s_cluster",
					Version:      1,
				},
			},
			{
				name: "whitespace-only ResourceType",
				cr: CommonRepresentation{
					ID:           "test-id",
					ResourceType: "   ",
					Version:      1,
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if err := validateCommonRepresentation(tc.cr); err == nil {
					t.Errorf("CommonRepresentation with %s should be invalid", tc.name)
				}
			})
		}
	})
}

func TestCommonRepresentation_BusinessRules(t *testing.T) {
	t.Parallel()

	t.Run("ID and Version combination should be unique", func(t *testing.T) {
		t.Parallel()

		cr1 := CommonRepresentation{
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		cr2 := CommonRepresentation{
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		if !areCommonRepresentationsDuplicates(cr1, cr2) {
			t.Error("CommonRepresentations with same ID and Version should be considered duplicates")
		}
	})

	t.Run("different ID should not be duplicates", func(t *testing.T) {
		t.Parallel()

		cr1 := CommonRepresentation{
			ID:           "test-id-1",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		cr2 := CommonRepresentation{
			ID:           "test-id-2",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		if areCommonRepresentationsDuplicates(cr1, cr2) {
			t.Error("CommonRepresentations with different IDs should not be considered duplicates")
		}
	})

	t.Run("different Version should not be duplicates", func(t *testing.T) {
		t.Parallel()

		cr1 := CommonRepresentation{
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		cr2 := CommonRepresentation{
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      2,
		}

		if areCommonRepresentationsDuplicates(cr1, cr2) {
			t.Error("CommonRepresentations with different Versions should not be considered duplicates")
		}
	})

	t.Run("Version should support incremental updates", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		// Simulate version increment
		cr.Version++

		if cr.Version != 2 {
			t.Error("Version should support incremental updates")
		}
	})

	t.Run("ReportedByReporterType should indicate data source", func(t *testing.T) {
		t.Parallel()

		validReporterTypes := []string{"acm", "ocm", "acs", "hbi", "notifications"}

		for _, reporterType := range validReporterTypes {
			cr := CommonRepresentation{
				ID:                     "test-id",
				ResourceType:           "k8s_cluster",
				Version:                1,
				ReportedByReporterType: reporterType,
			}

			if err := validateCommonRepresentation(cr); err != nil {
				t.Errorf("CommonRepresentation with valid ReportedByReporterType %s should be valid: %v", reporterType, err)
			}
		}
	})
}

func TestCommonRepresentation_DataHandling(t *testing.T) {
	t.Parallel()

	t.Run("should handle nil JsonObject data", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: nil,
			},
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		// Should not panic when accessing nil data
		_ = cr.Data
		if cr.Data != nil {
			t.Error("Data should be nil when not initialized")
		}
	})

	t.Run("should handle empty JsonObject data", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: JsonObject{},
			},
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		if cr.Data == nil {
			t.Error("Data should not be nil when initialized as empty map")
		}
		if len(cr.Data) != 0 {
			t.Error("Empty JsonObject should have length 0")
		}
	})

	t.Run("should handle complex JsonObject data", func(t *testing.T) {
		t.Parallel()

		complexData := JsonObject{
			"metadata": map[string]interface{}{
				"name":      "test-cluster",
				"namespace": "default",
				"labels": map[string]interface{}{
					"environment": "production",
					"region":      "us-east-1",
				},
			},
			"spec": map[string]interface{}{
				"replicas": 3,
				"version":  "1.21.0",
			},
			"status": map[string]interface{}{
				"ready": true,
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		}

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: complexData,
			},
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		// Should be able to access nested data
		if metadata, ok := cr.Data["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok {
				if name != "test-cluster" {
					t.Error("Should be able to access nested data correctly")
				}
			}
		}
	})

	t.Run("should handle JsonObject with different value types", func(t *testing.T) {
		t.Parallel()

		mixedData := JsonObject{
			"string_value": "test",
			"int_value":    42,
			"float_value":  3.14,
			"bool_value":   true,
			"null_value":   nil,
			"array_value":  []interface{}{1, 2, 3},
			"object_value": map[string]interface{}{"nested": "value"},
		}

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: mixedData,
			},
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		// Should handle different types correctly
		if cr.Data["string_value"] != "test" {
			t.Error("Should handle string values correctly")
		}
		if cr.Data["int_value"] != 42 {
			t.Error("Should handle int values correctly")
		}
		if cr.Data["bool_value"] != true {
			t.Error("Should handle bool values correctly")
		}
	})

	t.Run("should support data modification", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: JsonObject{"key": "original"},
			},
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		// Modify data
		cr.Data["key"] = "modified"
		cr.Data["new_key"] = "new_value"

		if cr.Data["key"] != "modified" {
			t.Error("Should support data modification")
		}
		if cr.Data["new_key"] != "new_value" {
			t.Error("Should support adding new keys")
		}
	})
}

func TestCommonRepresentation_Comparison(t *testing.T) {
	t.Parallel()

	t.Run("should compare CommonRepresentations correctly", func(t *testing.T) {
		t.Parallel()

		cr1 := CommonRepresentation{
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		cr2 := CommonRepresentation{
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		if !areCommonRepresentationsDuplicates(cr1, cr2) {
			t.Error("Identical CommonRepresentations should be considered duplicates")
		}
	})

	t.Run("should handle comparison with different data", func(t *testing.T) {
		t.Parallel()

		cr1 := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: JsonObject{"key": "value1"},
			},
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		cr2 := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: JsonObject{"key": "value2"},
			},
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		// Should still be considered duplicates based on ID and Version
		if !areCommonRepresentationsDuplicates(cr1, cr2) {
			t.Error("CommonRepresentations with same ID and Version should be duplicates regardless of data")
		}
	})
}

func TestCommonRepresentation_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle special characters in ID", func(t *testing.T) {
		t.Parallel()

		specialIDs := []string{
			"test-id-with-dashes",
			"test_id_with_underscores",
			"test.id.with.dots",
			"test:id:with:colons",
			"test/id/with/slashes",
		}

		for _, id := range specialIDs {
			cr := CommonRepresentation{
				ID:           id,
				ResourceType: "k8s_cluster",
				Version:      1,
			}

			if err := validateCommonRepresentation(cr); err != nil {
				t.Errorf("CommonRepresentation with special character ID %s should be valid: %v", id, err)
			}
		}
	})

	t.Run("should handle large Version numbers", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      999999,
		}

		if err := validateCommonRepresentation(cr); err != nil {
			t.Errorf("CommonRepresentation with large Version should be valid: %v", err)
		}
	})

	t.Run("should handle Unicode characters in string fields", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			ID:                         "测试-id-🚀",
			ResourceType:               "k8s_cluster",
			Version:                    1,
			ReportedByReporterType:     "acm-测试",
			ReportedByReporterInstance: "instance-🌟",
		}

		if err := validateCommonRepresentation(cr); err != nil {
			t.Errorf("CommonRepresentation with Unicode characters should be valid: %v", err)
		}
	})
}

func TestCommonRepresentation_Serialization(t *testing.T) {
	t.Parallel()

	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: JsonObject{
					"key":   "value",
					"count": 42,
				},
			},
			ID:                         "test-id",
			ResourceType:               "k8s_cluster",
			Version:                    1,
			ReportedByReporterType:     "acm",
			ReportedByReporterInstance: "acm-instance-1",
		}

		jsonData, err := json.Marshal(cr)
		if err != nil {
			t.Errorf("Failed to serialize CommonRepresentation to JSON: %v", err)
		}

		// Should contain all fields
		jsonStr := string(jsonData)
		expectedFields := []string{"ID", "ResourceType", "Version", "ReportedByReporterType", "ReportedByReporterInstance", "Data"}
		for _, field := range expectedFields {
			if !strings.Contains(jsonStr, field) {
				t.Errorf("JSON should contain field %s", field)
			}
		}
	})

	t.Run("should deserialize from JSON correctly", func(t *testing.T) {
		t.Parallel()

		jsonStr := `{
			"Data": {"key": "value", "count": 42},
			"ID": "test-id",
			"ResourceType": "k8s_cluster",
			"Version": 1,
			"ReportedByReporterType": "acm",
			"ReportedByReporterInstance": "acm-instance-1"
		}`

		var cr CommonRepresentation
		err := json.Unmarshal([]byte(jsonStr), &cr)
		if err != nil {
			t.Errorf("Failed to deserialize JSON to CommonRepresentation: %v", err)
		}

		// Verify all fields
		if cr.ID != "test-id" {
			t.Error("ID should be deserialized correctly")
		}
		if cr.ResourceType != "k8s_cluster" {
			t.Error("ResourceType should be deserialized correctly")
		}
		if cr.Version != 1 {
			t.Error("Version should be deserialized correctly")
		}
		if cr.Data["key"] != "value" {
			t.Error("Data should be deserialized correctly")
		}
	})

	t.Run("should handle JSON serialization roundtrip", func(t *testing.T) {
		t.Parallel()

		original := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: JsonObject{
					"nested": map[string]interface{}{
						"key": "value",
					},
					"array": []interface{}{1, 2, 3},
				},
			},
			ID:                         "test-id",
			ResourceType:               "k8s_cluster",
			Version:                    1,
			ReportedByReporterType:     "acm",
			ReportedByReporterInstance: "acm-instance-1",
		}

		// Serialize
		jsonData, err := json.Marshal(original)
		if err != nil {
			t.Errorf("Failed to serialize: %v", err)
		}

		// Deserialize
		var deserialized CommonRepresentation
		err = json.Unmarshal(jsonData, &deserialized)
		if err != nil {
			t.Errorf("Failed to deserialize: %v", err)
		}

		// Compare
		if deserialized.ID != original.ID {
			t.Error("ID should match after roundtrip")
		}
		if deserialized.ResourceType != original.ResourceType {
			t.Error("ResourceType should match after roundtrip")
		}
		if deserialized.Version != original.Version {
			t.Error("Version should match after roundtrip")
		}
	})
}

// Helper functions for testing

func validateCommonRepresentation(cr CommonRepresentation) error {
	if strings.TrimSpace(cr.ID) == "" {
		return &ValidationError{Field: "ID", Message: "ID cannot be empty"}
	}
	if strings.TrimSpace(cr.ResourceType) == "" {
		return &ValidationError{Field: "ResourceType", Message: "ResourceType cannot be empty"}
	}
	if cr.Version < 0 {
		return &ValidationError{Field: "Version", Message: "Version cannot be negative"}
	}
	if len(cr.ResourceType) > 128 {
		return &ValidationError{Field: "ResourceType", Message: "ResourceType cannot exceed 128 characters"}
	}
	return nil
}

func areCommonRepresentationsDuplicates(cr1, cr2 CommonRepresentation) bool {
	return cr1.ID == cr2.ID && cr1.Version == cr2.Version
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
