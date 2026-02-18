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
// Request — how to send a request via both protocols
// ---------------------------------------------------------------------------

// Request describes how to make a request via both gRPC and HTTP.
// Use [withBody] for the common case. Construct directly for low-level control.
type Request struct {
	GRPC func(ctx context.Context, client pb.KesselInventoryServiceClient) (proto.Message, error)
	HTTP func(ctx context.Context, baseURL string) (statusCode int, body []byte, err error)
}

// ---------------------------------------------------------------------------
// Response — polymorphic result from a transport invocation
// ---------------------------------------------------------------------------

// Response is the result of invoking a [Request] through a [Transport].
// Use [Assert] or [Extract] to verify the response.
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

// ---------------------------------------------------------------------------
// Transport — protocol-agnostic client
// ---------------------------------------------------------------------------

// Transport abstracts a single protocol for a test. Use [Transport.Invoke] to
// send a [Request] and receive a [Response].
type Transport struct {
	invoke func(ctx context.Context, req Request) Response
}

// Invoke sends a request through the transport and returns a [Response].
func (tr *Transport) Invoke(ctx context.Context, req Request) Response {
	return tr.invoke(ctx, req)
}

// ---------------------------------------------------------------------------
// Expectation — void verification of a Response
// ---------------------------------------------------------------------------

// Expectation holds per-protocol verifiers that do not return a value.
// Use [Assert] to apply an Expectation to a [Response].
type Expectation struct {
	GRPC func(t *testing.T, resp proto.Message, err error)
	HTTP func(t *testing.T, statusCode int, body []byte)
}

// And returns a new [Expectation] that first runs the original verifiers,
// then calls fn. Useful for composing extra verification (e.g. mock assertions).
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

// ---------------------------------------------------------------------------
// Extraction — value-returning verification of a Response
// ---------------------------------------------------------------------------

// Extraction holds per-protocol verifiers that return a typed value.
// Use [Extract] to apply an Extraction to a [Response].
type Extraction[T any] struct {
	GRPC func(t *testing.T, resp proto.Message, err error) T
	HTTP func(t *testing.T, statusCode int, body []byte) T
}

// ---------------------------------------------------------------------------
// Assert / Extract — free functions for polymorphic dispatch
// ---------------------------------------------------------------------------

// Assert applies an [Expectation] to a [Response], dispatching to the
// correct protocol arm via the Response's internal apply method.
func Assert(t *testing.T, res Response, e Expectation) {
	t.Helper()
	res.apply(
		func(resp proto.Message, err error) { e.GRPC(t, resp, err) },
		func(statusCode int, body []byte) { e.HTTP(t, statusCode, body) },
	)
}

// Extract applies an [Extraction] to a [Response], dispatching to the
// correct protocol arm and returning the typed result.
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
// GRPCCall — typed shorthand for invoking an RPC via the gRPC client
// ---------------------------------------------------------------------------

// GRPCCall is a function that calls a specific RPC method on a gRPC client.
type GRPCCall func(ctx context.Context, client pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error)

//nolint:revive // These are intentionally terse, readable RPC shorthand names.
var (
	// Check invokes the Check RPC.
	Check GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.Check(ctx, req.(*pb.CheckRequest))
	}
	// CheckSelf invokes the CheckSelf RPC.
	CheckSelf GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.CheckSelf(ctx, req.(*pb.CheckSelfRequest))
	}
	// CheckForUpdate invokes the CheckForUpdate RPC.
	CheckForUpdate GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.CheckForUpdate(ctx, req.(*pb.CheckForUpdateRequest))
	}
	// CheckBulk invokes the CheckBulk RPC.
	CheckBulk GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.CheckBulk(ctx, req.(*pb.CheckBulkRequest))
	}
	// CheckSelfBulk invokes the CheckSelfBulk RPC.
	CheckSelfBulk GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.CheckSelfBulk(ctx, req.(*pb.CheckSelfBulkRequest))
	}
	// ReportResource invokes the ReportResource RPC.
	ReportResource GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.ReportResource(ctx, req.(*pb.ReportResourceRequest))
	}
	// DeleteResource invokes the DeleteResource RPC.
	DeleteResource GRPCCall = func(ctx context.Context, c pb.KesselInventoryServiceClient, req proto.Message) (proto.Message, error) {
		return c.DeleteResource(ctx, req.(*pb.DeleteResourceRequest))
	}
)

