package middleware

import (
	"context"
	"errors"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
)

// ErrorMapping returns a middleware that maps domain errors to gRPC status codes.
// Errors that already have a gRPC status code (other than Unknown) pass through unchanged.
func ErrorMapping() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			resp, err := next(ctx, req)
			if err != nil {
				err = MapError(err)
			}
			return resp, err
		}
	}
}

// ErrorMappingStreamInterceptor returns a gRPC stream interceptor that maps
// domain errors to gRPC status codes. This is necessary because Kratos stream
// middleware doesn't intercept handler errors the same way as unary middleware.
func ErrorMappingStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		if err != nil {
			err = MapError(err)
		}
		return err
	}
}

// MapError maps domain and application errors to gRPC status codes.
// Errors that already have a gRPC status code (other than Unknown) pass through unchanged.
func MapError(err error) error {
	if err == nil {
		return nil
	}
	// Pass through errors that already have a gRPC status code
	if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
		return err
	}

	log.Errorf("request failed with application error. mapping to status code. error: %v", err)

	// Typed errors (matched by type via errors.As)
	var repReqErr *resources.RepresentationRequiredError
	if errors.As(err, &repReqErr) {
		return status.Errorf(codes.InvalidArgument, "invalid %s representation: representation required", repReqErr.Kind)
	}

	switch {
	// Application-layer errors (from usecase)
	case errors.Is(err, resources.ErrMetaAuthzContextMissing):
		return status.Error(codes.Unauthenticated, "authz context missing")
	case errors.Is(err, resources.ErrSelfSubjectMissing):
		return status.Error(codes.Unauthenticated, "self subject missing")
	case errors.Is(err, resources.ErrMetaAuthorizerUnavailable):
		return status.Error(codes.Internal, "meta authorizer unavailable")
	case errors.Is(err, resources.ErrMetaAuthorizationDenied):
		return status.Error(codes.PermissionDenied, "meta authorization denied")
	// Domain errors (from model)
	case errors.Is(err, model.ErrResourceNotFound):
		return status.Error(codes.NotFound, "resource not found")
	case errors.Is(err, model.ErrResourceAlreadyExists):
		return status.Error(codes.AlreadyExists, "resource already exists")
	case errors.Is(err, model.ErrInventoryIdMismatch):
		return status.Error(codes.FailedPrecondition, "resource inventory id mismatch")
	case errors.Is(err, model.ErrDatabaseError):
		return status.Error(codes.Internal, "internal error")
	// Validation errors (input validation failures)
	case errors.Is(err, model.ErrEmpty):
		return status.Error(codes.InvalidArgument, "required field is empty")
	case errors.Is(err, model.ErrTooLong):
		return status.Error(codes.InvalidArgument, "field exceeds maximum length")
	case errors.Is(err, model.ErrTooSmall):
		return status.Error(codes.InvalidArgument, "field below minimum value")
	case errors.Is(err, model.ErrInvalidURL):
		return status.Error(codes.InvalidArgument, "invalid URL")
	case errors.Is(err, model.ErrInvalidUUID):
		return status.Error(codes.InvalidArgument, "invalid UUID")
	case errors.Is(err, model.ErrInvalidData):
		return status.Error(codes.InvalidArgument, "invalid data structure")
	// Context errors
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return err
	}
}
