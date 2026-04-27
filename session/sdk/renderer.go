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
//
// Retention is controlled by RendererRetentionOptions. By default 4 MiB /
// 2000 completed turns are kept; use WithRendererRetention to override.
type Renderer struct {
	mu      sync.Mutex
	lines   []string // completed lines (flat path)
	partial string   // fragment of the current (incomplete) last line (flat path)

	turns          []*PresentationTurn // structured turn model
	currentTurn    *PresentationTurn   // the open (in-progress) turn, nil when no turn is active
	nextTurnNumber int                 // monotonically increasing turn counter

	currentTurnHasResponse bool
	currentTurnOpenTextRow int
	currentTurnInCodeFence bool

	// retention configuration and accounting
	retentionOpts     RendererRetentionOptions
	retainedFlatBytes int64 // approximate bytes in r.lines (excludes r.partial)
	retainedTurnBytes int64 // approximate bytes of completed/non-current turns
	evictedTurns      int64 // cumulative structured turns evicted
	evictedFlatLines  int64 // cumulative flat lines evicted
	evictedBytes      int64 // cumulative bytes freed across both paths
	truncatedRows     int64 // cumulative rows truncated from the current turn
	hasSentinelTurn   bool  // true when r.turns[0] is the eviction sentinel
	hasFlatMarker     bool  // true when r.lines[0] is the flat eviction marker
}

