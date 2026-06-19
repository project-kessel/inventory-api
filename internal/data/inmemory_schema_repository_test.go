package data

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

var validateSchemaTypeObject = NewJsonSchemaWithWorkspacesFromString(`{"type": "object"}`)

func TestInMemorySchemaRepository_CreateResource(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	resource, err := bizmodel.NewResourceSchemaRepresentation(resourceType, validateSchemaTypeObject)
	require.NoError(t, err)

	err = repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Verify resource was created
	retrieved, err := repo.GetResourceSchema(ctx, resourceType)
	assert.NoError(t, err)
	assert.Equal(t, resourceType, retrieved.ResourceType())
	assert.Equal(t, validateSchemaTypeObject, retrieved.Schema())
}

func TestInMemorySchemaRepository_CreateResource_AlreadyExists(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	resource, err := bizmodel.NewResourceSchemaRepresentation(resourceType, validateSchemaTypeObject)
	require.NoError(t, err)

	err = repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Try to create the same resource again
	err = repo.CreateResourceSchema(ctx, resource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource host already exists")
}

func TestInMemorySchemaRepository_GetResource(t *testing.T) {
	cases := []struct {
		name         string
		createType   string
		lookupType   string
		expectedType string
	}{
		{"lowercase lookup", "k8s_cluster", "k8s_cluster", "k8s_cluster"},
		{"uppercase lookup normalizes", "k8s_cluster", "K8S_CLUSTER", "k8s_cluster"},
		{"mixed-case lookup", "k8s_cluster", "K8s_Cluster", "k8s_cluster"},
		{"slash normalizes to underscore", "rhel_host", "rhel/host", "rhel_host"},
		{"uppercase + slash", "rhel_host", "RHEL/Host", "rhel_host"},
		{"create uppercase, lookup lowercase", "HOST", "host", "host"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := NewInMemorySchemaRepository()
			ctx := context.Background()

			createType, err := bizmodel.NewResourceType(tc.createType)
			require.NoError(t, err)
			lookupType, err := bizmodel.NewResourceType(tc.lookupType)
			require.NoError(t, err)
			expectedType, err := bizmodel.NewResourceType(tc.expectedType)
			require.NoError(t, err)

			createSchema, err := bizmodel.NewResourceSchemaRepresentation(createType, validateSchemaTypeObject)
			require.NoError(t, err)
			err = repo.CreateResourceSchema(ctx, createSchema)
			assert.NoError(t, err)

			retrieved, err := repo.GetResourceSchema(ctx, lookupType)
			assert.NoError(t, err)
			assert.Equal(t, expectedType, retrieved.ResourceType())
			assert.Equal(t, validateSchemaTypeObject, retrieved.Schema())
		})
	}
}

func TestInMemorySchemaRepository_GetResource_NotFound(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resourceType, err := bizmodel.NewResourceType("nonexistent")
	require.NoError(t, err)
	_, err = repo.GetResourceSchema(ctx, resourceType)
	assert.ErrorIs(t, err, bizmodel.ErrResourceSchemaNotFound)
}

func TestInMemorySchemaRepository_UpdateResource(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	resource, err := bizmodel.NewResourceSchemaRepresentation(resourceType, validateSchemaTypeObject)
	require.NoError(t, err)

	err = repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Update the resource
	updatedSchema := NewJsonSchemaWithWorkspacesFromString(`{"type": "object", "properties": {"name": {"type": "string"}}}`)
	updatedResource, err := bizmodel.NewResourceSchemaRepresentation(resourceType, updatedSchema)
	require.NoError(t, err)

	err = repo.UpdateResourceSchema(ctx, updatedResource)
	assert.NoError(t, err)

	// Verify update
	retrieved, err := repo.GetResourceSchema(ctx, resourceType)
	assert.NoError(t, err)
	assert.Equal(t, updatedSchema, retrieved.Schema())
}

