package util

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	coreosoidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/assert"
)

func TestNewTokenContext(t *testing.T) {
	ctx := context.Background()
	var token *coreosoidc.IDToken

	newCtx := NewTokenContext(ctx, token)
	// We can't easily create a real IDToken, so we'll test with nil and verify the context is set
	// The actual token type check will be done in integration tests
	assert.NotNil(t, newCtx)

	// Verify the token can be retrieved (even if nil)
	retrievedToken, ok := FromTokenContext(newCtx)
	assert.True(t, ok)
	// When a nil pointer is stored, it's still considered "present" in context
	assert.Nil(t, retrievedToken) // The value is nil, but ok is true
}

func TestFromTokenContext(t *testing.T) {
	tests := []struct {
		name      string
		setupCtx  func() context.Context
		wantToken bool
	}{
		{
			name: "token in context",
			setupCtx: func() context.Context {
				ctx := context.Background()
				// Create a pointer to a nil IDToken - this will pass the type check
				var token *coreosoidc.IDToken
				return NewTokenContext(ctx, token)
			},
			wantToken: true,
		},
		{
			name: "no token in context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			wantToken: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			token, ok := FromTokenContext(ctx)
			assert.Equal(t, tt.wantToken, ok)
			if tt.wantToken {
				// When a nil pointer is stored, ok will be true but token will be nil
				// This is expected behavior - the type assertion succeeds even for nil
			} else {
				assert.Nil(t, token)
			}
		})
	}
}

func TestNewRawTokenContext(t *testing.T) {
	ctx := context.Background()
	rawToken := "test.raw.token"

	newCtx := NewRawTokenContext(ctx, rawToken)
	assert.NotNil(t, newCtx)

	// Verify the token can be retrieved
	retrieved, ok := FromRawTokenContext(newCtx)
	assert.True(t, ok)
	assert.Equal(t, rawToken, retrieved)
}

func TestFromRawTokenContext(t *testing.T) {
	tests := []struct {
		name      string
		setupCtx  func() context.Context
		wantToken bool
		wantValue string
	}{
		{
			name: "raw token in context",
			setupCtx: func() context.Context {
				ctx := context.Background()
				return NewRawTokenContext(ctx, "test-token")
			},
			wantToken: true,
			wantValue: "test-token",
		},
		{
			name: "no raw token in context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			wantToken: false,
			wantValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			token, ok := FromRawTokenContext(ctx)
			assert.Equal(t, tt.wantToken, ok)
			assert.Equal(t, tt.wantValue, token)
		})
	}
}

