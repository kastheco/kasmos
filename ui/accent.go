package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// NewAccentStrip renders a one-line full-width accent strip for the given color.
func NewAccentStrip(color string, width int) string {
	if color == "" || width <= 0 {
		return ""
	}

	return lipgloss.NewStyle().
		Background(lipgloss.Color(color)).
		Render(strings.Repeat(" ", width))
}
