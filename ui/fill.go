package ui

import (
	"strings"
)

// FillBackground ensures the output has at least `height` lines so bubbletea's
// alt-screen renderer doesn't leave stale content below the rendered view.
// Width-padding is no longer needed because OSC 11 sets the terminal's default
// background to the theme base color — unstyled cells are already correct.
func FillBackground(s string, height int) string {
	if height <= 0 {
		return s
	}

	lines := strings.Split(s, "\n")

	// Extend to target height with blank lines.
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
