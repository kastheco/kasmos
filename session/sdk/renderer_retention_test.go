package sdk

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- helpers ----

func noRetention() RendererRetentionOptions {
	return RendererRetentionOptions{MaxBytes: 0, MaxTurns: 0}
}

func bytesOnly(max int64) RendererRetentionOptions {
	return RendererRetentionOptions{MaxBytes: max, MaxTurns: 0}
}

func turnsOnly(max int64) RendererRetentionOptions {
	return RendererRetentionOptions{MaxTurns: max, MaxBytes: 0}
}

func completeTurn(r *Renderer, id string) {
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: id})
	r.AddEvent(Event{Kind: EventTextDelta, TurnID: id, Text: "hello"})
	r.AddEvent(Event{Kind: EventTurnCompleted, TurnID: id})
}

// ---- constants and defaults ----

func TestRendererRetention_Defaults(t *testing.T) {
	r := NewRenderer()
	s := r.Stats()
	assert.Equal(t, DefaultTranscriptMaxBytes, s.MaxBytes)
	assert.Equal(t, DefaultTranscriptMaxTurns, s.MaxTurns)
}

func TestRendererRetention_WithOption_OverridesDefaults(t *testing.T) {
	r := NewRenderer(WithRendererRetention(RendererRetentionOptions{
		MaxBytes: 1024,
		MaxTurns: 10,
		Name:     "myinst",
	}))
	s := r.Stats()
	assert.Equal(t, int64(1024), s.MaxBytes)
	assert.Equal(t, int64(10), s.MaxTurns)
}

func TestRendererRetention_NegativeOptionsNormalizedToZero(t *testing.T) {
	r := NewRenderer(WithRendererRetention(RendererRetentionOptions{
		MaxBytes: -100,
		MaxTurns: -5,
	}))
	s := r.Stats()
	assert.Equal(t, int64(0), s.MaxBytes)
	assert.Equal(t, int64(0), s.MaxTurns)
}

func TestRendererRetention_SetRetention_NegativeNormalized(t *testing.T) {
	r := NewRenderer()
	r.SetRetention(RendererRetentionOptions{MaxBytes: -1, MaxTurns: -99})
	s := r.Stats()
	assert.Equal(t, int64(0), s.MaxBytes)
	assert.Equal(t, int64(0), s.MaxTurns)
}

// ---- stats math ----

func TestRendererRetention_Stats_BytesIncrease(t *testing.T) {
	r := NewRenderer(WithRendererRetention(noRetention()))
	s0 := r.Stats()
	assert.Equal(t, int64(0), s0.Bytes)

	r.AddEvent(Event{Kind: EventTextDelta, Text: "hello\n"})
	s1 := r.Stats()
	assert.Greater(t, s1.Bytes, int64(0), "bytes must increase after adding content")
}

func TestRendererRetention_Stats_LinesCount(t *testing.T) {
	r := NewRenderer(WithRendererRetention(noRetention()))
	r.AddEvent(Event{Kind: EventTextDelta, Text: "a\nb\nc"})
	r.AddEvent(Event{Kind: EventTurnCompleted})
	s := r.Stats()
	// "a", "b", "c" — three completed flat lines
	assert.GreaterOrEqual(t, s.Lines, int64(3))
}

func TestRendererRetention_Stats_TurnsCount(t *testing.T) {
	r := NewRenderer(WithRendererRetention(noRetention()))
	completeTurn(r, "t1")
	completeTurn(r, "t2")
	s := r.Stats()
	assert.Equal(t, int64(2), s.Turns)
}

func TestRendererRetention_Stats_TurnsExcludesSentinel(t *testing.T) {
	// Force eviction to create a sentinel, then verify Stats().Turns excludes it.
	r := NewRenderer(WithRendererRetention(turnsOnly(1)))
	completeTurn(r, "t1")
	completeTurn(r, "t2") // triggers eviction of t1, sentinel inserted
	s := r.Stats()
	assert.Equal(t, int64(1), s.Turns, "sentinel must not be counted in Stats().Turns")
	assert.Equal(t, int64(1), s.EvictedTurns)
}

// ---- zero-limits mode ----

