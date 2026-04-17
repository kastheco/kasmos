package sdk

import (
	"fmt"
	"time"
)

// PresentationRowKind classifies a single row within a PresentationTurn.
type PresentationRowKind string

const (
	// RowThinking is reserved for extended-thinking blocks (wave 3).
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
	Kind      PresentationRowKind
	Text      string
	Timestamp time.Time
	ToolName  string
	IsError   bool
}

// PresentationTurn groups all content rows produced within one agent response turn.
type PresentationTurn struct {
	ID          string
	Number      int
	StartedAt   time.Time
	CompletedAt time.Time
	Interrupted bool
	ToolCount   int
	Rows        []PresentationRow
}

// Running reports whether the turn is still in progress (not yet completed or interrupted).
func (t PresentationTurn) Running() bool {
	return t.CompletedAt.IsZero() && !t.Interrupted
}

// HeaderText returns a human-readable one-line summary of the turn suitable for
// use as a visual header. The elapsed time is derived from now for running turns
// and from the recorded CompletedAt for finished ones.
func (t PresentationTurn) HeaderText(now time.Time) string {
	var elapsed time.Duration
	if t.Running() {
		if !t.StartedAt.IsZero() {
			elapsed = now.Sub(t.StartedAt)
		}
	} else if !t.CompletedAt.IsZero() && !t.StartedAt.IsZero() {
		elapsed = t.CompletedAt.Sub(t.StartedAt)
	}

	if elapsed > 0 {
		return fmt.Sprintf("turn %d · %s", t.Number, formatElapsed(elapsed))
	}
	return fmt.Sprintf("turn %d", t.Number)
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
