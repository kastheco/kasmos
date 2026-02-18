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

const (
	newDialogStagePicker = 0
	newDialogStageForm   = 1
)

const (
	newDialogTypeFeatureSpec = "feature-spec"
	newDialogTypeGSD         = "gsd"
	newDialogTypeYolo        = "yolo"
)

type newFormModel struct {
	formType string

	featureSlug    textinput.Model
	featureMission textinput.Model

	gsdFilename textinput.Model
	gsdTasks    textarea.Model

	planFilename textinput.Model
	planTitle    textinput.Model
	planContent  textarea.Model

	focusedIdx int
	errMsg     string
}

func (m *Model) openNewDialog() tea.Cmd {
	m.showNewDialog = true
	m.newDialogStage = newDialogStagePicker
	m.newDialogType = ""
	m.newForm = nil
	m.updateKeyStates()
	return nil
}

func (m *Model) closeNewDialog() {
	m.showNewDialog = false
	m.newDialogStage = newDialogStagePicker
	m.newDialogType = ""
	m.newForm = nil
	m.updateKeyStates()
}

func newFormModelFor(formType string) *newFormModel {
	f := &newFormModel{formType: formType}

	switch formType {
	case newDialogTypeFeatureSpec:
		slug := styledTextInput()
		slug.Placeholder = "my-new-feature"
		slug.SetWidth(58)

		mission := styledTextInput()
		mission.Placeholder = "software-dev"
		mission.SetValue("software-dev")
		mission.SetWidth(58)

		f.featureSlug = slug
		f.featureMission = mission
	case newDialogTypeGSD:
		filename := styledTextInput()
		filename.Placeholder = "tasks.md"
		filename.SetValue("tasks.md")
		filename.SetWidth(58)

		tasks := styledTextArea()
		tasks.Placeholder = "one task per line"
		tasks.SetWidth(58)
		tasks.SetHeight(6)

		f.gsdFilename = filename
		f.gsdTasks = tasks
	case newDialogTypeYolo:
		filename := styledTextInput()
		filename.Placeholder = "yolo.md"
		filename.SetValue("yolo.md")
		filename.SetWidth(58)

		title := styledTextInput()
		title.Placeholder = "quick refactor"
		title.SetWidth(58)

		content := styledTextArea()
		content.Placeholder = "what needs doing..."
		content.SetWidth(58)
		content.SetHeight(6)

		f.planFilename = filename
		f.planTitle = title
		f.planContent = content
	default:
		return nil
	}

	_ = f.focusCurrentField()
	return f
}

func (f *newFormModel) fieldCount() int {
	switch f.formType {
	case newDialogTypeYolo:
		return 3
	default:
		return 2
	}
}

func (f *newFormModel) focusCurrentField() tea.Cmd {
	f.featureSlug.Blur()
	f.featureMission.Blur()
	f.gsdFilename.Blur()
	f.gsdTasks.Blur()
	f.planFilename.Blur()
	f.planTitle.Blur()
	f.planContent.Blur()

	switch f.formType {
	case newDialogTypeFeatureSpec:
		if f.focusedIdx == 0 {
			return f.featureSlug.Focus()
		}
		return f.featureMission.Focus()
	case newDialogTypeGSD:
		if f.focusedIdx == 0 {
			return f.gsdFilename.Focus()
		}
		return f.gsdTasks.Focus()
	case newDialogTypeYolo:
		switch f.focusedIdx {
		case 0:
			return f.planFilename.Focus()
		case 1:
			return f.planTitle.Focus()
		default:
			return f.planContent.Focus()
		}
	default:
		return nil
	}
}

func (f *newFormModel) cycleFocus(delta int) tea.Cmd {
	count := f.fieldCount()
	if count <= 0 {
		return nil
	}
	f.focusedIdx = (f.focusedIdx + delta + count) % count
	return f.focusCurrentField()
}

func (m *Model) updateNewDialog(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.showNewDialog {
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, m.keys.Back) {
			m.closeNewDialog()
			return m, func() tea.Msg { return newDialogCancelledMsg{} }
		}

		if m.newDialogStage == newDialogStagePicker {
			switch keyMsg.String() {
			case "s":
				return m, m.startNewDialogForm(newDialogTypeFeatureSpec)
			case "g":
				return m, m.startNewDialogForm(newDialogTypeGSD)
			case "y":
				return m, m.startNewDialogForm(newDialogTypeYolo)
			}
			return m, nil
		}

		if m.newDialogStage == newDialogStageForm {
			if m.newForm == nil {
				m.closeNewDialog()
				return m, nil
			}

			switch keyMsg.String() {
			case "tab":
				return m, m.newForm.cycleFocus(1)
			case "shift+tab":
				return m, m.newForm.cycleFocus(-1)
			case "enter":
				return m, m.submitNewDialogForm()
			}
		}
	}

	if m.newDialogStage != newDialogStageForm || m.newForm == nil {
		return m, nil
	}

	var cmd tea.Cmd
	switch m.newDialogType {
	case newDialogTypeFeatureSpec:
		if m.newForm.focusedIdx == 0 {
			m.newForm.featureSlug, cmd = m.newForm.featureSlug.Update(msg)
		} else {
			m.newForm.featureMission, cmd = m.newForm.featureMission.Update(msg)
		}
	case newDialogTypeGSD:
		if m.newForm.focusedIdx == 0 {
			m.newForm.gsdFilename, cmd = m.newForm.gsdFilename.Update(msg)
		} else {
			m.newForm.gsdTasks, cmd = m.newForm.gsdTasks.Update(msg)
		}
	case newDialogTypeYolo:
		switch m.newForm.focusedIdx {
		case 0:
			m.newForm.planFilename, cmd = m.newForm.planFilename.Update(msg)
		case 1:
			m.newForm.planTitle, cmd = m.newForm.planTitle.Update(msg)
		default:
			m.newForm.planContent, cmd = m.newForm.planContent.Update(msg)
		}
	}

	return m, cmd
}

