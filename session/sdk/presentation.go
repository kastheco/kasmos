package sdk

import (
	"fmt"
	"time"
)

// thinkingThreshold is the minimum elapsed time before a timing-only "thinking"
// row is injected into a running turn that has produced no real content yet.
const thinkingThreshold = 2 * time.Second

// maybeInjectThinking appends a timing-only RowThinking row to the turn copy
// when the turn is still running, has not yet produced any tool, result, prose,
// permission, or system rows, and has been running for longer than
// thinkingThreshold. Because this is called on a deep copy in CapturePresentation,
// the row disappears automatically once real content arrives.
func maybeInjectThinking(turn *PresentationTurn, now time.Time) {
	if !turn.Running() {
		return
	}
	if turn.StartedAt.IsZero() {
		return
	}
	elapsed := now.Sub(turn.StartedAt)
	if elapsed < thinkingThreshold {
		return
	}
	// Do not inject when the turn already has substantive content.
	for _, row := range turn.Rows {
		switch row.Kind {
		case RowTool, RowResult, RowProse, RowPermission, RowSystem:
			return
		}
	}
	turn.Rows = append(turn.Rows, PresentationRow{
		Kind:      RowThinking,
		Text:      fmt.Sprintf("thinking %.1fs", elapsed.Seconds()),
		Timestamp: now,
	})
}

// PresentationRowKind classifies a single row within a PresentationTurn.
type PresentationRowKind string

const (
	// RowThinking is a timing-only placeholder row for long-running turns that
	// have not yet produced substantive content.
	RowThinking PresentationRowKind = "thinking"
	// RowTool represents a tool invocation.
	RowTool PresentationRowKind = "tool"
	// RowResult represents a tool result.
	RowResult PresentationRowKind = "result"
	// RowSystem represents a transport-level system message.
	RowSystem PresentationRowKind = "system"
	// RowPermission represents a pending permission request.
	RowPermission PresentationRowKind = "permission"
	// RowResponse is a sentinel that marks where the assistant prose begins within a turn.
	RowResponse PresentationRowKind = "response"
	// RowProse holds one logical line of assistant prose text.
	RowProse PresentationRowKind = "prose"
	// RowStatus carries lifecycle annotations such as "[interrupted]".
	RowStatus PresentationRowKind = "status"
)

// PresentationRow is a single typed content row within a PresentationTurn.
// All fields are value types — safe to copy and share across goroutines.
type PresentationRow struct {
	Kind      PresentationRowKind `json:"kind"`
	Text      string              `json:"text"`
	Timestamp time.Time           `json:"timestamp"`
	ToolName  string              `json:"tool_name"`
	IsError   bool                `json:"is_error"`
}

// PresentationTurn groups all content rows produced within one agent response turn.
type PresentationTurn struct {
	ID          string            `json:"id"`
	Number      int               `json:"number"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
	Interrupted bool              `json:"interrupted"`
	ToolCount   int               `json:"tool_count"`
	Rows        []PresentationRow `json:"rows"`
}

// Running reports whether the turn is still in progress (not yet completed or interrupted).
func (t PresentationTurn) Running() bool {
	return t.CompletedAt.IsZero() && !t.Interrupted
}

// HeaderText returns a human-readable one-line summary of the turn suitable for
// use as a visual header. The elapsed time is derived from now for running turns
// and from the recorded CompletedAt for finished ones. Running turns append
// "• running" at the end. Tool count is included when > 0.
func (t PresentationTurn) HeaderText(now time.Time) string {
	var elapsed time.Duration
	if t.Running() {
		if !t.StartedAt.IsZero() {
			elapsed = now.Sub(t.StartedAt)
		}
	} else if !t.CompletedAt.IsZero() && !t.StartedAt.IsZero() {
		elapsed = t.CompletedAt.Sub(t.StartedAt)
	}

	label := fmt.Sprintf("turn %d", t.Number)
	if elapsed > 0 {
		label += " · " + formatElapsed(elapsed)
	}
	if t.ToolCount == 1 {
		label += " · 1 tool"
	} else if t.ToolCount > 1 {
		label += fmt.Sprintf(" · %d tools", t.ToolCount)
	}
	if t.Running() {
		label += " • running"
	}
	return label
}

// formatElapsed formats a duration as a compact human-readable string.
func formatElapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm%ds", m, s)
}
