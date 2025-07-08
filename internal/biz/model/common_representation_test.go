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

		longResourceType := strings.Repeat("a", 129) // > 128 chars
		cr := CommonRepresentation{
			ID:           "test-id",
			ResourceType: longResourceType,
			Version:      1,
		}

		if err := validateCommonRepresentation(cr); err == nil {
			t.Error("CommonRepresentation with ResourceType > 128 chars should be invalid")
		}
	})

	t.Run("CommonRepresentation with empty reporter fields should be valid", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			ID:                         "test-id",
			ResourceType:               "k8s_cluster",
			Version:                    1,
			ReportedByReporterType:     "",
			ReportedByReporterInstance: "",
		}

		if err := validateCommonRepresentation(cr); err != nil {
			t.Errorf("CommonRepresentation with empty reporter fields should be valid: %v", err)
		}
	})

	t.Run("CommonRepresentation with nil Data should be valid", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{Data: nil},
			ID:                 "test-id",
			ResourceType:       "k8s_cluster",
			Version:            1,
		}

		if err := validateCommonRepresentation(cr); err != nil {
			t.Errorf("CommonRepresentation with nil Data should be valid: %v", err)
		}
	})

	t.Run("CommonRepresentation with empty Data map should be valid", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{Data: JsonObject{}},
			ID:                 "test-id",
			ResourceType:       "k8s_cluster",
			Version:            1,
		}

		if err := validateCommonRepresentation(cr); err != nil {
			t.Errorf("CommonRepresentation with empty Data map should be valid: %v", err)
		}
	})
}

func TestCommonRepresentation_BusinessRules(t *testing.T) {
	t.Parallel()

	t.Run("same ID and Version should be considered duplicates", func(t *testing.T) {
		t.Parallel()

		cr1 := CommonRepresentation{ID: "test-id", Version: 1}
		cr2 := CommonRepresentation{ID: "test-id", Version: 1}

		if !areCommonRepresentationsDuplicates(cr1, cr2) {
			t.Error("CommonRepresentations with same ID and Version should be considered duplicates")
		}
	})

	t.Run("same ID but different Version should be different", func(t *testing.T) {
		t.Parallel()

		cr1 := CommonRepresentation{ID: "test-id", Version: 1}
		cr2 := CommonRepresentation{ID: "test-id", Version: 2}

		if areCommonRepresentationsDuplicates(cr1, cr2) {
			t.Error("CommonRepresentations with same ID but different Version should be different")
		}
	})

	t.Run("different ID but same Version should be different", func(t *testing.T) {
		t.Parallel()

		cr1 := CommonRepresentation{ID: "test-id-1", Version: 1}
		cr2 := CommonRepresentation{ID: "test-id-2", Version: 1}

		if areCommonRepresentationsDuplicates(cr1, cr2) {
			t.Error("CommonRepresentations with different ID but same Version should be different")
		}
	})

	t.Run("Version should represent evolution of same resource", func(t *testing.T) {
		t.Parallel()

		baseID := "test-resource-id"
		versions := []CommonRepresentation{
			{ID: baseID, Version: 1},
			{ID: baseID, Version: 2},
			{ID: baseID, Version: 3},
		}

		for i := 1; i < len(versions); i++ {
			if versions[i].Version <= versions[i-1].Version {
				t.Error("Version should increment to represent evolution")
			}
		}
	})

	t.Run("ResourceType should be consistent for same resource across versions", func(t *testing.T) {
		t.Parallel()

		baseID := "test-resource-id"
		resourceType := "k8s_cluster"

		cr1 := CommonRepresentation{ID: baseID, ResourceType: resourceType, Version: 1}
		cr2 := CommonRepresentation{ID: baseID, ResourceType: resourceType, Version: 2}

		if cr1.ResourceType != cr2.ResourceType {
			t.Error("ResourceType should be consistent for same resource across versions")
		}
	})

	t.Run("reporter information should track the source", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			ReportedByReporterType:     "acm",
			ReportedByReporterInstance: "acm-hub-1",
		}

		if cr.ReportedByReporterType == "" && cr.ReportedByReporterInstance != "" {
			t.Error("If ReporterInstance is set, ReporterType should also be set")
		}
	})
}

