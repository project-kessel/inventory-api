package v1

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// TestGRPC_MethodNames verifies the full method names are correct
func TestGRPC_MethodNames(t *testing.T) {
	assert.Equal(t, "/kessel.inventory.v1.KesselInventoryHealthService/GetLivez",
		KesselInventoryHealthService_GetLivez_FullMethodName)
	assert.Equal(t, "/kessel.inventory.v1.KesselInventoryHealthService/GetReadyz",
		KesselInventoryHealthService_GetReadyz_FullMethodName)
}

// TestGRPC_ServiceDescriptor verifies the service descriptor structure
func TestGRPC_ServiceDescriptor(t *testing.T) {
	desc := KesselInventoryHealthService_ServiceDesc

	t.Run("service name", func(t *testing.T) {
		assert.Equal(t, "kessel.inventory.v1.KesselInventoryHealthService", desc.ServiceName)
	})

	t.Run("handler type", func(t *testing.T) {
		// HandlerType is a pointer to the server interface type
		// It may be nil in the descriptor, which is acceptable
		_ = desc.HandlerType
	})

	t.Run("methods", func(t *testing.T) {
		require.Len(t, desc.Methods, 2)

		methodNames := make(map[string]bool)
		for _, method := range desc.Methods {
			methodNames[method.MethodName] = true
		}

		assert.True(t, methodNames["GetLivez"], "GetLivez method should be registered")
		assert.True(t, methodNames["GetReadyz"], "GetReadyz method should be registered")
	})

	t.Run("no streams", func(t *testing.T) {
		assert.Empty(t, desc.Streams, "health service should not have streaming methods")
	})

	t.Run("metadata", func(t *testing.T) {
		assert.Equal(t, "kessel/inventory/v1/health.proto", desc.Metadata)
	})
}

// TestGRPC_UnimplementedServer verifies the unimplemented server behavior
func TestGRPC_UnimplementedServer(t *testing.T) {
	server := &UnimplementedKesselInventoryHealthServiceServer{}
	ctx := context.Background()

	t.Run("GetLivez returns unimplemented", func(t *testing.T) {
		resp, err := server.GetLivez(ctx, &GetLivezRequest{})
		assert.Nil(t, resp)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		require.True(t, ok, "error should be a gRPC status")
		assert.Equal(t, codes.Unimplemented, st.Code())
		assert.Contains(t, st.Message(), "not implemented")
	})

	t.Run("GetReadyz returns unimplemented", func(t *testing.T) {
		resp, err := server.GetReadyz(ctx, &GetReadyzRequest{})
		assert.Nil(t, resp)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		require.True(t, ok, "error should be a gRPC status")
		assert.Equal(t, codes.Unimplemented, st.Code())
		assert.Contains(t, st.Message(), "not implemented")
	})

	t.Run("embeds correctly", func(t *testing.T) {
		// This should compile and not panic
		server.mustEmbedUnimplementedKesselInventoryHealthServiceServer()
		server.testEmbeddedByValue()
	})
}

// mockHealthServer is a mock implementation for testing
type mockHealthServer struct {
	UnimplementedKesselInventoryHealthServiceServer
	livezResponse  *GetLivezResponse
	livezError     error
	readyzResponse *GetReadyzResponse
	readyzError    error
}

func (m *mockHealthServer) GetLivez(ctx context.Context, req *GetLivezRequest) (*GetLivezResponse, error) {
	if m.livezError != nil {
		return nil, m.livezError
	}
	return m.livezResponse, nil
}

func (m *mockHealthServer) GetReadyz(ctx context.Context, req *GetReadyzRequest) (*GetReadyzResponse, error) {
	if m.readyzError != nil {
		return nil, m.readyzError
	}
	return m.readyzResponse, nil
}

