package sdk

import (
	"fmt"
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

func TestRenderer_ToolResult_ContentBlocksSurfacesTextSummary(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{
		Kind:       EventToolResult,
		ToolName:   "grep",
		ToolResult: `{"content":[{"type":"text","text":"match one\nmatch two"}]}`,
	})
	content := r.Capture()
	assert.Contains(t, content, "match one", "content[] text blocks must contribute a visible result summary")
}

func TestRenderer_ToolResult_GrepSummaryUsesMatchCount(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{
		Kind:       EventToolResult,
		ToolName:   "grep",
		ToolResult: `{"matches":[{"file":"app/main.go","line":12,"text":"first match"}],"total":1}`,
	})
	content := r.Capture()
	assert.Contains(t, content, "1 match")
}

func TestRenderer_ToolResult_GrepEmptySummaryUsesZeroMatches(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{
		Kind:       EventToolResult,
		ToolName:   "grep",
		ToolResult: `{"matches":[],"total":0}`,
	})
	content := r.Capture()
	assert.Contains(t, content, "0 matches")
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

func TestRenderer_CapturePresentation_TextDelta_ImplicitTurnUsesTurnID(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTextDelta, TurnID: "t1", Text: "hello"})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.Equal(t, "t1", turns[0].ID)
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

func TestRenderer_CapturePresentation_TextDelta_TrailingNewlineDoesNotAddEmptyRow(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventTextDelta, TurnID: "t1", Text: "line1\n"})
	r.AddEvent(Event{Kind: EventTextDelta, TurnID: "t1", Text: "line2"})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)

	var prose []string
	for _, row := range turns[0].Rows {
		if row.Kind == RowProse {
			prose = append(prose, row.Text)
		}
	}

	require.Len(t, prose, 2)
	assert.Equal(t, []string{"line1", "line2"}, prose)
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

func TestRenderer_CapturePresentation_UserPrompt_AddsUserRow(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventUserPrompt, Text: "show logs"})
	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	require.Len(t, turns[0].Rows, 1)
	assert.Equal(t, RowUser, turns[0].Rows[0].Kind)
	assert.Equal(t, "show logs", turns[0].Rows[0].Text)
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

func TestRenderer_ToolCall_CommandExecution_SanitizesDisplay(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{
		Kind:      EventToolCall,
		ToolName:  "commandExecution",
		ToolInput: "/usr/bin/ls -la",
		Timestamp: time.Now(),
	})

	content := r.Capture()
	assert.Contains(t, content, "• ls -la")
	assert.NotContains(t, content, "commandExecution")
	assert.NotContains(t, content, "/usr/bin/ls")
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

func TestRenderer_CapturePresentation_PermissionBeforeTurnStarted_KeepsSingleTurn(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventPermission, TurnID: "t1", PermissionDescription: "allow"})
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventTextDelta, TurnID: "t1", Text: "hello"})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.Equal(t, "t1", turns[0].ID)
	assert.False(t, turns[0].Interrupted)

	var statusRows int
	for _, row := range turns[0].Rows {
		if row.Kind == RowStatus {
			statusRows++
		}
	}
	assert.Equal(t, 0, statusRows)
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

func TestRenderer_CapturePresentation_SystemOutsideTurn_Preserved(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventSystem, Text: "transport failed"})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.False(t, turns[0].Running())

	var sysRows []PresentationRow
	for _, row := range turns[0].Rows {
		if row.Kind == RowSystem {
			sysRows = append(sysRows, row)
		}
	}
	require.Len(t, sysRows, 1)
	assert.Equal(t, "[system: transport failed]", sysRows[0].Text)
}

