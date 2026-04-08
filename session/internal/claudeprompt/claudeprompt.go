// Package claudeprompt provides a shared classifier for Claude Code's
// numbered-choice permission prompts. Both the session-layer parser and the
// tmux adapter use this package so detection stays consistent as Claude's UI
// evolves.
package claudeprompt

import "strings"

// Prompt represents a detected Claude Code permission prompt.
// Description is the human-readable tool invocation detail (e.g. "Bash: git status").
// Pattern is the allow-rule pattern from the "don't ask again" option (e.g. "go vet:*"),
// or the Description itself when no explicit pattern is present.
type Prompt struct {
	Description string
	Pattern     string
}

// tailRawLines is the maximum number of raw (including blank) lines examined
// from the end of the content.
const tailRawLines = 30

// Find scans plainContent (ANSI-stripped pane text) for a Claude Code
// permission prompt. It returns non-nil when a permission prompt is detected,
// nil otherwise.
//
// Two formats are recognised:
//
//  1. Numbered-choice format (current Claude Code):
//     Lines starting with "N. Yes" and "N. No" (e.g. "1. Yes, allow once",
//     "2. Yes, allow always", "3. No, and tell Claude what to do differently").
//     Blank lines between the header and the choices are tolerated.
//
//  2. Legacy Yes/No format:
//     Standalone "Yes" and "No" lines appearing within a few lines of a
//     recognised permission question ("Allow tool X?" or "Do you want to proceed?").
func Find(plainContent string) *Prompt {
	rawLines := strings.Split(plainContent, "\n")
	if len(rawLines) > tailRawLines {
		rawLines = rawLines[len(rawLines)-tailRawLines:]
	}

	if p := findNumberedPrompt(rawLines); p != nil {
		return p
	}
	return findLegacyPrompt(rawLines)
}

// findNumberedPrompt detects the current Claude Code numbered-choice dialog.
func findNumberedPrompt(lines []string) *Prompt {
	firstChoiceIdx := -1
	lastChoiceIdx := -1
	hasYes := false
	hasNo := false

	for i, line := range lines {
		t := strings.TrimSpace(line)
		if isNumberedYes(t) {
			hasYes = true
			if firstChoiceIdx < 0 {
				firstChoiceIdx = i
			}
			lastChoiceIdx = i
		} else if isNumberedNo(t) {
			hasNo = true
			lastChoiceIdx = i
		}
	}

	if !hasYes || !hasNo || firstChoiceIdx < 0 {
		return nil
	}

	// An active permission prompt sits at the bottom of the pane. If
	// substantial content follows the last numbered choice, the choices
	// are stale scrollback from an already-answered prompt.
	if !isPromptAtBottom(lines, lastChoiceIdx) {
		return nil
	}

	// Reject Claude Code's trust screen ("Do you trust the files in this
	// folder?") which has numbered Yes/No choices but is not a permission
	// prompt — kasmos handles it via the trust-tap mechanism.
	if isTrustScreenBefore(lines, firstChoiceIdx) {
		return nil
	}

	questionIdx, hasQuestion := findPermissionQuestionBefore(lines, firstChoiceIdx)
	description := ""
	if hasQuestion {
		description = findDescriptionForQuestion(lines, questionIdx, strings.TrimSpace(lines[questionIdx]))
	} else {
		description = findStructuredDescriptionBefore(lines, firstChoiceIdx)
	}

	// When a recognized permission question appears with numbered choices
	// at the pane bottom, the structural evidence is definitive — accept
	// even without a colon-prefixed tool description. Fall back to any
	// non-empty, non-question content line within a small window above
	// the question.
	if description == "" && hasQuestion {
		start := questionIdx - maxDescriptionDistance
		if start < 0 {
			start = 0
		}
		for i := questionIdx - 1; i >= start; i-- {
			t := strings.TrimSpace(lines[i])
			if t != "" && !strings.HasSuffix(t, "?") {
				description = t
				break
			}
		}
	}

	if description == "" {
		return nil
	}

	pattern := extractPatternFromChoices(lines, firstChoiceIdx)
	if pattern == "" {
		pattern = description
	}

	return &Prompt{Description: description, Pattern: pattern}
}

