package model

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

// Test scenarios for ReporterRepresentation domain model
//
// These tests focus on domain logic, business rules, and model behavior
// rather than database operations or infrastructure concerns.

func TestReporterRepresentation_TableName(t *testing.T) {
	t.Parallel()

	t.Run("should return correct table name", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{}
		expected := "reporter_representation"
		actual := rr.TableName()

		if actual != expected {
			t.Errorf("Expected table name %q, got %q", expected, actual)
		}
	})

	t.Run("should be consistent across different instances", func(t *testing.T) {
		t.Parallel()

		rr1 := ReporterRepresentation{LocalResourceID: "test1"}
		rr2 := ReporterRepresentation{LocalResourceID: "test2"}

		if rr1.TableName() != rr2.TableName() {
			t.Error("Table name should be consistent across different instances")
		}
	})

	t.Run("should match expected database table naming convention", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{}
		tableName := rr.TableName()

		// Check naming convention: lowercase with underscores
		if strings.Contains(tableName, " ") {
			t.Error("Table name should not contain spaces")
		}
		if strings.ToLower(tableName) != tableName {
			t.Error("Table name should be lowercase")
		}
	})
}

func TestReporterRepresentation_Structure(t *testing.T) {
	t.Parallel()

	t.Run("should properly embed BaseRepresentation", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{}

		// Check if BaseRepresentation is embedded
		rrType := reflect.TypeOf(rr)
		found := false
		for i := 0; i < rrType.NumField(); i++ {
			field := rrType.Field(i)
			if field.Type == reflect.TypeOf(BaseRepresentation{}) && field.Anonymous {
				found = true
				break
			}
		}

		if !found {
			t.Error("ReporterRepresentation should embed BaseRepresentation")
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
			"Version":            reflect.TypeOf(0),
			"ReporterInstanceID": reflect.TypeOf(""),
			"Generation":         reflect.TypeOf(0),
			"APIHref":            reflect.TypeOf(""),
			"ConsoleHref":        reflect.TypeOf(""),
			"CommonVersion":      reflect.TypeOf(0),
			"Tombstone":          reflect.TypeOf(false),
			"ReporterVersion":    reflect.TypeOf(""),
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
			"ReporterType":       "size:128",
			"ResourceType":       "size:128",
			"ReporterInstanceID": "size:256",
			"APIHref":            "size:256",
			"ConsoleHref":        "size:256",
		}

		for fieldName, expectedSize := range sizeConstraints {
			field, found := rrType.FieldByName(fieldName)
			if !found {
				t.Errorf("Field %s not found", fieldName)
				continue
			}

			tag := field.Tag.Get("gorm")
			if !strings.Contains(tag, expectedSize) {
				t.Errorf("Field %s should have %s constraint, got: %s", fieldName, expectedSize, tag)
			}
		}
	})
}