func TestRenderer_CapturePresentation_Warning_AddsRow(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	r.AddEvent(Event{Kind: EventWarning, Text: "mcp server startup is slow"})

	assert.Equal(t, "[warning: mcp server startup is slow]", r.Capture())

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	var warningRows []PresentationRow
	for _, row := range turns[0].Rows {
		if row.Kind == RowWarning {
			warningRows = append(warningRows, row)
		}
	}
	require.Len(t, warningRows, 1)
	assert.Equal(t, "[warning: mcp server startup is slow]", warningRows[0].Text)
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

func TestRenderer_CapturePresentation_ProseAfterToolStartsNewResponseBlock(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()

	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{Kind: EventTextDelta, TurnID: "t1", Text: "using architect first", Timestamp: ts})
	r.AddEvent(Event{Kind: EventToolCall, TurnID: "t1", ToolName: "read_file", ToolInput: `{"path":"CLAUDE.md"}`, Timestamp: ts})
	r.AddEvent(Event{Kind: EventToolResult, TurnID: "t1", ToolName: "read_file", ToolResult: `{"content":"line1\nline2"}`, Timestamp: ts})
	r.AddEvent(Event{Kind: EventTextDelta, TurnID: "t1", Text: "next i'm reading overlay code", Timestamp: ts})
	r.AddEvent(Event{Kind: EventTurnCompleted, TurnID: "t1", Timestamp: ts})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	// RowToolPreview is now inserted after RowResult for non-error textual results.
	require.Equal(t, []PresentationRowKind{
		RowResponse,
		RowProse,
		RowTool,
		RowResult,
		RowToolPreview,
		RowResponse,
		RowProse,
	}, rowKinds(turns[0].Rows))

	assert.Equal(t, "using architect first", turns[0].Rows[1].Text)
	assert.Equal(t, "next i'm reading overlay code", turns[0].Rows[6].Text)
}

// --- HeaderText tests ---

func TestRenderer_HeaderText_Running_AppendsBulletRunning(t *testing.T) {
	turn := &PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: time.Now().Add(-5 * time.Second),
	}
	assert.True(t, turn.Running())
	header := turn.HeaderText(time.Now())
	assert.Contains(t, header, "• running", "running turn header must contain '• running'")
	assert.Contains(t, header, "turn 1", "header must include turn number")
}

func TestRenderer_HeaderText_Interrupted_NoRunningLabel(t *testing.T) {
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   time.Now().Add(-5 * time.Second),
		Interrupted: true,
	}
	assert.False(t, turn.Running())
	header := turn.HeaderText(time.Now())
	assert.NotContains(t, header, "• running", "interrupted turn must not show running label")
}

func TestRenderer_HeaderText_Completed_NoRunningLabel(t *testing.T) {
	now := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      2,
		StartedAt:   now.Add(-10 * time.Second),
		CompletedAt: now,
	}
	assert.False(t, turn.Running())
	header := turn.HeaderText(now)
	assert.NotContains(t, header, "• running", "completed turn must not show running label")
	assert.Contains(t, header, "turn 2")
}

func TestRenderer_HeaderText_ToolCount_Singular(t *testing.T) {
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   time.Now().Add(-3 * time.Second),
		CompletedAt: time.Now(),
		ToolCount:   1,
	}
	header := turn.HeaderText(time.Now())
	assert.Contains(t, header, "1 tool", "singular tool count must appear in header")
	assert.NotContains(t, header, "1 tools")
}

func TestRenderer_HeaderText_ToolCount_Plural(t *testing.T) {
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   time.Now().Add(-3 * time.Second),
		CompletedAt: time.Now(),
		ToolCount:   5,
	}
	header := turn.HeaderText(time.Now())
	assert.Contains(t, header, "5 tools", "plural tool count must appear in header")
}

// --- Thinking row injection tests ---

func TestRenderer_ThinkingRow_InjectedWhenRunningAndEmpty(t *testing.T) {
	r := NewRenderer()
	past := time.Now().Add(-3 * time.Second) // 3s ago — past 2s threshold
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: past})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.True(t, turns[0].Running())

	var thinkingRows []PresentationRow
	for _, row := range turns[0].Rows {
		if row.Kind == RowThinking {
			thinkingRows = append(thinkingRows, row)
		}
	}
	require.Len(t, thinkingRows, 1, "running empty turn past threshold must have a thinking row")
	assert.Contains(t, thinkingRows[0].Text, "thinking")
}

