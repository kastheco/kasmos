package sdk

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPresentationTurn_JSONContract verifies that the wire-format field names
// are stable snake_case keys regardless of Go field-name casing. It also
// round-trips through JSON to confirm unmarshalling restores full fidelity.
func TestPresentationTurn_JSONContract(t *testing.T) {
	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	completed := time.Date(2024, 6, 1, 12, 0, 5, 0, time.UTC)

	turn := &PresentationTurn{
		ID:          "turn-abc",
		Number:      3,
		StartedAt:   ts,
		CompletedAt: completed,
		Interrupted: false,
		ToolCount:   2,
		Rows: []PresentationRow{
			{Kind: RowTool, Text: "bash(ls)", Timestamp: ts, ToolName: "bash", IsError: false},
			{Kind: RowResult, Text: "file.go", Timestamp: ts, ToolName: "bash", IsError: false},
			{Kind: RowResponse, Text: "", Timestamp: ts},
			{Kind: RowProse, Text: "Here is the output.", Timestamp: ts},
			{Kind: RowPermission, Text: "allow write?", Timestamp: ts},
			{Kind: RowSystem, Text: "rate limit", Timestamp: ts, IsError: true},
			{Kind: RowStatus, Text: "[interrupted]", Timestamp: ts},
		},
	}

	data, err := json.Marshal(turn)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	// Assert top-level field names.
	assert.Contains(t, raw, "id")
	assert.Contains(t, raw, "number")
	assert.Contains(t, raw, "started_at")
	assert.Contains(t, raw, "completed_at")
	assert.Contains(t, raw, "interrupted")
	assert.Contains(t, raw, "tool_count")
	assert.Contains(t, raw, "rows")

	// Assert row-level field names from the first row.
	rows, ok := raw["rows"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, rows)
	row0, ok := rows[0].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, row0, "kind")
	assert.Contains(t, row0, "text")
	assert.Contains(t, row0, "timestamp")
	assert.Contains(t, row0, "tool_name")
	assert.Contains(t, row0, "is_error")

	// Confirm values survive the round-trip.
	var decoded PresentationTurn
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, turn.ID, decoded.ID)
	assert.Equal(t, turn.Number, decoded.Number)
	assert.True(t, turn.StartedAt.Equal(decoded.StartedAt))
	assert.True(t, turn.CompletedAt.Equal(decoded.CompletedAt))
	assert.Equal(t, turn.Interrupted, decoded.Interrupted)
	assert.Equal(t, turn.ToolCount, decoded.ToolCount)
	require.Len(t, decoded.Rows, len(turn.Rows))
	for i, r := range turn.Rows {
		assert.Equal(t, r.Kind, decoded.Rows[i].Kind, "row %d kind", i)
		assert.Equal(t, r.Text, decoded.Rows[i].Text, "row %d text", i)
		assert.Equal(t, r.ToolName, decoded.Rows[i].ToolName, "row %d tool_name", i)
		assert.Equal(t, r.IsError, decoded.Rows[i].IsError, "row %d is_error", i)
	}
}

// TestPresentationTurn_JSONContract_Running verifies the running/unfinished
// variant: CompletedAt is zero and Interrupted is false.
func TestPresentationTurn_JSONContract_Running(t *testing.T) {
	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	turn := &PresentationTurn{
		ID:        "turn-running",
		Number:    1,
		StartedAt: ts,
		// CompletedAt is zero — turn is still running.
		Rows: []PresentationRow{
			{Kind: RowThinking, Text: "thinking 3.0s"},
		},
	}

	data, err := json.Marshal(turn)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "completed_at")
	assert.Nil(t, raw["completed_at"], "running turns must marshal zero completed_at as null")

	rows, ok := raw["rows"].([]any)
	require.True(t, ok)
	require.Len(t, rows, 1)
	row0, ok := rows[0].(map[string]any)
	require.True(t, ok)
	assert.Nil(t, row0["timestamp"], "zero row timestamps must marshal as null")

	var decoded PresentationTurn
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.True(t, decoded.Running(), "decoded running turn should report Running()=true")
	assert.Equal(t, turn.ID, decoded.ID)
	require.Len(t, decoded.Rows, 1)
	assert.Equal(t, RowThinking, decoded.Rows[0].Kind)
}
