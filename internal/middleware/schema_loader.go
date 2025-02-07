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
func LoadResourceSchema(resourceType string) (string, error) {
	schemaPath := filepath.Join(resourceDir, resourceType, resourceType+".json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read schema file for '%s': %w", resourceType, err)
	}
	return string(data), nil
}

// LoadReporterSchema finds the reporters schemas based on the directory structure of data/resources
func LoadReporterSchema(resourceType string, reporterType string) (string, error) {
	schemaPath := filepath.Join(resourceDir, resourceType, "reporters", reporterType, reporterType+".json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read schema file for '%s' and reporter '%s': %w", resourceType, reporterType, err)
	}
	return string(data), nil
}

// LoadValidReporters Takes the resource_type from the provided config.yaml and compares it to the defined reporter_types
func LoadValidReporters(resourceType string) ([]string, error) {
	configPath := filepath.Join(resourceDir, resourceType, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file for '%s': %w", resourceType, err)
	}

	var config struct {
		ResourceReporters []string `yaml:"resource_reporters"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config for '%s': %w", resourceType, err)
	}

	if config.ResourceReporters == nil {
		return nil, fmt.Errorf("missing 'resource_reporters' field in config for '%s'", resourceType)
	}

	return config.ResourceReporters, nil
}