// ---------------------------------------------------------------------------
// HTTPEndpoint — parsed HTTP method + path
// ---------------------------------------------------------------------------

// HTTPEndpoint describes an HTTP method and path for a request.
type HTTPEndpoint struct {
	Method string
	Path   string
}

// httpEndpoint parses a spec like "POST /api/kessel/v1beta2/check" into an [HTTPEndpoint].
func httpEndpoint(spec string) HTTPEndpoint {
	parts := strings.SplitN(spec, " ", 2)
	if len(parts) != 2 {
		panic("httpEndpoint: spec must be \"METHOD /path\", got: " + spec)
	}
	return HTTPEndpoint{Method: parts[0], Path: parts[1]}
}

// ---------------------------------------------------------------------------
// Request builder: withBody
// ---------------------------------------------------------------------------

// withBody creates a [Request] that sends the given proto message to both protocols.
// The gRPC side uses the provided [GRPCCall]. The HTTP side marshals req to proto-JSON
// and makes a raw HTTP request to baseURL+path.
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
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode, body, nil
		},
	}
}

// ---------------------------------------------------------------------------
// Expectation helpers (void)
// ---------------------------------------------------------------------------

// requireError verifies that both gRPC and HTTP return an equivalent error for code.
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

// requireErrorContaining is like [requireError] but also checks that the error
// message (gRPC) or response body (HTTP) contains substr.
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

// requireSuccess verifies a successful response without unmarshalling it.
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

// ---------------------------------------------------------------------------
// Extraction helper (value-returning)
// ---------------------------------------------------------------------------

// expectSuccess verifies a successful response on both protocols and returns
// the typed response. The HTTP JSON body is unmarshalled into a fresh Resp
// created by newResp.
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
// Server construction helpers
// ---------------------------------------------------------------------------

const testBufSize = 1024 * 1024

// newTestGRPCServer creates a gRPC server over bufconn and returns a client.
// Cleanup is registered via t.Cleanup.
func newTestGRPCServer(t *testing.T, cfg TestServerConfig) pb.KesselInventoryServiceClient {
	t.Helper()

	lis := bufconn.Listen(testBufSize)
	testEndpoint := &url.URL{Scheme: "grpc", Host: "bufconn"}
	validator, err := protovalidate.New()
	require.NoError(t, err)

	deps := servergrpc.ServerDeps{
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

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = conn.Close()
		_ = srv.Stop(context.Background())
	})

	return pb.NewKesselInventoryServiceClient(conn)
}

// newTestHTTPServer creates an HTTP test server with the same middleware chain
// as production and returns its base URL. Cleanup is registered via t.Cleanup.
func newTestHTTPServer(t *testing.T, cfg TestServerConfig) string {
	t.Helper()

	validator, err := protovalidate.New()
	require.NoError(t, err)

	deps := serverhttp.ServerDeps{
		Authenticator: cfg.Authenticator,
		Logger:        krlog.NewStdLogger(io.Discard),
		Metrics:       metrics.Server(),
		Validator:     validator,
	}

	kratosSrv, err := serverhttp.NewWithDeps(deps, nil, false)
	require.NoError(t, err)

	service := svc.NewKesselInventoryServiceV1beta2(cfg.Usecase)
	pb.RegisterKesselInventoryServiceHTTPServer(kratosSrv, service)

	ts := httptest.NewServer(kratosSrv)
	t.Cleanup(ts.Close)

	return ts.URL
}

// ---------------------------------------------------------------------------
// runServerTest — the main entry point
// ---------------------------------------------------------------------------

// runServerTest creates isolated gRPC and HTTP servers and runs the test
// function against both protocols. The factory returns both the server
// config and the test callback so that the test closure can capture
// setup-scoped objects (repos, mocks, etc.) via closure.
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
