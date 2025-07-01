package middleware_test

import (
	"fmt"
	"strings"
	"testing"

	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/stretchr/testify/assert"
)

// Helper functions
func TestNormalizeResourceTypeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"K8s_CLUSTER", "K8s_CLUSTER"},
		{"rhel/host", "rhel_host"},
		{"TEST/RESOURCE", "TEST_RESOURCE"},
		{"resource", "resource"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := middleware.NormalizeResourceType(tt.input)
			assert.Equal(t, tt.expected, result, "Normalized resource type doesn't match")
		})
	}
}

func TestValidateJSONSchema(t *testing.T) {
	schema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"workspace_id": { "type": "string" }
		},
		"required": ["workspace_id"]
	}`

	tests := []struct {
		name          string
		jsonInput     interface{}
		expectErr     bool
		expectedError string
	}{
		{
			name:      "Valid JSON",
			jsonInput: map[string]interface{}{"workspace_id": "workspace-123"},
			expectErr: false,
		},
		{
			name:          "Invalid JSON (missing required field)",
			jsonInput:     map[string]interface{}{"otherKey": "value"},
			expectErr:     true,
			expectedError: "validation failed: (root): workspace_id is required",
		},
		{
			name:          "Invalid JSON (wrong data type for workspace_id)",
			jsonInput:     map[string]interface{}{"workspace_id": 123},
			expectErr:     true,
			expectedError: "validation failed: workspace_id: Invalid type. Expected: string, given: integer", // Error due to wrong data type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := middleware.ValidateJSONSchema(schema, tt.jsonInput)
			if tt.expectErr {
				assert.Error(t, err, "Expected error but got nil")
				assert.Contains(t, err.Error(), tt.expectedError, "Error message mismatch")
			} else {
				assert.NoError(t, err, "Expected no error for valid JSON format")
			}
		})
	}
}

func TestExtractFields(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]interface{}
		key       string
		expected  interface{}
		expectErr bool
		testType  string // "map" or "string"
	}{
		// Tests for ExtractMapField
		{
			name:      "Valid map extraction",
			input:     map[string]interface{}{"key": map[string]interface{}{"subkey": "value"}},
			key:       "key",
			expected:  map[string]interface{}{"subkey": "value"},
			expectErr: false,
			testType:  "map",
		},
		{
			name:      "Invalid map extraction (not a map)",
			input:     map[string]interface{}{"key": "string_value"},
			key:       "key",
			expected:  nil,
			expectErr: true,
			testType:  "map",
		},
		{
			name:      "Invalid map extraction (nonexistent key)",
			input:     map[string]interface{}{},
			key:       "nonexistent_key",
			expected:  nil,
			expectErr: true,
			testType:  "map",
		},

		// Tests for ExtractStringField
		{
			name:      "Valid string extraction",
			input:     map[string]interface{}{"key": "value"},
			key:       "key",
			expected:  "value",
			expectErr: false,
			testType:  "string",
		},
		{
			name:      "Invalid string extraction (not a string)",
			input:     map[string]interface{}{"key": 123},
			key:       "key",
			expected:  "",
			expectErr: true,
			testType:  "string",
		},
		{
			name:      "Invalid string extraction (nonexistent key)",
			input:     map[string]interface{}{},
			key:       "nonexistent_key",
			expected:  "",
			expectErr: true,
			testType:  "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}
			var err error

			switch tt.testType {
			case "map":
				result, err = middleware.ExtractMapField(tt.input, tt.key, middleware.ValidateFieldExists())
			case "string":
				result, err = middleware.ExtractStringField(tt.input, tt.key, middleware.ValidateFieldExists())
			}

			if tt.expectErr {
				assert.Error(t, err, "Expected error but got nil")
			} else {
				assert.NoError(t, err, "Unexpected error")
				assert.Equal(t, tt.expected, result, "Extracted value doesn't match")
			}
		})
	}
}

func TestExtractFieldsWithOptions(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]interface{}
		key       string
		option    middleware.ExtractOption
		expected  interface{}
		expectErr bool
		testType  string // "map" or "string"
	}{
		// Tests for ValidateFieldExists option
		{
			name: "Map extraction with ValidateFieldExists - representations field exists",
			input: map[string]interface{}{
				"representations": map[string]interface{}{
					"reporter": map[string]interface{}{"satellite_id": "123"},
					"common":   map[string]interface{}{"workspace_id": "ws-456"},
				},
			},
			key:    "representations",
			option: middleware.ValidateFieldExists(),
			expected: map[string]interface{}{
				"reporter": map[string]interface{}{"satellite_id": "123"},
				"common":   map[string]interface{}{"workspace_id": "ws-456"},
			},
			expectErr: false,
			testType:  "map",
		},
		{
			name:      "Map extraction with ValidateFieldExists - representations field missing",
			input:     map[string]interface{}{"type": "host", "reporterType": "hbi"},
			key:       "representations",
			option:    middleware.ValidateFieldExists(),
			expected:  nil,
			expectErr: true,
			testType:  "map",
		},
		{
			name:      "String extraction with ValidateFieldExists - reporterType exists",
			input:     map[string]interface{}{"type": "host", "reporterType": "hbi"},
			key:       "reporterType",
			option:    middleware.ValidateFieldExists(),
			expected:  "hbi",
			expectErr: false,
			testType:  "string",
		},
		{
			name:      "String extraction with ValidateFieldExists - reporterType missing",
			input:     map[string]interface{}{"type": "host"},
			key:       "reporterType",
			option:    middleware.ValidateFieldExists(),
			expected:  "",
			expectErr: true,
			testType:  "string",
		},

		// Tests for default behavior (no options)
		{
			name:      "Map extraction with no options - representations missing (default behavior)",
			input:     map[string]interface{}{"type": "host", "reporterType": "hbi"},
			key:       "representations",
			option:    nil,
			expected:  map[string]interface{}(nil),
			expectErr: false,
			testType:  "map",
		},
		{
			name:      "String extraction with no options - reporterType missing (default behavior)",
			input:     map[string]interface{}{"type": "k8s_policy"},
			key:       "reporterType",
			option:    nil,
			expected:  "",
			expectErr: false,
			testType:  "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}
			var err error

			switch tt.testType {
			case "map":
				if tt.option != nil {
					result, err = middleware.ExtractMapField(tt.input, tt.key, tt.option)
				} else {
					result, err = middleware.ExtractMapField(tt.input, tt.key)
				}
			case "string":
				if tt.option != nil {
					result, err = middleware.ExtractStringField(tt.input, tt.key, tt.option)
				} else {
					result, err = middleware.ExtractStringField(tt.input, tt.key)
				}
			}

			if tt.expectErr {
				assert.Error(t, err, "Expected error but got nil")
			} else {
				assert.NoError(t, err, "Unexpected error")
				assert.Equal(t, tt.expected, result, "Extracted value doesn't match")
			}
		})
	}
}

func TestMarshalProtoToJSON(t *testing.T) {
	msg := &pbv1beta2.ReportResourceRequest{

		Type: "k8s_cluster",
	}
	jsonData, err := middleware.MarshalProtoToJSON(msg)
	assert.NoError(t, err, "Expected no error while marshalling protobuf to JSON")
	assert.Contains(t, string(jsonData), "k8s_cluster", "Expected resource type to be present in JSON")
}

func TestUnmarshalJSONToMap(t *testing.T) {
	tests := []struct {
		input     string
		expected  map[string]interface{}
		expectErr bool
	}{
		{
			input:     `{"key": "value"}`,
			expected:  map[string]interface{}{"key": "value"},
			expectErr: false,
		},
		{
			input:     `invalid json`,
			expected:  nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := middleware.UnmarshalJSONToMap([]byte(tt.input))
			if tt.expectErr {
				assert.Error(t, err, "Expected error but got nil")
			} else {
				assert.NoError(t, err, "Unexpected error")
				assert.Equal(t, tt.expected, result, "Unmarshalled map doesn't match")
			}
		})
	}
}

func TestValidateReporterRepresentation_NoSchemaCases(t *testing.T) {

	t.Run("returns error if no schema and data present", func(t *testing.T) {
		// No schema stored in cache!
		err := middleware.ValidateReporterRepresentation("host", "hbi", map[string]interface{}{"foo": "bar"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no schema found for 'host:hbi', but reporter representation was provided")
	})

	t.Run("returns nil if no schema and data is empty", func(t *testing.T) {
		// No schema stored in cache!
		err := middleware.ValidateReporterRepresentation("host", "hbi", map[string]interface{}{})
		assert.NoError(t, err)
	})
}

func TestValidateReporterRepresentation(t *testing.T) {
	tests := []struct {
		name                   string
		resourceType           string
		reporterType           string
		reporterRepresentation map[string]interface{}
		schema                 string
		expectError            bool
		expectedErrorContains  string
	}{
		{
			name:         "Schema with required fields - valid reporter data provided",
			resourceType: "host",
			reporterType: "hbi",
			reporterRepresentation: map[string]interface{}{
				"satellite_id":            "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
				"subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
				"insights_inventory_id":   "05707922-7b0a-4fe6-982d-6adbc7695b8f",
				"ansible_host":            "host-1",
			},
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"satellite_id": { "type": "string", "format": "uuid" },
					"subscription_manager_id": { "type": "string", "format": "uuid" },
					"insights_inventory_id": { "type": "string", "format": "uuid" },
					"ansible_host": { "type": "string", "maxLength": 255 }
				},
				"required": ["subscription_manager_id"]
			}`,
			expectError: false,
		},
		{
			name:                   "Schema with required fields - missing reporter data",
			resourceType:           "host",
			reporterType:           "hbi",
			reporterRepresentation: nil,
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"satellite_id": { "type": "string", "format": "uuid" },
					"subscription_manager_id": { "type": "string", "format": "uuid" }
				},
				"required": ["subscription_manager_id"]
			}`,
			expectError:           true,
			expectedErrorContains: "Missing 'reporter' field in payload - schema for 'host:hbi' has required fields",
		},
		{
			name:                   "Schema with required fields - empty reporter data",
			resourceType:           "k8s_cluster",
			reporterType:           "acm",
			reporterRepresentation: map[string]interface{}{},
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"cluster_id": { "type": "string" },
					"cluster_name": { "type": "string" }
				},
				"required": ["cluster_id"]
			}`,
			expectError:           true,
			expectedErrorContains: "Missing 'reporter' field in payload - schema for 'k8s_cluster:acm' has required fields",
		},
		{
			name:                   "Schema with NO required fields - missing reporter data (should pass)",
			resourceType:           "k8s_policy",
			reporterType:           "acm",
			reporterRepresentation: nil,
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" },
					"policy_name": { "type": "string" }
				},
				"required": []
			}`,
			expectError: false,
		},
		{
			name:                   "Schema with NO required fields - empty reporter data (should pass)",
			resourceType:           "k8s_policy",
			reporterType:           "acm",
			reporterRepresentation: map[string]interface{}{},
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" },
					"policy_name": { "type": "string" }
				},
				"required": []
			}`,
			expectError: false,
		},
		{
			name:         "Schema with NO required fields - valid reporter data provided",
			resourceType: "k8s_cluster",
			reporterType: "ocm",
			reporterRepresentation: map[string]interface{}{
				"cluster_id":   "cluster-abc123",
				"cluster_name": "production-cluster",
			},
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"cluster_id": { "type": "string" },
					"cluster_name": { "type": "string" }
				}
			}`,
			expectError: false,
		},
		{
			name:         "Schema validation fails - invalid data type",
			resourceType: "host",
			reporterType: "hbi",
			reporterRepresentation: map[string]interface{}{
				"subscription_manager_id": 12345, // Should be string, not number
			},
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"subscription_manager_id": { "type": "string" }
				},
				"required": ["subscription_manager_id"]
			}`,
			expectError:           true,
			expectedErrorContains: "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup schema in cache
			schemaKey := fmt.Sprintf("%s:%s", strings.ToLower(tt.resourceType), strings.ToLower(tt.reporterType))
			middleware.SchemaCache.Store(schemaKey, tt.schema)

			// Test the function
			err := middleware.ValidateReporterRepresentation(tt.resourceType, tt.reporterType, tt.reporterRepresentation)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrorContains != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			// Cleanup
			middleware.SchemaCache.Delete(schemaKey)
		})
	}
}

