package middleware

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// LoadResourceSchema finds the resources schema based on the directory structure of data/resources
func LoadResourceSchema(resourceType string, reporterType string, dir string) (string, bool, error) {
	schemaPath := filepath.Join(dir, resourceType, "reporters", reporterType, fmt.Sprintf("%s.json", resourceType))

	// Check if file exists
	if _, err := os.Stat(schemaPath); err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to check schema file for '%s': %w", resourceType, err)
	}

	// Read file
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to read schema file for '%s': %w", resourceType, err)
	}

	return string(data), true, nil
}

// Load Common Resource Data Schema
func LoadCommonResourceDataSchema(resourceType string, baseSchemaDir string) (string, error) {

	schemaPath := filepath.Join(baseSchemaDir, resourceType, "common_representation.json")

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read common resource schema: %w", err)
	}
	return string(data), nil
}

// LoadValidReporters retrieves valid reporters for a given resource type.
// It either loads from the cache (JSON-based) or the filesystem (YAML-based).
func LoadValidReporters(resourceType string) ([]string, error) {
	if viper.GetBool("resources.use_cache") {
		return LoadFromCache(resourceType)
	}
	return LoadFromFilesystem(resourceType)
}

// LoadValidReporters Takes the resource_type from the provided config.yaml and compares it to the defined reporter_types
func LoadFromFilesystem(resourceType string) ([]string, error) {
	var config struct {
		ResourceReporters []string `yaml:"resource_reporters"`
	}

	cacheKey := fmt.Sprintf("config:%s", resourceType)
	cachedConfig, ok := SchemaCache.Load(cacheKey)
	if !ok {
		return nil, fmt.Errorf("config not found for resource type '%s'", resourceType)
	}

	configData, ok := cachedConfig.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid config data type for resource type '%s' (expected string)", resourceType)
	}

	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config for '%s': %w", resourceType, err)
	}

	if config.ResourceReporters == nil {
		return nil, fmt.Errorf("missing 'resource_reporters' field in config for '%s'", resourceType)
	}

	return config.ResourceReporters, nil
}

func LoadFromCache(resourceType string) ([]string, error) {
	cacheKey := fmt.Sprintf("config:%s", resourceType)

	cachedConfig, ok := SchemaCache.Load(cacheKey)
	if !ok {
		return nil, fmt.Errorf("config not found in cache for resource type '%s'", resourceType)
	}

	var configData []byte

	// Handle different cases
	switch v := cachedConfig.(type) {
	case string:
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			// If not Base64, assume it's plain YAML
			configData = []byte(v)
		} else {
			configData = decoded
		}
	case []byte:
		configData = v
	case map[string]interface{}:
		// Convert JSON object back to bytes
		jsonData, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON config for '%s': %w", resourceType, err)
		}
		configData = jsonData
	default:
		return nil, fmt.Errorf("unexpected data type for '%s' in cache: %T", resourceType, cachedConfig)
	}

	// Parse YAML or JSON
	var config struct {
		ResourceReporters []string `yaml:"resource_reporters" json:"resource_reporters"`
	}
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config for '%s': %w", resourceType, err)
	}

	if config.ResourceReporters == nil {
		return nil, fmt.Errorf("missing 'resource_reporters' field in cache for '%s'", resourceType)
	}

	return config.ResourceReporters, nil
}