func TestReporterRepresentation_Validation(t *testing.T) {
	t.Parallel()

	t.Run("valid ReporterRepresentation with all required fields", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{
			BaseRepresentation: BaseRepresentation{
				Data: JsonObject{"key": "value"},
			},
			LocalResourceID:    "local-123",
			ReporterType:       "acm",
			ResourceType:       "k8s_cluster",
			Version:            1,
			ReporterInstanceID: "acm-instance-1",
			Generation:         1,
			APIHref:            "https://api.example.com/resource/123",
			ConsoleHref:        "https://console.example.com/resource/123",
			CommonVersion:      1,
			Tombstone:          false,
			ReporterVersion:    "1.0.0",
		}

		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("Valid ReporterRepresentation should not have validation errors: %v", err)
		}
	})

	t.Run("ReporterRepresentation with empty LocalResourceID should be invalid", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{
			LocalResourceID:    "",
			ReporterType:       "acm",
			ResourceType:       "k8s_cluster",
			Version:            1,
			ReporterInstanceID: "acm-instance-1",
			Generation:         1,
		}

		if err := validateReporterRepresentation(rr); err == nil {
			t.Error("ReporterRepresentation with empty LocalResourceID should be invalid")
		}
	})

	t.Run("ReporterRepresentation with empty ReporterType should be invalid", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{
			LocalResourceID:    "local-123",
			ReporterType:       "",
			ResourceType:       "k8s_cluster",
			Version:            1,
			ReporterInstanceID: "acm-instance-1",
			Generation:         1,
		}

		if err := validateReporterRepresentation(rr); err == nil {
			t.Error("ReporterRepresentation with empty ReporterType should be invalid")
		}
	})

	t.Run("ReporterRepresentation with negative Version should be invalid", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{
			LocalResourceID:    "local-123",
			ReporterType:       "acm",
			ResourceType:       "k8s_cluster",
			Version:            -1,
			ReporterInstanceID: "acm-instance-1",
			Generation:         1,
		}

		if err := validateReporterRepresentation(rr); err == nil {
			t.Error("ReporterRepresentation with negative Version should be invalid")
		}
	})

	t.Run("ReporterRepresentation with negative Generation should be invalid", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{
			LocalResourceID:    "local-123",
			ReporterType:       "acm",
			ResourceType:       "k8s_cluster",
			Version:            1,
			ReporterInstanceID: "acm-instance-1",
			Generation:         -1,
		}

		if err := validateReporterRepresentation(rr); err == nil {
			t.Error("ReporterRepresentation with negative Generation should be invalid")
		}
	})

	t.Run("ReporterRepresentation with field length constraints", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name  string
			field string
			value string
			limit int
		}{
			{"ReporterType too long", "ReporterType", strings.Repeat("a", 129), 128},
			{"ResourceType too long", "ResourceType", strings.Repeat("a", 129), 128},
			{"ReporterInstanceID too long", "ReporterInstanceID", strings.Repeat("a", 257), 256},
			{"APIHref too long", "APIHref", strings.Repeat("a", 257), 256},
			{"ConsoleHref too long", "ConsoleHref", strings.Repeat("a", 257), 256},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				rr := createValidReporterRepresentation()

				// Use reflection to set the field value
				rrValue := reflect.ValueOf(&rr).Elem()
				field := rrValue.FieldByName(tc.field)
				if field.IsValid() && field.CanSet() {
					field.SetString(tc.value)
				}

				if err := validateReporterRepresentation(rr); err == nil {
					t.Errorf("ReporterRepresentation with %s longer than %d characters should be invalid", tc.field, tc.limit)
				}
			})
		}
	})

	t.Run("ReporterRepresentation with whitespace-only fields should be invalid", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name  string
			field string
			value string
		}{
			{"whitespace-only LocalResourceID", "LocalResourceID", "   "},
			{"whitespace-only ReporterType", "ReporterType", "   "},
			{"whitespace-only ResourceType", "ResourceType", "   "},
			{"whitespace-only ReporterInstanceID", "ReporterInstanceID", "   "},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				rr := createValidReporterRepresentation()

				// Use reflection to set the field value
				rrValue := reflect.ValueOf(&rr).Elem()
				field := rrValue.FieldByName(tc.field)
				if field.IsValid() && field.CanSet() {
					field.SetString(tc.value)
				}

				if err := validateReporterRepresentation(rr); err == nil {
					t.Errorf("ReporterRepresentation with whitespace-only %s should be invalid", tc.field)
				}
			})
		}
	})
}

func TestReporterRepresentation_BusinessRules(t *testing.T) {
	t.Parallel()

	t.Run("unique constraint should be enforced by all key fields", func(t *testing.T) {
		t.Parallel()

		rr1 := ReporterRepresentation{
			LocalResourceID:    "local-123",
			ReporterType:       "acm",
			ResourceType:       "k8s_cluster",
			Version:            1,
			ReporterInstanceID: "acm-instance-1",
			Generation:         1,
		}

		rr2 := ReporterRepresentation{
			LocalResourceID:    "local-123",
			ReporterType:       "acm",
			ResourceType:       "k8s_cluster",
			Version:            1,
			ReporterInstanceID: "acm-instance-1",
			Generation:         1,
		}

		if !areReporterRepresentationsDuplicates(rr1, rr2) {
			t.Error("ReporterRepresentations with identical key fields should be considered duplicates")
		}
	})

	t.Run("different LocalResourceID should not be duplicates", func(t *testing.T) {
		t.Parallel()

		rr1 := createValidReporterRepresentation()
		rr2 := createValidReporterRepresentation()
		rr2.LocalResourceID = "different-local-id"

		if areReporterRepresentationsDuplicates(rr1, rr2) {
			t.Error("ReporterRepresentations with different LocalResourceID should not be duplicates")
		}
	})

	t.Run("different Generation should not be duplicates", func(t *testing.T) {
		t.Parallel()

		rr1 := createValidReporterRepresentation()
		rr2 := createValidReporterRepresentation()
		rr2.Generation = 2

		if areReporterRepresentationsDuplicates(rr1, rr2) {
			t.Error("ReporterRepresentations with different Generation should not be duplicates")
		}
	})

	t.Run("Generation should support incremental updates", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		originalGeneration := rr.Generation

		rr.Generation++

		if rr.Generation != originalGeneration+1 {
			t.Error("Generation should support incremental updates")
		}
	})

	t.Run("CommonVersion should link to CommonRepresentation", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.CommonVersion = 5

		if rr.CommonVersion != 5 {
			t.Error("CommonVersion should be settable to link to CommonRepresentation")
		}
	})

	t.Run("ReporterVersion should track reporter software version", func(t *testing.T) {
		t.Parallel()

		validVersions := []string{"1.0.0", "2.1.3", "1.0.0-beta.1", "1.0.0+build.123"}

		for _, version := range validVersions {
			rr := createValidReporterRepresentation()
			rr.ReporterVersion = version

			if err := validateReporterRepresentation(rr); err != nil {
				t.Errorf("ReporterRepresentation with valid ReporterVersion %s should be valid: %v", version, err)
			}
		}
	})
}

