package data

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
)

var validateSchemaTypeObject = NewJsonSchemaWithWorkspacesFromString(`{"type": "object"}`)

func TestInMemorySchemaRepository_CreateResource(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resource := model.ResourceSchemaRepresentation{
		ResourceType:     "host",
		ValidationSchema: validateSchemaTypeObject,
	}

	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Verify resource was created
	retrieved, err := repo.GetResourceSchema(ctx, "host")
	assert.NoError(t, err)
	assert.Equal(t, "host", retrieved.ResourceType)
	assert.Equal(t, validateSchemaTypeObject, retrieved.ValidationSchema)
}

func TestInMemorySchemaRepository_CreateResource_AlreadyExists(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resource := model.ResourceSchemaRepresentation{
		ResourceType:     "host",
		ValidationSchema: validateSchemaTypeObject,
	}

	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Try to create the same resource again
	err = repo.CreateResourceSchema(ctx, resource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource host already exists")
}

func TestInMemorySchemaRepository_GetResource(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resource := model.ResourceSchemaRepresentation{
		ResourceType:     "k8s_cluster",
		ValidationSchema: validateSchemaTypeObject,
	}

	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	retrieved, err := repo.GetResourceSchema(ctx, "k8s_cluster")
	assert.NoError(t, err)
	assert.Equal(t, "k8s_cluster", retrieved.ResourceType)
}

func TestInMemorySchemaRepository_GetResource_NotFound(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	_, err := repo.GetResourceSchema(ctx, "nonexistent")
	assert.ErrorIs(t, err, model.ResourceSchemaNotFound)
}

func TestInMemorySchemaRepository_UpdateResource(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resource := model.ResourceSchemaRepresentation{
		ResourceType:     "host",
		ValidationSchema: validateSchemaTypeObject,
	}

	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Update the resource
	updatedResource := model.ResourceSchemaRepresentation{
		ResourceType:     "host",
		ValidationSchema: NewJsonSchemaWithWorkspacesFromString(`{"type": "object", "properties": {"name": {"type": "string"}}}`),
	}

	err = repo.UpdateResourceSchema(ctx, updatedResource)
	assert.NoError(t, err)

	// Verify update
	retrieved, err := repo.GetResourceSchema(ctx, "host")
	assert.NoError(t, err)
	assert.Equal(t, updatedResource.ValidationSchema, retrieved.ValidationSchema)
}

func TestInMemorySchemaRepository_UpdateResource_NotFound(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resource := model.ResourceSchemaRepresentation{
		ResourceType:     "nonexistent",
		ValidationSchema: validateSchemaTypeObject,
	}

	err := repo.UpdateResourceSchema(ctx, resource)
	assert.ErrorIs(t, err, model.ResourceSchemaNotFound)
}

func TestInMemorySchemaRepository_DeleteResource(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resource := model.ResourceSchemaRepresentation{
		ResourceType:     "host",
		ValidationSchema: validateSchemaTypeObject,
	}

	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	err = repo.DeleteResourceSchema(ctx, "host")
	assert.NoError(t, err)

	// Verify deletion
	_, err = repo.GetResourceSchema(ctx, "host")
	assert.ErrorIs(t, err, model.ResourceSchemaNotFound)
}

func TestInMemorySchemaRepository_GetResources(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resources := []model.ResourceSchemaRepresentation{
		{ResourceType: "host", ValidationSchema: validateSchemaTypeObject},
		{ResourceType: "k8s_cluster", ValidationSchema: validateSchemaTypeObject},
		{ResourceType: "k8s_policy", ValidationSchema: validateSchemaTypeObject},
	}

	for _, r := range resources {
		err := repo.CreateResourceSchema(ctx, r)
		assert.NoError(t, err)
	}

	retrieved, err := repo.GetResourceSchemas(ctx)
	assert.NoError(t, err)
	assert.Len(t, retrieved, 3)
	assert.Contains(t, retrieved, "host")
	assert.Contains(t, retrieved, "k8s_cluster")
	assert.Contains(t, retrieved, "k8s_policy")
}

func TestInMemorySchemaRepository_CreateResourceReporter(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	// Create resource first
	resource := model.ResourceSchemaRepresentation{
		ResourceType:     "host",
		ValidationSchema: validateSchemaTypeObject,
	}
	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Create reporter
	reporter := model.ReporterSchemaRepresentation{
		ResourceType:     "host",
		ReporterType:     "hbi",
		ValidationSchema: NewJsonSchemaWithWorkspacesFromString(`{"type": "object", "properties": {"satellite_id": {"type": "string"}}}`),
	}

	err = repo.CreateReporterSchema(ctx, reporter)
	assert.NoError(t, err)

	// Verify reporter was created
	retrieved, err := repo.GetReporterSchema(ctx, "host", "hbi")
	assert.NoError(t, err)
	assert.Equal(t, "host", retrieved.ResourceType)
	assert.Equal(t, "hbi", retrieved.ReporterType)
}

func TestInMemorySchemaRepository_GetResourceReporter_NotFound(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	// Create resource first
	resource := model.ResourceSchemaRepresentation{
		ResourceType:     "host",
		ValidationSchema: validateSchemaTypeObject,
	}
	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Try to get non-existent reporter
	_, err = repo.GetReporterSchema(ctx, "host", "nonexistent")
	assert.ErrorIs(t, err, model.ReporterSchemaNotFound)
}

func TestInMemorySchemaRepository_UpdateResourceReporter(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	// Create resource and reporter
	resource := model.ResourceSchemaRepresentation{
		ResourceType:     "host",
		ValidationSchema: validateSchemaTypeObject,
	}
	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	reporter := model.ReporterSchemaRepresentation{
		ResourceType:     "host",
		ReporterType:     "hbi",
		ValidationSchema: validateSchemaTypeObject,
	}
	err = repo.CreateReporterSchema(ctx, reporter)
	assert.NoError(t, err)

	// Update reporter
	updatedReporter := model.ReporterSchemaRepresentation{
		ResourceType:     "host",
		ReporterType:     "hbi",
		ValidationSchema: NewJsonSchemaWithWorkspacesFromString(`{"type": "object", "properties": {"satellite_id": {"type": "string"}}}`),
	}
	err = repo.UpdateReporterSchema(ctx, updatedReporter)
	assert.NoError(t, err)

	// Verify update
	retrieved, err := repo.GetReporterSchema(ctx, "host", "hbi")
	assert.NoError(t, err)
	assert.Equal(t, updatedReporter.ValidationSchema, retrieved.ValidationSchema)
}

func TestInMemorySchemaRepository_DeleteResourceReporter(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	// Create resource and reporter
	resource := model.ResourceSchemaRepresentation{
		ResourceType:     "host",
		ValidationSchema: validateSchemaTypeObject,
	}
	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	reporter := model.ReporterSchemaRepresentation{
		ResourceType:     "host",
		ReporterType:     "hbi",
		ValidationSchema: validateSchemaTypeObject,
	}
	err = repo.CreateReporterSchema(ctx, reporter)
	assert.NoError(t, err)

	// Delete reporter
	err = repo.DeleteReporterSchema(ctx, "host", "hbi")
	assert.NoError(t, err)

	// Verify deletion
	_, err = repo.GetReporterSchema(ctx, "host", "hbi")
	assert.ErrorIs(t, err, model.ReporterSchemaNotFound)
}

func TestInMemorySchemaRepository_GetResourceReporters(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	// Create resource
	resource := model.ResourceSchemaRepresentation{
		ResourceType:     "host",
		ValidationSchema: validateSchemaTypeObject,
	}
	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Create multiple reporters
	reporters := []model.ReporterSchemaRepresentation{
		{ResourceType: "host", ReporterType: "hbi", ValidationSchema: validateSchemaTypeObject},
		{ResourceType: "host", ReporterType: "satellite", ValidationSchema: validateSchemaTypeObject},
		{ResourceType: "host", ReporterType: "insights", ValidationSchema: validateSchemaTypeObject},
	}

	for _, r := range reporters {
		err := repo.CreateReporterSchema(ctx, r)
		assert.NoError(t, err)
	}

	// Get all reporters for resource
	retrieved, err := repo.GetReporterSchemas(ctx, "host")
	assert.NoError(t, err)
	assert.Len(t, retrieved, 3)
	assert.Contains(t, retrieved, "hbi")
	assert.Contains(t, retrieved, "satellite")
	assert.Contains(t, retrieved, "insights")
}

func TestNewFromDir_InvalidDirectory(t *testing.T) {
	ctx := context.Background()
	service, err := NewInMemorySchemaRepositoryFromDir(ctx, "/tmp/wrong/dir", NewJsonSchemaWithWorkspacesFromString)
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "failed to read schema directory \"/tmp/wrong/dir\"")
}

