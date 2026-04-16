package tmux

import (
	"errors"
	"time"
)

// ErrCodexPermissionUnsupported is returned by codexAdapter.SendPermissionResponse
// because codex does not expose an interactive permission-prompt UI that kasmos can
// drive via key sequences. TmuxSession.Start launches codex with the bypass flag
// (codexBypassFlag) when SkipPermissions is enabled so this path is not reached
// in normal operation.
var ErrCodexPermissionUnsupported = errors.New("codex: permission prompt handling is not implemented")

// codexAdapter implements ProgramAdapter for the OpenAI Codex CLI.
type codexAdapter struct{}

// ReadyString returns an empty string because no stable printable startup
// banner has been confirmed from a live codex pane. TmuxSession.Start treats
// an empty ReadyString as a signal to wait out codexGracePeriod and then
// confirm the session is still alive instead of scanning pane content for a
// marker.
func (a codexAdapter) ReadyString() string {
	return ""
}

// NeedsTrustTap returns false — codex does not show a trust/confirmation
// screen on first launch.
func (a codexAdapter) NeedsTrustTap() bool {
	return false
}

// DetectPrompt returns true when the ANSI-stripped pane content contains a
// recognisable codex idle affordance. The implementation is deliberately
// conservative: no reliable idle marker has been confirmed from a live pane,
// so false is returned until one is established.
func (a codexAdapter) DetectPrompt(plainContent string) bool {
	lines := claudeRecentNonEmptyLines(plainContent, 10)
	if len(lines) == 0 {
		return false
	}
	// No repeatable idle marker confirmed from a live codex pane yet.
	// Leave this conservative until live capture identifies one; the
	// grace-period fallback in TmuxSession.Start keeps startup unblocked
	// in the meantime.
	return false
}

// MaxWaitTime returns the maximum time to wait for codex to reach the ready
// state. 30 seconds matches the other adapters; live startup data may prompt
// an increase in a later task.
func (a codexAdapter) MaxWaitTime() time.Duration {
	return 30 * time.Second
}

// BuildPromptArg returns the shell argument for codex's positional prompt
// (codex [PROMPT]). Short prompts are inlined with single-quote escaping.
// Long prompts are written to a temp file under <workDir>/.kasmos/ via
// writeFile and referenced with shell command substitution ($(cat ...)),
// matching the opencode long-prompt pattern.
func (a codexAdapter) BuildPromptArg(prompt, workDir string, writeFile func(string) string) string {
	if len(prompt) <= MaxInlinePromptLen {
		return shellEscapeSingleQuote(prompt)
	}
	path := writeFile(prompt)
	if path == "" {
		return shellEscapeSingleQuote(prompt)
	}
	return "\"$(cat " + shellEscapeSingleQuote(path) + ")\""
}

// SupportsCliPrompt returns true — codex accepts a prompt directly on the
// command line.
func (a codexAdapter) SupportsCliPrompt() bool {
	return true
}

// SendPermissionResponse returns ErrCodexPermissionUnsupported because codex
// does not expose an interactive permission prompt that kasmos can drive via
// key sequences. When SkipPermissions is set, TmuxSession.Start passes
// codexBypassFlag at launch time so permissions never surface in-pane.
func (a codexAdapter) SendPermissionResponse(session *TmuxSession, choice PermissionChoice) error {
	return ErrCodexPermissionUnsupported
}
