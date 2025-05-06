package v1beta2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// ValidateResourceRequest ensures that a ReportResourceRequest is properly structured.
func ValidateResourceRequest(req *ReportResourceRequest) error {
	if req == nil {
		return fmt.Errorf("resource request is nil or missing")
	}

	// Validate required top-level fields
	if strings.TrimSpace(req.Type) == "" {
		return fmt.Errorf("resource_type is required")
	}
	if strings.TrimSpace(req.ReporterType) == "" {
		return fmt.Errorf("reporter_type is required")
	}
	if strings.TrimSpace(req.ReporterInstanceId) == "" {
		return fmt.Errorf("reporter_instance_id is required")
	}

	// Validate metadata
	metadata := req.Representations.GetMetadata()
	if metadata == nil {
		return fmt.Errorf("representation_metadata is required")
	}
	if metadata.LocalResourceId == "" {
		return fmt.Errorf("local_resource_id is required")
	}
	if metadata.ApiHref == "" {
		return fmt.Errorf("api_href is required")
	}
	if metadata.ConsoleHref == proto.String("") {
		return fmt.Errorf("console_href is required")
	}

	// Validate common data
	if err := validateCommonResourceData(req.Representations.GetCommon()); err != nil {
		return fmt.Errorf("invalid common_resource_data: %w", err)
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

	if req.GetReference() == nil {
		return fmt.Errorf("Resource Reference is required")
	}

	if strings.TrimSpace(req.GetReference().GetResourceId()) == "" {
		return fmt.Errorf("local_resource_id is required")
	}

	if req.GetReference().GetReporter() == nil || strings.TrimSpace(req.GetReference().GetReporter().Type) == "" {
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
			name: "Valid RHEL Host with Reporter Data",
			request: &ReportResourceRequest{

				Type:               "rhel_host",
				ReporterType:       "HBI",
				ReporterInstanceId: "user@example.com",
				Representations: &ResourceRepresentations{
					Metadata: &RepresentationMetadata{
						LocalResourceId: "0123",
						ApiHref:         "https://api.example.com",
						ConsoleHref:     proto.String("https://console.example.com"),
						ReporterVersion: proto.String("q"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace-abc"),
						},
					},
					Reporter: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"satellite_id":          structpb.NewStringValue("f47ac10b-58cc-4372-a567-0e02b2c3d479"),
							"sub_manager_id":        structpb.NewStringValue("6fa459ea-ee8a-3ca4-894e-db77e160355e"),
							"insights_inventory_id": structpb.NewStringValue("1c6fb7dc-34dd-4ea5-a3a6-073acc33107b"),
							"ansible_host":          structpb.NewStringValue("host.example.com"),
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

				Type:               "k8s_cluster",
				ReporterType:       "OCM",
				ReporterInstanceId: "user@example.com",
				Representations: &ResourceRepresentations{
					Metadata: &RepresentationMetadata{
						LocalResourceId: "cluster-123",
						ApiHref:         "https://api.example.com",
						ConsoleHref:     proto.String("https://console.example.com"),
						ReporterVersion: proto.String("1.0.0"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace"),
						},
					},
					Reporter: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"external_cluster_id": structpb.NewStringValue("abcd-efgh-1234"),
							"cluster_status":      structpb.NewStringValue("READY"),
						},
					},
				},
			},
			expectErr: false,
		},

		// Valid RHEL Host (No `reporter` required)
		{
			name: "Valid RHEL Host (No Reporter Data)",
			request: &ReportResourceRequest{

				Type:               "rhel_host",
				ReporterType:       "HBI",
				ReporterInstanceId: "org-123",
				Representations: &ResourceRepresentations{
					Metadata: &RepresentationMetadata{
						LocalResourceId: "rhel-host-001",
						ApiHref:         "https://api.rhel.example.com",
						ConsoleHref:     proto.String("https://console.rhel.example.com"),
						ReporterVersion: proto.String("1.0.0"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("ws-456"),
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

				Type:               "notifications_integration",
				ReporterType:       "NOTIFICATIONS",
				ReporterInstanceId: "1",
				Representations: &ResourceRepresentations{
					Metadata: &RepresentationMetadata{
						LocalResourceId: "notifications-001",
						ApiHref:         "https://api.notifications.example.com",
						ConsoleHref:     proto.String("https://console.notifications.example.com"),
						ReporterVersion: proto.String("1.0.0"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("ws-456"),
						},
					},
				},
			},
			expectErr: false,
		},

		// Valid k8s_cluster w/ inventory_id
		{
			name: "Valid K8s Cluster with Inventory ID",
			request: &ReportResourceRequest{

				InventoryId:        proto.String("12"),
				Type:               "k8s_cluster",
				ReporterType:       "ACM",
				ReporterInstanceId: "user@example.com",
				Representations: &ResourceRepresentations{
					Metadata: &RepresentationMetadata{
						LocalResourceId: "0123",
						ApiHref:         "https://api.example.com",
						ConsoleHref:     proto.String("https://console.example.com"),
						ReporterVersion: proto.String("1.0.0"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace"),
						},
					},
				},
			},
			expectErr: false,
		},

		// Missing ReporterType
		{
			name: "Missing ReporterType",
			request: &ReportResourceRequest{

				Type:               "k8s_cluster",
				ReporterType:       "", // Missing
				ReporterInstanceId: "user@example.com",
				Representations: &ResourceRepresentations{
					Metadata: &RepresentationMetadata{
						LocalResourceId: "0123",
						ApiHref:         "https://api.example.com",
						ConsoleHref:     proto.String("https://console.example.com"),
						ReporterVersion: proto.String("1.0.0"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace"),
						},
					},
				},
			},
			expectErr: true,
		},

		// Missing ResourceType
		{
			name: "Missing ResourceType",
			request: &ReportResourceRequest{

				Type:               "", // Missing
				ReporterType:       "ACM",
				ReporterInstanceId: "user@example.com",
				Representations: &ResourceRepresentations{
					Metadata: &RepresentationMetadata{
						LocalResourceId: "0123",
						ApiHref:         "https://api.example.com",
						ConsoleHref:     proto.String("https://console.example.com"),
						ReporterVersion: proto.String("1.0.0"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace"),
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

				Type:               "k8s_cluster",
				ReporterType:       "OCM", // Missing
				ReporterInstanceId: "user@example.com",
				Representations: &ResourceRepresentations{
					Metadata: &RepresentationMetadata{
						ApiHref:         "https://api.example.com",
						ConsoleHref:     proto.String("https://console.example.com"),
						ReporterVersion: proto.String("1.0.0"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace"),
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

				Type:               "k8s_cluster",
				ReporterType:       "ACM",
				ReporterInstanceId: "user@example.com",
				Representations: &ResourceRepresentations{
					Metadata: &RepresentationMetadata{
						LocalResourceId: "0123",
						ConsoleHref:     proto.String("https://console.example.com"),
						ReporterVersion: proto.String("1.0.0"),
					},
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace"),
						},
					},
				},
			},
			expectErr: true,
		},

		// Missing `metadata`
		{
			name: "Missing ConsoleHref",
			request: &ReportResourceRequest{

				Type:               "k8s_cluster",
				ReporterType:       "ACM",
				ReporterInstanceId: "user@example.com",
				Representations: &ResourceRepresentations{
					Common: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"workspace_id": structpb.NewStringValue("workspace"),
						},
					},
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
				Reference: &ResourceReference{
					ResourceId:   "0123",
					ResourceType: "rhel_host",
					Reporter: &ReporterReference{
						Type: "HBI",
					},
				},
			},
			expectErr: false,
		},
		// Missing local_resource_id
		{
			name: "Missing local_resource_id",
			request: &DeleteResourceRequest{
				Reference: &ResourceReference{
					ResourceType: "rhel_host",
					Reporter: &ReporterReference{
						Type: "HBI",
					},
				},
			},
			expectErr: true,
		},
		// Missing reporter_type
		{
			name: "Missing reporter_type",
			request: &DeleteResourceRequest{
				Reference: &ResourceReference{
					ResourceType: "rhel_host",
				},
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