// isPromptAtBottom returns true when nothing but blank lines and footer
// chrome (keyboard-hint bars) follows the last numbered choice. This
// prevents stale prompts in the scrollback from triggering detection.
func isPromptAtBottom(lines []string, lastChoiceIdx int) bool {
	for i := lastChoiceIdx + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" || isFooterChrome(t) {
			continue
		}
		return false
	}
	return true
}

// isTrustScreenBefore returns true when the lines immediately before
// the first numbered choice contain Claude Code's workspace trust
// question ("Do you trust the files in this folder?"). This dialog
// uses the same Yes/No format as permission prompts but is handled
// separately via the trust-tap mechanism.
func isTrustScreenBefore(lines []string, firstChoiceIdx int) bool {
	start := firstChoiceIdx - 6
	if start < 0 {
		start = 0
	}
	for i := firstChoiceIdx - 1; i >= start; i-- {
		if strings.Contains(strings.ToLower(strings.TrimSpace(lines[i])), "do you trust the files") {
			return true
		}
	}
	return false
}

// isFooterChrome returns true for Claude Code's bottom-bar lines
// (e.g. "Esc to cancel · Tab to amend · ctrl+e to explain").
func isFooterChrome(line string) bool {
	return strings.Contains(line, "Esc to cancel") ||
		strings.Contains(line, "ctrl+e to explain") ||
		strings.Contains(line, "Tab to amend")
}

// findLegacyPrompt detects the old Claude Code "Yes" / "No" permission dialog
// where choices appear as standalone lines without numeric prefixes.
func findLegacyPrompt(lines []string) *Prompt {
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !isPermissionQuestion(t) {
			continue
		}

		// Look for standalone Yes and No within the next four lines.
		hasYes, hasNo := false, false
		end := i + 5
		if end > len(lines) {
			end = len(lines)
		}
		for _, nearby := range lines[i+1 : end] {
			nt := strings.TrimSpace(nearby)
			if strings.HasPrefix(nt, "Yes") {
				hasYes = true
			}
			if strings.HasPrefix(nt, "No") {
				hasNo = true
			}
		}
		if !hasYes || !hasNo {
			continue
		}

		description := findDescriptionForQuestion(lines, i, t)
		if description == "" {
			continue
		}
		return &Prompt{Description: description, Pattern: description}
	}
	return nil
}

// findDescriptionForQuestion returns the human-readable tool detail for a prompt.
// "Allow tool ...?" embeds the detail in the question itself, while
// "Do you want to proceed?" relies on the closest preceding structured detail
// line (for example "Bash: git status").
func findDescriptionForQuestion(lines []string, questionIdx int, question string) string {
	if strings.EqualFold(strings.TrimSpace(question), "Do you want to proceed?") {
		return findStructuredDescriptionBefore(lines, questionIdx)
	}
	return extractToolFromQuestion(question)
}

// maxDescriptionDistance is the maximum number of lines above the question
// or first choice that findStructuredDescriptionBefore will search. Claude
// Code always places the tool header within a few lines of the prompt — a
// larger window picks up unrelated file content (e.g. "47 [coverage:run]"
// from a config file diff) and produces wrong descriptions.
const maxDescriptionDistance = 8

// findStructuredDescriptionBefore returns the closest preceding structured
// detail line before beforeIdx, searching at most maxDescriptionDistance
// lines. Generic prose is intentionally rejected so that quoted transcript
// text like "This command requires approval" does not look like a live
// permission dialog.
func findStructuredDescriptionBefore(lines []string, beforeIdx int) string {
	start := beforeIdx - maxDescriptionDistance
	if start < 0 {
		start = 0
	}
	for i := beforeIdx - 1; i >= start; i-- {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		if strings.HasSuffix(t, "?") {
			if isPermissionQuestion(t) && !strings.EqualFold(t, "Do you want to proceed?") {
				return extractToolFromQuestion(t)
			}
			continue
		}
		if isStructuredDescriptionLine(t) {
			return t
		}
	}
	return ""
}