func TestRenderer_ThinkingRow_NotInjectedBelowThreshold(t *testing.T) {
	r := NewRenderer()
	recent := time.Now().Add(-500 * time.Millisecond) // 0.5s — under 2s threshold
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: recent})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	for _, row := range turns[0].Rows {
		assert.NotEqual(t, RowThinking, row.Kind, "thinking row must not appear below threshold")
	}
}

func TestRenderer_ThinkingRow_NotInjectedWithToolContent(t *testing.T) {
	r := NewRenderer()
	past := time.Now().Add(-5 * time.Second)
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: past})
	r.AddEvent(Event{Kind: EventToolCall, TurnID: "t1", ToolName: "bash", ToolInput: "{}", Timestamp: past})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	for _, row := range turns[0].Rows {
		assert.NotEqual(t, RowThinking, row.Kind, "thinking row must not appear when turn has tool content")
	}
}

func TestRenderer_ThinkingRow_NotInjectedForCompletedTurn(t *testing.T) {
	r := NewRenderer()
	past := time.Now().Add(-5 * time.Second)
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: past})
	r.AddEvent(Event{Kind: EventTurnCompleted, TurnID: "t1", Timestamp: time.Now()})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.False(t, turns[0].Running())
	for _, row := range turns[0].Rows {
		assert.NotEqual(t, RowThinking, row.Kind, "thinking row must not appear for completed turn")
	}
}

func TestRenderer_ThinkingRow_DisappearsWhenProseArrives(t *testing.T) {
	r := NewRenderer()
	past := time.Now().Add(-3 * time.Second)
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: past})

	// Before prose: thinking row should be present.
	turns1 := r.CapturePresentation()
	require.Len(t, turns1, 1)
	hasThinking := false
	for _, row := range turns1[0].Rows {
		if row.Kind == RowThinking {
			hasThinking = true
		}
	}
	assert.True(t, hasThinking, "thinking row must appear before prose arrives")

	// After prose arrives: no thinking row.
	r.AddEvent(Event{Kind: EventTextDelta, TurnID: "t1", Text: "hello", Timestamp: time.Now()})
	turns2 := r.CapturePresentation()
	require.Len(t, turns2, 1)
	for _, row := range turns2[0].Rows {
		assert.NotEqual(t, RowThinking, row.Kind, "thinking row must not appear once prose has arrived")
	}
}

// --- Cross-transport contract tests ---

// TestRenderer_ClaudeTransportContract tests the renderer against a synthetic
// event stream that mirrors what ClaudeTransport emits for a typical turn:
// TurnStarted → TextDelta → ToolCall → ToolResult → TextDelta → TurnCompleted.
func TestRenderer_ClaudeTransportContract(t *testing.T) {
	r := NewRenderer()
	turnID := "claude-turn-1"
	ts := time.Now()

	events := []Event{
		{Kind: EventTurnStarted, TurnID: turnID, Timestamp: ts},
		{Kind: EventTextDelta, TurnID: turnID, Text: "Let me check that file.", Timestamp: ts},
		{Kind: EventToolCall, TurnID: turnID, ToolName: "read_file", ToolInput: `{"path":"/foo/bar.go"}`, Timestamp: ts},
		{Kind: EventToolResult, TurnID: turnID, ToolName: "read_file", ToolResult: `{"content":"package main"}`, Timestamp: ts},
		{Kind: EventTextDelta, TurnID: turnID, Text: " Done.", Timestamp: ts},
		{Kind: EventTurnCompleted, TurnID: turnID, Timestamp: ts},
	}
	for _, e := range events {
		r.AddEvent(e)
	}

	turns := r.CapturePresentation()
	require.Len(t, turns, 1, "claude turn stream must produce exactly one turn")

	turn := turns[0]
	assert.Equal(t, turnID, turn.ID)
	assert.False(t, turn.Running(), "turn must be closed after TurnCompleted")
	assert.False(t, turn.Interrupted, "turn must not be interrupted after TurnCompleted")
	assert.Equal(t, 1, turn.ToolCount)

	kinds := rowKinds(turn.Rows)
	assert.Contains(t, kinds, RowResponse, "claude turn must contain RowResponse sentinel")
	assert.Contains(t, kinds, RowTool, "claude turn must contain RowTool")
	assert.Contains(t, kinds, RowResult, "claude turn must contain RowResult")
	assert.Contains(t, kinds, RowProse, "claude turn must contain RowProse")
}