func TestNewFromDir_ValidDirectory(t *testing.T) {
	ctx := context.Background()

	// Create temp directory structure
	tmpDir := t.TempDir()
	hostDir := filepath.Join(tmpDir, "host")
	reportersDir := filepath.Join(hostDir, "reporters", "hbi")

	err := os.MkdirAll(reportersDir, 0755)
	assert.NoError(t, err)

	// Create common schema
	commonSchema := `{"type": "object", "properties": {"workspace_id": {"type": "string"}}}`
	err = os.WriteFile(filepath.Join(hostDir, "common_representation.json"), []byte(commonSchema), 0644)
	assert.NoError(t, err)

	// Create reporter schema
	reporterSchema := `{"type": "object", "properties": {"satellite_id": {"type": "string"}}}`
	err = os.WriteFile(filepath.Join(reportersDir, "host.json"), []byte(reporterSchema), 0644)
	assert.NoError(t, err)

	// Test NewFromDir
	repo, err := NewInMemorySchemaRepositoryFromDir(ctx, tmpDir, NewJsonSchemaWithWorkspacesFromString)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify resource was loaded
	resource, err := repo.GetResourceSchema(ctx, "host")
	assert.NoError(t, err)
	assert.Equal(t, "host", resource.ResourceType)
	assert.Equal(t, NewJsonSchemaWithWorkspacesFromString(commonSchema), resource.ValidationSchema)

	// Verify reporter was loaded
	reporter, err := repo.GetReporterSchema(ctx, "host", "hbi")
	assert.NoError(t, err)
	assert.Equal(t, "host", reporter.ResourceType)
	assert.Equal(t, "hbi", reporter.ReporterType)
	assert.Equal(t, NewJsonSchemaWithWorkspacesFromString(reporterSchema), reporter.ValidationSchema)
}