func TestInMemorySchemaRepository_UpdateResource_NotFound(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resourceType, err := bizmodel.NewResourceType("nonexistent")
	require.NoError(t, err)
	resource, err := bizmodel.NewResourceSchemaRepresentation(resourceType, validateSchemaTypeObject)
	require.NoError(t, err)

	err = repo.UpdateResourceSchema(ctx, resource)
	assert.ErrorIs(t, err, bizmodel.ErrResourceSchemaNotFound)
}

func TestInMemorySchemaRepository_DeleteResource(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	resource, err := bizmodel.NewResourceSchemaRepresentation(resourceType, validateSchemaTypeObject)
	require.NoError(t, err)

	err = repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	err = repo.DeleteResourceSchema(ctx, resourceType)
	assert.NoError(t, err)

	// Verify deletion
	_, err = repo.GetResourceSchema(ctx, resourceType)
	assert.ErrorIs(t, err, bizmodel.ErrResourceSchemaNotFound)
}

func TestInMemorySchemaRepository_GetResources(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	rtHost, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	rtK8sCluster, err := bizmodel.NewResourceType("k8s_cluster")
	require.NoError(t, err)
	rtK8sPolicy, err := bizmodel.NewResourceType("k8s_policy")
	require.NoError(t, err)

	for _, rt := range []bizmodel.ResourceType{rtHost, rtK8sCluster, rtK8sPolicy} {
		schema, err := bizmodel.NewResourceSchemaRepresentation(rt, validateSchemaTypeObject)
		require.NoError(t, err)
		err = repo.CreateResourceSchema(ctx, schema)
		assert.NoError(t, err)
	}

	retrieved, err := repo.GetResourceSchemas(ctx)
	assert.NoError(t, err)
	assert.Len(t, retrieved, 3)
	assert.Contains(t, retrieved, rtHost)
	assert.Contains(t, retrieved, rtK8sCluster)
	assert.Contains(t, retrieved, rtK8sPolicy)
}

func TestInMemorySchemaRepository_CreateResourceReporter(t *testing.T) {
	reporterValidation := NewJsonSchemaWithWorkspacesFromString(`{"type": "object", "properties": {"satellite_id": {"type": "string"}}}`)

	cases := []struct {
		name                 string
		createReporterType   string
		lookupReporterType   string
		expectedReporterType string
	}{
		{"lowercase reporter", "hbi", "hbi", "hbi"},
		{"uppercase reporter lookup", "hbi", "HBI", "hbi"},
		{"create uppercase, lookup lowercase", "HBI", "hbi", "hbi"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := NewInMemorySchemaRepository()
			ctx := context.Background()

			resourceType, err := bizmodel.NewResourceType("host")
			require.NoError(t, err)
			resourceSchema, err := bizmodel.NewResourceSchemaRepresentation(resourceType, validateSchemaTypeObject)
			require.NoError(t, err)
			err = repo.CreateResourceSchema(ctx, resourceSchema)
			assert.NoError(t, err)

			createRep, err := bizmodel.NewReporterType(tc.createReporterType)
			require.NoError(t, err)
			reporterSchema, err := bizmodel.NewReporterSchemaRepresentation(resourceType, createRep, reporterValidation)
			require.NoError(t, err)
			err = repo.CreateReporterSchema(ctx, reporterSchema)
			assert.NoError(t, err)

			lookupRep, err := bizmodel.NewReporterType(tc.lookupReporterType)
			require.NoError(t, err)
			retrieved, err := repo.GetReporterSchema(ctx, resourceType, lookupRep)
			assert.NoError(t, err)
			expectedRep, err := bizmodel.NewReporterType(tc.expectedReporterType)
			require.NoError(t, err)
			assert.Equal(t, resourceType, retrieved.ResourceType())
			assert.Equal(t, expectedRep, retrieved.ReporterType())
			assert.Equal(t, reporterValidation, retrieved.Schema())
		})
	}
}

