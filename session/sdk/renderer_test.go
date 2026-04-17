package sdk

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderer_Empty_ReturnsEmptyString(t *testing.T) {
	r := NewRenderer()
	assert.Equal(t, "", r.Capture())
}

func TestRenderer_TextDelta_SingleLine(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "hello"})
	assert.Equal(t, "hello", r.Capture())
}

func TestRenderer_TextDelta_MultipleFragments(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "hel"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "lo"})
	assert.Equal(t, "hello", r.Capture())
}

func TestRenderer_TextDelta_NewlineBreaksLine(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "line1\nline2"})
	assert.Equal(t, "line1\nline2", r.Capture())
}

func TestRenderer_TextDelta_MultipleLines(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "a\nb\nc"})
	assert.Equal(t, "a\nb\nc", r.Capture())
}

func TestRenderer_ToolCall_ProducesFormattedLine(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{
		Kind:      EventToolCall,
		ToolName:  "bash",
		ToolInput: `{"cmd":"ls"}`,
		Timestamp: time.Now(),
	})
	content := r.Capture()
	assert.Contains(t, content, "bash")
}

func TestRenderer_ToolResult_ShortTextShowsSummary(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventToolResult, ToolName: "bash", ToolResult: "ok"})
	content := r.Capture()
	assert.Contains(t, content, "ok", "short successful tool output should render as compact summary")
}

func TestRenderer_ToolResult_LongTextCompressedToLineCount(t *testing.T) {
	r := NewRenderer()
	long := strings.Repeat("line\n", 50)
	r.AddEvent(Event{Kind: EventToolResult, ToolName: "bash", ToolResult: long})
	content := r.Capture()
	assert.Contains(t, content, "lines", "long tool output should compress to line count")
	assert.NotContains(t, content, "line\nline\nline\nline\nline", "raw bulk must not flood pane")
}

func TestRenderer_ToolResult_ErrorPayloadSurfaces(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventToolResult, ToolName: "bash", ToolResult: `{"success":false,"error":"permission denied"}`})
	content := r.Capture()
	assert.Contains(t, content, "permission denied", "failed tool calls must stay visible")
}

func TestRenderer_ToolResult_NonZeroExitSurfaces(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventToolResult, ToolName: "bash", ToolResult: `{"exit_code":2}`})
	content := r.Capture()
	assert.Contains(t, content, "exit=2")
}

func TestRenderer_Permission_ProducesLine(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventPermission, PermissionDescription: "run bash"})
	content := r.Capture()
	assert.Contains(t, content, "run bash")
}

func TestRenderer_SystemEvent_ProducesLine(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventSystem, Text: "started"})
	content := r.Capture()
	assert.Contains(t, content, "started")
}

func TestRenderer_SystemEvent_EmptyText_NoLine(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventSystem, Text: ""})
	assert.Equal(t, "", r.Capture())
}

func TestRenderer_TurnCompleted_FlushesPartial(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "partial"})
	r.AddEvent(Event{Kind: EventTurnCompleted})
	content := r.Capture()
	assert.Contains(t, content, "partial")
}

func TestRenderer_TurnInterrupted_FlushesPartial(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "partial"})
	r.AddEvent(Event{Kind: EventTurnInterrupted})
	content := r.Capture()
	assert.Contains(t, content, "partial")
}

func TestRenderer_CaptureRange_DashDash_ReturnsAll(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "a\nb\nc"})
	all := r.CaptureRange("-", "-")
	assert.Equal(t, r.Capture(), all)
}

func TestRenderer_CaptureRange_EmptyEmpty_ReturnsAll(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "a\nb\nc"})
	all := r.CaptureRange("", "")
	assert.Equal(t, r.Capture(), all)
}

func TestRenderer_CaptureRange_NumericSubset(t *testing.T) {
	r := NewRenderer()
	// Three lines: "a", "b", "c"
	r.AddEvent(Event{Kind: EventTextDelta, Text: "a\nb\nc"})
	// Force partial "c" to be committed as a line.
	r.AddEvent(Event{Kind: EventTurnCompleted})
	// Now lines are: ["a", "b", "c", ">"]
	// CaptureRange("0", "0") should return just "a"
	result := r.CaptureRange("0", "0")
	assert.Equal(t, "a", result)
}

