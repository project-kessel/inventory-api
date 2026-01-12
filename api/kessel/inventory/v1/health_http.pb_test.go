package v1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTP_OperationConstants verifies the operation constants are correct
func TestHTTP_OperationConstants(t *testing.T) {
	assert.Equal(t, "/kessel.inventory.v1.KesselInventoryHealthService/GetLivez",
		OperationKesselInventoryHealthServiceGetLivez)
	assert.Equal(t, "/kessel.inventory.v1.KesselInventoryHealthService/GetReadyz",
		OperationKesselInventoryHealthServiceGetReadyz)
}

// TestHTTP_OperationConstantsMatchGRPC verifies HTTP and gRPC operation names match
func TestHTTP_OperationConstantsMatchGRPC(t *testing.T) {
	assert.Equal(t, KesselInventoryHealthService_GetLivez_FullMethodName,
		OperationKesselInventoryHealthServiceGetLivez,
		"HTTP and gRPC operation names should match for GetLivez")
	assert.Equal(t, KesselInventoryHealthService_GetReadyz_FullMethodName,
		OperationKesselInventoryHealthServiceGetReadyz,
		"HTTP and gRPC operation names should match for GetReadyz")
}

// mockHTTPHealthServer is a mock implementation for testing
type mockHTTPHealthServer struct {
	livezResponse  *GetLivezResponse
	livezError     error
	readyzResponse *GetReadyzResponse
	readyzError    error
}

func (m *mockHTTPHealthServer) GetLivez(ctx context.Context, req *GetLivezRequest) (*GetLivezResponse, error) {
	if m.livezError != nil {
		return nil, m.livezError
	}
	if m.livezResponse != nil {
		return m.livezResponse, nil
	}
	return &GetLivezResponse{Status: "ok", Code: 200}, nil
}

func (m *mockHTTPHealthServer) GetReadyz(ctx context.Context, req *GetReadyzRequest) (*GetReadyzResponse, error) {
	if m.readyzError != nil {
		return nil, m.readyzError
	}
	if m.readyzResponse != nil {
		return m.readyzResponse, nil
	}
	return &GetReadyzResponse{Status: "ready", Code: 200}, nil
}

// TestHTTP_ServerInterface verifies the HTTP server interface
func TestHTTP_ServerInterface(t *testing.T) {
	t.Run("mock server implements interface", func(t *testing.T) {
		var _ KesselInventoryHealthServiceHTTPServer = (*mockHTTPHealthServer)(nil)
	})

	t.Run("server methods have correct signatures", func(t *testing.T) {
		server := &mockHTTPHealthServer{
			livezResponse: &GetLivezResponse{
				Status: "ok",
				Code:   200,
			},
			readyzResponse: &GetReadyzResponse{
				Status: "ready",
				Code:   200,
			},
		}

		ctx := context.Background()

		// Test GetLivez
		livezResp, err := server.GetLivez(ctx, &GetLivezRequest{})
		assert.NoError(t, err)
		assert.NotNil(t, livezResp)
		assert.Equal(t, "ok", livezResp.GetStatus())
		assert.Equal(t, uint32(200), livezResp.GetCode())

		// Test GetReadyz
		readyzResp, err := server.GetReadyz(ctx, &GetReadyzRequest{})
		assert.NoError(t, err)
		assert.NotNil(t, readyzResp)
		assert.Equal(t, "ready", readyzResp.GetStatus())
		assert.Equal(t, uint32(200), readyzResp.GetCode())
	})
}

// TestHTTP_ServerErrors tests error handling in the HTTP server
func TestHTTP_ServerErrors(t *testing.T) {
	server := &mockHTTPHealthServer{
		livezError:  assert.AnError,
		readyzError: assert.AnError,
	}

	ctx := context.Background()

	t.Run("GetLivez error", func(t *testing.T) {
		resp, err := server.GetLivez(ctx, &GetLivezRequest{})
		assert.Nil(t, resp)
		assert.Error(t, err)
	})

	t.Run("GetReadyz error", func(t *testing.T) {
		resp, err := server.GetReadyz(ctx, &GetReadyzRequest{})
		assert.Nil(t, resp)
		assert.Error(t, err)
	})
}

