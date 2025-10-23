package in_memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/project-kessel/inventory-api/internal/schema/api"
	"github.com/stretchr/testify/assert"
)

func TestInMemorySchemaRepository_CreateResource(t *testing.T) {
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	resource := api.ResourceSchema{
		ResourceType:               "host",
		CommonRepresentationSchema: `{"type": "object"}`,
	}

	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Verify resource was created
	retrieved, err := repo.GetResourceSchema(ctx, "host")
	assert.NoError(t, err)
	assert.Equal(t, "host", retrieved.ResourceType)
	assert.Equal(t, `{"type": "object"}`, retrieved.CommonRepresentationSchema)
}

func TestInMemorySchemaRepository_CreateResource_AlreadyExists(t *testing.T) {
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	resource := api.ResourceSchema{
		ResourceType:               "host",
		CommonRepresentationSchema: `{"type": "object"}`,
	}

	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Try to create the same resource again
	err = repo.CreateResourceSchema(ctx, resource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource host already exists")
}

func TestInMemorySchemaRepository_GetResource(t *testing.T) {
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	resource := api.ResourceSchema{
		ResourceType:               "k8s_cluster",
		CommonRepresentationSchema: `{"type": "object"}`,
	}

	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	retrieved, err := repo.GetResourceSchema(ctx, "k8s_cluster")
	assert.NoError(t, err)
	assert.Equal(t, "k8s_cluster", retrieved.ResourceType)
}

func TestInMemorySchemaRepository_GetResource_NotFound(t *testing.T) {
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	_, err := repo.GetResourceSchema(ctx, "nonexistent")
	assert.ErrorIs(t, err, api.ResourceSchemaNotFound)
}

func TestInMemorySchemaRepository_UpdateResource(t *testing.T) {
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	resource := api.ResourceSchema{
		ResourceType:               "host",
		CommonRepresentationSchema: `{"type": "object"}`,
	}

	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Update the resource
	updatedResource := api.ResourceSchema{
		ResourceType:               "host",
		CommonRepresentationSchema: `{"type": "object", "properties": {"name": {"type": "string"}}}`,
	}

	err = repo.UpdateResourceSchema(ctx, updatedResource)
	assert.NoError(t, err)

	// Verify update
	retrieved, err := repo.GetResourceSchema(ctx, "host")
	assert.NoError(t, err)
	assert.Equal(t, updatedResource.CommonRepresentationSchema, retrieved.CommonRepresentationSchema)
}

func TestInMemorySchemaRepository_UpdateResource_NotFound(t *testing.T) {
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	resource := api.ResourceSchema{
		ResourceType:               "nonexistent",
		CommonRepresentationSchema: `{"type": "object"}`,
	}

	err := repo.UpdateResourceSchema(ctx, resource)
	assert.ErrorIs(t, err, api.ResourceSchemaNotFound)
}

func TestInMemorySchemaRepository_DeleteResource(t *testing.T) {
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	resource := api.ResourceSchema{
		ResourceType:               "host",
		CommonRepresentationSchema: `{"type": "object"}`,
	}

	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	err = repo.DeleteResourceSchema(ctx, "host")
	assert.NoError(t, err)

	// Verify deletion
	_, err = repo.GetResourceSchema(ctx, "host")
	assert.ErrorIs(t, err, api.ResourceSchemaNotFound)
}

func TestInMemorySchemaRepository_GetResources(t *testing.T) {
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	resources := []api.ResourceSchema{
		{ResourceType: "host", CommonRepresentationSchema: `{"type": "object"}`},
		{ResourceType: "k8s_cluster", CommonRepresentationSchema: `{"type": "object"}`},
		{ResourceType: "k8s_policy", CommonRepresentationSchema: `{"type": "object"}`},
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
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	// Create resource first
	resource := api.ResourceSchema{
		ResourceType:               "host",
		CommonRepresentationSchema: `{"type": "object"}`,
	}
	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Create reporter
	reporter := api.ReporterSchema{
		ResourceType:                 "host",
		ReporterType:                 "hbi",
		ReporterRepresentationSchema: `{"type": "object", "properties": {"satellite_id": {"type": "string"}}}`,
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
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	// Create resource first
	resource := api.ResourceSchema{
		ResourceType:               "host",
		CommonRepresentationSchema: `{"type": "object"}`,
	}
	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Try to get non-existent reporter
	_, err = repo.GetReporterSchema(ctx, "host", "nonexistent")
	assert.ErrorIs(t, err, api.ReporterSchemaNotfound)
}

func TestInMemorySchemaRepository_UpdateResourceReporter(t *testing.T) {
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	// Create resource and reporter
	resource := api.ResourceSchema{
		ResourceType:               "host",
		CommonRepresentationSchema: `{"type": "object"}`,
	}
	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	reporter := api.ReporterSchema{
		ResourceType:                 "host",
		ReporterType:                 "hbi",
		ReporterRepresentationSchema: `{"type": "object"}`,
	}
	err = repo.CreateReporterSchema(ctx, reporter)
	assert.NoError(t, err)

	// Update reporter
	updatedReporter := api.ReporterSchema{
		ResourceType:                 "host",
		ReporterType:                 "hbi",
		ReporterRepresentationSchema: `{"type": "object", "properties": {"satellite_id": {"type": "string"}}}`,
	}
	err = repo.UpdateReporterSchema(ctx, updatedReporter)
	assert.NoError(t, err)

	// Verify update
	retrieved, err := repo.GetReporterSchema(ctx, "host", "hbi")
	assert.NoError(t, err)
	assert.Equal(t, updatedReporter.ReporterRepresentationSchema, retrieved.ReporterRepresentationSchema)
}

func TestInMemorySchemaRepository_DeleteResourceReporter(t *testing.T) {
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	// Create resource and reporter
	resource := api.ResourceSchema{
		ResourceType:               "host",
		CommonRepresentationSchema: `{"type": "object"}`,
	}
	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	reporter := api.ReporterSchema{
		ResourceType:                 "host",
		ReporterType:                 "hbi",
		ReporterRepresentationSchema: `{"type": "object"}`,
	}
	err = repo.CreateReporterSchema(ctx, reporter)
	assert.NoError(t, err)

	// Delete reporter
	err = repo.DeleteReporterSchema(ctx, "host", "hbi")
	assert.NoError(t, err)

	// Verify deletion
	_, err = repo.GetReporterSchema(ctx, "host", "hbi")
	assert.ErrorIs(t, err, api.ReporterSchemaNotfound)
}

func TestInMemorySchemaRepository_GetResourceReporters(t *testing.T) {
	repo := &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
	ctx := context.Background()

	// Create resource
	resource := api.ResourceSchema{
		ResourceType:               "host",
		CommonRepresentationSchema: `{"type": "object"}`,
	}
	err := repo.CreateResourceSchema(ctx, resource)
	assert.NoError(t, err)

	// Create multiple reporters
	reporters := []api.ReporterSchema{
		{ResourceType: "host", ReporterType: "hbi", ReporterRepresentationSchema: `{"type": "object"}`},
		{ResourceType: "host", ReporterType: "satellite", ReporterRepresentationSchema: `{"type": "object"}`},
		{ResourceType: "host", ReporterType: "insights", ReporterRepresentationSchema: `{"type": "object"}`},
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
	service, err := NewFromDir(ctx, "/tmp/wrong/dir")
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
	repo, err := NewFromDir(ctx, tmpDir)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify resource was loaded
	resource, err := repo.GetResourceSchema(ctx, "host")
	assert.NoError(t, err)
	assert.Equal(t, "host", resource.ResourceType)
	assert.Equal(t, commonSchema, resource.CommonRepresentationSchema)

	// Verify reporter was loaded
	reporter, err := repo.GetReporterSchema(ctx, "host", "hbi")
	assert.NoError(t, err)
	assert.Equal(t, "host", reporter.ResourceType)
	assert.Equal(t, "hbi", reporter.ReporterType)
	assert.Equal(t, reporterSchema, reporter.ReporterRepresentationSchema)
}

func TestNewFromJsonFile_InvalidFile(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFromJsonFile(ctx, "/tmp/nonexistent.json")
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
	repo, err := NewFromJsonFile(ctx, tmpFile)
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

	repo, err := NewFromJsonBytes(ctx, jsonContent)
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

	repo, err := NewFromJsonBytes(ctx, invalidJSON)
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

	repo, err := NewFromJsonBytes(ctx, jsonContent)
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
	repo := New()
	assert.NotNil(t, repo)
	assert.NotNil(t, repo.content)
}
