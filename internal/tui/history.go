package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	historypkg "github.com/user/kasmos/internal/history"
	"github.com/user/kasmos/internal/persist"
	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/worker"
)

type HistoryEntry = historypkg.Entry

func historyScanCmd(projectRoot, specsRoot, kasmosDir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := historypkg.Scan(projectRoot, specsRoot, kasmosDir)
		return historyScanCompleteMsg{Entries: entries, Err: err}
	}
}

func (m *Model) openHistoryOverlay() tea.Cmd {
	m.showHistory = true
	m.historyDetail = false
	m.historyLoading = true
	m.historyErr = nil
	m.historySelected = 0
	m.historyEntries = nil
	m.updateKeyStates()
	return historyScanCmd(".", "kitty-specs", ".kasmos")
}

func (m *Model) closeHistoryOverlay() {
	m.showHistory = false
	m.historyDetail = false
	m.historyLoading = false
	m.historyErr = nil
	m.updateKeyStates()
}

func (m *Model) updateHistory(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case historyScanCompleteMsg:
		m.historyLoading = false
		m.historyErr = msg.Err
		m.historyEntries = m.filterHistoryEntries(msg.Entries)
		if m.historySelected >= len(m.historyEntries) {
			m.historySelected = max(0, len(m.historyEntries)-1)
		}
		return m, nil

	case historyLoadMsg:
		if err := m.loadHistoryEntry(msg.Entry); err != nil {
			m.setViewportContent(fmt.Sprintf("failed to load history entry %q: %v", msg.Entry.Name, err), false)
			return m, nil
		}
		m.closeHistoryOverlay()
		m.refreshTableRows()
		m.refreshViewportFromSelected(false)
		m.updateKeyStates()
		return m, nil

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Back) {
			if m.historyDetail {
				m.historyDetail = false
				return m, nil
			}
			m.closeHistoryOverlay()
			return m, nil
		}

		if len(m.historyEntries) == 0 {
			return m, nil
		}

		if m.historyDetail {
			if key.Matches(msg, m.keys.Select) {
				entry := m.historyEntries[m.historySelected]
				return m, func() tea.Msg { return historyLoadMsg{Entry: entry} }
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Up):
			m.historySelected = max(0, m.historySelected-1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.historySelected = min(len(m.historyEntries)-1, m.historySelected+1)
			return m, nil
		case msg.String() == "d":
			m.historyDetail = true
			return m, nil
		case key.Matches(msg, m.keys.Select):
			entry := m.historyEntries[m.historySelected]
			return m, func() tea.Msg { return historyLoadMsg{Entry: entry} }
		}
	}

	return m, nil
}

func (m *Model) filterHistoryEntries(entries []historypkg.Entry) []HistoryEntry {
	filtered := make([]HistoryEntry, 0, len(entries))
	activeSource := absPathOrRaw(m.taskSourcePath)
	for _, entry := range entries {
		if entry.Type != historypkg.EntryYolo {
			if activeSource != "" && absPathOrRaw(entry.Path) == activeSource {
				continue
			}
		} else if strings.TrimSpace(entry.Name) == strings.TrimSpace(m.sessionID) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func absPathOrRaw(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return trimmed
	}
	return abs
}

func (m *Model) renderHistoryOverlay() string {
	lines := []string{dialogHeaderStyle.Render("history"), ""}
	if m.historyLoading {
		lines = append(lines, fmt.Sprintf("%s scanning project history...", m.spinner.View()))
		lines = append(lines, "", lipgloss.NewStyle().Foreground(colorMidGray).Render("esc close"))
		dialog := dialogStyle.Width(min(120, max(60, m.width-6))).Render(strings.Join(lines, "\n"))
		return m.renderWithBackdrop(dialog)
	}

	if m.historyErr != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorOrange).Render(m.historyErr.Error()))
	}

	if len(m.historyEntries) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorMidGray).Render("no history entries found."))
		lines = append(lines, "", lipgloss.NewStyle().Foreground(colorMidGray).Render("esc close"))
		dialog := dialogStyle.Width(min(120, max(60, m.width-6))).Render(strings.Join(lines, "\n"))
		return m.renderWithBackdrop(dialog)
	}

	if m.historyDetail {
		return m.renderHistoryDetail(lines)
	}

	lines = append(lines, "  type        name                          date        status         progress")
	for i, entry := range m.historyEntries {
		selector := " "
		if i == m.historySelected {
			selector = ">"
		}

		row := fmt.Sprintf("%s %-11s %-28s %-11s %-14s %s",
			selector,
			historyTypeBadge(string(entry.Type)),
			truncateMiddle(entry.Name, 28),
			formatHistoryDate(entry.Date),
			historyStatusBadge(entry.Status),
			historyProgress(entry),
		)
		if i == m.historySelected {
			row = lipgloss.NewStyle().Foreground(colorCream).Bold(true).Render(row)
		}
		lines = append(lines, row)
	}

	lines = append(lines, "", lipgloss.NewStyle().Foreground(colorMidGray).Render("j/k select  enter load as source  d detail  esc close"))
	dialog := dialogStyle.Width(min(130, max(72, m.width-4))).Render(strings.Join(lines, "\n"))
	return m.renderWithBackdrop(dialog)
}

