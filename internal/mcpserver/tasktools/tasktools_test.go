package tasktools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
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

func TestTaskUpdateContentHandler_UsesAuthoritativeHTTPStoreWhenStoreNil(t *testing.T) {
	backend := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	require.NoError(t, backend.Create(project, taskstore.TaskEntry{Filename: "shared-plan", Status: taskstore.StatusReady, CreatedAt: time.Now()}))

	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()

	repoDir := t.TempDir()
	initTaskToolTestRepo(t, repoDir)
	writeTaskToolConfig(t, repoDir, fmt.Sprintf("database_url = %q\n", srv.URL))
	t.Chdir(repoDir)

	handler := makeTaskUpdateContentHandler(project, nil)
	result, err := handler(context.Background(), mockReq(map[string]any{"filename": "shared-plan", "content": "# Plan\n\n## Wave 1\n\n### Task 1: write tests\n"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	appStore := taskstore.NewHTTPStore(srv.URL, project)
	content, readErr := appStore.GetContent(project, "shared-plan")
	require.NoError(t, readErr)
	assert.Equal(t, "# Plan\n\n## Wave 1\n\n### Task 1: write tests\n", content)
}

func TestTaskCreateHandler_FailsFastWhenAuthoritativeStoreUnreachable(t *testing.T) {
	repoDir := t.TempDir()
	initTaskToolTestRepo(t, repoDir)
	writeTaskToolConfig(t, repoDir, "database_url = \"http://127.0.0.1:1\"\n")
	t.Chdir(repoDir)

	handler := makeTaskCreateHandler("test-project", nil)
	result, err := handler(context.Background(), mockReq(map[string]any{"name": "new-plan"}))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, textResult(t, result), "task store unreachable")
}