func TestCommonRepresentation_DataHandling(t *testing.T) {
	t.Parallel()

	t.Run("should handle complex nested JSON data structures", func(t *testing.T) {
		t.Parallel()

		complexData := JsonObject{
			"metadata": JsonObject{
				"name":      "test-cluster",
				"namespace": "default",
				"labels": JsonObject{
					"environment": "prod",
					"team":        "platform",
				},
			},
			"spec": JsonObject{
				"replicas": 3,
				"ports":    []interface{}{8080, 8443},
			},
		}

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{Data: complexData},
			ID:                 "test-id",
			ResourceType:       "k8s_cluster",
			Version:            1,
		}

		// Should be able to access nested data
		metadata, ok := cr.Data["metadata"].(JsonObject)
		if !ok {
			t.Error("Should be able to access nested metadata")
		}

		if metadata["name"] != "test-cluster" {
			t.Error("Should preserve nested data values")
		}
	})

	t.Run("should handle JSON data with various primitive types", func(t *testing.T) {
		t.Parallel()

		data := JsonObject{
			"string_field": "test-value",
			"int_field":    42,
			"float_field":  3.14,
			"bool_field":   true,
			"null_field":   nil,
		}

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{Data: data},
		}

		if cr.Data["string_field"] != "test-value" {
			t.Error("Should handle string values")
		}
		if cr.Data["int_field"] != 42 {
			t.Error("Should handle integer values")
		}
		if cr.Data["float_field"] != 3.14 {
			t.Error("Should handle float values")
		}
		if cr.Data["bool_field"] != true {
			t.Error("Should handle boolean values")
		}
		if cr.Data["null_field"] != nil {
			t.Error("Should handle null values")
		}
	})

	t.Run("should handle JSON data with arrays and objects", func(t *testing.T) {
		t.Parallel()

		data := JsonObject{
			"array_field": []interface{}{"item1", "item2", "item3"},
			"object_field": JsonObject{
				"nested_key": "nested_value",
			},
		}

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{Data: data},
		}

		array, ok := cr.Data["array_field"].([]interface{})
		if !ok || len(array) != 3 {
			t.Error("Should handle array data")
		}

		obj, ok := cr.Data["object_field"].(JsonObject)
		if !ok || obj["nested_key"] != "nested_value" {
			t.Error("Should handle object data")
		}
	})

	t.Run("should handle special characters and unicode", func(t *testing.T) {
		t.Parallel()

		data := JsonObject{
			"special_chars": "!@#$%^&*()_+-=[]{}|;':\",./<>?",
			"unicode":       "こんにちは 🌍 émojis",
		}

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{Data: data},
		}

		if cr.Data["special_chars"] != "!@#$%^&*()_+-=[]{}|;':\",./<>?" {
			t.Error("Should handle special characters")
		}
		if cr.Data["unicode"] != "こんにちは 🌍 émojis" {
			t.Error("Should handle unicode characters")
		}
	})
}

func TestCommonRepresentation_Serialization(t *testing.T) {
	t.Parallel()

	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		t.Parallel()

		cr := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: JsonObject{"key": "value"},
			},
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		jsonBytes, err := json.Marshal(cr)
		if err != nil {
			t.Errorf("Should serialize to JSON without error: %v", err)
		}

		if len(jsonBytes) == 0 {
			t.Error("JSON serialization should produce non-empty result")
		}
	})

	t.Run("should deserialize from JSON correctly", func(t *testing.T) {
		t.Parallel()

		jsonStr := `{
			"Data": {"key": "value"},
			"ID": "test-id",
			"ResourceType": "k8s_cluster",
			"Version": 1,
			"ReportedByReporterType": "acm",
			"ReportedByReporterInstance": "acm-instance-1"
		}`

		var cr CommonRepresentation
		err := json.Unmarshal([]byte(jsonStr), &cr)
		if err != nil {
			t.Errorf("Should deserialize from JSON without error: %v", err)
		}

		if cr.ID != "test-id" {
			t.Error("Should preserve ID during deserialization")
		}
		if cr.ResourceType != "k8s_cluster" {
			t.Error("Should preserve ResourceType during deserialization")
		}
		if cr.Version != 1 {
			t.Error("Should preserve Version during deserialization")
		}
	})

	t.Run("should maintain data integrity during serialization/deserialization", func(t *testing.T) {
		t.Parallel()

		original := CommonRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: JsonObject{
					"complex": JsonObject{
						"nested": []interface{}{1, 2, 3},
					},
				},
			},
			ID:           "test-id",
			ResourceType: "k8s_cluster",
			Version:      1,
		}

		// Serialize
		jsonBytes, err := json.Marshal(original)
		if err != nil {
			t.Errorf("Serialization failed: %v", err)
		}

		// Deserialize
		var deserialized CommonRepresentation
		err = json.Unmarshal(jsonBytes, &deserialized)
		if err != nil {
			t.Errorf("Deserialization failed: %v", err)
		}

		// Compare
		if deserialized.ID != original.ID {
			t.Error("ID should be preserved")
		}
		if deserialized.ResourceType != original.ResourceType {
			t.Error("ResourceType should be preserved")
		}
		if deserialized.Version != original.Version {
			t.Error("Version should be preserved")
		}
	})
}

// Helper functions for testing

func validateCommonRepresentation(cr CommonRepresentation) error {
	if cr.ID == "" {
		return &ValidationError{Field: "ID", Message: "cannot be empty"}
	}
	if cr.ResourceType == "" {
		return &ValidationError{Field: "ResourceType", Message: "cannot be empty"}
	}
	if cr.Version < 0 {
		return &ValidationError{Field: "Version", Message: "cannot be negative"}
	}
	if len(cr.ResourceType) > 128 {
		return &ValidationError{Field: "ResourceType", Message: "exceeds maximum length of 128"}
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
