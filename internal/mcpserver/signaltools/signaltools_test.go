package signaltools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

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

func TestSignalCreateHandler_AcceptsHyphenatedSignalTypes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "signals.db")
	gw, err := taskstore.NewSQLiteSignalGateway(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	handler := makeSignalCreateHandler("test-project", gw)
	result, err := handler(context.Background(), mockReq(map[string]any{"signal_type": "planner-finished", "plan_file": "my-plan"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	signals, err := gw.List("test-project", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "planner_finished", signals[0].SignalType)

	var payload signalCreateResult
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &payload))
	assert.True(t, payload.Created)
	assert.Equal(t, "planner_finished", payload.SignalType)
}

func TestSignalCreateHandler_NormalizesReviewChangesAlias(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "signals.db")
	gw, err := taskstore.NewSQLiteSignalGateway(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	handler := makeSignalCreateHandler("test-project", gw)
	result, err := handler(context.Background(), mockReq(map[string]any{"signal_type": "review-changes", "plan_file": "my-plan", "payload": "needs fixes"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	signals, err := gw.List("test-project", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "review_changes_requested", signals[0].SignalType)
	assert.JSONEq(t, `{"body":"needs fixes"}`, signals[0].Payload)
}