func TestRenderer_CaptureRange_NegativeEnd(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "a\nb\nc"})
	r.AddEvent(Event{Kind: EventTurnCompleted})
	// Lines: ["a", "b", "c", ">"]
	// CaptureRange("-", "-1") should include all but the last line.
	result := r.CaptureRange("-", "-1")
	assert.NotContains(t, result, ">")
	assert.Contains(t, result, "a")
}

func TestRenderer_CaptureRange_OnEmptyRenderer(t *testing.T) {
	r := NewRenderer()
	assert.Equal(t, "", r.CaptureRange("-", "-"))
	assert.Equal(t, "", r.CaptureRange("0", "0"))
}

func TestRenderer_IsUpdated_ChangesWhenContentAdded(t *testing.T) {
	r := NewRenderer()
	hash1 := r.ContentHash()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "hello"})
	hash2 := r.ContentHash()
	assert.NotEqual(t, hash1, hash2)
}

func TestRenderer_IsUpdated_SameAfterNoChange(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "hello"})
	hash1 := r.ContentHash()
	hash2 := r.ContentHash()
	assert.Equal(t, hash1, hash2)
}

func TestRenderer_SequentialEvents_CorrectOrder(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "first\n"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "second"})
	content := r.Capture()
	firstIdx := indexOf(content, "first")
	secondIdx := indexOf(content, "second")
	assert.Less(t, firstIdx, secondIdx)
}

func indexOf(s, substr string) int {
	for i := range s {
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// rowKinds extracts the Kind from each PresentationRow.
func rowKinds(rows []PresentationRow) []PresentationRowKind {
	kinds := make([]PresentationRowKind, len(rows))
	for i, r := range rows {
		kinds[i] = r.Kind
	}
	return kinds
}

// --- CapturePresentation tests ---

func TestRenderer_CapturePresentation_Empty_ReturnsNil(t *testing.T) {
	r := NewRenderer()
	turns := r.CapturePresentation()
	assert.Nil(t, turns)
}

func TestRenderer_CapturePresentation_TurnStarted_CreatesTurn(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.Equal(t, "t1", turns[0].ID)
	assert.Equal(t, 1, turns[0].Number)
	assert.True(t, turns[0].Running())
}

func TestRenderer_CapturePresentation_TextDelta_ImplicitTurn(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, Text: "hello"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.Equal(t, 1, turns[0].Number)
	kinds := rowKinds(turns[0].Rows)
	assert.Contains(t, kinds, RowResponse)
	assert.Contains(t, kinds, RowProse)
}

func TestRenderer_CapturePresentation_TextDelta_ProseText(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "hello"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	var prose []string
	for _, row := range turns[0].Rows {
		if row.Kind == RowProse {
			prose = append(prose, row.Text)
		}
	}
	require.Len(t, prose, 1)
	assert.Equal(t, "hello", prose[0])
}

func TestRenderer_CapturePresentation_TextDelta_NewlineSplitsProseRows(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "line1\nline2"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	var prose []string
	for _, row := range turns[0].Rows {
		if row.Kind == RowProse {
			prose = append(prose, row.Text)
		}
	}
	require.Len(t, prose, 2)
	assert.Equal(t, "line1", prose[0])
	assert.Equal(t, "line2", prose[1])
}

func TestRenderer_CapturePresentation_TextDelta_FragmentsExtendLastProse(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "hel"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "lo"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	var prose []string
	for _, row := range turns[0].Rows {
		if row.Kind == RowProse {
			prose = append(prose, row.Text)
		}
	}
	require.Len(t, prose, 1)
	assert.Equal(t, "hello", prose[0])
}

func TestRenderer_CapturePresentation_ToolCall_AddsToolRow(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventToolCall, TurnID: "t1", ToolName: "bash", ToolInput: `{"command":"ls"}`})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	var toolRows []PresentationRow
	for _, row := range turns[0].Rows {
		if row.Kind == RowTool {
			toolRows = append(toolRows, row)
		}
	}
	require.Len(t, toolRows, 1)
	assert.Contains(t, toolRows[0].Text, "bash")
	assert.Equal(t, "bash", toolRows[0].ToolName)
	assert.Equal(t, 1, turns[0].ToolCount)
}

func TestRenderer_CapturePresentation_ToolResult_IsError(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventToolResult, TurnID: "t1", ToolResult: `{"success":false,"error":"denied"}`})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	var resultRows []PresentationRow
	for _, row := range turns[0].Rows {
		if row.Kind == RowResult {
			resultRows = append(resultRows, row)
		}
	}
	require.Len(t, resultRows, 1)
	assert.True(t, resultRows[0].IsError)
}

