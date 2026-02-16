package middleware

import (
	"context"

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
	}
)

func IsWriteEndpoint(method string) bool {
	return grpcWriteEndpoints[method]
}

func UnaryReadOnlyInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if IsWriteEndpoint(info.FullMethod) {
			return nil, status.Errorf(codes.Unavailable, "endpoint %s is a write endpoint and server is in read-only mode", info.FullMethod)
		}
		return handler(ctx, req)
	}
}