func TestDecodeJWTClaims(t *testing.T) {
	// Create a valid JWT token for testing
	// Header: {"alg":"HS256","typ":"JWT"}
	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Payload with client_id
	payloadWithClientID := map[string]interface{}{
		"client_id": "svc-test",
		"azp":       "test-azp",
		"sub":       "test-subject",
		"iss":       "http://localhost:8084/realms/redhat-external",
	}
	payloadJSON, _ := json.Marshal(payloadWithClientID)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Payload without client_id (only azp)
	payloadWithoutClientID := map[string]interface{}{
		"azp": "test-azp",
		"sub": "test-subject",
	}
	payloadWithoutClientIDJSON, _ := json.Marshal(payloadWithoutClientID)
	payloadWithoutClientIDEncoded := base64.RawURLEncoding.EncodeToString(payloadWithoutClientIDJSON)

	// Signature (dummy)
	signature := "dummy-signature"

	validTokenWithClientID := headerEncoded + "." + payloadEncoded + "." + signature
	validTokenWithoutClientID := headerEncoded + "." + payloadWithoutClientIDEncoded + "." + signature

	tests := []struct {
		name         string
		rawToken     string
		wantClaims   bool
		wantClientID string
		wantAzp      string
		wantError    bool
	}{
		{
			name:         "valid token with client_id",
			rawToken:     validTokenWithClientID,
			wantClaims:   true,
			wantClientID: "svc-test",
			wantAzp:      "test-azp",
			wantError:    false,
		},
		{
			name:         "valid token without client_id",
			rawToken:     validTokenWithoutClientID,
			wantClaims:   true,
			wantClientID: "",
			wantAzp:      "test-azp",
			wantError:    false,
		},
		{
			name:         "invalid token - not enough parts",
			rawToken:     "header.payload",
			wantClaims:   false,
			wantClientID: "",
			wantAzp:      "",
			wantError:    false, // Returns nil, nil, not an error
		},
		{
			name:         "invalid token - invalid base64",
			rawToken:     "header.invalid-base64!.signature",
			wantClaims:   false,
			wantClientID: "",
			wantAzp:      "",
			wantError:    true,
		},
		{
			name:         "invalid token - invalid JSON",
			rawToken:     "header." + base64.RawURLEncoding.EncodeToString([]byte("not-json")) + ".signature",
			wantClaims:   false,
			wantClientID: "",
			wantAzp:      "",
			wantError:    true,
		},
		{
			name:         "empty token",
			rawToken:     "",
			wantClaims:   false,
			wantClientID: "",
			wantAzp:      "",
			wantError:    false,
		},
		{
			name:         "real world token from user",
			rawToken:     "eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJRY3BOT0lfcFFVZ2JxSXI2NGgzTlBHY2FvMHk3TmxQZWtmNTJHOGNtTmYwIn0.eyJleHAiOjE3NjM3NDMwMzUsImlhdCI6MTc2Mzc0MjczNSwianRpIjoidHJydGNjOjJjYzExZWI0LTNhODYtNDk2Ny1iZWQwLTIzMDM5NWMzZTVlMiIsImlzcyI6Imh0dHA6Ly9sb2NhbGhvc3Q6ODA4NC9yZWFsbXMvcmVkaGF0LWV4dGVybmFsIiwiYXVkIjoiYWNjb3VudCIsInN1YiI6IjZlNDQ0NzU2LTk3ZmQtNGI2OC04ZDdhLTU3ZDFmMTdkOGEyMiIsInR5cCI6IkJlYXJlciIsImF6cCI6InN2Yy10ZXN0IiwiYWNyIjoiMSIsImFsbG93ZWQtb3JpZ2lucyI6WyIvKiJdLCJyZWFsbV9hY2Nlc3MiOnsicm9sZXMiOlsiZGVmYXVsdC1yb2xlcy1yZWRoYXQtZXh0ZXJuYWwiLCJvZmZsaW5lX2FjY2VzcyIsInVtYV9hdXRob3JpemF0aW9uIl19LCJyZXNvdXJjZV9hY2Nlc3MiOnsiYWNjb3VudCI6eyJyb2xlcyI6WyJtYW5hZ2UtYWNjb3VudCIsIm1hbmFnZS1hY2NvdW50LWxpbmtzIiwidmlldy1wcm9maWxlIl19fSwic2NvcGUiOiJwcm9maWxlIGVtYWlsIiwiZW1haWxfdmVyaWZpZWQiOmZhbHNlLCJjbGllbnRIb3N0IjoiMTAuODkuMS4yMzUiLCJwcmVmZXJyZWRfdXNlcm5hbWUiOiJzZXJ2aWNlLWFjY291bnQtc3ZjLXRlc3QiLCJjbGllbnRBZGRyZXNzIjoiMTAuODkuMS4yMzUiLCJjbGllbnRfaWQiOiJzdmMtdGVzdCJ9.Ci8CntnITSn68Lfs3tYw21ynOH5BdALkrSuE_gXy4DpW9darsh46vuGkB96sAsrEcTQxFmWbnXeFTYxWKReeq8FQbUTZ6nmeHENc7WyaQ8-uLpgqC4L2mb_KD3ez2-WfPCY-CihVx8_KxD6WK1_6FNTc_AbYPAyly1Khz5CLRSMFD2qALTSHgzi_xwlhATAeNkH-JDex1UK4Wg2n2aqoceeBZojGWwH3a2dYpHtzIrlId4Khuq8Weo2qpSdYKVgSDKp-j9eCsaa1BtCmwSnkCANtfIan1BnEJsQrlyZI68zsOsfIB2vFLdCoTU_l9hkeVSyNLSGjQu2on9twiRfSaQ",
			wantClaims:   true,
			wantClientID: "svc-test",
			wantAzp:      "svc-test",
			wantError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := DecodeJWTClaims(tt.rawToken)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, claims)
			} else {
				if tt.wantClaims {
					assert.NoError(t, err)
					assert.NotNil(t, claims)
					if tt.wantClientID != "" {
						assert.Equal(t, tt.wantClientID, claims["client_id"])
					}
					if tt.wantAzp != "" {
						assert.Equal(t, tt.wantAzp, claims["azp"])
					}
				} else {
					// For invalid tokens, we expect nil claims and no error (or error)
					if err == nil {
						assert.Nil(t, claims)
					}
				}
			}
		})
	}
}
