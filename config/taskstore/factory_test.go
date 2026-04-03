package taskstore

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestConfig(t *testing.T, repoDir, body string) {
	t.Helper()
	configDir := filepath.Join(repoDir, ".kasmos")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(body), 0o644))
}

// initTestRepo creates a bare .git directory so that config.GetConfigDir()
// anchors to repoDir on CI where temp dirs have no git root.
func initTestRepo(t *testing.T, repoDir string) {
	t.Helper()
	out, err := exec.Command("git", "init", repoDir).CombinedOutput()
	if err != nil {
		t.Skipf("git init failed (%v): %s", err, out)
	}
}

func startTestDaemonSocketServer(t *testing.T, handler http.Handler) string {
	t.Helper()

	// Set HOME and XDG_RUNTIME_DIR to a short temp dir so ResolvedDaemonSocketPath
	// never reads a real daemon.toml and test socket paths stay under the 108-byte
	// Unix domain socket limit on Linux.
	homeDir, err := os.MkdirTemp("", "ks-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(homeDir) })
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_RUNTIME_DIR", homeDir)

	socketPath := ResolvedDaemonSocketPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(socketPath), 0o755))
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	server := &http.Server{Handler: handler}

	go func() {
		_ = server.Serve(listener)
	}()

	t.Cleanup(func() {
		require.NoError(t, server.Close())
		_ = os.Remove(socketPath)
	})

	return socketPath
}

func newTestDaemonTaskStoreMux(t *testing.T, repos []string, taskHandler http.Handler) http.Handler {
	t.Helper()

	registered := make([]daemonRepoStatus, 0, len(repos))
	for _, project := range repos {
		registered = append(registered, daemonRepoStatus{Project: project})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(registered))
	})
	mux.Handle("/v1/ping", taskHandler)
	mux.Handle("/v1/projects/", taskHandler)
	return mux
}

func newTestDaemonMux(t *testing.T, repos []string, taskHandler, signalHandler http.Handler) http.Handler {
	t.Helper()

	registered := make([]daemonRepoStatus, 0, len(repos))
	for _, project := range repos {
		registered = append(registered, daemonRepoStatus{Project: project})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(registered))
	})
	mux.HandleFunc("GET /v1/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	if taskHandler != nil {
		mux.Handle("/v1/projects/", taskHandler)
	}
	if signalHandler != nil {
		mux.Handle("/v1/projects/", signalHandler)
	}
	return mux
}

func TestNewStoreFromConfig_HTTP(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(NewHandler(backend))
	defer srv.Close()

	store, err := NewStoreFromConfig(srv.URL, "test-project")
	require.NoError(t, err)
	require.NoError(t, store.Ping())
}

func TestNewStoreFromConfig_Empty(t *testing.T) {
	store, err := NewStoreFromConfig("", "test-project")
	require.NoError(t, err)
	// Returns nil store — caller should fall back to legacy behavior
	assert.Nil(t, store)
}

func TestNewStoreFromConfig_Unreachable(t *testing.T) {
	store, err := NewStoreFromConfig("http://127.0.0.1:1", "test-project")
	// Factory succeeds (lazy connect) but Ping fails
	require.NoError(t, err)
	require.Error(t, store.Ping())
}

func TestOpenAuthoritativeStore_UsesConfiguredHTTPAuthority(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(NewHandler(backend))
	defer srv.Close()

	repoDir := t.TempDir()
	initTestRepo(t, repoDir)
	writeTestConfig(t, repoDir, fmt.Sprintf("database_url = %q\n", srv.URL))
	t.Chdir(repoDir)

	store, err := OpenAuthoritativeStore("test-project")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	require.NoError(t, store.Create("test-project", TaskEntry{Filename: "authoritative", Status: StatusReady}))
	entry, err := backend.Get("test-project", "authoritative")
	require.NoError(t, err)
	assert.Equal(t, StatusReady, entry.Status)
}

func TestOpenAuthoritativeStore_UnreachableRemoteFails(t *testing.T) {
	repoDir := t.TempDir()
	initTestRepo(t, repoDir)
	writeTestConfig(t, repoDir, "database_url = \"http://127.0.0.1:1\"\n")
	t.Chdir(repoDir)

	store, err := OpenAuthoritativeStore("test-project")
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "task store unreachable")
}

func TestOpenAuthoritativeStore_UsesGlobalSQLiteWhenDatabaseURLUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repoDir := t.TempDir()
	initTestRepo(t, repoDir)
	t.Chdir(repoDir)

	store, err := OpenAuthoritativeStore("test-project")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.IsType(t, &SQLiteStore{}, store)

	require.NoError(t, store.Create("test-project", TaskEntry{Filename: "authoritative-local", Status: StatusReady}))

	// Verify data landed in the global DB, not a repo-local one.
	globalDBPath := filepath.Join(home, ".config", "kasmos", "taskstore.db")
	assert.Equal(t, globalDBPath, ResolvedDBPath())

	backingStore, err := NewSQLiteStore(globalDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = backingStore.Close() })

	entry, err := backingStore.Get("test-project", "authoritative-local")
	require.NoError(t, err)
	assert.Equal(t, StatusReady, entry.Status)
}

