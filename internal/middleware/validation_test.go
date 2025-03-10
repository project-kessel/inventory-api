package middleware_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/stretchr/testify/assert"
)

func getProjectRootPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd, nil
		}

		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}

	return "", fmt.Errorf("project root not found")
}

func TestSchemaValidation(t *testing.T) {
	projectRoot, err := getProjectRootPath()
	if err != nil {
		t.Fatalf("Failed to determine project root: %v", err)
	}

	schemaDir := filepath.Join(projectRoot, "data", "schema", "resources")

	tests := []struct {
		name           string
		resourceType   string
		reporterData   map[string]interface{}
		expectErr      bool
		expectedErrMsg string
		schemaExpected bool
	}{
		{
			name:         "Valid K8s Cluster with resourceData",
			resourceType: "k8s_cluster",
			reporterData: map[string]interface{}{
				"reporter_type":        "OCM",
				"reporter_instance_id": "user@example.com",
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
			expectErr:      false,
			schemaExpected: true,
		},
		{
			name:         "Valid RHEL Host (No schema expected)",
			resourceType: "rhel_host",
			reporterData: map[string]interface{}{
				"reporter_type":        "HBI",
				"reporter_instance_id": "org-123",
				"local_resource_id":    "rhel-host-001",
				"api_href":             "https://api.rhel.example.com",
				"console_href":         "https://console.rhel.example.com",
			},
			expectErr:      false,
			schemaExpected: false,
		},
		{
			name:         "Valid K8s Policy",
			resourceType: "k8s_policy",
			reporterData: map[string]interface{}{
				"reporter_type":        "ACM",
				"reporter_instance_id": "org-123",
				"local_resource_id":    "k8s-policy-001",
				"api_href":             "https://api.k8s.example.com",
				"console_href":         "https://console.k8s.example.com",
				"resourceData": map[string]interface{}{
					"disabled": true,
					"severity": "MEDIUM",
				},
			},
			expectErr:      false,
			schemaExpected: true,
		},
		{
			name:         "Valid Notifications Integration (No schema expected)",
			resourceType: "notifications_integration",
			reporterData: map[string]interface{}{
				"reporter_type":        "NOTIFICATIONS",
				"reporter_instance_id": "1",
				"local_resource_id":    "notifications-001",
				"api_href":             "https://api.notifications.example.com",
				"console_href":         "https://console.notifications.example.com",
			},
			expectErr:      false,
			schemaExpected: false,
		},

		// bad test cases
		{
			name:         "K8s Cluster missing resourceData",
			resourceType: "k8s_cluster",
			reporterData: map[string]interface{}{
				"reporter_type":        "OCM",
				"reporter_instance_id": "user@example.com",
				"local_resource_id":    "cluster-123",
				"api_href":             "www.example.com",
				"console_href":         "www.example.com",
				"resourceData":         map[string]interface{}{},
			},
			expectErr:      true,
			expectedErrMsg: "validation failed: (root): external_cluster_id is required; (root): cluster_status is required; (root): cluster_reason is required; (root): kube_version is required; (root): kube_vendor is required; (root): vendor_version is required; (root): cloud_platform is required",
			schemaExpected: true,
		},
		{
			name:         "K8s Cluster with missing required fields in resourceData",
			resourceType: "k8s_cluster",
			reporterData: map[string]interface{}{
				"reporter_type":        "OCM",
				"reporter_instance_id": "user@example.com",
				"local_resource_id":    "cluster-123",
				"api_href":             "www.example.com",
				"console_href":         "www.example.com",
				"resourceData": map[string]interface{}{
					"external_cluster_id": "abcd-efgh-1234",
					// Missing cluster_status, cluster_reason, kube_version, kube_vendor, vendor_version, cloud_platform
				},
			},
			expectErr:      true,
			expectedErrMsg: "validation failed: (root): cluster_status is required; (root): cluster_reason is required; (root): kube_version is required; (root): kube_vendor is required; (root): vendor_version is required; (root): cloud_platform is required",
			schemaExpected: true,
		},

		{
			name:         "K8s Cluster with incorrect data types",
			resourceType: "k8s_cluster",
			reporterData: map[string]interface{}{
				"reporter_type":        "OCM",
				"reporter_instance_id": "user@example.com",
				"local_resource_id":    "cluster-123",
				"api_href":             "www.example.com",
				"console_href":         "www.example.com",
				"resourceData": map[string]interface{}{
					"external_cluster_id": 12345, // Should be a string
					"cluster_status":      true,  // Should be a string
					"cluster_reason":      1.2,   // Should be a string
					"kube_version":        nil,   // Should be a string
					"kube_vendor":         "OPENSHIFT",
					"vendor_version":      "4.16",
					"cloud_platform":      "AWS_UPI",
				},
			},
			expectErr:      true,
			expectedErrMsg: "validation failed: external_cluster_id: Invalid type. Expected: string, given: integer; cluster_status: Invalid type. Expected: string, given: boolean; cluster_reason: Invalid type. Expected: string, given: number; kube_version: Invalid type. Expected: string, given: null",
			schemaExpected: true,
		},

		{
			name:         "RHEL Host with resourceData (which is not expected)",
			resourceType: "rhel_host",
			reporterData: map[string]interface{}{
				"reporter_type":        "HBI",
				"reporter_instance_id": "org-123",
				"local_resource_id":    "rhel-host-001",
				"api_href":             "https://api.rhel.example.com",
				"console_href":         "https://console.rhel.example.com",
				"resourceData": map[string]interface{}{
					"unexpected_key": "unexpected_value",
				},
			},
			expectErr:      true,
			expectedErrMsg: "no schema found for 'rhel_host', but 'resourceData' was provided",
			schemaExpected: false,
		},

		{
			name:         "Notifications Integration with resourceData (which is not expected)",
			resourceType: "notifications_integration",
			reporterData: map[string]interface{}{
				"reporter_type":        "NOTIFICATIONS",
				"reporter_instance_id": "1",
				"local_resource_id":    "notifications-001",
				"api_href":             "https://api.notifications.example.com",
				"console_href":         "https://console.notifications.example.com",
				"resourceData": map[string]interface{}{
					"unexpected_key": "unexpected_value",
				},
			},
			expectErr:      true,
			expectedErrMsg: "no schema found for 'notifications_integration', but 'resourceData' was provided",
			schemaExpected: false,
		},

		{
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
			expectErr:      true,
			expectedErrMsg: "no schema found for 'unknown_resource', but 'resourceData' was provided",
			schemaExpected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, exists, err := middleware.LoadResourceSchema(tt.resourceType, schemaDir)

			if tt.schemaExpected && !exists {
				t.Fatalf("Schema for '%s' does not exist but was expected", tt.resourceType)
			}

			if !tt.schemaExpected {
				assert.Nil(t, err, "Unexpected error occurred for resource without schema")
				return
			}

			err = middleware.ValidateJSONSchema(schema, tt.reporterData["resourceData"].(map[string]interface{}))

			if tt.expectErr {
				assert.NotNil(t, err, "Expected error but got nil")
				assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error message mismatch")
			} else {
				assert.Nil(t, err, "Unexpected error occurred")
			}
		})
	}
}
