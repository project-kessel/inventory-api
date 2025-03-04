package middleware

import (
	"context"
	"encoding/json"
	"fmt"
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

func marshalProtoToJSON(msg proto.Message) ([]byte, error) {
	data, err := protojson.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("ERROR: Failed to marshal message: %w", err)
	}
	fmt.Printf("DEBUG: Raw JSON: %s\n", string(data))
	return data, nil
}

func unmarshalJSONToMap(data []byte) (map[string]interface{}, error) {
	var resourceMap map[string]interface{}
	if err := json.Unmarshal(data, &resourceMap); err != nil {
		return nil, fmt.Errorf("ERROR: Failed to unmarshal JSON: %w", err)
	}
	fmt.Printf("DEBUG: Parsed resourceMap: %+v\n", resourceMap)
	return resourceMap, nil
}

// Extracts the top-level "resource" key from the ReportResource payload.
func extractResourceField(resourceMap map[string]interface{}) (map[string]interface{}, error) {
	resource, ok := resourceMap["resource"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("ERROR: Missing or invalid 'resource' field in payload")
	}
	fmt.Printf("DEBUG: Extracted resource: %+v\n", resource)
	return resource, nil
}

// Extracts the "resourceType" field from the resource payload.
func extractResourceType(resource map[string]interface{}) (string, error) {
	resourceTypeRaw, exists := resource["resourceType"]
	if !exists {
		return "", fmt.Errorf("ERROR: Missing 'resourceType' field in resource payload")
	}

	resourceType, ok := resourceTypeRaw.(string)
	if !ok || resourceType == "" {
		return "", fmt.Errorf("ERROR: 'resourceType' is not a valid string (got %T instead)", resourceTypeRaw)
	}
	fmt.Printf("DEBUG: Extracted resourceType: %s\n", resourceType)
	return resourceType, nil
}

// Extracts the "reporterData" map from the resource payload.
func extractReporterData(resource map[string]interface{}, resourceType string) (map[string]interface{}, error) {
	reporterData, ok := resource["reporterData"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("ERROR: Missing or invalid 'reporterData' field for resource '%s'", resourceType)
	}
	fmt.Printf("DEBUG: Extracted reporterData: %+v\n", reporterData)
	return reporterData, nil
}

// Extracts the "reporterType" from the "reporterData" field, such as ACM or HBI.
func extractResourceReporterType(reporterData map[string]interface{}, resourceType string) (string, error) {
	reporterType, ok := reporterData["reporterType"].(string)
	if !ok {
		return "", fmt.Errorf("ERROR: Missing or invalid 'reporterType' field for resource '%s'", resourceType)
	}
	fmt.Printf("DEBUG: Extracted reporterType: %s\n", reporterType)
	return reporterType, nil
}

// Validates the resource schema by checking if it exists and ensuring resourceData follows its structure.
func validateResourceSchema(resourceType string, reporterData map[string]interface{}) error {
	fmt.Println("DEBUG: Validating schema for resource type:", resourceType)
	resourceDataSchema, err := getSchemaFromCache(fmt.Sprintf("resource:%s", strings.ToLower(resourceType)))

	if err != nil {
		if _, exists := reporterData["resourceData"].(map[string]interface{}); exists {
			return fmt.Errorf("ERROR: No schema found for '%s', but 'resourceData' was provided. Submission is not allowed", resourceType)
		}
		fmt.Printf("WARNING: No schema found for '%s'. Treating as an abstract resource.\n", resourceType)
		return nil
	}

	if resourceData, exists := reporterData["resourceData"].(map[string]interface{}); exists {
		fmt.Println("DEBUG: Validating 'resourceData' against schema...")
		if err := validateJSONSchema(resourceDataSchema, resourceData); err != nil {
			return fmt.Errorf("ERROR: 'resourceData' validation failed for '%s': %w", resourceType, err)
		}
	}
	return nil
}

// Validates the "commonResourceData" field using a predefined schema.
func validateCommonResourceData(resource map[string]interface{}) error {
	fmt.Println("DEBUG: Validating 'commonResourceData' against schema...")
	commonSchema, err := getSchemaFromCache("common:common_resource_data")
	if err != nil {
		return fmt.Errorf("ERROR: Failed to load common resource schema: %w", err)
	}

	if commonResourceData, exists := resource["commonResourceData"].(map[string]interface{}); exists {
		if err := validateJSONSchema(commonSchema, commonResourceData); err != nil {
			return fmt.Errorf("ERROR: 'commonResourceData' validation failed: %w", err)
		}
	}
	return nil
}

// Validates the "reporterData" field using a predefined schema.
func validateReporterData(reporterData map[string]interface{}, resourceType string) error {
	fmt.Println("DEBUG: Validating 'reporterData' against schema...")
	reporterSchema, err := getSchemaFromCache("common:reporter_data")
	if err != nil {
		return fmt.Errorf("ERROR: Failed to load reporter schema: %w", err)
	}

	if err := validateJSONSchema(reporterSchema, reporterData); err != nil {
		return fmt.Errorf("ERROR: 'reporterData' validation failed for resource '%s': %w", resourceType, err)
	}
	return nil
}

// Extracts "local_resource_id" and "reporter_type" from a delete request and validates them.
func extractDeleteFields(resource map[string]interface{}) (string, string, error) {
	localResourceIDRaw, exists := resource["localResourceId"]
	if !exists {
		return "", "", fmt.Errorf("ERROR: Missing 'local_resource_id' field in resource payload")
	}

	localResourceID, ok := localResourceIDRaw.(string)
	if !ok || localResourceID == "" {
		return "", "", fmt.Errorf("ERROR: 'local_resource_id' is not a valid string (got %T instead)", localResourceIDRaw)
	}

	reporterTypeRaw, exists := resource["reporterType"]
	if !exists {
		return "", "", fmt.Errorf("ERROR: Missing 'reporter_type' field in resource payload")
	}

	reporterType, ok := reporterTypeRaw.(string)
	if !ok || reporterType == "" {
		return "", "", fmt.Errorf("ERROR: 'reporter_type' is not a valid string (got %T instead)", reporterTypeRaw)
	}

	fmt.Printf("DEBUG: Extracted local_resource_id: %s, reporter_type: %s\n", localResourceID, reporterType)
	return localResourceID, reporterType, nil
}

func validateJSONSchema(schemaStr string, jsonData interface{}) error {
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
