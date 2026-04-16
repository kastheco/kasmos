// Package common provides shared helpers used by session sub-packages
// (session/tmux, session/headless, session/sdk) that cannot live in the parent
// session package without creating import cycles.
package common

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kastheco/kasmos/config"
)

// ProgramKind identifies a supported agent program family.
type ProgramKind string

const (
	// ProgramUnknown is returned when the program cannot be identified.
	ProgramUnknown ProgramKind = ""
	// ProgramClaude identifies Claude Code.
	ProgramClaude ProgramKind = "claude"
	// ProgramCodex identifies the OpenAI Codex CLI.
	ProgramCodex ProgramKind = "codex"
	// ProgramOpenCode identifies OpenCode.
	ProgramOpenCode ProgramKind = "opencode"
)

var whiteSpaceRegex = regexp.MustCompile(`\s+`)

// ProgramBase returns the base executable name from the first
// whitespace-delimited token in program.
//
//	"claude --model opus"   → "claude"
//	"/usr/local/bin/codex"  → "codex"
//	""                      → ""
func ProgramBase(program string) string {
	trimmed := strings.TrimSpace(program)
	if trimmed == "" {
		return ""
	}
	exe := trimmed
	if i := strings.IndexAny(trimmed, " \t"); i >= 0 {
		exe = trimmed[:i]
	}
	return filepath.Base(exe)
}

// DetectProgramKind returns the ProgramKind for the given program string.
// Detection is case-insensitive and based on the base executable name only.
func DetectProgramKind(program string) ProgramKind {
	base := strings.ToLower(ProgramBase(program))
	switch {
	case strings.Contains(base, string(ProgramClaude)):
		return ProgramClaude
	case strings.Contains(base, string(ProgramCodex)):
		return ProgramCodex
	case strings.Contains(base, string(ProgramOpenCode)):
		return ProgramOpenCode
	default:
		return ProgramUnknown
	}
}

// ResolveExecutable resolves a bare command token to its full path via the
// user's shell config and PATH. Tokens that already contain a path separator
// or are empty are returned unchanged.
func ResolveExecutable(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" || strings.Contains(trimmed, "/") {
		return trimmed
	}
	resolved, err := config.ResolveCommandPath(trimmed)
	if err != nil || resolved == "" {
		return trimmed
	}
	return resolved
}

// SanitizeSessionName converts a human-readable session name into a safe
// identifier suitable for use in file names and tmux session names.
// Whitespace is stripped and dots are replaced with underscores.
func SanitizeSessionName(name string) string {
	s := whiteSpaceRegex.ReplaceAllString(name, "")
	s = strings.ReplaceAll(s, ".", "_")
	return s
}

// SupportsCLIPrompt reports whether the program accepts an initial prompt
// on the command line (as opposed to requiring interactive input after startup).
func SupportsCLIPrompt(program string) bool {
	switch DetectProgramKind(program) {
	case ProgramClaude, ProgramCodex, ProgramOpenCode:
		return true
	default:
		return false
	}
}
