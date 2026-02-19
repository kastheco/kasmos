package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/worker"
)

type spawnDialogDraft struct {
	Role   string
	Prompt string
	Files  string
}

type spawnDialogModel struct {
	roles      []spawnRoleOption
	roleIndex  int
	prompt     textarea.Model
	files      textinput.Model
	focusedIdx int
	taskID     string
}

type spawnRoleOption struct {
	role        string
	description string
}

type continueDialogModel struct {
	parentWorkerID string
	parentRole     string
	parentState    worker.WorkerState
	sessionID      string
	followUp       textarea.Model
	focusedIdx     int
}

type unfinishedDep struct {
	ID    string
	State string
}

const (
	spawnFocusRole = iota
	spawnFocusPrompt
	spawnFocusFiles
)

const (
	continueFocusFollowUp = iota
)

func (m *Model) openSpawnDialog() tea.Cmd {
	m.showSpawnDialog = true
	m.spawnDraft = spawnDialogDraft{Role: "coder"}
	m.spawnForm = newSpawnDialogModel()
	m.spawnForm.taskID = ""
	m.updateKeyStates()
	return m.spawnForm.focusCurrentField()
}

func (m *Model) openSpawnDialogWithPrefill(role, prompt string, files []string) tea.Cmd {
	m.showSpawnDialog = true
	m.spawnDraft = spawnDialogDraft{Role: role, Prompt: prompt, Files: strings.Join(files, ", ")}
	m.spawnForm = newSpawnDialogModelWithPrefill(role, prompt, files)
	m.spawnForm.taskID = ""
	m.updateKeyStates()
	return m.spawnForm.focusCurrentField()
}

func (m *Model) openBatchDialog() tea.Cmd {
	m.batchSelections = make([]bool, len(m.loadedTasks))
	m.batchFocusedIdx = 0
	m.showBatchDialog = true
	m.updateKeyStates()
	if idx := m.firstBatchSelectableIdx(); idx >= 0 {
		m.batchFocusedIdx = idx
	}
	return nil
}

func (m *Model) closeBatchDialog() {
	m.showBatchDialog = false
	m.batchSelections = nil
	m.batchFocusedIdx = 0
	m.updateKeyStates()
}

func (m *Model) unfinishedDeps(t task.Task) []unfinishedDep {
	stateByID := make(map[string]task.TaskState, len(m.loadedTasks))
	for _, loadedTask := range m.loadedTasks {
		stateByID[loadedTask.ID] = loadedTask.State
	}

	deps := make([]unfinishedDep, 0, len(t.Dependencies))
	for _, depID := range t.Dependencies {
		state, exists := stateByID[depID]
		if !exists {
			deps = append(deps, unfinishedDep{ID: depID, State: "unknown"})
			continue
		}
		if state == task.TaskDone {
			continue
		}
		deps = append(deps, unfinishedDep{ID: depID, State: taskStateLabel(state)})
	}

	return deps
}

func taskStateLabel(state task.TaskState) string {
	switch state {
	case task.TaskUnassigned:
		return "unassigned"
	case task.TaskBlocked:
		return "blocked"
	case task.TaskInProgress:
		return "in-progress"
	case task.TaskForReview:
		return "for-review"
	case task.TaskFailed:
		return "failed"
	case task.TaskDone:
		return "done"
	default:
		return "unknown"
	}
}

func newSpawnDialogModel() *spawnDialogModel {
	prompt := styledTextArea()
	prompt.Placeholder = "describe the task for this worker"
	prompt.SetWidth(58)
	prompt.SetHeight(6)

	files := styledTextInput()
	files.Placeholder = "path/to/file.go, another/file.go"
	files.SetWidth(58)

	form := &spawnDialogModel{
		roles: []spawnRoleOption{
			{role: "planner", description: "research and planning, read-only filesystem"},
			{role: "coder", description: "implementation, full tool access"},
			{role: "reviewer", description: "code review, read-only + test execution"},
			{role: "release", description: "merge, finalization, cleanup operations"},
		},
		roleIndex:  1,
		prompt:     prompt,
		files:      files,
		focusedIdx: spawnFocusRole,
	}

	form.prompt.Blur()
	form.files.Blur()
	return form
}

