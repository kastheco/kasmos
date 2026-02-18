package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/user/kasmos/internal/persist"
	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/worker"
)

type restoreSessionEntry struct {
	Path        string
	SessionID   string
	Timestamp   time.Time
	WorkerCount int
	TaskSource  string
	Active      bool
}

func newRestoreSessionEntry(state persist.SessionState, path string, active bool) restoreSessionEntry {
	id := strings.TrimSpace(state.SessionID)
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	sourceType := "yolo"
	if state.TaskSource != nil {
		t := strings.TrimSpace(state.TaskSource.Type)
		if t != "" && t != "ad-hoc" {
			sourceType = t
		}
	}

	timestamp := state.StartedAt
	if timestamp.IsZero() && state.FinishedAt != nil {
		timestamp = *state.FinishedAt
	}

	return restoreSessionEntry{
		Path:        path,
		SessionID:   id,
		Timestamp:   timestamp,
		WorkerCount: len(state.Workers),
		TaskSource:  sourceType,
		Active:      active,
	}
}

func (m *Model) openRestorePicker() tea.Cmd {
	m.showRestorePicker = true
	m.restoreEntries = nil
	m.restoreSelected = 0
	m.restoreLoading = true
	m.restoreErr = nil
	m.restoreNote = ""
	m.updateKeyStates()
	return restoreScanCmd(m.persister)
}

func (m *Model) closeRestorePicker() {
	m.showRestorePicker = false
	m.restoreEntries = nil
	m.restoreSelected = 0
	m.restoreLoading = false
	m.restoreErr = nil
	m.restoreNote = ""
	m.updateKeyStates()
}

func (m *Model) updateRestorePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case restoreScanCompleteMsg:
		m.restoreLoading = false
		m.restoreErr = msg.Err
		m.restoreNote = msg.Note
		m.restoreEntries = msg.Entries
		if m.restoreSelected >= len(m.restoreEntries) {
			m.restoreSelected = max(0, len(m.restoreEntries)-1)
		}
		return m, nil

	case restoreLoadCompleteMsg:
		m.restoreLoading = false
		if msg.Err != nil {
			m.restoreErr = fmt.Errorf("failed to load session: %w", msg.Err)
			return m, nil
		}
		if msg.State == nil {
			m.restoreErr = fmt.Errorf("failed to load session: empty session state")
			return m, nil
		}

		if err := m.applyRestoredSessionState(msg.State); err != nil {
			m.restoreErr = fmt.Errorf("failed to apply session: %w", err)
			return m, nil
		}

		m.transitionFromLauncher()
		m.closeRestorePicker()
		m.refreshTableRows()
		m.refreshViewportFromSelected(false)
		return m, nil

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Back) {
			m.closeRestorePicker()
			return m, nil
		}

		if m.restoreLoading {
			return m, nil
		}

		if len(m.restoreEntries) == 0 {
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Up):
			m.restoreSelected = max(0, m.restoreSelected-1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.restoreSelected = min(len(m.restoreEntries)-1, m.restoreSelected+1)
			return m, nil
		case key.Matches(msg, m.keys.Select) || msg.String() == "enter":
			selected := m.restoreEntries[m.restoreSelected]
			m.restoreLoading = true
			m.restoreErr = nil
			return m, restoreLoadCmd(m.persister, selected.Path)
		}
	}

	return m, nil
}

func (m *Model) renderRestorePicker() string {
	lines := []string{dialogHeaderStyle.Render("restore session"), ""}

	if m.restoreLoading && len(m.restoreEntries) == 0 {
		lines = append(lines, fmt.Sprintf("%s scanning sessions...", m.spinner.View()))
		lines = append(lines, "", lipgloss.NewStyle().Foreground(colorMidGray).Render("esc back"))
		dialog := dialogStyle.Width(min(120, max(60, m.width-6))).Render(strings.Join(lines, "\n"))
		return m.renderWithBackdrop(dialog)
	}

	if m.restoreErr != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorOrange).Render(m.restoreErr.Error()), "")
	}

	if m.restoreNote != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorMidGray).Render(m.restoreNote), "")
	}

	if len(m.restoreEntries) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorMidGray).Render("no restorable sessions found."))
		lines = append(lines, "", lipgloss.NewStyle().Foreground(colorMidGray).Render("esc back"))
		dialog := dialogStyle.Width(min(120, max(60, m.width-6))).Render(strings.Join(lines, "\n"))
		return m.renderWithBackdrop(dialog)
	}

	for i, entry := range m.restoreEntries {
		selector := " "
		if i == m.restoreSelected {
			selector = ">"
		}

		badge := "archived"
		if entry.Active {
			badge = "last active"
		}

		timestamp := "-"
		if !entry.Timestamp.IsZero() {
			timestamp = entry.Timestamp.Local().Format("2006-01-02 15:04")
		}

		row := fmt.Sprintf("%s %-12s %s", selector, badge, entry.SessionID)
		meta := fmt.Sprintf("  %s  workers:%d  source:%s", timestamp, entry.WorkerCount, entry.TaskSource)
		if i == m.restoreSelected {
			style := lipgloss.NewStyle().Foreground(colorCream).Bold(true)
			row = style.Render(row)
			meta = style.Render(meta)
		}
		lines = append(lines, row, meta)
	}

	helpText := "j/k select  enter restore  esc back"
	if m.restoreLoading {
		helpText = "restoring session..."
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(colorMidGray).Render(helpText))

	dialog := dialogStyle.Width(min(130, max(72, m.width-4))).Render(strings.Join(lines, "\n"))
	return m.renderWithBackdrop(dialog)
}

func (m *Model) applyRestoredSessionState(state *persist.SessionState) error {
	if state == nil {
		return fmt.Errorf("session state is nil")
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
