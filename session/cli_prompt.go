package session

import "strings"

// programSupportsCliPrompt returns true if the program supports an initial
// prompt via CLI flag (opencode --prompt) or positional arg (claude).
// NOTE: These match session/tmux.ProgramOpenCode and session/tmux.ProgramClaude.
// Keep in sync if program names change (can't import tmux — circular dep).
func programSupportsCliPrompt(program string) bool {
	return strings.HasSuffix(program, "opencode") || strings.HasSuffix(program, "claude")
}
