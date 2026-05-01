package linear_test

import (
	"errors"
	"testing"

	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFromEnvCredentialPrecedenceKASMOSLinearAPIKeyWins(t *testing.T) {
	t.Setenv("KASMOS_LINEAR_API_KEY", "kasmos-secret-token")
	t.Setenv("LINEAR_API_KEY", "linear-secret-token")

	cfg, err := linear.ConfigFromEnv()
	require.NoError(t, err)
	assert.Equal(t, "kasmos-secret-token", cfg.APIKey)
	assert.Equal(t, linear.DefaultEndpoint, cfg.Endpoint)
}

func TestConfigFromEnvCredentialPrecedenceUsesLinearAPIKeyWhenPreferredEmpty(t *testing.T) {
	tests := []struct {
		name         string
		preferredKey string
	}{
		{name: "empty", preferredKey: ""},
		{name: "whitespace", preferredKey: " \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KASMOS_LINEAR_API_KEY", tt.preferredKey)
			t.Setenv("LINEAR_API_KEY", "fallback-secret-token")

			cfg, err := linear.ConfigFromEnv()
			require.NoError(t, err)
			assert.Equal(t, "fallback-secret-token", cfg.APIKey)
			assert.Equal(t, linear.DefaultEndpoint, cfg.Endpoint)
		})
	}
}

func TestConfigFromEnvErrNotConfiguredWhenCredentialsMissing(t *testing.T) {
	t.Setenv("KASMOS_LINEAR_API_KEY", "")
	t.Setenv("LINEAR_API_KEY", "")

	cfg, err := linear.ConfigFromEnv()
	require.Error(t, err)
	assert.True(t, errors.Is(err, linear.ErrNotConfigured))
	assert.Equal(t, linear.Config{}, cfg)
}

func TestConfigFromEnvEndpointDefaultAndOverride(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{name: "endpoint default", endpoint: "", want: linear.DefaultEndpoint},
		{name: "endpoint override", endpoint: "https://linear.example/graphql", want: "https://linear.example/graphql"},
		{name: "endpoint whitespace default", endpoint: " \t\n ", want: linear.DefaultEndpoint},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KASMOS_LINEAR_API_KEY", "secret-token-value")
			t.Setenv("LINEAR_API_KEY", "")
			t.Setenv("KASMOS_LINEAR_API_URL", tt.endpoint)

			cfg, err := linear.ConfigFromEnv()
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.Endpoint)
		})
	}
}

func TestConfigFromLookupUsesProvidedValues(t *testing.T) {
	cfg, err := linear.ConfigFromLookup(func(key string) (string, bool) {
		values := map[string]string{
			"KASMOS_LINEAR_API_KEY": "lookup-secret-token",
			"KASMOS_LINEAR_API_URL": "https://lookup.example/graphql",
		}
		value, ok := values[key]
		return value, ok
	})

	require.NoError(t, err)
	assert.Equal(t, "lookup-secret-token", cfg.APIKey)
	assert.Equal(t, "https://lookup.example/graphql", cfg.Endpoint)
}

func TestConfigStringRedactsCredential(t *testing.T) {
	cfg := linear.Config{
		Endpoint: linear.DefaultEndpoint,
		APIKey:   "secret-token-value",
	}

	assert.NotContains(t, cfg.String(), "secret-token-value")
	assert.Contains(t, cfg.String(), "[REDACTED]")
}

func TestErrNotConfiguredDoesNotLeakEnvironmentValue(t *testing.T) {
	t.Setenv("KASMOS_LINEAR_API_KEY", "")
	t.Setenv("LINEAR_API_KEY", "")
	t.Setenv("KASMOS_LINEAR_API_URL", "https://secret-token-value.example/graphql")

	_, err := linear.ConfigFromEnv()
	require.Error(t, err)
	assert.True(t, errors.Is(err, linear.ErrNotConfigured))
	assert.NotContains(t, err.Error(), "secret-token-value")
}
