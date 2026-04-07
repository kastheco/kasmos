package tmux

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/kastheco/kasmos/session/internal/claudeprompt"
)

// ProgramAdapter encapsulates program-specific behavior for readiness detection,
// prompt detection, trust-screen handling, and CLI prompt syntax.
// Implementations exist for claude and opencode; aider/gemini support is dropped.
type ProgramAdapter interface {
	// ReadyString returns the substring to look for in pane content that signals
	// the program has fully started and is ready for input.
	ReadyString() string

	// NeedsTrustTap returns true if the program shows a trust/confirmation screen
	// on first launch that requires an Enter keystroke to dismiss.
	NeedsTrustTap() bool

	// DetectPrompt returns true when the plain (ANSI-stripped) pane content
	// indicates the program is idle and waiting for user input.
	DetectPrompt(plainContent string) bool

	// MaxWaitTime returns the maximum time to wait for the program to reach the
	// ready state before giving up.
	MaxWaitTime() time.Duration

	// BuildPromptArg returns the shell argument for the program's initial-prompt flag.
	// Short prompts are inlined (shell-escaped). Long prompts are written to a temp file
	// via writeFile (which receives the prompt text and returns the absolute path), then
	// referenced using the program's file-argument syntax.
	BuildPromptArg(prompt, workDir string, writeFile func(string) string) string

	// SupportsCliPrompt reports whether this program supports receiving a prompt
	// directly from the CLI (as opposed to via send-keys after startup).
	SupportsCliPrompt() bool

	// SendPermissionResponse sends the harness-specific key sequence for a
	// permission prompt choice through the tmux session.
	SendPermissionResponse(session *TmuxSession, choice PermissionChoice) error
}

// AdapterFor returns the ProgramAdapter for the given program string, or nil
// if the program has no special adapter (unknown/unsupported program).
func AdapterFor(program string) ProgramAdapter {
	binary := programBinary(program)
	switch {
	case strings.Contains(binary, "claude"):
		return claudeAdapter{}
	case strings.Contains(binary, "opencode"):
		return opencodeAdapter{}
	default:
		return nil
	}
}

func programBinary(program string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(program)))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// claudeAdapter implements ProgramAdapter for Claude Code.
type claudeAdapter struct{}

const claudePromptTailLines = 30

func (a claudeAdapter) ReadyString() string {
	return "Do you trust the files in this folder?"
}

func (a claudeAdapter) NeedsTrustTap() bool {
	return true
}

// DetectPrompt returns true when Claude is idle at its input prompt.
// It classifies the last non-empty lines of plain (ANSI-stripped) pane content.
// Activity markers are checked first so live work suppresses stale permission
// scrollback. Permission prompts are then detected via the shared claudeprompt
// classifier (numbered-choice format). Review and composer prompts are checked last.
func (a claudeAdapter) DetectPrompt(plainContent string) bool {
	lines := claudeRecentNonEmptyLines(plainContent, claudePromptTailLines)
	if len(lines) == 0 {
		return false
	}

	if claudeHasActivityMarker(lines) {
		return false
	}

	if claudeprompt.Find(plainContent) != nil {
		return true
	}

	if claudeHasReviewPrompt(lines) {
		return true
	}

	return claudeHasComposerPrompt(lines)
}

func claudeRecentNonEmptyLines(content string, limit int) []string {
	rawLines := strings.Split(content, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) <= limit {
		return lines
	}
	return lines[len(lines)-limit:]
}

func claudeHasActivityMarker(lines []string) bool {
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "Editing "):
			return true
		case strings.HasPrefix(line, "Writing "):
			return true
		case strings.HasPrefix(line, "Reading "):
			return true
		case strings.HasPrefix(line, "Running "):
			return true
		case strings.Contains(line, "Searching"):
			return true
		case strings.HasPrefix(line, "$ "):
			return true
		}
	}

	return false
}

func claudeHasReviewPrompt(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(line, "No, and tell Claude what to do differently") {
			return true
		}
	}

	return false
}

func claudeHasComposerPrompt(lines []string) bool {
	start := len(lines) - 3
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:] {
		if line == ">" || line == "›" {
			return true
		}
	}

	return false
}

func (a claudeAdapter) MaxWaitTime() time.Duration {
	return 30 * time.Second
}

// BuildPromptArg returns the shell argument for Claude's positional prompt.
// Short prompts are single-quote escaped inline; long prompts are written to a
// file under .kasmos/ and referenced via Claude Code's @file syntax.
func (a claudeAdapter) BuildPromptArg(prompt, workDir string, writeFile func(string) string) string {
	if len(prompt) <= MaxInlinePromptLen {
		return shellEscapeSingleQuote(prompt)
	}
	path := writeFile(prompt)
	if path == "" {
		return shellEscapeSingleQuote(prompt)
	}
	rel, err := filepath.Rel(workDir, path)
	if err != nil {
		rel = path
	}
	return "@" + rel
}

func (a claudeAdapter) SupportsCliPrompt() bool {
	return true
}

// opencodeAdapter implements ProgramAdapter for OpenCode.
type opencodeAdapter struct{}

func (a opencodeAdapter) ReadyString() string {
	return "Ask anything"
}

func (a opencodeAdapter) NeedsTrustTap() bool {
	return false
}

// DetectPrompt returns true when opencode is idle and waiting for input.
// opencode shows "esc interrupt" in its bottom bar only while a task is running.
// When idle the bar disappears, so absence of "esc interrupt" signals idle state.
// The caller must pass plain (ANSI-stripped) content.
func (a opencodeAdapter) DetectPrompt(plainContent string) bool {
	return !strings.Contains(plainContent, "esc interrupt")
}

func (a opencodeAdapter) MaxWaitTime() time.Duration {
	return 30 * time.Second
}

// BuildPromptArg returns the shell argument for opencode's --prompt flag.
// Short prompts are single-quote escaped inline; long prompts are written to a
// file and read back via shell command substitution ($(cat ...)) since opencode
// has no @file syntax.
func (a opencodeAdapter) BuildPromptArg(prompt, workDir string, writeFile func(string) string) string {
	if len(prompt) <= MaxInlinePromptLen {
		return shellEscapeSingleQuote(prompt)
	}
	path := writeFile(prompt)
	if path == "" {
		return shellEscapeSingleQuote(prompt)
	}
	return "\"$(cat " + shellEscapeSingleQuote(path) + ")\""
}

func (a opencodeAdapter) SupportsCliPrompt() bool {
	return true
}
