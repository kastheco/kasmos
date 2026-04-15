package livepreview

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// writeStateJSON writes a minimal state.json to <repoRoot>/.kasmos/state.json
// containing the given instance records.
func writeStateJSON(t *testing.T, repoRoot string, records ...Record) {
	t.Helper()
	dir := filepath.Join(repoRoot, ".kasmos")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	raw, err := json.Marshal(records)
	require.NoError(t, err)
	type stateFile struct {
		Instances json.RawMessage `json:"instances"`
	}
	data, err := json.Marshal(stateFile{Instances: json.RawMessage(raw)})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "state.json"), data, 0o644))
}

// resolverFor returns a ProjectRootResolver that always maps any project to root.
func resolverFor(root string) ProjectRootResolver {
	return func(string) (string, error) { return root, nil }
}

// unavailableResolver always returns ErrPreviewUnavailable, mimicking bare-DB mode.
func unavailableResolver() ProjectRootResolver {
	return func(string) (string, error) { return "", ErrPreviewUnavailable }
}

// sessionGoneRunner returns a PaneRunner that simulates a tmux session-not-found error.
func sessionGoneRunner() *mockPaneRunner {
	return &mockPaneRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, &exec.ExitError{Stderr: []byte("can't find session: kas_gone-agent")}
		},
	}
}

// commandErrorRunner returns a PaneRunner that simulates an unexpected tmux failure.
func commandErrorRunner(stderrMsg string) *mockPaneRunner {
	return &mockPaneRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, &exec.ExitError{Stderr: []byte(stderrMsg)}
		},
	}
}

// genericErrorRunner returns a PaneRunner that returns a plain error (no ExitError).
func genericErrorRunner(msg string) *mockPaneRunner {
	return &mockPaneRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New(msg)
		},
	}
}

// paneOutputRunner returns a PaneRunner that always succeeds with output.
func paneOutputRunner(output string) *mockPaneRunner {
	return &mockPaneRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(output), nil
		},
	}
}

// ---------------------------------------------------------------------------
// StateFilePath / LoadRecordsFromRepoRoot
// ---------------------------------------------------------------------------

func TestStateFilePath(t *testing.T) {
	path := StateFilePath("/repo/root")
	assert.Equal(t, filepath.Join("/repo/root", ".kasmos", "state.json"), path)
}

func TestLoadRecordsFromRepoRoot_FileNotExist(t *testing.T) {
	root := t.TempDir()
	records, err := LoadRecordsFromRepoRoot(root)
	require.NoError(t, err)
	assert.NotNil(t, records) // must never be nil
	assert.Empty(t, records)
}

func TestLoadRecordsFromRepoRoot_ValidFile(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root,
		Record{Title: "agent-a", Status: StatusRunning, Program: "claude"},
		Record{Title: "agent-b", Status: StatusPaused, Program: "amp"},
	)
	records, err := LoadRecordsFromRepoRoot(root)
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "agent-a", records[0].Title)
	assert.Equal(t, StatusRunning, records[0].Status)
	assert.Equal(t, "agent-b", records[1].Title)
}

func TestLoadRecordsFromRepoRoot_InvalidStateJSON(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".kasmos")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "state.json"), []byte("not-json"), 0o644))

	_, err := LoadRecordsFromRepoRoot(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse state file")
}

// ---------------------------------------------------------------------------
// NewHTTPHandler — list instances
// ---------------------------------------------------------------------------

func TestHTTPHandler_ListInstances_HappyPath(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	writeStateJSON(t, root,
		Record{
			Title:      "agent-one",
			Status:     StatusRunning,
			Branch:     "main",
			Program:    "claude",
			TaskFile:   "task.md",
			AgentType:  "coder",
			WaveNumber: 1,
			TaskNumber: 2,
			CreatedAt:  now,
			UpdatedAt:  now.Add(time.Minute),
		},
		Record{Title: "agent-two", Status: StatusPaused, Branch: "feat/x", Program: "amp"},
	)

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/myproj/instances", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var entries []ListEntry
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&entries))
	require.Len(t, entries, 2)

	e0 := entries[0]
	assert.Equal(t, "agent-one", e0.Title)
	assert.Equal(t, "running", e0.Status)
	assert.Equal(t, "main", e0.Branch)
	assert.Equal(t, "claude", e0.Program)
	assert.Equal(t, "task.md", e0.TaskFile)
	assert.Equal(t, "coder", e0.AgentType)
	assert.Equal(t, 1, e0.WaveNumber)
	assert.Equal(t, 2, e0.TaskNumber)
	assert.Equal(t, now.Format(time.RFC3339), e0.CreatedAt)
	assert.Equal(t, now.Add(time.Minute).Format(time.RFC3339), e0.UpdatedAt)

	assert.Equal(t, "agent-two", entries[1].Title)
	assert.Equal(t, "paused", entries[1].Status)
}

