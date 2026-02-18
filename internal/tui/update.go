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

	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/worker"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.showContinueDialog {
		return m.updateContinueDialog(msg)
	}

	if m.showQuitConfirm {
		return m.updateQuitConfirm(msg)
	}

	if m.showHistory {
		return m.updateHistory(msg)
	}

	if m.showBatchDialog {
		return m.updateBatchDialog(msg)
	}

	if m.showNewDialog {
		return m.updateNewDialog(msg)
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
			m.updateKeyStates()
			return m, nil
		}

		if m.showHelp {
			if key.Matches(msg, m.keys.Back) {
				m.showHelp = false
				m.updateKeyStates()
			}
			return m, nil
		}

		if m.layoutMode == layoutTooSmall {
			return m, nil
		}

		if key.Matches(msg, m.keys.New) {
			return m, m.openNewDialog()
		}

		if key.Matches(msg, m.keys.History) {
			return m, m.openHistoryOverlay()
		}

		if !m.fullScreen && key.Matches(msg, m.keys.NextPanel) {
			m.cyclePanel(1)
			m.updateKeyStates()
			return m, func() tea.Msg { return focusChangedMsg{To: m.focused} }
		}

		if !m.fullScreen && key.Matches(msg, m.keys.PrevPanel) {
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
		case panelTasks:
			return m.updateTaskPanelKeys(msg)
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
		if msg.TaskID != "" {
			for i := range m.loadedTasks {
				if m.loadedTasks[i].ID == msg.TaskID {
					m.loadedTasks[i].State = task.TaskInProgress
					m.loadedTasks[i].WorkerID = w.ID
					break
				}
			}
		}
		m.updateKeyStates()
		m.triggerPersist()

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
		m.triggerPersist()

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

	case newDialogCancelledMsg:
		m.closeNewDialog()
		return m, nil

	case specCreatedMsg:
		if msg.Err != nil {
			m.setViewportContent(formatCreateError("feature spec", msg.Err), false)
			return m, nil
		}
		m.swapTaskSource(&task.SpecKittySource{Dir: msg.Path})
		m.refreshTableRows()
		m.setViewportContent(fmt.Sprintf("created feature %q at %s", msg.Slug, msg.Path), false)
		return m, nil

	case gsdCreatedMsg:
		if msg.Err != nil {
			m.setViewportContent(formatCreateError("gsd task list", msg.Err), false)
			return m, nil
		}
		m.swapTaskSource(&task.GsdSource{FilePath: msg.Path})
		m.refreshTableRows()
		m.setViewportContent(fmt.Sprintf("created %s with %d tasks", msg.Path, msg.TaskCount), false)
		return m, nil

	case planCreatedMsg:
		if msg.Err != nil {
			m.setViewportContent(formatCreateError("planning doc", msg.Err), false)
			return m, nil
		}
		m.setViewportContent(fmt.Sprintf("created %s", msg.Path), false)
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
		m.logDaemonEvent(workerSpawnEvent(w.ID, w.Role, w.TaskID))
		m.workers = m.manager.All()
		m.refreshTableRows()
		m.triggerPersist()

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
		if w.Handle == nil && (w.State == worker.StateExited || w.State == worker.StateFailed || w.State == worker.StateKilled) {
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
		if strings.TrimSpace(msg.SessionID) != "" {
			w.SessionID = strings.TrimSpace(msg.SessionID)
		}
		if w.SessionID == "" && w.Output != nil {
			w.SessionID = worker.ExtractSessionID(w.Output.Content())
		}
		w.Handle = nil
		m.logDaemonEvent(workerExitEvent(w.ID, w.ExitCode, w.FormatDuration(), w.SessionID))

		if w.TaskID != "" {
			for i := range m.loadedTasks {
				if m.loadedTasks[i].ID != w.TaskID {
					continue
				}
				if w.State == worker.StateExited {
					m.loadedTasks[i].State = task.TaskForReview
					m.loadedTasks[i].WorkerID = w.ID
				} else {
					m.loadedTasks[i].State = task.TaskFailed
				}
				m.resolveTaskDependencies()
				break
			}
		}

		m.workers = m.manager.All()
		m.refreshTableRows()
		m.updateKeyStates()
		if w.ID == m.selectedWorkerID {
			m.refreshViewportFromSelected(true)
		}
		m.triggerPersist()
		if m.daemon {
			if cmd := m.checkDaemonComplete(); cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case workerMarkedDoneMsg:
		w := m.manager.Get(msg.WorkerID)
		if w == nil {
			return m, nil
		}
		if msg.Err != nil {
			m.setViewportContent(fmt.Sprintf("failed to mark %s done: %v", w.ID, msg.Err), false)
			return m, nil
		}

		w.State = worker.StateExited
		w.ExitCode = 0
		w.ExitedAt = time.Now()
		if strings.TrimSpace(msg.SessionID) != "" {
			w.SessionID = strings.TrimSpace(msg.SessionID)
		}
		if w.SessionID == "" && w.Output != nil {
			w.SessionID = worker.ExtractSessionID(w.Output.Content())
		}
		w.Handle = nil

		if w.TaskID != "" {
			for i := range m.loadedTasks {
				if m.loadedTasks[i].ID != w.TaskID {
					continue
				}
				m.loadedTasks[i].State = task.TaskDone
				m.loadedTasks[i].WorkerID = w.ID
				m.resolveTaskDependencies()
				break
			}
		}

		m.logDaemonEvent(workerExitEvent(w.ID, w.ExitCode, w.FormatDuration(), w.SessionID))
		m.workers = m.manager.All()
		m.refreshTableRows()
		m.updateKeyStates()
		if w.ID == m.selectedWorkerID {
			m.refreshViewportFromSelected(true)
			m.setViewportContent(fmt.Sprintf("marked %s done", w.ID), false)
		}
		m.triggerPersist()
		if m.daemon {
			if cmd := m.checkDaemonComplete(); cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case workerKillAndContinueMsg:
		w := m.manager.Get(msg.WorkerID)
		if w == nil {
			return m, nil
		}
		if msg.Err != nil {
			m.setViewportContent(fmt.Sprintf("failed to continue %s: %v", w.ID, msg.Err), false)
			return m, nil
		}

		w.State = worker.StateExited
		w.ExitCode = 0
		w.ExitedAt = time.Now()
		if strings.TrimSpace(msg.SessionID) != "" {
			w.SessionID = strings.TrimSpace(msg.SessionID)
		}
		if w.SessionID == "" && w.Output != nil {
			w.SessionID = worker.ExtractSessionID(w.Output.Content())
		}
		w.Handle = nil

		if w.TaskID != "" {
			for i := range m.loadedTasks {
				if m.loadedTasks[i].ID != w.TaskID {
					continue
				}
				m.loadedTasks[i].State = task.TaskDone
				m.loadedTasks[i].WorkerID = w.ID
				m.resolveTaskDependencies()
				break
			}
		}

		m.logDaemonEvent(workerExitEvent(w.ID, w.ExitCode, w.FormatDuration(), w.SessionID))
		m.workers = m.manager.All()
		m.refreshTableRows()
		m.updateKeyStates()
		if w.ID == m.selectedWorkerID {
			m.refreshViewportFromSelected(true)
		}
		m.triggerPersist()
		if w.SessionID == "" {
			m.setViewportContent(fmt.Sprintf("cannot continue %s: session id not found in output", w.ID), false)
			if m.daemon {
				if cmd := m.checkDaemonComplete(); cmd != nil {
					return m, cmd
				}
			}
			return m, nil
		}
		return m, m.openContinueDialog(w)

	case workerKilledMsg:
		if w := m.manager.Get(msg.WorkerID); w != nil {
			w.State = worker.StateKilled
			w.ExitedAt = time.Now()
			w.Handle = nil
			m.logDaemonEvent(workerKillEvent(msg.WorkerID))
			m.refreshTableRows()
			if w.ID == m.selectedWorkerID {
				m.refreshViewportFromSelected(true)
			}
			m.triggerPersist()
		}
		return m, nil

	case analyzeCompletedMsg:
		m.analysisLoading = false
		m.analysisMode = true
		if msg.Err != nil {
			m.analysisResult = &AnalysisResult{
				WorkerID:  msg.WorkerID,
				RootCause: fmt.Sprintf("analysis failed: %v", msg.Err),
			}
		} else {
			m.analysisResult = &AnalysisResult{
				WorkerID:        msg.WorkerID,
				RootCause:       msg.RootCause,
				SuggestedPrompt: msg.SuggestedPrompt,
			}
		}
		m.updateKeyStates()
		m.refreshViewportFromSelected(false)
		return m, nil

	case genPromptCompletedMsg:
		m.genPromptLoading = false
		m.updateKeyStates()
		if msg.Err != nil {
			m.refreshViewportFromSelected(false)
			return m, nil
		}

		role := "coder"
		for _, t := range m.loadedTasks {
			if t.ID == msg.TaskID && strings.TrimSpace(t.SuggestedRole) != "" {
				role = t.SuggestedRole
				break
			}
		}
		return m, m.openSpawnDialogWithPrefill(role, msg.Prompt, nil)

	case tickMsg:
		m.refreshTableRows()
		return m, tickCmd()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		m.refreshTableRows()
		if m.analysisLoading || m.genPromptLoading {
			m.refreshViewportFromSelected(false)
		}
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
	for _, w := range ordered {
		status := plainStatus(w.State, w.ExitCode)
		if w.State == worker.StateRunning {
			status = "⟳ running"
		}

		idLabel := w.ID
		if prefix := prefixes[w.ID]; prefix != "" {
			idLabel = prefix + w.ID
		}

		row := table.Row{idLabel, status, w.Role, w.FormatDuration()}
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
	if m.analysisLoading {
		m.setViewportContent(fmt.Sprintf("%s analyzing failure for %s...", m.spinner.View(), m.analysisWorkerID), false)
		return
	}

	if m.genPromptLoading {
		m.setViewportContent(fmt.Sprintf("%s generating implementation prompt...", m.spinner.View()), false)
		return
	}

	if m.analysisMode && m.analysisResult != nil {
		m.setViewportContent(m.renderAnalysisView(), false)
		return
	}

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

// handleAnalysisModeKeys handles key events when analysis mode is active.
// Returns the model, command, and whether the key was consumed.
func (m *Model) handleAnalysisModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Back) {
		m.analysisMode = false
		m.analysisResult = nil
		m.updateKeyStates()
		m.refreshViewportFromSelected(false)
		return m, nil
	}

	if key.Matches(msg, m.keys.Restart) && m.analysisResult != nil && strings.TrimSpace(m.analysisResult.SuggestedPrompt) != "" {
		role := "coder"
		if w := m.manager.Get(m.analysisResult.WorkerID); w != nil {
			role = w.Role
		}
		suggestedPrompt := m.analysisResult.SuggestedPrompt
		m.analysisMode = false
		m.analysisResult = nil
		m.updateKeyStates()
		return m, m.openSpawnDialogWithPrefill(role, suggestedPrompt, nil)
	}

	return m, nil
}

func (m *Model) updateFullScreenKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.analysisMode {
		return m.handleAnalysisModeKeys(msg)
	}

	if key.Matches(msg, m.keys.Back) {
		m.fullScreen = false
		m.recalculateLayout()
		m.updateKeyStates()
		return m, nil
	}

	if key.Matches(msg, m.keys.Continue) {
		return m, m.continueSelectedWorkerCmd()
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
	if m.analysisMode {
		return m.handleAnalysisModeKeys(msg)
	}

	if key.Matches(msg, m.keys.Spawn) {
		return m, m.openSpawnDialog()
	}

	if key.Matches(msg, m.keys.Continue) {
		return m, m.continueSelectedWorkerCmd()
	}

	if key.Matches(msg, m.keys.MarkDone) {
		selected := m.selectedWorker()
		if selected != nil && selected.State == worker.StateRunning && selected.Handle != nil {
			return m, markWorkerDoneCmd(selected.ID, selected.Handle, selected.Output, 3*time.Second)
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

	if key.Matches(msg, m.keys.Analyze) {
		selected := m.selectedWorker()
		if selected != nil && selected.State == worker.StateFailed {
			m.analysisMode = false
			m.analysisResult = nil
			m.analysisLoading = true
			m.analysisWorkerID = selected.ID
			m.updateKeyStates()
			m.refreshViewportFromSelected(false)

			outputTail := ""
			if selected.Output != nil {
				outputTail = selected.Output.Tail(200)
			}
			return m, analyzeCmd(m.backend, selected.ID, selected.Role, selected.ExitCode, selected.FormatDuration(), outputTail)
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.GenPrompt) {
		selectedTask := m.selectTaskForPromptGen()
		if selectedTask == nil {
			return m, nil
		}

		m.genPromptLoading = true
		m.updateKeyStates()
		m.refreshViewportFromSelected(false)
		return m, genPromptCmd(
			m.backend,
			selectedTask.ID,
			selectedTask.Title,
			selectedTask.Description,
			selectedTask.SuggestedRole,
			selectedTask.Dependencies,
		)
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

func (m *Model) updateTaskPanelKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Approve):
		if m.selectedTaskIdx >= 0 && m.selectedTaskIdx < len(m.loadedTasks) {
			if m.loadedTasks[m.selectedTaskIdx].State == task.TaskForReview {
				m.loadedTasks[m.selectedTaskIdx].State = task.TaskDone
				m.resolveTaskDependencies()
				m.updateKeyStates()
				m.triggerPersist()
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Reject):
		if m.selectedTaskIdx >= 0 && m.selectedTaskIdx < len(m.loadedTasks) {
			t := &m.loadedTasks[m.selectedTaskIdx]
			if t.State == task.TaskForReview {
				t.State = task.TaskUnassigned
				t.WorkerID = ""
				m.resolveTaskDependencies()
				m.updateKeyStates()
				m.triggerPersist()
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Continue):
		return m, m.continueSelectedWorkerCmd()
	case key.Matches(msg, m.keys.Up):
		if m.selectedTaskIdx > 0 {
			m.selectedTaskIdx--
		}
		m.updateKeyStates()
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if m.selectedTaskIdx < len(m.loadedTasks)-1 {
			m.selectedTaskIdx++
		}
		m.updateKeyStates()
		return m, nil
	case key.Matches(msg, m.keys.Select), key.Matches(msg, m.keys.Spawn):
		if m.selectedTaskIdx >= 0 && m.selectedTaskIdx < len(m.loadedTasks) {
			t := m.loadedTasks[m.selectedTaskIdx]
			if t.State == task.TaskUnassigned {
				role := t.SuggestedRole
				if role == "" {
					role = "coder"
				}
				return m, m.openSpawnDialogWithTaskPrefill(role, strings.TrimSpace(t.Description), nil, t.ID)
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Batch):
		return m, m.openBatchDialog()
	default:
		return m, nil
	}
}

func (m *Model) updateViewportKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.analysisMode {
		return m.handleAnalysisModeKeys(msg)
	}

	if key.Matches(msg, m.keys.Continue) {
		return m, m.continueSelectedWorkerCmd()
	}

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

func (m *Model) continueSelectedWorkerCmd() tea.Cmd {
	selected := m.selectedWorker()
	if selected == nil {
		return nil
	}

	if selected.State == worker.StateRunning && selected.Handle != nil {
		return killAndContinueCmd(selected.ID, selected.Handle, selected.Output)
	}

	if (selected.State == worker.StateExited || selected.State == worker.StateFailed) && strings.TrimSpace(selected.SessionID) != "" {
		return m.openContinueDialog(selected)
	}

	return nil
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

func (m *Model) openSpawnDialogWithTaskPrefill(role, prompt string, files []string, taskID string) tea.Cmd {
	m.showSpawnDialog = true
	m.spawnDraft = spawnDialogDraft{Role: role, Prompt: prompt, Files: strings.Join(files, ", ")}
	m.spawnForm = newSpawnDialogModelWithPrefill(role, prompt, files)
	m.spawnForm.taskID = taskID
	return m.spawnForm.focusCurrentField()
}

func (m *Model) spawnAllTasks() tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	for _, t := range m.loadedTasks {
		if t.State != task.TaskUnassigned {
			continue
		}

		role := t.SuggestedRole
		if role == "" {
			role = "coder"
		}

		id := m.manager.NextWorkerID()
		w := &worker.Worker{
			ID:        id,
			Role:      role,
			Prompt:    strings.TrimSpace(t.Description),
			TaskID:    t.ID,
			State:     worker.StateSpawning,
			SpawnedAt: time.Now(),
			Output:    worker.NewOutputBuffer(worker.DefaultMaxLines),
		}
		m.manager.Add(w)

		for i := range m.loadedTasks {
			if m.loadedTasks[i].ID == t.ID {
				m.loadedTasks[i].State = task.TaskInProgress
				m.loadedTasks[i].WorkerID = w.ID
				break
			}
		}

		cfg := worker.SpawnConfig{ID: w.ID, Role: w.Role, Prompt: w.Prompt}
		cmds = append(cmds, spawnWorkerCmd(m.backend, cfg))
	}

	m.workers = m.manager.All()
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) checkDaemonComplete() tea.Cmd {
	workers := m.manager.All()
	if len(workers) == 0 {
		return nil
	}

	for _, w := range workers {
		if w.State == worker.StateRunning || w.State == worker.StateSpawning {
			return nil
		}
	}

	if m.spawnAll {
		m.resolveTaskDependencies()
		if cmd := m.spawnAllTasks(); cmd != nil {
			return cmd
		}
	}

	total := 0
	passed := 0
	failed := 0
	for _, w := range workers {
		total++
		if w.State == worker.StateExited {
			passed++
		} else {
			failed++
		}
	}

	exitCode := 0
	if failed > 0 {
		exitCode = 1
	}

	m.daemonDone = true
	m.daemonExitCode = exitCode
	m.logDaemonEvent(sessionEndEvent(total, passed, failed, time.Since(m.sessionStart), exitCode))
	return tea.Quit
}

func (m *Model) resolveTaskDependencies() {
	doneIDs := make(map[string]bool, len(m.loadedTasks))
	for _, t := range m.loadedTasks {
		if t.State == task.TaskDone {
			doneIDs[t.ID] = true
		}
	}

	for i := range m.loadedTasks {
		if m.loadedTasks[i].State != task.TaskBlocked {
			continue
		}
		allDone := true
		for _, dep := range m.loadedTasks[i].Dependencies {
			if !doneIDs[dep] {
				allDone = false
				break
			}
		}
		if allDone {
			m.loadedTasks[i].State = task.TaskUnassigned
		}
	}
}

func (m *Model) selectTaskForPromptGen() *task.Task {
	if len(m.loadedTasks) == 0 {
		return nil
	}

	if selected := m.selectedWorker(); selected != nil && selected.TaskID != "" {
		for i := range m.loadedTasks {
			if m.loadedTasks[i].ID == selected.TaskID {
				return &m.loadedTasks[i]
			}
		}
	}

	if m.selectedTaskIdx >= 0 && m.selectedTaskIdx < len(m.loadedTasks) {
		return &m.loadedTasks[m.selectedTaskIdx]
	}

	for i := range m.loadedTasks {
		if m.loadedTasks[i].State == task.TaskUnassigned {
			return &m.loadedTasks[i]
		}
	}

	return &m.loadedTasks[0]
}
