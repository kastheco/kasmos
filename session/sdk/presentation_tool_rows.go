package sdk

import (
	"strings"

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
