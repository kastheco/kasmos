package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskactions"
	"github.com/kastheco/kasmos/config/taskstore"
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

func TestServeOpenSQLiteBackends_SharedDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_serve.db")

	sharedDB, store, gw, logger, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	require.NotNil(t, sharedDB)
	require.NotNil(t, store)
	require.NotNil(t, gw)
	require.NotNil(t, logger)

	// Verify the underlying DB is reachable.
	require.NoError(t, sharedDB.Ping())

	// store/gw/logger Close() are no-ops; only sharedDB.Close() tears down the pool.
	require.NoError(t, store.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, logger.Close())
	require.NoError(t, sharedDB.Close())
}

func TestServeMCPServer_MultipleReposNoError(t *testing.T) {
	// newServeMCPServer delegates to newConfiguredMCPServer which accepts
	// multiple roots. Verify no error is returned for the multi-root case
	// (nil store/gw/sharedDB are tolerated for MCP server construction).
	repoA := t.TempDir()
	repoB := t.TempDir()
	srv, err := newServeMCPServer(nil, nil, nil, []string{repoA, repoB})
	require.NoError(t, err)
	assert.NotNil(t, srv)
}

func TestServeMCPServer_ZeroRepoDB_RoutesFromDB(t *testing.T) {
	// Parity test: newServeMCPServer with zero repos and a sharedDB must
	// share the same DB-backed routing behaviour as the kas mcp path.
	dbPath := filepath.Join(t.TempDir(), "serve_test.db")
	sharedDB, store, gw, _, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	require.NoError(t, store.Create("myproject", taskstore.TaskEntry{
		Filename:    "my-task",
		Status:      taskstore.StatusReady,
		Description: "serve routing test",
		Content:     "serve DB routing content",
	}))

	srv, err := newServeMCPServer(store, gw, sharedDB, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	// task_show without a project arg should succeed: DB has exactly one project.
	content, isError := mcpToolsCall(t, srv.Handler(), "task_show", map[string]any{
		"filename": "my-task",
	})
	assert.False(t, isError, "task_show should succeed with DB-derived project in serve path; got: %s", content)
	assert.Contains(t, content, "serve DB routing content")
}

func TestNewProjectListHandler_SortedDistinctProjects(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "projects_test.db")
	sharedDB, store, gw, _, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	// Seed tasks in multiple projects (including duplicates and a signal project).
	require.NoError(t, store.Create("zebra", taskstore.TaskEntry{Filename: "t1", Status: taskstore.StatusReady}))
	require.NoError(t, store.Create("alpha", taskstore.TaskEntry{Filename: "t2", Status: taskstore.StatusReady}))
	require.NoError(t, store.Create("zebra", taskstore.TaskEntry{Filename: "t3", Status: taskstore.StatusReady}))
	require.NoError(t, gw.Create("beta", taskstore.SignalEntry{PlanFile: "plan", SignalType: "planner_finished", Payload: "{}"}))

	h := newProjectListHandler(sharedDB, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `["alpha","beta","zebra"]`, rec.Body.String())
}