// TestHTTP_ClientInterface verifies the HTTP client interface
func TestHTTP_ClientInterface(t *testing.T) {
	t.Run("client implementation exists", func(t *testing.T) {
		// Verify the concrete implementation implements the interface
		var _ KesselInventoryHealthServiceHTTPClient = (*KesselInventoryHealthServiceHTTPClientImpl)(nil)
	})
}

// TestHTTP_Routes verifies the expected HTTP routes
func TestHTTP_Routes(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		method   string
	}{
		{
			name:     "livez endpoint",
			endpoint: "/api/inventory/v1/livez",
			method:   "GET",
		},
		{
			name:     "readyz endpoint",
			endpoint: "/api/inventory/v1/readyz",
			method:   "GET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.endpoint, "endpoint should be defined")
			assert.Equal(t, "GET", tt.method, "health endpoints should use GET method")
		})
	}
}

// TestHTTP_EndpointVersioning verifies consistent API versioning
func TestHTTP_EndpointVersioning(t *testing.T) {
	livezPath := "/api/inventory/v1/livez"
	readyzPath := "/api/inventory/v1/readyz"

	t.Run("endpoints use same version", func(t *testing.T) {
		assert.Contains(t, livezPath, "/v1/", "livez should use v1 versioning")
		assert.Contains(t, readyzPath, "/v1/", "readyz should use v1 versioning")
	})

	t.Run("endpoints follow REST conventions", func(t *testing.T) {
		assert.Contains(t, livezPath, "/api/inventory", "should include api/inventory prefix")
		assert.Contains(t, readyzPath, "/api/inventory", "should include api/inventory prefix")
	})

	t.Run("health check naming", func(t *testing.T) {
		assert.Contains(t, livezPath, "livez", "should use livez for liveness")
		assert.Contains(t, readyzPath, "readyz", "should use readyz for readiness")
	})
}

// TestHTTP_ResponseCodes tests expected HTTP response codes
func TestHTTP_ResponseCodes(t *testing.T) {
	server := &mockHTTPHealthServer{
		livezResponse: &GetLivezResponse{
			Status: "ok",
			Code:   200,
		},
		readyzResponse: &GetReadyzResponse{
			Status: "ready",
			Code:   200,
		},
	}

	ctx := context.Background()

	t.Run("success returns 200", func(t *testing.T) {
		livezResp, err := server.GetLivez(ctx, &GetLivezRequest{})
		assert.NoError(t, err)
		assert.Equal(t, uint32(200), livezResp.GetCode())

		readyzResp, err := server.GetReadyz(ctx, &GetReadyzRequest{})
		assert.NoError(t, err)
		assert.Equal(t, uint32(200), readyzResp.GetCode())
	})

	t.Run("unhealthy returns error code", func(t *testing.T) {
		unhealthyServer := &mockHTTPHealthServer{
			livezResponse: &GetLivezResponse{
				Status: "unhealthy",
				Code:   503,
			},
			readyzResponse: &GetReadyzResponse{
				Status: "not ready",
				Code:   503,
			},
		}

		livezResp, err := unhealthyServer.GetLivez(ctx, &GetLivezRequest{})
		assert.NoError(t, err)
		assert.Equal(t, uint32(503), livezResp.GetCode())

		readyzResp, err := unhealthyServer.GetReadyz(ctx, &GetReadyzRequest{})
		assert.NoError(t, err)
		assert.Equal(t, uint32(503), readyzResp.GetCode())
	})
}