func newSpawnDialogModelWithPrefill(role, promptText string, files []string) *spawnDialogModel {
	form := newSpawnDialogModel()

	for i, opt := range form.roles {
		if opt.role == role {
			form.roleIndex = i
			break
		}
	}

	form.prompt.SetValue(promptText)

	if len(files) > 0 {
		form.files.SetValue(strings.Join(files, ", "))
	}

	return form
}

func (f *spawnDialogModel) focusCurrentField() tea.Cmd {
	f.prompt.Blur()
	f.files.Blur()

	switch f.focusedIdx {
	case spawnFocusPrompt:
		return f.prompt.Focus()
	case spawnFocusFiles:
		return f.files.Focus()
	default:
		return nil
	}
}

func (f *spawnDialogModel) cycleFocus(delta int) tea.Cmd {
	f.focusedIdx = (f.focusedIdx + delta + 3) % 3
	return f.focusCurrentField()
}

func (m *Model) closeSpawnDialog() {
	m.showSpawnDialog = false
	m.spawnForm = nil
	m.spawnDraft = spawnDialogDraft{}
	m.updateKeyStates()
}

func (m *Model) openContinueDialog(parent *worker.Worker) tea.Cmd {
	if parent == nil || parent.SessionID == "" {
		return nil
	}

	m.showContinueDialog = true
	m.continueParentID = parent.ID
	m.continueForm = newContinueDialogModel(parent)
	m.updateKeyStates()
	return m.continueForm.focusCurrentField()
}

func (m *Model) closeContinueDialog() {
	m.showContinueDialog = false
	m.continueParentID = ""
	m.continueForm = nil
	m.updateKeyStates()
}

func newContinueDialogModel(parent *worker.Worker) *continueDialogModel {
	followUp := styledTextArea()
	followUp.Placeholder = "describe what to do next..."
	followUp.SetWidth(58)
	followUp.SetHeight(6)
	followUp.Blur()

	return &continueDialogModel{
		parentWorkerID: parent.ID,
		parentRole:     parent.Role,
		parentState:    parent.State,
		sessionID:      parent.SessionID,
		followUp:       followUp,
		focusedIdx:     continueFocusFollowUp,
	}
}

func (f *continueDialogModel) focusCurrentField() tea.Cmd {
	if f == nil {
		return nil
	}
	f.followUp.Blur()
	return f.followUp.Focus()
}

func (f *continueDialogModel) cycleFocus(_ int) tea.Cmd {
	return f.focusCurrentField()
}

func (m *Model) updateContinueDialog(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.continueForm == nil {
		m.closeContinueDialog()
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, m.keys.Back) {
			m.closeContinueDialog()
			return m, func() tea.Msg { return continueDialogCancelledMsg{} }
		}

		switch keyMsg.String() {
		case "tab":
			return m, m.continueForm.cycleFocus(1)
		case "shift+tab":
			return m, m.continueForm.cycleFocus(-1)
		case "enter":
			followUp := strings.TrimSpace(m.continueForm.followUp.Value())
			if followUp == "" {
				return m, nil
			}
			submitted := continueDialogSubmittedMsg{
				ParentWorkerID: m.continueForm.parentWorkerID,
				SessionID:      m.continueForm.sessionID,
				FollowUp:       followUp,
			}
			m.closeContinueDialog()
			return m, func() tea.Msg { return submitted }
		}
	}

	var cmd tea.Cmd
	m.continueForm.followUp, cmd = m.continueForm.followUp.Update(msg)
	return m, cmd
}

func (m *Model) updateQuitConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if key.Matches(keyMsg, m.keys.Back) {
		m.showQuitConfirm = false
		m.updateKeyStates()
		return m, func() tea.Msg { return quitCancelledMsg{} }
	}

	switch keyMsg.String() {
	case "left", "shift+tab":
		m.quitConfirmFocused = (m.quitConfirmFocused + 1) % 2
		return m, nil
	case "right", "tab":
		m.quitConfirmFocused = (m.quitConfirmFocused + 1) % 2
		return m, nil
	case "enter":
		m.showQuitConfirm = false
		m.updateKeyStates()
		if m.quitConfirmFocused == 0 {
			return m, func() tea.Msg { return quitConfirmedMsg{} }
		}
		return m, func() tea.Msg { return quitCancelledMsg{} }
	}

	return m, nil
}