// TestRenderer_CodexTransportContract tests the renderer against a synthetic
// event stream that mirrors what CodexTransport emits: tool items arrive before
// agent prose delta, unlike Claude's interleaved pattern.
func TestRenderer_CodexTransportContract(t *testing.T) {
	r := NewRenderer()
	turnID := "codex-turn-1"
	ts := time.Now()

	// Codex: tool items come first, then prose delta.
	events := []Event{
		{Kind: EventTurnStarted, TurnID: turnID, Timestamp: ts},
		{Kind: EventToolCall, TurnID: turnID, ToolName: "shell", ToolInput: `{"command":"ls"}`, Timestamp: ts},
		{Kind: EventToolResult, TurnID: turnID, ToolName: "shell", ToolResult: "main.go\ngo.mod", Timestamp: ts},
		{Kind: EventTextDelta, TurnID: turnID, Text: "Found the files.", Timestamp: ts},
		{Kind: EventTurnCompleted, TurnID: turnID, Timestamp: ts},
	}
	for _, e := range events {
		r.AddEvent(e)
	}

	turns := r.CapturePresentation()
	require.Len(t, turns, 1, "codex turn stream must produce exactly one turn")

	turn := turns[0]
	assert.Equal(t, turnID, turn.ID)
	assert.False(t, turn.Running(), "turn must be closed after TurnCompleted")
	assert.False(t, turn.Interrupted, "turn must not be interrupted")
	assert.Equal(t, 1, turn.ToolCount)

	// Tool row must appear before RowResponse sentinel in codex streams.
	toolIdx, responseIdx := -1, -1
	for i, row := range turn.Rows {
		if row.Kind == RowTool && toolIdx < 0 {
			toolIdx = i
		}
		if row.Kind == RowResponse && responseIdx < 0 {
			responseIdx = i
		}
	}
	require.True(t, toolIdx >= 0, "codex turn must contain RowTool")
	require.True(t, responseIdx >= 0, "codex turn with prose must contain RowResponse sentinel")
	assert.Less(t, toolIdx, responseIdx, "tool rows must appear before RowResponse in codex stream order")
}

// TestRenderer_CodexTransportContract_SystemErrorClosesActiveTurn verifies the
// codex-specific error path where a final system event closes the active turn
// without leaving it marked running forever, even when the turn had tool content.
func TestRenderer_CodexTransportContract_SystemErrorClosesActiveTurn(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "codex-t1", Timestamp: ts})
	r.AddEvent(Event{Kind: EventToolCall, TurnID: "codex-t1", ToolName: "shell", ToolInput: `{"command":"ls"}`, Timestamp: ts})
	// Codex emits a final system event on error (e.g., context limit, server error).
	r.AddEvent(Event{Kind: EventSystem, Text: "context length exceeded", Final: true, Timestamp: ts})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.False(t, turns[0].Running(), "codex final system error must close the active turn (not leave it running)")
	assert.False(t, turns[0].Interrupted, "codex final system close must not mark turn as interrupted")
}

