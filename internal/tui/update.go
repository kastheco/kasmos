package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/table"
	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/user/kasmos/internal/worker"
)

var (
	sessionTextPattern = regexp.MustCompile(`session:\s+(ses_[a-zA-Z0-9]+)`)
	sessionJSONPattern = regexp.MustCompile(`"session_id"\s*:\s*"(ses_[a-zA-Z0-9]+)"`)
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.showSpawnDialog {
		return m.updateSpawnDialog(msg)
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		prev := m.layoutMode
		m.recalculateLayout()
		m.refreshTableRows()
		m.refreshViewportFromSelected(false)
		if prev != m.layoutMode {
			cmds = append(cmds, func() tea.Msg {
				return layoutChangedMsg{From: prev, To: m.layoutMode}
			})
		}

		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.ForceQuit, m.keys.Quit) {
			return m, tea.Quit
		}

		if key.Matches(msg, m.keys.Help) {
			m.showHelp = !m.showHelp
			return m, nil
		}

		if m.showHelp {
			if key.Matches(msg, m.keys.Back) {
				m.showHelp = false
			}
			return m, nil
		}

		if m.layoutMode == layoutTooSmall {
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.NextPanel):
			m.cyclePanel(1)
			m.updateKeyStates()
			return m, func() tea.Msg { return focusChangedMsg{To: m.focused} }
		case key.Matches(msg, m.keys.PrevPanel):
			m.cyclePanel(-1)
			m.updateKeyStates()
			return m, func() tea.Msg { return focusChangedMsg{To: m.focused} }
		}

		if m.focused == panelTable {
			if key.Matches(msg, m.keys.Spawn) {
				return m, m.openSpawnDialog()
			}

			if key.Matches(msg, m.keys.Up, m.keys.Down) {
				var cmd tea.Cmd
				m.table, cmd = m.table.Update(msg)
				m.syncSelectionFromTable()
				m.refreshViewportFromSelected(false)
				return m, cmd
			}
		}

		var cmd tea.Cmd
		switch m.focused {
		case panelTable:
			m.table, cmd = m.table.Update(msg)
			m.syncSelectionFromTable()
			m.refreshViewportFromSelected(false)
		case panelViewport:
			m.viewport, cmd = m.viewport.Update(msg)
		}
		return m, cmd

	case spawnDialogSubmittedMsg:
		role := strings.TrimSpace(msg.Role)
		prompt := strings.TrimSpace(msg.Prompt)
		if role == "" {
			role = "coder"
		}

		id := m.manager.NextWorkerID()
		w := &worker.Worker{
			ID:        id,
			Role:      role,
			Prompt:    prompt,
			Files:     msg.Files,
			TaskID:    msg.TaskID,
			State:     worker.StateSpawning,
			SpawnedAt: time.Now(),
			Output:    worker.NewOutputBuffer(worker.DefaultMaxLines),
		}
		m.manager.Add(w)
		m.workers = m.manager.All()
		if m.selectedWorkerID == "" {
			m.selectedWorkerID = w.ID
		}
		m.refreshTableRows()
		m.refreshViewportFromSelected(true)

		cfg := worker.SpawnConfig{ID: w.ID, Role: w.Role, Prompt: w.Prompt, Files: w.Files}
		return m, spawnWorkerCmd(m.backend, cfg)

	case spawnDialogCancelledMsg:
		m.closeSpawnDialog()
		return m, nil

	case workerSpawnedMsg:
		w := m.manager.Get(msg.WorkerID)
		if w == nil {
			return m, nil
		}
		if err := w.Transition(worker.StateRunning); err != nil {
			w.State = worker.StateRunning
		}
		w.Handle = msg.Handle
		if w.SpawnedAt.IsZero() {
			w.SpawnedAt = time.Now()
		}
		m.workers = m.manager.All()
		m.refreshTableRows()

		readWorkerOutput(w.ID, w.Handle.Stdout(), m.program)
		return m, waitWorkerCmd(w.ID, w.Handle)

	case workerOutputMsg:
		w := m.manager.Get(msg.WorkerID)
		if w == nil {
			return m, nil
		}
		if w.Output == nil {
			w.Output = worker.NewOutputBuffer(worker.DefaultMaxLines)
		}
		w.Output.Append(msg.Data)
		if w.ID == m.selectedWorkerID {
			m.refreshViewportFromSelected(true)
		}
		return m, nil

	case workerExitedMsg:
		w := m.manager.Get(msg.WorkerID)
		if w == nil {
			return m, nil
		}

		w.ExitCode = msg.ExitCode
		if msg.Duration > 0 {
			w.ExitedAt = w.SpawnedAt.Add(msg.Duration)
		} else {
			w.ExitedAt = time.Now()
		}
		if msg.Err != nil || msg.ExitCode != 0 {
			w.State = worker.StateFailed
		} else {
			w.State = worker.StateExited
		}
		if msg.SessionID != "" {
			w.SessionID = msg.SessionID
		} else if w.Output != nil {
			w.SessionID = extractSessionID(w.Output.Content())
		}
		w.Handle = nil

		m.workers = m.manager.All()
		m.refreshTableRows()
		if w.ID == m.selectedWorkerID {
			m.refreshViewportFromSelected(true)
		}
		return m, nil

	case workerKilledMsg:
		if w := m.manager.Get(msg.WorkerID); w != nil {
			w.State = worker.StateKilled
			w.ExitedAt = time.Now()
			w.Handle = nil
			m.refreshTableRows()
			if w.ID == m.selectedWorkerID {
				m.refreshViewportFromSelected(true)
			}
		}
		return m, nil

	case tickMsg:
		m.refreshTableRows()
		return m, tickCmd()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		m.refreshTableRows()
		return m, cmd
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) refreshTableRows() {
	m.workers = m.manager.All()
	rows := make([]table.Row, 0, len(m.workers))
	withTask := len(m.workerTableColumns()) == 5
	for _, w := range m.workers {
		status := statusIndicator(w.State, w.ExitCode)
		if w.State == worker.StateRunning {
			status = m.spinner.View() + " running"
		}

		row := table.Row{w.ID, status, roleBadge(w.Role), w.FormatDuration()}
		if withTask {
			task := w.TaskID
			if task == "" {
				task = "-"
			}
			row = append(row, task)
		}
		rows = append(rows, row)
	}

	m.table.SetRows(rows)
	m.syncSelectionFromTable()
}