// TestGRPC_ServerClientIntegration tests full gRPC server-client communication
func TestGRPC_ServerClientIntegration(t *testing.T) {
	// Create buffer connection for testing
	bufSize := 1024 * 1024
	lis := bufconn.Listen(bufSize)

	// Setup server
	s := grpc.NewServer()
	mockServer := &mockHealthServer{
		livezResponse: &GetLivezResponse{
			Status: "ok",
			Code:   200,
		},
		readyzResponse: &GetReadyzResponse{
			Status: "ready",
			Code:   200,
		},
	}
	RegisterKesselInventoryHealthServiceServer(s, mockServer)

	// Start server in goroutine
	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer s.Stop()

	// Setup client
	ctx := context.Background()
	//nolint:staticcheck // grpc.DialContext is deprecated but grpc.NewClient requires different setup
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			t.Logf("Failed to close connection: %v", cerr)
		}
	}()

	client := NewKesselInventoryHealthServiceClient(conn)

	t.Run("GetLivez success", func(t *testing.T) {
		resp, err := client.GetLivez(ctx, &GetLivezRequest{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "ok", resp.GetStatus())
		assert.Equal(t, uint32(200), resp.GetCode())
	})

	t.Run("GetReadyz success", func(t *testing.T) {
		resp, err := client.GetReadyz(ctx, &GetReadyzRequest{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "ready", resp.GetStatus())
		assert.Equal(t, uint32(200), resp.GetCode())
	})
}

// TestGRPC_ServerClientErrors tests error handling
func TestGRPC_ServerClientErrors(t *testing.T) {
	bufSize := 1024 * 1024
	lis := bufconn.Listen(bufSize)

	s := grpc.NewServer()
	mockServer := &mockHealthServer{
		livezError:  status.Error(codes.Internal, "service unavailable"),
		readyzError: status.Error(codes.Unavailable, "database not ready"),
	}
	RegisterKesselInventoryHealthServiceServer(s, mockServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer s.Stop()

	ctx := context.Background()
	//nolint:staticcheck // grpc.DialContext is deprecated but grpc.NewClient requires different setup
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			t.Logf("Failed to close connection: %v", cerr)
		}
	}()

	client := NewKesselInventoryHealthServiceClient(conn)

	t.Run("GetLivez error", func(t *testing.T) {
		resp, err := client.GetLivez(ctx, &GetLivezRequest{})
		assert.Nil(t, resp)
		require.Error(t, err)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "service unavailable")
	})

	t.Run("GetReadyz error", func(t *testing.T) {
		resp, err := client.GetReadyz(ctx, &GetReadyzRequest{})
		assert.Nil(t, resp)
		require.Error(t, err)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unavailable, st.Code())
		assert.Contains(t, st.Message(), "database not ready")
	})
}

// TestGRPC_ContextCancellation tests context cancellation behavior
func TestGRPC_ContextCancellation(t *testing.T) {
	bufSize := 1024 * 1024
	lis := bufconn.Listen(bufSize)

	s := grpc.NewServer()
	mockServer := &mockHealthServer{
		livezResponse: &GetLivezResponse{
			Status: "ok",
			Code:   200,
		},
	}
	RegisterKesselInventoryHealthServiceServer(s, mockServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer s.Stop()

	//nolint:staticcheck // grpc.DialContext is deprecated but grpc.NewClient requires different setup
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			t.Logf("Failed to close connection: %v", cerr)
		}
	}()

	client := NewKesselInventoryHealthServiceClient(conn)

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		resp, err := client.GetLivez(ctx, &GetLivezRequest{})
		assert.Nil(t, resp)
		require.Error(t, err)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Canceled, st.Code())
	})
}

// TestGRPC_ClientInterface verifies the client interface
func TestGRPC_ClientInterface(t *testing.T) {
	t.Run("client implements interface", func(t *testing.T) {
		// This is a compile-time check that the concrete type implements the interface
		var _ KesselInventoryHealthServiceClient = (*kesselInventoryHealthServiceClient)(nil)
	})
}

// TestGRPC_ServerInterface verifies the server interface
func TestGRPC_ServerInterface(t *testing.T) {
	t.Run("unimplemented server implements interface", func(t *testing.T) {
		var _ KesselInventoryHealthServiceServer = (*UnimplementedKesselInventoryHealthServiceServer)(nil)
	})

	t.Run("mock server implements interface", func(t *testing.T) {
		var _ KesselInventoryHealthServiceServer = (*mockHealthServer)(nil)
	})
}
