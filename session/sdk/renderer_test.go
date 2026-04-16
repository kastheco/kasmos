package sdk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestRenderer_ToolResult_ProducesLine(t *testing.T) {
	r := NewRenderer()
	r.AddEvent(Event{Kind: EventToolResult, ToolName: "bash", ToolResult: "ok"})
	content := r.Capture()
	assert.NotEmpty(t, content)
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