func (m *Model) openBlockedConfirmDialog(taskIdx int) {
	m.showBlockedConfirm = true
	m.blockedConfirmTaskIdx = taskIdx
	m.blockedConfirmFocused = 1
	m.updateKeyStates()
}

func (m *Model) closeBlockedConfirmDialog() {
	m.showBlockedConfirm = false
	m.blockedConfirmTaskIdx = 0
	m.blockedConfirmFocused = 0
	m.updateKeyStates()
}

func (m *Model) updateBlockedConfirmDialog(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, m.keys.Back):
		m.closeBlockedConfirmDialog()
		return m, nil
	case keyMsg.String() == "left" || keyMsg.String() == "right" || keyMsg.String() == "tab":
		if m.blockedConfirmFocused == 0 {
			m.blockedConfirmFocused = 1
		} else {
			m.blockedConfirmFocused = 0
		}
		return m, nil
	case keyMsg.String() == "enter":
		if m.blockedConfirmFocused == 0 {
			taskIdx := m.blockedConfirmTaskIdx
			m.closeBlockedConfirmDialog()
			return m, func() tea.Msg {
				return blockedConfirmProceedMsg{TaskIdx: taskIdx}
			}
		}
		m.closeBlockedConfirmDialog()
		return m, nil
	}

	return m, nil
}

func (m *Model) updateSpawnDialog(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, m.keys.Back) {
			m.closeSpawnDialog()
			return m, func() tea.Msg { return spawnDialogCancelledMsg{} }
		}
		if m.spawnForm == nil {
			m.closeSpawnDialog()
			return m, nil
		}

		switch {
		case keyMsg.String() == "tab":
			return m, m.spawnForm.cycleFocus(1)
		case keyMsg.String() == "shift+tab":
			return m, m.spawnForm.cycleFocus(-1)
		case keyMsg.String() == "up" && m.spawnForm.focusedIdx == spawnFocusRole:
			if m.spawnForm.roleIndex > 0 {
				m.spawnForm.roleIndex--
			}
			return m, nil
		case keyMsg.String() == "down" && m.spawnForm.focusedIdx == spawnFocusRole:
			if m.spawnForm.roleIndex < len(m.spawnForm.roles)-1 {
				m.spawnForm.roleIndex++
			}
			return m, nil
		case keyMsg.String() == "enter" && m.spawnForm.focusedIdx == spawnFocusRole:
			prompt := strings.TrimSpace(m.spawnForm.prompt.Value())
			if prompt == "" {
				return m, nil
			}
			submitted := spawnDialogSubmittedMsg{
				Role:   m.spawnForm.roles[m.spawnForm.roleIndex].role,
				Prompt: prompt,
				TaskID: m.spawnForm.taskID,
			}
			m.closeSpawnDialog()
			return m, func() tea.Msg { return submitted }
		case keyMsg.String() == "enter" && m.spawnForm.focusedIdx == spawnFocusFiles:
			m.spawnDraft = spawnDialogDraft{
				Role:   m.spawnForm.roles[m.spawnForm.roleIndex].role,
				Prompt: strings.TrimSpace(m.spawnForm.prompt.Value()),
				Files:  strings.TrimSpace(m.spawnForm.files.Value()),
			}
			submitted := spawnDialogSubmittedMsg{
				Role:   m.spawnDraft.Role,
				Prompt: m.spawnDraft.Prompt,
				Files:  parseSpawnFiles(m.spawnDraft.Files),
				TaskID: m.spawnForm.taskID,
			}
			m.closeSpawnDialog()
			return m, func() tea.Msg { return submitted }
		}
	}

	if m.spawnForm == nil {
		m.closeSpawnDialog()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.spawnForm.focusedIdx {
	case spawnFocusPrompt:
		m.spawnForm.prompt, cmd = m.spawnForm.prompt.Update(msg)
	case spawnFocusFiles:
		m.spawnForm.files, cmd = m.spawnForm.files.Update(msg)
	default:
		cmd = nil
	}

	return m, cmd
}

func parseSpawnFiles(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}

	parts := strings.Split(input, ",")
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		file := strings.TrimSpace(part)
		if file == "" {
			continue
		}
		files = append(files, file)
	}

	return files
}

