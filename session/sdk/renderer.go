package sdk

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Renderer accumulates structured events and renders them as line-based text.
// It is safe for concurrent use.
//
// Events are converted to text fragments and accumulated in an internal line
// buffer. The Capture and CaptureRange methods render the buffer on demand.
// CaptureRange supports the same "-"/"numeric" start/end semantics as tmux
// capture-pane -S/-E so existing callers need no changes.
//
// In parallel, AddEvent maintains a turn-grouped presentation model accessible
// via CapturePresentation. The flat and structured paths share formatting
// helpers but are otherwise independent.
type Renderer struct {
	mu      sync.Mutex
	lines   []string // completed lines (flat path)
	partial string   // fragment of the current (incomplete) last line (flat path)

	turns          []*PresentationTurn // structured turn model
	currentTurn    *PresentationTurn   // the open (in-progress) turn, nil when no turn is active
	nextTurnNumber int                 // monotonically increasing turn counter

	currentTurnHasResponse bool
	currentTurnOpenProse   int
}

// NewRenderer constructs an empty Renderer.
func NewRenderer() *Renderer {
	return &Renderer{}
}

// AddEvent incorporates a structured event into the renderer buffer.
// The flat line buffer (Capture/CaptureRange) and the structured turn model
// (CapturePresentation) are both updated in a single locked call.
// Safe to call concurrently.
func (r *Renderer) AddEvent(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch e.Kind {
	case EventTextDelta:
		// Flat path — unchanged.
		r.appendText(e.Text)
		// Structured path — create implicit turn if needed and append prose rows.
		turn := r.ensureTurn(e.TurnID, e.Timestamp)
		r.appendTurnText(turn, e.Text, e.Timestamp)

	case EventUserPrompt:
		r.appendLine("> " + e.Text)
		turn := r.ensureTurn(e.TurnID, e.Timestamp)
		r.closeStructuredProseBlock()
		turn.Rows = append(turn.Rows, PresentationRow{
			Kind:      RowUser,
			Text:      e.Text,
			Timestamp: e.Timestamp,
		})

	case EventToolCall:
		line := formatToolCallLine(e.ToolName, e.ToolInput)
		// Flat path.
		r.appendLine(line)
		// Structured path.
		turn := r.ensureTurn(e.TurnID, e.Timestamp)
		r.closeStructuredProseBlock()
		turn.Rows = append(turn.Rows, PresentationRow{
			Kind:      RowTool,
			Text:      line,
			Timestamp: e.Timestamp,
			ToolName:  e.ToolName,
		})
		turn.ToolCount++

	case EventToolResult:
		// Render a short, single-line result summary. Long payloads (file
		// dumps, command output) are compressed to "→ N lines" rather than
		// flooding the pane, matching codex-cli's UX — but we still want
		// SOMETHING visible so a genuine MCP/tool failure doesn't silently
		// look like success. Explicit error signals (success=false, error
		// key, non-zero exit_code) render as "✗ …" with the message.
		if line := formatToolResultLine(e.ToolResult); line != "" {
			// Flat path.
			r.appendLine(line)
			// Structured path.
			turn := r.ensureTurn(e.TurnID, e.Timestamp)
			r.closeStructuredProseBlock()
			turn.Rows = append(turn.Rows, PresentationRow{
				Kind:      RowResult,
				Text:      line,
				Timestamp: e.Timestamp,
				ToolName:  e.ToolName,
				IsError:   strings.HasPrefix(line, "✗ "),
			})
		}

	case EventPermission:
		line := fmt.Sprintf("[permission: %s]", e.PermissionDescription)
		// Flat path.
		r.appendLine(line)
		// Structured path — permission belongs to the interrupted turn; create
		// an implicit turn if no turn has started yet (transport emitted
		// permission before turn_started).
		turn := r.ensureTurn(e.TurnID, e.Timestamp)
		r.closeStructuredProseBlock()
		turn.Rows = append(turn.Rows, PresentationRow{
			Kind:      RowPermission,
			Text:      line,
			Timestamp: e.Timestamp,
		})

	case EventSystem:
		// Flat path — unchanged.
		if e.Text != "" {
			r.appendLine("[system: " + e.Text + "]")
		}
		// Structured path — preserve system rows even when no turn is currently
		// open so the structured preview cannot drop startup/transport errors that
		// still appear in the flat capture.
		if e.Text != "" {
			row := PresentationRow{
				Kind:      RowSystem,
				Text:      "[system: " + e.Text + "]",
				Timestamp: e.Timestamp,
			}
			if r.currentTurn != nil {
				r.closeStructuredProseBlock()
				r.currentTurn.Rows = append(r.currentTurn.Rows, row)
			} else {
				r.appendStandaloneTurn(e.TurnID, e.Timestamp, row)
			}
		}
		// A final system event closes the current turn without interrupting it.
		// This handles the codex error path where a final system event ends a
		// turn without a separate turn_interrupted notification.
		if e.Final && r.currentTurn != nil {
			ts := e.Timestamp
			if ts.IsZero() {
				ts = time.Now()
			}
			r.currentTurn.CompletedAt = ts
			r.clearCurrentTurn()
		}

	case EventTurnStarted:
		// Flat path — no visible marker needed.
		// Structured path — if a permission/system row already created an implicit
		// turn for this TurnID, keep using that turn rather than showing it as
		// interrupted and starting a duplicate numbered turn.
		if r.currentTurn != nil && (r.currentTurn.ID == e.TurnID || r.currentTurn.ID == "") {
			if r.currentTurn.ID == "" {
				r.currentTurn.ID = e.TurnID
			}
			return
		}
		// Otherwise close any open turn as interrupted, then open a new one.
		if r.currentTurn != nil {
			r.currentTurn.Interrupted = true
			ts := e.Timestamp
			if ts.IsZero() {
				ts = time.Now()
			}
			r.currentTurn.Rows = append(r.currentTurn.Rows, PresentationRow{
				Kind:      RowStatus,
				Text:      "[interrupted]",
				Timestamp: ts,
			})
			r.clearCurrentTurn()
		}
		r.startTurn(e.TurnID, e.Timestamp)

	case EventTurnCompleted:
		// Flat path — flush any pending partial line.
		r.flushPartial()
		// Structured path — close the turn normally (no RowStatus row).
		if r.currentTurn != nil {
			ts := e.Timestamp
			if ts.IsZero() {
				ts = time.Now()
			}
			r.currentTurn.CompletedAt = ts
			r.clearCurrentTurn()
		}

	case EventTurnInterrupted:
		// Flat path — flush partial and add marker.
		r.flushPartial()
		r.lines = append(r.lines, "[interrupted]")
		// Structured path — mark turn interrupted and add RowStatus row.
		if r.currentTurn != nil {
			r.currentTurn.Interrupted = true
			ts := e.Timestamp
			if ts.IsZero() {
				ts = time.Now()
			}
			r.currentTurn.Rows = append(r.currentTurn.Rows, PresentationRow{
				Kind:      RowStatus,
				Text:      "[interrupted]",
				Timestamp: ts,
			})
			r.clearCurrentTurn()
		}
	}
}

