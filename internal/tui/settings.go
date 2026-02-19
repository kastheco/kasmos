package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/user/kasmos/internal/config"
	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/worker"
)

const settingsSaveDir = "."

var (
	settingsRoles          = []string{"planner", "coder", "reviewer", "release"}
	settingsTaskSources    = []string{"spec-kitty", "gsd", "yolo"}
	settingsReasoningModes = []string{"default", "low", "medium", "high"}
)

type settingsRowKind int

const (
	settingsRowTaskSource settingsRowKind = iota
	settingsRowRoleModel
	settingsRowRoleReasoning
)

type settingsRow struct {
	kind settingsRowKind
	role string
}

type settingsModel struct {
	rows       []settingsRow
	selected   int
	modelInput map[string]textinput.Model
	saving     bool
	saveErr    error
}

func (m *Model) ensureConfigDefaults() {
	if m.config == nil {
		m.config = config.DefaultConfig()
	}

	def := config.DefaultConfig()
	if m.config.Agents == nil {
		m.config.Agents = make(map[string]config.AgentConfig, len(def.Agents))
	}

	for _, role := range settingsRoles {
		cfg, ok := m.config.Agents[role]
		if !ok {
			m.config.Agents[role] = def.Agents[role]
			continue
		}
		if strings.TrimSpace(cfg.Model) == "" {
			cfg.Model = def.Agents[role].Model
		}
		cfg.Reasoning = normalizeReasoning(cfg.Reasoning)
		m.config.Agents[role] = cfg
	}

	if !slices.Contains(settingsTaskSources, m.config.DefaultTaskSource) {
		m.config.DefaultTaskSource = def.DefaultTaskSource
	}
}

func (m *Model) openSettingsView() tea.Cmd {
	m.ensureConfigDefaults()
	m.showSettings = true
	m.settingsForm = newSettingsModel(m.config)
	m.updateKeyStates()
	return m.settingsForm.focusSelected()
}

func (m *Model) closeSettingsView() {
	m.showSettings = false
	m.settingsForm = nil
	m.updateKeyStates()
}

func newSettingsModel(cfg *config.Config) *settingsModel {
	form := &settingsModel{
		rows:       make([]settingsRow, 0, 1+len(settingsRoles)*2),
		modelInput: make(map[string]textinput.Model, len(settingsRoles)),
	}

	form.rows = append(form.rows, settingsRow{kind: settingsRowTaskSource})
	for _, role := range settingsRoles {
		form.rows = append(form.rows,
			settingsRow{kind: settingsRowRoleModel, role: role},
			settingsRow{kind: settingsRowRoleReasoning, role: role},
		)

		input := styledTextInput()
		input.SetWidth(32)
		input.SetValue(cfg.Agents[role].Model)
		input.Blur()
		form.modelInput[role] = input
	}

	return form
}

func (s *settingsModel) selectedRow() settingsRow {
	if s == nil || len(s.rows) == 0 {
		return settingsRow{}
	}
	idx := s.selected
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s.rows) {
		idx = len(s.rows) - 1
	}
	return s.rows[idx]
}

func (s *settingsModel) focusSelected() tea.Cmd {
	if s == nil {
		return nil
	}

	for role, input := range s.modelInput {
		input.Blur()
		s.modelInput[role] = input
	}

	row := s.selectedRow()
	if row.kind != settingsRowRoleModel {
		return nil
	}

	input := s.modelInput[row.role]
	cmd := input.Focus()
	s.modelInput[row.role] = input
	return cmd
}

func (s *settingsModel) move(delta int) tea.Cmd {
	if s == nil || len(s.rows) == 0 {
		return nil
	}
	s.selected = (s.selected + delta + len(s.rows)) % len(s.rows)
	return s.focusSelected()
}

