// Dual-protocol test framework for KesselInventoryService.
//
// Every test runs against both gRPC (bufconn) and HTTP (httptest) to ensure
// equivalent behavior. The entry point is [runServerTest]:
//
//	runServerTest(t, func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)) {
//	    // setup — create mocks, repos, usecase
//	    return TestServerConfig{...}, func(t *testing.T, tr *Transport) {
//	        // test — drive requests and assert responses
//	        res := tr.Invoke(ctx, withBody(req, Check, httpEndpoint("POST /...")))
//	        Assert(t, res, requireError(codes.Unauthenticated))
//	    }
//	})
//
// The factory is called once per protocol, producing isolated state for each
// run. The test callback captures setup-scoped objects (repos, mocks) via
// closure. This keeps protocol dispatch invisible to the test logic.
//
// # Request construction
//
// [withBody] handles the common case: marshal a proto message for both
// protocols. For low-level control (custom headers, malformed JSON), construct
// a [Request] directly.
//
// # Response verification
//
// Two styles, depending on whether you need the typed response:
//
//   - Void:  Assert(t, res, requireError(codes.InvalidArgument))
//   - Typed: resp := Extract(t, res, expectSuccess(func() *pb.CheckResponse { return &pb.CheckResponse{} }))
//
// Composable: requireError(code).And(func(t *testing.T) { mock.AssertExpectations(t) })
//
// # Polymorphism
//
// [Response] is an interface with unexported apply method. [grpcResponse] and
// [httpResponse] implement it so [Assert] and [Extract] dispatch without type
// switches. Test code never sees the concrete type.
package resources_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/url"
	"strings"
	"testing"

	nethttp "net/http"
	"net/http/httptest"

	"buf.build/go/protovalidate"
	krlog "github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	httpstatus "github.com/go-kratos/kratos/v2/transport/http/status"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
	servergrpc "github.com/project-kessel/inventory-api/internal/server/grpc"
	serverhttp "github.com/project-kessel/inventory-api/internal/server/http"
	svc "github.com/project-kessel/inventory-api/internal/service/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	usecase "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
)

// ---------------------------------------------------------------------------
// TestServerConfig
// ---------------------------------------------------------------------------

// TestServerConfig holds configuration for creating isolated test servers.
type TestServerConfig struct {
	Usecase       *usecase.Usecase
	Authenticator authnapi.Authenticator
}

// ---------------------------------------------------------------------------
// Request / Response / Transport — the core invocation chain
// ---------------------------------------------------------------------------

// Request describes how to invoke an RPC via both gRPC and HTTP.
// Use [withBody] for the common case. Construct directly for low-level
// control (e.g. custom headers, malformed JSON).
type Request struct {
	GRPC func(ctx context.Context, client pb.KesselInventoryServiceClient) (proto.Message, error)
	HTTP func(ctx context.Context, baseURL string) (statusCode int, body []byte, err error)
}

// Response is the polymorphic result of invoking a [Request] through a
// [Transport]. Concrete types ([grpcResponse], [httpResponse]) are unexported;
// use [Assert] or [Extract] to verify.
type Response interface {
	apply(grpcFn func(proto.Message, error), httpFn func(int, []byte))
}

type grpcResponse struct {
	resp proto.Message
	err  error
}

func (r *grpcResponse) apply(grpcFn func(proto.Message, error), _ func(int, []byte)) {
	grpcFn(r.resp, r.err)
}

type httpResponse struct {
	statusCode int
	body       []byte
}

func (r *httpResponse) apply(_ func(proto.Message, error), httpFn func(int, []byte)) {
	httpFn(r.statusCode, r.body)
}

// Transport is the protocol-agnostic client injected into every test callback.
type Transport struct {
	invoke func(ctx context.Context, req Request) Response
}

// Invoke sends req through this transport and returns a polymorphic [Response].
func (tr *Transport) Invoke(ctx context.Context, req Request) Response {
	return tr.invoke(ctx, req)
}

// ---------------------------------------------------------------------------
// Expectation / Extraction — verification of a Response
// ---------------------------------------------------------------------------

// Expectation verifies a [Response] without returning a value.
// Built-in helpers: [requireError], [requireErrorContaining], [requireSuccess].
type Expectation struct {
	GRPC func(t *testing.T, resp proto.Message, err error)
	HTTP func(t *testing.T, statusCode int, body []byte)
}

// And composes an additional check after the original verifiers run.
//
//	Assert(t, res, requireError(codes.Internal).And(func(t *testing.T) {
//	    mock.AssertExpectations(t)
//	}))
func (e Expectation) And(fn func(t *testing.T)) Expectation {
	return Expectation{
		GRPC: func(t *testing.T, resp proto.Message, err error) {
			t.Helper()
			e.GRPC(t, resp, err)
			fn(t)
		},
		HTTP: func(t *testing.T, statusCode int, body []byte) {
			t.Helper()
			e.HTTP(t, statusCode, body)
			fn(t)
		},
	}
}