func TestRendererRetention_ZeroLimits_NoEviction(t *testing.T) {
	r := NewRenderer(WithRendererRetention(noRetention()))
	for i := range 10 {
		completeTurn(r, "t"+string(rune('0'+i)))
	}
	s := r.Stats()
	assert.Equal(t, int64(0), s.EvictedTurns)
	assert.Equal(t, int64(0), s.EvictedLines)
	assert.Equal(t, int64(0), s.EvictedBytes)
}

func TestRendererRetention_ZeroLimits_BytesStillAccurate(t *testing.T) {
	r := NewRenderer(WithRendererRetention(noRetention()))
	r.AddEvent(Event{Kind: EventTextDelta, Text: "hello world\n"})
	s := r.Stats()
	assert.Greater(t, s.Bytes, int64(0), "bytes must be tracked even with zero limits")
}

// ---- turn cap eviction ----

func TestRendererRetention_TurnCap_OldestEvicted(t *testing.T) {
	r := NewRenderer(WithRendererRetention(turnsOnly(2)))
	completeTurn(r, "t1")
	completeTurn(r, "t2")
	completeTurn(r, "t3") // triggers eviction of t1

	turns := r.CapturePresentation()
	ids := make([]string, 0)
	for _, t := range turns {
		if !t.isSentinel {
			ids = append(ids, t.ID)
		}
	}
	assert.NotContains(t, ids, "t1", "t1 should have been evicted")
	assert.Contains(t, ids, "t2")
	assert.Contains(t, ids, "t3")
}

func TestRendererRetention_TurnCap_EvictedTurnsCounter(t *testing.T) {
	r := NewRenderer(WithRendererRetention(turnsOnly(1)))
	completeTurn(r, "t1")
	completeTurn(r, "t2")
	completeTurn(r, "t3")

	s := r.Stats()
	assert.Equal(t, int64(2), s.EvictedTurns)
}

func TestRendererRetention_TurnCap_CurrentTurnPreserved(t *testing.T) {
	r := NewRenderer(WithRendererRetention(turnsOnly(0)))
	// Start a turn but don't complete it.
	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "current"})
	r.AddEvent(Event{Kind: EventTextDelta, TurnID: "current", Text: "in progress"})

	// Complete a bunch of other turns first.
	for i := range 5 {
		completeTurn(r, "other"+string(rune('0'+i)))
	}

	// The current turn must still be in the presentation.
	turns := r.CapturePresentation()
	var found bool
	for _, t := range turns {
		if t.ID == "current" {
			found = true
		}
	}
	assert.True(t, found, "active current turn must not be evicted")
}

func TestRendererRetention_TurnCap_SentinelUpdatedNotDuplicated(t *testing.T) {
	r := NewRenderer(WithRendererRetention(turnsOnly(1)))
	completeTurn(r, "t1")
	completeTurn(r, "t2") // evicts t1, creates sentinel
	completeTurn(r, "t3") // evicts t2, updates sentinel (not duplicated)

	turns := r.CapturePresentation()
	sentinelCount := 0
	for _, t := range turns {
		if t.isSentinel {
			sentinelCount++
		}
	}
	assert.Equal(t, 1, sentinelCount, "only one sentinel must exist after multiple evictions")

	// Sentinel text should reflect cumulative count.
	if sentinelCount == 1 {
		var sentinel *PresentationTurn
		for _, t := range turns {
			if t.isSentinel {
				sentinel = t
			}
		}
		require.NotNil(t, sentinel)
		require.Len(t, sentinel.Rows, 1)
		assert.Equal(t, RowSystem, sentinel.Rows[0].Kind)
		assert.Equal(t, "earlier turns evicted: 2", sentinel.Rows[0].Text)
	}
}

// ---- byte cap eviction (structured turns) ----

func TestRendererRetention_ByteCap_TurnsEvicted(t *testing.T) {
	// Each turn is ~turnOverhead(128) + few rowBytes. Use a very small cap.
	cap := int64(300)
	r := NewRenderer(WithRendererRetention(bytesOnly(cap)))

	completeTurn(r, "t1")
	completeTurn(r, "t2")
	completeTurn(r, "t3")

	s := r.Stats()
	assert.Greater(t, s.EvictedTurns, int64(0), "some turns should have been evicted to stay under byte cap")
}

