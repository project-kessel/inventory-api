package middleware_test

import (
	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"path/filepath"
	"testing"

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
				result, err = middleware.ExtractMapField(tt.input, tt.key)
			case "string":
				result, err = middleware.ExtractStringField(tt.input, tt.key)
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
		Resource: &pbv1beta2.Resource{
			ResourceType: "k8s_cluster",
		},
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

func loadCommonSchemaAndValidate(t *testing.T, resourceType string, schemaDir string, commonResourceData map[string]interface{}) {
	commonSchema, err := middleware.LoadCommonResourceDataSchema(resourceType, schemaDir)
	if err != nil {
		t.Fatalf("Failed to load common resource schema: %v", err)
	}

	err = middleware.ValidateJSONSchema(commonSchema, commonResourceData)
	if err != nil {
		t.Fatalf("Validation failed for commonResourceData: %v", err)
	}
}

func runValidationTest(t *testing.T, tt struct {
	name               string
	resourceType       string
	reporterData       map[string]interface{}
	commonResourceData map[string]interface{}
	expectErr          bool
	expectedErrMsg     string
	schemaExpected     bool
}, schemaDir string) {
	err := middleware.ValidateReporterResourceData(tt.resourceType, tt.reporterData)

	if tt.expectErr {
		assert.NotNil(t, err, "Expected error but got nil")
		assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error message mismatch")
	} else {
		assert.Nil(t, err, "Unexpected error occurred")
	}

	if tt.commonResourceData != nil {
		loadCommonSchemaAndValidate(t, tt.resourceType, schemaDir, tt.commonResourceData)
	}
}

func TestSchemaValidation(t *testing.T) {
	projectRoot, err := middleware.GetProjectRootPath()
	if err != nil {
		t.Fatalf("Failed to determine project root: %v", err)
	}

	schemaDir := filepath.Join(projectRoot, "data", "schema", "resources")
	if err := middleware.PreloadAllSchemas(schemaDir); err != nil {
		t.Fatalf("Failed to preload schemas: %v", err)
	}

	tests := []struct {
		name               string
		resourceType       string
		reporterData       map[string]interface{}
		commonResourceData map[string]interface{}
		expectErr          bool
		expectedErrMsg     string
		schemaExpected     bool
	}{
		// Valid test cases
		{
			name:         "Valid K8s Cluster with resourceData",
			resourceType: "k8s_cluster",
			reporterData: map[string]interface{}{
				"reporterType":         "OCM",
				"reporter_instance_id": "user@example.com",
				"reporter_version":     "0.1",
				"local_resource_id":    "cluster-123",
				"api_href":             "www.example.com",
				"console_href":         "www.example.com",
				"resourceData": map[string]interface{}{
					"external_cluster_id": "abcd-efgh-1234",
					"cluster_status":      "READY",
					"cluster_reason":      "All systems operational",
					"kube_version":        "1.31",
					"kube_vendor":         "OPENSHIFT",
					"vendor_version":      "4.16",
					"cloud_platform":      "AWS_UPI",
				},
			},
			commonResourceData: map[string]interface{}{
				"workspace_id": "workspace-123",
			},
			expectErr:      false,
			schemaExpected: true,
		},
		{
			name:         "Valid RHEL Host with Resource Data",
			resourceType: "host",
			reporterData: map[string]interface{}{
				"reporterType":         "HBI",
				"reporter_instance_id": "org-123",
				"local_resource_id":    "rhel-host-001",
				"api_href":             "https://api.rhel.example.com",
				"console_href":         "https://console.rhel.example.com",
				"resourceData": map[string]interface{}{
					"satellite_id":          "550e8400-e29b-41d4-a716-446655440000",
					"sub_manager_id":        "550e8400-e29b-41d4-a716-446655440000",
					"insights_inventory_id": "550e8400-e29b-41d4-a716-446655440000",
					"ansible_host":          "abc",
				},
			},
			commonResourceData: map[string]interface{}{
				"workspace_id": "workspace-123",
			},
			expectErr:      false,
			schemaExpected: false,
		},
		{
			name:         "Valid K8s Policy",
			resourceType: "k8s_policy",
			reporterData: map[string]interface{}{
				"reporterType":         "ACM",
				"reporter_instance_id": "org-123",
				"local_resource_id":    "k8s-policy-001",
				"api_href":             "https://api.k8s.example.com",
				"console_href":         "https://console.k8s.example.com",
				"resourceData": map[string]interface{}{
					"disabled": true,
					"severity": "MEDIUM",
				},
			},
			commonResourceData: map[string]interface{}{
				"workspace_id": "workspace-123",
			},
			expectErr:      false,
			schemaExpected: true,
		},
		{
			name:         "Valid Notifications Integration (No schema expected)",
			resourceType: "notifications_integration",
			reporterData: map[string]interface{}{
				"reporterType":         "NOTIFICATIONS",
				"reporter_instance_id": "1",
				"local_resource_id":    "notifications-001",
				"api_href":             "https://api.notifications.example.com",
				"console_href":         "https://console.notifications.example.com",
			},
			commonResourceData: map[string]interface{}{
				"workspace_id": "workspace-123",
			},
			expectErr:      false,
			schemaExpected: false,
		},

		// Bad test cases
		{
			name:         "K8s Cluster missing resourceData",
			resourceType: "k8s_cluster",
			reporterData: map[string]interface{}{
				"reporterType":         "OCM",
				"reporter_instance_id": "user@example.com",
				"local_resource_id":    "cluster-123",
				"api_href":             "www.example.com",
				"console_href":         "www.example.com",
				"resourceData": map[string]interface{}{
					"a": "a",
				},
			},
			commonResourceData: map[string]interface{}{
				"workspace_id": "workspace-123",
			},
			expectErr:      true,
			expectedErrMsg: "validation failed: (root): external_cluster_id is required; (root): cluster_status is required; (root): cluster_reason is required; (root): kube_version is required; (root): kube_vendor is required; (root): vendor_version is required; (root): cloud_platform is required",
			schemaExpected: true,
		},
		{
			name:         "K8s Cluster with missing required fields in resourceData",
			resourceType: "k8s_cluster",
			reporterData: map[string]interface{}{
				"reporterType":         "OCM",
				"reporter_instance_id": "user@example.com",
				"local_resource_id":    "cluster-123",
				"api_href":             "www.example.com",
				"console_href":         "www.example.com",
				"resourceData": map[string]interface{}{
					"external_cluster_id": "abcd-efgh-1234",
					// Missing cluster_status, cluster_reason, kube_version, kube_vendor, vendor_version, cloud_platform
				},
			},
			commonResourceData: map[string]interface{}{
				"workspace_id": "workspace-123",
			},
			expectErr:      true,
			expectedErrMsg: "validation failed: (root): cluster_status is required; (root): cluster_reason is required; (root): kube_version is required; (root): kube_vendor is required; (root): vendor_version is required; (root): cloud_platform is required",
			schemaExpected: true,
		},
		{
			name:         "K8s Cluster with incorrect data types",
			resourceType: "k8s_cluster",
			reporterData: map[string]interface{}{
				"reporterType":         "OCM",
				"reporter_instance_id": "user@example.com",
				"local_resource_id":    "cluster-123",
				"api_href":             "www.example.com",
				"console_href":         "www.example.com",
				"resourceData": map[string]interface{}{
					"external_cluster_id": 1234, // Invalid type
					"cluster_status":      "READY",
					"cluster_reason":      "All systems operational",
					"kube_version":        "1.31",
					"kube_vendor":         "OPENSHIFT",
					"vendor_version":      "4.16",
					"cloud_platform":      "AWS_UPI",
				},
			},
			commonResourceData: map[string]interface{}{
				"workspace_id": "workspace-123",
			},
			expectErr:      true,
			expectedErrMsg: "validation failed: external_cluster_id: Invalid type. Expected: string, given: integer",
			schemaExpected: true,
		},
		{
			name:         "Notifications Integration with resourceData (which is not expected)",
			resourceType: "notifications_integration",
			reporterData: map[string]interface{}{
				"reporterType":         "NOTIFICATIONS",
				"reporter_instance_id": "1",
				"local_resource_id":    "notifications-001",
				"api_href":             "https://api.notifications.example.com",
				"console_href":         "https://console.notifications.example.com",
				"resourceData": map[string]interface{}{
					"unexpected_key": "unexpected_value",
				},
			},
			commonResourceData: map[string]interface{}{
				"workspace_id": "workspace-123",
			},
			expectErr:      true,
			expectedErrMsg: "resourceData validation failed for 'notifications_integration:notifications': validation failed: (root): reporter_type is required; (root): reporter_instance_id is required; (root): local_resource_id is required",
			schemaExpected: false,
		},

		/*{
			name:         "Unknown resourceType",
			resourceType: "unknown_resource",
			reporterData: map[string]interface{}{
				"reporter_type":        "CUSTOM",
				"reporter_instance_id": "custom-001",
				"local_resource_id":    "custom-123",
				"api_href":             "www.example.com",
				"console_href":         "www.example.com",
				"resourceData": map[string]interface{}{
					"unexpected_field": "data",
				},
			},
			commonResourceData: map[string]interface{}{
				"workspace_id": "workspace-123",
			},
			expectErr:      true,
			expectedErrMsg: "no schema found for 'unknown_resource', but 'resourceData' was provided. Submission is not allowed",
			schemaExpected: false,
		},*/
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runValidationTest(t, tt, schemaDir)
		})
	}
}