func TestInMemorySchemaRepository_GetResourceReporter_NotFound(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	resource, err := bizmodel.NewResourceSchemaRepresentation(resourceType, validateSchemaTypeObject)
	require.NoError(t, err)
	err = repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	repNonexistent, err := bizmodel.NewReporterType("nonexistent")
	require.NoError(t, err)
	// Try to get non-existent reporter
	_, err = repo.GetReporterSchema(ctx, resourceType, repNonexistent)
	assert.ErrorIs(t, err, bizmodel.ErrReporterSchemaNotFound)
}

func TestInMemorySchemaRepository_UpdateResourceReporter(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	repHbi, err := bizmodel.NewReporterType("hbi")
	require.NoError(t, err)

	// Create resource and reporter
	resource, err := bizmodel.NewResourceSchemaRepresentation(resourceType, validateSchemaTypeObject)
	require.NoError(t, err)
	err = repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	reporter, err := bizmodel.NewReporterSchemaRepresentation(resourceType, repHbi, validateSchemaTypeObject)
	require.NoError(t, err)
	err = repo.CreateReporterSchema(ctx, reporter)
	assert.NoError(t, err)

	// Update reporter
	updatedSchema := NewJsonSchemaWithWorkspacesFromString(`{"type": "object", "properties": {"satellite_id": {"type": "string"}}}`)
	updatedReporter, err := bizmodel.NewReporterSchemaRepresentation(resourceType, repHbi, updatedSchema)
	require.NoError(t, err)
	err = repo.UpdateReporterSchema(ctx, updatedReporter)
	assert.NoError(t, err)

	// Verify update
	retrieved, err := repo.GetReporterSchema(ctx, resourceType, repHbi)
	assert.NoError(t, err)
	assert.Equal(t, updatedSchema, retrieved.Schema())
}

func TestInMemorySchemaRepository_DeleteResourceReporter(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	repHbi, err := bizmodel.NewReporterType("hbi")
	require.NoError(t, err)

	// Create resource and reporter
	resource, err := bizmodel.NewResourceSchemaRepresentation(resourceType, validateSchemaTypeObject)
	require.NoError(t, err)
	err = repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	reporter, err := bizmodel.NewReporterSchemaRepresentation(resourceType, repHbi, validateSchemaTypeObject)
	require.NoError(t, err)
	err = repo.CreateReporterSchema(ctx, reporter)
	assert.NoError(t, err)

	// Delete reporter
	err = repo.DeleteReporterSchema(ctx, resourceType, repHbi)
	assert.NoError(t, err)

	// Verify deletion
	_, err = repo.GetReporterSchema(ctx, resourceType, repHbi)
	assert.ErrorIs(t, err, bizmodel.ErrReporterSchemaNotFound)
}

func TestInMemorySchemaRepository_GetResourceReporters(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	repHbi, err := bizmodel.NewReporterType("hbi")
	require.NoError(t, err)
	repSatellite, err := bizmodel.NewReporterType("satellite")
	require.NoError(t, err)
	repInsights, err := bizmodel.NewReporterType("insights")
	require.NoError(t, err)

	// Create resource
	resource, err := bizmodel.NewResourceSchemaRepresentation(resourceType, validateSchemaTypeObject)
	require.NoError(t, err)
	err = repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Create multiple reporters
	for _, rep := range []bizmodel.ReporterType{repHbi, repSatellite, repInsights} {
		reporterSchema, err := bizmodel.NewReporterSchemaRepresentation(resourceType, rep, validateSchemaTypeObject)
		require.NoError(t, err)
		err = repo.CreateReporterSchema(ctx, reporterSchema)
		assert.NoError(t, err)
	}

	// Get all reporters for resource
	retrieved, err := repo.GetReporterSchemas(ctx, resourceType)
	assert.NoError(t, err)
	assert.Len(t, retrieved, 3)
	assert.Contains(t, retrieved, repHbi)
	assert.Contains(t, retrieved, repSatellite)
	assert.Contains(t, retrieved, repInsights)
}