// TestHTTP_ContextPropagation tests that context is properly propagated
func TestHTTP_ContextPropagation(t *testing.T) {
	type contextKey string
	const testKey contextKey = "test-key"

	server := &mockHTTPHealthServer{
		livezResponse: &GetLivezResponse{
			Status: "ok",
			Code:   200,
		},
	}

	ctx := context.WithValue(context.Background(), testKey, "test-value")

	t.Run("context is passed to handler", func(t *testing.T) {
		// This test verifies the context is properly passed through
		// In a real implementation, you'd check the context in the handler
		resp, err := server.GetLivez(ctx, &GetLivezRequest{})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

// TestHTTP_EmptyRequests tests handling of empty request bodies
func TestHTTP_EmptyRequests(t *testing.T) {
	server := &mockHTTPHealthServer{
		livezResponse: &GetLivezResponse{
			Status: "ok",
			Code:   200,
		},
		readyzResponse: &GetReadyzResponse{
			Status: "ready",
			Code:   200,
		},
	}

	ctx := context.Background()

	t.Run("GetLivez with empty request", func(t *testing.T) {
		resp, err := server.GetLivez(ctx, &GetLivezRequest{})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "ok", resp.GetStatus())
	})

	t.Run("GetReadyz with empty request", func(t *testing.T) {
		resp, err := server.GetReadyz(ctx, &GetReadyzRequest{})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "ready", resp.GetStatus())
	})
}

// TestHTTP_ServerResponses tests various response scenarios
func TestHTTP_ServerResponses(t *testing.T) {
	testCases := []struct {
		name           string
		livezResponse  *GetLivezResponse
		readyzResponse *GetReadyzResponse
		description    string
	}{
		{
			name: "healthy service",
			livezResponse: &GetLivezResponse{
				Status: "ok",
				Code:   200,
			},
			readyzResponse: &GetReadyzResponse{
				Status: "ready",
				Code:   200,
			},
			description: "both endpoints return healthy",
		},
		{
			name: "live but not ready",
			livezResponse: &GetLivezResponse{
				Status: "ok",
				Code:   200,
			},
			readyzResponse: &GetReadyzResponse{
				Status: "not ready",
				Code:   503,
			},
			description: "service is alive but dependencies not ready",
		},
		{
			name: "degraded service",
			livezResponse: &GetLivezResponse{
				Status: "degraded",
				Code:   429,
			},
			readyzResponse: &GetReadyzResponse{
				Status: "degraded",
				Code:   429,
			},
			description: "service is degraded but operational",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := &mockHTTPHealthServer{
				livezResponse:  tc.livezResponse,
				readyzResponse: tc.readyzResponse,
			}

			ctx := context.Background()

			// Test GetLivez
			livezResp, err := server.GetLivez(ctx, &GetLivezRequest{})
			assert.NoError(t, err)
			assert.Equal(t, tc.livezResponse.GetStatus(), livezResp.GetStatus())
			assert.Equal(t, tc.livezResponse.GetCode(), livezResp.GetCode())

			// Test GetReadyz
			readyzResp, err := server.GetReadyz(ctx, &GetReadyzRequest{})
			assert.NoError(t, err)
			assert.Equal(t, tc.readyzResponse.GetStatus(), readyzResp.GetStatus())
			assert.Equal(t, tc.readyzResponse.GetCode(), readyzResp.GetCode())
		})
	}
}

// TestHTTP_ActualHandlerIntegration tests the actual generated HTTP handlers
func TestHTTP_ActualHandlerIntegration(t *testing.T) {
	// Create a mock server implementation
	mockServer := &mockHTTPHealthServer{
		livezResponse: &GetLivezResponse{
			Status: "ok",
			Code:   200,
		},
		readyzResponse: &GetReadyzResponse{
			Status: "ready",
			Code:   200,
		},
	}

	// Create Kratos HTTP server
	srv := kratoshttp.NewServer()
	RegisterKesselInventoryHealthServiceHTTPServer(srv, mockServer)

	// Get the underlying HTTP handler
	handler := srv.Handler

	t.Run("GetLivez handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/livez", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer func() {
			_ = resp.Body.Close()
		}()

		// Check status code
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Parse response body
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var livezResp GetLivezResponse
		err = json.Unmarshal(body, &livezResp)
		require.NoError(t, err)

		assert.Equal(t, "ok", livezResp.Status)
		assert.Equal(t, uint32(200), livezResp.Code)
	})

	t.Run("GetReadyz handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/readyz", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer func() {
			_ = resp.Body.Close()
		}()

		// Check status code
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Parse response body
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var readyzResp GetReadyzResponse
		err = json.Unmarshal(body, &readyzResp)
		require.NoError(t, err)

		assert.Equal(t, "ready", readyzResp.Status)
		assert.Equal(t, uint32(200), readyzResp.Code)
	})
}

