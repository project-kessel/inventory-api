package middleware

import (
	"context"
	"fmt"

	"github.com/spf13/viper"

	"os"
	"path/filepath"

	"buf.build/go/protovalidate"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

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
					if err := ValidateReportResourceJSON(v); err != nil {
						return nil, errors.BadRequest("REPORT_RESOURCE_JSON_VALIDATOR", err.Error()).WithCause(err)
					}
				}
			}
			return handler(ctx, req)
		}
	}
}

func ValidateReportResourceJSON(msg proto.Message) error {
	data, err := MarshalProtoToJSON(msg)
	if err != nil {
		return err
	}

	reportResourceMap, err := UnmarshalJSONToMap(data)
	if err != nil {
		return err
	}

	resourceType, err := ExtractStringField(reportResourceMap, "type", ValidateFieldExists())
	if err != nil {
		return err
	}

	reporterType, err := ExtractStringField(reportResourceMap, "reporterType", ValidateFieldExists())
	if err != nil {
		return err
	}

	// Validate the combination of resource_type and reporter_type e.g. k8s_cluster & ACM
	if err := ValidateResourceReporterCombination(resourceType, reporterType); err != nil {
		return err
	}

	representations, err := ExtractMapField(reportResourceMap, "representations", ValidateFieldExists())
	if err != nil {
		return err
	}

	reporterRepresentation, err := ExtractMapField(representations, "reporter")
	if err != nil {
		return err
	}

	// Remove any fields with null values before validation.
	var sanitizedReporterRepresentation map[string]interface{}
	if reporterRepresentation != nil {
		sanitizedReporterRepresentation = RemoveNulls(reporterRepresentation)
	} else {
		sanitizedReporterRepresentation = nil
	}

	// Overwrite the proto's reporter struct with the sanitized version so downstream layers don't see nulls.
	if rr, ok := msg.(*pbv1beta2.ReportResourceRequest); ok && sanitizedReporterRepresentation != nil {
		sanitizedStruct, err2 := structpb.NewStruct(sanitizedReporterRepresentation)
		if err2 != nil {
			return fmt.Errorf("failed to rebuild reporter struct: %w", err2)
		}
		rr.Representations.Reporter = sanitizedStruct
	}

	// Validate reporter-specific data using the sanitized map
	if err := ValidateReporterRepresentation(resourceType, reporterType, sanitizedReporterRepresentation); err != nil {
		return err
	}

	commonRepresentation, err := ExtractMapField(representations, "common")
	if err != nil {
		return err
	}

	// Validate common data
	if err := ValidateCommonRepresentation(resourceType, commonRepresentation); err != nil {
		return err
	}

	return nil
}
