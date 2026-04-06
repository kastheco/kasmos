package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/daemon/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeCmd_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	// Verify the serve subcommand is registered
	cmd, _, err := rootCmd.Find([]string{"serve"})
	require.NoError(t, err)
	assert.Equal(t, "serve", cmd.Name())
}

func TestServeCmd_DefaultPort(t *testing.T) {
	cmd := NewServeCmd()
	assert.Contains(t, cmd.UseLine(), "serve")
	// Verify default flag values
	port, _ := cmd.Flags().GetInt("port")
	assert.Equal(t, 7433, port)
}

func TestServeCmd_MCPFlags(t *testing.T) {
	cmd := NewServeCmd()

	require.NotNil(t, cmd.Flags().Lookup("mcp"))
	assert.Equal(t, "true", cmd.Flags().Lookup("mcp").DefValue)

	require.NotNil(t, cmd.Flags().Lookup("mcp-port"))
	assert.Equal(t, "7434", cmd.Flags().Lookup("mcp-port").DefValue)

	port, err := cmd.Flags().GetInt("mcp-port")
	require.NoError(t, err)
	assert.Equal(t, 7434, port)
}

func TestServeCmd_MCPDisabled(t *testing.T) {
	cmd := NewServeCmd()

	err := cmd.Flags().Set("mcp", "false")
	require.NoError(t, err)

	val, err := cmd.Flags().GetBool("mcp")
	require.NoError(t, err)
	assert.False(t, val)
}

func TestServeCmd_RepoFlag(t *testing.T) {
	cmd := NewServeCmd()

	repoFlag := cmd.Flags().Lookup("repo")
	require.NotNil(t, repoFlag)
	assert.Equal(t, "stringSlice", repoFlag.Value.Type())

	err := cmd.Flags().Set("repo", "/tmp/one,/tmp/two")
	require.NoError(t, err)

	repos, err := cmd.Flags().GetStringSlice("repo")
	require.NoError(t, err)
	assert.Equal(t, []string{"/tmp/one", "/tmp/two"}, repos)
}

func TestServeCmd_RepoAndDBMutuallyExclusive(t *testing.T) {
	repoRoot := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "taskstore.db")

	cmd := NewServeCmd()
	cmd.SetArgs([]string{"--db", dbPath, "--repo", repoRoot})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestServeCmd_ProjectValidationMiddleware404(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := projectValidationMiddleware(map[string]struct{}{"known": {}}, next)

	t.Run("missing project returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/missing/tasks", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, `{"error":"project not found: missing"}`, rec.Body.String())
	})

	t.Run("known project reaches next handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/known/tasks", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("non project path passes through", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestResolveServeRepoPaths_AutoDetectsDaemonRepos(t *testing.T) {
	repoA := t.TempDir() // e.g. /tmp/TestXXX/alpha
	repoB := t.TempDir() // e.g. /tmp/TestXXX/bravo

	old := listDaemonRepoStatuses
	listDaemonRepoStatuses = func() ([]api.RepoStatus, error) {
		return []api.RepoStatus{
			{Path: repoA, Project: filepath.Base(repoA)},
			{Path: repoB, Project: filepath.Base(repoB)},
		}, nil
	}
	t.Cleanup(func() { listDaemonRepoStatuses = old })

	cmd := NewServeCmd()
	// Neither --db nor --repo set → auto-detect.
	got, err := resolveServeRepoPaths(cmd, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{repoA, repoB}, got)
}

func TestResolveServeRepoPaths_SkipsAutoDetectWhenDBFlagSet(t *testing.T) {
	old := listDaemonRepoStatuses
	listDaemonRepoStatuses = func() ([]api.RepoStatus, error) {
		t.Fatal("should not call daemon when --db is set")
		return nil, nil
	}
	t.Cleanup(func() { listDaemonRepoStatuses = old })

	cmd := NewServeCmd()
	require.NoError(t, cmd.Flags().Set("db", "/tmp/custom.db"))

	got, err := resolveServeRepoPaths(cmd, nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestResolveServeRepoPaths_FallsBackWhenDaemonUnavailable(t *testing.T) {
	old := listDaemonRepoStatuses
	listDaemonRepoStatuses = func() ([]api.RepoStatus, error) {
		return nil, fmt.Errorf("connection refused")
	}
	t.Cleanup(func() { listDaemonRepoStatuses = old })

	cmd := NewServeCmd()
	got, err := resolveServeRepoPaths(cmd, nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestResolveServeRepoPaths_ReturnsExplicitRepos(t *testing.T) {
	old := listDaemonRepoStatuses
	listDaemonRepoStatuses = func() ([]api.RepoStatus, error) {
		t.Fatal("should not call daemon when explicit repos provided")
		return nil, nil
	}
	t.Cleanup(func() { listDaemonRepoStatuses = old })

	cmd := NewServeCmd()
	explicit := []string{"/tmp/one", "/tmp/two"}
	got, err := resolveServeRepoPaths(cmd, explicit)
	require.NoError(t, err)
	assert.Equal(t, explicit, got)
}

func TestServeMCPServer_MultipleReposNoError(t *testing.T) {
	// After Task 2, newServeMCPServer delegates to newConfiguredMCPServer
	// which accepts multiple roots. Verify no error is returned for the
	// multi-root case (we pass nil store/gw since newConfiguredMCPServer
	// tolerates them for the purpose of MCP server construction).
	repoA := t.TempDir()
	repoB := t.TempDir()
	srv, err := newServeMCPServer(nil, nil, []string{repoA, repoB})
	require.NoError(t, err)
	assert.NotNil(t, srv)
}
