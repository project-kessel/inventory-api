package middleware

import (
	"context"
	"fmt"
	"strings"

	coreosoidc "github.com/coreos/go-oidc/v3/oidc"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/project-kessel/inventory-api/internal/authn"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/util"
)

const (
	// authorizationKey holds the key used to store the JWT Token in the request tokenHeader.
	authorizationKey string = "authorization"
)

// grpcStreamTransporter adapts gRPC metadata to transport.Transporter interface.
// Keep this minimal: only data that actually varies.
type grpcStreamTransporter struct {
	md metadata.MD
	op string
}

func (t *grpcStreamTransporter) Kind() transport.Kind {
	return transport.KindGRPC
}

func (t *grpcStreamTransporter) Endpoint() string {
	// Not available for streams
	return ""
}

func (t *grpcStreamTransporter) Operation() string {
	return t.op
}

func (t *grpcStreamTransporter) RequestHeader() transport.Header {
	return &grpcMetadataHeader{md: t.md}
}

// Unused today: keep a cheap, empty header for future use
var emptyGRPCHeader = &grpcMetadataHeader{md: metadata.MD{}}

func (t *grpcStreamTransporter) ReplyHeader() transport.Header {
	return emptyGRPCHeader
}

type grpcMetadataHeader struct {
	md metadata.MD
}

func (h *grpcMetadataHeader) Get(key string) string {
	vals := h.md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// These are currently unused by authenticators, but must exist to satisfy transport.Header.
// Keeping them as simple delegations documents that they're not part of current behavior.
func (h *grpcMetadataHeader) Set(key string, value string) {
	h.md.Set(key, value)
}

func (h *grpcMetadataHeader) Add(key string, value string) {
	h.md.Append(key, value)
}

func (h *grpcMetadataHeader) Keys() []string {
	keys := make([]string, 0, len(h.md))
	for k := range h.md {
		keys = append(keys, k)
	}
	return keys
}

func (h *grpcMetadataHeader) Values(key string) []string {
	return h.md.Get(key)
}

// StreamAuthConfig holds configuration for the stream authentication interceptor.
type StreamAuthConfig struct {
	authenticator authnapi.Authenticator
	logger        log.Logger
}

// StreamAuthOption is a functional option for configuring StreamAuthInterceptor.
type StreamAuthOption func(*StreamAuthConfig)

// NewStreamAuthInterceptor creates a stream authentication interceptor using the AggregatingAuthenticator.
// If authenticator is provided, it will be used directly.
// If authenticator is nil, it will be created from the config using authn.New (backwards compatible).
//
// The interceptor uses the aggregating authenticator to authenticate gRPC streams, supporting
// all authenticator types in the chain (OIDC, x-rh-identity, allow-unauthenticated, etc.).
func NewStreamAuthInterceptor(config authn.CompletedConfig, authenticator authnapi.Authenticator, logger log.Logger, opts ...StreamAuthOption) (*StreamAuthInterceptor, error) {

	// If authenticator is not provided, create it from config (backwards compatible)
	if authenticator == nil {
		authnLogger := log.NewHelper(log.With(logger, "subsystem", "authn", "component", "stream-interceptor"))
		var err error
		authenticator, err = authn.New(config, authnLogger)
		if err != nil {
			return nil, err
		}
	}

	return NewStreamAuthInterceptorFromAuthenticator(authenticator, logger, opts...)
}

// NewStreamAuthInterceptorFromAuthenticator creates a stream authentication interceptor
// using a pre-configured authenticator. This is useful for tests where the authenticator
// is already constructed.
func NewStreamAuthInterceptorFromAuthenticator(authenticator authnapi.Authenticator, logger log.Logger, opts ...StreamAuthOption) (*StreamAuthInterceptor, error) {
	if authenticator == nil {
		return nil, fmt.Errorf("authenticator is required")
	}

	cfg := &StreamAuthConfig{
		authenticator: authenticator,
		logger:        logger,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return &StreamAuthInterceptor{cfg: cfg}, nil
}

// StreamAuthInterceptor implements gRPC stream authentication.
type StreamAuthInterceptor struct {
	cfg *StreamAuthConfig
}

// Interceptor returns the gRPC stream server interceptor function.
func (i *StreamAuthInterceptor) Interceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		newCtx := ss.Context()

		md, ok := metadata.FromIncomingContext(newCtx)
		if !ok {
			return kerrors.Unauthorized("UNAUTHORIZED", fmt.Sprintf("Missing metadata for stream method: %s", info.FullMethod))
		}

		// Create transport adapter for gRPC stream
		transporter := &grpcStreamTransporter{
			md: md,
			op: info.FullMethod,
		}

		// Use aggregating authenticator to authenticate the stream
		claims, decision := i.cfg.authenticator.Authenticate(newCtx, transporter)

		// Log decision at info level to diagnose authentication issues
		// Only log non-sensitive fields to prevent information leakage
		logHelper := log.NewHelper(i.cfg.logger)
		if claims != nil {
			logHelper.Infof("Stream authentication decision for %s: %s (subject: %s, authType: %s)",
				info.FullMethod, decision, claims.SubjectId, claims.AuthType)
		} else {
			logHelper.Infof("Stream authentication decision for %s: %s (claims: nil)", info.FullMethod, decision)
		}

		if err := validateAuthDecision(decision, claims); err != nil {
			// Log at appropriate level based on decision
			if decision == authnapi.Deny || decision == authnapi.Ignore {
				logHelper.Warnf("Stream authentication failed for %s: %v", info.FullMethod, err)
			} else {
				logHelper.Errorf("Stream authentication failed for %s: %v", info.FullMethod, err)
			}
			return err
		}

		// Log only non-sensitive fields at debug level
		logHelper.Debugf("Stream authentication allowed for %s (subject: %s, authType: %s)",
			info.FullMethod, claims.SubjectId, claims.AuthType)

		// Store claims in AuthzContext (the authoritative source for auth info)
		newCtx = EnsureAuthzContext(newCtx, claims)

		// Preserve token if available (for compatibility)
		newCtx = preserveStreamTokenContext(newCtx, md)

		wrappedStream := &authServerStream{ServerStream: ss, ctx: newCtx}
		return handler(srv, wrappedStream)
	}
}

type authServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (a *authServerStream) Context() context.Context {
	return a.ctx
}

// preserveStreamTokenContext extracts and preserves the token in context for legacy compatibility.
// Note: ClientID is now available via AuthzContext.Claims.ClientID for OIDC auth.
// This token preservation can be removed once all callers migrate to using Claims.
func preserveStreamTokenContext(ctx context.Context, md metadata.MD) context.Context {
	if token, ok := util.FromTokenContext(ctx); ok {
		return util.NewTokenContext(ctx, token)
	}

	authHeader := md.Get(authorizationKey)
	if len(authHeader) == 0 {
		return ctx
	}

	parts := strings.SplitN(authHeader[0], " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return util.NewRawTokenContext(ctx, parts[1])
	}

	return ctx
}

// FromStreamContext retrieves the OIDC ID token from context.
func FromStreamContext(ctx context.Context) (*coreosoidc.IDToken, bool) {
	return util.FromTokenContext(ctx)
}

// GetClientIDFromContext extracts the client_id from a JWT token stored in the context.
// Deprecated: Use Claims.ClientID instead once ClientID is added to Claims.
func GetClientIDFromContext(ctx context.Context) string {
	// First, try to get the verified IDToken from context
	token, ok := util.FromTokenContext(ctx)
	if ok {
		claims := &StreamTokenClaims{}
		err := token.Claims(claims)
		if err != nil {
			return ""
		}
		if claims.ClientID != "" {
			return claims.ClientID
		}
		return claims.AuthorizedParty
	}

	// Fallback: try to get raw token from context and decode it
	rawToken, ok := util.FromRawTokenContext(ctx)
	if !ok {
		return ""
	}

	claimsMap, err := util.DecodeJWTClaims(rawToken)
	if err != nil || claimsMap == nil {
		return ""
	}

	if clientID, ok := claimsMap["client_id"].(string); ok && clientID != "" {
		return clientID
	}

	if azp, ok := claimsMap["azp"].(string); ok && azp != "" {
		return azp
	}

	return ""
}

// StreamTokenClaims holds the values we want to extract from the JWT - matching OIDC authenticator
type StreamTokenClaims struct {
	Audience          string `json:"aud"`
	Issuer            string `json:"iss"`
	Subject           string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	ClientID          string `json:"client_id"`
	AuthorizedParty   string `json:"azp"`
}