func TestReporterRepresentation_TombstoneLogic(t *testing.T) {
	t.Parallel()

	t.Run("Tombstone should default to false", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{}

		if rr.Tombstone != false {
			t.Error("Tombstone should default to false")
		}
	})

	t.Run("Tombstone should be settable to true", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.Tombstone = true

		if rr.Tombstone != true {
			t.Error("Tombstone should be settable to true")
		}
	})

	t.Run("Tombstone should indicate resource deletion", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()

		// Resource exists
		rr.Tombstone = false
		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("ReporterRepresentation with Tombstone=false should be valid: %v", err)
		}

		// Resource deleted
		rr.Tombstone = true
		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("ReporterRepresentation with Tombstone=true should be valid: %v", err)
		}
	})

	t.Run("Tombstone should support soft delete pattern", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()

		// Mark as deleted
		rr.Tombstone = true
		rr.Generation++

		if !rr.Tombstone {
			t.Error("Tombstone should remain true after marking as deleted")
		}
		if rr.Generation <= 1 {
			t.Error("Generation should increment when marking as deleted")
		}
	})
}

func TestReporterRepresentation_VersioningLogic(t *testing.T) {
	t.Parallel()

	t.Run("Version should support zero value", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.Version = 0

		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("ReporterRepresentation with Version=0 should be valid: %v", err)
		}
	})

	t.Run("Generation should support zero value", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.Generation = 0

		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("ReporterRepresentation with Generation=0 should be valid: %v", err)
		}
	})

	t.Run("Version and Generation should be independent", func(t *testing.T) {
		t.Parallel()

		rr1 := createValidReporterRepresentation()
		rr1.Version = 1
		rr1.Generation = 5

		rr2 := createValidReporterRepresentation()
		rr2.Version = 3
		rr2.Generation = 2

		// Both should be valid
		if err := validateReporterRepresentation(rr1); err != nil {
			t.Errorf("ReporterRepresentation with Version=1, Generation=5 should be valid: %v", err)
		}
		if err := validateReporterRepresentation(rr2); err != nil {
			t.Errorf("ReporterRepresentation with Version=3, Generation=2 should be valid: %v", err)
		}
	})

	t.Run("CommonVersion should support zero value", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.CommonVersion = 0

		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("ReporterRepresentation with CommonVersion=0 should be valid: %v", err)
		}
	})
}

func TestReporterRepresentation_HrefValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid URLs should be accepted", func(t *testing.T) {
		t.Parallel()

		validURLs := []string{
			"https://api.example.com/resource/123",
			"http://localhost:8080/api/v1/resource",
			"https://console.redhat.com/insights/inventory/123",
			"https://api.openshift.com/api/clusters_mgmt/v1/clusters/abc123",
		}

		for _, url := range validURLs {
			rr := createValidReporterRepresentation()
			rr.APIHref = url
			rr.ConsoleHref = url

			if err := validateReporterRepresentation(rr); err != nil {
				t.Errorf("ReporterRepresentation with valid URL %s should be valid: %v", url, err)
			}
		}
	})

	t.Run("invalid URLs should be rejected", func(t *testing.T) {
		t.Parallel()

		invalidURLs := []string{
			"not-a-url",
			"ftp://example.com/resource",
			"://missing-scheme",
			"https://",
			"http://",
		}

		for _, url := range invalidURLs {
			rr := createValidReporterRepresentation()
			rr.APIHref = url

			if err := validateReporterRepresentation(rr); err == nil {
				t.Errorf("ReporterRepresentation with invalid APIHref %s should be invalid", url)
			}

			rr = createValidReporterRepresentation()
			rr.ConsoleHref = url

			if err := validateReporterRepresentation(rr); err == nil {
				t.Errorf("ReporterRepresentation with invalid ConsoleHref %s should be invalid", url)
			}
		}
	})

	t.Run("empty URLs should be valid", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.APIHref = ""
		rr.ConsoleHref = ""

		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("ReporterRepresentation with empty URLs should be valid: %v", err)
		}
	})
}

