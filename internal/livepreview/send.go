package livepreview

import (
	"context"
	"strings"
)

// SendPrompt sends a text prompt to the running instance's tmux pane.
//
// It normalises \r\n to \n, splits on \n, and for each line issues:
//   - non-empty: tmux send-keys -l -t <session> <line>, then tmux send-keys -t <session> Enter
//   - empty:     tmux send-keys -t <session> Enter  (bare Enter tap)
//
// This mirrors the tmux semantics from session/tmux/pane_io.go SendKeys +
// TapEnter, extended to multi-line prompts.
//
// Error classification mirrors CapturePane via runPaneCommand:
//   - ErrSessionGone when the session is gone
//   - *CommandError for other non-zero tmux exits
//   - raw error otherwise
func SendPrompt(ctx context.Context, runner PaneRunner, rec Record, prompt string) error {
	session := SessionName(rec.Title)

	// Normalise Windows-style line endings before splitting.
	prompt = strings.ReplaceAll(prompt, "\r\n", "\n")
	lines := strings.Split(prompt, "\n")

	for _, line := range lines {
		if line != "" {
			// Send the line content literally (no tmux key-name interpretation).
			if _, err := runPaneCommand(ctx, runner, "send-keys", "-l", "-t", session, line); err != nil {
				return err
			}
		}
		// Tap Enter after every line (empty or not).
		if _, err := runPaneCommand(ctx, runner, "send-keys", "-t", session, "Enter"); err != nil {
			return err
		}
	}
	return nil
}
