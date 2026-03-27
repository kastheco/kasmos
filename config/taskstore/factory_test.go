package taskstore

import (
	"fmt"
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
	writeTestConfig(t, repoDir, "database_url = \"http://127.0.0.1:1\"\n")
	t.Chdir(repoDir)

	store, err := OpenAuthoritativeStore("test-project")
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "task store unreachable")
}

func TestOpenAuthoritativeSignalGateway_UnreachableRemoteFails(t *testing.T) {
	repoDir := t.TempDir()
	writeTestConfig(t, repoDir, "database_url = \"http://127.0.0.1:1\"\n")
	t.Chdir(repoDir)

	gw, err := OpenAuthoritativeSignalGateway("test-project")
	require.Error(t, err)
	assert.Nil(t, gw)
	assert.Contains(t, err.Error(), "task store unreachable")
}

func TestResolvedDBPath(t *testing.T) {
	runGit := func(t *testing.T, repo string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	}

	t.Run("returns taskstore.db under .kasmos in working directory", func(t *testing.T) {
		projectDir := t.TempDir()
		t.Chdir(projectDir)

		dbPath := ResolvedDBPath()

		assert.Equal(t, filepath.Join(projectDir, ".kasmos", "taskstore.db"), dbPath)
	})

	t.Run("returns taskstore.db under main repo root from worktree", func(t *testing.T) {
		repoDir := t.TempDir()
		t.Setenv("HOME", t.TempDir())

		runGit(t, repoDir, "init", "-b", "main")
		runGit(t, repoDir, "config", "user.email", "test@example.com")
		runGit(t, repoDir, "config", "user.name", "test")
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("init\n"), 0o644))
		runGit(t, repoDir, "add", ".")
		runGit(t, repoDir, "commit", "-m", "initial")

		runGit(t, repoDir, "branch", "plan/worktree-db")
		worktreeParent := t.TempDir()
		worktreeDir := filepath.Join(worktreeParent, "worktree-db")
		runGit(t, repoDir, "worktree", "add", worktreeDir, "plan/worktree-db")
		t.Chdir(worktreeDir)

		dbPath := ResolvedDBPath()
		assert.Equal(t, filepath.Join(repoDir, ".kasmos", "taskstore.db"), dbPath)
	})
}
