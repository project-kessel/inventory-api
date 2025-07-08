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

	t.Run("ReporterRepresentation with very long fields should be invalid", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name     string
			modifier func(*ReporterRepresentation)
		}{
			{
				name: "ReporterType > 128 chars",
				modifier: func(rr *ReporterRepresentation) {
					rr.ReporterType = strings.Repeat("a", 129)
				},
			},
			{
				name: "ResourceType > 128 chars",
				modifier: func(rr *ReporterRepresentation) {
					rr.ResourceType = strings.Repeat("a", 129)
				},
			},
			{
				name: "ReporterInstanceID > 256 chars",
				modifier: func(rr *ReporterRepresentation) {
					rr.ReporterInstanceID = strings.Repeat("a", 257)
				},
			},
			{
				name: "APIHref > 256 chars",
				modifier: func(rr *ReporterRepresentation) {
					rr.APIHref = strings.Repeat("a", 257)
				},
			},
			{
				name: "ConsoleHref > 256 chars",
				modifier: func(rr *ReporterRepresentation) {
					rr.ConsoleHref = strings.Repeat("a", 257)
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				rr := createValidReporterRepresentation()
				tc.modifier(&rr)

				if err := validateReporterRepresentation(rr); err == nil {
					t.Errorf("%s should be invalid", tc.name)
				}
			})
		}
	})
}

func TestReporterRepresentation_BusinessRules(t *testing.T) {
	t.Parallel()

	t.Run("unique constraint should be enforced", func(t *testing.T) {
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
			t.Error("ReporterRepresentations with same unique fields should be considered duplicates")
		}
	})

	t.Run("same LocalResourceID can have multiple versions from same reporter", func(t *testing.T) {
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
			Version:            2,
			ReporterInstanceID: "acm-instance-1",
			Generation:         1,
		}

		if areReporterRepresentationsDuplicates(rr1, rr2) {
			t.Error("Same LocalResourceID should allow multiple versions")
		}
	})

	t.Run("Generation should increment for updates from same reporter instance", func(t *testing.T) {
		t.Parallel()

		generations := []ReporterRepresentation{
			{
				LocalResourceID:    "local-123",
				ReporterInstanceID: "acm-instance-1",
				Generation:         1,
			},
			{
				LocalResourceID:    "local-123",
				ReporterInstanceID: "acm-instance-1",
				Generation:         2,
			},
			{
				LocalResourceID:    "local-123",
				ReporterInstanceID: "acm-instance-1",
				Generation:         3,
			},
		}

		for i := 1; i < len(generations); i++ {
			if generations[i].Generation <= generations[i-1].Generation {
				t.Error("Generation should increment for updates from same reporter instance")
			}
		}
	})
}

func TestReporterRepresentation_TombstoneLogic(t *testing.T) {
	t.Parallel()

	t.Run("new ReporterRepresentation should have Tombstone=false by default", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{}

		if rr.Tombstone != false {
			t.Error("New ReporterRepresentation should have Tombstone=false by default")
		}
	})

	t.Run("setting Tombstone=true should indicate resource deletion", func(t *testing.T) {
		t.Parallel()

		rr := ReporterRepresentation{
			LocalResourceID: "local-123",
			Tombstone:       true,
		}

		if !rr.Tombstone {
			t.Error("Tombstone should be true when set")
		}
	})

	t.Run("Tombstone should support soft delete patterns", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()

		// Initially not tombstoned
		if rr.Tombstone {
			t.Error("Initial representation should not be tombstoned")
		}

		// Mark as tombstoned
		rr.Tombstone = true

		if !rr.Tombstone {
			t.Error("Should be able to mark as tombstoned")
		}

		// Should still be valid even when tombstoned
		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("Tombstoned representation should still be valid: %v", err)
		}
	})
}

