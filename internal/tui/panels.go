package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"

	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/worker"
)

func (m *Model) renderHeader() string {
	title := " " + renderGradientTitle("kasmos") + "  " + dimSubtitleStyle.Render("agent orchestrator")
	v := m.version
	if v != "" && v[0] != 'v' {
		v = "v" + v
	}
	version := versionStyle.Render(v)
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
		empty := lipgloss.NewStyle().Foreground(colorMidGray).Render("no workers yet") +
			"\n\n" +
			lipgloss.NewStyle().Foreground(colorLightGray).Render("press n to create your first task")
		body = lipgloss.Place(m.tableInnerWidth, max(1, m.tableInnerHeight-1), lipgloss.Center, lipgloss.Center, empty)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("workers"),
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

	title := "output"
	if m.analysisMode && m.analysisResult != nil {
		title = fmt.Sprintf("analysis: %s", m.analysisResult.WorkerID)
	}
	if selected := m.selectedWorker(); selected != nil {
		if !m.analysisMode {
			title = fmt.Sprintf("output: %s %s", selected.ID, selected.Role)
			if selected.ParentID != "" {
				title = fmt.Sprintf("%s <- %s", title, selected.ParentID)
			}
		}
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render(title),
		lipgloss.NewStyle().
			MaxWidth(m.viewportInnerWidth).
			MaxHeight(max(1, m.viewportInnerHeight-1)).
			Render(m.viewport.View()),
	)

	return panelStyle(m.focused == panelViewport).
		Width(m.viewportInnerWidth).
		Height(m.viewportInnerHeight).
		Render(content)
}

func (m *Model) renderAnalysisView() string {
	if m.analysisResult == nil {
		return ""
	}

	r := m.analysisResult
	dividerWidth := max(1, min(40, m.viewportInnerWidth))
	lines := []string{
		analysisHeaderStyle.Render(fmt.Sprintf("analysis: %s", r.WorkerID)),
		strings.Repeat("-", dividerWidth),
		"",
		rootCauseLabelStyle.Render("root cause:"),
		r.RootCause,
	}

	if strings.TrimSpace(r.SuggestedPrompt) != "" {
		lines = append(lines,
			"",
			suggestedFixLabelStyle.Render("suggested fix:"),
			r.SuggestedPrompt,
			"",
			analysisHintStyle.Render("press r to restart with suggested prompt"),
		)
	}

	return strings.Join(lines, "\n")
}

func (m *Model) renderFullScreen() string {
	contentHeight := max(0, m.height-m.chromeHeight())
	const (
		borderH = 4
		borderV = 2
	)

	vpInnerWidth := max(1, m.width-borderH)
	vpInnerHeight := max(1, contentHeight-borderV)

	title := "output"
	if m.analysisMode && m.analysisResult != nil {
		title = fmt.Sprintf("analysis: %s", m.analysisResult.WorkerID)
	}
	if selected := m.selectedWorker(); selected != nil {
		if !m.analysisMode {
			title = fmt.Sprintf("output: %s %s - %s", selected.ID, selected.Role, truncateMiddle(strings.TrimSpace(selected.Prompt), 40))
		}
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
			lipgloss.NewStyle().
				MaxWidth(vpInnerWidth).
				MaxHeight(max(1, vpInnerHeight-1)).
				Render(m.viewport.View()),
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
	if m.hasTaskSource() && len(m.loadedTasks) > 0 {
		taskCounts := m.taskCounts()
		taskInfo := fmt.Sprintf("tasks: %d done . %d active . %d pending", taskCounts.done, taskCounts.active, taskCounts.pending)
		left = " " + taskInfo + "  |" + left
	}

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

	title := lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("tasks")

	if len(m.loadedTasks) == 0 {
		empty := lipgloss.NewStyle().Foreground(colorMidGray).Render("no tasks loaded")
		content := lipgloss.JoinVertical(lipgloss.Left, title, empty)
		return panelStyle(m.focused == panelTasks).
			Width(m.tasksInnerWidth).
			Height(m.tasksInnerHeight).
			Render(content)
	}

	selected := m.selectedTaskIdx
	if selected < 0 {
		selected = 0
	}
	if selected >= len(m.loadedTasks) {
		selected = len(m.loadedTasks) - 1
	}

	availableLines := max(1, m.tasksInnerHeight-1)
	start, end := m.visibleTaskWindow(selected, availableLines)

	items := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		isSelected := m.focused == panelTasks && i == selected
		items = append(items, m.renderTaskItem(m.loadedTasks[i], isSelected))
	}

	taskList := strings.Join(items, "\n")
	content := lipgloss.JoinVertical(lipgloss.Left, title, taskList)

	return panelStyle(m.focused == panelTasks).
		Width(m.tasksInnerWidth).
		Height(m.tasksInnerHeight).
		Render(content)
}