// Extraction verifies a [Response] and returns a typed value.
// Built-in helper: [expectSuccess].
type Extraction[T any] struct {
	GRPC func(t *testing.T, resp proto.Message, err error) T
	HTTP func(t *testing.T, statusCode int, body []byte) T
}

// ---------------------------------------------------------------------------
// Assert / Extract — polymorphic dispatch
// ---------------------------------------------------------------------------

// Assert applies an [Expectation] to a [Response]. Protocol dispatch is
// handled internally; callers never need a type switch.
func Assert(t *testing.T, res Response, e Expectation) {
	t.Helper()
	res.apply(
		func(resp proto.Message, err error) { e.GRPC(t, resp, err) },
		func(statusCode int, body []byte) { e.HTTP(t, statusCode, body) },
	)
}

// Extract applies an [Extraction] to a [Response] and returns the result.
func Extract[T any](t *testing.T, res Response, e Extraction[T]) T {
	t.Helper()
	var result T
	res.apply(
		func(resp proto.Message, err error) { result = e.GRPC(t, resp, err) },
		func(statusCode int, body []byte) { result = e.HTTP(t, statusCode, body) },
	)
	return result
}

// ---------------------------------------------------------------------------
// RPC shorthands and request building
// ---------------------------------------------------------------------------

// GRPCCall is a type-erased RPC invoker passed to [withBody].
type GRPCCall func(ctx context.Context, client pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error)

//nolint:revive // Intentionally terse — used dozens of times as withBody(req, Check, ...).
var (
	Check GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.Check(ctx, req.(*pb.CheckRequest))
	}
	CheckSelf GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.CheckSelf(ctx, req.(*pb.CheckSelfRequest))
	}
	CheckForUpdate GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.CheckForUpdate(ctx, req.(*pb.CheckForUpdateRequest))
	}
	CheckBulk GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.CheckBulk(ctx, req.(*pb.CheckBulkRequest))
	}
	CheckSelfBulk GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.CheckSelfBulk(ctx, req.(*pb.CheckSelfBulkRequest))
	}
	ReportResource GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.ReportResource(ctx, req.(*pb.ReportResourceRequest))
	}
	DeleteResource GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.DeleteResource(ctx, req.(*pb.DeleteResourceRequest))
	}
)

// HTTPEndpoint is a parsed "METHOD /path" pair for the HTTP side of a [Request].
type HTTPEndpoint struct {
	Method string
	Path   string
}

// httpEndpoint parses "POST /api/kessel/v1beta2/check" into an [HTTPEndpoint].
func httpEndpoint(spec string) HTTPEndpoint {
	parts := strings.SplitN(spec, " ", 2)
	if len(parts) != 2 {
		panic("httpEndpoint: spec must be \"METHOD /path\", got: " + spec)
	}
	return HTTPEndpoint{Method: parts[0], Path: parts[1]}
}

// withBody builds a [Request] that sends req via both gRPC (using g) and HTTP
// (proto-JSON POST/DELETE/etc. to baseURL+h.Path).
func withBody(req proto.Message, g GRPCCall, h HTTPEndpoint) Request {
	return Request{
		GRPC: func(ctx context.Context, client pb.KesselInventoryServiceClient) (proto.Message, error) {
			return g(ctx, client, req)
		},
		HTTP: func(ctx context.Context, baseURL string) (int, []byte, error) {
			jsonBody, err := protojson.Marshal(req)
			if err != nil {
				return 0, nil, err
			}

			httpReq, err := nethttp.NewRequestWithContext(ctx, h.Method, baseURL+h.Path, bytes.NewReader(jsonBody))
			if err != nil {
				return 0, nil, err
			}
			httpReq.Header.Set("Content-Type", "application/json")

			resp, err := nethttp.DefaultClient.Do(httpReq)
			if err != nil {
				return 0, nil, err
			}
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode, body, nil
		},
	}
}

// ---------------------------------------------------------------------------
// Built-in Expectation / Extraction helpers
// ---------------------------------------------------------------------------

// requireError checks that the response is an error with the given gRPC code.
// The HTTP status is derived automatically via Kratos status mapping.
func requireError(code codes.Code) Expectation {
	expectedHTTP := httpstatus.FromGRPCCode(code)
	return Expectation{
		GRPC: func(t *testing.T, resp proto.Message, err error) {
			t.Helper()
			require.Error(t, err)
			assert.Nil(t, resp)
			s, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, code, s.Code())
		},
		HTTP: func(t *testing.T, statusCode int, _ []byte) {
			t.Helper()
			assert.Equal(t, expectedHTTP, statusCode)
		},
	}
}

// requireErrorContaining is like [requireError] but also asserts that the
// error message (gRPC) or response body (HTTP) contains substr.
func requireErrorContaining(code codes.Code, substr string) Expectation {
	expectedHTTP := httpstatus.FromGRPCCode(code)
	return Expectation{
		GRPC: func(t *testing.T, resp proto.Message, err error) {
			t.Helper()
			require.Error(t, err)
			assert.Nil(t, resp)
			s, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, code, s.Code())
			assert.Contains(t, err.Error(), substr)
		},
		HTTP: func(t *testing.T, statusCode int, body []byte) {
			t.Helper()
			assert.Equal(t, expectedHTTP, statusCode)
			assert.Contains(t, string(body), substr)
		},
	}
}