func TestReporterRepresentation_DataHandling(t *testing.T) {
	t.Parallel()

	t.Run("should handle nil JsonObject data", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.Data = nil

		// Should not panic when accessing nil data
		_ = rr.Data
		if rr.Data != nil {
			t.Error("Data should be nil when not initialized")
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

		rr := createValidReporterRepresentation()
		rr.Data = complexData

		// Should be able to access nested data
		if metadata, ok := rr.Data["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok {
				if name != "test-cluster" {
					t.Error("Should be able to access nested data correctly")
				}
			}
		}
	})

	t.Run("should support data modification", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.Data = JsonObject{"key": "original"}

		// Modify data
		rr.Data["key"] = "modified"
		rr.Data["new_key"] = "new_value"

		if rr.Data["key"] != "modified" {
			t.Error("Should support data modification")
		}
		if rr.Data["new_key"] != "new_value" {
			t.Error("Should support adding new keys")
		}
	})
}

func TestReporterRepresentation_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle special characters in string fields", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.LocalResourceID = "test-id-with-special-chars!@#$%"
		rr.ReporterInstanceID = "instance_with_underscores"
		rr.ReporterVersion = "1.0.0-beta.1+build.123"

		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("ReporterRepresentation with special characters should be valid: %v", err)
		}
	})

	t.Run("should handle large numeric values", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.Version = 999999
		rr.Generation = 999999
		rr.CommonVersion = 999999

		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("ReporterRepresentation with large numeric values should be valid: %v", err)
		}
	})

	t.Run("should handle Unicode characters", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.LocalResourceID = "测试-resource-🚀"
		rr.ReporterType = "acm-测试"
		rr.ReporterInstanceID = "instance-🌟"

		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("ReporterRepresentation with Unicode characters should be valid: %v", err)
		}
	})
}