// NewRenderer constructs an empty Renderer. Functional options (e.g.
// WithRendererRetention) are applied in order before the renderer is returned.
// Calling NewRenderer() with no arguments is equivalent to the previous
// zero-argument signature and applies the default retention limits.
func NewRenderer(opts ...RendererOption) *Renderer {
	r := &Renderer{
		currentTurnOpenTextRow: -1,
		retentionOpts:          DefaultRendererRetentionOptions(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// AddEvent incorporates a structured event into the renderer buffer.
// The flat line buffer (Capture/CaptureRange) and the structured turn model
// (CapturePresentation) are both updated in a single locked call.
// Safe to call concurrently.
func (r *Renderer) AddEvent(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer r.enforceRetentionLocked()

	switch e.Kind {
	case EventTextDelta:
		// Flat path — unchanged.
		r.appendText(e.Text)
		// Structured path — create implicit turn if needed and append text rows.
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
		// Append one RowToolDiff per diff payload immediately after the tool row.
		for _, diff := range extractToolDiffs(e.ToolName, e.ToolInput, diffPreviewMaxLines) {
			turn.Rows = append(turn.Rows, PresentationRow{
				Kind:      RowToolDiff,
				Timestamp: e.Timestamp,
				ToolName:  e.ToolName,
				ToolDiff:  diff,
			})
		}

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
			isErr := strings.HasPrefix(line, "✗ ")
			row := PresentationRow{
				Kind:      RowResult,
				Text:      line,
				Timestamp: e.Timestamp,
				ToolName:  e.ToolName,
				IsError:   isErr,
			}
			// For commandExecution, decode exit code and output for structured
			// rendering so the pane can show a colour-coded glyph + plain output.
			if strings.EqualFold(strings.TrimSpace(e.ToolName), "commandExecution") {
				var ec struct {
					ExitCode *int   `json:"exit_code"`
					Output   string `json:"output"`
				}
				if err := json.Unmarshal([]byte(strings.TrimSpace(e.ToolResult)), &ec); err == nil && ec.ExitCode != nil {
					row.ExitCode = ec.ExitCode
					row.Output = normalizeCommandResultOutput(ec.Output)
					row.IsError = *ec.ExitCode != 0
				}
			}
			turn.Rows = append(turn.Rows, row)
			// Append a RowToolPreview row for non-error textual results that
			// are not already represented by structured command output.
			if !row.IsError && row.ExitCode == nil {
				if preview := extractToolPreview(e.ToolName, e.ToolResult, toolPreviewMaxLines); preview != nil {
					if !isRedundantToolPreview(line, preview) {
						turn.Rows = append(turn.Rows, PresentationRow{
							Kind:        RowToolPreview,
							Timestamp:   e.Timestamp,
							ToolName:    e.ToolName,
							ToolPreview: preview,
						})
					}
				}
			}
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

	case EventWarning:
		if e.Text != "" {
			r.appendLine("[warning: " + e.Text + "]")
		}
		if e.Text != "" {
			row := PresentationRow{
				Kind:      RowWarning,
				Text:      "[warning: " + e.Text + "]",
				Timestamp: e.Timestamp,
			}
			if r.currentTurn != nil {
				if e.Final {
					r.finalizeOpenTurnTextRow(r.currentTurn)
				}
				r.closeStructuredProseBlock()
				r.currentTurn.Rows = append(r.currentTurn.Rows, row)
			} else {
				r.appendStandaloneTurn(e.TurnID, e.Timestamp, row)
			}
		}
		if e.Final && r.currentTurn != nil {
			ts := e.Timestamp
			if ts.IsZero() {
				ts = time.Now()
			}
			r.finalizeOpenTurnTextRow(r.currentTurn)
			r.currentTurn.CompletedAt = ts
			r.clearCurrentTurn()
		}

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
				if e.Final {
					r.finalizeOpenTurnTextRow(r.currentTurn)
				}
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
			r.finalizeOpenTurnTextRow(r.currentTurn)
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
			r.finalizeOpenTurnTextRow(r.currentTurn)
			r.currentTurn.CompletedAt = ts
			r.clearCurrentTurn()
		}

	case EventTurnInterrupted:
		// Flat path — flush partial and add marker.
		r.appendLine("[interrupted]")
		// Structured path — mark turn interrupted and add RowStatus row.
		if r.currentTurn != nil {
			r.currentTurn.Interrupted = true
			ts := e.Timestamp
			if ts.IsZero() {
				ts = time.Now()
			}
			r.finalizeOpenTurnTextRow(r.currentTurn)
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
		t.Activity = deriveTurnActivity(t, now)
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
// operates on the structured turn's Rows slice. The first text chunk in a
// turn inserts a RowResponse sentinel before prose/code rows so renderers can
// identify where assistant text begins without inspecting row text.
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
		// No newline — extend the current partial text row.
		r.appendOpenTurnTextRow(turn, parts[0], ts)
		return
	}

	// First chunk completes the current partial text row, or is a complete
	// standalone line when no row was open.
	if r.currentTurnOpenTextRow >= 0 {
		turn.Rows[r.currentTurnOpenTextRow].Text += parts[0]
		r.finalizeOpenTurnTextRow(turn)
	} else {
		r.appendCompleteTurnTextLine(turn, parts[0], ts)
	}

	// Middle chunks are complete text rows.
	for _, p := range parts[1 : len(parts)-1] {
		r.appendCompleteTurnTextLine(turn, p, ts)
	}

	// Last chunk starts a new partial text row unless the delta ended with a
	// newline, in which case there is no open text fragment to extend.
	if tail := parts[len(parts)-1]; tail != "" {
		r.appendOpenTurnTextRow(turn, tail, ts)
	}
}

func (r *Renderer) currentTurnTextKind() PresentationRowKind {
	if r.currentTurnInCodeFence {
		return RowCodeBlock
	}
	return RowProse
}

func (r *Renderer) appendOpenTurnTextRow(turn *PresentationTurn, text string, ts time.Time) {
	if r.currentTurnOpenTextRow >= 0 {
		turn.Rows[r.currentTurnOpenTextRow].Text += text
		return
	}
	turn.Rows = append(turn.Rows, PresentationRow{
		Kind:      r.currentTurnTextKind(),
		Text:      text,
		Timestamp: ts,
	})
	r.currentTurnOpenTextRow = len(turn.Rows) - 1
}

func (r *Renderer) appendCompleteTurnTextLine(turn *PresentationTurn, text string, ts time.Time) {
	if _, ok := ParseMarkdownFenceLine(text); ok {
		r.currentTurnInCodeFence = !r.currentTurnInCodeFence
		r.currentTurnOpenTextRow = -1
		return
	}
	turn.Rows = append(turn.Rows, PresentationRow{
		Kind:      r.currentTurnTextKind(),
		Text:      text,
		Timestamp: ts,
	})
	r.currentTurnOpenTextRow = -1
}

func (r *Renderer) finalizeOpenTurnTextRow(turn *PresentationTurn) {
	if r.currentTurnOpenTextRow < 0 || r.currentTurnOpenTextRow >= len(turn.Rows) {
		r.currentTurnOpenTextRow = -1
		return
	}
	idx := r.currentTurnOpenTextRow
	if _, ok := ParseMarkdownFenceLine(turn.Rows[idx].Text); ok {
		turn.Rows = append(turn.Rows[:idx], turn.Rows[idx+1:]...)
		r.currentTurnInCodeFence = !r.currentTurnInCodeFence
	}
	r.currentTurnOpenTextRow = -1
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
	r.currentTurnOpenTextRow = -1
	r.currentTurnInCodeFence = false
	return turn
}

func (r *Renderer) clearCurrentTurn() {
	if r.currentTurn != nil {
		// Account for completed turn bytes in the structured retained total.
		r.retainedTurnBytes += turnBytes(r.currentTurn)
	}
	r.currentTurn = nil
	r.currentTurnHasResponse = false
	r.currentTurnOpenTextRow = -1
	r.currentTurnInCodeFence = false
}

// closeStructuredProseBlock marks the current text fragment as closed so the
// next text delta starts a fresh response section instead of extending prose
// that was already followed by tool/system/permission rows.
func (r *Renderer) closeStructuredProseBlock() {
	r.currentTurnOpenTextRow = -1
	r.currentTurnInCodeFence = false
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

// deepCopyTurns returns a fully independent copy of src. Delegates to
// ClonePresentationTurns which handles all nested pointer fields.
func deepCopyTurns(src []*PresentationTurn) []*PresentationTurn {
	return ClonePresentationTurns(src)
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
	newLine := r.partial + parts[0]
	r.lines = append(r.lines, newLine)
	r.retainedFlatBytes += flatLineBytes(newLine)
	// Middle chunks are complete lines.
	for _, p := range parts[1 : len(parts)-1] {
		r.lines = append(r.lines, p)
		r.retainedFlatBytes += flatLineBytes(p)
	}
	// Last chunk starts a new partial line.
	r.partial = parts[len(parts)-1]
}

// appendLine commits a full line, flushing any pending partial first.
// Must be called with r.mu held.
func (r *Renderer) appendLine(line string) {
	r.flushPartial()
	r.lines = append(r.lines, line)
	r.retainedFlatBytes += flatLineBytes(line)
}

// flushPartial moves the current partial fragment to the lines slice.
// Must be called with r.mu held.
func (r *Renderer) flushPartial() {
	if r.partial != "" {
		r.lines = append(r.lines, r.partial)
		r.retainedFlatBytes += flatLineBytes(r.partial)
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
// Eviction and truncation counters are included so a retention event is
// observable even when the visible tail text is unchanged.
func (r *Renderer) ContentHash() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	base := strings.Join(r.allLines(), "\n")
	if r.evictedTurns+r.evictedFlatLines+r.truncatedRows == 0 {
		return base
	}
	return fmt.Sprintf("%s\x00ev:%d,%d,%d,%d", base, r.evictedTurns, r.evictedFlatLines, r.evictedBytes, r.truncatedRows)
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
	cmd = strings.TrimSpace(cmd)
	for range 3 {
		inner, ok := unwrapShellCommandWrapper(cmd)
		if !ok {
			break
		}
		cmd = inner
	}
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

type shellFieldSpan struct {
	Value string
	Start int
}

func unwrapShellCommandWrapper(cmd string) (string, bool) {
	fields := shellFieldSpans(cmd)
	if len(fields) < 3 {
		return "", false
	}
	shell := filepath.Base(fields[0].Value)
	if shell != "zsh" && shell != "bash" && shell != "sh" {
		return "", false
	}
	if fields[1].Value != "-lc" && fields[1].Value != "-c" {
		return "", false
	}
	if len(fields) == 3 {
		return strings.TrimSpace(fields[2].Value), true
	}
	return strings.TrimSpace(cmd[fields[2].Start:]), true
}

func shellFieldSpans(s string) []shellFieldSpan {
	var fields []shellFieldSpan
	for i := 0; i < len(s); {
		for i < len(s) && s[i] <= ' ' {
			i++
		}
		if i >= len(s) {
			break
		}
		start := i
		var b strings.Builder
		var quote byte
		escaped := false
		for i < len(s) {
			ch := s[i]
			if escaped {
				b.WriteByte(ch)
				escaped = false
				i++
				continue
			}
			if ch == '\\' && quote != '\'' {
				escaped = true
				i++
				continue
			}
			if quote != 0 {
				if ch == quote {
					quote = 0
				} else {
					b.WriteByte(ch)
				}
				i++
				continue
			}
			if ch == '\'' || ch == '"' {
				quote = ch
				i++
				continue
			}
			if ch <= ' ' {
				break
			}
			b.WriteByte(ch)
			i++
		}
		fields = append(fields, shellFieldSpan{Value: b.String(), Start: start})
		for i < len(s) && s[i] <= ' ' {
			i++
		}
	}
	return fields
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
		if exit, ok := obj["exit_code"].(float64); ok {
			output := ""
			if v, ok2 := obj["output"].(string); ok2 {
				output = normalizeCommandResultOutput(v)
			}
			if int(exit) != 0 {
				if output != "" {
					return fmt.Sprintf("✗ exit=%d: %s", int(exit), truncateOneLine(output, 80))
				}
				return fmt.Sprintf("✗ exit=%d", int(exit))
			}
			// exit_code == 0: summarize output into the standard single-line
			// text form, or return a bare ✓ when no output so the row is still
			// appended.
			if output != "" {
				return summarizeToolResultText(output)
			}
			return "✓"
		}
		if summary := summarizeStructuredCollection(obj); summary != "" {
			return summary
		}
		if text := extractTextFromObject(obj); text != "" {
			return summarizeToolResultText(text)
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		if len(keys) > 0 {
			return "→ " + truncateOneLine(strings.Join(keys, ","), 60)
		}
		return ""
	}

	// Array payload — count items.
	var arr []any
	if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
		if text := extractTextFromValue(arr); text != "" {
			return summarizeToolResultText(text)
		}
		return fmt.Sprintf("→ %d items", len(arr))
	}

	// Plain text — compact to first line (or a short slice). Long output
	// gets a line-count summary instead of the full content.
	return summarizeToolResultText(trimmed)
}

func summarizeToolResultText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) > 3 {
		return fmt.Sprintf("→ %d lines", len(lines))
	}
	return "→ " + truncateOneLine(trimmed, 120)
}

const commandResultOutputMaxRunes = 120

func normalizeCommandResultOutput(text string) string {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return ""
	}
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	parts := strings.Split(normalized, "\n")
	collapsed := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			collapsed = append(collapsed, part)
		}
	}
	return truncateOneLine(strings.Join(collapsed, " "), commandResultOutputMaxRunes)
}