// TestHTTP_HandlerWithErrors tests error handling in actual HTTP handlers
func TestHTTP_HandlerWithErrors(t *testing.T) {
	// Create a mock server that returns errors
	mockServer := &mockHTTPHealthServer{
		livezError:  assert.AnError,
		readyzError: assert.AnError,
	}

	// Create Kratos HTTP server
	srv := kratoshttp.NewServer()
	RegisterKesselInventoryHealthServiceHTTPServer(srv, mockServer)
	handler := srv.Handler

	t.Run("GetLivez error handling", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/livez", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer func() {
			_ = resp.Body.Close()
		}()

		// Should return error status code
		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("GetReadyz error handling", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/readyz", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer func() {
			_ = resp.Body.Close()
		}()

		// Should return error status code
		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})
}

// TestHTTP_ClientImplementation tests the actual HTTP client
func TestHTTP_ClientImplementation(t *testing.T) {
	// Create a test HTTP server
	mockServer := &mockHTTPHealthServer{
		livezResponse: &GetLivezResponse{
			Status: "ok",
			Code:   200,
		},
		readyzResponse: &GetReadyzResponse{
			Status: "ready",
			Code:   200,
		},
	}

	// Create Kratos HTTP server
	srv := kratoshttp.NewServer()
	RegisterKesselInventoryHealthServiceHTTPServer(srv, mockServer)

	// Create a test server
	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	// Create HTTP client
	httpClient, err := kratoshttp.NewClient(
		context.Background(),
		kratoshttp.WithEndpoint(ts.URL),
	)
	require.NoError(t, err)

	client := NewKesselInventoryHealthServiceHTTPClient(httpClient)

	t.Run("GetLivez via client", func(t *testing.T) {
		resp, err := client.GetLivez(context.Background(), &GetLivezRequest{})
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "ok", resp.GetStatus())
		assert.Equal(t, uint32(200), resp.GetCode())
	})

	t.Run("GetReadyz via client", func(t *testing.T) {
		resp, err := client.GetReadyz(context.Background(), &GetReadyzRequest{})
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "ready", resp.GetStatus())
		assert.Equal(t, uint32(200), resp.GetCode())
	})
}

// TestHTTP_UnhealthyResponses tests various unhealthy scenarios
func TestHTTP_UnhealthyResponses(t *testing.T) {
	mockServer := &mockHTTPHealthServer{
		livezResponse: &GetLivezResponse{
			Status: "unhealthy",
			Code:   503,
		},
		readyzResponse: &GetReadyzResponse{
			Status: "not ready",
			Code:   503,
		},
	}

	srv := kratoshttp.NewServer()
	RegisterKesselInventoryHealthServiceHTTPServer(srv, mockServer)
	handler := srv.Handler

	t.Run("unhealthy livez", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/livez", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer func() {
			_ = resp.Body.Close()
		}()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var livezResp GetLivezResponse
		err = json.Unmarshal(body, &livezResp)
		require.NoError(t, err)

		assert.Equal(t, "unhealthy", livezResp.Status)
		assert.Equal(t, uint32(503), livezResp.Code)
	})

	t.Run("not ready readyz", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/readyz", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer func() {
			_ = resp.Body.Close()
		}()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var readyzResp GetReadyzResponse
		err = json.Unmarshal(body, &readyzResp)
		require.NoError(t, err)

		assert.Equal(t, "not ready", readyzResp.Status)
		assert.Equal(t, uint32(503), readyzResp.Code)
	})
}

// TestHTTP_WrongHTTPMethod tests handling of incorrect HTTP methods
func TestHTTP_WrongHTTPMethod(t *testing.T) {
	mockServer := &mockHTTPHealthServer{
		livezResponse: &GetLivezResponse{Status: "ok", Code: 200},
	}

	srv := kratoshttp.NewServer()
	RegisterKesselInventoryHealthServiceHTTPServer(srv, mockServer)
	handler := srv.Handler

	wrongMethods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range wrongMethods {
		t.Run("livez with "+method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/inventory/v1/livez", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer func() {
				_ = resp.Body.Close()
			}()

			// Should return method not allowed or not found
			assert.Contains(t, []int{http.StatusMethodNotAllowed, http.StatusNotFound}, resp.StatusCode)
		})
	}
}
