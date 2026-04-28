package sdk

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// thinkingThreshold is the minimum elapsed time before a timing-only "thinking"
// row is injected into a running turn that has produced no real content yet.
const thinkingThreshold = 2 * time.Second

// maybeInjectThinking appends a timing-only RowThinking row to the turn copy
// when the turn is still running, has not yet produced any tool, result, prose,
// permission, warning, or system rows, and has been running for longer than
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
		case RowTool, RowResult, RowProse, RowCodeBlock, RowPermission, RowWarning, RowSystem:
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
	// RowToolDiff is a structured diff payload row that follows a RowTool row
	// for diff-producing tools (Edit, Write, MultiEdit, apply_patch, fileChange).
	RowToolDiff PresentationRowKind = "tool_diff"
	// RowResult represents a tool result.
	RowResult PresentationRowKind = "result"
	// RowToolPreview is a structured text-preview payload row that follows a
	// RowResult row for non-error textual tool results.
	RowToolPreview PresentationRowKind = "tool_preview"
	// RowWarning represents a transport-level warning message.
	RowWarning PresentationRowKind = "warning"
	// RowSystem represents a transport-level system message.
	RowSystem PresentationRowKind = "system"
	// RowPermission represents a pending permission request.
	RowPermission PresentationRowKind = "permission"
	// RowResponse is a sentinel that marks where the assistant prose begins within a turn.
	RowResponse PresentationRowKind = "response"
	// RowProse holds one logical line of assistant prose text.
	RowProse PresentationRowKind = "prose"
	// RowCodeBlock holds one logical line inside a fenced assistant code block.
	RowCodeBlock PresentationRowKind = "code_block"
	// RowStatus carries lifecycle annotations such as "[interrupted]".
	RowStatus PresentationRowKind = "status"
)

// ToolDiffLineKind classifies a single line within a ToolDiffPayload.
type ToolDiffLineKind string

const (
	// DiffLineContext is an unchanged line present in both old and new.
	DiffLineContext ToolDiffLineKind = "context"
	// DiffLineAdded is a line present only in the new version.
	DiffLineAdded ToolDiffLineKind = "added"
	// DiffLineRemoved is a line present only in the old version.
	DiffLineRemoved ToolDiffLineKind = "removed"
)

// ToolDiffLine represents a single line in a structured file diff.
// OldNumber/NewNumber are 1-based and omitted when not applicable.
type ToolDiffLine struct {
	Kind      ToolDiffLineKind `json:"kind"`
	OldNumber *int             `json:"old_number,omitempty"`
	NewNumber *int             `json:"new_number,omitempty"`
	OldText   string           `json:"old_text,omitempty"`
	NewText   string           `json:"new_text,omitempty"`
}

// ToolDiffPayload holds a render-ready diff for a single file. Renderers
// consume the Lines slice directly without running their own diff algorithm.
type ToolDiffPayload struct {
	Path            string         `json:"path,omitempty"`
	Lines           []ToolDiffLine `json:"lines,omitempty"`
	Truncated       bool           `json:"truncated,omitempty"`
	HiddenLineCount int            `json:"hidden_line_count,omitempty"`
}

// ToolPreviewPayload holds a capped line slice from a tool result for display.
type ToolPreviewPayload struct {
	Lines           []string `json:"lines,omitempty"`
	Truncated       bool     `json:"truncated,omitempty"`
	HiddenLineCount int      `json:"hidden_line_count,omitempty"`
}

