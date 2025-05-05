package middleware

import (
	"context"

	"github.com/spf13/viper"

	"os"
	"path/filepath"

	"github.com/bufbuild/protovalidate-go"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"google.golang.org/protobuf/proto"

	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
)

var (
	resourceDir = viper.GetString("resources.schemaPath")
)

func Validation(validator protovalidate.Validator) middleware.Middleware {
	if resourceDirFilePath, exists := os.LookupEnv("RESOURCE_DIR"); exists {
		absPath, err := filepath.Abs(resourceDirFilePath)
		if err != nil {
			log.Errorf("failed to resolve absolute path for RESOURCE_DIR file: %v", err)
		}
		resourceDir = absPath
	}

	if err := PreloadAllSchemas(resourceDir); err != nil {
		log.Warnf("Failed to preload schemas: %v", err)
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if v, ok := req.(proto.Message); ok {
				if err := validator.Validate(v); err != nil {
					return nil, errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
				}

				switch v.(type) {
				case *pbv1beta2.ReportResourceRequest:
					if err := validateReportResourceJSON(v); err != nil {
						return nil, errors.BadRequest("REPORT_RESOURCE_JSON_VALIDATOR", err.Error()).WithCause(err)
					}
				}
			}
			return handler(ctx, req)
		}
	}
}

func validateReportResourceJSON(msg proto.Message) error {
	data, err := MarshalProtoToJSON(msg)
	if err != nil {
		return err
	}

	reportResourceMap, err := UnmarshalJSONToMap(data)
	if err != nil {
		return err
	}

	resource, err := ExtractMapField(reportResourceMap, "resource")
	if err != nil {
		return err
	}

	resourceType, err := ExtractStringField(resource, "type")
	if err != nil {
		return err
	}

	reporterType, err := ExtractStringField(resource, "reporterType")
	if err != nil {
		return err
	}

	resourceRepresentation, err := ExtractMapField(resource, "representations")
	if err != nil {
		return err
	}

	reporterRepresentation, err := ExtractMapField(resourceRepresentation, "reporter")
	if err != nil {
		return err
	}

	commonRepresentation, err := ExtractMapField(resourceRepresentation, "common")
	if err != nil {
		return err
	}

	// Validate the combination of resource_type and reporter_type e.g. k8s_cluster & ACM
	if err := ValidateResourceReporterCombination(resourceType, reporterType); err != nil {
		return err
	}

	// Validate reporter-specific data
	if err := ValidateReporterRepresentation(resourceType, reporterType, reporterRepresentation); err != nil {
		return err
	}

	// Validate common data
	if err := ValidateCommonRepresentation(resourceType, commonRepresentation); err != nil {
		return err
	}

	return nil
}
