package model

import (
	"testing"
)

// Domain tests for ReporterRepresentationMetadata
//
// These tests focus on domain logic, business rules, and model behavior
// rather than database operations or infrastructure concerns.
//
// Domain tests validate:
// - Business validation rules and constraints
// - Domain behavior and business logic
// - Factory method behavior and validation
// - URL validation and href handling
// - Data handling and transformation logic
// - Model comparison and equality semantics

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

func TestReporterRepresentationMetadata_BusinessRules(t *testing.T) {
	t.Parallel()

	t.Run("should enforce APIHref validation through factory", func(t *testing.T) {
		t.Parallel()

		// Test invalid APIHref URL
		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"not-a-valid-url", // invalid APIHref
			nil,
			1,
			false,
			nil,
		)
		AssertValidationError(t, err, "APIHref", "Invalid APIHref should be rejected")

		// Test empty APIHref
		_, err = NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"", // empty APIHref
			nil,
			1,
			false,
			nil,
		)
		AssertValidationError(t, err, "APIHref", "Empty APIHref should be rejected")
	})

	t.Run("should enforce ConsoleHref validation when provided", func(t *testing.T) {
		t.Parallel()

		// Test invalid ConsoleHref URL
		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			stringPtr("not-a-valid-url"), // invalid ConsoleHref
			1,
			false,
			nil,
		)
		AssertValidationError(t, err, "ConsoleHref", "Invalid ConsoleHref should be rejected")
	})

	t.Run("should enforce key field validation", func(t *testing.T) {
		t.Parallel()

		// Test empty LocalResourceID
		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"", // empty LocalResourceID
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
		AssertValidationError(t, err, "LocalResourceID", "Empty LocalResourceID should be rejected")

		// Test empty ReporterType
		_, err = NewReporterRepresentation(
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

		// Test empty ResourceType
		_, err = NewReporterRepresentation(
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

		// Test empty ReporterInstanceID
		_, err = NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"", // empty ReporterInstanceID
			1,
			"https://api.example.com",
			nil,
			1,
			false,
			nil,
		)
		AssertValidationError(t, err, "ReporterInstanceID", "Empty ReporterInstanceID should be rejected")
	})
}

func TestReporterRepresentationMetadata_HrefValidation(t *testing.T) {
	t.Parallel()

	t.Run("should validate APIHref URL format", func(t *testing.T) {
		t.Parallel()

		validURLs := []string{
			"https://api.example.com/resource/123",
			"http://localhost:8080/api/v1/resource",
			"https://api.redhat.com/api/inventory/v1/hosts/123",
		}

		for _, url := range validURLs {
			t.Run("valid_url_"+url, func(t *testing.T) {
				t.Parallel()
				_, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					"test-local-id",
					"hbi",
					"host",
					1,
					"test-instance",
					1,
					url,
					nil,
					1,
					false,
					nil,
				)
				AssertNoError(t, err, "Valid APIHref URL should be accepted")
			})
		}
	})

	t.Run("should reject invalid APIHref URLs", func(t *testing.T) {
		t.Parallel()

		invalidURLs := []string{
			"not-a-url",
			"ftp://example.com/resource",
			"",
			"://missing-scheme",
		}

		for _, url := range invalidURLs {
			t.Run("invalid_url_"+url, func(t *testing.T) {
				t.Parallel()
				_, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					"test-local-id",
					"hbi",
					"host",
					1,
					"test-instance",
					1,
					url,
					nil,
					1,
					false,
					nil,
				)
				AssertError(t, err, "Invalid APIHref URL should be rejected")
			})
		}
	})

	t.Run("should validate ConsoleHref when provided", func(t *testing.T) {
		t.Parallel()

		validURLs := []string{
			"https://console.redhat.com/insights/inventory/123",
			"https://console.example.com/resource/456",
		}

		for _, url := range validURLs {
			t.Run("valid_console_url_"+url, func(t *testing.T) {
				t.Parallel()
				_, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					"test-local-id",
					"hbi",
					"host",
					1,
					"test-instance",
					1,
					"https://api.example.com",
					stringPtr(url),
					1,
					false,
					nil,
				)
				AssertNoError(t, err, "Valid ConsoleHref URL should be accepted")
			})
		}
	})

	t.Run("should reject invalid ConsoleHref URLs", func(t *testing.T) {
		t.Parallel()

		invalidURLs := []string{
			"not-a-url",
			"ftp://example.com/resource",
			"://missing-scheme",
		}

		for _, url := range invalidURLs {
			t.Run("invalid_console_url_"+url, func(t *testing.T) {
				t.Parallel()
				_, err := NewReporterRepresentation(
					JsonObject{"test": "data"},
					"test-local-id",
					"hbi",
					"host",
					1,
					"test-instance",
					1,
					"https://api.example.com",
					stringPtr(url),
					1,
					false,
					nil,
				)
				AssertError(t, err, "Invalid ConsoleHref URL should be rejected")
			})
		}
	})

	t.Run("should allow nil ConsoleHref", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterRepresentation(
			JsonObject{"test": "data"},
			"test-local-id",
			"hbi",
			"host",
			1,
			"test-instance",
			1,
			"https://api.example.com",
			nil, // nil ConsoleHref should be valid
			1,
			false,
			nil,
		)
		AssertNoError(t, err, "Nil ConsoleHref should be valid")
	})
}

func TestReporterRepresentationMetadata_DataHandling(t *testing.T) {
	t.Parallel()

	t.Run("should handle different resource and reporter type combinations", func(t *testing.T) {
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
			})
		}
	})

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