func TestNewFromJsonFile_InvalidFile(t *testing.T) {
	ctx := context.Background()
	repo, err := NewInMemorySchemaRepositoryFromJsonFile(ctx, "/tmp/nonexistent.json", NewJsonSchemaWithWorkspacesFromString)
	assert.Error(t, err)
	assert.Nil(t, repo)
	assert.Contains(t, err.Error(), "failed to read schema cache file")
}

func TestNewFromJsonFile_ValidFile(t *testing.T) {
	ctx := context.Background()

	// Create temp JSON file
	jsonContent := `{
		"common:host": "{\"type\": \"object\"}",
		"host:hbi": "{\"type\": \"object\", \"properties\": {\"satellite_id\": {\"type\": \"string\"}}}"
	}`

	tmpFile := filepath.Join(t.TempDir(), "schema_cache.json")
	err := os.WriteFile(tmpFile, []byte(jsonContent), 0644)
	assert.NoError(t, err)

	// Test NewFromJsonFile
	repo, err := NewInMemorySchemaRepositoryFromJsonFile(ctx, tmpFile, NewJsonSchemaWithWorkspacesFromString)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify resource was loaded
	resource, err := repo.GetResourceSchema(ctx, "host")
	assert.NoError(t, err)
	assert.Equal(t, "host", resource.ResourceType)

	// Verify reporter was loaded
	reporter, err := repo.GetReporterSchema(ctx, "host", "hbi")
	assert.NoError(t, err)
	assert.Equal(t, "host", reporter.ResourceType)
	assert.Equal(t, "hbi", reporter.ReporterType)
}

func TestNewFromJsonBytes_ValidJSON(t *testing.T) {
	ctx := context.Background()

	jsonContent := []byte(`{
		"common:host": "{\"type\": \"object\"}",
		"common:k8s_cluster": "{\"type\": \"object\"}",
		"host:hbi": "{\"type\": \"object\"}",
		"k8s_cluster:acm": "{\"type\": \"object\"}"
	}`)

	repo, err := NewFromJsonBytes(ctx, jsonContent, NewJsonSchemaWithWorkspacesFromString)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify resources were loaded
	resources, err := repo.GetResourceSchemas(ctx)
	assert.NoError(t, err)
	assert.Contains(t, resources, "host")
	assert.Contains(t, resources, "k8s_cluster")

	// Verify reporters were loaded
	hostReporter, err := repo.GetReporterSchema(ctx, "host", "hbi")
	assert.NoError(t, err)
	assert.Equal(t, "host", hostReporter.ResourceType)
	assert.Equal(t, "hbi", hostReporter.ReporterType)

	k8sReporter, err := repo.GetReporterSchema(ctx, "k8s_cluster", "acm")
	assert.NoError(t, err)
	assert.Equal(t, "k8s_cluster", k8sReporter.ResourceType)
	assert.Equal(t, "acm", k8sReporter.ReporterType)
}

