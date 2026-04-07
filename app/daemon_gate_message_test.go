package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sentinelDaemonStartCmd is injected via the daemonStartCommand seam so tests
// verify the call site delegates correctly without depending on the host OS.
const sentinelDaemonStartCmd = "kas-daemon-start-sentinel"

// withSentinelDaemonStartCommand replaces daemonStartCommand for the duration
// of the test and restores it on cleanup.
func withSentinelDaemonStartCommand(t *testing.T) {
	t.Helper()
	old := daemonStartCommand
	daemonStartCommand = func() string { return sentinelDaemonStartCmd }
	t.Cleanup(func() { daemonStartCommand = old })
}

// TestCheckDaemonStatus_UsesPlatformStartCommand verifies that all three
// failure paths in checkDaemonStatus produce a message that contains the value
// returned by daemonStartCommand, not a hardcoded OS-specific string.
func TestCheckDaemonStatus_UsesPlatformStartCommand(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(repoPath, 0o755))

	withSentinelDaemonStartCommand(t)

	oldManaged := repoManagedByDaemon
	repoManagedByDaemon = func(string) bool { return false }
	t.Cleanup(func() { repoManagedByDaemon = oldManaged })

	t.Run("socket unavailable", func(t *testing.T) {
		// Point HOME/XDG_RUNTIME_DIR at a temp dir that has no socket so the
		// dial fails immediately.
		emptyDir := t.TempDir()
		t.Setenv("HOME", emptyDir)
		t.Setenv("XDG_RUNTIME_DIR", emptyDir)

		status := checkDaemonStatus(repoPath)

		assert.False(t, status.ready, "should not be ready when socket is unavailable")
		assert.Contains(t, status.message, sentinelDaemonStartCmd,
			"message must contain the platform start command")
		assert.Contains(t, status.message, "kas daemon add",
			"message must contain the registration hint")
	})

	t.Run("non-2xx daemon status response", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		startTestDaemonSocketServer(t, mux)

		status := checkDaemonStatus(repoPath)

		assert.False(t, status.ready, "should not be ready on non-2xx response")
		assert.Contains(t, status.message, sentinelDaemonStartCmd,
			"message must contain the platform start command")
		assert.True(t,
			strings.Contains(status.message, "status check failed") ||
				strings.Contains(status.message, "daemon"),
			"message should reference the daemon status check failure")
	})

	t.Run("unreadable daemon status JSON", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not-valid-json{{{"))
		})
		startTestDaemonSocketServer(t, mux)

		status := checkDaemonStatus(repoPath)

		assert.False(t, status.ready, "should not be ready when JSON is unreadable")
		assert.Contains(t, status.message, sentinelDaemonStartCmd,
			"message must contain the platform start command")
		assert.True(t,
			strings.Contains(status.message, "could not be read") ||
				strings.Contains(status.message, "daemon"),
			"message should reference the unreadable status response")
	})
}