// TurnActivity describes the current activity of a running turn. It is
// derived on demand in CapturePresentation and set only on the deep copy.
type TurnActivity struct {
	Kind      string    `json:"kind"`
	Label     string    `json:"label,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

// MarshalJSON encodes zero StartedAt as null consistent with the rest of the
// presentation model.
func (a TurnActivity) MarshalJSON() ([]byte, error) {
	type turnActivityJSON struct {
		Kind      string     `json:"kind"`
		Label     string     `json:"label,omitempty"`
		StartedAt *time.Time `json:"started_at"`
	}
	return json.Marshal(turnActivityJSON{
		Kind:      a.Kind,
		Label:     a.Label,
		StartedAt: optionalJSONTime(a.StartedAt),
	})
}

// UnmarshalJSON decodes null StartedAt back to zero value.
func (a *TurnActivity) UnmarshalJSON(data []byte) error {
	type turnActivityJSON struct {
		Kind      string     `json:"kind"`
		Label     string     `json:"label,omitempty"`
		StartedAt *time.Time `json:"started_at"`
	}
	var aux turnActivityJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	a.Kind = aux.Kind
	a.Label = aux.Label
	a.StartedAt = time.Time{}
	if aux.StartedAt != nil {
		a.StartedAt = *aux.StartedAt
	}
	return nil
}

// PresentationRow is a single typed content row within a PresentationTurn.
// Zero timestamps marshal as JSON null so browser clients do not need to
// special-case Go's zero time sentinel.
// ToolDiff and ToolPreview are non-nil only for RowToolDiff and RowToolPreview
// rows respectively.
type PresentationRow struct {
	Kind PresentationRowKind
	Text string
	// Zero timestamps still encode as JSON null via MarshalJSON below.
	Timestamp   time.Time
	ToolName    string
	IsError     bool
	ToolDiff    *ToolDiffPayload    // non-nil for RowToolDiff rows only
	ToolPreview *ToolPreviewPayload // non-nil for RowToolPreview rows only
	// ExitCode and Output are non-zero only for commandExecution RowResult rows.
	// ExitCode nil means "no exit code information"; 0 means success.
	ExitCode *int   // non-nil when a structured command exit code is available
	Output   string // normalized single-line stdout/aggregated output from the command
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
	// Activity is derived in CapturePresentation for running turns only.
	// It is nil on completed and interrupted turns, and nil in the live
	// renderer's internal state (set only on the deep copy).
	Activity *TurnActivity

	// isSentinel marks the eviction sentinel turn at index 0 of the renderer's
	// internal turns slice. It is unexported and invisible to JSON consumers.
	isSentinel bool
}

type presentationRowJSON struct {
	Kind        PresentationRowKind `json:"kind"`
	Text        string              `json:"text"`
	Timestamp   *time.Time          `json:"timestamp"`
	ToolName    string              `json:"tool_name"`
	IsError     bool                `json:"is_error"`
	ToolDiff    *ToolDiffPayload    `json:"tool_diff,omitempty"`
	ToolPreview *ToolPreviewPayload `json:"tool_preview,omitempty"`
	ExitCode    *int                `json:"exit_code,omitempty"`
	Output      string              `json:"output,omitempty"`
}

type presentationTurnJSON struct {
	ID          string            `json:"id"`
	Number      int               `json:"number"`
	StartedAt   *time.Time        `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at"`
	Interrupted bool              `json:"interrupted"`
	ToolCount   int               `json:"tool_count"`
	Rows        []PresentationRow `json:"rows"`
	Activity    *TurnActivity     `json:"activity,omitempty"`
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
		ExitCode:    r.ExitCode,
		Output:      r.Output,
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
	r.ExitCode = aux.ExitCode
	r.Output = aux.Output
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
		Activity:    t.Activity,
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
	t.Activity = aux.Activity
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

// ClonePresentationTurns returns a fully independent deep copy of src.
// All nested Rows slices, ToolDiff, ToolPreview, and Activity pointer fields
// are freshly allocated so callers can safely mutate the result without
// affecting the original. Returns nil when src is empty.
func ClonePresentationTurns(src []*PresentationTurn) []*PresentationTurn {
	if len(src) == 0 {
		return nil
	}
	out := make([]*PresentationTurn, len(src))
	for i, t := range src {
		if t == nil {
			continue
		}
		cp := *t
		cp.Rows = make([]PresentationRow, len(t.Rows))
		for j, row := range t.Rows {
			rowCp := row
			if row.ExitCode != nil {
				exitCode := *row.ExitCode
				rowCp.ExitCode = &exitCode
			}
			if row.ToolDiff != nil {
				td := *row.ToolDiff
				td.Lines = cloneToolDiffLines(row.ToolDiff.Lines)
				rowCp.ToolDiff = &td
			}
			if row.ToolPreview != nil {
				tp := *row.ToolPreview
				tp.Lines = cloneStringSlice(row.ToolPreview.Lines)
				rowCp.ToolPreview = &tp
			}
			cp.Rows[j] = rowCp
		}
		if t.Activity != nil {
			actCp := *t.Activity
			cp.Activity = &actCp
		}
		out[i] = &cp
	}
	return out
}

func cloneToolDiffLines(src []ToolDiffLine) []ToolDiffLine {
	if src == nil {
		return nil
	}
	out := make([]ToolDiffLine, len(src))
	for i, line := range src {
		lineCp := line
		if line.OldNumber != nil {
			n := *line.OldNumber
			lineCp.OldNumber = &n
		}
		if line.NewNumber != nil {
			n := *line.NewNumber
			lineCp.NewNumber = &n
		}
		out[i] = lineCp
	}
	return out
}

func cloneStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

// deriveTurnActivity returns a TurnActivity describing the current work of a
// running turn. Returns nil for completed or interrupted turns. The rules are
// evaluated in order; the first matching rule wins:
//
//  1. Last unresolved RowTool (no subsequent RowResult) → kind:"tool"
//  2. Last RowPermission → kind:"permission"
//  3. No substantive rows at all → kind:"thinking"
//  4. Any other running state → kind:"working"
func deriveTurnActivity(turn *PresentationTurn, now time.Time) *TurnActivity {
	if !turn.Running() {
		return nil
	}

	// Rule 1: last unresolved RowTool.
	lastToolIdx := -1
	for i := len(turn.Rows) - 1; i >= 0; i-- {
		if turn.Rows[i].Kind == RowTool {
			lastToolIdx = i
			break
		}
	}
	if lastToolIdx >= 0 {
		hasResult := false
		for i := lastToolIdx + 1; i < len(turn.Rows); i++ {
			if turn.Rows[i].Kind == RowResult {
				hasResult = true
				break
			}
		}
		if !hasResult {
			toolRow := turn.Rows[lastToolIdx]
			label := strings.TrimPrefix(toolRow.Text, "• ")
			return &TurnActivity{
				Kind:      "tool",
				Label:     label,
				StartedAt: toolRow.Timestamp,
			}
		}
	}

	// Rule 2: last RowPermission on the turn.
	for i := len(turn.Rows) - 1; i >= 0; i-- {
		if turn.Rows[i].Kind == RowPermission {
			return &TurnActivity{
				Kind:      "permission",
				Label:     "permission requested",
				StartedAt: turn.Rows[i].Timestamp,
			}
		}
	}

	// Rule 3: no substantive rows yet → thinking.
	hasSubstantive := false
	for _, row := range turn.Rows {
		switch row.Kind {
		case RowTool, RowResult, RowProse, RowCodeBlock, RowPermission, RowWarning, RowSystem:
			hasSubstantive = true
		}
	}
	if !hasSubstantive {
		return &TurnActivity{
			Kind:      "thinking",
			Label:     "thinking",
			StartedAt: turn.StartedAt,
		}
	}

	// Rule 4: working.
	label := "working"
	if lastToolIdx >= 0 {
		if name := turn.Rows[lastToolIdx].ToolName; name != "" {
			label = name
		}
	}
	return &TurnActivity{
		Kind:      "working",
		Label:     label,
		StartedAt: turn.StartedAt,
	}
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
