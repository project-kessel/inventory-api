package v1beta2

import (
	"fmt"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
	"strings"
	"testing"
)

// ValidateResourceRequest ensures that a ReportResourceRequest is properly structured.
func ValidateResourceRequest(req *ReportResourceRequest) error {
	if req == nil || req.Resource == nil {
		return fmt.Errorf("resource request is nil or missing")
	}

	// Validate Resource Type
	if req.Resource.ResourceType == "" {
		return fmt.Errorf("resource_type is required")
	}

	// Validate Reporter Data
	if err := validateReporterData(req.Resource.ReporterData); err != nil {
		return fmt.Errorf("invalid reporter_data: %w", err)
	}

	// Validate Common Resource Data
	if err := validateCommonResourceData(req.Resource.CommonResourceData); err != nil {
		return fmt.Errorf("invalid common_resource_data: %w", err)
	}

	return nil
}

// Validate ReporterData structure
func validateReporterData(reporter *ReporterData) error {
	if reporter == nil {
		return fmt.Errorf("reporter_data is required")
	}

	// Required string fields
	requiredFields := map[string]string{
		"reporter_type":        reporter.ReporterType,
		"reporter_instance_id": reporter.ReporterInstanceId,
		"local_resource_id":    reporter.LocalResourceId,
		"api_href":             reporter.ApiHref,
		"console_href":         reporter.ConsoleHref,
	}

	for field, value := range requiredFields {
		if value == "" {
			return fmt.Errorf("%s is required", field)
		}
	}

	return nil
}

// Validate CommonResourceData, ensuring workspace_id exists
func validateCommonResourceData(commonData *structpb.Struct) error {
	if commonData == nil {
		return fmt.Errorf("common_resource_data is required")
	}

	if workspaceID, exists := commonData.Fields["workspace_id"]; !exists || workspaceID.GetStringValue() == "" {
		return fmt.Errorf("workspace_id is required in common_resource_data")
	}

	return nil
}

// ValidateDeleteRequest validates a DeleteResourceRequest
func ValidateDeleteRequest(req *DeleteResourceRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}

	if strings.TrimSpace(req.LocalResourceId) == "" {
		return fmt.Errorf("local_resource_id is required")
	}

	if strings.TrimSpace(req.ReporterType) == "" {
		return fmt.Errorf("reporter_type is required")
	}

	return nil
}

