package interceptor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/project-kessel/inventory-api/internal/middleware"

	coreosoidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/project-kessel/inventory-api/internal/authn"
	"github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	// bearerWord the bearer key word for authorization
	bearerWord string = "bearer"

	// authorizationKey holds the key used to store the JWT Token in the request tokenHeader.
	authorizationKey string = "authorization"
)

type StreamAuthConfig struct {
	authn.CompletedConfig
	clientContext context.Context
	verifier      *coreosoidc.IDTokenVerifier
}

type StreamAuthOption func(*StreamAuthConfig)

func NewStreamAuthInterceptor(config authn.CompletedConfig, opts ...StreamAuthOption) (*StreamAuthInterceptor, error) {
	cfg := &StreamAuthConfig{
		CompletedConfig: config,
	}

	if config.Oidc.PrincipalUserDomain == "" {
		config.Oidc.PrincipalUserDomain = "localhost"
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if config.Oidc != nil {
		// Use the same OIDC provider setup as the OIDC authenticator
		ctx := coreosoidc.ClientContext(context.Background(), config.Oidc.Client)
		provider, err := coreosoidc.NewProvider(ctx, config.Oidc.AuthorizationServerURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
		}

		oidcConfig := &coreosoidc.Config{
			ClientID:          config.Oidc.ClientId,
			SkipClientIDCheck: config.Oidc.SkipClientIDCheck,
			SkipIssuerCheck:   config.Oidc.SkipIssuerCheck,
		}

		cfg.clientContext = ctx
		cfg.verifier = provider.Verifier(oidcConfig)
	}

	return &StreamAuthInterceptor{cfg: cfg}, nil
}

type StreamAuthInterceptor struct {
	cfg *StreamAuthConfig
}

func (i *StreamAuthInterceptor) Interceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

		if i.cfg.verifier == nil {
			return jwt.ErrMissingKeyFunc
		}

		newCtx := ss.Context()
		md, ok := metadata.FromIncomingContext(newCtx)
		if !ok {
			return jwt.ErrMissingJwtToken
		}

		authHeader, ok := md[authorizationKey]
		if !ok || len(authHeader) == 0 {
			return jwt.ErrMissingJwtToken
		}

		auths := strings.SplitN(authHeader[0], " ", 2)
		if len(auths) != 2 || !strings.EqualFold(auths[0], bearerWord) {
			return jwt.ErrMissingJwtToken
		}

		jwtToken := auths[1]

		// Use OIDC verification like the OIDC authenticator
		idToken, err := i.cfg.verifier.Verify(i.cfg.clientContext, jwtToken)
		if err != nil {
			log.Errorf("failed to verify the access token: %v", err)
			return jwt.ErrTokenInvalid
		}

		// Extract claims using the same structure as OIDC authenticator
		claims := &Claims{}
		err = idToken.Claims(claims)
		if err != nil {
			log.Errorf("failed to extract claims: %v", err)
			return jwt.ErrTokenInvalid
		}

		// Audience check matching OIDC authenticator
		if i.cfg.Oidc.EnforceAudCheck {
			if claims.Audience != i.cfg.Oidc.ClientId {
				log.Debugf("aud does not match the requesting client-id -- decision DENY")
				return errors.New("audience mismatch")
			}
		}

		// Create identity matching OIDC authenticator
		if claims.Subject != "" && i.cfg.Oidc.PrincipalUserDomain != "" {
			principal := fmt.Sprintf("%s/%s", i.cfg.Oidc.PrincipalUserDomain, claims.Subject)
			newCtx = NewContextIdentity(newCtx, api.Identity{Principal: principal})
		}

		// Store the ID token in context for compatibility
		newCtx = util.NewTokenContext(newCtx, idToken)

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

// Claims holds the values we want to extract from the JWT - matching OIDC authenticator
type Claims struct {
	Audience          string `json:"aud"`
	Issuer            string `json:"iss"`
	Subject           string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	ClientID          string `json:"client_id"`
}

func NewContextIdentity(ctx context.Context, identity api.Identity) context.Context {
	return context.WithValue(ctx, middleware.IdentityRequestKey, identity)
}

func FromContextIdentity(ctx context.Context) (api.Identity, bool) {
	identity, ok := ctx.Value(middleware.IdentityRequestKey).(api.Identity)
	return identity, ok
}

func FromContext(ctx context.Context) (*coreosoidc.IDToken, bool) {
	return util.FromTokenContext(ctx)
}

// GetClientIDFromContext extracts the client_id from a JWT token stored in the context.
func GetClientIDFromContext(ctx context.Context) string {
	// First, try to get the verified IDToken from context
	token, ok := util.FromTokenContext(ctx)
	if ok {
		claims := &Claims{}
		err := token.Claims(claims)
		if err != nil {
			return ""
		}
		if claims.ClientID != "" {
			return claims.ClientID
		}
		return claims.Audience
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

	if aud, ok := claimsMap["aud"].(string); ok && aud != "" {
		return aud
	}

	return ""
}
