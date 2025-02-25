package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bufbuild/protovalidate-go"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	rel "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	pb2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2/resources"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

var (
	resourceDir       = os.Getenv("RESOURCE_DIR")
	AbstractResources = map[string]struct{}{} // Tracks resource types marked as abstract (no resource_data)
)

func isDeleteRequest(v interface{}) bool {
	switch v.(type) {
	case *pb.DeleteK8SClusterRequest,
		*pb.DeleteRhelHostRequest,
		*pb.DeleteK8SPolicyRequest,
		*pb.DeleteNotificationsIntegrationRequest,
		*pb2.DeleteResourceRequest:
		return true
	default:
		return false
	}
}

func isRelationshipRequest(v interface{}) bool {
	switch v.(type) {
	case *rel.CreateK8SPolicyIsPropagatedToK8SClusterRequest,
		*rel.UpdateK8SPolicyIsPropagatedToK8SClusterRequest,
		*rel.DeleteK8SPolicyIsPropagatedToK8SClusterRequest:
		return true
	default:
		return false
	}
}

func Validation(validator protovalidate.Validator) middleware.Middleware {
	if resourceDirFilePath, exists := os.LookupEnv("RESOURCE_DIR"); exists {
		fmt.Println(resourceDirFilePath)
		absPath, err := filepath.Abs(resourceDirFilePath)
		if err != nil {
			log.Errorf("failed to resolve absolute path for RESOURCE_DIR file: %v", err)
		}
		resourceDir = absPath
	}

	if err := PreloadAllSchemas(resourceDir); err != nil {
		log.Fatalf("Failed to preload schemas: %v", err)
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if v, ok := req.(proto.Message); ok {
				if isDeleteRequest(v) || isRelationshipRequest(v) {
					// run the protovalidate validation if it is a delete or relationship request
					if err := validator.Validate(v); err != nil {
						return nil, errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
					}
				} else {
					// Otherwise, run both protovalidate and JSON validation.
					if err := validator.Validate(v); err != nil {
						return nil, errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
					}
					if err := validateResourceJSON(v); err != nil {
						return nil, errors.BadRequest("JSON_VALIDATOR", err.Error()).WithCause(err)
					}
				}
			}
			return handler(ctx, req)
		}
	}
}

func validateResourceJSON(msg proto.Message) error {
	data, err := protojson.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	var resourceMap map[string]interface{}
	if err := json.Unmarshal(data, &resourceMap); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Retrieve the dynamic resource type (top-level key)
	var resourceType string
	for key := range resourceMap {
		resourceType = key
		break
	}

	resource, ok := resourceMap[resourceType].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing or invalid resource field for resource '%s'", resourceType)
	}

	metadata, ok := resource["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing or invalid metadata field for resource '%s'", resourceType)
	}

	metadataResourceType, ok := metadata["resource_type"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid resource_type for resource '%s'", metadataResourceType)
	}

	// Normalize the resource_type for cases with /
	normalizedResourceType := NormalizeResourceType(metadataResourceType)

	resourceDataField, resourceDataExists := resource["resource_data"].(map[string]interface{})
	if !resourceDataExists {
		AbstractResources[metadataResourceType] = struct{}{}
	} else if _, isAbstract := AbstractResources[metadataResourceType]; isAbstract {
		return fmt.Errorf("resource_type '%s' is abstract and cannot have resource_data", metadataResourceType)
	} else {
		// Validate resource_data if not abstract
		resourceSchema, err := getSchemaFromCache(fmt.Sprintf("%s:%s", "resource", strings.ToLower(normalizedResourceType)))
		if err != nil {
			return fmt.Errorf("failed to load reporter schema '%s': %w", normalizedResourceType, err)
		}
		if err := validateJSONAgainstSchema(resourceSchema, resourceDataField); err != nil {
			return fmt.Errorf("resource validation failed for '%s': %w", normalizedResourceType, err)
		}
	}

	reporterData, ok := resource["reporter_data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing or invalid reporter_data field for resource '%s'", metadataResourceType)
	}

	reporterType, ok := reporterData["reporter_type"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid reporter_type for resource '%s'", resourceType)
	}

	// Check for valid resource -> reporter combinations
	if err := ValidateCombination(normalizedResourceType, reporterType); err != nil {
		return fmt.Errorf("resource-reporter compatibility validation failed for resource '%s': %w", resourceType, err)
	}

	reporterSchema, err := getSchemaFromCache(fmt.Sprintf("%s:%s", normalizedResourceType, strings.ToLower(reporterType)))
	if err != nil {
		return fmt.Errorf("failed to load reporter schema for '%s': %w", reporterType, err)
	}

	// Validate reporter_data against the reporter schema
	if err := validateJSONAgainstSchema(reporterSchema, reporterData); err != nil {
		return fmt.Errorf("reporter validation failed for resource '%s': %w", resourceType, err)
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