func (m *Model) syncSelectionFromTable() {
	rows := m.table.Rows()
	if len(rows) == 0 {
		m.selectedWorkerID = ""
		return
	}

	cursor := m.table.Cursor()
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(rows) {
		cursor = len(rows) - 1
		m.table.SetCursor(cursor)
	}
	if len(rows[cursor]) == 0 {
		m.selectedWorkerID = ""
		return
	}

	m.selectedWorkerID = fmt.Sprintf("%v", rows[cursor][0])
}

func (m *Model) refreshViewportFromSelected(autoFollow bool) {
	w := m.selectedWorker()
	if w == nil || w.Output == nil {
		m.setViewportContent(welcomeViewportText(), false)
		return
	}
	m.setViewportContent(w.Output.Content(), autoFollow)
}

func (m *Model) setViewportContent(content string, autoFollow bool) {
	atBottom := m.viewport.AtBottom()
	m.viewport.SetContent(content)
	if autoFollow && atBottom {
		m.viewport.GotoBottom()
	}
}

func (m *Model) selectedWorker() *worker.Worker {
	if m.selectedWorkerID == "" {
		return nil
	}
	return m.manager.Get(m.selectedWorkerID)
}

func extractSessionID(output string) string {
	if match := sessionTextPattern.FindStringSubmatch(output); len(match) > 1 {
		return match[1]
	}
	if match := sessionJSONPattern.FindStringSubmatch(output); len(match) > 1 {
		return match[1]
	}
	return ""
}
