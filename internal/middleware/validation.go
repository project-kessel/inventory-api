package middleware

import (
	"context"
	"fmt"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"os"
	"path/filepath"
	"regexp"

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
				log.Infof("Incoming request: %+v", req)

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

	resourceType, err := ExtractStringField(reportResourceMap, "type")
	if err != nil {
		return err
	}

	reporterType, err := ExtractStringField(reportResourceMap, "reporterType")
	if err != nil {
		return err
	}

	resourceRepresentation, err := ExtractMapField(reportResourceMap, "representations")
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

var representationTypePattern = regexp.MustCompile(`^([a-z][a-z0-9_]{1,61}[a-z0-9]/)*[a-z][a-z0-9_]{1,62}[a-z0-9]$`)

func IsValidRepresentationType(val string) bool {
	return representationTypePattern.MatchString(val)
}

func ValidateRepresentationType(rt *pbv1beta2.RepresentationType) error {
	if rt == nil {
		return fmt.Errorf("object_type is required")
	}
	if !IsValidRepresentationType(rt.GetResourceType()) {
		return fmt.Errorf("resource_type does not match required pattern")
	}
	// reporter_type is optional
	if rt.ReporterType != nil && *rt.ReporterType != "" && !IsValidRepresentationType(rt.GetReporterType()) {
		return fmt.Errorf("reporter_type does not match required pattern")
	}
	return nil
}

func ValidateStreamedListObject(msg proto.Message) error {
	slo, ok := msg.(*pbv1beta2.StreamedListObjectsRequest)
	if !ok {
		return fmt.Errorf("expected StreamedListObjectsRequest")
	}
	if err := ValidateRepresentationType(slo.GetObjectType()); err != nil {
		return err
	}
	return nil
}

func StreamValidationInterceptor(validator protovalidate.Validator) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrapper := &requestValidatingWrapper{ServerStream: ss, Validator: validator}
		return handler(srv, wrapper)
	}
}

type requestValidatingWrapper struct {
	grpc.ServerStream
	protovalidate.Validator
}

func (w *requestValidatingWrapper) RecvMsg(m interface{}) error {
	err := w.ServerStream.RecvMsg(m)
	if err != nil {
		return err
	}

	if v, ok := m.(proto.Message); ok {
		switch v.(type) {
		case *pbv1beta2.StreamedListObjectsRequest:
			if err := ValidateStreamedListObject(v); err != nil {
				return errors.BadRequest("STREAMED_LIST_OBJECT_JSON_VALIDATOR", err.Error()).WithCause(err)
			}

		}
		if err = w.Validate(v); err != nil {
			return errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
		}
	}

	return nil
}

func (w *requestValidatingWrapper) SendMsg(m interface{}) error {
	return w.ServerStream.SendMsg(m)
}