// TestRenderer_ClaudeTransportContract_MultiTurn verifies that multiple sequential
// Claude turns produce distinct PresentationTurns in the correct order.
func TestRenderer_ClaudeTransportContract_MultiTurn(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()

	for i, turnID := range []string{"claude-t1", "claude-t2"} {
		r.AddEvent(Event{Kind: EventTurnStarted, TurnID: turnID, Timestamp: ts})
		r.AddEvent(Event{Kind: EventTextDelta, TurnID: turnID, Text: fmt.Sprintf("response %d", i+1), Timestamp: ts})
		r.AddEvent(Event{Kind: EventTurnCompleted, TurnID: turnID, Timestamp: ts})
	}

	turns := r.CapturePresentation()
	require.Len(t, turns, 2, "two claude turns must produce two PresentationTurns")
	assert.Equal(t, "claude-t1", turns[0].ID)
	assert.Equal(t, "claude-t2", turns[1].ID)
	assert.Equal(t, 1, turns[0].Number)
	assert.Equal(t, 2, turns[1].Number)
	assert.False(t, turns[0].Running())
	assert.False(t, turns[1].Running())
}

// --- RowToolDiff and RowToolPreview injection tests ---

func TestRenderer_ToolCall_Edit_InjectsToolDiffRow(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:      EventToolCall,
		TurnID:    "t1",
		ToolName:  "Edit",
		ToolInput: `{"path":"main.go","old_string":"hello","new_string":"world"}`,
		Timestamp: ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)

	kinds := rowKinds(turns[0].Rows)
	toolIdx := -1
	for i, k := range kinds {
		if k == RowTool {
			toolIdx = i
			break
		}
	}
	require.True(t, toolIdx >= 0, "must have a RowTool row")
	require.True(t, toolIdx+1 < len(kinds), "must have a row after RowTool")
	assert.Equal(t, RowToolDiff, kinds[toolIdx+1], "RowToolDiff must immediately follow RowTool")

	diffRow := turns[0].Rows[toolIdx+1]
	require.NotNil(t, diffRow.ToolDiff)
	assert.Equal(t, "main.go", diffRow.ToolDiff.Path)
}

func TestRenderer_ToolCall_NonDiffTool_NoToolDiffRow(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:      EventToolCall,
		TurnID:    "t1",
		ToolName:  "bash",
		ToolInput: `{"command":"ls"}`,
		Timestamp: ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	for _, row := range turns[0].Rows {
		assert.NotEqual(t, RowToolDiff, row.Kind, "non-diff tool must not produce RowToolDiff")
	}
}

func TestRenderer_ToolResult_NonError_InjectsToolPreviewRow(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:       EventToolResult,
		TurnID:     "t1",
		ToolName:   "read_file",
		ToolResult: `{"content":"line1\nline2"}`,
		Timestamp:  ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)

	kinds := rowKinds(turns[0].Rows)
	resultIdx := -1
	for i, k := range kinds {
		if k == RowResult {
			resultIdx = i
			break
		}
	}
	require.True(t, resultIdx >= 0, "must have a RowResult row")
	require.True(t, resultIdx+1 < len(kinds), "must have a row after RowResult")
	assert.Equal(t, RowToolPreview, kinds[resultIdx+1], "RowToolPreview must immediately follow RowResult")

	previewRow := turns[0].Rows[resultIdx+1]
	require.NotNil(t, previewRow.ToolPreview)
	assert.Equal(t, []string{"line1", "line2"}, previewRow.ToolPreview.Lines)
}

func TestRenderer_ToolResult_ContentBlocks_InjectsToolPreviewRow(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:       EventToolResult,
		TurnID:     "t1",
		ToolName:   "grep",
		ToolResult: `{"content":[{"type":"text","text":"match one"},{"type":"text","text":"match two"}]}`,
		Timestamp:  ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)

	kinds := rowKinds(turns[0].Rows)
	resultIdx := -1
	for i, k := range kinds {
		if k == RowResult {
			resultIdx = i
			break
		}
	}
	require.True(t, resultIdx >= 0, "must have a RowResult row")
	require.True(t, resultIdx+1 < len(kinds), "must have a row after RowResult")
	assert.Equal(t, RowToolPreview, kinds[resultIdx+1], "RowToolPreview must immediately follow RowResult")

	previewRow := turns[0].Rows[resultIdx+1]
	require.NotNil(t, previewRow.ToolPreview)
	assert.Equal(t, []string{"match one", "match two"}, previewRow.ToolPreview.Lines)
}

