package tasktools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	var entries []taskListEntry
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "done-plan", entries[0].Filename)
	assert.Equal(t, "done", entries[0].Status)
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
