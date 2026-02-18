package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/table"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/user/kasmos/internal/worker"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.showContinueDialog {
		return m.updateContinueDialog(msg)
	}

	if m.showQuitConfirm {
		return m.updateQuitConfirm(msg)
	}

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
		if m.fullScreen {
			m.resizeFullScreenViewport()
		} else {
			m.recalculateLayout()
		}
		m.refreshTableRows()
		m.refreshViewportFromSelected(false)
		if prev != m.layoutMode {
			cmds = append(cmds, func() tea.Msg {
				return layoutChangedMsg{From: prev, To: m.layoutMode}
			})
		}

		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		// Phase 1: Global keys
		if key.Matches(msg, m.keys.ForceQuit) {
			return m, tea.Quit
		}

		if key.Matches(msg, m.keys.Quit) {
			running := m.runningWorkersCount()
			if running == 0 {
				return m, tea.Quit
			}
			m.showQuitConfirm = true
			m.quitConfirmFocused = 1
			m.updateKeyStates()
			return m, nil
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

		if key.Matches(msg, m.keys.NextPanel) {
			m.cyclePanel(1)
			m.updateKeyStates()
			return m, func() tea.Msg { return focusChangedMsg{To: m.focused} }
		}

		if key.Matches(msg, m.keys.PrevPanel) {
			m.cyclePanel(-1)
			m.updateKeyStates()
			return m, func() tea.Msg { return focusChangedMsg{To: m.focused} }
		}

		// Phase 2: Fullscreen keys
		if m.fullScreen {
			return m.updateFullScreenKeys(msg)
		}

		// Phase 3: Panel-specific keys
		switch m.focused {
		case panelTable:
			return m.updateTableKeys(msg)
		case panelViewport:
			return m.updateViewportKeys(msg)
		default:
			return m, nil
		}

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

	case continueDialogSubmittedMsg:
		parent := m.manager.Get(msg.ParentWorkerID)
		if parent == nil {
			return m, nil
		}
		id := m.manager.NextWorkerID()
		w := &worker.Worker{
			ID:        id,
			Role:      parent.Role,
			Prompt:    msg.FollowUp,
			ParentID:  msg.ParentWorkerID,
			State:     worker.StateSpawning,
			SpawnedAt: time.Now(),
			Output:    worker.NewOutputBuffer(worker.DefaultMaxLines),
		}
		m.manager.Add(w)
		m.workers = m.manager.All()
		m.selectedWorkerID = w.ID
		m.refreshTableRows()
		m.refreshViewportFromSelected(true)

		cfg := worker.SpawnConfig{
			ID:              w.ID,
			Role:            w.Role,
			Prompt:          msg.FollowUp,
			ContinueSession: msg.SessionID,
		}
		return m, spawnWorkerCmd(m.backend, cfg)

	case continueDialogCancelledMsg:
		m.closeContinueDialog()
		return m, nil

	case quitConfirmedMsg:
		for _, w := range m.manager.All() {
			if w.State == worker.StateRunning && w.Handle != nil {
				_ = w.Handle.Kill(3 * time.Second)
			}
		}
		return m, tea.Quit

	case quitCancelledMsg:
		m.showQuitConfirm = false
		m.updateKeyStates()
		return m, nil

	case workerSpawnedMsg:
		w := m.manager.Get(msg.WorkerID)
		if w == nil {
			return m, nil
		}
		// Force to running — the transition may fail if the worker was already
		// in an unexpected state (e.g., killed during spawn), but we trust the
		// backend's spawned confirmation.
		w.State = worker.StateRunning
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
			w.SessionID = worker.ExtractSessionID(w.Output.Content())
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
	ordered, prefixes := workerTreeRows(m.workers)
	m.tableRowWorkerIDs = make([]string, 0, len(ordered))
	rows := make([]table.Row, 0, len(ordered))
	withTask := len(m.workerTableColumns()) == 5
	treePrefixStyle := lipgloss.NewStyle().Foreground(colorMidGray).Faint(true)
	for _, w := range ordered {
		status := statusIndicator(w.State, w.ExitCode)
		if w.State == worker.StateRunning {
			status = m.spinner.View() + " running"
		}

		idLabel := w.ID
		if prefix := prefixes[w.ID]; prefix != "" {
			idLabel = treePrefixStyle.Render(prefix) + w.ID
		}

		row := table.Row{idLabel, status, roleBadge(w.Role), w.FormatDuration()}
		if withTask {
			task := w.TaskID
			if task == "" {
				task = "-"
			}
			row = append(row, task)
		}
		rows = append(rows, row)
		m.tableRowWorkerIDs = append(m.tableRowWorkerIDs, w.ID)
	}

	m.table.SetRows(rows)
	if m.selectedWorkerID != "" {
		for i, id := range m.tableRowWorkerIDs {
			if id == m.selectedWorkerID {
				m.table.SetCursor(i)
				break
			}
		}
	}
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
	if cursor < 0 || cursor >= len(m.tableRowWorkerIDs) {
		m.selectedWorkerID = ""
		m.updateKeyStates()
		return
	}

	m.selectedWorkerID = m.tableRowWorkerIDs[cursor]
	m.updateKeyStates()
}

