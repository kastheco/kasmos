package tasktools

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTaskToolConfig(t *testing.T, repoDir, body string) {
	t.Helper()
	configDir := filepath.Join(repoDir, ".kasmos")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(body), 0o644))
}

func initTaskToolTestRepo(t *testing.T, dir string) {
	t.Helper()
	out, err := exec.Command("git", "init", dir).CombinedOutput()
	if err != nil {
		t.Skipf("git init failed (%v): %s", err, out)
	}
}

func startTaskToolDaemonSocketServer(t *testing.T, handler http.Handler) string {
	t.Helper()

	homeDir, err := os.MkdirTemp("", "ks-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(homeDir) })
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_RUNTIME_DIR", homeDir)

	socketPath := filepath.Join(homeDir, "kasmos", "kas.sock")
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

func newTaskToolDaemonMux(t *testing.T, repos []string, taskHandler http.Handler) http.Handler {
	t.Helper()

	registered := make([]taskstoreTestRepoStatus, 0, len(repos))
	for _, project := range repos {
		registered = append(registered, taskstoreTestRepoStatus{Project: project})
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

type taskstoreTestRepoStatus struct {
	Project string `json:"project"`
}

func mockReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func textResult(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	return tc.Text
}

func TestTaskShowHandler_ReturnsStoredContent(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: "my-plan", Status: taskstore.StatusReady, CreatedAt: time.Now()}))
	require.NoError(t, store.SetContent(project, "my-plan", "# plan\n"))

	handler := makeTaskShowHandler(project, store)
	result, err := handler(context.Background(), mockReq(map[string]any{"filename": "my-plan.md"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "# plan\n", textResult(t, result))
}

func TestTaskListHandler_FiltersByStatus(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: "ready-plan", Status: taskstore.StatusReady, CreatedAt: time.Now()}))
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: "done-plan", Status: taskstore.StatusDone, CreatedAt: time.Now()}))

	handler := makeTaskListHandler(project, store)
	result, err := handler(context.Background(), mockReq(map[string]any{"status": "done"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var wrapper taskListResult
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &wrapper))
	require.Len(t, wrapper.Tasks, 1)
	assert.Equal(t, "done-plan", wrapper.Tasks[0].Filename)
	assert.Equal(t, "done", wrapper.Tasks[0].Status)
}

func TestTaskCreateHandler_DefaultsBranchAndReadyStatus(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"

	handler := makeTaskCreateHandler(project, store)
	result, err := handler(context.Background(), mockReq(map[string]any{"name": "new-plan"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	entry, err := store.Get(project, "new-plan")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, entry.Status)
	assert.Equal(t, "plan/new-plan", entry.Branch)
	assert.Equal(t, taskstore.ExecutionState{}, entry.ExecutionState)
}

func TestTaskCreateHandler_DecodesEscapedMultilineContent(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"

	handler := makeTaskCreateHandler(project, store)
	result, err := handler(context.Background(), mockReq(map[string]any{
		"name":    "new-plan",
		"content": "# Plan\\n\\n## Wave 1\\n\\n### Task 1: write tests\\n",
	}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := store.GetContent(project, "new-plan")
	require.NoError(t, err)
	assert.Equal(t, "# Plan\n\n## Wave 1\n\n### Task 1: write tests\n", content)
}

func TestTaskUpdateContentHandler_ReturnsWarningForDraft(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: "draft", Status: taskstore.StatusReady, CreatedAt: time.Now()}))

	handler := makeTaskUpdateContentHandler(project, store)
	result, err := handler(context.Background(), mockReq(map[string]any{"filename": "draft", "content": "# Draft\n\n**Goal:** in progress\n"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var payload taskMutationResult
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &payload))
	assert.True(t, payload.Updated)
	assert.Contains(t, payload.Warning, "no wave headers found")
}

func TestTaskDeleteHandler_DeletesStoredTask(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	filename := "delete-me"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: filename, Status: taskstore.StatusReady, CreatedAt: time.Now()}))

	handler := makeTaskDeleteHandler(project, store)
	result, err := handler(context.Background(), mockReq(map[string]any{"filename": filename + ".md"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var payload taskMutationResult
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &payload))
	assert.Equal(t, filename, payload.Filename)
	assert.True(t, payload.Deleted)

	_, err = store.Get(project, filename)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTaskDeleteHandler_ReturnsErrorWhenTaskMissing(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"

	handler := makeTaskDeleteHandler(project, store)
	result, err := handler(context.Background(), mockReq(map[string]any{"filename": "missing"}))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, textResult(t, result), "task_delete:")
	assert.Contains(t, textResult(t, result), "not found")
}

