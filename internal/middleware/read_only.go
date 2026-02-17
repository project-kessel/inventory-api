package middleware

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// writeEndpoints defines all grpc/http endpoints for write operations
	// Since almost all Kessel endpoints require data (POST/DELETE), we cannot block based
	// on method, so the specific endpoints to deny must be defined.
	//
	// a map is used to improve lookup speed by avoiding loops or uses slices pkg
	writeEndpoints = map[string]bool{
		"/kessel.inventory.v1beta2.KesselInventoryService/ReportResource": true,
		"/kessel.inventory.v1beta2.KesselInventoryService/DeleteResource": true,
		"/api/kessel/v1beta2/resources":                                   true,
	}
)

// IsWriteEndpoint checks the provided endpoint against defined write endpoints
func IsWriteEndpoint(endpoint string) bool {
	return writeEndpoints[endpoint]
}

// UnaryReadOnlyInterceptor blocks grpc write endpoints when the server is in read-only mode.
// This interceptor should only be added to the interceptor chain when read-only mode is enabled.
func UnaryReadOnlyInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if IsWriteEndpoint(info.FullMethod) {
			return nil, status.Errorf(codes.Unavailable, "endpoint %s is a write endpoint and server is in read-only mode", info.FullMethod)
		}
		return handler(ctx, req)
	}
}

// HTTPReadOnlyMiddleware blocks http write endpoints when the server is in read-only mode.
// This middleware should only be added to the middleware chain when read-only mode is enabled.
func HTTPReadOnlyMiddleware(handler middleware.Handler) middleware.Handler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		t, ok := transport.FromServerContext(ctx)
		if !ok {
			return handler(ctx, req)
		}

		endpoint := t.Operation()
		if IsWriteEndpoint(endpoint) {
			return nil, errors.ServiceUnavailable(
				"READ_ONLY_MODE",
				fmt.Sprintf("endpoint %s is a write endpoint and server is in read-only mode", endpoint))
		}
		return handler(ctx, req)
	}
}
