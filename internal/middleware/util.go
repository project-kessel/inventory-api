package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/xeipuuv/gojsonschema"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"strings"
)

type contextKey struct {
	name string
}

func GetFromContext[V any, I any](i I) func(context.Context) (*V, error) {
	typ := fmt.Sprintf("%T", new(V))

	return (func(ctx context.Context) (*V, error) {
		obj := ctx.Value(i)
		if obj == nil {
			return nil, fmt.Errorf("expected %s", typ)
		}
		req, ok := obj.(*V)
		if !ok {
			return nil, fmt.Errorf("object stored in request context couldn't convert to %s", typ)
		}
		return req, nil

	})
}

func NormalizeResourceType(resourceType string) string {
	return strings.ReplaceAll(resourceType, "/", "_")
}

func MarshalProtoToJSON(msg proto.Message) ([]byte, error) {
	data, err := protojson.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("ERROR: Failed to marshal message: %w", err)
	}
	return data, nil
}

func UnmarshalJSONToMap(data []byte) (map[string]interface{}, error) {
	var resourceMap map[string]interface{}
	if err := json.Unmarshal(data, &resourceMap); err != nil {
		return nil, fmt.Errorf("ERROR: Failed to unmarshal JSON: %w", err)
	}
	return resourceMap, nil
}

// Extracts a Map Field from another map
func ExtractMapField(data map[string]interface{}, key string) (map[string]interface{}, error) {
	value, exists := data[key]
	if !exists {
		return nil, fmt.Errorf("ERROR: Missing '%s' field in payload", key)
	}

	mapValue, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("ERROR: '%s' is not a valid object", key)
	}

	return mapValue, nil
}

// Extracts a String Field from a map
func ExtractStringField(data map[string]interface{}, key string) (string, error) {
	value, exists := data[key]
	if !exists {
		return "", fmt.Errorf("missing '%s' field in payload", key)
	}

	strValue, ok := value.(string)
	if !ok || strValue == "" {
		return "", fmt.Errorf("'%s' is not a valid string (got %T instead)", key, value)
	}

	return strValue, nil
}

// ValidateCommonResourceData Validates the "commonResourceData" field using a predefined schema.
func ValidateCommonData(resourceType string, commonData map[string]interface{}) error {
	commonSchemaKey := fmt.Sprintf("common:%s", strings.ToLower(resourceType))
	commonSchema, err := getSchemaFromCache(commonSchemaKey)
	if err != nil {
		return fmt.Errorf("failed to load common resource schema for '%s': %w", resourceType, err)
	}

	if err := ValidateJSONSchema(commonSchema, commonData); err != nil {
		return fmt.Errorf("common resource data validation failed for '%s': %w", resourceType, err)
	}

	return nil
}

// Validates the reporter-specific data against its schema based on resourceType and reporterType.
func ValidateReporterData(resourceType string, reporterType string, reporterData map[string]interface{}) error {
	// Construct the schema key using the format: resourceType:reporterType
	schemaKey := fmt.Sprintf("%s:%s", strings.ToLower(resourceType), strings.ToLower(reporterType))
	resourceDataSchema, err := getSchemaFromCache(schemaKey)

	// Case 1: No schema found for resourceType:reporterType
	if err != nil {
		if len(reporterData) > 0 {
			return fmt.Errorf("no schema found for '%s', but reporter data was provided. Submission is not allowed", schemaKey)
		}
		log.Debugf("no schema found for %s, treating as abstract reporter representation", schemaKey)
		return nil
	}

	// Case 2: Schema found, validate data (if present)
	if len(reporterData) > 0 {
		if err := ValidateJSONSchema(resourceDataSchema, reporterData); err != nil {
			return fmt.Errorf("reporter data validation failed for '%s': %w", schemaKey, err)
		}
	}

	return nil
}

func ValidateJSONSchema(schemaStr string, jsonData interface{}) error {
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

func GetProjectRootPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd, nil
		}

		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}

	return "", fmt.Errorf("project root not found")
}
