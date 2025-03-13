package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/xeipuuv/gojsonschema"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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
			return nil, fmt.Errorf("Expected %s", typ)
		}
		req, ok := obj.(*V)
		if !ok {
			return nil, fmt.Errorf("Object stored in request context couldn't convert to %s", typ)
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

// Validates the "commonResourceData" field using a predefined schema.
func ValidateCommonResourceData(resource map[string]interface{}) error {
	commonSchema, err := getSchemaFromCache("common:common_resource_data")
	if err != nil {
		return fmt.Errorf("failed to load common resource schema: %w", err)
	}

	if commonResourceData, exists := resource["commonResourceData"].(map[string]interface{}); exists {
		if err := ValidateJSONSchema(commonSchema, commonResourceData); err != nil {
			return fmt.Errorf("commonResourceData validation failed: %w", err)
		}
	}
	return nil
}

// Validates the "reporterData" field using a predefined schema.
func validateReporterData(reporterData map[string]interface{}, resourceType string) error {
	reporterSchema, err := getSchemaFromCache("common:reporter_data")
	if err != nil {
		return fmt.Errorf("failed to load reporter schema: %w", err)
	}

	if err := ValidateJSONSchema(reporterSchema, reporterData); err != nil {
		return fmt.Errorf("reporterData validation failed for resource '%s': %w", resourceType, err)
	}
	return nil
}

// Validates the reporter resource schema by checking if it exists and ensuring resourceData follows its structure.
func ValidateReporterResourceData(resourceType string, reporterData map[string]interface{}) error {
	resourceDataSchema, err := getSchemaFromCache(fmt.Sprintf("resource:%s", strings.ToLower(resourceType)))

	resourceData, hasResourceData := reporterData["resourceData"]

	// Case 1: If no schema exists for the resourceType, treat it as an abstract resource and log a warning
	if err != nil {
		if hasResourceData {
			// If resourceData is provided but no schema is found, return error
			return fmt.Errorf("no schema found for '%s', but 'resourceData' was provided. Submission is not allowed", resourceType)
		}
		log.Debugf("no schema found for %s, treating as an abstract resource", resourceType)
		return nil
	}

	// Case 2: If resourceData is provided, validate it
	if hasResourceData {
		if resourceDataMap, ok := resourceData.(map[string]interface{}); ok {
			if len(resourceDataMap) == 0 {
				// If resourceData is empty but the schema allows it, pass validation
			} else {
				// If resourceData is populated, validate it against the schema
				if err := ValidateJSONSchema(resourceDataSchema, resourceDataMap); err != nil {
					return fmt.Errorf("resourceData validation failed for '%s': %w", resourceType, err)
				}
			}
		} else {
			// If resourceData is not a valid map, return an error
			return fmt.Errorf("resourceData should be a valid map for resource '%s'", resourceType)
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
