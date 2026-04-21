package sdk

import (
	"encoding/json"
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
	// RowUser records a user-authored prompt in the turn history.
	RowUser PresentationRowKind = "user"
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
	// RowToolDiff holds a structured file-diff block produced by an editing tool.
	RowToolDiff PresentationRowKind = "tool_diff"
	// RowToolPreview holds a structured file-content preview block from a read tool.
	RowToolPreview PresentationRowKind = "tool_preview"
)

// PresentationRow is a single typed content row within a PresentationTurn.
// All fields are value types — safe to copy and share across goroutines.
// Zero timestamps marshal as JSON null so browser clients do not need to
// special-case Go's zero time sentinel.
type PresentationRow struct {
	Kind PresentationRowKind
	Text string
	// Zero timestamps still encode as JSON null via MarshalJSON below.
	Timestamp   time.Time
	ToolName    string
	IsError     bool
	ToolDiff    *ToolDiffPayload    // set on RowToolDiff rows
	ToolPreview *ToolPreviewPayload // set on RowToolPreview rows
}

// PresentationTurn groups all content rows produced within one agent response turn.
type PresentationTurn struct {
	ID     string
	Number int
	// Zero timestamps still encode as JSON null via MarshalJSON below.
	StartedAt time.Time
	// Zero timestamps still encode as JSON null via MarshalJSON below.
	CompletedAt time.Time
	Interrupted bool
	ToolCount   int
	Rows        []PresentationRow
}

type presentationRowJSON struct {
	Kind        PresentationRowKind `json:"kind"`
	Text        string              `json:"text"`
	Timestamp   *time.Time          `json:"timestamp"`
	ToolName    string              `json:"tool_name"`
	IsError     bool                `json:"is_error"`
	ToolDiff    *ToolDiffPayload    `json:"tool_diff,omitempty"`
	ToolPreview *ToolPreviewPayload `json:"tool_preview,omitempty"`
}

type presentationTurnJSON struct {
	ID          string            `json:"id"`
	Number      int               `json:"number"`
	StartedAt   *time.Time        `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at"`
	Interrupted bool              `json:"interrupted"`
	ToolCount   int               `json:"tool_count"`
	Rows        []PresentationRow `json:"rows"`
}

func optionalJSONTime(ts time.Time) *time.Time {
	if ts.IsZero() {
		return nil
	}
	copy := ts
	return &copy
}

// MarshalJSON encodes zero timestamps as null in the wire format.
func (r PresentationRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(presentationRowJSON{
		Kind:        r.Kind,
		Text:        r.Text,
		Timestamp:   optionalJSONTime(r.Timestamp),
		ToolName:    r.ToolName,
		IsError:     r.IsError,
		ToolDiff:    r.ToolDiff,
		ToolPreview: r.ToolPreview,
	})
}

// UnmarshalJSON decodes null timestamps back to the zero time value.
func (r *PresentationRow) UnmarshalJSON(data []byte) error {
	var aux presentationRowJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.Kind = aux.Kind
	r.Text = aux.Text
	r.ToolName = aux.ToolName
	r.IsError = aux.IsError
	r.ToolDiff = aux.ToolDiff
	r.ToolPreview = aux.ToolPreview
	r.Timestamp = time.Time{}
	if aux.Timestamp != nil {
		r.Timestamp = *aux.Timestamp
	}
	return nil
}

// MarshalJSON encodes zero turn timestamps as null in the wire format.
func (t PresentationTurn) MarshalJSON() ([]byte, error) {
	return json.Marshal(presentationTurnJSON{
		ID:          t.ID,
		Number:      t.Number,
		StartedAt:   optionalJSONTime(t.StartedAt),
		CompletedAt: optionalJSONTime(t.CompletedAt),
		Interrupted: t.Interrupted,
		ToolCount:   t.ToolCount,
		Rows:        t.Rows,
	})
}

// UnmarshalJSON decodes null turn timestamps back to zero values.
func (t *PresentationTurn) UnmarshalJSON(data []byte) error {
	var aux presentationTurnJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	t.ID = aux.ID
	t.Number = aux.Number
	t.Interrupted = aux.Interrupted
	t.ToolCount = aux.ToolCount
	t.Rows = aux.Rows
	t.StartedAt = time.Time{}
	t.CompletedAt = time.Time{}
	if aux.StartedAt != nil {
		t.StartedAt = *aux.StartedAt
	}
	if aux.CompletedAt != nil {
		t.CompletedAt = *aux.CompletedAt
	}
	return nil
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
