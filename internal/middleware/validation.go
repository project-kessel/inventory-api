package middleware

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"google.golang.org/grpc"
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

	return sanitizeStruct(&rr.Representations.Reporter, "reporter")
}

// StreamValidationInterceptor returns a gRPC stream interceptor that validates
// incoming messages using protovalidate. This ensures streaming RPCs (e.g.
// StreamedListObjects) receive the same proto validation as unary RPCs.
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
	if err := w.ServerStream.RecvMsg(m); err != nil {
		return err
	}

	if v, ok := m.(proto.Message); ok {
		if err := w.Validate(v); err != nil {
			return errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
		}
	}

	return nil
}

func sanitizeStruct(s **structpb.Struct, name string) error {
	if *s == nil {
		return nil
	}

	m := (*s).AsMap()
	if m == nil || !hasNullsRecursive(m) {
		return nil
	}

	sanitized := RemoveNulls(m)
	rebuilt, err := structpb.NewStruct(sanitized)
	if err != nil {
		return fmt.Errorf("failed to rebuild %s struct: %w", name, err)
	}
	*s = rebuilt
	return nil
}
