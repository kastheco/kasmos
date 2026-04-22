package sdk

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// ToolCallIndent is the indentation applied to tool-call lines beneath an assistant turn.
const ToolCallIndent = "  "

// ToolChildIndent is the indentation applied to tool-result / child lines nested under a tool call.
const ToolChildIndent = "    "

// SplitToolCallText splits a rendered tool-call text line into a head (bullet + tool name) and
// the remaining args portion. toolName is trimmed before matching.
//
// Splitting rules:
//   - Returns (text, "") when text == "", trimmedToolName == "", or text does not start with "• <name>".
//   - Returns ("• <name>", "") when text == "• <name>" exactly.
//   - Returns ("• <name>", text[len("• <name>")+1:]) when text starts with "• <name> ".
func SplitToolCallText(text, toolName string) (head, args string) {
	trimmed := strings.TrimSpace(toolName)
	if text == "" || trimmed == "" {
		return text, ""
	}
	prefix := "• " + trimmed
	if !strings.HasPrefix(text, prefix) {
		return text, ""
	}
	if text == prefix {
		return prefix, ""
	}
	if strings.HasPrefix(text, prefix+" ") {
		return prefix, text[len(prefix)+1:]
	}
	return text, ""
}

// RenderToolCallLine renders a split tool-call line applying headStyle to head and argsStyle to args.
//
//   - Returns "" when both head and args are empty.
//   - Returns headStyle.Render(head) when args is empty.
//   - Otherwise returns headStyle.Render(head) + " " + argsStyle.Render(args).
func RenderToolCallLine(head, args string, headStyle, argsStyle lipgloss.Style) string {
	if head == "" && args == "" {
		return ""
	}
	if args == "" {
		return headStyle.Render(head)
	}
	return headStyle.Render(head) + " " + argsStyle.Render(args)
}

// RenderPromptLine renders a one-token prefix followed by a content segment.
// It is used for quiet transcript rows like user prompts where the prefix
// should carry a stronger accent than the body text.
func RenderPromptLine(prefix, text string, prefixStyle, textStyle lipgloss.Style) string {
	if text == "" {
		return prefixStyle.Render(prefix)
	}
	return prefixStyle.Render(prefix) + " " + textStyle.Render(text)
}

// RenderPromptLineWithTimestamp renders a prompt-style transcript row with an
// optional subtle timestamp aligned to the right edge when width allows.
func RenderPromptLineWithTimestamp(prefix, text string, ts time.Time, width int, prefixStyle, textStyle, timestampStyle lipgloss.Style) string {
	return renderLineWithTimestamp(RenderPromptLine(prefix, text, prefixStyle, textStyle), ts, width, timestampStyle)
}

// RenderTextLineWithTimestamp renders a plain transcript row with an optional
// subtle timestamp aligned to the right edge when width allows.
func RenderTextLineWithTimestamp(text string, ts time.Time, width int, textStyle, timestampStyle lipgloss.Style) string {
	return renderLineWithTimestamp(textStyle.Render(text), ts, width, timestampStyle)
}

// RenderStructuredChildLine renders rows that begin with the standard "│ "
// gutter used by preview and diff blocks, allowing callers to accent the
// gutter separately from the row content.
func RenderStructuredChildLine(row string, gutterStyle, contentStyle lipgloss.Style) string {
	if strings.HasPrefix(row, "│ ") {
		return gutterStyle.Render("│ ") + contentStyle.Render(strings.TrimPrefix(row, "│ "))
	}
	return contentStyle.Render(row)
}

func renderLineWithTimestamp(base string, ts time.Time, width int, timestampStyle lipgloss.Style) string {
	label := formatTranscriptTimestamp(ts)
	if label == "" {
		return base
	}

	styledLabel := timestampStyle.Render(label)
	if width <= 0 {
		return base + " " + styledLabel
	}

	baseWidth := lipgloss.Width(base)
	labelWidth := lipgloss.Width(label)
	if baseWidth+1+labelWidth <= width {
		return base + strings.Repeat(" ", width-baseWidth-labelWidth) + styledLabel
	}
	if labelWidth < width {
		return base + "\n" + strings.Repeat(" ", width-labelWidth) + styledLabel
	}
	return base + " " + styledLabel
}

func formatTranscriptTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Local().Format("15:04")
}
