package data

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
	relationsTokenLifeDuration    = 5 * time.Minute
	relationsCacheCleanupInterval = 5 * time.Minute
	relationsGrantType            = "client_credentials"
)

type secureMetadataCreds map[string]string

func (c secureMetadataCreds) RequireTransportSecurity() bool { return true }
func (c secureMetadataCreds) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return c, nil
}

func withBearerToken(token string) grpc.CallOption {
	return grpc.PerRPCCredentials(secureMetadataCreds{"Authorization": "Bearer " + token})
}

type insecureMetadataCreds map[string]string

func (c insecureMetadataCreds) RequireTransportSecurity() bool { return false }
func (c insecureMetadataCreds) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return c, nil
}

func withInsecureBearerToken(token string) grpc.CallOption {
	return grpc.PerRPCCredentials(insecureMetadataCreds{"Authorization": "Bearer " + token})
}

type relationsTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type relationsTokenClient struct {
	ClientID       string
	ClientSecret   string
	URL            string
	EnableOIDCAuth bool
	Insecure       bool
	cache          *cache.Cache
}

func newRelationsTokenClient(config *relationsTokenClientConfig) *relationsTokenClient {
	return &relationsTokenClient{
		ClientID:       config.clientId,
		ClientSecret:   config.clientSecret,
		URL:            config.url,
		EnableOIDCAuth: config.enableOIDCAuth,
		Insecure:       config.insecure,
		cache:          cache.New(relationsTokenLifeDuration, relationsCacheCleanupInterval),
	}
}

func (a *relationsTokenClient) getCachedToken(tokenKey string) (string, error) {
	cachedToken, isCached := a.cache.Get(tokenKey)
	ct, _ := cachedToken.(string)
	if isCached {
		return ct, nil
	}
	return "", fmt.Errorf("failed to retrieve cached token")
}

func isJWTTokenExpired(accessToken string) (bool, time.Time) {
	if token, _ := jwt.Parse(accessToken, nil); token != nil {
		tokenClaims := token.Claims.(jwt.MapClaims)
		if _, ok := tokenClaims["exp"]; ok {
			expTime := time.Unix(int64(tokenClaims["exp"].(float64)), 0)
			return time.Now().After(expTime), expTime
		}
	}
	return true, time.Time{}
}

func (a *relationsTokenClient) getToken() (*relationsTokenResponse, error) {
	cachedTokenKey := fmt.Sprintf("%s%s", a.URL, a.ClientID)
	cachedToken, _ := a.getCachedToken(cachedTokenKey)
	isExpired, _ := isJWTTokenExpired(cachedToken)
	if cachedToken != "" && !isExpired {
		return &relationsTokenResponse{AccessToken: cachedToken}, nil
	}

	client := &http.Client{}
	data := url.Values{}
	data.Set("client_id", a.ClientID)
	data.Set("client_secret", a.ClientSecret)
	data.Set("grant_type", relationsGrantType)
	req, err := http.NewRequest("POST", a.URL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var tokenResponse relationsTokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token response: %w", err)
	}
	a.cache.Set(cachedTokenKey, tokenResponse.AccessToken, relationsCacheCleanupInterval)
	return &tokenResponse, nil
}
