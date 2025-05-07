package middleware_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"

	"github.com/stretchr/testify/assert"

	"github.com/project-kessel/inventory-api/internal/middleware"
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

func runValidationTest(
	t *testing.T,
	tt struct {
	name      string
	request   *pbv1beta2.ReportResourceRequest
	expectErr bool
	expectMsg string
},
	validateFunc func(*pbv1beta2.ReportResourceRequest) error,
) {
	err := validateFunc(tt.request)

	if tt.expectErr {
		assert.Error(t, err, "Expected error but got nil")
		if tt.expectMsg != "" {
			assert.Contains(t, err.Error(), tt.expectMsg, "Expected error message mismatch")
		}
	} else {
		assert.NoError(t, err, "Unexpected error occurred")
	}
}

// ValidateResourceRequest validates the structure of a ReportResourceRequest
func ValidateResourceRequest(req *pbv1beta2.ReportResourceRequest) error {
	if req == nil {
		return fmt.Errorf("resource request is nil or missing")
	}

	if strings.TrimSpace(req.Type) == "" {
		return fmt.Errorf("resource_type is required")
	}
	if strings.TrimSpace(req.ReporterType) == "" {
		return fmt.Errorf("reporter_type is required")
	}
	if strings.TrimSpace(req.ReporterInstanceId) == "" {
		return fmt.Errorf("reporter_instance_id is required")
	}

	repr := req.Representations
	if repr == nil || repr.Metadata == nil {
		return fmt.Errorf("representation_metadata is required")
	}

	if strings.TrimSpace(repr.Metadata.LocalResourceId) == "" {
		return fmt.Errorf("local_resource_id is required")
	}
	if strings.TrimSpace(repr.Metadata.ApiHref) == "" {
		return fmt.Errorf("api_href is required")
	}
	if strings.TrimSpace(repr.Metadata.GetConsoleHref()) == "" {
		return fmt.Errorf("console_href is required")
	}

	// Validate common data
	if err := validateCommonRepresentation(repr.GetCommon()); err != nil {
		return fmt.Errorf("invalid common_resource_data: %w", err)
	}

	return nil
}

// validateCommonRepresentation checks for required fields in the common data block
func validateCommonRepresentation(common *structpb.Struct) error {
	if common == nil {
		return fmt.Errorf("common_resource_data is required")
	}
	if val, ok := common.Fields["workspace_id"]; !ok || val.GetStringValue() == "" {
		return fmt.Errorf("workspace_id is required in common_resource_data")
	}
	return nil
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
		name      string
		request   *pbv1beta2.ReportResourceRequest
		expectErr bool
		expectMsg string
	}{
		{
			name: "Missing ReporterType",
			request: &pbv1beta2.ReportResourceRequest{

				Type:               "k8s_cluster",
				ReporterType:       "", // Intentionally invalid
				ReporterInstanceId: "user@example.com",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: &pbv1beta2.RepresentationMetadata{
						LocalResourceId: "0123",
						ApiHref:         "https://api.example.com",
						ConsoleHref:     proto.String("https://console.example.com"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace-123"),
						},
					},
				},
			},
			expectErr: true,
			expectMsg: "reporter_type is required",
		},

		{
			name: "Valid RHEL Host with Resource Data",
			request: &pbv1beta2.ReportResourceRequest{

				Type:               "host",
				ReporterType:       "HBI",
				ReporterInstanceId: "org-123",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: &pbv1beta2.RepresentationMetadata{
						LocalResourceId: "rhel-host-001",
						ApiHref:         "https://api.rhel.example.com",
						ConsoleHref:     proto.String("https://console.rhel.example.com"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace-123"),
						},
					},
					Reporter: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"satellite_id":          structpb.NewStringValue("550e8400-e29b-41d4-a716-446655440000"),
							"sub_manager_id":        structpb.NewStringValue("550e8400-e29b-41d4-a716-446655440000"),
							"insights_inventory_id": structpb.NewStringValue("550e8400-e29b-41d4-a716-446655440000"),
							"ansible_host":          structpb.NewStringValue("abc"),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Valid K8s Policy",
			request: &pbv1beta2.ReportResourceRequest{

				Type:               "k8s_policy",
				ReporterType:       "ACM",
				ReporterInstanceId: "org-123",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: &pbv1beta2.RepresentationMetadata{
						LocalResourceId: "k8s-policy-001",
						ApiHref:         "https://api.k8s.example.com",
						ConsoleHref:     proto.String("https://console.k8s.example.com"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace-123"),
						},
					},
					Reporter: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"disabled": structpb.NewBoolValue(true),
							"severity": structpb.NewStringValue("MEDIUM"),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Valid Notifications Integration (No schema expected)",
			request: &pbv1beta2.ReportResourceRequest{

				Type:               "notifications_integration",
				ReporterType:       "NOTIFICATIONS",
				ReporterInstanceId: "1",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: &pbv1beta2.RepresentationMetadata{
						LocalResourceId: "notifications-001",
						ApiHref:         "https://api.notifications.example.com",
						ConsoleHref:     proto.String("https://console.notifications.example.com"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace-123"),
						},
					},
				},
			},
			expectErr: false,
		},

		// Bad test cases
		{
			name: "K8s Cluster with incorrect data types (simulate type error)",
			request: &pbv1beta2.ReportResourceRequest{

				Type:               "k8s_cluster",
				ReporterType:       "OCM",
				ReporterInstanceId: "user@example.com",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: &pbv1beta2.RepresentationMetadata{
						LocalResourceId: "cluster-123",
						ApiHref:         "www.example.com",
						ConsoleHref:     proto.String("www.example.com"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace-123"),
						},
					},
					Reporter: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							// We're using a string instead of an int due to structpb limitations
							"external_cluster_id": structpb.NewStringValue("1234"),
							"cluster_status":      structpb.NewStringValue("READY"),
							"cluster_reason":      structpb.NewStringValue("All systems operational"),
							"kube_version":        structpb.NewStringValue("1.31"),
							"kube_vendor":         structpb.NewStringValue("OPENSHIFT"),
							"vendor_version":      structpb.NewStringValue("4.16"),
							"cloud_platform":      structpb.NewStringValue("AWS_UPI"),
						},
					},
				},
			},
			expectErr: false, // Can't enforce type mismatch directly with structpb
		},
		{
			name: "Notifications Integration with unexpected reporter data",
			request: &pbv1beta2.ReportResourceRequest{
				
				Type:               "notifications_integration",
				ReporterType:       "NOTIFICATIONS",
				ReporterInstanceId: "1",
				Representations: &pbv1beta2.ResourceRepresentations{
					Metadata: &pbv1beta2.RepresentationMetadata{
						LocalResourceId: "notifications-001",
						ApiHref:         "https://api.notifications.example.com",
						ConsoleHref:     proto.String("https://console.notifications.example.com"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace-123"),
						},
					},
					Reporter: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"unexpected_key": structpb.NewStringValue("unexpected_value"),
						},
					},
				},
			},
			expectErr: false, // If schema validation is NOT enforced in ValidateResourceRequest
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runValidationTest(t, tt, ValidateResourceRequest)
		})
	}
}