func TestRenderer_ToolResult_GrepMatches_InjectsToolPreviewRow(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:       EventToolResult,
		TurnID:     "t1",
		ToolName:   "grep",
		ToolResult: `{"matches":[{"file":"app/main.go","line":12,"text":"first match"},{"file":"app/input.go","line":27,"text":"second match"}],"total":2}`,
		Timestamp:  ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)

	kinds := rowKinds(turns[0].Rows)
	resultIdx := -1
	for i, k := range kinds {
		if k == RowResult {
			resultIdx = i
			break
		}
	}
	require.True(t, resultIdx >= 0, "must have a RowResult row")
	require.True(t, resultIdx+1 < len(kinds), "must have a row after RowResult")
	assert.Equal(t, RowToolPreview, kinds[resultIdx+1], "RowToolPreview must immediately follow RowResult")

	previewRow := turns[0].Rows[resultIdx+1]
	require.NotNil(t, previewRow.ToolPreview)
	assert.Equal(t, []string{"app/main.go:12: first match", "app/input.go:27: second match"}, previewRow.ToolPreview.Lines)
}

func TestRenderer_ToolResult_PreviewRowsCappedToFiveLines(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}

	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:       EventToolResult,
		TurnID:     "t1",
		ToolName:   "bash",
		ToolResult: strings.Join(lines, "\n"),
		Timestamp:  ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)

	var previewRow *PresentationRow
	for i := range turns[0].Rows {
		if turns[0].Rows[i].Kind == RowToolPreview {
			previewRow = &turns[0].Rows[i]
			break
		}
	}

	require.NotNil(t, previewRow, "must include a RowToolPreview row")
	require.NotNil(t, previewRow.ToolPreview)
	assert.Len(t, previewRow.ToolPreview.Lines, toolPreviewMaxLines)
	assert.Equal(t, "line 1", previewRow.ToolPreview.Lines[0])
	assert.Equal(t, fmt.Sprintf("line %d", toolPreviewMaxLines), previewRow.ToolPreview.Lines[toolPreviewMaxLines-1])
	assert.True(t, previewRow.ToolPreview.Truncated)
	assert.Equal(t, len(lines)-toolPreviewMaxLines, previewRow.ToolPreview.HiddenLineCount)
}

func TestRenderer_ToolResult_SingleLineSummary_DoesNotInjectDuplicateToolPreviewRow(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:       EventToolResult,
		TurnID:     "t1",
		ToolName:   "git_diff",
		ToolResult: "no changes",
		Timestamp:  ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)

	kinds := rowKinds(turns[0].Rows)
	assert.Equal(t, []PresentationRowKind{RowResult}, kinds)
	assert.Equal(t, "→ no changes", turns[0].Rows[0].Text)
}

func TestRenderer_ToolResult_ExitCodeZero_DoesNotInjectDuplicateToolPreviewRow(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:       EventToolResult,
		TurnID:     "t1",
		ToolName:   "commandExecution",
		ToolResult: `{"exit_code":0,"output":"tests passed"}`,
		Timestamp:  ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)

	kinds := rowKinds(turns[0].Rows)
	assert.Equal(t, []PresentationRowKind{RowResult}, kinds,
		"successful commandExecution should produce a single RowResult, not a duplicate RowToolPreview")
	assert.Equal(t, "→ tests passed", turns[0].Rows[0].Text)
	assert.Equal(t, "tests passed", turns[0].Rows[0].Output)
}

