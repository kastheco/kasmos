package statustools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/livestatus"
	"github.com/kastheco/kasmos/internal/mcpserver/routing"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiveStatusHandlerGracefullyDegradesWithoutDaemon(t *testing.T) {
	const project = "kasmos"
	store := taskstore.NewTestSQLiteStore(t)
	entries := []taskstore.TaskEntry{
		{Filename: "plan", Status: taskstore.StatusPlanning},
		{Filename: "ready", Status: taskstore.StatusReady, ExecutionState: taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveWaiting)}},
		{Filename: "build", Status: taskstore.StatusImplementing, LatestReviewFeedback: "changes requested"},
		{Filename: "review", Status: taskstore.StatusReviewing},
		{Filename: "verify", Status: taskstore.StatusVerifying},
	}
	for _, entry := range entries {
		require.NoError(t, store.Create(project, entry))
	}

	handler := makeLiveStatusHandler(
		routing.NewRegisterConfig(project, []string{project}),
		store,
		filepath.Join(t.TempDir(), "missing.sock"),
	)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)

	var status livestatus.LiveStatus
	require.NoError(t, json.Unmarshal([]byte(text.Text), &status))
	assert.Equal(t, livestatus.SchemaVersion, status.SchemaVersion)
	assert.Equal(t, livestatus.LifecycleCounts{Planning: 1, Ready: 1, Implementing: 1, Reviewing: 1, Verifying: 1, Total: 5}, status.Lifecycle)
	assert.False(t, status.DaemonRunning)
	assert.Empty(t, status.ActiveAgents)
	assert.ElementsMatch(t, []string{livestatus.KindNeedsDecision, livestatus.KindReviewFeedback}, []string{status.Attention[0].Kind, status.Attention[1].Kind})
}