func TestOpenAuthoritativeSignalGateway_UnreachableRemoteFails(t *testing.T) {
	repoDir := t.TempDir()
	initTestRepo(t, repoDir)
	writeTestConfig(t, repoDir, "database_url = \"http://127.0.0.1:1\"\n")
	t.Chdir(repoDir)

	gw, err := OpenAuthoritativeSignalGateway("test-project")
	require.Error(t, err)
	assert.Nil(t, gw)
	assert.Contains(t, err.Error(), "task store unreachable")
}

func TestOpenAuthoritativeSignalGateway_ReachableRemoteFailsWithoutSignalAccess(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(NewHandler(backend))
	defer srv.Close()

	repoDir := t.TempDir()
	initTestRepo(t, repoDir)
	writeTestConfig(t, repoDir, fmt.Sprintf("database_url = %q\n", srv.URL))
	t.Chdir(repoDir)

	gw, err := OpenAuthoritativeSignalGateway("test-project")
	require.Error(t, err)
	assert.Nil(t, gw)
	assert.Contains(t, err.Error(), "does not expose signal gateway access")
}

func TestOpenAuthoritativeSignalGateway_UsesGlobalSQLiteWhenDatabaseURLUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repoDir := t.TempDir()
	initTestRepo(t, repoDir)
	t.Chdir(repoDir)

	gw, err := OpenAuthoritativeSignalGateway("test-project")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })
	require.IsType(t, &SQLiteSignalGateway{}, gw)

	require.NoError(t, gw.Create("test-project", SignalEntry{PlanFile: "feature", SignalType: "planner_finished", Payload: "{}"}))

	// Verify data landed in the global DB, not a repo-local one.
	globalDBPath := filepath.Join(home, ".config", "kasmos", "taskstore.db")
	backingGateway, err := NewSQLiteSignalGateway(globalDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = backingGateway.Close() })

	signals, err := backingGateway.List("test-project", SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "feature", signals[0].PlanFile)
	assert.Equal(t, "planner_finished", signals[0].SignalType)
}

func TestResolvedDaemonSocketPath_UsesDaemonTomlOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	configDir := filepath.Join(home, ".config", "kasmos")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	override := filepath.Join(t.TempDir(), "custom.sock")
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "daemon.toml"), []byte("socket_path = \""+override+"\"\n"), 0o644))

	assert.Equal(t, override, ResolvedDaemonSocketPath())
}

func TestOpenDaemonBackedSignalGateway_UsesDaemonWhenProjectRegistered(t *testing.T) {
	project := "test-project"
	dbPath := filepath.Join(t.TempDir(), "signals.db")
	backend, err := NewSQLiteSignalGateway(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = backend.Close() })

	repoDir := t.TempDir()
	initTestRepo(t, repoDir)
	startTestDaemonSocketServer(t, newTestDaemonMux(t, []string{project}, nil, NewSignalHandler(backend)))
	t.Chdir(repoDir)

	gw, err := OpenDaemonBackedSignalGateway(project)
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	require.NoError(t, gw.Create(project, SignalEntry{PlanFile: "feature", SignalType: "planner_finished"}))
	signals, err := backend.List(project, SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "planner_finished", signals[0].SignalType)
	assert.Equal(t, "feature", signals[0].PlanFile)

	claimed, err := gw.Claim(project, "worker-1")
	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, signals[0].PlanFile, claimed.PlanFile)
	require.NoError(t, gw.MarkProcessed(claimed.ID, SignalDone, "ok"))

	processed, err := backend.List(project, SignalDone)
	require.NoError(t, err)
	require.Len(t, processed, 1)
	assert.Equal(t, claimed.ID, processed[0].ID)
	assert.Equal(t, "ok", processed[0].Result)
}

func TestOpenDaemonBackedSignalGateway_UnregisteredProjectFails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "signals.db")
	backend, err := NewSQLiteSignalGateway(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = backend.Close() })

	repoDir := t.TempDir()
	initTestRepo(t, repoDir)
	startTestDaemonSocketServer(t, newTestDaemonMux(t, []string{"other-project"}, nil, NewSignalHandler(backend)))
	t.Chdir(repoDir)

	gw, err := OpenDaemonBackedSignalGateway("test-project")
	require.Error(t, err)
	assert.Nil(t, gw)
	assert.Contains(t, err.Error(), "not registered")
}

func TestResolvedDBPath(t *testing.T) {
	t.Run("returns global taskstore.db under HOME/.config/kasmos", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		dbPath := ResolvedDBPath()

		assert.Equal(t, filepath.Join(homeDir, ".config", "kasmos", "taskstore.db"), dbPath)
	})

	t.Run("same global path regardless of working directory or git repo", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		// change into an unrelated temp dir — path must not change
		projectDir := t.TempDir()
		t.Chdir(projectDir)

		dbPath := ResolvedDBPath()
		assert.Equal(t, filepath.Join(homeDir, ".config", "kasmos", "taskstore.db"), dbPath)
	})
}

func TestGlobalDBPath(t *testing.T) {
	t.Run("returns path under HOME/.config/kasmos", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		dbPath := GlobalDBPath()

		assert.Equal(t, filepath.Join(homeDir, ".config", "kasmos", "taskstore.db"), dbPath)
	})
}