func TestNewFromDir_InvalidDirectory(t *testing.T) {
	ctx := context.Background()
	service, err := NewInMemorySchemaRepositoryFromDir(ctx, "/tmp/wrong/dir", DefaultSchemaFactory)
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
	repo, err := NewInMemorySchemaRepositoryFromDir(ctx, tmpDir, DefaultSchemaFactory)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	rtHost, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	repHbi, err := bizmodel.NewReporterType("hbi")
	require.NoError(t, err)

	// Verify resource was loaded
	resource, err := repo.GetResourceSchema(ctx, rtHost)
	assert.NoError(t, err)
	assert.Equal(t, rtHost, resource.ResourceType())
	assert.Equal(t, NewJsonSchemaWithWorkspacesFromString(commonSchema), resource.Schema())

	// Verify reporter was loaded
	reporter, err := repo.GetReporterSchema(ctx, rtHost, repHbi)
	assert.NoError(t, err)
	assert.Equal(t, rtHost, reporter.ResourceType())
	assert.Equal(t, repHbi, reporter.ReporterType())
	assert.Equal(t, NewJsonSchemaWithWorkspacesFromString(reporterSchema), reporter.Schema())
}

func TestNewFromJsonFile_InvalidFile(t *testing.T) {
	ctx := context.Background()
	repo, err := NewInMemorySchemaRepositoryFromJsonFile(ctx, "/tmp/nonexistent.json", DefaultSchemaFactory)
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
	repo, err := NewInMemorySchemaRepositoryFromJsonFile(ctx, tmpFile, DefaultSchemaFactory)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	rtHost, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	repHbi, err := bizmodel.NewReporterType("hbi")
	require.NoError(t, err)

	// Verify resource was loaded
	resource, err := repo.GetResourceSchema(ctx, rtHost)
	assert.NoError(t, err)
	assert.Equal(t, rtHost, resource.ResourceType())

	// Verify reporter was loaded
	reporter, err := repo.GetReporterSchema(ctx, rtHost, repHbi)
	assert.NoError(t, err)
	assert.Equal(t, rtHost, reporter.ResourceType())
	assert.Equal(t, repHbi, reporter.ReporterType())
}

func TestNewFromJsonBytes_ValidJSON(t *testing.T) {
	ctx := context.Background()

	jsonContent := []byte(`{
		"common:host": "{\"type\": \"object\"}",
		"common:k8s_cluster": "{\"type\": \"object\"}",
		"host:hbi": "{\"type\": \"object\"}",
		"k8s_cluster:acm": "{\"type\": \"object\"}"
	}`)

	repo, err := NewFromJsonBytes(ctx, jsonContent, DefaultSchemaFactory)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	rtHost, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	rtK8sCluster, err := bizmodel.NewResourceType("k8s_cluster")
	require.NoError(t, err)
	repHbi, err := bizmodel.NewReporterType("hbi")
	require.NoError(t, err)
	repAcm, err := bizmodel.NewReporterType("acm")
	require.NoError(t, err)

	// Verify resources were loaded
	resources, err := repo.GetResourceSchemas(ctx)
	assert.NoError(t, err)
	assert.Contains(t, resources, rtHost)
	assert.Contains(t, resources, rtK8sCluster)

	// Verify reporters were loaded
	hostReporter, err := repo.GetReporterSchema(ctx, rtHost, repHbi)
	assert.NoError(t, err)
	assert.Equal(t, rtHost, hostReporter.ResourceType())
	assert.Equal(t, repHbi, hostReporter.ReporterType())

	k8sReporter, err := repo.GetReporterSchema(ctx, rtK8sCluster, repAcm)
	assert.NoError(t, err)
	assert.Equal(t, rtK8sCluster, k8sReporter.ResourceType())
	assert.Equal(t, repAcm, k8sReporter.ReporterType())
}

