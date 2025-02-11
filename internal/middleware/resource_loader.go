package middleware

import (
	"github.com/go-kratos/kratos/v2/log"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

// Config struct to read `resource_type` from `config.yaml`
type Config struct {
	ResourceType string `yaml:"resource_type"`
}

// Reads `config.yaml` and extracts `resource_type`
func getResourceType(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", err
	}

	if config.ResourceType == "" {
		return "", err // Ensures we don't add empty resource types
	}

	return config.ResourceType, nil
}

// LoadResources loads in the resources configured in  the data/resources directory
func LoadResources() {
	if resourceDir == "" {
		resourceDir = "data/resources"
		log.Infof("Using local resources directory")
	}

	// Read all directories in `resourceDir`
	resourceDirs, err := os.ReadDir(resourceDir)
	if err != nil {
		log.Fatalf("Failed to read resource directory %s: %v", resourceDir, err)
	}

	log.Infof("Reading resource directory: %s", resourceDir)

	for _, dir := range resourceDirs {
		if !dir.IsDir() {
			continue
		}

		// Locate `config.yaml`
		configPath := filepath.Join(resourceDir, dir.Name(), "config.yaml")

		// Skip if `config.yaml` does not exist
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			log.Warnf("Skipping %s, no config.yaml found at %s", dir.Name(), configPath)
			continue
		}

		// Extract `resource_type` from `config.yaml`
		resourceType, err := getResourceType(configPath)
		if err != nil {
			log.Warnf("Failed to read resource_type %s from %s: %v", resourceType, configPath, err)
			continue
		}

	}
}
