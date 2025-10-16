package in_memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func loadResourceSchema(resourceType string, reporterType string, dir string) (string, bool, error) {
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

func loadCommonResourceDataSchema(resourceType string, baseSchemaDir string) (string, error) {

	schemaPath := filepath.Join(baseSchemaDir, resourceType, "common_representation.json")

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read common resource schema: %w", err)
	}
	return string(data), nil
}

func normalizeResourceType(resourceType string) string {
	return strings.ReplaceAll(resourceType, "/", "_")
}