// CapturePresentation returns a deep copy of the structured turn model.
// The returned slice and all nested rows are safe for callers to mutate.
// Returns nil when no events have produced any turns yet.
//
// A timing-only RowThinking row is injected into any running turn that has
// been waiting longer than thinkingThreshold without producing real content.
// Because the injection happens on the copy, it disappears automatically once
// the turn accumulates tool, prose, permission, or system rows.
func (r *Renderer) CapturePresentation() []*PresentationTurn {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	copied := deepCopyTurns(r.turns)
	for _, t := range copied {
		maybeInjectThinking(t, now)
	}
	return copied
}

// ensureTurn returns the current open turn, creating an implicit one with the
// given turnID when no turn is active. Must be called with r.mu held.
func (r *Renderer) ensureTurn(turnID string, ts time.Time) *PresentationTurn {
	if r.currentTurn != nil {
		return r.currentTurn
	}
	return r.startTurn(turnID, ts)
}

// appendTurnText mirrors the newline/fragment behaviour of appendText but
// operates on the structured turn's Rows slice. The first prose chunk in a
// turn inserts a RowResponse sentinel before the prose rows so renderers can
// identify where assistant prose begins without inspecting row text.
// Must be called with r.mu held.
func (r *Renderer) appendTurnText(turn *PresentationTurn, text string, ts time.Time) {
	if text == "" {
		return
	}

	if !r.currentTurnHasResponse {
		turn.Rows = append(turn.Rows, PresentationRow{
			Kind:      RowResponse,
			Timestamp: ts,
		})
		r.currentTurnHasResponse = true
	}

	parts := strings.Split(text, "\n")

	if len(parts) == 1 {
		// No newline — extend the current partial prose row.
		if r.currentTurnOpenProse >= 0 {
			turn.Rows[r.currentTurnOpenProse].Text += parts[0]
		} else {
			turn.Rows = append(turn.Rows, PresentationRow{
				Kind: RowProse, Text: parts[0], Timestamp: ts,
			})
			r.currentTurnOpenProse = len(turn.Rows) - 1
		}
		return
	}

	// First chunk completes the current partial prose row.
	if r.currentTurnOpenProse >= 0 {
		turn.Rows[r.currentTurnOpenProse].Text += parts[0]
	} else {
		turn.Rows = append(turn.Rows, PresentationRow{
			Kind: RowProse, Text: parts[0], Timestamp: ts,
		})
	}
	r.currentTurnOpenProse = -1

	// Middle chunks are complete prose rows.
	for _, p := range parts[1 : len(parts)-1] {
		turn.Rows = append(turn.Rows, PresentationRow{
			Kind: RowProse, Text: p, Timestamp: ts,
		})
	}

	// Last chunk starts a new partial prose row unless the delta ended with a
	// newline, in which case there is no open prose fragment to extend.
	if tail := parts[len(parts)-1]; tail != "" {
		turn.Rows = append(turn.Rows, PresentationRow{
			Kind: RowProse, Text: tail, Timestamp: ts,
		})
		r.currentTurnOpenProse = len(turn.Rows) - 1
	}
}

