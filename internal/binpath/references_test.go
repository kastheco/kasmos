package binpath

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: write a file and ensure parent dirs exist.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// ── InspectProjectFiles ───────────────────────────────────────────────────────

func TestInspectProjectFiles_NoFiles(t *testing.T) {
	dir := t.TempDir()
	refs := InspectProjectFiles(dir)
	// No files → no references
	assert.Empty(t, refs)
}

func TestInspectProjectFiles_McpJSON_AbsPath(t *testing.T) {
	dir := t.TempDir()
	kasPath := filepath.Join(dir, "bin", "kas")
	writeFile(t, kasPath, "")
	mcpJSON := `{
  "mcpServers": {
    "kasmos": {
      "type": "stdio",
      "command": "` + kasPath + `",
      "args": ["mcp"]
    }
  }
}`
	writeFile(t, filepath.Join(dir, ".mcp.json"), mcpJSON)

	refs := InspectProjectFiles(dir)

	require.Len(t, refs, 1)
	assert.Equal(t, ".mcp.json", refs[0].File)
	assert.Equal(t, "mcpServers.kasmos", refs[0].Label)
	assert.Equal(t, kasPath, refs[0].RawPath)
	assert.NotEmpty(t, refs[0].Normalized)
	assert.Empty(t, refs[0].Note)
}

func TestInspectProjectFiles_McpJSON_BareCommand(t *testing.T) {
	dir := t.TempDir()
	mcpJSON := `{
  "mcpServers": {
    "kasmos": {
      "type": "stdio",
      "command": "kas",
      "args": ["mcp"]
    }
  }
}`
	writeFile(t, filepath.Join(dir, ".mcp.json"), mcpJSON)

	refs := InspectProjectFiles(dir)

	require.Len(t, refs, 1)
	assert.Equal(t, "kas", refs[0].RawPath)
	assert.Empty(t, refs[0].Normalized, "bare name should not normalize to abs path")
	assert.NotEmpty(t, refs[0].Note, "bare name should have a note")
}

func TestInspectProjectFiles_McpJSON_Placeholder(t *testing.T) {
	dir := t.TempDir()
	mcpJSON := `{
  "mcpServers": {
    "kasmos": {
      "type": "stdio",
      "command": "__KAS_BIN__",
      "args": ["mcp"]
    }
  }
}`
	writeFile(t, filepath.Join(dir, ".mcp.json"), mcpJSON)

	refs := InspectProjectFiles(dir)

	require.Len(t, refs, 1)
	assert.Equal(t, "__KAS_BIN__", refs[0].RawPath)
	assert.Empty(t, refs[0].Normalized)
	assert.Contains(t, refs[0].Note, "placeholder")
}

func TestInspectProjectFiles_McpJSON_NoKasmos(t *testing.T) {
	dir := t.TempDir()
	mcpJSON := `{
  "mcpServers": {
    "other-server": {
      "type": "stdio",
      "command": "/usr/bin/other",
      "args": []
    }
  }
}`
	writeFile(t, filepath.Join(dir, ".mcp.json"), mcpJSON)

	refs := InspectProjectFiles(dir)
	assert.Empty(t, refs, "no kasmos entry → no references")
}

func TestInspectProjectFiles_McpJSON_SharedHTTP(t *testing.T) {
	dir := t.TempDir()
	mcpJSON := `{
  "mcpServers": {
    "kasmos": {
      "type": "http",
      "url": "http://127.0.0.1:7434/mcp"
    }
  }
}`
	writeFile(t, filepath.Join(dir, ".mcp.json"), mcpJSON)

	refs := InspectProjectFiles(dir)

	require.Len(t, refs, 1)
	assert.Equal(t, TransportSharedHTTP, refs[0].Transport)
	assert.Equal(t, "http://127.0.0.1:7434/mcp", refs[0].RawPath)
	assert.Empty(t, refs[0].Note, "shared http entry should not carry a stale-transport note")
}

