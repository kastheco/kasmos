package ui

import (
	"charm.land/lipgloss/v2"
	apptheme "github.com/kastheco/kasmos/internal/theme"
)

var (
	activePalette = apptheme.DefaultPalette()

	// Base tones
	ColorBase    = lipgloss.Color(string(activePalette.Base))
	ColorSurface = lipgloss.Color(string(activePalette.Surface))
	ColorOverlay = lipgloss.Color(string(activePalette.Overlay))
	ColorMuted   = lipgloss.Color(string(activePalette.Muted))
	ColorSubtle  = lipgloss.Color(string(activePalette.Subtle))
	ColorText    = lipgloss.Color(string(activePalette.Text))

	// Semantic colors
	ColorLove = lipgloss.Color(string(activePalette.Love)) // error, danger
	ColorGold = lipgloss.Color(string(activePalette.Gold)) // warning
	ColorRose = lipgloss.Color(string(activePalette.Rose)) // accent, secondary
	ColorPine = lipgloss.Color(string(activePalette.Pine)) // link
	ColorFoam = lipgloss.Color(string(activePalette.Foam)) // info, running
	ColorIris = lipgloss.Color(string(activePalette.Iris)) // highlight, primary

	// Gradient endpoints for the banner and focused tab label
	GradientStart = string(activePalette.GradientStart)
	GradientEnd   = string(activePalette.GradientEnd)
)
