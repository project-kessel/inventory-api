package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/xeipuuv/gojsonschema"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}
	return data, nil
}

func UnmarshalJSONToMap(data []byte) (map[string]interface{}, error) {
	var resourceMap map[string]interface{}
	if err := json.Unmarshal(data, &resourceMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return resourceMap, nil
}

// ExtractOption configures extraction behavior
type ExtractOption func(*extractConfig)

type extractConfig struct {
	validateFieldExists bool
}

// ValidateFieldExists makes the extraction fail if the field doesn't exist
func ValidateFieldExists() ExtractOption {
	return func(c *extractConfig) {
		c.validateFieldExists = true
	}
}

// Extracts a Map Field from another map
func ExtractMapField(data map[string]interface{}, key string, opts ...ExtractOption) (map[string]interface{}, error) {
	config := &extractConfig{validateFieldExists: false}
	for _, opt := range opts {
		opt(config)
	}

	value, exists := data[key]
	if !exists {
		if config.validateFieldExists {
			return nil, fmt.Errorf("missing '%s' field in payload", key)
		}
		return nil, nil // Return nil without error when field doesn't exist and not required
	}

	mapValue, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("'%s' is not a valid object", key)
	}

	return mapValue, nil
}

// Extracts a String Field from a map
func ExtractStringField(data map[string]interface{}, key string, opts ...ExtractOption) (string, error) {
	config := &extractConfig{validateFieldExists: false}
	for _, opt := range opts {
		opt(config)
	}

	value, exists := data[key]
	if !exists {
		if config.validateFieldExists {
			return "", fmt.Errorf("missing '%s' field in payload", key)
		}
		return "", nil // Return empty string without error when field doesn't exist and not required
	}

	strValue, ok := value.(string)
	if !ok || strValue == "" {
		return "", fmt.Errorf("'%s' is not a valid string (got %T instead)", key, value)
	}

	return strValue, nil
}

// ValidateCommonRepresentation Validates the "common" field in ResourceRepresentations using a predefined schema.
func ValidateCommonRepresentation(resourceType string, commonRepresentation map[string]interface{}) error {
	commonSchemaKey := fmt.Sprintf("common:%s", strings.ToLower(resourceType))
	commonSchema, err := GetSchemaFromCache(commonSchemaKey)
	if err != nil {
		return fmt.Errorf("failed to load common representation schema for '%s': %w", resourceType, err)
	}

	// Check if schema has required fields to determine if commonRepresentation is required
	hasRequiredFields, err := schemaHasRequiredFields(commonSchema)
	if err != nil {
		return fmt.Errorf("failed to analyze common schema for '%s': %w", resourceType, err)
	}

	// If schema has required fields but commonRepresentation is nil/empty, that's an error
	if hasRequiredFields && len(commonRepresentation) == 0 {
		return fmt.Errorf("missing 'common' field in payload - schema for '%s' has required fields", resourceType)
	}

	// Validate data if present
	if len(commonRepresentation) > 0 {
		if err := ValidateJSONSchema(commonSchema, commonRepresentation); err != nil {
			return fmt.Errorf("common representation validation failed for '%s': %w", resourceType, err)
		}
	}

	return nil
}

// schemaHasRequiredFields checks if a JSON schema has any required fields
func schemaHasRequiredFields(schemaStr string) (bool, error) {
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		return false, fmt.Errorf("failed to parse schema: %w", err)
	}

	required, exists := schema["required"]
	if !exists {
		return false, nil
	}

	requiredArray, ok := required.([]interface{})
	if !ok {
		return false, nil
	}

	return len(requiredArray) > 0, nil
}

// Validates the reporter-specific representation against its schema based on resourceType and reporterType.
func ValidateReporterRepresentation(resourceType string, reporterType string, reporterRepresentation map[string]interface{}) error {
	// Construct the schema key using the format: resourceType:reporterType
	schemaKey := fmt.Sprintf("%s:%s", strings.ToLower(resourceType), strings.ToLower(reporterType))
	reporterRepresentationSchema, err := GetSchemaFromCache(schemaKey)

	// Case 1: No schema found for resourceType:reporterType
	if err != nil {
		if len(reporterRepresentation) > 0 {
			return fmt.Errorf("no schema found for '%s', but reporter representation was provided. Submission is not allowed", schemaKey)
		}
		log.Debugf("no schema found for %s, treating as abstract reporter representation", schemaKey)
		return nil
	}

	// Case 2: Schema found - check if it has required fields to determine if reporterRepresentation is required
	hasRequiredFields, err := schemaHasRequiredFields(reporterRepresentationSchema)
	if err != nil {
		return fmt.Errorf("failed to analyze schema for '%s': %w", schemaKey, err)
	}

	// If schema has required fields but reporterRepresentation is nil/empty, that's an error
	if hasRequiredFields && len(reporterRepresentation) == 0 {
		return fmt.Errorf("missing 'reporter' field in payload - schema for '%s' has required fields", schemaKey)
	}

	// Case 3: Validate data if present
	if len(reporterRepresentation) > 0 {
		if err := ValidateJSONSchema(reporterRepresentationSchema, reporterRepresentation); err != nil {
			return fmt.Errorf("reporter representation validation failed for '%s': %w", schemaKey, err)
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
