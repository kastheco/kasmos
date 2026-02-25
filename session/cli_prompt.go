package session

import "strings"

// programSupportsCliPrompt returns true if the program supports an initial
// prompt via CLI flag (opencode --prompt) or positional arg (claude).
func programSupportsCliPrompt(program string) bool {
	return strings.HasSuffix(program, "opencode") || strings.HasSuffix(program, "claude")
}
