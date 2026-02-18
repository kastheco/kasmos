package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
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

const (
	spawnFocusRole = iota
	spawnFocusPrompt
	spawnFocusFiles
)

func (m *Model) openSpawnDialog() tea.Cmd {
	m.showSpawnDialog = true
	m.spawnDraft = spawnDialogDraft{Role: "coder"}
	m.spawnForm = newSpawnDialogModel()
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