func (m *Model) refreshViewportFromSelected(autoFollow bool) {
	w := m.selectedWorker()
	if w == nil || w.Output == nil {
		m.setViewportContent(welcomeViewportText(), false)
		return
	}
	content := w.Output.Content()
	if w.ParentID != "" {
		parentRole := "unknown"
		if parent := m.manager.Get(w.ParentID); parent != nil {
			parentRole = parent.Role
		}
		line := lipgloss.NewStyle().Foreground(colorMidGray).Faint(true).
			Render(fmt.Sprintf("← continued from %s (%s)", w.ParentID, parentRole))
		content = line + "\n" + content
	}
	m.setViewportContent(content, autoFollow)
}

func (m *Model) setViewportContent(content string, autoFollow bool) {
	wasAtBottom := m.viewport.AtBottom()
	m.viewport.SetContent(content)
	if autoFollow && (wasAtBottom || m.autoFollow) {
		m.viewport.GotoBottom()
		m.autoFollow = true
	}
}

func (m *Model) updateFullScreenKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Back) {
		m.fullScreen = false
		m.recalculateLayout()
		m.updateKeyStates()
		return m, nil
	}

	if key.Matches(msg, m.keys.Continue) {
		selected := m.selectedWorker()
		if selected != nil &&
			(selected.State == worker.StateExited || selected.State == worker.StateFailed) &&
			selected.SessionID != "" {
			return m, m.openContinueDialog(selected)
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Restart) {
		selected := m.selectedWorker()
		if selected != nil && (selected.State == worker.StateFailed || selected.State == worker.StateKilled) {
			return m, m.openSpawnDialogWithPrefill(selected.Role, selected.Prompt, selected.Files)
		}
		return m, nil
	}

	return m.updateViewportScrollKeys(msg)
}

func (m *Model) updateTableKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Spawn) {
		return m, m.openSpawnDialog()
	}

	if key.Matches(msg, m.keys.Continue) {
		selected := m.selectedWorker()
		if selected != nil &&
			(selected.State == worker.StateExited || selected.State == worker.StateFailed) &&
			selected.SessionID != "" {
			return m, m.openContinueDialog(selected)
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Kill) {
		selected := m.selectedWorker()
		if selected != nil && selected.State == worker.StateRunning && selected.Handle != nil {
			return m, killWorkerCmd(selected.ID, selected.Handle, 3*time.Second)
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Restart) {
		selected := m.selectedWorker()
		if selected != nil && (selected.State == worker.StateFailed || selected.State == worker.StateKilled) {
			return m, m.openSpawnDialogWithPrefill(selected.Role, selected.Prompt, selected.Files)
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Fullscreen, m.keys.Select) {
		if m.selectedWorker() != nil {
			m.fullScreen = true
			m.resizeFullScreenViewport()
			m.updateKeyStates()
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Up, m.keys.Down) {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		m.syncSelectionFromTable()
		m.refreshViewportFromSelected(false)
		return m, cmd
	}

	return m, nil
}

func (m *Model) updateViewportKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Fullscreen) {
		if m.selectedWorker() != nil {
			m.fullScreen = true
			m.resizeFullScreenViewport()
			m.updateKeyStates()
		}
		return m, nil
	}

	return m.updateViewportScrollKeys(msg)
}

func (m *Model) updateViewportScrollKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.ScrollDown, m.keys.Down):
		m.viewport.LineDown(1)
		if m.viewport.AtBottom() {
			m.autoFollow = true
		}
		return m, nil
	case key.Matches(msg, m.keys.ScrollUp, m.keys.Up):
		m.viewport.LineUp(1)
		m.autoFollow = false
		return m, nil
	case key.Matches(msg, m.keys.HalfDown):
		m.viewport.HalfViewDown()
		if m.viewport.AtBottom() {
			m.autoFollow = true
		}
		return m, nil
	case key.Matches(msg, m.keys.HalfUp):
		m.viewport.HalfViewUp()
		m.autoFollow = false
		return m, nil
	case key.Matches(msg, m.keys.GotoBottom):
		m.viewport.GotoBottom()
		m.autoFollow = true
		return m, nil
	case key.Matches(msg, m.keys.GotoTop):
		m.viewport.GotoTop()
		m.autoFollow = false
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) resizeFullScreenViewport() {
	contentHeight := max(0, m.height-m.chromeHeight())
	const (
		borderH = 4
		borderV = 2
	)
	m.viewport.SetWidth(max(1, m.width-borderH))
	m.viewport.SetHeight(max(1, contentHeight-borderV-1))
}

func (m *Model) selectedWorker() *worker.Worker {
	if m.selectedWorkerID == "" {
		return nil
	}
	return m.manager.Get(m.selectedWorkerID)
}

func (m *Model) runningWorkersCount() int {
	count := 0
	for _, w := range m.manager.All() {
		if w.State == worker.StateRunning {
			count++
		}
	}
	return count
}
