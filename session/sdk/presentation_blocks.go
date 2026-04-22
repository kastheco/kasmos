package sdk

import (
	"fmt"
	"strings"
	"time"
)

// BuildToolDiffBlock renders a ToolDiffPayload into display rows ready for
// caller-specific colour styling. Each row is prefixed with a muted │ gutter,
// followed by a right-aligned line number, a +/- (or space) marker, and the
// line text.  The returned slice contains no ANSI codes; callers apply their
// own lipgloss styling per line.
//
// When payload.Truncated && payload.HiddenLineCount > 0, a final "│ … +N more
// lines" row is appended.
func BuildToolDiffBlock(payload *ToolDiffPayload, width int) []string {
	if payload == nil {
		return nil
	}

	// Determine the widest line number for column alignment.
	maxNum := 0
	for _, dl := range payload.Lines {
		if dl.OldNumber != nil && *dl.OldNumber > maxNum {
			maxNum = *dl.OldNumber
		}
		if dl.NewNumber != nil && *dl.NewNumber > maxNum {
			maxNum = *dl.NewNumber
		}
	}
	numWidth := len(fmt.Sprintf("%d", maxNum))
	if numWidth < 1 {
		numWidth = 1
	}

	rows := make([]string, 0, len(payload.Lines)+1)

	for _, dl := range payload.Lines {
		var sb strings.Builder
		sb.WriteString("│ ")

		switch dl.Kind {
		case DiffLineRemoved:
			num := blankNum(dl.OldNumber, numWidth)
			sb.WriteString(num)
			sb.WriteString(" - ")
			sb.WriteString(dl.OldText)
		case DiffLineAdded:
			num := blankNum(dl.NewNumber, numWidth)
			sb.WriteString(num)
			sb.WriteString(" + ")
			sb.WriteString(dl.NewText)
		default: // DiffLineContext
			num := blankNum(dl.OldNumber, numWidth)
			sb.WriteString(num)
			sb.WriteString("   ")
			sb.WriteString(dl.OldText)
		}

		rows = append(rows, sb.String())
	}

	if payload.Truncated && payload.HiddenLineCount > 0 {
		rows = append(rows, fmt.Sprintf("│ … +%d more lines", payload.HiddenLineCount))
	}

	return rows
}

// blankNum formats n as a right-aligned decimal string of width w.
// If n is nil it returns a string of w spaces.
func blankNum(n *int, w int) string {
	if n == nil {
		return strings.Repeat(" ", w)
	}
	return fmt.Sprintf("%*d", w, *n)
}

// BuildToolPreviewBlock renders a ToolPreviewPayload into display rows ready
// for caller-specific colour styling. Each line is prefixed with "│ ".
// Display rows are capped to toolPreviewMaxLines so older cached payloads do
// not exceed the current preview limit. When hidden lines remain, a trailing
// "│ … +N more lines" row is appended. The returned rows contain no ANSI
// codes.
func BuildToolPreviewBlock(payload *ToolPreviewPayload, width int) []string {
	if payload == nil {
		return nil
	}
	lines := payload.Lines
	hiddenLineCount := payload.HiddenLineCount
	if len(lines) > toolPreviewMaxLines {
		hiddenLineCount += len(lines) - toolPreviewMaxLines
		lines = lines[:toolPreviewMaxLines]
	}

	rows := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		rows = append(rows, "│ "+line)
	}
	if hiddenLineCount > 0 {
		rows = append(rows, fmt.Sprintf("│ … +%d more lines", hiddenLineCount))
	}
	return rows
}

// FormatActivityLabel formats a TurnActivity into a compact one-line label
// suitable for a pinned sticky strip. In narrow mode the activity label text is
// suppressed and only the elapsed clock is shown, e.g. "✺ 00:12". In normal
// mode both label and clock are shown, e.g. "✺ editing renderer.go  00:12".
// Returns an empty string when activity is nil.
func FormatActivityLabel(activity *TurnActivity, now time.Time, narrow bool) string {
	if activity == nil {
		return ""
	}

	elapsed := time.Duration(0)
	if !activity.StartedAt.IsZero() {
		elapsed = now.Sub(activity.StartedAt)
		if elapsed < 0 {
			elapsed = 0
		}
	}
	elapsedStr := formatElapsedClock(elapsed)

	if narrow {
		return "✺ " + elapsedStr
	}

	label := activity.Label
	if label == "" {
		label = activity.Kind
	}
	return "✺ " + label + "  " + elapsedStr
}

// formatElapsedClock formats a duration as MM:SS (or H:MM:SS for durations ≥
// 1 hour). This is used for the sticky activity strip where a clock-style
// display is more readable than the prose "5m30s" format used for turn headers.
func formatElapsedClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
