package middleware

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

func ValidateCombination(resourceType, reporterType string) error {
	resourceReporters, err := LoadValidReporters(resourceType)
	if err != nil {
		return fmt.Errorf("failed to load valid reporters for '%s': %w", resourceType, err)
	}

	// check if the resources reporter_type exists in the list of resource_reporters
	for _, validReporter := range resourceReporters {
		if reporterType == validReporter {
			return nil
		}
	}
	return fmt.Errorf("invalid reporter_type: %s for resource_type: %s", reporterType, resourceType)
}

// LoadResourceSchema finds the resources schema based on the directory structure of data/resources
func LoadResourceSchema(resourceType string, dir string) (string, bool, error) {
	schemaPath := filepath.Join(dir, resourceType, resourceType+".json")
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

// LoadReporterSchema finds the reporters schemas based on the directory structure of data/resources
func LoadReporterSchema(resourceType string, reporterType string, dir string) (string, error) {
	schemaPath := filepath.Join(dir, resourceType, "reporters", reporterType, reporterType+".json")

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read schema file for '%s' and reporter '%s': %w", resourceType, reporterType, err)
	}
	return string(data), nil
}

// LoadValidReporters Takes the resource_type from the provided config.yaml and compares it to the defined reporter_types
func LoadValidReporters(resourceType string) ([]string, error) {
	var config struct {
		ResourceReporters []string `yaml:"resource_reporters"`
	}

	cacheKey := fmt.Sprintf("config:%s", resourceType)
	cachedConfig, ok := schemaCache.Load(cacheKey)
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
