package kessel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/patrickmn/go-cache"
	"google.golang.org/grpc"
)

const (
	tokenLifeDuration            = 5 * time.Minute
	cacheCleanupInterval         = 5 * time.Minute
	client_credentials_granttype = "client_credentials"
)

type secureMetadataCreds map[string]string

func (c secureMetadataCreds) RequireTransportSecurity() bool { return true }
func (c secureMetadataCreds) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return c, nil
}

// WithBearerToken returns a grpc.CallOption that adds a standard HTTP Bearer
// token to all requests sent from a client.
func WithBearerToken(token string) grpc.CallOption {
	return grpc.PerRPCCredentials(secureMetadataCreds{"Authorization": "Bearer " + token})
}

type insecureMetadataCreds map[string]string

func (c insecureMetadataCreds) RequireTransportSecurity() bool { return false }
func (c insecureMetadataCreds) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return c, nil
}

// WithInsecureBearerToken returns a grpc.CallOption that adds a standard HTTP
// Bearer token to all requests sent from an insecure client.
//
// Must be used in conjunction with `insecure.NewCredentials()`.
func WithInsecureBearerToken(token string) grpc.CallOption {
	return grpc.PerRPCCredentials(insecureMetadataCreds{"Authorization": "Bearer " + token})
}

// NewTokenClient creates and returns a new tokenClient client.
func NewTokenClient(config *tokenClientConfig) *tokenClient {
	return &tokenClient{
		ClientID:       config.clientId,
		ClientSecret:   config.clientSecret,
		URL:            config.url,
		EnableOIDCAuth: config.enableOIDCAuth,
		Insecure:       config.insecure,
		cache:          cache.New(tokenLifeDuration, cacheCleanupInterval),
	}
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type tokenClient struct {
	ClientID       string
	ClientSecret   string
	URL            string
	EnableOIDCAuth bool
	Insecure       bool
	cache          *cache.Cache
}

func (a *tokenClient) GetCachedToken(tokenKey string) (string, error) {
	cachedToken, isCached := a.cache.Get(tokenKey)
	ct, _ := cachedToken.(string)
	if isCached {
		return ct, nil
	}
	return "", fmt.Errorf("failed to retrieve cached token")
}

func IsJWTTokenExpired(accessToken string) (bool, time.Time) {
	if token, _ := jwt.Parse(accessToken, nil); token != nil {
		tokenClaims := token.Claims.(jwt.MapClaims)
		if _, ok := tokenClaims["exp"]; ok {
			expTime := time.Unix(int64(tokenClaims["exp"].(float64)), 0)
			return time.Now().After(expTime), expTime
		}
	}
	return true, time.Time{}
}

func (a *tokenClient) getToken() (*TokenResponse, error) {

	cachedTokenKey := fmt.Sprintf("%s%s", a.URL, a.ClientID)
	cachedToken, _ := a.GetCachedToken(cachedTokenKey)
	IsExpired, _ := IsJWTTokenExpired(cachedToken)
	if cachedToken != "" && !IsExpired {
		return &TokenResponse{AccessToken: cachedToken}, nil
	}

	client := &http.Client{}
	data := url.Values{}
	data.Set("client_id", a.ClientID)
	data.Set("client_secret", a.ClientSecret)
	data.Set("grant_type", client_credentials_granttype)
	req, err := http.NewRequest("POST", a.URL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {

		return nil, fmt.Errorf("failed to parse token response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var tokenResponse TokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token response: %v", err)
	}
	a.cache.Set(cachedTokenKey, tokenResponse.AccessToken, cacheCleanupInterval)
	return &tokenResponse, nil
}