func TestResourceValidation(t *testing.T) {
	failedTests := 0 // Only track failed tests

	tests := []struct {
		name      string
		request   *ReportResourceRequest
		expectErr bool
	}{
		// Valid RHEL Host
		{
			name: "Valid RHEL Host",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType: "rhel_host",
					ReporterData: &ReporterData{
						ReporterType:       "HBI",
						ReporterInstanceId: "user@example.com",
						LocalResourceId:    "0123",
						ApiHref:            "www.example.com",
						ConsoleHref:        "www.example.com",
					},
					CommonResourceData: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": {Kind: &structpb.Value_StringValue{StringValue: "workspace"}},
						},
					},
				},
			},
			expectErr: false,
		},
		// Valid K8s Cluster
		{
			name: "Valid K8s Cluster",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType: "k8s_cluster",
					ReporterData: &ReporterData{
						ReporterType:       "OCM",
						ReporterInstanceId: "user@example.com",
						LocalResourceId:    "cluster-123",
						ApiHref:            "www.example.com",
						ConsoleHref:        "www.example.com",
						ResourceData: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"external_cluster_id": {Kind: &structpb.Value_StringValue{StringValue: "abcd-efgh-1234"}},
								"cluster_status":      {Kind: &structpb.Value_StringValue{StringValue: "READY"}},
							},
						},
					},
					CommonResourceData: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": {Kind: &structpb.Value_StringValue{StringValue: "workspace"}},
						},
					},
				},
			},
			expectErr: false,
		},

		// Valid RHEL Host (No `resource_data` required)
		{
			name: "Valid RHEL Host",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType: "rhel_host",
					ReporterData: &ReporterData{
						ReporterType:       "HBI",
						ReporterInstanceId: "org-123",
						LocalResourceId:    "rhel-host-001",
						ApiHref:            "https://api.rhel.example.com",
						ConsoleHref:        "https://console.rhel.example.com",
					},
					CommonResourceData: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": {Kind: &structpb.Value_StringValue{StringValue: "ws-456"}},
						},
					},
				},
			},
			expectErr: false,
		},
		// Valid Notifications Integration
		{
			name: "Valid Notifications Integration",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType: "notifications_integration",
					ReporterData: &ReporterData{
						ReporterType:       "NOTIFICATIONS",
						ReporterInstanceId: "1",
						LocalResourceId:    "notifications-001",
						ApiHref:            "https://api.notifications.example.com",
						ConsoleHref:        "https://console.notifications.example.com",
					},
					CommonResourceData: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": {Kind: &structpb.Value_StringValue{StringValue: "ws-456"}},
						},
					},
				},
			},
			expectErr: false,
		},
		// Valid RHEL Host w/ inventory_id
		{
			name: "Valid K8s_cluster",
			request: &ReportResourceRequest{
				Resource: &Resource{
					InventoryId:  "12",
					ResourceType: "k8s_cluster",
					ReporterData: &ReporterData{
						ReporterType:       "ACM",
						ReporterInstanceId: "user@example.com",
						LocalResourceId:    "0123",
						ApiHref:            "www.example.com",
						ConsoleHref:        "www.example.com",
					},
					CommonResourceData: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": {Kind: &structpb.Value_StringValue{StringValue: "workspace"}},
						},
					},
				},
			},
			expectErr: false,
		},
		// Missing `ResourceType`
		{
			name: "Missing resource_type",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType: "", // Missing
					ReporterData: &ReporterData{
						ReporterType:       "HBI",
						ReporterInstanceId: "org-123",
						LocalResourceId:    "rhel-host-001",
						ApiHref:            "https://api.rhel.example.com",
						ConsoleHref:        "https://console.rhel.example.com",
					},
					CommonResourceData: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": {Kind: &structpb.Value_StringValue{StringValue: "ws-456"}},
						},
					},
				},
			},
			expectErr: true,
		},

		// Missing `ReporterType`
		{
			name: "Missing ResourceType",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType: "k8s_cluster",
					ReporterData: &ReporterData{
						ReporterType:       "", // Missing
						ReporterInstanceId: "user@example.com",
						LocalResourceId:    "cluster-123",
						ApiHref:            "www.example.com",
						ConsoleHref:        "www.example.com",
					},
					CommonResourceData: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": {Kind: &structpb.Value_StringValue{StringValue: "workspace"}},
						},
					},
				},
			},
			expectErr: true,
		},
		// Missing `ReporterInstanceId`
		{
			name: "Missing ReporterInstanceId",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType: "rhel_host",
					ReporterData: &ReporterData{
						ReporterType:       "HBI",
						ReporterInstanceId: "",
						LocalResourceId:    "rhel-host-001",
						ApiHref:            "https://api.rhel.example.com",
						ConsoleHref:        "https://console.rhel.example.com",
					},
					CommonResourceData: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": {Kind: &structpb.Value_StringValue{StringValue: "ws-456"}},
						},
					},
				},
			},
			expectErr: true,
		},
		// Missing `LocalResourceId`
		{
			name: "Missing LocalResourceId",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType: "rhel_host",
					ReporterData: &ReporterData{
						ReporterType:       "HBI",
						ReporterInstanceId: "user@example.com",
						LocalResourceId:    "",
						ApiHref:            "https://api.rhel.example.com",
						ConsoleHref:        "https://console.rhel.example.com",
					},
					CommonResourceData: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": {Kind: &structpb.Value_StringValue{StringValue: "ws-456"}},
						},
					},
				},
			},
			expectErr: true,
		},
		// Missing `ApiHref`
		{
			name: "Missing ApiHref",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType: "rhel_host",
					ReporterData: &ReporterData{
						ReporterType:       "HBI",
						ReporterInstanceId: "user@example.com",
						LocalResourceId:    "rhel-host-001",
						ApiHref:            "",
						ConsoleHref:        "https://console.rhel.example.com",
					},
					CommonResourceData: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": {Kind: &structpb.Value_StringValue{StringValue: "ws-456"}},
						},
					},
				},
			},
			expectErr: true,
		},
		// Missing `ConsoleHref`
		{
			name: "Missing ConsoleHref",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType: "rhel_host",
					ReporterData: &ReporterData{
						ReporterType:       "HBI",
						ReporterInstanceId: "user@example.com",
						LocalResourceId:    "rhel-host-001",
						ApiHref:            "https://api.rhel.example.com",
						ConsoleHref:        "",
					},
					CommonResourceData: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": {Kind: &structpb.Value_StringValue{StringValue: "ws-456"}},
						},
					},
				},
			},
			expectErr: true,
		},
		// Missing workspace ID
		{
			name: "Missing workspace_id",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType: "rhel_host",
					ReporterData: &ReporterData{
						ReporterType:       "HBI",
						ReporterInstanceId: "user@example.com",
						LocalResourceId:    "0123",
						ApiHref:            "www.example.com",
						ConsoleHref:        "www.example.com",
					},
					CommonResourceData: nil,
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResourceRequest(tt.request)
			if (err == nil) != !tt.expectErr {
				failedTests++
			}
			assert.Equal(t, tt.expectErr, err != nil, "Unexpected validation result")
		})
	}

	t.Logf("Passed Tests: %d / %d", len(tests)-failedTests, len(tests))
}

func TestDeleteResourceValidation(t *testing.T) {
	failedTests := 0

	tests := []struct {
		name      string
		request   *DeleteResourceRequest
		expectErr bool
	}{
		// Valid Delete Request
		{
			name: "Valid Delete Request",
			request: &DeleteResourceRequest{
				LocalResourceId: "0123",
				ReporterType:    "HBI",
			},
			expectErr: false,
		},
		// Missing local_resource_id
		{
			name: "Missing local_resource_id",
			request: &DeleteResourceRequest{
				ReporterType: "HBI",
			},
			expectErr: true,
		},
		// Missing reporter_type
		{
			name: "Missing reporter_type",
			request: &DeleteResourceRequest{
				LocalResourceId: "0123",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeleteRequest(tt.request)
			if (err == nil) != !tt.expectErr {
				failedTests++
			}
			assert.Equal(t, tt.expectErr, err != nil, "Unexpected validation result")
		})
	}

	t.Logf("Passed Tests: %d / %d", len(tests)-failedTests, len(tests))
}