func TestTaskUpdateContentHandler_DecodesEscapedMultilineContent(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: "my-plan", Status: taskstore.StatusReady, CreatedAt: time.Now()}))

	handler := makeTaskUpdateContentHandler(project, store)
	result, err := handler(context.Background(), mockReq(map[string]any{
		"filename": "my-plan",
		"content":  "# Plan\\n\\n## Wave 1\\n\\n### Task 1: write tests\\n",
	}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, readErr := store.GetContent(project, "my-plan")
	require.NoError(t, readErr)
	assert.Equal(t, "# Plan\n\n## Wave 1\n\n### Task 1: write tests\n", content)
}

func TestTaskShowHandler_UsesAuthoritativeStoreWhenStoreNilEvenWhenDaemonRegistered(t *testing.T) {
	backend := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	require.NoError(t, backend.Create(project, taskstore.TaskEntry{Filename: "shared-plan", Status: taskstore.StatusReady, CreatedAt: time.Now()}))
	require.NoError(t, backend.SetContent(project, "shared-plan", "# stale\n"))

	repoDir := t.TempDir()
	initTaskToolTestRepo(t, repoDir)
	t.Chdir(repoDir)
	startTaskToolDaemonSocketServer(t, newTaskToolDaemonMux(t, []string{project}, taskstore.NewHandler(backend)))

	authoritative, err := taskstore.OpenAuthoritativeStore(project)
	require.NoError(t, err)
	require.NoError(t, authoritative.Create(project, taskstore.TaskEntry{Filename: "shared-plan", Status: taskstore.StatusReady, CreatedAt: time.Now()}))
	require.NoError(t, authoritative.SetContent(project, "shared-plan", "# fresh\n"))
	require.NoError(t, authoritative.Close())

	handler := makeTaskShowHandler(project, nil)
	result, err := handler(context.Background(), mockReq(map[string]any{"filename": "shared-plan"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "# fresh\n", textResult(t, result))
}

func TestTaskTransitionHandler_SupportsReviewChangesAlias(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: "my-plan", Status: taskstore.StatusReviewing, CreatedAt: time.Now()}))

	handler := makeTaskTransitionHandler(project, store)
	result, err := handler(context.Background(), mockReq(map[string]any{"filename": "my-plan", "event": "review_changes"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	entry, err := store.Get(project, "my-plan")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status)
}

func TestTaskTransitionHandler_ForceCompletesWhenRequested(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: "my-plan", Status: taskstore.StatusReady, CreatedAt: time.Now()}))

	handler := makeTaskTransitionHandler(project, store)
	result, err := handler(context.Background(), mockReq(map[string]any{"filename": "my-plan", "event": "review_approved", "force": true}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	entry, err := store.Get(project, "my-plan")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusDone, entry.Status)

	var payload taskMutationResult
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &payload))
	assert.True(t, payload.Forced)
	assert.Equal(t, "done", payload.Status)
}

func TestTaskTransitionHandler_ForcePlannerFinishedKeepsReadyCompatibility(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: "my-plan", Status: taskstore.StatusPlanning, CreatedAt: time.Now()}))

	handler := makeTaskTransitionHandler(project, store)
	result, err := handler(context.Background(), mockReq(map[string]any{"filename": "my-plan", "event": "planner_finished", "force": true}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	entry, err := store.Get(project, "my-plan")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, entry.Status)
	assert.Equal(t, "planned", entry.ExecutionState.Phase)

	ps, err := taskstate.Load(store, project, "")
	require.NoError(t, err)
	stateEntry, ok := ps.Entry("my-plan")
	require.True(t, ok)
	assert.True(t, taskstate.IsPlannedReady(stateEntry))

	var payload taskMutationResult
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &payload))
	assert.True(t, payload.Forced)
	assert.Equal(t, "ready", payload.Status)
}

func TestTaskUpdateContentHandler_UsesAuthoritativeStoreWhenStoreNilEvenWhenDaemonRegistered(t *testing.T) {
	backend := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	require.NoError(t, backend.Create(project, taskstore.TaskEntry{Filename: "shared-plan", Status: taskstore.StatusReady, CreatedAt: time.Now()}))
	require.NoError(t, backend.SetContent(project, "shared-plan", "# stale\n"))

	repoDir := t.TempDir()
	initTaskToolTestRepo(t, repoDir)
	startTaskToolDaemonSocketServer(t, newTaskToolDaemonMux(t, []string{project}, taskstore.NewHandler(backend)))
	t.Chdir(repoDir)

	authoritative, err := taskstore.OpenAuthoritativeStore(project)
	require.NoError(t, err)
	require.NoError(t, authoritative.Create(project, taskstore.TaskEntry{Filename: "shared-plan", Status: taskstore.StatusReady, CreatedAt: time.Now()}))
	require.NoError(t, authoritative.Close())

	handler := makeTaskUpdateContentHandler(project, nil)
	result, err := handler(context.Background(), mockReq(map[string]any{"filename": "shared-plan", "content": "# Plan\n\n## Wave 1\n\n### Task 1: write tests\n"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	authoritative, err = taskstore.OpenAuthoritativeStore(project)
	require.NoError(t, err)
	content, readErr := authoritative.GetContent(project, "shared-plan")
	require.NoError(t, readErr)
	assert.Equal(t, "# Plan\n\n## Wave 1\n\n### Task 1: write tests\n", content)
	require.NoError(t, authoritative.Close())

	staleContent, readErr := backend.GetContent(project, "shared-plan")
	require.NoError(t, readErr)
	assert.Equal(t, "# stale\n", staleContent)
}

func TestTaskCreateHandler_UsesAuthoritativeStoreWhenDaemonUnavailable(t *testing.T) {
	repoDir := t.TempDir()
	initTaskToolTestRepo(t, repoDir)
	t.Chdir(repoDir)

	handler := makeTaskCreateHandler("test-project", nil)
	result, err := handler(context.Background(), mockReq(map[string]any{"name": "new-plan"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	store, err := taskstore.OpenAuthoritativeStore("test-project")
	require.NoError(t, err)
	entry, err := store.Get("test-project", "new-plan")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, entry.Status)
	assert.Equal(t, "plan/new-plan", entry.Branch)
	require.NoError(t, store.Close())
}
