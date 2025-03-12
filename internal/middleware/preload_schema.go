package middleware

import (
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"sync"
)

var schemaCache sync.Map

func PreloadAllSchemas(resourceDir string) error {
	// Set default resource directory if not provided
	if resourceDir == "" {
		resourceDir = "data/schema/resources"
		log.Infof("Using local resources directory: %s", resourceDir)
	}

	// Read all resource directories
	resourceDirs, err := os.ReadDir(resourceDir)
	if err != nil {
		return fmt.Errorf("Failed to read resource directory %s: %w", resourceDir, err)
	}

	log.Infof("Reading resource directory: %s", resourceDir)

	reporterDataSchema, err := LoadReporterSchema(resourceDir)
	if err != nil {
		return fmt.Errorf("failed to load common resource schema: %w", err)
	}
	schemaCache.Store("common:reporter_data", reporterDataSchema)

	for _, dir := range resourceDirs {
		if !dir.IsDir() {
			continue
		}

		resourceType := dir.Name()
		_, err := loadConfigFile(resourceDir, resourceType)
		if err != nil {
			return err
		}

		// Load the common resource data schema
		commonResourceSchema, err := LoadCommonResourceDataSchema(resourceType, resourceDir)
		if err != nil {
			return fmt.Errorf("failed to load common resource schema: %w", err)
		}
		schemaCache.Store("common:common_resource_data", commonResourceSchema)

		// Load resource schema using LoadResourceSchema
		resourceSchema, isResourceExists, err := LoadResourceSchema(resourceType, resourceDir)
		if err != nil {
			return fmt.Errorf("failed to load resource schema for '%s': %w", resourceType, err)
		}

		if isResourceExists {
			schemaCache.Store(fmt.Sprintf("resource:%s", resourceType), resourceSchema)
		}

		// Load reporter schemas using LoadReporterSchema
		reportersDir := filepath.Join(resourceDir, resourceType, "reporters")

		if _, err := os.Stat(reportersDir); os.IsNotExist(err) {
			continue // Skip if no reporters directory exists
		}

		reporterDirs, err := os.ReadDir(reportersDir)
		if err != nil {
			return fmt.Errorf("failed to read reporters directory for '%s': %w", resourceType, err)
		}

		// Iterate through reporters and load their schemas
		normalizeResourceType := NormalizeResourceType(resourceType)
		for _, reporter := range reporterDirs {
			if !reporter.IsDir() {
				continue
			}
			reporterType := reporter.Name()
			reporterSchema, err := LoadReporterSchema(resourceDir)
			if err != nil {
				return fmt.Errorf("failed to load reporter schema for '%s' and reporter '%s': %w", resourceType, reporterType, err)
			}
			schemaCache.Store(fmt.Sprintf("%s:%s", normalizeResourceType, reporterType), reporterSchema)
		}
	}

	return nil
}

// Retrieves schema from cache
func getSchemaFromCache(cacheKey string) (string, error) {
	if cachedSchema, ok := schemaCache.Load(cacheKey); ok {
		return cachedSchema.(string), nil
	}
	return "", fmt.Errorf("schema not found for key '%s'", cacheKey)
}

func loadConfigFile(resourceDir string, resourceType string) (struct {
	ResourceType      string   `yaml:"resource_type"`
	ResourceReporters []string `yaml:"resource_reporters"`
}, error) {
	var config struct {
		ResourceType      string   `yaml:"resource_type"`
		ResourceReporters []string `yaml:"resource_reporters"`
	}
	configPath := filepath.Join(resourceDir, resourceType, "config.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file for '%s': %w", resourceType, err)
	}
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return config, fmt.Errorf("failed to unmarshal config for '%s': %w", resourceType, err)
	}
	if config.ResourceReporters == nil {
		return config, fmt.Errorf("missing 'resource_reporters' field in config for '%s'", resourceType)
	}
	configResourceType := NormalizeResourceType(config.ResourceType)
	schemaCache.Store(fmt.Sprintf("config:%s", configResourceType), configData)
	return config, nil
}
