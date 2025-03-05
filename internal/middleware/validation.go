package middleware

import (
	"context"
	"fmt"
	"github.com/bufbuild/protovalidate-go"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
)

var (
	resourceDir       = os.Getenv("RESOURCE_DIR")
	AbstractResources = map[string]struct{}{} // Tracks resource types marked as abstract (no resource_data)
)

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
				if err := validator.Validate(v); err != nil {
					return nil, errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
				}

				switch v.(type) {
				case *pbv1beta2.ReportResourceRequest:
					if err := validateResourceReporterJSON(v); err != nil {
						return nil, errors.BadRequest("REPORT_RESOURCE_JSON_VALIDATOR", err.Error()).WithCause(err)
					}
				case *pbv1beta2.DeleteResourceRequest:
					if err := validateResourceDeletionJSON(v); err != nil {
						return nil, errors.BadRequest("DELETE_RESOURCE_JSON_VALIDATOR", err.Error()).WithCause(err)
					}
				}
			}
			return handler(ctx, req)
		}
	}
}

func validateResourceReporterJSON(msg proto.Message) error {
	data, err := marshalProtoToJSON(msg)
	if err != nil {
		return err
	}

	reportResourceMap, err := unmarshalJSONToMap(data)
	if err != nil {
		return err
	}

	resource, err := extractMapField(reportResourceMap, "resource")
	if err != nil {
		return err
	}

	resourceType, err := extractStringField(resource, "resourceType")
	if err != nil {
		return err
	}

	reporterData, err := extractMapField(resource, "reporterData")
	if err != nil {
		return err
	}

	reporterType, err := extractStringField(reporterData, "reporterType")
	if err != nil {
		return err
	}

	if err := ValidateResourceReporterCombination(resourceType, reporterType); err != nil {
		return err
	}

	if err := validateReporterResourceData(resourceType, reporterData); err != nil {
		return err
	}

	if err := validateCommonResourceData(resource); err != nil {
		return err
	}

	if err := validateReporterData(reporterData, resourceType); err != nil {
		return err
	}

	return nil
}

// Validates resource deletion by extracting required fields from the request.
func validateResourceDeletionJSON(msg proto.Message) error {
	data, err := marshalProtoToJSON(msg)
	if err != nil {
		return err
	}

	deleteResourceMap, err := unmarshalJSONToMap(data)
	if err != nil {
		return err
	}

	_, err = extractStringField(deleteResourceMap, "localResourceId")
	if err != nil {
		return err
	}

	_, err = extractStringField(deleteResourceMap, "reporterType")
	if err != nil {
		return err
	}

	return nil
}