func TestNewFromJsonBytes_InvalidJSON(t *testing.T) {
	ctx := context.Background()

	invalidJSON := []byte(`{invalid json`)

	repo, err := NewFromJsonBytes(ctx, invalidJSON, DefaultSchemaFactory)
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

	repo, err := NewFromJsonBytes(ctx, jsonContent, DefaultSchemaFactory)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	rtHost, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	rtK8sCluster, err := bizmodel.NewResourceType("k8s_cluster")
	require.NoError(t, err)

	// Verify resources were loaded
	resources, err := repo.GetResourceSchemas(ctx)
	assert.NoError(t, err)
	assert.Len(t, resources, 2)
	assert.Contains(t, resources, rtHost)
	assert.Contains(t, resources, rtK8sCluster)

	// Verify no reporters exist
	reporters, err := repo.GetReporterSchemas(ctx, rtHost)
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

			resourceType, err := bizmodel.NewResourceType(tt.resourceType)
			require.NoError(t, err)
			reporterType, err := bizmodel.NewReporterType(tt.reporterType)
			require.NoError(t, err)

			schemaContent, exists, err := loadResourceSchema(resourceType.String(), reporterType.String(), tmpDir)

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

			resourceType, err := bizmodel.NewResourceType(tt.resourceType)
			require.NoError(t, err)

			schemaContent, err := loadCommonResourceDataSchema(resourceType.String(), tmpDir)

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

func TestNewResourceType_Normalization(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{name: "forward slash replaced", input: "rhel/host", expected: "rhel_host"},
		{name: "underscore preserved", input: "k8s_cluster", expected: "k8s_cluster"},
		{name: "multiple slashes", input: "org/team/resource", expected: "org_team_resource"},
		{name: "empty string", input: "", expectErr: true},
		{name: "already normalized", input: "host", expected: "host"},
		{name: "mixed case lowered", input: "K8s_CLUSTER", expected: "k8s_cluster"},
		{name: "mixed case with slash", input: "TEST/RESOURCE", expected: "test_resource"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, err := bizmodel.NewResourceType(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, rt.String())
			assert.Equal(t, tt.expected, rt.Serialize(), "Serialize() should return already-normalized value")
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

		rtHost, err := bizmodel.NewResourceType("host")
		require.NoError(t, err)

		// Verify each reporter has its own schema
		for _, reporter := range reporters {
			rep, err := bizmodel.NewReporterType(reporter)
			require.NoError(t, err)
			schemaContent, exists, err := loadResourceSchema(rtHost.String(), rep.String(), tmpDir)
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

		repAcm, err := bizmodel.NewReporterType("acm")
		require.NoError(t, err)

		// Verify each resource has its own ACM schema
		for _, resource := range resources {
			resourceType, err := bizmodel.NewResourceType(resource)
			require.NoError(t, err)
			schemaContent, exists, err := loadResourceSchema(resourceType.String(), repAcm.String(), tmpDir)
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
		for resourceTypeName, expectedSchema := range resources {
			resourceType, err := bizmodel.NewResourceType(resourceTypeName)
			require.NoError(t, err)
			schemaContent, err := loadCommonResourceDataSchema(resourceType.String(), tmpDir)
			assert.NoError(t, err)
			assert.Equal(t, expectedSchema, schemaContent)
		}
	})
}

func TestReporterMutationsPersist(t *testing.T) {
	repo := NewInMemorySchemaRepository()
	ctx := context.Background()

	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	repHbi, err := bizmodel.NewReporterType("hbi")
	require.NoError(t, err)

	resourceSchema, err := bizmodel.NewResourceSchemaRepresentation(resourceType, validateSchemaTypeObject)
	require.NoError(t, err)
	err = repo.CreateResourceSchema(ctx, resourceSchema)
	assert.NoError(t, err)

	reporterSchema, err := bizmodel.NewReporterSchemaRepresentation(resourceType, repHbi, validateSchemaTypeObject)
	require.NoError(t, err)
	err = repo.CreateReporterSchema(ctx, reporterSchema)
	assert.NoError(t, err)

	reporters, _ := repo.GetReporterSchemas(ctx, resourceType)
	assert.Contains(t, reporters, repHbi)
}