func TestRenderer_ToolResult_ExitCodeZero_NormalizesMultilineCommandOutput(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:       EventToolResult,
		TurnID:     "t1",
		ToolName:   "commandExecution",
		ToolResult: "{\"exit_code\":0,\"output\":\"tests passed\\n\\nwith warnings\"}",
		Timestamp:  ts,
	})

	assert.Equal(t, "→ tests passed with warnings", r.Capture())

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)

	kinds := rowKinds(turns[0].Rows)
	assert.Equal(t, []PresentationRowKind{RowResult}, kinds,
		"normalized commandExecution output should not produce a duplicate RowToolPreview")
	assert.Equal(t, "→ tests passed with warnings", turns[0].Rows[0].Text)
	assert.Equal(t, "tests passed with warnings", turns[0].Rows[0].Output)
}

func TestRenderer_ToolResult_ExitCodeZero_CapsStructuredCommandOutput(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	longOutput := strings.Repeat("x", 200)
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:       EventToolResult,
		TurnID:     "t1",
		ToolName:   "commandExecution",
		ToolResult: fmt.Sprintf(`{"exit_code":0,"output":"%s"}`, longOutput),
		Timestamp:  ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	require.Len(t, turns[0].Rows, 1)

	assert.Equal(t,
		truncateOneLine(longOutput, commandResultOutputMaxRunes),
		turns[0].Rows[0].Output,
		"structured commandExecution output must be capped before it is stored in the presentation model")
}

func TestRenderer_ToolResult_Error_NoToolPreviewRow(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:       EventToolResult,
		TurnID:     "t1",
		ToolName:   "bash",
		ToolResult: `{"success":false,"error":"denied"}`,
		Timestamp:  ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	for _, row := range turns[0].Rows {
		assert.NotEqual(t, RowToolPreview, row.Kind, "error result must not produce RowToolPreview")
	}
}

func TestRenderer_ToolResult_DiffTool_NoToolPreviewRow(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:      EventToolCall,
		TurnID:    "t1",
		ToolName:  "Edit",
		ToolInput: `{"path":"a.go","old_string":"old","new_string":"new"}`,
		Timestamp: ts,
	})
	r.AddEvent(Event{
		Kind:       EventToolResult,
		TurnID:     "t1",
		ToolName:   "Edit",
		ToolResult: `{"content":"result"}`,
		Timestamp:  ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	for _, row := range turns[0].Rows {
		assert.NotEqual(t, RowToolPreview, row.Kind, "Edit result must not produce RowToolPreview")
	}
}

func TestRenderer_CapturePresentation_DeepCopy_ToolDiffAndPreview(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:      EventToolCall,
		TurnID:    "t1",
		ToolName:  "Edit",
		ToolInput: `{"path":"x.go","old_string":"old","new_string":"new"}`,
		Timestamp: ts,
	})
	r.AddEvent(Event{
		Kind:       EventToolResult,
		TurnID:     "t1",
		ToolName:   "read_file",
		ToolResult: `{"content":"hello"}`,
		Timestamp:  ts,
	})

	copy1 := r.CapturePresentation()
	require.Len(t, copy1, 1)

	// Find and mutate ToolDiff in copy1.
	for _, row := range copy1[0].Rows {
		if row.ToolDiff != nil {
			row.ToolDiff.Path = "mutated"
		}
		if row.ToolPreview != nil && len(row.ToolPreview.Lines) > 0 {
			row.ToolPreview.Lines[0] = "mutated"
		}
	}

	// Second capture must not see mutations.
	copy2 := r.CapturePresentation()
	require.Len(t, copy2, 1)
	for _, row := range copy2[0].Rows {
		if row.ToolDiff != nil {
			assert.NotEqual(t, "mutated", row.ToolDiff.Path, "ToolDiff in second copy must not be affected by mutation of first copy")
		}
		if row.ToolPreview != nil && len(row.ToolPreview.Lines) > 0 {
			assert.NotEqual(t, "mutated", row.ToolPreview.Lines[0], "ToolPreview in second copy must not be affected by mutation of first copy")
		}
	}
}

// --- Activity derivation tests ---