func (m *Model) renderSpawnDialog() string {
	if m.spawnForm == nil {
		return m.renderWithBackdrop("")
	}

	roleLines := make([]string, 0, len(m.spawnForm.roles))
	for i, opt := range m.spawnForm.roles {
		marker := "○"
		if i == m.spawnForm.roleIndex {
			marker = "●"
		}
		line := fmt.Sprintf("  %s %-8s %s", marker, opt.role, opt.description)
		if i == m.spawnForm.roleIndex {
			line = lipgloss.NewStyle().Foreground(colorPurple).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(colorLightGray).Render(line)
		}
		roleLines = append(roleLines, line)
	}

	roleBox := strings.Join(roleLines, "\n")
	helpText := lipgloss.NewStyle().Foreground(colorMidGray).Render("tab/s-tab field  up/down role  enter on files to spawn  esc cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		dialogHeaderStyle.Render("spawn worker"),
		"",
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("agent role"),
		roleBox,
		"",
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("prompt"),
		m.spawnForm.prompt.View(),
		"",
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("attach files (optional)"),
		m.spawnForm.files.View(),
		"",
		helpText,
	)

	dialog := dialogStyle.Width(70).Render(content)
	return m.renderWithBackdrop(dialog)
}

func (m *Model) updateBatchDialog(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if key.Matches(keyMsg, m.keys.Back) {
		m.closeBatchDialog()
		return m, nil
	}

	if len(m.loadedTasks) == 0 {
		m.closeBatchDialog()
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		m.moveBatchFocus(-1)
		return m, nil
	case "down", "j":
		m.moveBatchFocus(1)
		return m, nil
	case " ":
		if m.batchFocusedIdx >= 0 && m.batchFocusedIdx < len(m.loadedTasks) && m.loadedTasks[m.batchFocusedIdx].State == task.TaskUnassigned {
			m.batchSelections[m.batchFocusedIdx] = !m.batchSelections[m.batchFocusedIdx]
		}
		return m, nil
	case "enter":
		cmds := make([]tea.Cmd, 0)
		for i, selected := range m.batchSelections {
			if !selected || i >= len(m.loadedTasks) {
				continue
			}
			t := m.loadedTasks[i]
			if t.State != task.TaskUnassigned {
				continue
			}
			role := t.SuggestedRole
			if role == "" {
				role = "coder"
			}
			msg := spawnDialogSubmittedMsg{
				Role:   role,
				Prompt: strings.TrimSpace(t.Description),
				Files:  nil,
				TaskID: t.ID,
			}
			cmds = append(cmds, func(m spawnDialogSubmittedMsg) tea.Cmd {
				return func() tea.Msg { return m }
			}(msg))
		}
		m.closeBatchDialog()
		if len(cmds) == 0 {
			return m, nil
		}
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

func (m *Model) moveBatchFocus(dir int) {
	if len(m.loadedTasks) == 0 {
		m.batchFocusedIdx = 0
		return
	}

	idx := m.batchFocusedIdx
	for range len(m.loadedTasks) {
		idx = (idx + dir + len(m.loadedTasks)) % len(m.loadedTasks)
		if m.loadedTasks[idx].State == task.TaskUnassigned {
			m.batchFocusedIdx = idx
			return
		}
	}
}

func (m *Model) firstBatchSelectableIdx() int {
	for i, t := range m.loadedTasks {
		if t.State == task.TaskUnassigned {
			return i
		}
	}
	return -1
}

func (m *Model) renderBatchDialog() string {
	lines := make([]string, 0, len(m.loadedTasks))
	for i, t := range m.loadedTasks {
		if t.State != task.TaskUnassigned {
			continue
		}
		check := "[ ]"
		if i < len(m.batchSelections) && m.batchSelections[i] {
			check = "[x]"
		}
		style := lipgloss.NewStyle().Foreground(colorLightGray)
		if i == m.batchFocusedIdx {
			style = style.Foreground(colorPurple).Bold(true)
		}
		lines = append(lines, style.Render(fmt.Sprintf("  %s %s  %s", check, t.ID, t.Title)))
	}

	if len(lines) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorMidGray).Render("  no unassigned tasks available"))
	}

	helpText := lipgloss.NewStyle().Foreground(colorMidGray).Render("j/k navigate  space toggle  enter spawn selected  esc cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		dialogHeaderStyle.Render("batch spawn"),
		"",
		strings.Join(lines, "\n"),
		"",
		helpText,
	)

	dialog := dialogStyle.Width(70).Render(content)
	return m.renderWithBackdrop(dialog)
}

