package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

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
	return m.spawnForm.focusCurrentField()
}

func (m *Model) openSpawnDialogWithPrefill(role, prompt string, files []string) tea.Cmd {
	m.showSpawnDialog = true
	m.spawnDraft = spawnDialogDraft{Role: role, Prompt: prompt, Files: strings.Join(files, ", ")}
	m.spawnForm = newSpawnDialogModelWithPrefill(role, prompt, files)
	return m.spawnForm.focusCurrentField()
}

func newSpawnDialogModel() *spawnDialogModel {
	prompt := styledTextArea()
	prompt.Placeholder = "Describe the task for this worker"
	prompt.SetWidth(58)
	prompt.SetHeight(6)

	files := styledTextInput()
	files.Placeholder = "path/to/file.go, another/file.go"
	files.SetWidth(58)

	form := &spawnDialogModel{
		roles: []spawnRoleOption{
			{role: "planner", description: "Research and planning, read-only filesystem"},
			{role: "coder", description: "Implementation, full tool access"},
			{role: "reviewer", description: "Code review, read-only + test execution"},
			{role: "release", description: "Merge, finalization, cleanup operations"},
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
}

func (m *Model) openContinueDialog(parent *worker.Worker) tea.Cmd {
	if parent == nil || parent.SessionID == "" {
		return nil
	}

	m.showContinueDialog = true
	m.continueParentID = parent.ID
	m.continueForm = newContinueDialogModel(parent)
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
	followUp.Placeholder = "Describe what to do next..."
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
	helpText := lipgloss.NewStyle().Foreground(colorMidGray).Render("tab/S-tab field  up/down role  enter on files to spawn  esc cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		dialogHeaderStyle.Render("Spawn Worker"),
		"",
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("Agent Role"),
		roleBox,
		"",
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("Prompt"),
		m.spawnForm.prompt.View(),
		"",
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("Attach Files (optional)"),
		m.spawnForm.files.View(),
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
		fmt.Sprintf("Worker: %s", m.continueForm.parentWorkerID),
		lipgloss.JoinHorizontal(lipgloss.Left, "Role: ", roleBadge(m.continueForm.parentRole)),
		fmt.Sprintf("Status: %s", parentStatus),
		lipgloss.JoinHorizontal(lipgloss.Left,
			"Session: ",
			lipgloss.NewStyle().Foreground(colorLightBlue).Render(m.continueForm.sessionID),
		),
	)

	helpText := lipgloss.NewStyle().Foreground(colorMidGray).Render("tab/S-tab field  enter submit  esc cancel")
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		dialogHeaderStyle.Render("Continue Session"),
		"",
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("Parent Worker"),
		meta,
		"",
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("Follow-up Message"),
		m.continueForm.followUp.View(),
		"",
		helpText,
	)

	dialog := dialogStyle.Width(70).Render(content)
	return m.renderWithBackdrop(dialog)
}

func (m *Model) renderQuitConfirm() string {
	running := m.runningWorkersCount()
	body := fmt.Sprintf("%d workers are still running. They will be terminated.", running)

	forceStyle := inactiveButtonStyle
	cancelStyle := inactiveButtonStyle
	if m.quitConfirmFocused == 0 {
		forceStyle = alertButtonStyle
	} else {
		cancelStyle = activeButtonStyle
	}

	buttons := lipgloss.JoinHorizontal(
		lipgloss.Left,
		forceStyle.Render("Force Quit"),
		"  ",
		cancelStyle.Render("Cancel"),
	)

	header := lipgloss.NewStyle().Foreground(colorOrange).Bold(true).Render("⚠ Quit kasmos?")
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
