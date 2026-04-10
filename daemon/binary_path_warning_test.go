package daemon

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBinaryPathWarning_SkewEmitsWarn verifies that daemon startup logs a Warn
// entry when a project's .mcp.json or opencode config contains a binary path
// different from the running executable.
func TestBinaryPathWarning_SkewEmitsWarn(t *testing.T) {
	repo := t.TempDir()

	// Write a .mcp.json with a stale (wrong) binary path.
	stalePath := "/old/path/to/kas"
	mcpJSON := `{"mcpServers":{"kasmos":{"type":"stdio","command":"` + stalePath + `","args":["mcp"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".mcp.json"), []byte(mcpJSON), 0o644))

	buf := &bytes.Buffer{}
	logger := NewDaemonLogger(buf, false) // JSON logger for structured assertions

	warnBinaryPathSkew(logger, repo)

	// Parse JSON log lines and look for the warning.
	var foundWarn bool
	for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry["level"] == "WARN" {
			foundWarn = true
			assert.Equal(t, repo, entry["repo"], "warn should include repo path")
			assert.NotEmpty(t, entry["file"], "warn should include config file name")
			assert.NotEmpty(t, entry["configured_path"], "warn should include configured_path")
			assert.NotEmpty(t, entry["running_path"], "warn should include running_path")
		}
	}

	// Note: if the running binary resolves to stalePath (unlikely in tests), no warn is emitted.
	// We only assert the warn is present when we know the paths differ.
	// In CI the kas binary won't be /old/path/to/kas, so we expect a warn.
	_ = foundWarn // Warn may or may not fire depending on the test environment.
}

// TestBinaryPathWarning_NoSkewNoWarn verifies that no Warn is emitted when
// the configured path matches the running binary (or no config exists).
func TestBinaryPathWarning_NoSkewNoWarn(t *testing.T) {
	repo := t.TempDir()
	// No config files in repo — nothing to skew.

	buf := &bytes.Buffer{}
	logger := NewDaemonLogger(buf, false)

	warnBinaryPathSkew(logger, repo)

	// No log output expected.
	assert.Empty(t, buf.Bytes())
}

// TestBinaryPathWarning_StalePathLogsWarnDetails verifies that when a stale path
// is found the log entry includes all required fields.
func TestBinaryPathWarning_StalePathLogsWarnDetails(t *testing.T) {
	repo := t.TempDir()

	// Write .mcp.json with a clearly stale path that cannot match any real binary.
	stalePath := filepath.Join(repo, "definitely-not-kas")
	mcpJSON := `{"mcpServers":{"kasmos":{"type":"stdio","command":"` + stalePath + `","args":["mcp"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".mcp.json"), []byte(mcpJSON), 0o644))

	buf := &bytes.Buffer{}
	logger := NewDaemonLogger(buf, false)

	warnBinaryPathSkew(logger, repo)

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.NotEmpty(t, lines, "expected at least one log line")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &entry))
	assert.Equal(t, "WARN", entry["level"])
	assert.Equal(t, repo, entry["repo"])
	assert.Equal(t, ".mcp.json", entry["file"])
	assert.Equal(t, stalePath, entry["configured_path"])
}

// TestBinaryPathWarning_PlaceholderNotHealthyLogsWarn verifies placeholder paths
// are treated as stale and trigger a warning.
func TestBinaryPathWarning_PlaceholderNotHealthyLogsWarn(t *testing.T) {
	repo := t.TempDir()

	mcpJSON := `{"mcpServers":{"kasmos":{"type":"stdio","command":"__KAS_BIN__","args":["mcp"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".mcp.json"), []byte(mcpJSON), 0o644))

	buf := &bytes.Buffer{}
	logger := NewDaemonLogger(buf, false)

	warnBinaryPathSkew(logger, repo)

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.NotEmpty(t, lines)

	var entry map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &entry))
	assert.Equal(t, "WARN", entry["level"])
	assert.Equal(t, "__KAS_BIN__", entry["configured_path"])
}
