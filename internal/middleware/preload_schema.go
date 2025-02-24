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
		resourceDir = "data/resources"
		log.Infof("Using local resources directory")
	}

	// Read all resource directories
	resourceDirs, err := os.ReadDir(resourceDir)
	if err != nil {
		log.Fatalf("Failed to read resource directory %s: %v", resourceDir, err)
	}

	log.Infof("Reading resource directory: %s", resourceDir)

	for _, dir := range resourceDirs {
		if !dir.IsDir() {
			continue
		}

		resourceType := dir.Name()

		_, err := loadConfigFile(resourceDir, resourceType)
		if err != nil {
			return err
		}

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

		for _, reporterDir := range reporterDirs {
			if !reporterDir.IsDir() {
				continue
			}

			reporterType := reporterDir.Name()
			reporterSchema, err := LoadReporterSchema(resourceType, reporterType, resourceDir)
			if err != nil {
				return fmt.Errorf("failed to load reporter schema for '%s' and reporter '%s': %w", resourceType, reporterType, err)
			}
			normalizeResourceType := NormalizeResourceType(resourceType)
			schemaCache.Store(fmt.Sprintf("%s:%s", normalizeResourceType, reporterType), reporterSchema)
		}
	}

	return nil
}

func getSchemaFromCache(cacheKey string) (string, error) {
	//if cacheKey == "notifications/integration:notifications" {
	//	cacheKey = "notifications:notifications"
	//}
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
	c := fmt.Sprintf("config:%s", configResourceType)
	schemaCache.Store(c, configData)
	return config, nil
}
