package resources_test

import (
	"fmt"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2/resources"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
	"strings"
	"testing"
)

// ValidateResourceRequest ensures that a ReportResourceRequest is properly structured.
func ValidateResourceRequest(req *pbv1beta2.ReportResourceRequest) error {
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
func validateReporterData(reporter *v1beta2.ReporterData) error {
	if reporter == nil {
		return fmt.Errorf("reporter_data is required")
	}

	// Required string fields
	requiredFields := map[string]string{
		"reporter_type":        reporter.ReporterType,
		"reporter_instance_id": reporter.ReporterInstanceId,
		"reporter_version":     reporter.ReporterVersion,
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
func ValidateDeleteRequest(req *pbv1beta2.DeleteResourceRequest) error {
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
		request   *pbv1beta2.ReportResourceRequest
		expectErr bool
	}{
		// Valid RHEL Host
		{
			name: "Valid RHEL Host",
			request: &pbv1beta2.ReportResourceRequest{
				Resource: &pbv1beta2.Resource{
					ResourceType: "rhel_host",
					ReporterData: &v1beta2.ReporterData{
						ReporterType:       "HBI",
						ReporterInstanceId: "user@example.com",
						ReporterVersion:    "0.1",
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
		// Missing workspace ID
		{
			name: "Missing workspace_id",
			request: &pbv1beta2.ReportResourceRequest{
				Resource: &pbv1beta2.Resource{
					ResourceType: "rhel_host",
					ReporterData: &v1beta2.ReporterData{
						ReporterType:       "HBI",
						ReporterInstanceId: "user@example.com",
						ReporterVersion:    "0.1",
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
		request   *pbv1beta2.DeleteResourceRequest
		expectErr bool
	}{
		// Valid Delete Request
		{
			name: "Valid Delete Request",
			request: &pbv1beta2.DeleteResourceRequest{
				LocalResourceId: "0123",
				ReporterType:    "HBI",
			},
			expectErr: false,
		},
		// Missing local_resource_id
		{
			name: "Missing local_resource_id",
			request: &pbv1beta2.DeleteResourceRequest{
				ReporterType: "HBI",
			},
			expectErr: true,
		},
		// Missing reporter_type
		{
			name: "Missing reporter_type",
			request: &pbv1beta2.DeleteResourceRequest{
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
