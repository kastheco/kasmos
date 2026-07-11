package cmd

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/internal/livestatus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startMonitorTestDaemon(t *testing.T, handler http.Handler) string {
	t.Helper()
	socket := t.TempDir() + "/daemon.sock"
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	return socket
}

func TestMonitorWidgetWritesPreview(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "preview.html")
	cmd := NewMonitorCmd()
	cmd.SetArgs([]string{"widget", "--out", outPath})
	require.NoError(t, cmd.Execute())
	content, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "kasmos-monitor-root")
	assert.Contains(t, string(content), "callTool:async function")
}

func TestMonitorCmd_HasSubcommands(t *testing.T) {
	cmd := NewMonitorCmd()
	subcommands := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subcommands[sub.Name()] = true
	}
	assert.True(t, subcommands["status"], "missing 'status' subcommand")
}

func TestMonitorCmd_DefaultIsTail(t *testing.T) {
	cmd := NewMonitorCmd()
	assert.NotNil(t, cmd.RunE, "default monitor command should have RunE for live tail")
}

func TestPrintMonitorEvent_KindFilter(t *testing.T) {
	var out bytes.Buffer

	err := printMonitorEvent(&out, `{"kind":"wave_started","message":"wave"}`, "", "", []string{"wave_failed"}, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, out.String())

	err = printMonitorEvent(&out, `{"kind":"wave_failed","message":"failed"}`, "", "", []string{"wave_failed"}, time.Time{})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "[wave_failed]")
}

func TestPrintMonitorEvent_DetailBranch(t *testing.T) {
	var out bytes.Buffer
	payload := `{"kind":"wave_failed","message":"wave 3 failed","detail":"{\"retry_generation\":1}"}`

	err := printMonitorEvent(&out, payload, "", "", nil, time.Time{})
	require.NoError(t, err)

	assert.Equal(t, "[wave_failed] wave 3 failed  detail={\"retry_generation\":1}\n", out.String())
}

func TestParseMonitorTimestampAcceptsFractionalSeconds(t *testing.T) {
	t.Parallel()

	parsed, err := parseMonitorTimestamp("2026-04-24T12:00:00.123456789Z")
	require.NoError(t, err)
	assert.Equal(t, 123456789, parsed.Nanosecond())
}

func TestMonitorEventMapMatchesSinceAcceptsFractionalTimestamp(t *testing.T) {
	t.Parallel()

	since, err := parseMonitorTimestamp("2026-04-24T12:00:00.123456Z")
	require.NoError(t, err)

	event := map[string]interface{}{
		"kind":      "wave_failed",
		"timestamp": "2026-04-24T12:00:00.123456789Z",
	}

	assert.True(t, monitorEventMapMatches(event, "", "", nil, since))
}

func TestMonitorStatusJSON(t *testing.T) {
	want := livestatus.LiveStatus{SchemaVersion: livestatus.SchemaVersion, Project: "kasmos", DaemonRunning: true, ActiveAgents: []livestatus.ActiveAgent{}, Attention: []livestatus.AttentionItem{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/repos", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"project":"kasmos"}]`))
	})
	mux.HandleFunc("GET /v1/repos/kasmos/live-status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(want)
	})

	cmd := NewMonitorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--socket", startMonitorTestDaemon(t, mux), "status", "--json"})
	require.NoError(t, cmd.Execute())
	var got livestatus.LiveStatus
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, want, got)
}

func TestMonitorStatusTextBackwardCompatibility(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"repos":["kasmos"]}`))
	})

	cmd := NewMonitorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--socket", startMonitorTestDaemon(t, mux), "status"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "daemon status snapshot:\n")
	assert.Contains(t, out.String(), "  repos:\n")
}