func isRedundantToolPreview(summaryLine string, preview *ToolPreviewPayload) bool {
	if preview == nil || preview.Truncated || preview.HiddenLineCount != 0 || len(preview.Lines) != 1 {
		return false
	}
	line := strings.TrimSpace(preview.Lines[0])
	return summaryLine == line || summaryLine == summarizeToolResultText(line)
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

// splitOutputLines splits output into individual lines, trimming exactly one
// trailing newline before splitting. Consecutive newlines are preserved as
// empty elements, and zero-output commands still produce a predictable
// single empty element.
func splitOutputLines(output string) []string {
	// Trim exactly one trailing newline.
	s := output
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return strings.Split(s, "\n")
}

func shellTurnStatusText(exitCode int, truncated bool, statusMsg string) string {
	if exitCode == 0 && !truncated && statusMsg == "" {
		return ""
	}
	if statusMsg != "" {
		return statusMsg
	}
	switch {
	case truncated && exitCode != 0:
		return fmt.Sprintf("exit %d · output truncated at 64 KiB", exitCode)
	case truncated:
		return "output truncated at 64 KiB"
	default:
		return fmt.Sprintf("exit %d", exitCode)
	}
}

// AddShellTurn appends a completed standalone turn containing a user row for
// the command, a response sentinel, one prose row per output line, and an
// optional status row when exitCode != 0, output was truncated, or statusMsg
// is non-empty. Does NOT interrupt any currently-open agent turn — the new
// turn is appended to r.turns with both StartedAt and CompletedAt set to now.
func (r *Renderer) AddShellTurn(command, output string, exitCode int, truncated bool, statusMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer r.enforceRetentionLocked()
	now := time.Now()
	r.nextTurnNumber++
	turn := &PresentationTurn{
		Number:      r.nextTurnNumber,
		StartedAt:   now,
		CompletedAt: now,
	}
	turn.Rows = append(turn.Rows, PresentationRow{
		Kind:      RowUser,
		Text:      "! " + command,
		Timestamp: now,
	})
	turn.Rows = append(turn.Rows, PresentationRow{
		Kind:      RowResponse,
		Timestamp: now,
	})
	outputLines := splitOutputLines(output)
	for _, line := range outputLines {
		turn.Rows = append(turn.Rows, PresentationRow{
			Kind:      RowProse,
			Text:      line,
			Timestamp: now,
		})
	}
	statusText := shellTurnStatusText(exitCode, truncated, statusMsg)
	if statusText != "" {
		turn.Rows = append(turn.Rows, PresentationRow{
			Kind:      RowStatus,
			Text:      statusText,
			Timestamp: now,
		})
	}
	// Flat line buffer: include a visible "! <cmd>" line plus output lines and
	// optional status so CapturePaneContent stays consistent with the
	// structured model.
	r.appendLine("! " + command)
	for _, line := range outputLines {
		r.appendLine(line)
	}
	if statusText != "" {
		r.appendLine(statusText)
	}
	r.turns = append(r.turns, turn)
	r.retainedTurnBytes += turnBytes(turn)
	// Do NOT assign r.currentTurn — an existing agent turn must remain open.
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
