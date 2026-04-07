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
	hasYes := false
	hasNo := false

	for i, line := range lines {
		t := strings.TrimSpace(line)
		if isNumberedYes(t) {
			hasYes = true
			if firstChoiceIdx < 0 {
				firstChoiceIdx = i
			}
		} else if isNumberedNo(t) {
			hasNo = true
		}
	}

	if !hasYes || !hasNo || firstChoiceIdx < 0 {
		return nil
	}

	description := findDescriptionBefore(lines, firstChoiceIdx)
	pattern := extractPatternFromChoices(lines, firstChoiceIdx)
	if pattern == "" {
		pattern = description
	}

	return &Prompt{Description: description, Pattern: pattern}
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

		description := extractToolFromQuestion(t)
		return &Prompt{Description: description, Pattern: description}
	}
	return nil
}

// findDescriptionBefore returns the tool-detail line immediately preceding the
// first numbered choice. It skips blank lines and question lines (ending with
// "?"). Lines with no structural marker (colon or slash) are treated as generic
// headers and skipped; if no structured line is found a second pass picks the
// closest non-empty line regardless.
func findDescriptionBefore(lines []string, beforeIdx int) string {
	// First pass: prefer structured lines (contain ":" or "/").
	for i := beforeIdx - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		if strings.HasSuffix(t, "?") {
			continue
		}
		if strings.Contains(t, ":") || strings.Contains(t, "/") {
			return t
		}
	}

	// Second pass: accept any non-empty, non-question line.
	for i := beforeIdx - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		if strings.HasSuffix(t, "?") {
			// Try to extract from "Allow tool X?" question.
			if lower := strings.ToLower(t); strings.Contains(lower, "allow tool ") {
				return extractToolFromQuestion(t)
			}
			continue
		}
		return t
	}
	return ""
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
func isNumberedChoice(trimmed, word string) bool {
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
