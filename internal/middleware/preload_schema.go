package middleware

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var schemaCache sync.Map

func PreloadAllSchemas(resourceDir string) error {
	if viper.GetBool("resources.use_cache") {
		if err := LoadSchemaCacheFromJSON("schema_cache.json"); err != nil {
			log.Errorf("Failed to load schema cache from JSON: %v", err)
			return err
		}
		log.Info("Using JSON cache based on resources directory")
	} else {
		if err := PreloadAllSchemasFromFilesystem(resourceDir); err != nil {
			log.Errorf("Failed to preload schemas from filesystem: %v", err)
			return err
		}
		log.Infof("Using local resources directory: %s", resourceDir)
	}
	return nil
}

func PreloadAllSchemasFromFilesystem(resourceDir string) error {
	if resourceDir == "" {
		resourceDir = viper.GetString("resources.schemaPath")
	}
	resourceDirs, err := os.ReadDir(resourceDir)
	if err != nil {
		return fmt.Errorf("no directories inside schema directory")
	}

	for _, dir := range resourceDirs {
		if !dir.IsDir() {
			continue
		}
		resourceType := NormalizeResourceType(dir.Name())

		// Load and store common resource schema
		commonResourceSchema, err := LoadCommonResourceDataSchema(resourceType, resourceDir)
		if err == nil {
			schemaCache.Store(fmt.Sprintf("common:%s", resourceType), commonResourceSchema)
		}

		_, err = loadConfigFile(resourceDir, resourceType)
		if err != nil {
			log.Errorf("Failed to load config file for '%s': %v", resourceType, err)
			return err
		}

		reportersDir := filepath.Join(resourceDir, resourceType, "reporters")
		if _, err := os.Stat(reportersDir); os.IsNotExist(err) {
			continue
		}

		reporterDirs, err := os.ReadDir(reportersDir)
		if err != nil {
			log.Errorf("Failed to read reporters directory for '%s': %v", resourceType, err)
			continue
		}

		for _, reporter := range reporterDirs {
			if !reporter.IsDir() {
				continue
			}
			reporterType := reporter.Name()
			reporterSchema, isReporterSchemaExists, err := LoadResourceSchema(resourceType, reporterType, resourceDir)
			if err == nil && isReporterSchemaExists {
				schemaCache.Store(fmt.Sprintf("%s:%s", resourceType, reporterType), reporterSchema)
			} else {
				log.Warnf("No schema found for %s:%s", resourceType, reporterType)
			}
		}
	}

	return nil
}

// LoadSchemaCacheFromJSON loads schema cache from a JSON file
func LoadSchemaCacheFromJSON(filePath string) error {
	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read schema cache file: %w", err)
	}

	cacheMap := make(map[string]interface{})
	err = json.Unmarshal(jsonData, &cacheMap)
	if err != nil {
		return fmt.Errorf("failed to unmarshal schema cache JSON: %w", err)
	}

	for key, value := range cacheMap {
		schemaCache.Store(key, value)
	}

	log.Infof("Schema cache successfully loaded from %s", filePath)
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