func (m *Model) renderHistoryDetail(lines []string) string {
	entry := m.historyEntries[m.historySelected]
	lines = append(lines,
		lipgloss.NewStyle().Bold(true).Render(entry.Name),
		strings.Repeat("-", min(40, max(20, len(entry.Name)))),
		fmt.Sprintf("type:   %s", entry.Type),
		fmt.Sprintf("path:   %s", entry.Path),
		fmt.Sprintf("status: %s (%s)", entry.Status, historyProgress(entry)),
		"",
	)

	if len(entry.Details) > 0 {
		heading := "details:"
		switch entry.Type {
		case historypkg.EntrySpecKitty:
			heading = "work packages:"
		case historypkg.EntryGSD:
			heading = "tasks:"
		case historypkg.EntryYolo:
			heading = "workers:"
		}
		lines = append(lines, heading)
		maxRows := max(3, m.height-14)
		for i, detail := range entry.Details {
			if i >= maxRows {
				lines = append(lines, "  ...")
				break
			}
			lines = append(lines, "  "+detail)
		}
	}

	lines = append(lines, "", lipgloss.NewStyle().Foreground(colorMidGray).Render("enter load as source  esc back"))
	dialog := dialogStyle.Width(min(130, max(72, m.width-4))).Render(strings.Join(lines, "\n"))
	return m.renderWithBackdrop(dialog)
}

func formatHistoryDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("Jan 02")
}

func historyProgress(entry HistoryEntry) string {
	switch entry.Type {
	case historypkg.EntrySpecKitty:
		return fmt.Sprintf("%d/%d wps", entry.DoneCount, entry.TaskCount)
	case historypkg.EntryGSD:
		return fmt.Sprintf("%d/%d tasks", entry.DoneCount, entry.TaskCount)
	case historypkg.EntryYolo:
		return fmt.Sprintf("%d workers", entry.WorkerCount)
	default:
		return entry.Summary
	}
}

func (m *Model) loadHistoryEntry(entry historypkg.Entry) error {
	switch entry.Type {
	case historypkg.EntrySpecKitty:
		m.swapTaskSource(&task.SpecKittySource{Dir: entry.Path})
		return nil
	case historypkg.EntryGSD:
		m.swapTaskSource(&task.GsdSource{FilePath: entry.Path})
		return nil
	case historypkg.EntryYolo:
		return m.loadArchivedSession(entry.Path)
	default:
		return fmt.Errorf("unsupported history type %q", entry.Type)
	}
}

func (m *Model) loadArchivedSession(path string) error {
	if m.persister == nil {
		return fmt.Errorf("session persister is not configured")
	}

	state, err := m.persister.LoadFromPath(path)
	if err != nil {
		return err
	}

	m.manager = worker.NewWorkerManager()
	m.workers = nil
	m.selectedWorkerID = ""

	for _, snap := range state.Workers {
		w := persist.SnapshotToWorker(snap)
		if w.State == worker.StateRunning || w.State == worker.StateSpawning {
			w.State = worker.StateKilled
			w.ExitedAt = time.Now()
		}
		m.manager.Add(w)
		if m.selectedWorkerID == "" {
			m.selectedWorkerID = w.ID
		}
	}

	m.workers = m.manager.All()
	m.manager.ResetWorkerCounter(state.NextWorkerNum)
	m.sessionID = state.SessionID
	if !state.StartedAt.IsZero() {
		m.sessionStartedAt = state.StartedAt
	}

	if state.TaskSource == nil {
		m.swapTaskSource(&task.YoloSource{})
		return nil
	}

	source, err := sourceFromPersistConfig(*state.TaskSource)
	if err != nil {
		return err
	}
	m.swapTaskSource(source)
	return nil
}

func sourceFromPersistConfig(cfg persist.TaskSourceConfig) (task.Source, error) {
	switch strings.TrimSpace(cfg.Type) {
	case "", "yolo", "ad-hoc":
		return &task.YoloSource{}, nil
	case "spec-kitty":
		return &task.SpecKittySource{Dir: cfg.Path}, nil
	case "gsd":
		return &task.GsdSource{FilePath: cfg.Path}, nil
	default:
		return nil, fmt.Errorf("unsupported task source type %q", cfg.Type)
	}
}