func (m *Model) submitNewDialogForm() tea.Cmd {
	if m.newForm == nil {
		return nil
	}

	m.newForm.errMsg = ""

	switch m.newDialogType {
	case newDialogTypeFeatureSpec:
		slug := strings.TrimSpace(m.newForm.featureSlug.Value())
		mission := strings.TrimSpace(m.newForm.featureMission.Value())
		if slug == "" {
			m.newForm.errMsg = "slug is required."
			return nil
		}
		if !isKnownMission(mission) {
			m.newForm.errMsg = "mission must be software-dev, documentation, or research."
			return nil
		}
		m.closeNewDialog()
		return specCreateCmd(slug, mission)

	case newDialogTypeGSD:
		filename := strings.TrimSpace(m.newForm.gsdFilename.Value())
		tasks := parseLines(m.newForm.gsdTasks.Value())
		if filename == "" {
			m.newForm.errMsg = "filename is required."
			return nil
		}
		if len(tasks) == 0 {
			m.newForm.errMsg = "provide at least one task line."
			return nil
		}
		m.closeNewDialog()
		return gsdCreateCmd(filename, tasks)

	case newDialogTypeYolo:
		filename := strings.TrimSpace(m.newForm.planFilename.Value())
		title := strings.TrimSpace(m.newForm.planTitle.Value())
		content := strings.TrimSpace(m.newForm.planContent.Value())
		if filename == "" {
			m.newForm.errMsg = "filename is required."
			return nil
		}
		if title == "" {
			m.newForm.errMsg = "title is required."
			return nil
		}
		m.closeNewDialog()
		return planCreateCmd(filename, title, content)
	}

	m.closeNewDialog()
	return nil
}

func isKnownMission(mission string) bool {
	switch strings.TrimSpace(mission) {
	case "software-dev", "documentation", "research":
		return true
	default:
		return false
	}
}

func parseLines(value string) []string {
	parts := strings.Split(value, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lines = append(lines, part)
	}
	return lines
}

func (m *Model) renderNewDialog() string {
	if !m.showNewDialog {
		return ""
	}

	if m.newDialogStage == newDialogStagePicker {
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			dialogHeaderStyle.Render("new spec/plan"),
			"",
			newDialogOptionStyle.Render("[s] feature spec")+"  "+newDialogMutedStyle.Render("create a spec-kitty feature with research + planning"),
			newDialogOptionStyle.Render("[g] gsd task list")+"  "+newDialogMutedStyle.Render("create a checkbox task markdown file"),
			newDialogOptionStyle.Render("[y] yolo mode")+"    "+newDialogMutedStyle.Render("quick task with optional planning"),
			"",
			newDialogHelpStyle.Render("s/g/y select . esc cancel"),
		)
		dialog := dialogStyle.Width(80).Render(content)
		return m.renderWithBackdrop(dialog)
	}

	if m.newForm == nil {
		return m.renderWithBackdrop("")
	}

	errorLine := ""
	if strings.TrimSpace(m.newForm.errMsg) != "" {
		errorLine = newDialogErrorStyle.Render(m.newForm.errMsg)
	}

	switch m.newDialogType {
	case newDialogTypeFeatureSpec:
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			dialogHeaderStyle.Render("new feature spec"),
			"",
			lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("slug"),
			m.newForm.featureSlug.View(),
			"",
			lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("mission"),
			m.newForm.featureMission.View(),
			newDialogMutedStyle.Render("mission: software-dev | documentation | research"),
			errorLine,
			newDialogHelpStyle.Render("enter submit . tab next field . esc cancel"),
		)
		dialog := dialogStyle.Width(70).Render(content)
		return m.renderWithBackdrop(dialog)

	case newDialogTypeGSD:
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			dialogHeaderStyle.Render("new gsd task list"),
			"",
			lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("filename"),
			m.newForm.gsdFilename.View(),
			"",
			lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("tasks (one per line)"),
			m.newForm.gsdTasks.View(),
			errorLine,
			newDialogHelpStyle.Render("enter submit . tab next field . esc cancel"),
		)
		dialog := dialogStyle.Width(70).Render(content)
		return m.renderWithBackdrop(dialog)

	case newDialogTypeYolo:
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			dialogHeaderStyle.Render("yolo mode"),
			"",
			lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("filename"),
			m.newForm.planFilename.View(),
			"",
			lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("title"),
			m.newForm.planTitle.View(),
			"",
			lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("content"),
			m.newForm.planContent.View(),
			errorLine,
			newDialogHelpStyle.Render("enter submit . tab next field . esc cancel"),
		)
		dialog := dialogStyle.Width(70).Render(content)
		return m.renderWithBackdrop(dialog)
	default:
		return m.renderWithBackdrop("")
	}
}

func (m *Model) startNewDialogForm(dialogType string) tea.Cmd {
	m.newDialogStage = newDialogStageForm
	m.newDialogType = dialogType
	m.newForm = newFormModelFor(dialogType)
	if m.newForm == nil {
		m.closeNewDialog()
		return nil
	}
	m.updateKeyStates()
	return m.newForm.focusCurrentField()
}

func formatCreateError(kind string, err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("failed to create %s: %v", kind, err)
}