func TestInspectProjectFiles_Opencode_LocalStringCommand(t *testing.T) {
	dir := t.TempDir()
	kasPath := filepath.Join(dir, "bin", "kas")
	writeFile(t, kasPath, "")
	oc := `{
  "mcp": {
    "kasmos": {
      "type": "local",
      "command": "` + kasPath + `"
    }
  }
}`
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), oc)

	refs := InspectProjectFiles(dir)

	require.Len(t, refs, 1)
	assert.Equal(t, kasPath, refs[0].RawPath)
	assert.Equal(t, "mcp.kasmos", refs[0].Label)
}

func TestInspectProjectFiles_Opencode_LocalArrayCommand(t *testing.T) {
	dir := t.TempDir()
	kasPath := filepath.Join(dir, "bin", "kas")
	writeFile(t, kasPath, "")
	oc := `{
  "mcp": {
    "kasmos": {
      "type": "local",
      "command": ["` + kasPath + `", "mcp"]
    }
  }
}`
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), oc)

	refs := InspectProjectFiles(dir)

	require.Len(t, refs, 1)
	assert.Equal(t, kasPath, refs[0].RawPath)
}

func TestInspectProjectFiles_Opencode_RemoteType(t *testing.T) {
	dir := t.TempDir()
	oc := `{
  "mcp": {
    "kasmos": {
      "type": "remote",
      "url": "http://127.0.0.1:7434/mcp"
    }
  }
}`
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), oc)

	refs := InspectProjectFiles(dir)

	require.Len(t, refs, 1)
	assert.Equal(t, TransportSharedHTTP, refs[0].Transport)
	assert.Equal(t, "http://127.0.0.1:7434/mcp", refs[0].RawPath)
	assert.Empty(t, refs[0].Note, "shared http entry should not carry a stale-transport note")
}

func TestInspectProjectFiles_Opencode_JSONC_Comments(t *testing.T) {
	dir := t.TempDir()
	kasPath := filepath.Join(dir, "bin", "kas")
	writeFile(t, kasPath, "")
	oc := `{
  // kasmos MCP configuration
  "mcp": {
    "kasmos": {
      "type": "local",
      "command": "` + kasPath + `", // path to binary
    }
  }
}`
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), oc)

	refs := InspectProjectFiles(dir)

	require.Len(t, refs, 1)
	assert.Equal(t, kasPath, refs[0].RawPath)
}

func TestInspectProjectFiles_BothFiles(t *testing.T) {
	dir := t.TempDir()
	kasPath := filepath.Join(dir, "bin", "kas")
	writeFile(t, kasPath, "")

	mcpJSON := `{"mcpServers":{"kasmos":{"type":"stdio","command":"` + kasPath + `","args":["mcp"]}}}`
	writeFile(t, filepath.Join(dir, ".mcp.json"), mcpJSON)

	oc := `{"mcp":{"kasmos":{"type":"local","command":"` + kasPath + `"}}}`
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), oc)

	refs := InspectProjectFiles(dir)
	assert.Len(t, refs, 2, "should have one reference per config file")
}

// ── InspectServiceFiles ───────────────────────────────────────────────────────

func TestInspectServiceFiles_Linux_NotInstalled(t *testing.T) {
	home := t.TempDir()
	refs := InspectServiceFiles(home, "linux")

	// Missing files → references with "not installed" note
	for _, r := range refs {
		assert.Empty(t, r.Normalized, "missing file should have empty normalized path")
		assert.Contains(t, r.Note, "not installed")
	}
	// Should have entries for kasmos.service and kasmosdb.service
	assert.Len(t, refs, 3, "kasmos.service has ExecStart+ExecStop, kasmosdb.service has ExecStart")
}

func TestInspectServiceFiles_Linux_AbsPath(t *testing.T) {
	home := t.TempDir()
	kasPath := filepath.Join(home, "bin", "kas")
	writeFile(t, kasPath, "")

	svcDir := filepath.Join(home, ".config", "systemd", "user")
	require.NoError(t, os.MkdirAll(svcDir, 0o755))

	svc := "[Service]\nExecStart=" + kasPath + " daemon start --foreground\nExecStop=" + kasPath + " daemon stop\n"
	require.NoError(t, os.WriteFile(filepath.Join(svcDir, "kasmos.service"), []byte(svc), 0o644))

	refs := InspectServiceFiles(home, "linux")

	// kasmos.service: 2 refs; kasmosdb.service: 1 ref (not installed)
	assert.Len(t, refs, 3)

	// Find ExecStart ref for kasmos.service
	var startRef *Reference
	for i := range refs {
		if refs[i].File == "kasmos.service" && refs[i].Label == "ExecStart" {
			startRef = &refs[i]
			break
		}
	}
	require.NotNil(t, startRef)
	assert.Equal(t, kasPath, startRef.RawPath)
	assert.NotEmpty(t, startRef.Normalized)
	assert.Empty(t, startRef.Note)
}