// requireSuccess checks for 200/no-error without unmarshalling the body.
func requireSuccess() Expectation {
	return Expectation{
		GRPC: func(t *testing.T, resp proto.Message, err error) {
			t.Helper()
			require.NoError(t, err)
			require.NotNil(t, resp)
		},
		HTTP: func(t *testing.T, statusCode int, _ []byte) {
			t.Helper()
			require.Equal(t, nethttp.StatusOK, statusCode)
		},
	}
}

// expectSuccess checks for success and returns the typed proto response.
// HTTP JSON is unmarshalled into a fresh instance from newResp.
func expectSuccess[Resp proto.Message](newResp func() Resp) Extraction[Resp] {
	return Extraction[Resp]{
		GRPC: func(t *testing.T, resp proto.Message, err error) Resp {
			t.Helper()
			require.NoError(t, err)
			require.NotNil(t, resp)
			return resp.(Resp)
		},
		HTTP: func(t *testing.T, statusCode int, body []byte) Resp {
			t.Helper()
			require.Equal(t, nethttp.StatusOK, statusCode)
			r := newResp()
			require.NoError(t, protojson.Unmarshal(body, r))
			return r
		},
	}
}

// ---------------------------------------------------------------------------
// Server construction (internal to this framework)
// ---------------------------------------------------------------------------

const testBufSize = 1024 * 1024

// newTestGRPCServer spins up a gRPC server over bufconn with the full
// middleware chain (authn, validation, metrics) and returns a connected client.
func newTestGRPCServer(t *testing.T, cfg TestServerConfig) pb.KesselInventoryServiceClient {
	t.Helper()

	lis := bufconn.Listen(testBufSize)
	testEndpoint := &url.URL{Scheme: "grpc", Host: "bufconn"}
	validator, err := protovalidate.New()
	require.NoError(t, err)

	deps := servergrpc.ServerConfig{
		Authenticator: cfg.Authenticator,
		Logger:        krlog.NewStdLogger(io.Discard),
		Metrics:       metrics.Server(),
		Validator:     validator,
		ServerOptions: []kgrpc.ServerOption{kgrpc.Endpoint(testEndpoint), kgrpc.Listener(lis)},
	}

	srv, err := servergrpc.NewWithDeps(deps)
	require.NoError(t, err)

	service := svc.NewKesselInventoryServiceV1beta2(cfg.Usecase)
	pb.RegisterKesselInventoryServiceServer(srv, service)

	go func() {
		if startErr := srv.Start(context.Background()); startErr != nil {
			t.Logf("gRPC server exited: %v", startErr)
		}
	}()
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return pb.NewKesselInventoryServiceClient(conn)
}

// newTestHTTPServer spins up an HTTP test server with the full middleware
// chain and returns its base URL.
func newTestHTTPServer(t *testing.T, cfg TestServerConfig) string {
	t.Helper()

	validator, err := protovalidate.New()
	require.NoError(t, err)

	deps := serverhttp.ServerConfig{
		AuthnMiddleware: middleware.Authentication(cfg.Authenticator),
		Logger:          krlog.NewStdLogger(io.Discard),
		Metrics:         metrics.Server(),
		Validator:       validator,
	}

	kratosSrv, err := serverhttp.NewWithDeps(deps)
	require.NoError(t, err)

	service := svc.NewKesselInventoryServiceV1beta2(cfg.Usecase)
	pb.RegisterKesselInventoryServiceHTTPServer(kratosSrv, service)

	ts := httptest.NewServer(kratosSrv)
	t.Cleanup(ts.Close)

	return ts.URL
}

// ---------------------------------------------------------------------------
// runServerTest — entry point
// ---------------------------------------------------------------------------

// runServerTest runs factory once per protocol, producing independent server
// state for gRPC and HTTP. The factory returns both the config and the test
// callback; the callback captures setup-scoped objects via closure.
func runServerTest(
	t *testing.T,
	factory func(t *testing.T) (TestServerConfig, func(t *testing.T, tr *Transport)),
) {
	t.Helper()

	t.Run("grpc", func(t *testing.T) {
		cfg, test := factory(t)
		client := newTestGRPCServer(t, cfg)
		tr := &Transport{invoke: func(ctx context.Context, req Request) Response {
			resp, err := req.GRPC(ctx, client)
			return &grpcResponse{resp: resp, err: err}
		}}
		test(t, tr)
	})

	t.Run("http", func(t *testing.T) {
		cfg, test := factory(t)
		baseURL := newTestHTTPServer(t, cfg)
		tr := &Transport{invoke: func(ctx context.Context, req Request) Response {
			statusCode, body, err := req.HTTP(ctx, baseURL)
			require.NoError(t, err, "HTTP transport error")
			return &httpResponse{statusCode: statusCode, body: body}
		}}
		test(t, tr)
	})
}
