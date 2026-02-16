package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// grpcWriteEndpoints defines all grpc endpoints for write operations
	// a map is used to improve lookup speed when checking the current method in interceptor
	// by avoiding loops or uses slices pkg
	grpcWriteEndpoints = map[string]bool{
		"/kessel.inventory.v1beta2.KesselInventoryService/ReportResource": true,
		"/kessel.inventory.v1beta2.KesselInventoryService/DeleteResource": true,
		"/api/kessel/v1beta2/resources":                                   true,
	}
)

func IsWriteEndpoint(method string) bool {
	return grpcWriteEndpoints[method]
}

// UnaryReadOnlyInterceptor blocks write endpoints when the server is in read-only mode.
// This interceptor should only be added to the interceptor chain when read-only mode is enabled.
func UnaryReadOnlyInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if IsWriteEndpoint(info.FullMethod) {
			return nil, status.Errorf(codes.Unavailable, "endpoint %s is a write endpoint and server is in read-only mode", info.FullMethod)
		}
		return handler(ctx, req)
	}
}

// HTTPReadOnlyMiddleware blocks write endpoints when the server is in read-only mode.
// This middleware should only be added to the middleware chain when read-only mode is enabled.
func HTTPReadOnlyMiddleware(handler middleware.Handler) middleware.Handler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		t, ok := transport.FromServerContext(ctx)
		if !ok {
			return handler(ctx, req)
		}

		endpoint := t.Operation()
		if IsWriteEndpoint(endpoint) {
			return nil, status.Errorf(http.StatusForbidden, fmt.Sprintf("endpoint %s is a write endpoint and server is in read-only mode", endpoint))
		}
		return handler(ctx, req)
	}
}
