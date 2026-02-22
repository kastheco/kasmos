package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FillBackground ensures every line in s occupies exactly `width` printable
// columns (padding short lines with bg-colored spaces) and that there are at
// least `height` total lines (appending bg-filled blank lines as needed).
//
// This is the "global background layer" — call it once on the final composed
// View string so that every cell on the terminal screen carries the theme's
// base colour. Individual component styles still set their own backgrounds
// for content cells; FillBackground catches the gaps between components,
// the vertical fill from lipgloss.Height, and any other transparent cells.
func FillBackground(s string, width, height int, bg lipgloss.TerminalColor) string {
	if width <= 0 || height <= 0 {
		return s
	}

	filler := lipgloss.NewStyle().Background(bg)
	lines := strings.Split(s, "\n")

	// Extend to target height with blank lines.
	for len(lines) < height {
		lines = append(lines, "")
	}

	// Pad each line to target width with bg-coloured spaces.
	for i := range lines {
		w := lipgloss.Width(lines[i])
		if w < width {
			lines[i] += filler.Render(strings.Repeat(" ", width-w))
		}
	}

	return strings.Join(lines, "\n")
}
