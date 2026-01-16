package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateAuthenticator_OIDC(t *testing.T) {
	tests := []struct {
		name    string
		config  interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "oidc authenticator requires",
		},
		{
			name:    "wrong config type",
			config:  "invalid",
			wantErr: true,
			errMsg:  "oidc authenticator requires *oidc.CompletedConfig",
		},
		// Note: Testing with valid OIDC config requires actual OIDC provider setup
		// which is complex and better tested in integration tests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := CreateAuthenticator(TypeOIDC, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, auth)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, auth)
			}
		})
	}
}

func TestCreateAuthenticator_Guest(t *testing.T) {
	tests := []struct {
		name    string
		config  interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: false,
		},
		{
			name:    "non-nil config",
			config:  "invalid",
			wantErr: true,
			errMsg:  "guest authenticator does not require config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := CreateAuthenticator(TypeGuest, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, auth)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, auth)
			}
		})
	}
}

func TestCreateAuthenticator_AllowUnauthenticated(t *testing.T) {
	auth, err := CreateAuthenticator(TypeAllowUnauthenticated, nil)
	assert.NoError(t, err)
	assert.NotNil(t, auth)
}

func TestCreateAuthenticator_XRhIdentity(t *testing.T) {
	tests := []struct {
		name    string
		config  interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: false,
		},
		{
			name:    "non-nil config",
			config:  "invalid",
			wantErr: true,
			errMsg:  "x-rh-identity authenticator does not require config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := CreateAuthenticator(TypeXRhIdentity, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, auth)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, auth)
			}
		})
	}
}

func TestCreateAuthenticator_UnknownType(t *testing.T) {
	auth, err := CreateAuthenticator("unknown-type", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown authenticator type")
	assert.Nil(t, auth)
}