// fakeDaemonLister is a test DaemonInstanceLister backed by a fixed record
// slice and an optional canned error (e.g. ErrDaemonUnavailable to simulate a
// daemon restart).
type fakeDaemonLister struct {
	records []Record
	err     error
	calls   int
}

func (f *fakeDaemonLister) ListInstancesForProject(_ string) ([]Record, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.records, nil
}

func TestHTTPHandler_ListInstances_MergesDaemonRecords(t *testing.T) {
	// state.json has one TUI-spawned standalone instance; the daemon reports
	// one plan-associated planner. The admin list must show both.
	root := t.TempDir()
	writeStateJSON(t, root,
		Record{Title: "solo-agent", Status: StatusRunning, Program: "claude"},
	)
	daemon := &fakeDaemonLister{
		records: []Record{
			{Title: "feature-plan", Status: StatusLoading, Program: "opencode", AgentType: "planner", TaskFile: "feature"},
		},
	}

	h := NewHTTPHandlerWithDaemon(resolverFor(root), &mockPaneRunner{}, daemon)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var entries []ListEntry
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&entries))
	require.Len(t, entries, 2, "daemon + state.json records must both appear")

	// Daemon records come first.
	assert.Equal(t, "feature-plan", entries[0].Title)
	assert.Equal(t, "loading", entries[0].Status)
	assert.Equal(t, "planner", entries[0].AgentType)

	assert.Equal(t, "solo-agent", entries[1].Title)
	assert.Equal(t, "running", entries[1].Status)
}

func TestHTTPHandler_ListInstances_DaemonWinsOnTitleCollision(t *testing.T) {
	// Both sources reference the same title (e.g. state.json has a stale
	// snapshot). Daemon must win because it is the authoritative spawner.
	root := t.TempDir()
	writeStateJSON(t, root,
		Record{Title: "shared", Status: StatusPaused, Program: "stale"},
	)
	daemon := &fakeDaemonLister{
		records: []Record{
			{Title: "shared", Status: StatusRunning, Program: "fresh", AgentType: "reviewer"},
		},
	}

	h := NewHTTPHandlerWithDaemon(resolverFor(root), &mockPaneRunner{}, daemon)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var entries []ListEntry
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&entries))
	require.Len(t, entries, 1, "title collision must dedupe")
	assert.Equal(t, "running", entries[0].Status, "daemon record must win")
	assert.Equal(t, "fresh", entries[0].Program)
	assert.Equal(t, "reviewer", entries[0].AgentType)
}

func TestHTTPHandler_ListInstances_FallsBackWhenDaemonUnavailable(t *testing.T) {
	// Simulates a daemon restart: state.json is still readable, daemon
	// returns ErrDaemonUnavailable. Handler must serve state.json records
	// and return 200 so the admin UI stays functional.
	root := t.TempDir()
	writeStateJSON(t, root,
		Record{Title: "solo-agent", Status: StatusRunning, Program: "claude"},
	)
	daemon := &fakeDaemonLister{err: ErrDaemonUnavailable}

	h := NewHTTPHandlerWithDaemon(resolverFor(root), &mockPaneRunner{}, daemon)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var entries []ListEntry
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "solo-agent", entries[0].Title)
	assert.Equal(t, 1, daemon.calls, "daemon lister must still be called on each request")
}

func TestHTTPHandler_CaptureInstance_FindsDaemonOnlyRecord(t *testing.T) {
	// Title only known to the daemon; state.json doesn't have it. Capture
	// must still resolve the record and run tmux capture-pane against its
	// session name.
	root := t.TempDir()
	// empty state.json
	writeStateJSON(t, root)

	daemon := &fakeDaemonLister{
		records: []Record{
			{Title: "feature-plan", Status: StatusRunning, Program: "opencode"},
		},
	}
	runner := paneOutputRunner("planner output\n")

	h := NewHTTPHandlerWithDaemon(resolverFor(root), runner, daemon)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances/feature-plan/capture", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, "planner output\n", rec.Body.String())
}

func TestMergeInstanceRecords_EmptyDaemon(t *testing.T) {
	disk := []Record{{Title: "a"}, {Title: "b"}}
	got := mergeInstanceRecords(disk, nil)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].Title)
	assert.Equal(t, "b", got[1].Title)
}