func TestNewProjectListHandler_EmptyDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty_projects.db")
	sharedDB, _, _, _, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	h := newProjectListHandler(sharedDB, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestNewProjectListHandler_NilDB(t *testing.T) {
	h := newProjectListHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"projects db unavailable"}`, rec.Body.String())
}

func TestNewProjectListHandler_RepoScopedExcludesStaleDBProjects(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "scoped.db")
	sharedDB, store, _, _, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	// DB has three projects, but only two are in the valid set.
	require.NoError(t, store.Create("active", taskstore.TaskEntry{Filename: "t1", Status: taskstore.StatusReady}))
	require.NoError(t, store.Create("stale", taskstore.TaskEntry{Filename: "t2", Status: taskstore.StatusReady}))
	require.NoError(t, store.Create("also-active", taskstore.TaskEntry{Filename: "t3", Status: taskstore.StatusReady}))

	valid := map[string]struct{}{"active": {}, "also-active": {}}
	h := newProjectListHandler(sharedDB, valid)

	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	// "stale" must NOT appear; result is sorted.
	assert.JSONEq(t, `["active","also-active"]`, rec.Body.String())
}

func TestNewProjectListHandler_RepoScopedIncludesRepoWithNoDBRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "no_rows.db")
	sharedDB, _, _, _, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	// DB is empty, but two repos are registered.
	valid := map[string]struct{}{"fresh-repo": {}, "another-repo": {}}
	h := newProjectListHandler(sharedDB, valid)

	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	// Both registered repos appear even though the DB has no rows for them.
	assert.JSONEq(t, `["another-repo","fresh-repo"]`, rec.Body.String())
}

// TestNewServeAPIRootMux_ContentRouteWinsOverTaskAPI proves that a
// PUT /content request is handled by the taskactions handler (which calls
// IngestContent and updates goal/subtasks) rather than by the generic
// taskstore handler (which would only update raw content bytes).
func TestNewServeAPIRootMux_ContentRouteWinsOverTaskAPI(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "content_route.db")
	sharedDB, store, _, logger, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	const project = "myproj"
	const filename = "my-task"

	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: filename,
		Status:   taskstore.StatusReady,
	}))

	taskAPI := taskstore.NewHandler(store)
	auditAPI := auditlog.NewHandler(logger)
	actionsAPI := taskactions.NewHandler(store)

	mux := newServeAPIRootMux(sharedDB, serveRepoRegistration{}, taskAPI, auditAPI, actionsAPI)

	// Markdown content with a clear goal heading so IngestContent populates Goal.
	const mdContent = "# Goal\n\nimplement the feature\n"

	req := httptest.NewRequest(http.MethodPut,
		"/v1/projects/"+project+"/tasks/"+filename+"/content",
		strings.NewReader(mdContent))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "response body: %s", rec.Body.String())

	// Verify the response JSON contains the goal extracted by taskparser — this
	// only happens when taskactions.handleContent (IngestContent path) ran,
	// not the raw taskstore SetContent endpoint.
	assert.Contains(t, rec.Body.String(), "implement the feature",
		"goal extracted by IngestContent must appear in response")
}

func TestNewServeAPIRootMux_GoalRouteWinsOverTaskAPI(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "goal_route.db")
	sharedDB, store, _, logger, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	const project = "myproj"
	const filename = "my-task"

	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: filename,
		Status:   taskstore.StatusReady,
	}))

	taskAPI := taskstore.NewHandler(store)
	auditAPI := auditlog.NewHandler(logger)
	actionsAPI := taskactions.NewHandler(store)

	mux := newServeAPIRootMux(sharedDB, serveRepoRegistration{}, taskAPI, auditAPI, actionsAPI)

	req := httptest.NewRequest(http.MethodPut,
		"/v1/projects/"+project+"/tasks/"+filename+"/goal",
		strings.NewReader(`{"goal":"ship it"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "response body: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "ship it",
		"goal route should be handled by taskactions and return the updated task entry")
}

// TestNewServeAPIRootMux_ActionRouteProjectValidation proves that in
// repo-scoped mode the projectValidationMiddleware rejects unknown projects
// for action routes (e.g. /available-actions) just as it does for taskAPI.
func TestNewServeAPIRootMux_ActionRouteProjectValidation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "action_validation.db")
	sharedDB, store, _, logger, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	validProjects := map[string]struct{}{"known-proj": {}}
	repoRegs := serveRepoRegistration{valid: validProjects}

	taskAPI := projectValidationMiddleware(repoRegs.valid, taskstore.NewHandler(store))
	auditAPI := projectValidationMiddleware(repoRegs.valid, auditlog.NewHandler(logger))
	actionsAPI := projectValidationMiddleware(repoRegs.valid, taskactions.NewHandler(store))

	mux := newServeAPIRootMux(sharedDB, repoRegs, taskAPI, auditAPI, actionsAPI)

	t.Run("unknown project returns 404 on available-actions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/v1/projects/unknown/tasks/some-task/available-actions", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, `{"error":"project not found: unknown"}`, rec.Body.String())
	})

	t.Run("unknown project returns 404 on transition", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost,
			"/v1/projects/unknown/tasks/some-task/transition",
			strings.NewReader(`{"event":"plan_start"}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, `{"error":"project not found: unknown"}`, rec.Body.String())
	})
}
