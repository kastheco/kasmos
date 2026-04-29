package ui

import (
	"charm.land/lipgloss/v2"
	apptheme "github.com/kastheco/kasmos/internal/theme"
	"github.com/kastheco/kasmos/ui/overlay"
)

// ApplyPalette updates the active TUI palette and rebuilds package-level style caches.
func ApplyPalette(p apptheme.Palette) {
	activePalette = p

	ColorBase = lipgloss.Color(string(p.Base))
	ColorSurface = lipgloss.Color(string(p.Surface))
	ColorOverlay = lipgloss.Color(string(p.Overlay))
	ColorMuted = lipgloss.Color(string(p.Muted))
	ColorSubtle = lipgloss.Color(string(p.Subtle))
	ColorText = lipgloss.Color(string(p.Text))
	ColorLove = lipgloss.Color(string(p.Love))
	ColorGold = lipgloss.Color(string(p.Gold))
	ColorRose = lipgloss.Color(string(p.Rose))
	ColorPine = lipgloss.Color(string(p.Pine))
	ColorFoam = lipgloss.Color(string(p.Foam))
	ColorIris = lipgloss.Color(string(p.Iris))
	GradientStart = string(p.GradientStart)
	GradientEnd = string(p.GradientEnd)

	rebuildBannerFrames()
	rebuildMenuStyles()
	rebuildNavigationPanelStyles()
	rebuildStatusBarStyles()
	rebuildInfoPaneStyles()
	rebuildAuditPaneStyles()
	rebuildTabbedWindowStyles()
	rebuildPreviewStyles()
	overlay.ApplyPalette(p)
}

// ActivePalette returns the palette currently applied to the TUI.
func ActivePalette() apptheme.Palette {
	return activePalette
}
