package in_memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadResourceSchema(t *testing.T) {
	tests := []struct {
		name             string
		resourceType     string
		reporterType     string
		setupFiles       func(string) error
		expectSchema     string
		expectExists     bool
		expectErr        bool
		expectedErrorMsg string
	}{
		{
			name:         "Valid schema file exists",
			resourceType: "host",
			reporterType: "hbi",
			setupFiles: func(tmpDir string) error {
				schemaDir := filepath.Join(tmpDir, "host", "reporters", "hbi")
				if err := os.MkdirAll(schemaDir, 0755); err != nil {
					return err
				}
				schema := `{"type": "object", "properties": {"satellite_id": {"type": "string"}}}`
				return os.WriteFile(filepath.Join(schemaDir, "host.json"), []byte(schema), 0644)
			},
			expectSchema: `{"type": "object", "properties": {"satellite_id": {"type": "string"}}}`,
			expectExists: true,
			expectErr:    false,
		},
		{
			name:         "Schema file does not exist",
			resourceType: "host",
			reporterType: "nonexistent",
			setupFiles: func(tmpDir string) error {
				return nil
			},
			expectSchema: "",
			expectExists: false,
			expectErr:    false,
		},
		{
			name:         "Directory exists but schema file missing",
			resourceType: "k8s_cluster",
			reporterType: "acm",
			setupFiles: func(tmpDir string) error {
				schemaDir := filepath.Join(tmpDir, "k8s_cluster", "reporters", "acm")
				return os.MkdirAll(schemaDir, 0755)
			},
			expectSchema: "",
			expectExists: false,
			expectErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			err := tt.setupFiles(tmpDir)
			assert.NoError(t, err)

			schema, exists, err := loadResourceSchema(tt.resourceType, tt.reporterType, tmpDir)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectExists, exists)
				assert.Equal(t, tt.expectSchema, schema)
			}
		})
	}
}

func TestLoadCommonResourceDataSchema(t *testing.T) {
	tests := []struct {
		name             string
		resourceType     string
		setupFiles       func(string) error
		expectSchema     string
		expectErr        bool
		expectedErrorMsg string
	}{
		{
			name:         "Valid common schema file exists",
			resourceType: "host",
			setupFiles: func(tmpDir string) error {
				resourceDir := filepath.Join(tmpDir, "host")
				if err := os.MkdirAll(resourceDir, 0755); err != nil {
					return err
				}
				schema := `{"type": "object", "properties": {"workspace_id": {"type": "string"}}}`
				return os.WriteFile(filepath.Join(resourceDir, "common_representation.json"), []byte(schema), 0644)
			},
			expectSchema: `{"type": "object", "properties": {"workspace_id": {"type": "string"}}}`,
			expectErr:    false,
		},
		{
			name:         "Common schema file does not exist",
			resourceType: "nonexistent",
			setupFiles: func(tmpDir string) error {
				return nil
			},
			expectSchema:     "",
			expectErr:        true,
			expectedErrorMsg: "failed to read common resource schema",
		},
		{
			name:         "Resource directory exists but common schema missing",
			resourceType: "k8s_cluster",
			setupFiles: func(tmpDir string) error {
				resourceDir := filepath.Join(tmpDir, "k8s_cluster")
				return os.MkdirAll(resourceDir, 0755)
			},
			expectSchema:     "",
			expectErr:        true,
			expectedErrorMsg: "failed to read common resource schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			err := tt.setupFiles(tmpDir)
			assert.NoError(t, err)

			schema, err := loadCommonResourceDataSchema(tt.resourceType, tmpDir)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectSchema, schema)
			}
		})
	}
}

func TestNormalizeResourceType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Resource type with forward slash",
			input:    "rhel/host",
			expected: "rhel_host",
		},
		{
			name:     "Resource type without forward slash",
			input:    "k8s_cluster",
			expected: "k8s_cluster",
		},
		{
			name:     "Resource type with multiple forward slashes",
			input:    "org/team/resource",
			expected: "org_team_resource",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Resource type already normalized",
			input:    "host",
			expected: "host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeResourceType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadResourceSchema_ComplexScenarios(t *testing.T) {
	t.Run("Multiple reporters for same resource", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Setup host resource with multiple reporters
		reporters := []string{"hbi", "satellite", "insights"}
		for _, reporter := range reporters {
			schemaDir := filepath.Join(tmpDir, "host", "reporters", reporter)
			err := os.MkdirAll(schemaDir, 0755)
			assert.NoError(t, err)

			schema := `{"type": "object", "properties": {"` + reporter + `_id": {"type": "string"}}}`
			err = os.WriteFile(filepath.Join(schemaDir, "host.json"), []byte(schema), 0644)
			assert.NoError(t, err)
		}

		// Verify each reporter has its own schema
		for _, reporter := range reporters {
			schema, exists, err := loadResourceSchema("host", reporter, tmpDir)
			assert.NoError(t, err)
			assert.True(t, exists)
			assert.Contains(t, schema, reporter+"_id")
		}
	})

	t.Run("Different resources with same reporter type", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Setup multiple resources with ACM reporter
		resources := []string{"k8s_cluster", "k8s_policy"}
		for _, resource := range resources {
			schemaDir := filepath.Join(tmpDir, resource, "reporters", "acm")
			err := os.MkdirAll(schemaDir, 0755)
			assert.NoError(t, err)

			schema := `{"type": "object", "properties": {"` + resource + `_field": {"type": "string"}}}`
			err = os.WriteFile(filepath.Join(schemaDir, resource+".json"), []byte(schema), 0644)
			assert.NoError(t, err)
		}

		// Verify each resource has its own ACM schema
		for _, resource := range resources {
			schema, exists, err := loadResourceSchema(resource, "acm", tmpDir)
			assert.NoError(t, err)
			assert.True(t, exists)
			assert.Contains(t, schema, resource+"_field")
		}
	})
}

func TestLoadCommonResourceDataSchema_ComplexScenarios(t *testing.T) {
	t.Run("Multiple resources with common schemas", func(t *testing.T) {
		tmpDir := t.TempDir()

		resources := map[string]string{
			"host":        `{"type": "object", "properties": {"workspace_id": {"type": "string"}}}`,
			"k8s_cluster": `{"type": "object", "properties": {"cluster_name": {"type": "string"}}}`,
			"k8s_policy":  `{"type": "object", "properties": {"policy_name": {"type": "string"}}}`,
		}

		for resourceType, schemaContent := range resources {
			resourceDir := filepath.Join(tmpDir, resourceType)
			err := os.MkdirAll(resourceDir, 0755)
			assert.NoError(t, err)

			err = os.WriteFile(filepath.Join(resourceDir, "common_representation.json"), []byte(schemaContent), 0644)
			assert.NoError(t, err)
		}

		// Verify each resource has its own common schema
		for resourceType, expectedSchema := range resources {
			schema, err := loadCommonResourceDataSchema(resourceType, tmpDir)
			assert.NoError(t, err)
			assert.Equal(t, expectedSchema, schema)
		}
	})
}

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
			result := NormalizeResourceType(tt.input)
			assert.Equal(t, tt.expected, result, "Normalized resource type doesn't match")
		})
	}
}