func (r *Renderer) startTurn(turnID string, ts time.Time) *PresentationTurn {
	if ts.IsZero() {
		ts = time.Now()
	}
	r.nextTurnNumber++
	turn := &PresentationTurn{
		ID:        turnID,
		Number:    r.nextTurnNumber,
		StartedAt: ts,
	}
	r.turns = append(r.turns, turn)
	r.currentTurn = turn
	r.currentTurnHasResponse = false
	r.currentTurnOpenProse = -1
	return turn
}

func (r *Renderer) clearCurrentTurn() {
	r.currentTurn = nil
	r.currentTurnHasResponse = false
	r.currentTurnOpenProse = -1
}

// closeStructuredProseBlock marks the current prose fragment as closed so the
// next text delta starts a fresh response section instead of extending prose
// that was already followed by tool/system/permission rows.
func (r *Renderer) closeStructuredProseBlock() {
	r.currentTurnOpenProse = -1
	if r.currentTurnHasResponse {
		r.currentTurnHasResponse = false
	}
}

func (r *Renderer) appendStandaloneTurn(turnID string, ts time.Time, row PresentationRow) {
	turn := r.startTurn(turnID, ts)
	if row.Timestamp.IsZero() {
		row.Timestamp = turn.StartedAt
	}
	turn.Rows = append(turn.Rows, row)
	turn.CompletedAt = row.Timestamp
	r.clearCurrentTurn()
}