func TestRendererRetention_ByteCap_BoundsMixedFlatStructuredStats(t *testing.T) {
	cap := int64(4096)
	r := NewRenderer(WithRendererRetention(bytesOnly(cap)))
	payload := strings.Repeat("x", 256) + "\n"

	for i := range 80 {
		id := fmt.Sprintf("t%d", i)
		r.AddEvent(Event{Kind: EventTurnStarted, TurnID: id})
		r.AddEvent(Event{Kind: EventUserPrompt, TurnID: id, Text: "run synthetic stream"})
		r.AddEvent(Event{Kind: EventTextDelta, TurnID: id, Text: payload})
		r.AddEvent(Event{Kind: EventTurnCompleted, TurnID: id})
	}

	s := r.Stats()
	assert.Greater(t, s.EvictedTurns, int64(0), "structured history should be evicted")
	assert.Greater(t, s.EvictedLines, int64(0), "flat history should be evicted")
	assert.LessOrEqual(t, s.Bytes, cap+cap/4, "aggregate Stats().Bytes must settle within the accepted overhead")
}

// ---- flat line eviction ----

func TestRendererRetention_FlatCap_LinesEvicted(t *testing.T) {
	// flatLineBytes("hello") = 32+5 = 37. Cap at 100 bytes → keeps ~2 lines.
	r := NewRenderer(WithRendererRetention(bytesOnly(100)))

	for i := range 10 {
		r.AddEvent(Event{Kind: EventTextDelta, Text: "hello" + string(rune('0'+i)) + "\n"})
	}

	s := r.Stats()
	assert.Greater(t, s.EvictedLines, int64(0), "flat lines should have been evicted under byte cap")
}

func TestRendererRetention_FlatCap_MarkerAppearsAtHead(t *testing.T) {
	r := NewRenderer(WithRendererRetention(bytesOnly(100)))

	for i := range 20 {
		r.AddEvent(Event{Kind: EventTextDelta, Text: "line" + string(rune('0'+i)) + "\n"})
	}

	content := r.Capture()
	assert.True(t, strings.HasPrefix(content, "[earlier lines evicted:"),
		"eviction marker must appear at head of flat capture")
}

func TestRendererRetention_FlatCap_MarkerNotDuplicated(t *testing.T) {
	r := NewRenderer(WithRendererRetention(bytesOnly(80)))

	// Add lines to trigger eviction twice.
	for i := range 30 {
		r.AddEvent(Event{Kind: EventTextDelta, Text: "x" + string(rune('0'+i)) + "\n"})
	}

	content := r.Capture()
	count := strings.Count(content, "[earlier lines evicted:")
	assert.Equal(t, 1, count, "flat eviction marker must not be duplicated")
}

func TestRendererRetention_FlatCap_OversizedPartialTrimmed(t *testing.T) {
	cap := int64(1024)
	r := NewRenderer(WithRendererRetention(bytesOnly(cap)))

	r.AddEvent(Event{
		Kind:   EventTextDelta,
		TurnID: "t1",
		Text:   strings.Repeat("x", 4096),
	})

	s := r.Stats()
	assert.LessOrEqual(t, s.Bytes, cap, "oversized no-newline deltas must be trimmed under the aggregate cap")
	assert.Less(t, len(r.Capture()), 4096, "flat partial capture must be shortened when it is the only flat data")
}

// ---- active-turn row truncation ----

func TestRendererRetention_ActiveTurnTruncation_RowsRemoved(t *testing.T) {
	// Very tight byte cap so even the current turn overflows.
	// Each row is ~rowOverhead(64) + text bytes. One row ~ 64+5 = 69 bytes.
	// Turn overhead is 128. Cap at 150 bytes → only 1 row fits in current turn.
	r := NewRenderer(WithRendererRetention(bytesOnly(150)))

	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	for i := range 5 {
		r.AddEvent(Event{Kind: EventTextDelta, TurnID: "t1", Text: "line" + string(rune('0'+i)) + "\n"})
	}

	s := r.Stats()
	assert.Greater(t, s.TruncatedRows, int64(0), "rows must be truncated when current turn exceeds byte cap")
}

func TestRendererRetention_ActiveTurnTruncation_TurnPreserved(t *testing.T) {
	r := NewRenderer(WithRendererRetention(bytesOnly(150)))

	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1"})
	for i := range 5 {
		r.AddEvent(Event{Kind: EventTextDelta, TurnID: "t1", Text: "line" + string(rune('0'+i)) + "\n"})
	}

	turns := r.CapturePresentation()
	var found bool
	for _, t := range turns {
		if t.ID == "t1" {
			found = true
		}
	}
	assert.True(t, found, "current turn must not be removed even when truncated")
}

