// Package codexprompt provides a shared classifier for codex CLI permission
// prompts. Both the session-layer parser and the tmux adapter use this package
// so detection stays consistent across prompt shape variants.
package codexprompt

import "strings"

// Shape identifies the structural variant of a codex permission prompt.
type Shape string

const (
	// ShapeMCP is the 4-option MCP tool-approval menu:
	// 1. Allow / 2. Allow for this session / 3. Always allow / 4. Cancel
	ShapeMCP Shape = "codex-mcp"

	// ShapeSandbox is the 3-option sandbox-access menu:
	// 1. Allow / 2. Allow always / 3. <reject-variant> (no 4. option)
	ShapeSandbox Shape = "codex-sandbox"
)

// Prompt represents a detected codex permission prompt.
type Prompt struct {
	// Description is the human-readable question line (e.g. `Allow the kasmos
	// MCP server to run tool "read_file"?`), or empty when the question line
	// could not be located above the menu.
	Description string
	// Shape identifies which prompt variant was matched.
	Shape Shape
}

// Find scans plainContent (ANSI-stripped pane text) for a codex permission
// prompt. It returns non-nil when exactly one of the supported shapes is
// detected, nil otherwise.
//
// Two shapes are recognised:
//
//  1. MCP shape — 4-option numbered menu ending with "enter to submit":
//     1. Allow / 2. Allow for this session / 3. Always allow / 4. Cancel
//
//  2. Sandbox shape — 3-option numbered menu (no 4. option):
//     1. Allow / 2. Allow always / 3. <any reject-variant text>
func Find(plainContent string) *Prompt {
	lines := strings.Split(plainContent, "\n")

	// Locate the last "enter to submit" footer — codex's most distinctive
	// structural marker.
	footerIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(strings.TrimSpace(lines[i]), "enter to submit") {
			footerIdx = i
			break
		}
	}
	if footerIdx < 0 {
		return nil
	}

	// Stale-prompt guard: if non-empty transcript content appears after the
	// footer, the menu belongs to an already-answered prompt in the scrollback.
	for i := footerIdx + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return nil
		}
	}

	// Inspect at most 8 lines above the footer (matches the existing parser's
	// intent) for the numbered option lines.
	windowStart := footerIdx - 8
	if windowStart < 0 {
		windowStart = 0
	}

	has1Allow := false
	has2AllowSession := false
	has2AllowAlways := false
	has3 := false
	has3AlwaysAllow := false
	has4Cancel := false

	for i := windowStart; i < footerIdx; i++ {
		t := normalizeOptionLine(strings.TrimSpace(lines[i]))
		lower := strings.ToLower(t)

		switch {
		case strings.HasPrefix(t, "1.") && strings.Contains(lower, "allow"):
			has1Allow = true
		case strings.HasPrefix(t, "2.") && strings.Contains(lower, "for this session"):
			has2AllowSession = true
		case strings.HasPrefix(t, "2.") && strings.Contains(lower, "allow always"):
			has2AllowAlways = true
		case strings.HasPrefix(t, "3."):
			has3 = true
			if strings.Contains(lower, "always allow") {
				has3AlwaysAllow = true
			}
		case strings.HasPrefix(t, "4.") && strings.Contains(lower, "cancel"):
			has4Cancel = true
		}
	}

	var shape Shape
	switch {
	case has1Allow && has2AllowSession && has3AlwaysAllow && has4Cancel:
		shape = ShapeMCP
	case has1Allow && has2AllowAlways && has3 && !has4Cancel:
		shape = ShapeSandbox
	default:
		return nil
	}

	// Description: walk backward from the footer to the nearest question line
	// (ends with "?"), stopping at a "Field " header boundary. Option lines
	// are traversed silently — they do not end with "?" and do not start with
	// "Field ".
	description := ""
	for i := footerIdx - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		if strings.HasSuffix(t, "?") {
			description = t
			break
		}
		if strings.HasPrefix(t, "Field ") {
			break
		}
	}

	return &Prompt{Description: description, Shape: shape}
}

// normalizeOptionLine strips leading cursor/selection markers (›, », ❯) and
// surrounding whitespace from a trimmed option line so that prefix checks on
// the option number are reliable regardless of which item is currently selected.
func normalizeOptionLine(t string) string {
	// Strip common codex TUI cursor markers followed by optional whitespace.
	t = strings.TrimLeft(t, "›»❯")
	return strings.TrimSpace(t)
}
