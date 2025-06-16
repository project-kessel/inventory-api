package v1beta2_test

import (
	"encoding/json"
	"testing"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestRepresentationMetadata_Getters(t *testing.T) {
	consoleHref := "https://console.example.com"
	reporterVersion := "v1.0.1"

	meta := &v1beta2.RepresentationMetadata{
		LocalResourceId: "abc-123",
		ApiHref:         "https://api.example.com/resource/abc-123",
		ConsoleHref:     &consoleHref,
		ReporterVersion: &reporterVersion,
	}

	assert.Equal(t, "abc-123", meta.GetLocalResourceId())
	assert.Equal(t, "https://api.example.com/resource/abc-123", meta.GetApiHref())
	assert.Equal(t, "https://console.example.com", meta.GetConsoleHref())
	assert.Equal(t, "v1.0.1", meta.GetReporterVersion())
}

func TestRepresentationMetadata_Getters_NilOptionals(t *testing.T) {
	meta := &v1beta2.RepresentationMetadata{
		LocalResourceId: "xyz-789",
		ApiHref:         "https://api.example.com/resource/xyz-789",
	}

	assert.Equal(t, "", meta.GetConsoleHref(), "Expected empty string for nil ConsoleHref")
	assert.Equal(t, "", meta.GetReporterVersion(), "Expected empty string for nil ReporterVersion")
}

func TestRepresentationMetadata_InvalidJSON_Unmarshal(t *testing.T) {
	invalidJSON := `{"local_resource_id": "bad-id", "api_href": 123}` // api_href should be a string

	var meta v1beta2.RepresentationMetadata
	err := json.Unmarshal([]byte(invalidJSON), &meta)

	assert.Error(t, err, "Expected error when unmarshalling invalid JSON types")
}

func TestRepresentationMetadata_EmptyJSON_Unmarshal(t *testing.T) {
	emptyJSON := `{}`

	var meta v1beta2.RepresentationMetadata
	err := json.Unmarshal([]byte(emptyJSON), &meta)

	assert.NoError(t, err)
	assert.Equal(t, "", meta.GetLocalResourceId())
	assert.Equal(t, "", meta.GetApiHref())
	assert.Equal(t, "", meta.GetConsoleHref())
	assert.Equal(t, "", meta.GetReporterVersion())
}

func TestRepresentationMetadata_PartialFields(t *testing.T) {
	jsonData := `{
		"local_resource_id": "partial-id",
		"console_href": "https://console.partial"
	}`

	var meta v1beta2.RepresentationMetadata
	err := json.Unmarshal([]byte(jsonData), &meta)

	assert.NoError(t, err)
	assert.Equal(t, "partial-id", meta.GetLocalResourceId())
	assert.Equal(t, "", meta.GetApiHref())
	assert.Equal(t, "https://console.partial", meta.GetConsoleHref())
	assert.Equal(t, "", meta.GetReporterVersion())
}