func TestValidateCommonRepresentation(t *testing.T) {
	tests := []struct {
		name                  string
		resourceType          string
		commonRepresentation  map[string]interface{}
		schema                string
		expectError           bool
		expectedErrorContains string
	}{
		{
			name:         "Schema with required fields - valid common data provided",
			resourceType: "host",
			commonRepresentation: map[string]interface{}{
				"workspace_id": "ws-a64d17d0-aec3-410a-acd0-e0b85b22c076",
			},
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" }
				},
				"required": ["workspace_id"]
			}`,
			expectError: false,
		},
		{
			name:                 "Schema with required fields - missing common data",
			resourceType:         "host",
			commonRepresentation: nil,
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" }
				},
				"required": ["workspace_id"]
			}`,
			expectError:           true,
			expectedErrorContains: "Missing 'common' field in payload - schema for 'host' has required fields",
		},
		{
			name:                 "Schema with required fields - empty common data",
			resourceType:         "k8s_cluster",
			commonRepresentation: map[string]interface{}{},
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" },
					"organization_id": { "type": "string" }
				},
				"required": ["workspace_id"]
			}`,
			expectError:           true,
			expectedErrorContains: "Missing 'common' field in payload - schema for 'k8s_cluster' has required fields",
		},
		{
			name:                 "Schema with NO required fields - missing common data (should pass)",
			resourceType:         "k8s_policy",
			commonRepresentation: nil,
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" },
					"organization_id": { "type": "string" }
				},
				"required": []
			}`,
			expectError: false,
		},
		{
			name:                 "Schema with NO required fields - empty common data (should pass)",
			resourceType:         "k8s_policy",
			commonRepresentation: map[string]interface{}{},
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" },
					"organization_id": { "type": "string" }
				}
			}`,
			expectError: false,
		},
		{
			name:         "Schema with NO required fields - valid common data provided",
			resourceType: "notifications_integration",
			commonRepresentation: map[string]interface{}{
				"workspace_id":    "ws-456",
				"organization_id": "org-789",
			},
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" },
					"organization_id": { "type": "string" }
				}
			}`,
			expectError: false,
		},
		{
			name:         "Schema validation fails - invalid data type",
			resourceType: "host",
			commonRepresentation: map[string]interface{}{
				"workspace_id": 12345, // Should be string, not number
			},
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" }
				},
				"required": ["workspace_id"]
			}`,
			expectError:           true,
			expectedErrorContains: "validation failed",
		},
		{
			name:         "Schema validation fails - missing required field in data",
			resourceType: "k8s_cluster",
			commonRepresentation: map[string]interface{}{
				"organization_id": "org-123",
				// Missing required workspace_id
			},
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" },
					"organization_id": { "type": "string" }
				},
				"required": ["workspace_id", "organization_id"]
			}`,
			expectError:           true,
			expectedErrorContains: "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup schema in cache
			schemaKey := fmt.Sprintf("common:%s", strings.ToLower(tt.resourceType))
			middleware.SchemaCache.Store(schemaKey, tt.schema)

			// Test the function
			err := middleware.ValidateCommonRepresentation(tt.resourceType, tt.commonRepresentation)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrorContains != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			// Cleanup
			middleware.SchemaCache.Delete(schemaKey)
		})
	}
}
