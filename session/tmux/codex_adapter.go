package tmux

import (
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/session/internal/codexprompt"
)

// ErrCodexPermissionUnsupported was the original sentinel returned before codex
// permission-prompt handling was implemented. Retained for backward compatibility
// with any callers that check for it.
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

// SendPermissionResponse captures the live pane, classifies the prompt shape
// with codexprompt.Find, and sends the appropriate key sequence to codex's
// numbered permission menu.
//
// MCP shape (4-option menu):
//
//	PermissionAllowOnce   → "1" + Enter
//	PermissionAllowAlways → "3" + Enter
//	PermissionReject      → Escape
//
// Sandbox shape (3-option menu):
//
//	PermissionAllowOnce   → "1" + Enter
//	PermissionAllowAlways → "2" + Enter
//	PermissionReject      → "3" + Enter
//
// If the pane no longer matches either shape (stale or gone), a warning is
// logged and Escape is sent as a safe fallback.
func (a codexAdapter) SendPermissionResponse(session *TmuxSession, choice PermissionChoice) error {
	content, err := session.CapturePaneContent()
	if err != nil {
		return fmt.Errorf("SendPermissionResponse: capture pane: %w", err)
	}

	prompt := codexprompt.Find(ansi.Strip(content))

	sendEscape := func() error {
		cmd := exec.Command("tmux", "send-keys", "-t", session.sanitizedName, "Escape")
		if err := session.cmdExec.Run(cmd); err != nil {
			return fmt.Errorf("SendPermissionResponse: send Escape: %w", err)
		}
		return nil
	}

	if prompt == nil {
		if log.WarningLog != nil {
			log.WarningLog.Printf("SendPermissionResponse: pane no longer shows a codex permission prompt; sending Escape as fallback")
		}
		return sendEscape()
	}

	switch prompt.Shape {
	case codexprompt.ShapeMCP:
		if choice == PermissionReject {
			return sendEscape()
		}
		key := "1" // AllowOnce → option 1
		if choice == PermissionAllowAlways {
			key = "3" // Always allow → option 3
		}
		if err := session.SendKeys(key); err != nil {
			return fmt.Errorf("SendPermissionResponse: send %q: %w", key, err)
		}
		if err := session.TapEnter(); err != nil {
			return fmt.Errorf("SendPermissionResponse: confirm selection: %w", err)
		}
		return nil

	case codexprompt.ShapeSandbox:
		switch choice {
		case PermissionAllowOnce:
			if err := session.SendKeys("1"); err != nil {
				return fmt.Errorf("SendPermissionResponse: send %q: %w", "1", err)
			}
			if err := session.TapEnter(); err != nil {
				return fmt.Errorf("SendPermissionResponse: confirm selection: %w", err)
			}
		case PermissionAllowAlways:
			if err := session.SendKeys("2"); err != nil {
				return fmt.Errorf("SendPermissionResponse: send %q: %w", "2", err)
			}
			if err := session.TapEnter(); err != nil {
				return fmt.Errorf("SendPermissionResponse: confirm selection: %w", err)
			}
		case PermissionReject:
			if err := session.SendKeys("3"); err != nil {
				return fmt.Errorf("SendPermissionResponse: send %q: %w", "3", err)
			}
			if err := session.TapEnter(); err != nil {
				return fmt.Errorf("SendPermissionResponse: confirm selection: %w", err)
			}
		}
		return nil

	default:
		if log.WarningLog != nil {
			log.WarningLog.Printf("SendPermissionResponse: unrecognised codex prompt shape %q; sending Escape as fallback", prompt.Shape)
		}
		return sendEscape()
	}
}
