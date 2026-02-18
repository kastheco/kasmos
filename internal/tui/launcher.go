package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/gamut"
)

type launcherMenuItem struct {
	key         string
	label       string
	description string
}

var launcherAsciiArt = []string{
	" _                                ",
	"| | ____ _ ___ _ __ ___   ___  ___",
	"| |/ / _` / __| '_ ` _ \\ / _ \\/ __|",
	"|   < (_| \\__ \\ | | | | | (_) \\__ \\",
	"|_|\\_\\__,_|___/_| |_| |_|\\___/|___/",
}

var launcherMenuItems = []launcherMenuItem{
	{key: "n", label: "new task", description: "spawn a worker in yolo mode"},
	{key: "f", label: "create feature spec", description: "start spec-kitty feature creation"},
	{key: "p", label: "create plan", description: "start spec-kitty plan flow"},
	{key: "h", label: "view history", description: "browse past sessions"},
	{key: "r", label: "restore session", description: "load a previous session"},
	{key: "s", label: "settings", description: "configure agent models"},
	{key: "q", label: "quit", description: "exit kasmos"},
}

func (m *Model) renderLauncher(width, height int) string {
	logo := renderLauncherBranding()
	version := renderLauncherVersion(m.version)
	menu := renderLauncherMenu()
	tip := lipgloss.NewStyle().Foreground(colorMidGray).Render("press a key to get started")

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		logo,
		version,
		"",
		menu,
		"",
		tip,
	)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func renderLauncherBranding() string {
	start, startOK := colorful.MakeColor(colorHotPink)
	end, endOK := colorful.MakeColor(colorPurple)
	if !startOK || !endOK {
		return lipgloss.NewStyle().Foreground(colorHotPink).Bold(true).Render(strings.Join(launcherAsciiArt, "\n"))
	}

	colors := gamut.Blends(start, end, len(launcherAsciiArt))
	lines := make([]string, 0, len(launcherAsciiArt))
	for i, line := range launcherAsciiArt {
		lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(gamut.ToHex(colors[i]))).Bold(true)
		lines = append(lines, lineStyle.Render(line))
	}

	return strings.Join(lines, "\n")
}

func renderLauncherVersion(version string) string {
	v := strings.TrimSpace(version)
	if v == "" {
		v = "dev"
	}
	if v[0] != 'v' {
		v = "v" + v
	}

	return lipgloss.NewStyle().Foreground(colorLightGray).Faint(true).Render(v)
}

func renderLauncherMenu() string {
	badgeStyle := lipgloss.NewStyle().Foreground(colorWhite).Background(colorDarkGray).Bold(true).Padding(0, 1)
	labelStyle := lipgloss.NewStyle().Foreground(colorCream)
	descStyle := lipgloss.NewStyle().Foreground(colorMidGray)

	lines := make([]string, 0, len(launcherMenuItems)*2)
	for _, item := range launcherMenuItems {
		row := lipgloss.JoinHorizontal(lipgloss.Left, badgeStyle.Render(item.key), " ", labelStyle.Render(item.label))
		desc := descStyle.Render("  " + item.description)
		lines = append(lines, row, desc)
	}

	return strings.Join(lines, "\n")
}
