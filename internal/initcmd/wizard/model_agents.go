package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type agentMode int

const (
	agentBrowseMode agentMode = iota
	agentEditMode
)

type agentStepModel struct {
	agents     []AgentState
	cursor     int
	mode       agentMode
	harnesses  []string
	modelCache map[string][]string
}

func newAgentStep(agents []AgentState, harnesses []string, modelCache map[string][]string) *agentStepModel {
	agentCopy := append([]AgentState(nil), agents...)
	harnessCopy := append([]string(nil), harnesses...)
	cacheCopy := map[string][]string{}
	for name, models := range modelCache {
		cacheCopy[name] = append([]string(nil), models...)
	}

	return &agentStepModel{
		agents:     agentCopy,
		cursor:     0,
		mode:       agentBrowseMode,
		harnesses:  harnessCopy,
		modelCache: cacheCopy,
	}
}

func (m *agentStepModel) Init() tea.Cmd {
	return nil
}

func (m *agentStepModel) Update(msg tea.Msg) (stepModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		m.cursorUp()
	case "down", "j":
		m.cursorDown()
	case " ":
		m.toggleEnabled()
	case "enter":
		m.mode = agentEditMode
	case "tab":
		return m, func() tea.Msg { return stepDoneMsg{} }
	case "esc":
		return m, func() tea.Msg { return stepBackMsg{} }
	case "q":
		return m, tea.Quit
	}

	return m, nil
}

func (m *agentStepModel) View(width, height int) string {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	leftWidth := width / 3
	if leftWidth > 32 {
		leftWidth = 32
	}
	if leftWidth < 1 {
		leftWidth = 1
	}

	rightWidth := width - leftWidth - 1
	if rightWidth < 1 {
		rightWidth = 1
	}

	left := m.renderRolePanel(leftWidth, height)
	right := m.renderDetailPanel(rightWidth, height)
	separator := separatorStyle.Render("┊")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, separator, right)
}

func (m *agentStepModel) Apply(state *State) {
	state.Agents = append([]AgentState(nil), m.agents...)
}

func (m *agentStepModel) cursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *agentStepModel) cursorDown() {
	if m.cursor < m.maxNavigableIndex() {
		m.cursor++
	}
}

func (m *agentStepModel) maxNavigableIndex() int {
	max := len(m.agents) - 1
	if max > 2 {
		max = 2
	}
	if max < 0 {
		return 0
	}
	return max
}

func (m *agentStepModel) toggleEnabled() {
	if m.cursor < 0 || m.cursor >= len(m.agents) {
		return
	}
	m.agents[m.cursor].Enabled = !m.agents[m.cursor].Enabled
}

func (m *agentStepModel) renderRolePanel(width, height int) string {
	if width < 1 {
		width = 1
	}

	rows := []string{titleStyle.Render("ROLES"), ""}
	for i := 0; i <= m.maxNavigableIndex() && i < len(m.agents); i++ {
		agent := m.agents[i]

		dot := dotDisabledStyle.Render("○")
		if agent.Enabled {
			dot = dotEnabledStyle.Render("●")
		}

		prefix := " "
		lineStyle := roleNormalStyle
		harnessStyle := roleMutedStyle
		if i == m.cursor {
			prefix = roleActiveStyle.Render("›")
			lineStyle = roleActiveStyle
			harnessStyle = roleActiveStyle
		}

		line := fmt.Sprintf("%s %s %-8s %s", prefix, dot, agent.Role, harnessStyle.Render(agent.Harness))
		rows = append(rows, lineStyle.Render(line))
	}

	rows = append(rows, "", hintDescStyle.Render("j/k navigate · enter edit · space toggle · tab next step · q quit"))
	panel := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(width).Height(height).Render(panel)
}

func (m *agentStepModel) renderDetailPanel(width, height int) string {
	if width < 1 {
		width = 1
	}
	if m.cursor < 0 || m.cursor >= len(m.agents) {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}

	a := m.agents[m.cursor]
	temp := a.Temperature
	if temp == "" {
		temp = "default"
	}
	effort := a.Effort
	if effort == "" {
		effort = "default"
	}
	state := "disabled"
	if a.Enabled {
		state = "enabled"
	}

	lines := []string{
		titleStyle.Render(strings.ToUpper(a.Role)),
		subtitleStyle.Render(RoleDescription(a.Role)),
		"",
		fmt.Sprintf("%s %s", labelStyle.Render("harness:"), valueStyle.Render(a.Harness)),
		fmt.Sprintf("%s %s", labelStyle.Render("model:"), valueStyle.Render(a.Model)),
		fmt.Sprintf("%s %s", labelStyle.Render("effort:"), valueStyle.Render(effort)),
		fmt.Sprintf("%s %s", labelStyle.Render("temperature:"), valueStyle.Render(temp)),
		fmt.Sprintf("%s %s", labelStyle.Render("status:"), valueStyle.Render(state)),
		"",
		subtitleStyle.Render(RolePhaseText(a.Role)),
	}

	panel := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(width).Height(height).Render(panel)
}
