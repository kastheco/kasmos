package ui

import "github.com/charmbracelet/lipgloss"

var fallbackBannerRaw = `██╗  ██╗██╗     ██╗ ██████╗ ██╗   ██╗███████╗
██║ ██╔╝██║     ██║██╔═══██╗██║   ██║██╔════╝
█████╔╝ ██║     ██║██║   ██║██║   ██║█████╗
██╔═██╗ ██║     ██║██║▄▄ ██║██║   ██║██╔══╝
██║  ██╗███████╗██║╚██████╔╝╚██████╔╝███████╗
╚═╝  ╚═╝╚══════╝╚═╝ ╚══▀▀═╝  ╚═════╝ ╚══════╝`

var FallBackText = lipgloss.JoinVertical(lipgloss.Center,
	GradientText(fallbackBannerRaw, GradientStart, GradientEnd))
