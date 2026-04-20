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
			{Kind: RowUser, Text: "show me the logs", Timestamp: ts},
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

// TestPresentationTurn_JSONContract_WithPayloadFields verifies that ToolDiff,
// ToolPreview, and Activity fields survive a JSON round-trip.
func TestPresentationTurn_JSONContract_WithPayloadFields(t *testing.T) {
	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	oldN, newN := 1, 1

	turn := &PresentationTurn{
		ID:        "turn-xyz",
		Number:    1,
		StartedAt: ts,
		Rows: []PresentationRow{
			{
				Kind:      RowToolDiff,
				Timestamp: ts,
				ToolName:  "Edit",
				ToolDiff: &ToolDiffPayload{
					Path: "foo.go",
					Lines: []ToolDiffLine{
						{Kind: DiffLineContext, OldNumber: &oldN, NewNumber: &newN, OldText: "ctx", NewText: "ctx"},
					},
					Truncated:       false,
					HiddenLineCount: 0,
				},
			},
			{
				Kind:      RowToolPreview,
				Timestamp: ts,
				ToolName:  "read_file",
				ToolPreview: &ToolPreviewPayload{
					Lines:           []string{"line1", "line2"},
					Truncated:       true,
					HiddenLineCount: 5,
				},
			},
		},
		Activity: &TurnActivity{
			Kind:      "tool",
			Label:     "Edit foo.go",
			StartedAt: ts,
		},
	}

	data, err := json.Marshal(turn)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	// Activity must appear in the wire format.
	assert.Contains(t, raw, "activity")

	// Rows must encode ToolDiff and ToolPreview payloads.
	rows, ok := raw["rows"].([]any)
	require.True(t, ok)
	require.Len(t, rows, 2)

	row0, ok := rows[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "tool_diff", row0["kind"])
	assert.NotNil(t, row0["tool_diff"])

	row1, ok := rows[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "tool_preview", row1["kind"])
	assert.NotNil(t, row1["tool_preview"])

	// Full round-trip.
	var decoded PresentationTurn
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.Len(t, decoded.Rows, 2)
	require.NotNil(t, decoded.Rows[0].ToolDiff)
	assert.Equal(t, "foo.go", decoded.Rows[0].ToolDiff.Path)
	require.Len(t, decoded.Rows[0].ToolDiff.Lines, 1)
	assert.Equal(t, DiffLineContext, decoded.Rows[0].ToolDiff.Lines[0].Kind)

	require.NotNil(t, decoded.Rows[1].ToolPreview)
	assert.Equal(t, []string{"line1", "line2"}, decoded.Rows[1].ToolPreview.Lines)
	assert.True(t, decoded.Rows[1].ToolPreview.Truncated)
	assert.Equal(t, 5, decoded.Rows[1].ToolPreview.HiddenLineCount)

	require.NotNil(t, decoded.Activity)
	assert.Equal(t, "tool", decoded.Activity.Kind)
	assert.Equal(t, "Edit foo.go", decoded.Activity.Label)
	assert.True(t, decoded.Activity.StartedAt.Equal(ts))
}

// TestClonePresentationTurns_NilAndEmpty verifies degenerate inputs.
func TestClonePresentationTurns_NilAndEmpty(t *testing.T) {
	assert.Nil(t, ClonePresentationTurns(nil))
	assert.Nil(t, ClonePresentationTurns([]*PresentationTurn{}))
}

// TestClonePresentationTurns_DeepCopy_ToolDiff verifies that mutating the
// clone's ToolDiff does not affect the original.
func TestClonePresentationTurns_DeepCopy_ToolDiff(t *testing.T) {
	oldN, newN := 1, 1
	src := []*PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Rows: []PresentationRow{
				{
					Kind:     RowToolDiff,
					ToolName: "Edit",
					ToolDiff: &ToolDiffPayload{
						Path: "a.go",
						Lines: []ToolDiffLine{
							{Kind: DiffLineAdded, NewNumber: &newN, NewText: "added"},
							{Kind: DiffLineRemoved, OldNumber: &oldN, OldText: "removed"},
						},
					},
				},
			},
		},
	}

	cloned := ClonePresentationTurns(src)
	require.Len(t, cloned, 1)

	// Mutate the clone.
	cloned[0].Rows[0].ToolDiff.Path = "mutated.go"
	cloned[0].Rows[0].ToolDiff.Lines[0].NewText = "mutated"

	// Original must be unchanged.
	assert.Equal(t, "a.go", src[0].Rows[0].ToolDiff.Path)
	assert.Equal(t, "added", src[0].Rows[0].ToolDiff.Lines[0].NewText)
}

// TestClonePresentationTurns_DeepCopy_ToolPreview verifies that mutating the
// clone's ToolPreview does not affect the original.
func TestClonePresentationTurns_DeepCopy_ToolPreview(t *testing.T) {
	src := []*PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Rows: []PresentationRow{
				{
					Kind:     RowToolPreview,
					ToolName: "read_file",
					ToolPreview: &ToolPreviewPayload{
						Lines: []string{"line1", "line2"},
					},
				},
			},
		},
	}

	cloned := ClonePresentationTurns(src)
	require.Len(t, cloned, 1)

	// Mutate the clone.
	cloned[0].Rows[0].ToolPreview.Lines[0] = "mutated"

	// Original must be unchanged.
	assert.Equal(t, "line1", src[0].Rows[0].ToolPreview.Lines[0])
}

// TestClonePresentationTurns_DeepCopy_Activity verifies that mutating the
// clone's Activity does not affect the original.
func TestClonePresentationTurns_DeepCopy_Activity(t *testing.T) {
	ts := time.Now()
	src := []*PresentationTurn{
		{
			ID:     "t1",
			Number: 1,
			Activity: &TurnActivity{
				Kind:      "tool",
				Label:     "original",
				StartedAt: ts,
			},
		},
	}

	cloned := ClonePresentationTurns(src)
	require.Len(t, cloned, 1)

	cloned[0].Activity.Label = "mutated"

	assert.Equal(t, "original", src[0].Activity.Label)
}

// TestClonePresentationTurns_NilTurn_HandledGracefully verifies that nil turns
// in the input slice are handled without panic.
func TestClonePresentationTurns_NilTurn_HandledGracefully(t *testing.T) {
	src := []*PresentationTurn{nil, {ID: "t1", Number: 1}}
	cloned := ClonePresentationTurns(src)
	require.Len(t, cloned, 2)
	assert.Nil(t, cloned[0])
	assert.Equal(t, "t1", cloned[1].ID)
}