func (m *Model) renderContinueDialog() string {
	if m.continueForm == nil {
		return m.renderWithBackdrop("")
	}

	parentStatus := statusIndicator(m.continueForm.parentState, 0)
	meta := lipgloss.JoinVertical(
		lipgloss.Left,
		fmt.Sprintf("worker: %s", m.continueForm.parentWorkerID),
		lipgloss.JoinHorizontal(lipgloss.Left, "role: ", roleBadge(m.continueForm.parentRole)),
		fmt.Sprintf("status: %s", parentStatus),
		lipgloss.JoinHorizontal(lipgloss.Left,
			"session: ",
			lipgloss.NewStyle().Foreground(colorLightBlue).Render(m.continueForm.sessionID),
		),
	)

	helpText := lipgloss.NewStyle().Foreground(colorMidGray).Render("tab/s-tab field  enter submit  esc cancel")
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		dialogHeaderStyle.Render("continue session"),
		"",
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("parent worker"),
		meta,
		"",
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("follow-up message"),
		m.continueForm.followUp.View(),
		"",
		helpText,
	)

	dialog := dialogStyle.Width(70).Render(content)
	return m.renderWithBackdrop(dialog)
}

func (m *Model) renderQuitConfirm() string {
	running := m.runningWorkersCount()
	body := fmt.Sprintf("%d workers are still running. they will be terminated.", running)

	forceStyle := inactiveButtonStyle
	cancelStyle := inactiveButtonStyle
	if m.quitConfirmFocused == 0 {
		forceStyle = alertButtonStyle
	} else {
		cancelStyle = activeButtonStyle
	}

	buttons := lipgloss.JoinHorizontal(
		lipgloss.Left,
		forceStyle.Render("force quit"),
		"  ",
		cancelStyle.Render("cancel"),
	)

	header := lipgloss.NewStyle().Foreground(colorOrange).Bold(true).Render("⚠ quit kasmos?")
	helpText := lipgloss.NewStyle().Foreground(colorMidGray).Render("left/right or tab switch  enter select  esc cancel")
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		lipgloss.NewStyle().Foreground(colorLightGray).Render(body),
		"",
		buttons,
		"",
		helpText,
	)

	dialog := alertDialogStyle.Width(64).Render(content)
	return m.renderWithBackdrop(dialog)
}

func (m *Model) renderBlockedConfirmDialog() string {
	if m.blockedConfirmTaskIdx < 0 || m.blockedConfirmTaskIdx >= len(m.loadedTasks) {
		return m.renderWithBackdrop("")
	}

	t := m.loadedTasks[m.blockedConfirmTaskIdx]
	deps := m.unfinishedDeps(t)

	header := lipgloss.NewStyle().Foreground(colorOrange).Bold(true).Render("! blocked task")
	taskInfo := lipgloss.NewStyle().Foreground(colorLightGray).Render(fmt.Sprintf("%s - %s", t.ID, t.Title))

	depLines := make([]string, 0, len(deps))
	for _, dep := range deps {
		depLines = append(depLines, fmt.Sprintf("  %s (%s)", dep.ID, lipgloss.NewStyle().Foreground(colorOrange).Render(dep.State)))
	}

	depSection := lipgloss.NewStyle().Foreground(colorMidGray).Render("(no unfinished dependencies detected)")
	if len(depLines) > 0 {
		depSection = lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Foreground(colorCream).Render("unfinished dependencies:"),
			strings.Join(depLines, "\n"),
		)
	}

	spawnStyle := inactiveButtonStyle
	cancelStyle := inactiveButtonStyle
	if m.blockedConfirmFocused == 0 {
		spawnStyle = alertButtonStyle
	} else {
		cancelStyle = activeButtonStyle
	}

	buttons := lipgloss.JoinHorizontal(
		lipgloss.Left,
		spawnStyle.Render("spawn anyway"),
		"  ",
		cancelStyle.Render("cancel"),
	)

	helpText := lipgloss.NewStyle().Foreground(colorMidGray).Render("left/right or tab switch  enter select  esc cancel")
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		taskInfo,
		"",
		depSection,
		"",
		buttons,
		"",
		helpText,
	)

	dialog := alertDialogStyle.Width(64).Render(content)
	return m.renderWithBackdrop(dialog)
}