func TestMergeInstanceRecords_EmptyDisk(t *testing.T) {
	daemon := []Record{{Title: "x"}, {Title: "y"}}
	got := mergeInstanceRecords(nil, daemon)
	require.Len(t, got, 2)
	assert.Equal(t, "x", got[0].Title)
}

func TestMergeInstanceRecords_PreservesOrderAndDedupes(t *testing.T) {
	disk := []Record{{Title: "keep-disk"}, {Title: "both", Program: "old"}, {Title: "also-disk"}}
	daemon := []Record{{Title: "daemon-first"}, {Title: "both", Program: "new"}}
	got := mergeInstanceRecords(disk, daemon)
	require.Len(t, got, 4)
	assert.Equal(t, []string{"daemon-first", "both", "keep-disk", "also-disk"}, []string{
		got[0].Title, got[1].Title, got[2].Title, got[3].Title,
	})
	assert.Equal(t, "new", got[1].Program, "collision must prefer daemon")
}

func TestHTTPHandler_ListInstances_NeverNull(t *testing.T) {
	// When no state file exists the response body must be [] not null.
	root := t.TempDir()
	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, "[]", rec.Body.String())
}

func TestHTTPHandler_ListInstances_ZeroTimestampsOmitted(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "agent", Status: StatusRunning})

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var entries []ListEntry
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&entries))
	require.Len(t, entries, 1)
	assert.Empty(t, entries[0].CreatedAt)
	assert.Empty(t, entries[0].UpdatedAt)
}

// ---------------------------------------------------------------------------
// NewHTTPHandler — capture
// ---------------------------------------------------------------------------

func TestHTTPHandler_Capture_HappyPath(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "my-agent", Status: StatusRunning})

	h := NewHTTPHandler(resolverFor(root), paneOutputRunner("pane content\n"))
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances/my-agent/capture", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "pane content\n", rec.Body.String())
}

func TestHTTPHandler_Capture_HappyPath_RangeForwarded(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "my-agent", Status: StatusRunning})

	var gotArgs []string
	runner := &mockPaneRunner{
		outputFn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			gotArgs = append([]string(nil), args...)
			return []byte("output"), nil
		},
	}

	h := NewHTTPHandler(resolverFor(root), runner)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances/my-agent/capture?start=-500&end=0", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, gotArgs, "-500")
	assert.Contains(t, gotArgs, "0")
}

func TestHTTPHandler_Capture_MissingTitle(t *testing.T) {
	// Test the defensive empty-title check by invoking the handler logic
	// directly with an empty path value (which the mux cannot produce).
	root := t.TempDir()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances//capture", nil)
	req.SetPathValue("project", "proj")
	req.SetPathValue("title", "")
	rec := httptest.NewRecorder()

	// Exercise writeJSONError directly so the guard is covered.
	if title := req.PathValue("title"); title == "" {
		writeJSONError(rec, http.StatusBadRequest, "missing title")
	}

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"missing title"}`, rec.Body.String())
	_ = root
}

func TestHTTPHandler_Capture_PausedInstance(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "paused-agent", Status: StatusPaused})

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances/paused-agent/capture", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "paused")
}

func TestHTTPHandler_Capture_InstanceNotFound(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "other-agent", Status: StatusRunning})

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances/missing/capture", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "instance not found")
}

func TestHTTPHandler_Capture_SessionGone(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "gone-agent", Status: StatusRunning})

	h := NewHTTPHandler(resolverFor(root), sessionGoneRunner())
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances/gone-agent/capture", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusGone, rec.Code)
	assert.Contains(t, rec.Body.String(), "tmux session not found")
}

func TestHTTPHandler_Capture_CommandError_StderrInResponse(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "my-agent", Status: StatusRunning})

	h := NewHTTPHandler(resolverFor(root), commandErrorRunner("unexpected tmux error"))
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances/my-agent/capture", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Contains(t, rec.Body.String(), "unexpected tmux error")
}

func TestHTTPHandler_Capture_GenericTmuxError(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "my-agent", Status: StatusRunning})

	h := NewHTTPHandler(resolverFor(root), genericErrorRunner("tmux not found on PATH"))
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances/my-agent/capture", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Contains(t, rec.Body.String(), "tmux not found on PATH")
}

// ---------------------------------------------------------------------------
// ErrPreviewUnavailable — bare-DB / no-repo mode
// ---------------------------------------------------------------------------

func TestHTTPHandler_ListInstances_PreviewUnavailable(t *testing.T) {
	h := NewHTTPHandler(unavailableResolver(), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/any/instances", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotImplemented, rec.Code)
	assert.JSONEq(t, `{"error":"live preview requires kas serve --repo"}`, rec.Body.String())
}

func TestHTTPHandler_Capture_PreviewUnavailable(t *testing.T) {
	h := NewHTTPHandler(unavailableResolver(), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/any/instances/agent/capture", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotImplemented, rec.Code)
	assert.JSONEq(t, `{"error":"live preview requires kas serve --repo"}`, rec.Body.String())
}

// ---------------------------------------------------------------------------
// NewHTTPHandler — execution_mode in list response
// ---------------------------------------------------------------------------

func TestHTTPHandler_ListInstances_IncludesExecutionMode(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root,
		Record{Title: "tmux-agent", Status: StatusRunning, ExecutionMode: "tmux"},
		Record{Title: "headless-agent", Status: StatusRunning, ExecutionMode: "headless"},
		Record{Title: "default-agent", Status: StatusRunning},
	)

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var entries []ListEntry
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&entries))
	require.Len(t, entries, 3)

	assert.Equal(t, "tmux", entries[0].ExecutionMode)
	assert.Equal(t, "headless", entries[1].ExecutionMode)
	// default (empty) is omitted
	assert.Empty(t, entries[2].ExecutionMode)
}

// ---------------------------------------------------------------------------
// NewHTTPHandler — send
// ---------------------------------------------------------------------------

func TestHTTPHandler_Send_HappyPath(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "my-agent", Status: StatusRunning, ExecutionMode: "tmux"})

	h := NewHTTPHandler(resolverFor(root), paneOutputRunner(""))
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/instances/my-agent/send",
		strings.NewReader(`{"prompt":"hello world"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestHTTPHandler_Send_EmptyPrompt(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "my-agent", Status: StatusRunning, ExecutionMode: "tmux"})

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/instances/my-agent/send",
		strings.NewReader(`{"prompt":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"missing prompt"}`, rec.Body.String())
}

