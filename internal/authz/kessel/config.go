package kessel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/authzed/grpcutil"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type Config struct {
	*Options
}

func NewConfig(o *Options) *Config {
	return &Config{Options: o}
}

type completedConfig struct {
	gRPCConn *grpc.ClientConn
}

type CompletedConfig struct {
	*completedConfig
}

func (c *Config) Complete(ctx context.Context) (CompletedConfig, []error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.EmptyDialOption{})

	if c.enable_oidc_auth {
		var token *TokenResponse
		var err error
		// Initial token fetch
		token, err = c.getToken()
		if err != nil {
			return CompletedConfig{}, []error{err}
		}
		opts = append(opts, grpcutil.WithInsecureBearerToken(token.AccessToken))

		// Start a Go routine to refresh the token if it expires
		go func() {
			repeatDuration := time.Duration(c.token_refresh_minutes) * time.Minute
			ticker := time.NewTicker(repeatDuration)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if IsJWTTokenExpired(token.AccessToken) {
						token, err = c.getToken()
						if err != nil {
							log.Printf("Error refreshing token: %v", err)
							continue
						}
						// Update the token in the options
						opts = append([]grpc.DialOption{grpcutil.WithInsecureBearerToken(token.AccessToken)}, opts[1:]...)
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	if !c.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsConfig, _ := grpcutil.WithSystemCerts(grpcutil.VerifyCA)
		opts = append(opts, tlsConfig)
	}

	conn, err := grpc.NewClient(
		c.URL,
		opts...,
	)
	if err != nil {
		return CompletedConfig{}, []error{err}
	}
	if err != nil {
		return CompletedConfig{}, []error{err}
	}

	return CompletedConfig{&completedConfig{gRPCConn: conn}}, nil
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func (c *Config) getToken() (*TokenResponse, error) {
	client := &http.Client{}
	data := url.Values{}
	data.Set("client_id", c.sa_client_id)
	data.Set("client_secret", c.sa_client_secret)
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", c.sso_token_endpoint, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var tokenResponse TokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, err
	}

	return &tokenResponse, nil
}

func IsJWTTokenExpired(accessToken string) bool {
	if token, _ := jwt.Parse(accessToken, nil); token != nil {
		tokenClaims := token.Claims.(jwt.MapClaims)
		if _, ok := tokenClaims["exp"]; ok {
			expTime := time.Unix(int64(tokenClaims["exp"].(float64)), 0)
			return time.Now().After(expTime)
		}
	}
	return true
}
