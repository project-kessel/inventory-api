package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bufbuild/protovalidate-go"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

const defaultResourceDir = "data/resources"

var (
	resourceDir          = os.Getenv("RESOURCE_DIR")
	AllowedResourceTypes = map[string]struct{}{}
	AbstractResources    = map[string]struct{}{} // Tracks resource types marked as abstract (no resource_data)
)

func Validation(validator *protovalidate.Validator) middleware.Middleware {
	if resourceDir == "" {
		resourceDir = defaultResourceDir
	}
	resourceDirs, err := os.ReadDir(resourceDir)
	if err != nil {
		log.Fatalf("Failed to read resource directory %s: %v", resourceDir, err)
	}
	log.Infof("Read resource directory %s:", resourceDir)

	for _, dir := range resourceDirs {
		if !dir.IsDir() {
			continue
		}

		AllowedResourceTypes[dir.Name()] = struct{}{}
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if v, ok := req.(proto.Message); ok {
				if err := validator.Validate(v); err != nil {
					return nil, errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
				}
				if err := ValidateResourceJSON(v); err != nil {
					return nil, errors.BadRequest("JSON_VALIDATOR", err.Error()).WithCause(err)
				}
			}
			return handler(ctx, req)
		}
	}
}

func ValidateResourceJSON(msg proto.Message) error {
	data, err := protojson.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	var resourceMap map[string]interface{}
	if err := json.Unmarshal(data, &resourceMap); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	resource, ok := resourceMap["resource"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing or invalid resource field")
	}

	metadata, ok := resource["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing or invalid metadata field")
	}
	resourceType, ok := metadata["resource_type"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid resource_type")
	}

	// Check for `resource_data` and handle abstract resources
	resourceData, resourceDataExists := resource["resource_data"].(map[string]interface{})
	if !resourceDataExists {
		AbstractResources[resourceType] = struct{}{}
	} else if _, isAbstract := AbstractResources[resourceType]; isAbstract {
		return fmt.Errorf("resource_type '%s' is abstract and cannot have resource_data", resourceType)
	} else {
		// Validate `resource_data` if present and not abstract
		resourceSchema, err := loadSchema(resourceType)
		if err != nil {
			return fmt.Errorf("failed to load schema for '%s': %w", resourceType, err)
		}
		if err := validateJSONAgainstSchema(resourceSchema, resourceData); err != nil {
			return fmt.Errorf("resource validation failed: %w", err)
		}
	}

	reporterData, ok := resource["reporter_data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing or invalid reporter_data field")
	}
	reporterType, ok := reporterData["reporter_type"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid reporter_type")
	}

	// Check for valid resource -> reporter combinations
	if err := ValidateCombination(resourceType, reporterType); err != nil {
		return fmt.Errorf("resource-reporter compatibility validation failed: %w", err)
	}

	reporterSchema, err := loadReporterSchema(resourceType, strings.ToLower(reporterType))
	if err != nil {
		return fmt.Errorf("failed to load reporter schema for '%s': %w", reporterType, err)
	}

	// Validate `reporter_data` against the reporter schema
	if err := validateJSONAgainstSchema(reporterSchema, reporterData); err != nil {
		return fmt.Errorf("reporter validation failed: %w", err)
	}

	return nil
}

func validateJSONAgainstSchema(schemaStr string, jsonData interface{}) error {
	schemaLoader := gojsonschema.NewStringLoader(schemaStr)
	dataLoader := gojsonschema.NewGoLoader(jsonData)

	result, err := gojsonschema.Validate(schemaLoader, dataLoader)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}
	if !result.Valid() {
		var errMsgs []string
		for _, desc := range result.Errors() {
			errMsgs = append(errMsgs, desc.String())
		}
		return fmt.Errorf("validation failed: %s", strings.Join(errMsgs, "; "))
	}
	return nil
}

func ValidateCombination(resourceType, reporterType string) error {
	resourceReporters, err := loadValidReporters(resourceType)
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

func loadSchema(resourceType string) (string, error) {
	schemaPath := filepath.Join(resourceDir, resourceType, resourceType+".json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read schema file for '%s': %w", resourceType, err)
	}
	return string(data), nil
}

func loadReporterSchema(resourceType string, reporterType string) (string, error) {
	schemaPath := filepath.Join(resourceDir, resourceType, "reporters", reporterType, reporterType+".json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read schema file for '%s' and reporter '%s': %w", resourceType, reporterType, err)
	}
	return string(data), nil
}

func loadValidReporters(resourceType string) ([]string, error) {
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
