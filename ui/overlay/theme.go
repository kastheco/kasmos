package overlay

import "github.com/charmbracelet/lipgloss"

// Rosé Pine Moon palette — mirrors ui/theme.go.
// https://rosepinetheme.com/palette/
var (
	// Base tones
	colorBase    = lipgloss.Color("#232136")
	colorOverlay = lipgloss.Color("#393552")
	colorMuted   = lipgloss.Color("#6e6a86")
	colorSubtle  = lipgloss.Color("#908caa")
	colorText    = lipgloss.Color("#e0def4")

	// Semantic colors
	colorLove = lipgloss.Color("#eb6f92") // error, danger
	colorGold = lipgloss.Color("#f6c177") // warning
	colorFoam = lipgloss.Color("#9ccfd8") // info, running
	colorIris = lipgloss.Color("#c4a7e7") // highlight, primary
)
