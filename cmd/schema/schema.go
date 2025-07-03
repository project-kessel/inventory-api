package schema

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var schemaDir = "data/schema/resources"

const schemaCacheFile = "schema_cache.json"

// SchemaCache stores the structured schema data
var schemaCache = make(map[string]interface{})

// Read JSON file
func readJSONFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Normalize the resources type field in YAML content
func normalizeYAMLResourceType(yamlContent []byte) ([]byte, error) {
	// Parse YAML into a map
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(yamlContent, &yamlData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Normalize the resources type if it exists
	if resourceType, exists := yamlData["type"].(string); exists {
		normalized := middleware.NormalizeResourceType(resourceType)
		yamlData["type"] = normalized
	}

	// Convert back to YAML format
	normalizedYAML, err := yaml.Marshal(yamlData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize YAML: %w", err)
	}

	return normalizedYAML, nil
}

// Read YAML file, normalize resource type, and encode to Base64
func encodeYAMLToBase64(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	normalizedData, err := normalizeYAMLResourceType(data)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(normalizedData)
	return encoded, nil
}

// Preload schemas
func preloadSchemas() error {

	resources, err := os.ReadDir(schemaDir)
	if err != nil {
		return fmt.Errorf("failed to read schema directory: %w", err)
	}

	for _, resource := range resources {
		if !resource.IsDir() {
			continue
		}

		resourceType := middleware.NormalizeResourceType(resource.Name())
		resourcePath := filepath.Join(schemaDir, resource.Name())

		// Load common resource data schema
		commonSchemaPath := filepath.Join(resourcePath, "common_representation.json")
		if _, err := os.Stat(commonSchemaPath); err == nil {
			if jsonData, err := readJSONFile(commonSchemaPath); err == nil {
				schemaCache[fmt.Sprintf("common:%s", resourceType)] = jsonData
			}
		}

		// Load config.yaml and encode in Base64
		configPath := filepath.Join(resourcePath, "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			if encodedConfig, err := encodeYAMLToBase64(configPath); err == nil {
				schemaCache[fmt.Sprintf("config:%s", resourceType)] = encodedConfig
			}
		}

		// Process reporter schemas
		reportersPath := filepath.Join(resourcePath, "reporters")
		if reporterDirs, err := os.ReadDir(reportersPath); err == nil {
			for _, reporter := range reporterDirs {
				if !reporter.IsDir() {
					continue
				}

				reporterType := middleware.NormalizeResourceType(reporter.Name())
				reporterPath := filepath.Join(reportersPath, reporter.Name())

				// Encode reporter's config.yaml
				reporterConfigPath := filepath.Join(reporterPath, "config.yaml")
				if _, err := os.Stat(reporterConfigPath); err == nil {
					if encodedConfig, err := encodeYAMLToBase64(reporterConfigPath); err == nil {
						schemaCache[fmt.Sprintf("config:%s:%s", resourceType, reporterType)] = encodedConfig
					}
				}

				// Load reporter-specific schema JSON
				reporterSchemaPath := filepath.Join(reporterPath, fmt.Sprintf("%s.json", resource.Name()))
				if _, err := os.Stat(reporterSchemaPath); err == nil {
					if jsonData, err := readJSONFile(reporterSchemaPath); err == nil {
						schemaCache[fmt.Sprintf("%s:%s", resourceType, reporterType)] = jsonData
					}
				}
			}
		}
	}

	return nil
}

// Save schemaCache to a JSON file
func saveSchemaCache() error {
	data, err := json.MarshalIndent(schemaCache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema cache: %w", err)
	}

	if err := os.WriteFile(schemaCacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write schema cache to file: %w", err)
	}

	log.Infof("Schema cache successfully written to %s", schemaCacheFile)
	return nil
}

// NewCommand creates a new Cobra command for schema preloading
func NewCommand(loggerOptions common.LoggerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preload-schema",
		Short: "Preload schema cache from filesystem",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
			logHelper := log.NewHelper(log.With(logger, "subsystem", "schema"))

			// Preload schemas
			if err := preloadSchemas(); err != nil {
				logHelper.Errorf("Error preloading schemas: %v", err)
				return err
			}

			// Save schema cache
			if err := saveSchemaCache(); err != nil {
				logHelper.Errorf("Error saving schema cache: %v", err)
				return err
			}

			logHelper.Info("Schema cache updated successfully.")
			return nil
		},
	}

	return cmd
}