func TestRendererRetention_ActiveTurnTruncation_OversizedOpenTextRowShortened(t *testing.T) {
	cap := int64(1024)
	r := NewRenderer(WithRendererRetention(bytesOnly(cap)))

	r.AddEvent(Event{
		Kind:   EventTextDelta,
		TurnID: "t1",
		Text:   strings.Repeat("x", 4096),
	})

	s := r.Stats()
	assert.Greater(t, s.TruncatedRows, int64(0), "open text row content must be truncated as a last resort")
	assert.LessOrEqual(t, s.Bytes, cap, "current-turn truncation must bring aggregate stats under the byte cap")

	turns := r.CapturePresentation()
	require.Len(t, turns, 1)
	require.NotEmpty(t, turns[0].Rows)
	last := turns[0].Rows[len(turns[0].Rows)-1]
	assert.NotEmpty(t, last.Text, "truncation should keep the newest suffix of the oversized open row")
	assert.Less(t, len(last.Text), 4096, "structured open text row must be shortened")
}

// ---- shell turn accounting ----

func TestRendererRetention_ShellTurn_CountedInStats(t *testing.T) {
	r := NewRenderer(WithRendererRetention(noRetention()))
	r.AddShellTurn("ls", "file1\nfile2", 0, false, "")
	s := r.Stats()
	assert.Equal(t, int64(1), s.Turns)
	assert.Greater(t, s.Bytes, int64(0))
}

func TestRendererRetention_ShellTurn_EvictedByTurnCap(t *testing.T) {
	r := NewRenderer(WithRendererRetention(turnsOnly(1)))
	r.AddShellTurn("ls", "out1", 0, false, "")
	r.AddShellTurn("pwd", "out2", 0, false, "")
	s := r.Stats()
	assert.Equal(t, int64(1), s.EvictedTurns)
	assert.Equal(t, int64(1), s.Turns)
}

// ---- CaptureRange after trimming ----

func TestRendererRetention_CaptureRange_AfterFlatEviction(t *testing.T) {
	r := NewRenderer(WithRendererRetention(bytesOnly(200)))

	for i := range 20 {
		r.AddEvent(Event{Kind: EventTextDelta, Text: "line" + string(rune('0'+i)) + "\n"})
	}

	// CaptureRange should still work and return a valid slice.
	result := r.CaptureRange("0", "2")
	assert.NotEmpty(t, result)
	// The first line after marker must be the marker itself or a real line.
	lines := strings.Split(result, "\n")
	assert.GreaterOrEqual(t, len(lines), 1)
}

// ---- ContentHash changes on retention ----

func TestRendererRetention_ContentHash_ChangesOnTurnEviction(t *testing.T) {
	r := NewRenderer(WithRendererRetention(turnsOnly(1)))

	completeTurn(r, "t1")
	hash1 := r.ContentHash()

	completeTurn(r, "t2") // evicts t1
	hash2 := r.ContentHash()

	assert.NotEqual(t, hash1, hash2, "ContentHash must change when a turn is evicted")
}

func TestRendererRetention_ContentHash_ChangesOnFlatEviction(t *testing.T) {
	r := NewRenderer(WithRendererRetention(bytesOnly(100)))

	r.AddEvent(Event{Kind: EventTextDelta, Text: "a\n"})
	r.AddEvent(Event{Kind: EventTextDelta, Text: "b\n"})
	hash1 := r.ContentHash()

	// Add many more lines to push past the cap.
	for i := range 20 {
		r.AddEvent(Event{Kind: EventTextDelta, Text: "line" + string(rune('0'+i)) + "\n"})
	}
	hash2 := r.ContentHash()

	assert.NotEqual(t, hash1, hash2, "ContentHash must change when flat lines are evicted")
}

// ---- JSON round-trip for RendererStats ----

