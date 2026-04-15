package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskactions"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/livepreview"
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

func TestNewDynamicProjectRootResolver_ReturnsRootForKnownProject(t *testing.T) {
	dir := t.TempDir() // e.g. /tmp/TestXXX/myrepo

	old := listDaemonRepoStatuses
	listDaemonRepoStatuses = func() ([]api.RepoStatus, error) {
		return []api.RepoStatus{{Path: dir, Project: filepath.Base(dir)}}, nil
	}
	t.Cleanup(func() { listDaemonRepoStatuses = old })

	resolve := newDynamicProjectRootResolver()
	got, err := resolve(filepath.Base(dir))
	require.NoError(t, err)
	assert.Equal(t, dir, got)
}

func TestNewDynamicProjectRootResolver_ReturnsNotFoundForUnknownProject(t *testing.T) {
	dir := t.TempDir()

	old := listDaemonRepoStatuses
	listDaemonRepoStatuses = func() ([]api.RepoStatus, error) {
		return []api.RepoStatus{{Path: dir, Project: filepath.Base(dir)}}, nil
	}
	t.Cleanup(func() { listDaemonRepoStatuses = old })

	resolve := newDynamicProjectRootResolver()
	_, err := resolve("no-such-project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project not found: no-such-project")
}

func TestNewDynamicProjectRootResolver_ReturnsErrPreviewUnavailableWhenDaemonDown(t *testing.T) {
	old := listDaemonRepoStatuses
	listDaemonRepoStatuses = func() ([]api.RepoStatus, error) {
		return nil, fmt.Errorf("connection refused")
	}
	t.Cleanup(func() { listDaemonRepoStatuses = old })

	resolve := newDynamicProjectRootResolver()
	_, err := resolve("anyproject")
	require.ErrorIs(t, err, livepreview.ErrPreviewUnavailable)
}

