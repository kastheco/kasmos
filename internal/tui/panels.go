package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"

	"github.com/user/kasmos/internal/worker"
)

const appVersion = "v0.1.0"

func (m *Model) renderHeader() string {
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

func (m *Model) renderWorkerTable() string {
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

func (m *Model) renderViewport() string {
	if m.viewportInnerWidth <= 0 || m.viewportInnerHeight <= 0 {
		return ""
	}

	title := "Output"
	if selected := m.selectedWorker(); selected != nil {
		title = fmt.Sprintf("Output: %s %s", selected.ID, selected.Role)
		if selected.ParentID != "" {
			title = fmt.Sprintf("%s <- %s", title, selected.ParentID)
		}
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render(title),
		m.viewport.View(),
	)

	return panelStyle(m.focused == panelViewport).
		Width(m.viewportInnerWidth).
		Height(m.viewportInnerHeight).
		Render(content)
}

func (m *Model) renderFullScreen() string {
	contentHeight := max(0, m.height-m.chromeHeight())
	const (
		borderH = 4
		borderV = 2
	)

	vpInnerWidth := max(1, m.width-borderH)
	vpInnerHeight := max(1, contentHeight-borderV)
	m.viewport.SetWidth(vpInnerWidth)
	m.viewport.SetHeight(max(1, vpInnerHeight-1))

	title := "Output"
	if selected := m.selectedWorker(); selected != nil {
		title = fmt.Sprintf("Output: %s %s - %s", selected.ID, selected.Role, truncateMiddle(strings.TrimSpace(selected.Prompt), 40))
	}

	viewportPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPurple).
		Padding(0, 1).
		Width(vpInnerWidth).
		Height(vpInnerHeight).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render(title),
			m.viewport.View(),
		))

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderHeader(),
		viewportPanel,
		m.renderFullScreenStatusBar(),
		m.renderHelpBar(),
	)

	return view
}

func (m *Model) renderFullScreenStatusBar() string {
	selected := m.selectedWorker()
	if selected == nil {
		line := " -  -  -  exit(-)  duration: -  session: -  scroll: - "
		return statusBarStyle.Width(m.width).Render(line)
	}

	session := selected.SessionID
	if session == "" {
		session = "-"
	}

	exit := "-"
	if selected.State == worker.StateExited || selected.State == worker.StateFailed {
		exit = fmt.Sprintf("%d", selected.ExitCode)
	}

	scroll := "-"
	if m.viewport.TotalLineCount() > 0 {
		scroll = fmt.Sprintf("%.0f%%", m.viewport.ScrollPercent()*100)
	}

	line := fmt.Sprintf(" %s %s  %s  exit(%s)  duration: %s  session: %s  scroll: %s ",
		selected.ID,
		selected.Role,
		selected.State,
		exit,
		selected.FormatDuration(),
		session,
		scroll,
	)

	return statusBarStyle.Width(m.width).Render(line)
}

func truncateMiddle(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len([]rune(s)) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return strings.Repeat(".", maxLen)
	}
	return string([]rune(s)[:maxLen-3]) + "..."
}

func workerTreeRows(workers []*worker.Worker) ([]*worker.Worker, map[string]string) {
	if len(workers) == 0 {
		return nil, map[string]string{}
	}

	byID := make(map[string]*worker.Worker, len(workers))
	children := make(map[string][]*worker.Worker, len(workers))
	roots := make([]*worker.Worker, 0, len(workers))
	for _, w := range workers {
		byID[w.ID] = w
	}
	for _, w := range workers {
		if w.ParentID == "" || byID[w.ParentID] == nil {
			roots = append(roots, w)
			continue
		}
		children[w.ParentID] = append(children[w.ParentID], w)
	}

	ordered := make([]*worker.Worker, 0, len(workers))
	prefixes := make(map[string]string, len(workers))
	visited := make(map[string]bool, len(workers))

	var walk func(node *worker.Worker, depth int, ancestorHasNext []bool, isLast bool)
	walk = func(node *worker.Worker, depth int, ancestorHasNext []bool, isLast bool) {
		if node == nil || visited[node.ID] {
			return
		}
		visited[node.ID] = true

		if depth > 0 {
			var b strings.Builder
			for i := 0; i < depth-1; i++ {
				if ancestorHasNext[i] {
					b.WriteString("│ ")
				} else {
					b.WriteString("  ")
				}
			}
			if isLast {
				b.WriteString("└─")
			} else {
				b.WriteString("├─")
			}
			prefixes[node.ID] = b.String()
		}

		ordered = append(ordered, node)
		next := children[node.ID]
		for i, child := range next {
			walk(child, depth+1, append(ancestorHasNext, i < len(next)-1), i == len(next)-1)
		}
	}

	for i, root := range roots {
		walk(root, 0, nil, i == len(roots)-1)
	}
	for _, w := range workers {
		if !visited[w.ID] {
			walk(w, 0, nil, true)
		}
	}

	return ordered, prefixes
}

func (m *Model) renderStatusBar() string {
	counts := m.workerCounts()
	left := fmt.Sprintf(" %s %d running  %s %d done  %s %d failed  %s %d killed  %s %d pending",
		m.spinner.View(), counts.running,
		successStyle.Render("✓"), counts.done,
		failStyle.Render("✗"), counts.failed,
		warningStyle.Render("☠"), counts.killed,
		lipgloss.NewStyle().Foreground(colorPending).Render("○"), counts.pending,
	)

	scrollStr := "-"
	if m.focused == panelViewport && m.viewport.TotalLineCount() > 0 {
		scrollStr = fmt.Sprintf("%.0f%%", m.viewport.ScrollPercent()*100)
	}
	right := fmt.Sprintf("mode: %s  scroll: %s ", m.modeName(), scrollStr)
	gap := strings.Repeat(" ", max(0, m.width-lipgloss.Width(left)-lipgloss.Width(right)-2))
	bar := left + gap + right
	return statusBarStyle.Width(m.width).Render(bar)
}

func (m *Model) renderHelpBar() string {
	h := m.help
	h.Width = m.width
	h.ShowAll = false
	return h.View(m.keys)
}

func (m *Model) renderTasksPanel() string {
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

func (m *Model) renderHelpOverlay() string {
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

type workerStateCounts struct {
	running int
	done    int
	failed  int
	killed  int
	pending int
}

func (m *Model) workerCounts() workerStateCounts {
	counts := workerStateCounts{}
	for _, w := range m.workers {
		switch w.State {
		case worker.StateRunning:
			counts.running++
		case worker.StateExited:
			counts.done++
		case worker.StateFailed:
			counts.failed++
		case worker.StateKilled:
			counts.killed++
		case worker.StatePending, worker.StateSpawning:
			counts.pending++
		}
	}
	return counts
}