func TestNewFromJsonBytes_InvalidJSON(t *testing.T) {
	ctx := context.Background()

	invalidJSON := []byte(`{invalid json`)

	repo, err := NewFromJsonBytes(ctx, invalidJSON, NewJsonSchemaWithWorkspacesFromString)
	assert.Error(t, err)
	assert.Nil(t, repo)
	assert.Contains(t, err.Error(), "failed to unmarshal schema cache JSON")
}

func TestNewFromJsonBytes_OnlyCommonSchemas(t *testing.T) {
	ctx := context.Background()

	jsonContent := []byte(`{
		"common:host": "{\"type\": \"object\"}",
		"common:k8s_cluster": "{\"type\": \"object\"}"
	}`)

	repo, err := NewFromJsonBytes(ctx, jsonContent, NewJsonSchemaWithWorkspacesFromString)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify resources were loaded
	resources, err := repo.GetResourceSchemas(ctx)
	assert.NoError(t, err)
	assert.Len(t, resources, 2)
	assert.Contains(t, resources, "host")
	assert.Contains(t, resources, "k8s_cluster")

	// Verify no reporters exist
	reporters, err := repo.GetReporterSchemas(ctx, "host")
	assert.NoError(t, err)
	assert.Empty(t, reporters)
}

func TestNew(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	assert.NotNil(t, repo)
	assert.NotNil(t, repo.content)
}

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
				schemaContent := `{"type": "object", "properties": {"satellite_id": {"type": "string"}}}`
				return os.WriteFile(filepath.Join(schemaDir, "host.json"), []byte(schemaContent), 0644)
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

			schemaContent, exists, err := loadResourceSchema(tt.resourceType, tt.reporterType, tmpDir)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectExists, exists)
				assert.Equal(t, tt.expectSchema, schemaContent)
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
				schemaContent := `{"type": "object", "properties": {"workspace_id": {"type": "string"}}}`
				return os.WriteFile(filepath.Join(resourceDir, "common_representation.json"), []byte(schemaContent), 0644)
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

			schemaContent, err := loadCommonResourceDataSchema(tt.resourceType, tmpDir)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectSchema, schemaContent)
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
		{
			name:     "K8s_CLUSTER",
			input:    "K8s_CLUSTER",
			expected: "k8s_cluster",
		},
		{
			name:     "rhel/host",
			input:    "rhel/host",
			expected: "rhel_host",
		},
		{
			name:     "TEST/RESOURCE",
			input:    "TEST/RESOURCE",
			expected: "test_resource",
		},
		{
			name:     "resource",
			input:    "resource",
			expected: "resource",
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

			schemaContent := `{"type": "object", "properties": {"` + reporter + `_id": {"type": "string"}}}`
			err = os.WriteFile(filepath.Join(schemaDir, "host.json"), []byte(schemaContent), 0644)
			assert.NoError(t, err)
		}

		// Verify each reporter has its own schema
		for _, reporter := range reporters {
			schemaContent, exists, err := loadResourceSchema("host", reporter, tmpDir)
			assert.NoError(t, err)
			assert.True(t, exists)
			assert.Contains(t, schemaContent, reporter+"_id")
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

			schemaContent := `{"type": "object", "properties": {"` + resource + `_field": {"type": "string"}}}`
			err = os.WriteFile(filepath.Join(schemaDir, resource+".json"), []byte(schemaContent), 0644)
			assert.NoError(t, err)
		}

		// Verify each resource has its own ACM schema
		for _, resource := range resources {
			schemaContent, exists, err := loadResourceSchema(resource, "acm", tmpDir)
			assert.NoError(t, err)
			assert.True(t, exists)
			assert.Contains(t, schemaContent, resource+"_field")
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
			schemaContent, err := loadCommonResourceDataSchema(resourceType, tmpDir)
			assert.NoError(t, err)
			assert.Equal(t, expectedSchema, schemaContent)
		}
	})
}

func TestReporterMutationsPersist(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	err := repo.CreateResourceSchema(ctx, model.ResourceSchemaRepresentation{
		ResourceType:     "host",
		ValidationSchema: validateSchemaTypeObject,
	})

	assert.NoError(t, err)

	err = repo.CreateReporterSchema(ctx, model.ReporterSchemaRepresentation{
		ResourceType:     "host",
		ReporterType:     "hbi",
		ValidationSchema: validateSchemaTypeObject,
	})

	assert.NoError(t, err)

	reporters, _ := repo.GetReporterSchemas(ctx, "host")
	assert.Contains(t, reporters, "hbi")
}
