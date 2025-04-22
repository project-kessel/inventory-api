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

	resource := req.Resource

	// Validate required top-level fields
	if strings.TrimSpace(resource.ResourceType) == "" {
		return fmt.Errorf("resource_type is required")
	}
	if strings.TrimSpace(resource.ReporterType) == "" {
		return fmt.Errorf("reporter_type is required")
	}
	if strings.TrimSpace(resource.ReporterInstanceId) == "" {
		return fmt.Errorf("reporter_instance_id is required")
	}

	// Validate metadata
	metadata := resource.ResourceRepresentation.GetMetadata()
	if metadata == nil {
		return fmt.Errorf("representation_metadata is required")
	}
	if metadata.LocalResourceId == "" {
		return fmt.Errorf("local_resource_id is required")
	}
	if metadata.ApiHref == "" {
		return fmt.Errorf("api_href is required")
	}
	if metadata.ConsoleHref == "" {
		return fmt.Errorf("console_href is required")
	}

	// Validate common data
	if err := validateCommonResourceData(resource.ResourceRepresentation.GetCommon()); err != nil {
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
			name: "Valid RHEL Host with Reporter Data",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType:       "rhel_host",
					ReporterType:       "HBI",
					ReporterInstanceId: "user@example.com",
					ResourceRepresentation: &ResourceRepresentations{
						Metadata: &RepresentationMetadata{
							LocalResourceId: "0123",
							ApiHref:         "https://api.example.com",
							ConsoleHref:     "https://console.example.com",
							ReporterVersion: "q",
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
			},
			expectErr: false,
		},

		// Valid K8s Cluster
		{
			name: "Valid K8s Cluster",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType:       "k8s_cluster",
					ReporterType:       "OCM",
					ReporterInstanceId: "user@example.com",
					ResourceRepresentation: &ResourceRepresentations{
						Metadata: &RepresentationMetadata{
							LocalResourceId: "cluster-123",
							ApiHref:         "https://api.example.com",
							ConsoleHref:     "https://console.example.com",
							ReporterVersion: "1.0.0",
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
			},
			expectErr: false,
		},

		// Valid RHEL Host (No `reporter` required)
		{
			name: "Valid RHEL Host (No Reporter Data)",
			request: &ReportResourceRequest{
				Resource: &Resource{
					ResourceType:       "rhel_host",
					ReporterType:       "HBI",
					ReporterInstanceId: "org-123",
					ResourceRepresentation: &ResourceRepresentations{
						Metadata: &RepresentationMetadata{
							LocalResourceId: "rhel-host-001",
							ApiHref:         "https://api.rhel.example.com",
							ConsoleHref:     "https://console.rhel.example.com",
							ReporterVersion: "1.0.0",
						},
						Common: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"workspace_id": structpb.NewStringValue("ws-456"),
							},
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
					ResourceType:       "notifications_integration",
					ReporterType:       "NOTIFICATIONS",
					ReporterInstanceId: "1",
					ResourceRepresentation: &ResourceRepresentations{
						Metadata: &RepresentationMetadata{
							LocalResourceId: "notifications-001",
							ApiHref:         "https://api.notifications.example.com",
							ConsoleHref:     "https://console.notifications.example.com",
							ReporterVersion: "1.0.0",
						},
						Common: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"workspace_id": structpb.NewStringValue("ws-456"),
							},
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
				Resource: &Resource{
					InventoryId:        "12",
					ResourceType:       "k8s_cluster",
					ReporterType:       "ACM",
					ReporterInstanceId: "user@example.com",
					ResourceRepresentation: &ResourceRepresentations{
						Metadata: &RepresentationMetadata{
							LocalResourceId: "0123",
							ApiHref:         "https://api.example.com",
							ConsoleHref:     "https://console.example.com",
							ReporterVersion: "1.0.0",
						},
						Common: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"workspace_id": structpb.NewStringValue("workspace"),
							},
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
				Resource: &Resource{
					ResourceType:       "k8s_cluster",
					ReporterType:       "", // Missing
					ReporterInstanceId: "user@example.com",
					ResourceRepresentation: &ResourceRepresentations{
						Metadata: &RepresentationMetadata{
							LocalResourceId: "0123",
							ApiHref:         "https://api.example.com",
							ConsoleHref:     "https://console.example.com",
							ReporterVersion: "1.0.0",
						},
						Common: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"workspace_id": structpb.NewStringValue("workspace"),
							},
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
				Resource: &Resource{
					ResourceType:       "", // Missing
					ReporterType:       "ACM",
					ReporterInstanceId: "user@example.com",
					ResourceRepresentation: &ResourceRepresentations{
						Metadata: &RepresentationMetadata{
							LocalResourceId: "0123",
							ApiHref:         "https://api.example.com",
							ConsoleHref:     "https://console.example.com",
							ReporterVersion: "1.0.0",
						},
						Common: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"workspace_id": structpb.NewStringValue("workspace"),
							},
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
					ResourceType:       "k8s_cluster",
					ReporterType:       "OCM", // Missing
					ReporterInstanceId: "user@example.com",
					ResourceRepresentation: &ResourceRepresentations{
						Metadata: &RepresentationMetadata{
							ApiHref:         "https://api.example.com",
							ConsoleHref:     "https://console.example.com",
							ReporterVersion: "1.0.0",
						},
						Common: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"workspace_id": structpb.NewStringValue("workspace"),
							},
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
					ResourceType:       "k8s_cluster",
					ReporterType:       "ACM",
					ReporterInstanceId: "user@example.com",
					ResourceRepresentation: &ResourceRepresentations{
						Metadata: &RepresentationMetadata{
							LocalResourceId: "0123",
							ConsoleHref:     "https://console.example.com",
							ReporterVersion: "1.0.0",
						},
						Common: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"workspace_id": structpb.NewStringValue("workspace"),
							},
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
					ResourceType:       "k8s_cluster",
					ReporterType:       "ACM",
					ReporterInstanceId: "user@example.com",
					ResourceRepresentation: &ResourceRepresentations{
						Metadata: &RepresentationMetadata{
							LocalResourceId: "0123",
							ApiHref:         "https://api.example.com",
							ReporterVersion: "1.0.0",
						},
						Common: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"workspace_id": structpb.NewStringValue("workspace"),
							},
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