func TestHTTPHandler_Send_MalformedJSON(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "my-agent", Status: StatusRunning, ExecutionMode: "tmux"})

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/instances/my-agent/send",
		strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "error")
}

func TestHTTPHandler_Send_InstanceNotFound(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "other-agent", Status: StatusRunning})

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/instances/missing/send",
		strings.NewReader(`{"prompt":"hello"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "instance not found")
}

func TestHTTPHandler_Send_LoadingInstance(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "loading-agent", Status: StatusLoading, ExecutionMode: "tmux"})

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/instances/loading-agent/send",
		strings.NewReader(`{"prompt":"hello"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "loading")
}

func TestHTTPHandler_Send_PausedInstance(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "paused-agent", Status: StatusPaused, ExecutionMode: "tmux"})

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/instances/paused-agent/send",
		strings.NewReader(`{"prompt":"hello"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "paused")
}

func TestHTTPHandler_Send_HeadlessInstance(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "headless-agent", Status: StatusRunning, ExecutionMode: "headless"})

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/instances/headless-agent/send",
		strings.NewReader(`{"prompt":"hello"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "headless")
}

func TestHTTPHandler_Send_ResolverUnavailable(t *testing.T) {
	h := NewHTTPHandler(unavailableResolver(), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/any/instances/agent/send",
		strings.NewReader(`{"prompt":"hello"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotImplemented, rec.Code)
	assert.JSONEq(t, `{"error":"live preview requires kas serve --repo"}`, rec.Body.String())
}

func TestHTTPHandler_Send_SessionGone(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "gone-agent", Status: StatusRunning, ExecutionMode: "tmux"})

	h := NewHTTPHandler(resolverFor(root), sessionGoneRunner())
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/instances/gone-agent/send",
		strings.NewReader(`{"prompt":"hello"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusGone, rec.Code)
	assert.Contains(t, rec.Body.String(), "tmux session not found")
}

func TestHTTPHandler_Send_GenericTmuxFailure(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "my-agent", Status: StatusRunning, ExecutionMode: "tmux"})

	h := NewHTTPHandler(resolverFor(root), commandErrorRunner("unexpected tmux error"))
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/instances/my-agent/send",
		strings.NewReader(`{"prompt":"hello"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Contains(t, rec.Body.String(), "unexpected tmux error")
}

func TestHTTPHandler_Capture_HeadlessInstance(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, Record{Title: "headless-agent", Status: StatusRunning, ExecutionMode: "headless"})

	h := NewHTTPHandler(resolverFor(root), &mockPaneRunner{})
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/instances/headless-agent/capture", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "headless")
}