// deepCopyTurns returns a fully independent copy of src. The returned pointers
// and all Rows slices are freshly allocated so callers can safely mutate them.
func deepCopyTurns(src []*PresentationTurn) []*PresentationTurn {
	if len(src) == 0 {
		return nil
	}
	out := make([]*PresentationTurn, len(src))
	for i, t := range src {
		cp := *t // value copy of PresentationTurn
		cp.Rows = make([]PresentationRow, len(t.Rows))
		copy(cp.Rows, t.Rows) // PresentationRow has no pointer fields
		out[i] = &cp
	}
	return out
}

// appendText appends a raw text fragment (which may contain newlines) to the
// renderer buffer. Internal helper — must be called with r.mu held.
func (r *Renderer) appendText(text string) {
	parts := strings.Split(text, "\n")
	if len(parts) == 1 {
		// No newline — extend the current partial line.
		r.partial += parts[0]
		return
	}
	// First chunk completes the current partial line.
	r.lines = append(r.lines, r.partial+parts[0])
	// Middle chunks are complete lines.
	for _, p := range parts[1 : len(parts)-1] {
		r.lines = append(r.lines, p)
	}
	// Last chunk starts a new partial line.
	r.partial = parts[len(parts)-1]
}

// appendLine commits a full line, flushing any pending partial first.
// Must be called with r.mu held.
func (r *Renderer) appendLine(line string) {
	r.flushPartial()
	r.lines = append(r.lines, line)
}

// flushPartial moves the current partial fragment to the lines slice.
// Must be called with r.mu held.
func (r *Renderer) flushPartial() {
	if r.partial != "" {
		r.lines = append(r.lines, r.partial)
		r.partial = ""
	}
}

// allLines returns a combined slice of completed lines plus the current
// partial (if non-empty). Must be called with r.mu held.
func (r *Renderer) allLines() []string {
	all := make([]string, len(r.lines))
	copy(all, r.lines)
	if r.partial != "" {
		all = append(all, r.partial)
	}
	return all
}

// Capture returns all accumulated content joined by newlines.
func (r *Renderer) Capture() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.allLines(), "\n")
}

// CaptureRange returns a slice of the line buffer between start and end.
//
// The start and end parameters follow tmux capture-pane -S/-E semantics:
//   - "" or "-"  → beginning (for start) or end (for end) of history
//   - non-negative integer → 0-based line index from the top
//   - negative integer → offset from the last line (-1 = last, -2 = second-to-last)
func (r *Renderer) CaptureRange(start, end string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	all := r.allLines()
	n := len(all)
	if n == 0 {
		return ""
	}

	s := resolveLineIndex(start, n, 0)
	e := resolveLineIndex(end, n, n-1)

	if s < 0 {
		s = 0
	}
	if e >= n {
		e = n - 1
	}
	if s > e {
		return ""
	}
	return strings.Join(all[s:e+1], "\n")
}

// ContentHash returns a string that changes whenever the accumulated content
// changes. Callers use it to implement HasUpdated-style change detection.
func (r *Renderer) ContentHash() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.allLines(), "\n")
}

// formatToolCallLine renders a tool invocation as a single compact line
// with a bullet prefix and a short, human-readable argument summary. Keeps
// the agent prose readable when tools fire in rapid succession. The raw
// ToolInput is json (or a codex dynamic-tool arg blob) — we pick out the
// most informative field from a short allowlist (path/pattern/command/
// query/title/url) and fall back to a truncated raw dump when none match.
func formatToolCallLine(name, rawInput string) string {
	summary := summariseToolArgs(rawInput)
	if strings.EqualFold(strings.TrimSpace(name), "commandExecution") {
		if summary == "" {
			return "• command"
		}
		return "• " + summary
	}
	if summary == "" {
		return fmt.Sprintf("• %s", name)
	}
	return fmt.Sprintf("• %s %s", name, summary)
}

