package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
)

const appVersion = "v0.1.0"

func (m Model) renderHeader() string {
	title := " " + renderGradientTitle("kasmos") + "  " + dimSubtitleStyle.Render("agent orchestrator")
	version := versionStyle.Render(appVersion)
	gap := strings.Repeat(" ", max(1, m.width-lipgloss.Width(title)-lipgloss.Width(version)))
	line := title + gap + version
	if !m.hasTaskSource() {
		return lipgloss.JoinVertical(lipgloss.Left, line, "")
	}

	subtitle := sourceSubtitleStyle.Render(fmt.Sprintf("%s: %s", m.taskSourceType, m.taskSourcePath))
	return lipgloss.JoinVertical(lipgloss.Left, line, subtitle)
}

func (m Model) renderWorkerTable() string {
	if m.tableInnerWidth <= 0 || m.tableInnerHeight <= 0 {
		return ""
	}

	body := m.table.View()
	if len(m.table.Rows()) == 0 {
		empty := lipgloss.NewStyle().Foreground(colorMidGray).Render("No workers yet") +
			"\n\n" +
			lipgloss.NewStyle().Foreground(colorLightGray).Render("Press s to spawn your first worker")
		body = lipgloss.Place(m.tableInnerWidth, max(1, m.tableInnerHeight-1), lipgloss.Center, lipgloss.Center, empty)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("Workers"),
		body,
	)

	return panelStyle(m.focused == panelTable).
		Width(m.tableInnerWidth).
		Height(m.tableInnerHeight).
		Render(content)
}

func (m Model) renderViewport() string {
	if m.viewportInnerWidth <= 0 || m.viewportInnerHeight <= 0 {
		return ""
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("Output"),
		m.viewport.View(),
	)

	return panelStyle(m.focused == panelViewport).
		Width(m.viewportInnerWidth).
		Height(m.viewportInnerHeight).
		Render(content)
}

func (m Model) renderStatusBar() string {
	left := " 0 workers"
	scrollStr := "-"
	if m.focused == panelViewport && m.viewport.TotalLineCount() > 0 {
		scrollStr = fmt.Sprintf("%.0f%%", m.viewport.ScrollPercent()*100)
	}
	right := fmt.Sprintf("mode: %s  scroll: %s ", m.modeName(), scrollStr)
	gap := strings.Repeat(" ", max(0, m.width-lipgloss.Width(left)-lipgloss.Width(right)-2))
	m.statusBar = left + gap + right
	return statusBarStyle.Width(m.width).Render(m.statusBar)
}

func (m Model) renderHelpBar() string {
	h := m.help
	h.Width = m.width
	h.ShowAll = false
	return h.View(m.keys)
}

func (m Model) renderTasksPanel() string {
	if m.tasksInnerWidth <= 0 || m.tasksInnerHeight <= 0 {
		return ""
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("Tasks"),
		lipgloss.NewStyle().Foreground(colorMidGray).Render("No tasks loaded"),
	)

	return panelStyle(m.focused == panelTasks).
		Width(m.tasksInnerWidth).
		Height(m.tasksInnerHeight).
		Render(content)
}

func (m Model) renderHelpOverlay() string {
	h := m.help
	h.ShowAll = true
	h.Width = min(74, max(30, m.width-10))

	overlay := lipgloss.JoinVertical(
		lipgloss.Left,
		dialogHeaderStyle.Render("Keyboard Shortcuts"),
		"",
		h.View(m.keys),
	)

	dialogWidth := min(78, max(36, m.width-6))
	dialog := dialogStyle.Width(dialogWidth).Render(overlay)

	return m.renderWithBackdrop(dialog)
}
