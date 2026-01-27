package middleware

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
)

// Validation creates a middleware that performs proto validation and sanitizes request data.
// Schema-based validation (reporter/resource combination, common and reporter representations)
// is handled in the business layer (resources.Usecase.ReportResource).
func Validation(validator protovalidate.Validator) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if v, ok := req.(proto.Message); ok {
				if err := validator.Validate(v); err != nil {
					return nil, errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
				}

				// Sanitize ReportResourceRequest to remove null values from reporter representation
				if rr, ok := v.(*pbv1beta2.ReportResourceRequest); ok {
					if err := sanitizeReportResourceRequest(rr); err != nil {
						return nil, errors.BadRequest("SANITIZER", err.Error()).WithCause(err)
					}
				}
			}
			return handler(ctx, req)
		}
	}
}

// sanitizeReportResourceRequest removes null values from the reporter representation
// so downstream layers don't need to handle them.
func sanitizeReportResourceRequest(rr *pbv1beta2.ReportResourceRequest) error {
	if rr.GetRepresentations() == nil || rr.GetRepresentations().GetReporter() == nil {
		return nil
	}

	reporterMap := rr.GetRepresentations().GetReporter().AsMap()
	if reporterMap == nil {
		return nil
	}

	sanitized := RemoveNulls(reporterMap)
	if sanitized == nil {
		return nil
	}

	sanitizedStruct, err := structpb.NewStruct(sanitized)
	if err != nil {
		return fmt.Errorf("failed to rebuild reporter struct: %w", err)
	}
	rr.Representations.Reporter = sanitizedStruct

	return nil
}
