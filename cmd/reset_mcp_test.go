package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRepoResetWriteMCP(t *testing.T) {
	t.Run("creates http entry pointing at shared endpoint", func(t *testing.T) {
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

		assert.Equal(t, "http", kasmos["type"], "transport must be http")
		assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])

		// Ensure no legacy stdio keys are present.
		assert.NotContains(t, kasmos, "command", "stdio command key must not be present")
		assert.NotContains(t, kasmos, "args", "stdio args key must not be present")
	})

	t.Run("does not write legacy stdio transport", func(t *testing.T) {
		dir := t.TempDir()

		err := defaultRepoResetWriteMCP(dir)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
		require.NoError(t, err)

		assert.NotContains(t, string(data), `"stdio"`, "must not write stdio transport")
		assert.Contains(t, string(data), "7434", "must reference shared HTTP port")
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