func findPermissionQuestionBefore(lines []string, beforeIdx int) (int, bool) {
	start := beforeIdx - 6
	if start < 0 {
		start = 0
	}
	for i := beforeIdx - 1; i >= start; i-- {
		if isPermissionQuestion(strings.TrimSpace(lines[i])) {
			return i, true
		}
	}
	return 0, false
}

func isStructuredDescriptionLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if idx := strings.Index(trimmed, ":"); idx > 0 && idx < len(trimmed)-1 {
		prefixWords := strings.Fields(strings.TrimSpace(trimmed[:idx]))
		if len(prefixWords) >= 1 && len(prefixWords) <= 2 {
			return true
		}
	}

	return strings.HasPrefix(trimmed, "/") ||
		strings.HasPrefix(trimmed, "./") ||
		strings.HasPrefix(trimmed, "../")
}

// extractPatternFromChoices scans numbered choices starting at from for the
// "don't ask again for: <pattern>" substring and returns the pattern, or ""
// if not found.
func extractPatternFromChoices(lines []string, from int) string {
	const marker = "don't ask again for: "
	for _, line := range lines[from:] {
		t := strings.TrimSpace(line)
		lower := strings.ToLower(t)
		if idx := strings.Index(lower, marker); idx >= 0 {
			return strings.TrimSpace(t[idx+len(marker):])
		}
	}
	return ""
}

// isPermissionQuestion returns true for lines that are a recognised Claude
// permission question (must end with "?" and contain a known anchor phrase).
func isPermissionQuestion(line string) bool {
	if !strings.HasSuffix(line, "?") {
		return false
	}
	lower := strings.ToLower(line)
	return strings.Contains(lower, "allow tool ") ||
		strings.Contains(lower, "do you want to proceed")
}

// extractToolFromQuestion pulls the tool name from "Allow tool X?" lines.
// Returns the trimmed remainder after the "allow tool " prefix, with the
// trailing "?" removed.
func extractToolFromQuestion(line string) string {
	lower := strings.ToLower(line)
	const prefix = "allow tool "
	idx := strings.Index(lower, prefix)
	if idx < 0 {
		return strings.TrimSuffix(strings.TrimSpace(line), "?")
	}
	rest := line[idx+len(prefix):]
	return strings.TrimSpace(strings.TrimSuffix(rest, "?"))
}

// isNumberedYes reports whether trimmed is a numbered "Yes" choice
// (e.g. "1. Yes, allow once").
func isNumberedYes(trimmed string) bool {
	return isNumberedChoice(trimmed, "Yes")
}

// isNumberedNo reports whether trimmed is a numbered "No" choice
// (e.g. "3. No, and tell Claude what to do differently").
func isNumberedNo(trimmed string) bool {
	return isNumberedChoice(trimmed, "No")
}

// isNumberedChoice reports whether trimmed begins with a single digit, a
// period, a space, and then the given word prefix (e.g. "1. Yes, allow once").
// Cursor/chrome prefixes such as "❯ ", "> ", or ") " before the digit are
// stripped before checking.
func isNumberedChoice(trimmed, word string) bool {
	// Strip TUI cursor/chrome prefixes.
	for _, pfx := range []string{"❯ ", ") ", "> "} {
		if strings.HasPrefix(trimmed, pfx) {
			trimmed = trimmed[len(pfx):]
			break
		}
	}
	// Minimum: "1. Y" or "1. N" = 4 chars
	if len(trimmed) < 4 {
		return false
	}
	if trimmed[0] < '1' || trimmed[0] > '9' {
		return false
	}
	if trimmed[1] != '.' || trimmed[2] != ' ' {
		return false
	}
	return strings.HasPrefix(trimmed[3:], word)
}