func TestReporterRepresentation_Serialization(t *testing.T) {
	t.Parallel()

	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.Data = JsonObject{
			"key":   "value",
			"count": 42,
		}

		jsonData, err := json.Marshal(rr)
		if err != nil {
			t.Errorf("Failed to serialize ReporterRepresentation to JSON: %v", err)
		}

		// Should contain all fields
		jsonStr := string(jsonData)
		expectedFields := []string{
			"LocalResourceID", "ReporterType", "ResourceType", "Version",
			"ReporterInstanceID", "Generation", "APIHref", "ConsoleHref",
			"CommonVersion", "Tombstone", "ReporterVersion", "Data",
		}
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
			"LocalResourceID": "local-123",
			"ReporterType": "acm",
			"ResourceType": "k8s_cluster",
			"Version": 1,
			"ReporterInstanceID": "acm-instance-1",
			"Generation": 1,
			"APIHref": "https://api.example.com/resource/123",
			"ConsoleHref": "https://console.example.com/resource/123",
			"CommonVersion": 1,
			"Tombstone": false,
			"ReporterVersion": "1.0.0"
		}`

		var rr ReporterRepresentation
		err := json.Unmarshal([]byte(jsonStr), &rr)
		if err != nil {
			t.Errorf("Failed to deserialize JSON to ReporterRepresentation: %v", err)
		}

		// Verify all fields
		if rr.LocalResourceID != "local-123" {
			t.Error("LocalResourceID should be deserialized correctly")
		}
		if rr.ReporterType != "acm" {
			t.Error("ReporterType should be deserialized correctly")
		}
		if rr.ResourceType != "k8s_cluster" {
			t.Error("ResourceType should be deserialized correctly")
		}
		if rr.Version != 1 {
			t.Error("Version should be deserialized correctly")
		}
		if rr.Generation != 1 {
			t.Error("Generation should be deserialized correctly")
		}
		if rr.Tombstone != false {
			t.Error("Tombstone should be deserialized correctly")
		}
		if rr.Data["key"] != "value" {
			t.Error("Data should be deserialized correctly")
		}
	})

	t.Run("should handle JSON serialization roundtrip", func(t *testing.T) {
		t.Parallel()

		original := createValidReporterRepresentation()
		original.Data = JsonObject{
			"nested": map[string]interface{}{
				"key": "value",
			},
			"array": []interface{}{1, 2, 3},
		}

		// Serialize
		jsonData, err := json.Marshal(original)
		if err != nil {
			t.Errorf("Failed to serialize: %v", err)
		}

		// Deserialize
		var deserialized ReporterRepresentation
		err = json.Unmarshal(jsonData, &deserialized)
		if err != nil {
			t.Errorf("Failed to deserialize: %v", err)
		}

		// Compare key fields
		if deserialized.LocalResourceID != original.LocalResourceID {
			t.Error("LocalResourceID should match after roundtrip")
		}
		if deserialized.ReporterType != original.ReporterType {
			t.Error("ReporterType should match after roundtrip")
		}
		if deserialized.ResourceType != original.ResourceType {
			t.Error("ResourceType should match after roundtrip")
		}
		if deserialized.Version != original.Version {
			t.Error("Version should match after roundtrip")
		}
		if deserialized.Generation != original.Generation {
			t.Error("Generation should match after roundtrip")
		}
		if deserialized.Tombstone != original.Tombstone {
			t.Error("Tombstone should match after roundtrip")
		}
	})
}

// Helper functions for testing

func createValidReporterRepresentation() ReporterRepresentation {
	return ReporterRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{"key": "value"},
		},
		LocalResourceID:    "local-123",
		ReporterType:       "acm",
		ResourceType:       "k8s_cluster",
		Version:            1,
		ReporterInstanceID: "acm-instance-1",
		Generation:         1,
		APIHref:            "https://api.example.com/resource/123",
		ConsoleHref:        "https://console.example.com/resource/123",
		CommonVersion:      1,
		Tombstone:          false,
		ReporterVersion:    "1.0.0",
	}
}

func validateReporterRepresentation(rr ReporterRepresentation) error {
	if strings.TrimSpace(rr.LocalResourceID) == "" {
		return &ValidationError{Field: "LocalResourceID", Message: "LocalResourceID cannot be empty"}
	}
	if strings.TrimSpace(rr.ReporterType) == "" {
		return &ValidationError{Field: "ReporterType", Message: "ReporterType cannot be empty"}
	}
	if strings.TrimSpace(rr.ResourceType) == "" {
		return &ValidationError{Field: "ResourceType", Message: "ResourceType cannot be empty"}
	}
	if strings.TrimSpace(rr.ReporterInstanceID) == "" {
		return &ValidationError{Field: "ReporterInstanceID", Message: "ReporterInstanceID cannot be empty"}
	}
	if rr.Version < 0 {
		return &ValidationError{Field: "Version", Message: "Version cannot be negative"}
	}
	if rr.Generation < 0 {
		return &ValidationError{Field: "Generation", Message: "Generation cannot be negative"}
	}
	if len(rr.ReporterType) > 128 {
		return &ValidationError{Field: "ReporterType", Message: "ReporterType cannot exceed 128 characters"}
	}
	if len(rr.ResourceType) > 128 {
		return &ValidationError{Field: "ResourceType", Message: "ResourceType cannot exceed 128 characters"}
	}
	if len(rr.ReporterInstanceID) > 256 {
		return &ValidationError{Field: "ReporterInstanceID", Message: "ReporterInstanceID cannot exceed 256 characters"}
	}
	if len(rr.APIHref) > 256 {
		return &ValidationError{Field: "APIHref", Message: "APIHref cannot exceed 256 characters"}
	}
	if len(rr.ConsoleHref) > 256 {
		return &ValidationError{Field: "ConsoleHref", Message: "ConsoleHref cannot exceed 256 characters"}
	}
	if rr.APIHref != "" {
		if err := validateURL(rr.APIHref); err != nil {
			return &ValidationError{Field: "APIHref", Message: "APIHref must be a valid URL"}
		}
	}
	if rr.ConsoleHref != "" {
		if err := validateURL(rr.ConsoleHref); err != nil {
			return &ValidationError{Field: "ConsoleHref", Message: "ConsoleHref must be a valid URL"}
		}
	}
	return nil
}

func areReporterRepresentationsDuplicates(rr1, rr2 ReporterRepresentation) bool {
	return rr1.LocalResourceID == rr2.LocalResourceID &&
		rr1.ReporterType == rr2.ReporterType &&
		rr1.ResourceType == rr2.ResourceType &&
		rr1.Version == rr2.Version &&
		rr1.ReporterInstanceID == rr2.ReporterInstanceID &&
		rr1.Generation == rr2.Generation
}

func validateURL(urlStr string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return &ValidationError{Field: "URL", Message: "URL must have scheme and host"}
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &ValidationError{Field: "URL", Message: "URL must use http or https scheme"}
	}
	return nil
}
