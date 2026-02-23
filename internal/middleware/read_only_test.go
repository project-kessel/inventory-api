package middleware

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/project-kessel/inventory-api/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsWriteEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		expected bool
	}{
		{
			name:     "ReportResource is a write endpoint",
			method:   "/kessel.inventory.v1beta2.KesselInventoryService/ReportResource",
			expected: true,
		},
		{
			name:     "DeleteResource is a write endpoint",
			method:   "/kessel.inventory.v1beta2.KesselInventoryService/DeleteResource",
			expected: true,
		},
		{
			name:     "HTTP resources endpoint is a write endpoint",
			method:   "/api/kessel/v1beta2/resources",
			expected: true,
		},
		{
			name:     "non-write endpoint returns false",
			method:   "/kessel.inventory.v1beta2.KesselInventoryService/Check",
			expected: false,
		},
		{
			name:     "empty string returns false",
			method:   "",
			expected: false,
		},
		{
			name:     "unknown endpoint returns false",
			method:   "/some/unknown/endpoint",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsWriteEndpoint(tt.method)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnaryReadOnlyInterceptor_BlocksWriteEndpoints(t *testing.T) {
	interceptor := UnaryReadOnlyInterceptor()

	tests := []struct {
		name         string
		fullMethod   string
		expectErr    bool
		expectedCode codes.Code
	}{
		{
			name:         "blocks ReportResource",
			fullMethod:   "/kessel.inventory.v1beta2.KesselInventoryService/ReportResource",
			expectErr:    true,
			expectedCode: codes.Unavailable,
		},
		{
			name:         "blocks DeleteResource",
			fullMethod:   "/kessel.inventory.v1beta2.KesselInventoryService/DeleteResource",
			expectErr:    true,
			expectedCode: codes.Unavailable,
		},
		{
			name:         "allows non-write endpoint",
			fullMethod:   "/kessel.inventory.v1beta2.KesselInventoryService/Check",
			expectErr:    false,
			expectedCode: codes.OK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			info := &grpc.UnaryServerInfo{
				FullMethod: tt.fullMethod,
			}

			handlerCalled := false
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				handlerCalled = true
				return "success", nil
			}

			result, err := interceptor(ctx, "test-request", info, handler)

			if tt.expectErr {
				assert.Error(t, err)
				assert.False(t, handlerCalled, "handler should not be called for write endpoints")
				assert.Nil(t, result)

				st, ok := status.FromError(err)
				require.True(t, ok, "error should be a gRPC status error")
				assert.Equal(t, tt.expectedCode, st.Code())
				assert.Contains(t, st.Message(), tt.fullMethod)
				assert.Contains(t, st.Message(), "read-only mode")
			} else {
				assert.NoError(t, err)
				assert.True(t, handlerCalled, "handler should be called for non-write endpoints")
				assert.Equal(t, "success", result)
			}
		})
	}
}

func TestHTTPReadOnlyMiddleware_BlocksWriteEndpoints(t *testing.T) {
	tests := []struct {
		name         string
		operation    string
		hasTransport bool
		expectErr    bool
		expectedCode int
	}{
		{
			name:         "blocks HTTP resources endpoint",
			operation:    "/api/kessel/v1beta2/resources",
			hasTransport: true,
			expectErr:    true,
			expectedCode: http.StatusServiceUnavailable,
		},
		{
			name:         "allows non-write endpoint",
			operation:    "/api/kessel/v1beta2/check",
			hasTransport: true,
			expectErr:    false,
			expectedCode: http.StatusOK,
		},
		{
			name:         "allows request when transport is missing",
			operation:    "/api/kessel/v1beta2/check",
			hasTransport: false,
			expectErr:    false,
			expectedCode: http.StatusOK,
		},
		{
			name:         "allows unknown endpoint",
			operation:    "/some/unknown/endpoint",
			hasTransport: true,
			expectErr:    false,
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.hasTransport {
				transporter := &mocks.MockTransporter{OperationValue: tt.operation}
				ctx = transport.NewServerContext(context.Background(), transporter)
			} else {
				ctx = context.Background()
			}

			handlerCalled := false
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				handlerCalled = true
				return "success", nil
			}

			// Create middleware with handler
			middlewareWithHandler := HTTPReadOnlyMiddleware(handler)

			result, err := middlewareWithHandler(ctx, "test-request")

			if tt.expectErr {
				assert.Error(t, err)
				assert.False(t, handlerCalled, "handler should not be called for write endpoints")
				assert.Nil(t, result)

				// Check that it's a kratos error with the correct HTTP status code
				kratosErr := errors.FromError(err)
				require.NotNil(t, kratosErr, "error should be a kratos error")
				assert.Equal(t, int32(tt.expectedCode), kratosErr.GetCode(), "HTTP status code should match")
				assert.Contains(t, kratosErr.GetMessage(), tt.operation)
				assert.Contains(t, kratosErr.GetMessage(), "read-only mode")
				assert.Equal(t, "READ_ONLY_MODE", kratosErr.GetReason())
			} else {
				assert.NoError(t, err)
				assert.True(t, handlerCalled, "handler should be called")
				assert.Equal(t, "success", result)
			}
		})
	}
}

func TestHTTPReadOnlyMiddleware_HandlerError(t *testing.T) {
	transporter := &mocks.MockTransporter{OperationValue: "/kessel.inventory.v1beta2.KesselInventoryService/GetResource"}
	ctx := transport.NewServerContext(context.Background(), transporter)

	expectedErr := status.Error(codes.Internal, "handler error")
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, expectedErr
	}

	middlewareWithHandler := HTTPReadOnlyMiddleware(handler)

	result, err := middlewareWithHandler(ctx, "test-request")

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)
}