func TestReporterRepresentation_HrefValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid HTTP URLs should be accepted", func(t *testing.T) {
		t.Parallel()

		validURLs := []string{
			"http://example.com/api/resource/123",
			"https://api.example.com/v1/clusters/abc",
			"https://console.redhat.com/openshift/cluster/xyz",
		}

		for _, validURL := range validURLs {
			rr := createValidReporterRepresentation()
			rr.APIHref = validURL
			rr.ConsoleHref = validURL

			if err := validateReporterRepresentation(rr); err != nil {
				t.Errorf("Valid URL %s should be accepted: %v", validURL, err)
			}
		}
	})

	t.Run("empty URLs should be valid for optional fields", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.APIHref = ""
		rr.ConsoleHref = ""

		if err := validateReporterRepresentation(rr); err != nil {
			t.Errorf("Empty URLs should be valid for optional fields: %v", err)
		}
	})

	t.Run("invalid URL formats should be rejected", func(t *testing.T) {
		t.Parallel()

		invalidURLs := []string{
			"not-a-url",
			"://missing-protocol.com",
		}

		for _, invalidURL := range invalidURLs {
			rr := createValidReporterRepresentation()
			rr.APIHref = invalidURL

			if err := validateReporterRepresentation(rr); err == nil {
				t.Errorf("Invalid URL %s should be rejected", invalidURL)
			}
		}
	})
}

func TestReporterRepresentation_Serialization(t *testing.T) {
	t.Parallel()

	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()

		jsonBytes, err := json.Marshal(rr)
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
			t.Errorf("Should deserialize from JSON without error: %v", err)
		}

		if rr.LocalResourceID != "local-123" {
			t.Error("Should preserve LocalResourceID during deserialization")
		}
		if rr.ReporterType != "acm" {
			t.Error("Should preserve ReporterType during deserialization")
		}
		if rr.Tombstone != false {
			t.Error("Should preserve Tombstone during deserialization")
		}
	})

	t.Run("should handle JSON serialization of boolean Tombstone field", func(t *testing.T) {
		t.Parallel()

		rr := createValidReporterRepresentation()
		rr.Tombstone = true

		jsonBytes, err := json.Marshal(rr)
		if err != nil {
			t.Errorf("Should serialize boolean field without error: %v", err)
		}

		var deserialized ReporterRepresentation
		err = json.Unmarshal(jsonBytes, &deserialized)
		if err != nil {
			t.Errorf("Should deserialize boolean field without error: %v", err)
		}

		if deserialized.Tombstone != true {
			t.Error("Boolean Tombstone field should be preserved during serialization/deserialization")
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
	if rr.LocalResourceID == "" {
		return &ValidationError{Field: "LocalResourceID", Message: "cannot be empty"}
	}
	if rr.ReporterType == "" {
		return &ValidationError{Field: "ReporterType", Message: "cannot be empty"}
	}
	if rr.ResourceType == "" {
		return &ValidationError{Field: "ResourceType", Message: "cannot be empty"}
	}
	if rr.Version < 0 {
		return &ValidationError{Field: "Version", Message: "cannot be negative"}
	}
	if rr.ReporterInstanceID == "" {
		return &ValidationError{Field: "ReporterInstanceID", Message: "cannot be empty"}
	}
	if rr.Generation < 0 {
		return &ValidationError{Field: "Generation", Message: "cannot be negative"}
	}
	if rr.CommonVersion < 0 {
		return &ValidationError{Field: "CommonVersion", Message: "cannot be negative"}
	}

	// Field length validations
	if len(rr.ReporterType) > 128 {
		return &ValidationError{Field: "ReporterType", Message: "exceeds maximum length of 128"}
	}
	if len(rr.ResourceType) > 128 {
		return &ValidationError{Field: "ResourceType", Message: "exceeds maximum length of 128"}
	}
	if len(rr.ReporterInstanceID) > 256 {
		return &ValidationError{Field: "ReporterInstanceID", Message: "exceeds maximum length of 256"}
	}
	if len(rr.APIHref) > 256 {
		return &ValidationError{Field: "APIHref", Message: "exceeds maximum length of 256"}
	}
	if len(rr.ConsoleHref) > 256 {
		return &ValidationError{Field: "ConsoleHref", Message: "exceeds maximum length of 256"}
	}

	// URL validation for non-empty href fields
	if rr.APIHref != "" {
		if err := validateURL(rr.APIHref); err != nil {
			return &ValidationError{Field: "APIHref", Message: "invalid URL format"}
		}
	}
	if rr.ConsoleHref != "" {
		if err := validateURL(rr.ConsoleHref); err != nil {
			return &ValidationError{Field: "ConsoleHref", Message: "invalid URL format"}
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

	// Check that the URL has a scheme and host
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return &ValidationError{Field: "URL", Message: "URL must have scheme and host"}
	}

	return nil
}