func summariseToolArgs(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" || trimmed == "{}" {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		// Not JSON — just truncate the raw string.
		return truncateOneLine(shortenCommandDisplay(trimmed), 80)
	}
	// Priority order: most informative single-field summaries first. Paths
	// get basename'd because absolute paths waste the whole line on the
	// prefix and hide the meaningful tail.
	if v, ok := obj["path"].(string); ok && v != "" {
		return filepath.Base(v)
	}
	if v, ok := obj["filename"].(string); ok && v != "" {
		return filepath.Base(v)
	}
	if v, ok := obj["command"].(string); ok && v != "" {
		return truncateOneLine(shortenCommandDisplay(v), 80)
	}
	if v, ok := obj["pattern"].(string); ok && v != "" {
		return truncateOneLine(v, 60)
	}
	if v, ok := obj["query"].(string); ok && v != "" {
		return truncateOneLine(v, 60)
	}
	if v, ok := obj["url"].(string); ok && v != "" {
		return truncateOneLine(v, 80)
	}
	if v, ok := obj["title"].(string); ok && v != "" {
		return truncateOneLine(v, 60)
	}
	// Last resort: list argument keys so the reader at least knows the
	// shape without getting a wall of json.
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return ""
	}
	return truncateOneLine(strings.Join(keys, ","), 60)
}

func shortenCommandDisplay(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return cmd
	}
	switch {
	case strings.HasPrefix(fields[0], "/usr/bin/"), strings.HasPrefix(fields[0], "/bin/"):
		fields[0] = filepath.Base(fields[0])
	}
	return strings.Join(fields, " ")
}

// formatToolResultLine renders a short, single-line summary of a tool
// result. Error-shaped payloads (success=false, "error" field, non-zero
// exit_code) get a "✗ <message>" marker. Successful JSON arrays are
// compressed to "→ N items" / "→ N lines" so we don't flood the pane.
// Short plain-text output passes through truncated; long text becomes
// "→ first-line…" so the operator can still see SOMETHING came back.
func formatToolResultLine(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return ""
	}

	// Try structured JSON first.
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
		if success, ok := obj["success"].(bool); ok && !success {
			msg := truncateOneLine(strings.TrimSpace(fmt.Sprint(obj["error"])), 120)
			if msg == "" || msg == "<nil>" {
				msg = "tool returned success=false"
			}
			return "✗ " + msg
		}
		if errVal, ok := obj["error"]; ok {
			if msg, _ := errVal.(string); strings.TrimSpace(msg) != "" {
				return "✗ " + truncateOneLine(msg, 120)
			}
		}
		if exit, ok := obj["exit_code"].(float64); ok && exit != 0 {
			return fmt.Sprintf("✗ exit=%d", int(exit))
		}
	}

	// Array payload — count items.
	var arr []any
	if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
		return fmt.Sprintf("→ %d items", len(arr))
	}

	// Plain text — compact to first line (or a short slice). Long output
	// gets a line-count summary instead of the full content.
	lines := strings.Split(trimmed, "\n")
	if len(lines) > 3 {
		return fmt.Sprintf("→ %d lines", len(lines))
	}
	return "→ " + truncateOneLine(trimmed, 120)
}

// truncateOneLine collapses internal whitespace and caps the string at n
// visible runes, appending an ellipsis when clipped. Used so multi-line
// tool arguments render as a single-line summary.
func truncateOneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if n <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// resolveLineIndex converts a tmux-style start/end string to a 0-based line
// index within n total lines. defaultIdx is returned for "" and "-".
func resolveLineIndex(s string, n, defaultIdx int) int {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || trimmed == "-" {
		return defaultIdx
	}
	idx, err := strconv.Atoi(trimmed)
	if err != nil {
		return defaultIdx
	}
	if idx < 0 {
		return n + idx
	}
	return idx
}
