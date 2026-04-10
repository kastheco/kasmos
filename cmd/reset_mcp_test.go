package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/internal/binpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRepoResetWriteMCP(t *testing.T) {
	t.Run("creates stdio entry pointing at resolved kas path", func(t *testing.T) {
		dir := t.TempDir()

		err := defaultRepoResetWriteMCP(dir)
		require.NoError(t, err)

		dest := filepath.Join(dir, ".mcp.json")
		assert.FileExists(t, dest)

		data, err := os.ReadFile(dest)
		require.NoError(t, err)

		var cfg map[string]any
		require.NoError(t, json.Unmarshal(data, &cfg))

		servers, ok := cfg["mcpServers"].(map[string]any)
		require.True(t, ok, "mcpServers key must be present")

		kasmos, ok := servers["kasmos"].(map[string]any)
		require.True(t, ok, "kasmos entry must be present")

		assert.Equal(t, "stdio", kasmos["type"], "transport must be stdio, not http")
		assert.Equal(t, binpath.ResolveOrFallback().Executable, kasmos["command"])

		args, ok := kasmos["args"].([]any)
		require.True(t, ok, "args must be an array")
		assert.Equal(t, []any{"mcp"}, args)

		// Ensure no legacy http url key is present.
		assert.NotContains(t, kasmos, "url", "legacy http url key must not be present")
	})

	t.Run("does not write legacy http transport", func(t *testing.T) {
		dir := t.TempDir()

		err := defaultRepoResetWriteMCP(dir)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
		require.NoError(t, err)

		assert.NotContains(t, string(data), `"http"`, "must not write http transport")
		assert.NotContains(t, string(data), "7434", "must not reference legacy HTTP port")
	})

	t.Run("preserves unrelated mcp servers", func(t *testing.T) {
		dir := t.TempDir()
		existing := `{"mcpServers":{"other-tool":{"type":"stdio","command":"other-tool"}}}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(existing), 0o644))

		err := defaultRepoResetWriteMCP(dir)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
		require.NoError(t, err)

		var cfg map[string]any
		require.NoError(t, json.Unmarshal(data, &cfg))

		servers := cfg["mcpServers"].(map[string]any)
		assert.Contains(t, servers, "kasmos", "kasmos entry must be added")
		assert.Contains(t, servers, "other-tool", "unrelated server must be preserved")
	})
}