func TestDeriveTurnActivity_UnresolvedTool(t *testing.T) {
	ts := time.Now()
	turn := &PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: ts,
		Rows: []PresentationRow{
			{Kind: RowTool, Text: "• read_file main.go", ToolName: "read_file", Timestamp: ts},
		},
	}
	act := deriveTurnActivity(turn, ts)
	require.NotNil(t, act)
	assert.Equal(t, "tool", act.Kind)
	assert.Equal(t, "read_file main.go", act.Label)
}

func TestDeriveTurnActivity_ResolvedTool_NoActivity(t *testing.T) {
	ts := time.Now()
	turn := &PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: ts,
		Rows: []PresentationRow{
			{Kind: RowTool, Text: "• bash ls", ToolName: "bash", Timestamp: ts},
			{Kind: RowResult, Text: "→ ok", Timestamp: ts},
		},
	}
	act := deriveTurnActivity(turn, ts)
	// Tool is resolved — should fall through to "working".
	require.NotNil(t, act)
	assert.Equal(t, "working", act.Kind)
}

func TestDeriveTurnActivity_Permission(t *testing.T) {
	ts := time.Now()
	turn := &PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: ts,
		Rows: []PresentationRow{
			{Kind: RowTool, Text: "• bash", ToolName: "bash", Timestamp: ts},
			{Kind: RowResult, Text: "→ ok", Timestamp: ts},
			{Kind: RowPermission, Text: "[permission: run bash]", Timestamp: ts},
		},
	}
	act := deriveTurnActivity(turn, ts)
	require.NotNil(t, act)
	assert.Equal(t, "permission", act.Kind)
	assert.Equal(t, "permission requested", act.Label)
}

func TestDeriveTurnActivity_NoRows_Thinking(t *testing.T) {
	ts := time.Now()
	turn := &PresentationTurn{
		ID:        "t1",
		Number:    1,
		StartedAt: ts,
	}
	act := deriveTurnActivity(turn, ts)
	require.NotNil(t, act)
	assert.Equal(t, "thinking", act.Kind)
	assert.Equal(t, "thinking", act.Label)
}

func TestDeriveTurnActivity_CompletedTurn_ReturnsNil(t *testing.T) {
	ts := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   ts,
		CompletedAt: ts,
	}
	act := deriveTurnActivity(turn, ts)
	assert.Nil(t, act)
}

func TestDeriveTurnActivity_InterruptedTurn_ReturnsNil(t *testing.T) {
	ts := time.Now()
	turn := &PresentationTurn{
		ID:          "t1",
		Number:      1,
		StartedAt:   ts,
		Interrupted: true,
	}
	act := deriveTurnActivity(turn, ts)
	assert.Nil(t, act)
}

func TestRenderer_CapturePresentation_ActivitySetOnRunningTurn(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{
		Kind:      EventToolCall,
		TurnID:    "t1",
		ToolName:  "bash",
		ToolInput: `{"command":"ls"}`,
		Timestamp: ts,
	})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.True(t, turns[0].Running())
	require.NotNil(t, turns[0].Activity)
	assert.Equal(t, "tool", turns[0].Activity.Kind)
}

func TestRenderer_CapturePresentation_ActivityNilOnCompletedTurn(t *testing.T) {
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{Kind: EventTurnCompleted, TurnID: "t1", Timestamp: ts})

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	assert.False(t, turns[0].Running())
	assert.Nil(t, turns[0].Activity)
}

func TestRenderer_CapturePresentation_ActivityNotStoredInternally(t *testing.T) {
	// Activity must be set only on the deep copy, not the internal turn.
	r := NewRenderer()
	ts := time.Now()
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})

	turns1 := r.CapturePresentation()
	// Mutate the activity on the copy.
	if turns1[0].Activity != nil {
		turns1[0].Activity.Kind = "mutated"
	}

	turns2 := r.CapturePresentation()
	require.Len(t, turns2, 1)
	if turns2[0].Activity != nil {
		assert.NotEqual(t, "mutated", turns2[0].Activity.Kind)
	}
}