func (m *Model) renderTaskItem(t task.Task, selected bool) string {
	idStyle := lipgloss.NewStyle().Bold(true)
	if selected {
		idStyle = idStyle.Foreground(colorPurple)
	}
	line1 := fmt.Sprintf("%s %s  %s", taskStatusIndicator(t.State), idStyle.Render(t.ID), t.Title)

	line2 := m.taskMetaLine(t)
	if line2 == "" {
		return line1
	}

	return lipgloss.JoinVertical(lipgloss.Left, line1, line2)
}

func (m *Model) visibleTaskWindow(selected, availableLines int) (int, int) {
	if len(m.loadedTasks) == 0 {
		return 0, 0
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= len(m.loadedTasks) {
		selected = len(m.loadedTasks) - 1
	}
	if availableLines <= 0 {
		return selected, min(len(m.loadedTasks), selected+1)
	}

	start := selected
	end := selected + 1
	used := m.taskLineCount(m.loadedTasks[selected])

	for end < len(m.loadedTasks) {
		need := m.taskLineCount(m.loadedTasks[end])
		if used+need > availableLines {
			break
		}
		used += need
		end++
	}

	for start > 0 {
		need := m.taskLineCount(m.loadedTasks[start-1])
		if used+need > availableLines {
			break
		}
		used += need
		start--
	}

	for end < len(m.loadedTasks) {
		need := m.taskLineCount(m.loadedTasks[end])
		if used+need > availableLines {
			break
		}
		used += need
		end++
	}

	return start, end
}

func (m *Model) taskLineCount(t task.Task) int {
	if m.taskMetaLine(t) == "" {
		return 1
	}
	return 2
}

func (m *Model) taskMetaLine(t task.Task) string {
	if t.State == task.TaskDone {
		return ""
	}
	if t.WorkerID != "" {
		return lipgloss.NewStyle().Foreground(colorLightBlue).Render("-> " + t.WorkerID)
	}
	if strings.TrimSpace(t.SuggestedRole) != "" {
		return lipgloss.NewStyle().Foreground(colorMidGray).Render("role: " + t.SuggestedRole)
	}
	return ""
}

func taskStatusIndicator(state task.TaskState) string {
	s := lipgloss.NewStyle()
	switch state {
	case task.TaskDone:
		return s.Foreground(colorDone).Render("✓")
	case task.TaskForReview:
		return s.Foreground(colorLightBlue).Render("◆")
	case task.TaskInProgress:
		return s.Foreground(colorRunning).Render("◌")
	case task.TaskBlocked:
		return s.Foreground(colorOrange).Render("⊘")
	case task.TaskFailed:
		return s.Foreground(colorFailed).Render("✗")
	default:
		return s.Foreground(colorPending).Render("○")
	}
}

func firstBlockingDep(t task.Task) string {
	if len(t.Dependencies) > 0 {
		return t.Dependencies[0]
	}
	return ""
}

func (m *Model) renderHelpOverlay() string {
	h := m.help
	h.ShowAll = true
	h.Width = min(74, max(30, m.width-10))

	overlay := lipgloss.JoinVertical(
		lipgloss.Left,
		dialogHeaderStyle.Render("keyboard shortcuts"),
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

type taskStateCounts struct {
	done    int
	active  int
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

func (m *Model) taskCounts() taskStateCounts {
	counts := taskStateCounts{}
	for _, t := range m.loadedTasks {
		switch t.State {
		case task.TaskDone:
			counts.done++
		case task.TaskInProgress:
			counts.active++
		default:
			counts.pending++
		}
	}
	return counts
}
