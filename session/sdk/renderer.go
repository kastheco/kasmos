package sdk

import (
	"fmt"
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
		r.appendLine(fmt.Sprintf("[tool: %s %s]", e.ToolName, e.ToolInput))
	case EventToolResult:
		r.appendLine(fmt.Sprintf("[result: %s]", e.ToolResult))
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