func TestInspectServiceFiles_Linux_PercentHExpansion(t *testing.T) {
	home := t.TempDir()
	kasPath := filepath.Join(home, "bin", "kas")
	writeFile(t, kasPath, "")

	svcDir := filepath.Join(home, ".config", "systemd", "user")
	require.NoError(t, os.MkdirAll(svcDir, 0o755))

	// Use %h in ExecStart — should expand to home
	svc := "[Service]\nExecStart=%h/bin/kas daemon start --foreground\nExecStop=%h/bin/kas daemon stop\n"
	require.NoError(t, os.WriteFile(filepath.Join(svcDir, "kasmos.service"), []byte(svc), 0o644))

	refs := InspectServiceFiles(home, "linux")

	var startRef *Reference
	for i := range refs {
		if refs[i].File == "kasmos.service" && refs[i].Label == "ExecStart" {
			startRef = &refs[i]
			break
		}
	}
	require.NotNil(t, startRef)
	assert.Equal(t, "%h/bin/kas", startRef.RawPath)
	assert.Equal(t, kasPath, startRef.Normalized)
}

func TestInspectServiceFiles_Linux_Placeholder(t *testing.T) {
	home := t.TempDir()
	svcDir := filepath.Join(home, ".config", "systemd", "user")
	require.NoError(t, os.MkdirAll(svcDir, 0o755))

	svc := "[Service]\nExecStart=__KAS_BIN__ daemon start --foreground\nExecStop=__KAS_BIN__ daemon stop\n"
	require.NoError(t, os.WriteFile(filepath.Join(svcDir, "kasmos.service"), []byte(svc), 0o644))

	refs := InspectServiceFiles(home, "linux")

	for _, r := range refs {
		if r.File == "kasmos.service" {
			assert.Empty(t, r.Normalized)
			assert.Contains(t, r.Note, "placeholder")
		}
	}
}

func TestInspectServiceFiles_Darwin_NotInstalled(t *testing.T) {
	home := t.TempDir()
	refs := InspectServiceFiles(home, "darwin")

	// Both plists missing → not installed
	assert.Len(t, refs, 2)
	for _, r := range refs {
		assert.Contains(t, r.Note, "not installed")
	}
}

func TestInspectServiceFiles_Darwin_AbsPath(t *testing.T) {
	home := t.TempDir()
	kasPath := filepath.Join(home, "bin", "kas")
	writeFile(t, kasPath, "")

	laDir := filepath.Join(home, "Library", "LaunchAgents")
	require.NoError(t, os.MkdirAll(laDir, 0o755))

	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.kasmos.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>` + kasPath + `</string>
        <string>daemon</string>
        <string>start</string>
        <string>--foreground</string>
    </array>
</dict>
</plist>`
	require.NoError(t, os.WriteFile(filepath.Join(laDir, "com.kasmos.daemon.plist"), []byte(plist), 0o644))

	refs := InspectServiceFiles(home, "darwin")

	var daemonRef *Reference
	for i := range refs {
		if refs[i].File == "com.kasmos.daemon.plist" {
			daemonRef = &refs[i]
			break
		}
	}
	require.NotNil(t, daemonRef)
	assert.Equal(t, kasPath, daemonRef.RawPath)
	assert.NotEmpty(t, daemonRef.Normalized)
	assert.Empty(t, daemonRef.Note)
}

func TestInspectServiceFiles_UnknownOS(t *testing.T) {
	home := t.TempDir()
	refs := InspectServiceFiles(home, "windows")
	assert.Empty(t, refs, "unsupported OS returns no references")
}
