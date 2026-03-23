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

func TestSignalCreateHandler_PayloadContracts(t *testing.T) {
	tests := []struct {
		name        string
		signalType  string
		payload     string
		wantError   string
		wantStored  string
		wantSignal  string
		wantPayload string
	}{
		{
			name:        "implement task finished accepts structured payload",
			signalType:  "implement-task-finished",
			payload:     `{"wave_number":2,"task_number":3}`,
			wantStored:  "implement_task_finished",
			wantSignal:  "implement_task_finished",
			wantPayload: `{"wave_number":2,"task_number":3}`,
		},
		{
			name:       "implement task finished rejects non numeric payload",
			signalType: "implement-task-finished",
			payload:    `{"wave_number":"x","task_number":3}`,
			wantError:  "wave_number must be a number",
		},
		{
			name:       "implement task finished rejects non numeric task payload",
			signalType: "implement-task-finished",
			payload:    `{"wave_number":2,"task_number":"x"}`,
			wantError:  "task_number must be a number",
		},
		{
			name:       "implement wave rejects fractional payload",
			signalType: "implement-wave",
			payload:    `{"wave_number":1.5}`,
			wantError:  "wave_number must be a whole number",
		},
		{
			name:       "architect compatibility signal rejects payload",
			signalType: "elaborator-finished",
			payload:    "unexpected payload",
			wantError:  "does not accept a payload",
		},
		{
			name:        "architect compatibility signal stores empty payload",
			signalType:  "elaborator-finished",
			payload:     "",
			wantStored:  "elaborator_finished",
			wantSignal:  "elaborator_finished",
			wantPayload: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "signals.db")
			gw, err := taskstore.NewSQLiteSignalGateway(dbPath)
			require.NoError(t, err)
			t.Cleanup(func() { _ = gw.Close() })

			handler := makeSignalCreateHandler("test-project", gw)
			result, err := handler(context.Background(), mockReq(map[string]any{"signal_type": tt.signalType, "plan_file": "my-plan", "payload": tt.payload}))
			require.NoError(t, err)

			if tt.wantError != "" {
				require.True(t, result.IsError)
				assert.Contains(t, textResult(t, result), tt.wantError)

				signals, listErr := gw.List("test-project", taskstore.SignalPending)
				require.NoError(t, listErr)
				assert.Empty(t, signals)
				return
			}

			assert.False(t, result.IsError)
			signals, listErr := gw.List("test-project", taskstore.SignalPending)
			require.NoError(t, listErr)
			require.Len(t, signals, 1)
			assert.Equal(t, tt.wantStored, signals[0].SignalType)
			assert.Equal(t, tt.wantPayload, signals[0].Payload)

			var payload signalCreateResult
			require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &payload))
			assert.True(t, payload.Created)
			assert.Equal(t, tt.wantSignal, payload.SignalType)
		})
	}
}