func TestNewDynamicProjectRootResolver_QueriesDaemonOnEachCall(t *testing.T) {
	dir := t.TempDir()
	callCount := 0

	old := listDaemonRepoStatuses
	listDaemonRepoStatuses = func() ([]api.RepoStatus, error) {
		callCount++
		return []api.RepoStatus{{Path: dir, Project: filepath.Base(dir)}}, nil
	}
	t.Cleanup(func() { listDaemonRepoStatuses = old })

	resolve := newDynamicProjectRootResolver()
	_, _ = resolve(filepath.Base(dir))
	_, _ = resolve(filepath.Base(dir))
	assert.Equal(t, 2, callCount, "resolver must query daemon on every call, not cache")
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
	sharedDB, store, gw, logger, err := openServeSQLiteBackends(dbPath)
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
	actionsAPI := taskactions.NewHandler(store, gw)

	mux := newServeAPIRootMux(sharedDB, serveRepoRegistration{}, taskAPI, auditAPI, actionsAPI, http.NotFoundHandler())

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
	sharedDB, store, gw, logger, err := openServeSQLiteBackends(dbPath)
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
	actionsAPI := taskactions.NewHandler(store, gw)

	mux := newServeAPIRootMux(sharedDB, serveRepoRegistration{}, taskAPI, auditAPI, actionsAPI, http.NotFoundHandler())

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
	sharedDB, store, gw, logger, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	validProjects := map[string]struct{}{"known-proj": {}}
	repoRegs := serveRepoRegistration{valid: validProjects}

	taskAPI := projectValidationMiddleware(repoRegs.valid, taskstore.NewHandler(store))
	auditAPI := projectValidationMiddleware(repoRegs.valid, auditlog.NewHandler(logger))
	actionsAPI := projectValidationMiddleware(repoRegs.valid, taskactions.NewHandler(store, gw))

	mux := newServeAPIRootMux(sharedDB, repoRegs, taskAPI, auditAPI, actionsAPI, http.NotFoundHandler())

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

// TestNewServeAPIRootMux_TransitionEmitsGatewaySignal is a wiring regression
// that proves the shared gateway instance is threaded through the serve stack:
// a planner_finished transition via the root mux must create a pending signal.
func TestNewServeAPIRootMux_TransitionEmitsGatewaySignal(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "transition_emit.db")
	sharedDB, store, gw, logger, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	const project = "myproj"
	const filename = "plan-task"

	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: filename,
		Status:   taskstore.StatusReady,
	}))
	require.NoError(t, store.Update(project, filename, taskstore.TaskEntry{
		Filename: filename,
		Status:   taskstore.StatusPlanning,
	}))

	// Minimal valid plan content: goal + wave + task headings.
	const validPlan = "**Goal:** build something great\n\n## Wave 1\n\n### Task 1: do the thing\n\nimplement the thing\n"
	require.NoError(t, store.SetContent(project, filename, validPlan))

	taskAPI := taskstore.NewHandler(store)
	auditAPI := auditlog.NewHandler(logger)
	actionsAPI := taskactions.NewHandler(store, gw)

	mux := newServeAPIRootMux(sharedDB, serveRepoRegistration{}, taskAPI, auditAPI, actionsAPI, http.NotFoundHandler())

	req := httptest.NewRequest(http.MethodPost,
		"/v1/projects/"+project+"/tasks/"+filename+"/transition",
		strings.NewReader(`{"event":"planner_finished"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "response body: %s", rec.Body.String())

	signals, err := gw.List(project, taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1, "expected exactly one pending signal after planner_finished")
	assert.Equal(t, filename, signals[0].PlanFile)
	assert.Equal(t, "planner_finished", signals[0].SignalType)
}

// ---------------------------------------------------------------------------
// Preview route registration tests
// ---------------------------------------------------------------------------

// testServePreviewMux is a helper that builds a root mux with the given
// previewAPI wired in, using an in-memory DB for the required taskstore.
func testServePreviewMux(t *testing.T, previewAPI http.Handler) *http.ServeMux {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "preview_test.db")
	sharedDB, store, gw, logger, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	taskAPI := taskstore.NewHandler(store)
	auditAPI := auditlog.NewHandler(logger)
	actionsAPI := taskactions.NewHandler(store, gw)
	return newServeAPIRootMux(sharedDB, serveRepoRegistration{}, taskAPI, auditAPI, actionsAPI, previewAPI)
}

// TestNewServeAPIRootMux_PreviewInstancesRouteRegistered verifies that
// GET /v1/projects/{project}/instances is routed to previewAPI.
func TestNewServeAPIRootMux_PreviewInstancesRouteRegistered(t *testing.T) {
	called := false
	previewAPI := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mux := testServePreviewMux(t, previewAPI)

	req := httptest.NewRequest(http.MethodGet, "/v1/projects/myproj/instances", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.True(t, called, "previewAPI must be called for GET /instances")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestNewServeAPIRootMux_PreviewCaptureRouteRegistered verifies that
// GET /v1/projects/{project}/instances/{title}/capture is routed to previewAPI.
func TestNewServeAPIRootMux_PreviewCaptureRouteRegistered(t *testing.T) {
	called := false
	previewAPI := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mux := testServePreviewMux(t, previewAPI)

	req := httptest.NewRequest(http.MethodGet, "/v1/projects/myproj/instances/agent1/capture", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.True(t, called, "previewAPI must be called for GET /instances/{title}/capture")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestNewServeAPIRootMux_PreviewProjectValidation404 verifies that in
// repo-scoped mode unknown projects return 404 for the instances list route.
func TestNewServeAPIRootMux_PreviewProjectValidation404(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "preview_validation.db")
	sharedDB, store, gw, logger, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	repoRoot := t.TempDir()
	validProjects := map[string]struct{}{"known-proj": {}}
	repoRegs := serveRepoRegistration{
		valid:          validProjects,
		rootsByProject: map[string]string{"known-proj": repoRoot},
	}

	resolve := func(project string) (string, error) {
		root, ok := repoRegs.rootsByProject[project]
		if !ok {
			return "", fmt.Errorf("project not found: %s", project)
		}
		return root, nil
	}
	previewHandler := livepreview.NewHTTPHandler(resolve, &livepreview.ExecPaneRunner{})
	previewAPI := projectValidationMiddleware(repoRegs.valid, previewHandler)

	taskAPI := projectValidationMiddleware(repoRegs.valid, taskstore.NewHandler(store))
	auditAPI := projectValidationMiddleware(repoRegs.valid, auditlog.NewHandler(logger))
	actionsAPI := projectValidationMiddleware(repoRegs.valid, taskactions.NewHandler(store, gw))

	mux := newServeAPIRootMux(sharedDB, repoRegs, taskAPI, auditAPI, actionsAPI, previewAPI)

	t.Run("unknown project returns 404 on instances list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/unknown-proj/instances", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, `{"error":"project not found: unknown-proj"}`, rec.Body.String())
	})

	t.Run("known project reaches preview handler", func(t *testing.T) {
		// known-proj has no state file → returns empty JSON array.
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/known-proj/instances", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, "[]", rec.Body.String())
	})
}

// TestNewServeAPIRootMux_CaptureContentTypePlain verifies that the capture
// endpoint returns Content-Type: text/plain; charset=utf-8.
func TestNewServeAPIRootMux_CaptureContentTypePlain(t *testing.T) {
	repoRoot := t.TempDir()

	// Write a running instance to the state file.
	kasDir := filepath.Join(repoRoot, ".kasmos")
	require.NoError(t, os.MkdirAll(kasDir, 0o755))
	type stateFile struct {
		Instances json.RawMessage `json:"instances"`
	}
	type instanceRec struct {
		Title   string `json:"title"`
		Status  int    `json:"status"`
		Program string `json:"program"`
		Branch  string `json:"branch"`
	}
	raw, err := json.Marshal([]instanceRec{{Title: "agent1", Status: 0, Program: "claude", Branch: "main"}})
	require.NoError(t, err)
	data, err := json.Marshal(stateFile{Instances: json.RawMessage(raw)})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(kasDir, "state.json"), data, 0o644))

	// Build a preview handler with a mock runner that returns pane output.
	mockRunner := &captureTestRunner{output: "hello from tmux\n"}
	resolve := func(string) (string, error) { return repoRoot, nil }
	previewAPI := livepreview.NewHTTPHandler(resolve, mockRunner)

	mux := testServePreviewMux(t, previewAPI)

	req := httptest.NewRequest(http.MethodGet, "/v1/projects/myproj/instances/agent1/capture", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "hello from tmux\n", rec.Body.String())
}

// captureTestRunner is a minimal livepreview.PaneRunner for use in cmd tests.
type captureTestRunner struct {
	output string
}

func (r *captureTestRunner) Output(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return []byte(r.output), nil
}
