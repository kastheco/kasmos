package sdk

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Renderer accumulates structured events and renders them as line-based text.
// It is safe for concurrent use.
//
// Events are converted to text fragments and accumulated in an internal line
// buffer. The Capture and CaptureRange methods render the buffer on demand.
// CaptureRange supports the same "-"/"numeric" start/end semantics as tmux
// capture-pane -S/-E so existing callers need no changes.
type Renderer struct {
	mu      sync.Mutex
	lines   []string // completed lines
	partial string   // fragment of the current (incomplete) last line
}

// NewRenderer constructs an empty Renderer.
func NewRenderer() *Renderer {
	return &Renderer{}
}

// AddEvent incorporates a structured event into the renderer buffer.
// Safe to call concurrently.
func (r *Renderer) AddEvent(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch e.Kind {
	case EventTextDelta:
		r.appendText(e.Text)
	case EventToolCall:
		r.appendLine(formatToolCallLine(e.ToolName, e.ToolInput))
	case EventToolResult:
		// Results are hidden by default — most are long payloads (file
		// contents, command output) that drown the agent's prose, matching
		// how codex-cli's interactive UI summarises successful calls. Only
		// surface a marker when the result explicitly carries an error
		// signal so the user sees something went wrong.
		if summary := formatToolResultError(e.ToolResult); summary != "" {
			r.appendLine(summary)
		}
	case EventPermission:
		r.appendLine(fmt.Sprintf("[permission: %s]", e.PermissionDescription))
	case EventSystem:
		if e.Text != "" {
			r.appendLine("[system: " + e.Text + "]")
		}
	case EventTurnCompleted, EventTurnInterrupted:
		// Flush any pending partial line before the marker.
		r.flushPartial()
		if e.Kind == EventTurnInterrupted {
			r.lines = append(r.lines, "[interrupted]")
		}
	case EventTurnStarted:
		// No visible marker needed.
	}
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
		return truncateOneLine(trimmed, 80)
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
		return truncateOneLine(v, 80)
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

// formatToolResultError returns a compact marker when the result payload
// carries an error signal (explicit success=false, an "error" key, or a
// non-zero exit_code). All other results are suppressed to keep the pane
// focused on agent prose. Matches the codex-cli UX of only surfacing
// tool failures inline while quietly dropping successful outputs.
func formatToolResultError(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	// Look for the common failure signals in a JSON payload. If the result
	// isn't structured json we don't try to guess — better a quiet drop
	// than a false-positive error label on ordinary output.
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return ""
	}
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
	return ""
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
