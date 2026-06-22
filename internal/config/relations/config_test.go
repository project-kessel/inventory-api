package relations

import (
	"context"
	"testing"

	"github.com/project-kessel/inventory-api/internal/config/relations/kessel"
	"github.com/project-kessel/inventory-api/internal/config/relations/spicedb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Complete_AllowAll_Success(t *testing.T) {
	cfg := &Config{
		Authz:  AllowAll,
		Kessel: nil,
	}

	completed, errs := cfg.Complete(context.Background())

	assert.Nil(t, errs, "AllowAll config should complete without errors")
	assert.NotNil(t, completed.completedConfig, "Completed config should not be nil")
	assert.Equal(t, AllowAll, completed.Authz, "Authz should be AllowAll")
}

func TestConfig_Complete_SpiceDB_Success(t *testing.T) {
	spicedbCfg := spicedb.NewConfig(&spicedb.Options{
		Endpoint:   "localhost:50051",
		TokenFile:  "/tmp/token",
		SchemaFile: "/tmp/schema.zed",
		UseTLS:     false,
	})

	cfg := &Config{
		Authz:   SpiceDB,
		SpiceDB: spicedbCfg,
	}

	completed, errs := cfg.Complete(context.Background())

	assert.Nil(t, errs, "SpiceDB config should complete without errors")
	assert.NotNil(t, completed.completedConfig, "Completed config should not be nil")
	assert.Equal(t, SpiceDB, completed.Authz, "Authz should be SpiceDB")
}

func TestConfig_Complete_Kessel_SwallowsErrors(t *testing.T) {
	// This test documents the buggy behavior where Complete swallows
	// errors from kessel.Config.Complete() and returns nil instead.
	// See config.go line 41-42:
	//   if ksl, errs := c.Kessel.Complete(ctx); errs != nil {
	//       return CompletedConfig{}, nil  // BUG: should return errs
	//   }
	//
	// This bug means that if kessel.Config.Complete ever returns an error
	// (e.g., grpc.NewClient fails), the error is silently swallowed and
	// nil is returned instead, giving the caller no indication of failure.
	//
	// NOTE: In practice, grpc.NewClient rarely fails during creation (it defers
	// connection establishment), so this bug may not manifest often, but the
	// principle is wrong: errors should be propagated, not swallowed.

	// For now, we test the happy path (no error from kessel.Complete)
	// and document that the error-swallowing code path exists but is
	// difficult to trigger in tests.
	kesselCfg := kessel.NewConfig(&kessel.Options{
		URL:            "localhost:9000",
		Insecure:       true,
		ClientId:       "test-client",
		ClientSecret:   "test-secret",
		TokenEndpoint:  "http://localhost:8080/token",
		EnableOidcAuth: false,
	})

	cfg := &Config{
		Authz:  Kessel,
		Kessel: kesselCfg,
	}

	completed, errs := cfg.Complete(context.Background())

	// Current behavior: when kessel.Complete succeeds, no errors
	assert.Nil(t, errs, "When kessel.Complete succeeds, no errors should be returned")
	assert.NotNil(t, completed.completedConfig, "Completed config should not be nil")
	assert.Equal(t, Kessel, completed.Authz, "Authz should be Kessel")
	assert.NotNil(t, completed.Kessel, "Kessel config should be present")

	// The bug exists in the code but is documented here in comments:
	// IF kessel.Complete were to return errors, they would be swallowed
	// and nil would be returned instead (see config.go:42).
}

func TestCheckRelationsImpl_AllCases(t *testing.T) {
	tests := []struct {
		name         string
		authz        string
		expectedType string
	}{
		{
			name:         "AllowAll returns allow-all",
			authz:        AllowAll,
			expectedType: AllowAll,
		},
		{
			name:         "Kessel returns kessel",
			authz:        Kessel,
			expectedType: Kessel,
		},
		{
			name:         "SpiceDB returns spicedb",
			authz:        SpiceDB,
			expectedType: SpiceDB,
		},
		{
			name:         "Unknown value returns unknown",
			authz:        "some-random-value",
			expectedType: "unknown",
		},
		{
			name:         "Empty string returns unknown",
			authz:        "",
			expectedType: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CompletedConfig{
				completedConfig: &completedConfig{
					Authz: tt.authz,
				},
			}

			result := CheckRelationsImpl(config)
			require.Equal(t, tt.expectedType, result)
		})
	}
}