func TestRenderer_CapturePresentation_ToolResult_NotError(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventToolResult, TurnID: "t1", ToolResult: "ok"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	var resultRows []PresentationRow
	for _, row := range turns[0].Rows {
		if row.Kind == RowResult {
			resultRows = append(resultRows, row)
		}
	}
	require.Len(t, resultRows, 1)
	assert.False(t, resultRows[0].IsError)
}

func TestRenderer_CapturePresentation_Permission_AddsRow(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventPermission, TurnID: "t1", PermissionDescription: "run bash"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	var permRows []PresentationRow
	for _, row := range turns[0].Rows {
		if row.Kind == RowPermission {
			permRows = append(permRows, row)
		}
	}
	require.Len(t, permRows, 1)
	assert.Contains(t, permRows[0].Text, "run bash")
}

func TestRenderer_CapturePresentation_Permission_BeforeTurnStarted_ImplicitTurn(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventPermission, PermissionDescription: "allow"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	var permRows int
	for _, row := range turns[0].Rows {
		if row.Kind == RowPermission {
			permRows++
		}
	}
	assert.Equal(t, 1, permRows)
}

func TestRenderer_CapturePresentation_System_AddsRow(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventSystem, Text: "agent started"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	var sysRows []PresentationRow
	for _, row := range turns[0].Rows {
		if row.Kind == RowSystem {
			sysRows = append(sysRows, row)
		}
	}
	require.Len(t, sysRows, 1)
	assert.Contains(t, sysRows[0].Text, "agent started")
}

func TestRenderer_CapturePresentation_System_EmptyText_NoRow(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventSystem, Text: ""})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	for _, row := range turns[0].Rows {
		assert.NotEqual(t, RowSystem, row.Kind)
	}
}

func TestRenderer_CapturePresentation_TurnCompleted_ClosesTurn(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "done"})
	r.AddEvent(Event{Kind: EventTurnCompleted, TurnID: "t1"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.False(t, turns[0].Running())
	assert.False(t, turns[0].Interrupted)
	for _, row := range turns[0].Rows {
		assert.NotEqual(t, RowStatus, row.Kind, "normal completion must not add RowStatus row")
	}
}

func TestRenderer_CapturePresentation_TurnInterrupted_SetsInterrupted(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventTurnInterrupted, TurnID: "t1"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.True(t, turns[0].Interrupted)
	var statusRows int
	for _, row := range turns[0].Rows {
		if row.Kind == RowStatus {
			statusRows++
		}
	}
	assert.Equal(t, 1, statusRows)
}

func TestRenderer_CapturePresentation_FinalSystem_ClosesTurn(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventSystem, Text: "error", Final: true})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.False(t, turns[0].Running(), "final system event must close the current turn")
	assert.False(t, turns[0].Interrupted, "final system close must not mark turn as interrupted")
}

func TestRenderer_CapturePresentation_NewTurnStarted_ClosesExistingAsInterrupted(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "partial"})
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t2"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 2)
	assert.True(t, turns[0].Interrupted, "first turn must be marked interrupted when second starts")
	assert.True(t, turns[1].Running(), "second turn must be running")
}

func TestRenderer_CapturePresentation_DeepCopy_Safe(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "hello"})

	turns1 := r.CapturePresentation()
	// Mutate returned copy — must not affect internal state.
	turns1[0].Rows = nil

	turns2 := r.CapturePresentation()
	require.Len(t, turns2, 1)
	assert.NotEmpty(t, turns2[0].Rows, "internal data must not be affected by mutations to returned copy")
}

func TestRenderer_CapturePresentation_FlatPathUnchanged(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "hello\nworld"})
	r.AddEvent(Event{Kind: EventToolCall, ToolName: "bash", ToolInput: `{"command":"ls"}`})
	r.AddEvent(Event{Kind: EventTurnCompleted})

	content := r.Capture()
	assert.Contains(t, content, "hello")
	assert.Contains(t, content, "world")
	assert.Contains(t, content, "bash")
}

func TestRenderer_CapturePresentation_CompletedTurnProseRetained(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "final prose"})
	r.AddEvent(Event{Kind: EventTurnCompleted})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	var prose []string
	for _, row := range turns[0].Rows {
		if row.Kind == RowProse {
			prose = append(prose, row.Text)
		}
	}
	assert.NotEmpty(t, prose, "completed turn must retain prose rows")
}