func (m *Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.settingsForm == nil {
		m.closeSettingsView()
		return m, nil
	}

	if saveMsg, ok := msg.(settingsSavedMsg); ok {
		m.settingsForm.saving = false
		m.settingsForm.saveErr = saveMsg.Err
		if saveMsg.Err == nil {
			m.closeSettingsView()
		}
		return m, nil
	}

	if m.settingsForm.saving {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if key.Matches(keyMsg, m.keys.Back) {
		if err := m.applySettingsFromForm(); err != nil {
			m.settingsForm.saveErr = err
			return m, nil
		}
		m.settingsForm.saving = true
		m.settingsForm.saveErr = nil
		return m, settingsSaveCmd(cloneConfig(m.config), settingsSaveDir)
	}

	if key.Matches(keyMsg, m.keys.Up) {
		return m, m.settingsForm.move(-1)
	}
	if key.Matches(keyMsg, m.keys.Down) {
		return m, m.settingsForm.move(1)
	}

	row := m.settingsForm.selectedRow()
	switch row.kind {
	case settingsRowTaskSource:
		switch keyMsg.String() {
		case "left", "h":
			m.cycleTaskSourceSetting(-1)
		case "right", "l", "enter", " ":
			m.cycleTaskSourceSetting(1)
		}
		return m, nil

	case settingsRowRoleReasoning:
		switch keyMsg.String() {
		case "left", "h":
			m.cycleReasoningSetting(row.role, -1)
		case "right", "l", "enter", " ":
			m.cycleReasoningSetting(row.role, 1)
		}
		return m, nil

	case settingsRowRoleModel:
		input := m.settingsForm.modelInput[row.role]
		var cmd tea.Cmd
		input, cmd = input.Update(msg)
		m.settingsForm.modelInput[row.role] = input
		return m, cmd
	}

	return m, nil
}

func (m *Model) applySettingsFromForm() error {
	if m.settingsForm == nil {
		return fmt.Errorf("settings form is not initialized")
	}

	for _, role := range settingsRoles {
		agent := m.config.Agents[role]
		model := strings.TrimSpace(m.settingsForm.modelInput[role].Value())
		if model == "" {
			return fmt.Errorf("%s model cannot be empty", role)
		}
		agent.Model = model
		agent.Reasoning = normalizeReasoning(agent.Reasoning)
		m.config.Agents[role] = agent
	}

	if !slices.Contains(settingsTaskSources, m.config.DefaultTaskSource) {
		return fmt.Errorf("default task source %q is invalid", m.config.DefaultTaskSource)
	}

	return nil
}

func (m *Model) cycleTaskSourceSetting(delta int) {
	current := m.config.DefaultTaskSource
	idx := slices.Index(settingsTaskSources, current)
	if idx < 0 {
		idx = 0
	}
	idx = (idx + delta + len(settingsTaskSources)) % len(settingsTaskSources)
	m.config.DefaultTaskSource = settingsTaskSources[idx]
	m.settingsForm.saveErr = nil
}

func (m *Model) cycleReasoningSetting(role string, delta int) {
	agent := m.config.Agents[role]
	idx := slices.Index(settingsReasoningModes, normalizeReasoning(agent.Reasoning))
	if idx < 0 {
		idx = 0
	}
	idx = (idx + delta + len(settingsReasoningModes)) % len(settingsReasoningModes)
	agent.Reasoning = settingsReasoningModes[idx]
	m.config.Agents[role] = agent
	m.settingsForm.saveErr = nil
}

func normalizeReasoning(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	if slices.Contains(settingsReasoningModes, v) {
		return v
	}
	return "default"
}

func cloneConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return nil
	}

	clone := &config.Config{
		DefaultTaskSource: cfg.DefaultTaskSource,
		Agents:            make(map[string]config.AgentConfig, len(cfg.Agents)),
	}
	for role, agent := range cfg.Agents {
		clone.Agents[role] = agent
	}

	return clone
}

func (m *Model) agentConfig(role string) config.AgentConfig {
	m.ensureConfigDefaults()
	agent := m.config.Agents[role]
	agent.Model = strings.TrimSpace(agent.Model)
	agent.Reasoning = normalizeReasoning(agent.Reasoning)
	return agent
}

func (m *Model) roleSpawnConfig(base worker.SpawnConfig) worker.SpawnConfig {
	agent := m.agentConfig(base.Role)
	base.Model = agent.Model
	base.Reasoning = agent.Reasoning
	return base
}

func (m *Model) defaultTaskSource() task.Source {
	m.ensureConfigDefaults()

	switch m.config.DefaultTaskSource {
	case "spec-kitty":
		if source := task.AutoDetectSpecKitty(); source != nil {
			return source
		}
	case "gsd":
		if source := task.AutoDetectGSD(); source != nil {
			return source
		}
	}

	return &task.YoloSource{}
}

func (m *Model) renderSettingsView() string {
	if m.settingsForm == nil {
		return m.renderWithBackdrop("")
	}

	lines := []string{
		dialogHeaderStyle.Render("settings"),
		"",
		lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("default task source"),
	}

	for i, row := range m.settingsForm.rows {
		selector := "  "
		if i == m.settingsForm.selected {
			selector = "> "
		}

		lineStyle := lipgloss.NewStyle().Foreground(colorLightGray)
		if i == m.settingsForm.selected {
			lineStyle = lineStyle.Foreground(colorCream).Bold(true)
		}

		switch row.kind {
		case settingsRowTaskSource:
			lines = append(lines, lineStyle.Render(fmt.Sprintf("%sdefault source: %s", selector, m.config.DefaultTaskSource)))
			lines = append(lines, "")
			lines = append(lines, lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render("agent roles"))

		case settingsRowRoleModel:
			input := m.settingsForm.modelInput[row.role]
			lines = append(lines, lineStyle.Render(fmt.Sprintf("%s%s model:", selector, row.role)), "  "+input.View())

		case settingsRowRoleReasoning:
			reasoning := normalizeReasoning(m.config.Agents[row.role].Reasoning)
			lines = append(lines, lineStyle.Render(fmt.Sprintf("%s%s reasoning: %s", selector, row.role, reasoning)))
		}
	}

	helpText := "up/down row  type model text  left/right cycle  esc save + return"
	if m.settingsForm.saving {
		helpText = fmt.Sprintf("%s saving settings...", m.spinner.View())
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(colorMidGray).Render(helpText))

	if m.settingsForm.saveErr != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorOrange).Render("error: "+m.settingsForm.saveErr.Error()))
	}

	dialog := dialogStyle.Width(74).Render(strings.Join(lines, "\n"))
	return m.renderWithBackdrop(dialog)
}