func TestRendererStats_JSONRoundTrip(t *testing.T) {
	r := NewRenderer(WithRendererRetention(RendererRetentionOptions{
		MaxBytes: 512,
		MaxTurns: 10,
		Name:     "test",
	}))
	completeTurn(r, "t1")

	s := r.Stats()
	data, err := json.Marshal(s)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	// Verify all expected JSON keys are present.
	for _, key := range []string{
		"bytes", "lines", "turns", "max_bytes", "max_turns",
		"evicted_turns", "evicted_lines", "evicted_bytes", "truncated_rows",
	} {
		assert.Contains(t, raw, key, "JSON key %q must be present", key)
	}

	// Round-trip fidelity.
	var decoded RendererStats
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, s.Bytes, decoded.Bytes)
	assert.Equal(t, s.Lines, decoded.Lines)
	assert.Equal(t, s.Turns, decoded.Turns)
	assert.Equal(t, s.MaxBytes, decoded.MaxBytes)
	assert.Equal(t, s.MaxTurns, decoded.MaxTurns)
}

// ---- JSON round-trip ignores unexported accounting fields (isSentinel) ----

func TestRendererRetention_SentinelTurn_JSONDoesNotExposeIsSentinel(t *testing.T) {
	// Force a sentinel to be created.
	r := NewRenderer(WithRendererRetention(turnsOnly(1)))
	completeTurn(r, "t1")
	completeTurn(r, "t2")

	turns := r.CapturePresentation()
	require.NotEmpty(t, turns)

	// Find the sentinel.
	var sentinel *PresentationTurn
	for _, t := range turns {
		if t.isSentinel {
			sentinel = t
			break
		}
	}
	require.NotNil(t, sentinel, "sentinel must exist after eviction")

	// Marshal sentinel to JSON.
	data, err := json.Marshal(sentinel)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	// isSentinel must not appear in JSON output.
	assert.NotContains(t, raw, "isSentinel", "unexported isSentinel must not appear in JSON")
	assert.NotContains(t, raw, "is_sentinel", "unexported isSentinel must not appear in JSON")

	// Standard fields must still be present.
	assert.Contains(t, raw, "rows")
	rows, ok := raw["rows"].([]any)
	require.True(t, ok)
	require.Len(t, rows, 1)
}

// ---- mixed event stats math ----

func TestRendererRetention_MixedEvents_StatsAccurate(t *testing.T) {
	r := NewRenderer(WithRendererRetention(noRetention()))
	ts := time.Now()

	r.AddEvent(Event{Kind: EventTurnStarted, TurnID: "t1", Timestamp: ts})
	r.AddEvent(Event{Kind: EventUserPrompt, TurnID: "t1", Text: "run ls", Timestamp: ts})
	r.AddEvent(Event{Kind: EventToolCall, TurnID: "t1", ToolName: "bash", ToolInput: `{"command":"ls"}`, Timestamp: ts})
	r.AddEvent(Event{Kind: EventToolResult, TurnID: "t1", ToolName: "bash", ToolResult: "file1\nfile2", Timestamp: ts})
	r.AddEvent(Event{Kind: EventTextDelta, TurnID: "t1", Text: "done\n", Timestamp: ts})
	r.AddEvent(Event{Kind: EventTurnCompleted, TurnID: "t1", Timestamp: ts})

	s := r.Stats()
	assert.Equal(t, int64(1), s.Turns)
	assert.Greater(t, s.Bytes, int64(0))
	assert.Greater(t, s.Lines, int64(0))
	assert.Equal(t, int64(0), s.EvictedTurns)
	assert.Equal(t, int64(0), s.EvictedLines)
}

// ---- RendererOption functional option pattern ----

func TestRendererRetention_NewRenderer_NoOptions_HasDefaults(t *testing.T) {
	r := NewRenderer()
	s := r.Stats()
	assert.Equal(t, DefaultTranscriptMaxBytes, s.MaxBytes)
	assert.Equal(t, DefaultTranscriptMaxTurns, s.MaxTurns)
}

func TestRendererRetention_NewRenderer_OptionApplied(t *testing.T) {
	r := NewRenderer(WithRendererRetention(RendererRetentionOptions{MaxBytes: 9999, MaxTurns: 77}))
	s := r.Stats()
	assert.Equal(t, int64(9999), s.MaxBytes)
	assert.Equal(t, int64(77), s.MaxTurns)
}

func TestRendererRetention_SetRetention_CanBeCalledAfterEvents(t *testing.T) {
	r := NewRenderer(WithRendererRetention(noRetention()))
	completeTurn(r, "t1")

	// Update retention after events.
	r.SetRetention(RendererRetentionOptions{MaxBytes: 9999999, MaxTurns: 500})
	s := r.Stats()
	assert.Equal(t, int64(9999999), s.MaxBytes)
	assert.Equal(t, int64(500), s.MaxTurns)
}
