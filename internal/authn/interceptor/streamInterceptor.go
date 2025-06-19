package interceptor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/project-kessel/inventory-api/internal/middleware"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/project-kessel/inventory-api/internal/authn"
	"github.com/project-kessel/inventory-api/internal/authn/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type authKey struct{}

const (
	// bearerWord the bearer key word for authorization
	bearerWord string = "bearer"

	// authorizationKey holds the key used to store the JWT Token in the request tokenHeader.
	authorizationKey string = "authorization"
)

type StreamAuthConfig struct {
	authn.CompletedConfig
	signingMethod jwtv5.SigningMethod
	claims        func() jwtv5.Claims
	tokenHeader   map[string]interface{}
	jwks          keyfunc.Keyfunc
}
type StreamAuthOption func(*StreamAuthConfig)

func WithClaims(claimsFunc func() jwtv5.Claims) StreamAuthOption {
	return func(o *StreamAuthConfig) {
		o.claims = claimsFunc
	}
}

func WithTokenHeader(header map[string]interface{}) StreamAuthOption {
	return func(o *StreamAuthConfig) {
		o.tokenHeader = header
	}
}

func WithSigningMethod(signingMethod jwtv5.SigningMethod) StreamAuthOption {
	return func(o *StreamAuthConfig) {
		o.signingMethod = signingMethod
	}
}

func NewStreamAuthInterceptor(config authn.CompletedConfig, opts ...StreamAuthOption) (*StreamAuthInterceptor, error) {
	cfg := &StreamAuthConfig{
		CompletedConfig: config,
		signingMethod:   jwtv5.SigningMethodRS256,
	}

	if config.Oidc.PrincipalUserDomain == "" {
		config.Oidc.PrincipalUserDomain = "localhost"
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if config.Oidc != nil {
		jwks, err := FetchJwks(config.Oidc.AuthorizationServerURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
		}
		cfg.jwks = jwks
	}

	return &StreamAuthInterceptor{cfg: cfg}, nil
}

type StreamAuthInterceptor struct {
	cfg *StreamAuthConfig
}

func (i *StreamAuthInterceptor) Interceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

		if i.cfg.jwks == nil {
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
		var (
			tokenInfo *jwtv5.Token
			err       error
		)

		if i.cfg.claims != nil {
			tokenInfo, err = jwtv5.ParseWithClaims(jwtToken, i.cfg.claims(), i.cfg.jwks.Keyfunc)
		} else {
			tokenInfo, err = jwtv5.Parse(jwtToken, i.cfg.jwks.Keyfunc)
		}

		if err != nil {
			if errors.Is(err, jwtv5.ErrTokenMalformed) || errors.Is(err, jwtv5.ErrTokenUnverifiable) {
				return jwt.ErrTokenInvalid
			}
			if errors.Is(err, jwtv5.ErrTokenNotValidYet) || errors.Is(err, jwtv5.ErrTokenExpired) {
				return jwt.ErrTokenExpired
			}
			return jwt.ErrTokenParseFail
		}

		if !tokenInfo.Valid {
			return jwt.ErrTokenInvalid
		}

		if tokenInfo.Method != i.cfg.signingMethod {
			return jwt.ErrUnSupportSigningMethod
		}

		newCtx = NewContext(newCtx, tokenInfo.Claims)
		sub, err := tokenInfo.Claims.GetSubject()
		if err != nil {
			return err
		}

		audience, err := tokenInfo.Claims.GetAudience()
		if err != nil {
			return err
		}
		if len(audience) == 0 {
			return errors.New("no audience claim found")
		}

		if i.cfg.Oidc.EnforceAudCheck {
			if audience[0] != i.cfg.Oidc.ClientId {
				log.Debugf("aud does not match the requesting client-id -- decision DENY")
				return errors.New("audience mismatch")
			}
		}

		if sub != "" && i.cfg.Oidc.PrincipalUserDomain != "" {
			principal := fmt.Sprintf("%s/%s", i.cfg.Oidc.PrincipalUserDomain, sub)
			newCtx = NewContextIdentity(newCtx, api.Identity{Principal: principal})
		}

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

func NewContext(ctx context.Context, info jwtv5.Claims) context.Context {
	return context.WithValue(ctx, authKey{}, info)
}

func NewContextIdentity(ctx context.Context, identity api.Identity) context.Context {
	return context.WithValue(ctx, middleware.IdentityRequestKey, identity)
}

func FromContextIdentity(ctx context.Context) (api.Identity, bool) {
	identity, ok := ctx.Value(middleware.IdentityRequestKey).(api.Identity)
	return identity, ok
}

func FromContext(ctx context.Context) (jwtv5.Claims, bool) {
	claims, ok := ctx.Value(authKey{}).(jwtv5.Claims)
	return claims, ok
}

func FetchJwks(authServerUrl string) (keyfunc.Keyfunc, error) {
	jwksURL := fmt.Sprintf(authServerUrl + "/protocol/openid-connect/certs")
	jwks, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		log.Fatalf("Failed to create JWK Set from resource at the given URL: %s.\nError: %s", authServerUrl, err)
		return nil, err
	}
	return jwks, nil
}
