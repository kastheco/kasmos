package linearruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDisabledOrMissingAPIKey(t *testing.T) {
	t.Setenv("KASMOS_LINEAR_API_KEY", "")
	t.Setenv("LINEAR_API_KEY", "")

	t.Run("missing config returns nil", func(t *testing.T) {
		resolved, err := Resolve(context.Background(), t.TempDir(), "proj", Options{})
		require.NoError(t, err)
		assert.Nil(t, resolved)
	})

	t.Run("disabled triggers returns nil", func(t *testing.T) {
		repo := writeRuntimeRepo(t, `[linear.triggers]
enabled = false
`)
		resolved, err := Resolve(context.Background(), repo, "proj", Options{})
		require.NoError(t, err)
		assert.Nil(t, resolved)
	})

	t.Run("missing api key returns nil", func(t *testing.T) {
		repo := writeRuntimeRepo(t, runtimeTriggerConfig())
		resolved, err := Resolve(context.Background(), repo, "proj", Options{})
		require.NoError(t, err)
		assert.Nil(t, resolved)
	})
}

func TestResolveBuildsPollerIngestorAndRepoLookup(t *testing.T) {
	t.Setenv("KASMOS_LINEAR_API_KEY", "")
	t.Setenv("LINEAR_API_KEY", "")
	t.Setenv("KASMOS_LINEAR_WEBHOOK_SECRET", "env-secret")
	repo := writeRuntimeRepo(t, runtimeTriggerConfig())
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".env"), []byte("KASMOS_LINEAR_API_KEY=dotenv-key\nKASMOS_LINEAR_WEBHOOK_SECRET=dotenv-secret\n"), 0o600))

	store := taskstore.NewTestSQLiteStore(t)
	resolved, err := Resolve(context.Background(), repo, "proj", Options{Store: store})
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.NotNil(t, resolved.Poller)
	require.NotNil(t, resolved.Ingestor)
	assert.Equal(t, "dotenv-key", resolved.LinearCfg.APIKey)
	secret, ok := resolved.SecretLookup("KASMOS_LINEAR_WEBHOOK_SECRET")
	require.True(t, ok)
	assert.Equal(t, "env-secret", secret)
}

func TestLinearConfigForRepoUsesEnvThenDotEnv(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".env"), []byte("KASMOS_LINEAR_API_KEY=dotenv-key\nKASMOS_LINEAR_API_URL=https://dotenv.example/graphql\n"), 0o600))
	t.Setenv("KASMOS_LINEAR_API_KEY", "env-key")
	t.Setenv("KASMOS_LINEAR_API_URL", "")

	cfg, err := LinearConfigForRepo(repo)
	require.NoError(t, err)
	assert.Equal(t, "env-key", cfg.APIKey)
	assert.Equal(t, "https://dotenv.example/graphql", cfg.Endpoint)

	t.Setenv("KASMOS_LINEAR_API_KEY", "")
	cfg, err = LinearConfigForRepo(repo)
	require.NoError(t, err)
	assert.Equal(t, "dotenv-key", cfg.APIKey)
}

func TestLinearConfigForRepoMissingKey(t *testing.T) {
	t.Setenv("KASMOS_LINEAR_API_KEY", "")
	t.Setenv("LINEAR_API_KEY", "")
	_, err := LinearConfigForRepo(t.TempDir())
	require.Error(t, err)
	assert.True(t, errors.Is(err, linear.ErrNotConfigured))
}

func writeRuntimeRepo(t *testing.T, content string) string {
	t.Helper()
	repo := t.TempDir()
	kasDir := filepath.Join(repo, ".kasmos")
	require.NoError(t, os.MkdirAll(kasDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(kasDir, config.TOMLConfigFileName), []byte(content), 0o644))
	return repo
}

func runtimeTriggerConfig() string {
	return `[linear.triggers]
enabled = true
verbs = ["plan"]

[linear.triggers.actor]
allowed_user_ids = ["user-1"]

[[linear.triggers.routes]]
team_id = "team-1"
topic = "eng"

[linear.triggers.webhook]
enabled = true
`
}
