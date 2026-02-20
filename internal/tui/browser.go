package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

type FeaturePhase int

const (
	PhaseSpecOnly FeaturePhase = iota
	PhasePlanReady
	PhaseTasksReady
)

func (p FeaturePhase) String() string {
	switch p {
	case PhaseSpecOnly:
		return "spec only"
	case PhasePlanReady:
		return "plan ready"
	case PhaseTasksReady:
		return "tasks ready"
	default:
		return "unknown"
	}
}

func phaseBadge(phase FeaturePhase, wpCount int) string {
	switch phase {
	case PhaseSpecOnly:
		return lipgloss.NewStyle().Foreground(colorMidGray).Render("spec only")
	case PhasePlanReady:
		return lipgloss.NewStyle().Foreground(colorLightBlue).Render("plan ready")
	case PhaseTasksReady:
		return lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("tasks ready (%d WPs)", wpCount))
	default:
		return lipgloss.NewStyle().Foreground(colorMidGray).Render(phase.String())
	}
}

type FeatureEntry struct {
	Number  string
	Slug    string
	Dir     string
	Phase   FeaturePhase
	WPCount int
}

type lifecycleAction struct {
	label       string
	description string
	role        string
	promptFmt   string
}

func actionsForPhase(phase FeaturePhase) []lifecycleAction {
	switch phase {
	case PhaseSpecOnly:
		return []lifecycleAction{
			{
				label:       "clarify",
				description: "run /spec-kitty.clarify",
				role:        "planner",
				promptFmt:   "Run /spec-kitty.clarify for feature %s",
			},
			{
				label:       "plan",
				description: "run /spec-kitty.plan",
				role:        "planner",
				promptFmt:   "Run /spec-kitty.plan for feature %s",
			},
		}
	case PhasePlanReady:
		return []lifecycleAction{
			{
				label:       "tasks",
				description: "run /spec-kitty.tasks",
				role:        "planner",
				promptFmt:   "Run /spec-kitty.tasks for feature %s",
			},
		}
	case PhaseTasksReady:
		return nil
	default:
		return nil
	}
}

func parseFeatureDir(name string) (number, slug string) {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) == 1 {
		return name, ""
	}
	return parts[0], parts[1]
}

func scanFeatures() ([]FeatureEntry, error) {
	specFiles, err := filepath.Glob(filepath.Join("kitty-specs", "*", "spec.md"))
	if err != nil {
		return nil, fmt.Errorf("scan features: %w", err)
	}

	entries := make([]FeatureEntry, 0, len(specFiles))
	for _, specFile := range specFiles {
		featureDir := filepath.Dir(specFile)
		dirName := filepath.Base(featureDir)
		number, slug := parseFeatureDir(dirName)
		phase, wpCount := detectPhase(featureDir)

		entries = append(entries, FeatureEntry{
			Number:  number,
			Slug:    slug,
			Dir:     featureDir,
			Phase:   phase,
			WPCount: wpCount,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Number > entries[j].Number
	})

	return entries, nil
}

func detectPhase(featureDir string) (FeaturePhase, int) {
	planPath := filepath.Join(featureDir, "plan.md")
	_, err := os.Stat(planPath)
	planExists := err == nil

	wpFiles, err := filepath.Glob(filepath.Join(featureDir, "tasks", "WP*.md"))
	if err == nil && len(wpFiles) > 0 {
		return PhaseTasksReady, len(wpFiles)
	}

	if planExists {
		return PhasePlanReady, 0
	}

	return PhaseSpecOnly, 0
}

func filterFeatures(entries []FeatureEntry, query string) []int {
	if query == "" {
		indices := make([]int, len(entries))
		for i := range entries {
			indices[i] = i
		}
		return indices
	}

	lower := strings.ToLower(query)
	indices := make([]int, 0, len(entries))
	for i, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Slug), lower) {
			indices = append(indices, i)
		}
	}

	return indices
}

func (m *Model) openFeatureBrowser() tea.Cmd {
	if err := ensureSpecKittyAvailable(); err != nil {
		m.launcherNote = err.Error()
		return nil
	}

	entries, err := scanFeatures()
	if err != nil {
		m.launcherNote = fmt.Sprintf("failed to scan features: %v", err)
		return nil
	}

	m.showFeatureBrowser = true
	m.featureEntries = entries
	m.featureFiltered = filterFeatures(entries, "")
	m.featureSelectedIdx = 0
	m.featureActionsOpen = false
	m.featureActionIdx = 0
	m.featureFilterActive = false
	m.featureFilter = styledTextInput()
	m.featureFilter.Placeholder = "filter features..."
	m.featureFilter.SetWidth(40)
	m.launcherNote = ""
	m.updateKeyStates()
	return nil
}

func (m *Model) closeFeatureBrowser() {
	m.showFeatureBrowser = false
	m.featureEntries = nil
	m.featureFiltered = nil
	m.featureSelectedIdx = 0
	m.featureActionsOpen = false
	m.featureActionIdx = 0
	m.featureFilterActive = false
	m.featureFilter = textinput.Model{}
	m.updateKeyStates()
}

func (m *Model) renderFeatureBrowser() string {
	if len(m.featureEntries) == 0 {
		return m.renderFeatureBrowserEmpty()
	}

	lines := m.renderFeatureList()

	filterLine := ""
	if m.featureFilterActive || m.featureFilter.Value() != "" {
		filterLine = m.renderBrowserFilter()
	}

	helpText := m.browserHelpText()

	parts := []string{
		dialogHeaderStyle.Render("browse features"),
		"",
	}
	parts = append(parts, strings.Join(lines, "\n"))
	parts = append(parts, "")
	if filterLine != "" {
		parts = append(parts, filterLine)
	}
	parts = append(parts, helpText)

	content := strings.Join(parts, "\n")
	dialog := dialogStyle.Width(min(76, m.width-4)).Render(content)
	return m.renderWithBackdrop(dialog)
}

func (m *Model) browserHelpText() string {
	if m.featureFilterActive {
		return lipgloss.NewStyle().Foreground(colorMidGray).Render(
			"type to filter  enter confirm  esc clear")
	}
	if m.featureActionsOpen {
		return lipgloss.NewStyle().Foreground(colorMidGray).Render(
			"j/k navigate  enter/right select  esc/left back")
	}
	return lipgloss.NewStyle().Foreground(colorMidGray).Render(
		"j/k navigate  enter/right select  / filter  esc back")
}

func (m *Model) renderFeatureList() []string {
	lines := make([]string, 0, len(m.featureFiltered)*2)

	for listIdx, entryIdx := range m.featureFiltered {
		entry := m.featureEntries[entryIdx]
		isSelected := listIdx == m.featureSelectedIdx

		line := m.renderFeatureEntry(entry, isSelected)
		lines = append(lines, line)

		if isSelected && m.featureActionsOpen {
			actionLines := m.renderActionLines(entry)
			lines = append(lines, actionLines...)
		}
	}

	return lines
}

func (m *Model) renderFeatureEntry(entry FeatureEntry, selected bool) string {
	selector := "  "
	if selected && !m.featureActionsOpen {
		selector = "> "
	} else if selected && m.featureActionsOpen {
		selector = "  "
	}

	numStyle := lipgloss.NewStyle().Foreground(colorMidGray)
	slugStyle := lipgloss.NewStyle().Foreground(colorLightGray)
	if selected {
		numStyle = numStyle.Foreground(colorCream)
		slugStyle = slugStyle.Foreground(colorCream).Bold(true)
	}

	badge := phaseBadge(entry.Phase, entry.WPCount)
	return fmt.Sprintf("%s%s  %s   %s",
		selector,
		numStyle.Render(entry.Number),
		slugStyle.Render(entry.Slug),
		badge,
	)
}

func (m *Model) renderActionLines(entry FeatureEntry) []string {
	actions := actionsForPhase(entry.Phase)
	if len(actions) == 0 {
		return nil
	}

	treeStyle := lipgloss.NewStyle().Foreground(colorMidGray)
	actionStyle := lipgloss.NewStyle().Foreground(colorLightGray)
	descStyle := lipgloss.NewStyle().Foreground(colorMidGray)
	selectedActionStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	selectedDescStyle := lipgloss.NewStyle().Foreground(colorCream)

	lines := make([]string, 0, len(actions))
	for i, action := range actions {
		isLast := i == len(actions)-1
		isActionSelected := i == m.featureActionIdx

		treeChar := "|--"
		if isLast {
			treeChar = "'--"
		}

		selector := " "
		aStyle := actionStyle
		dStyle := descStyle
		if isActionSelected {
			selector = ">"
			aStyle = selectedActionStyle
			dStyle = selectedDescStyle
		}

		line := fmt.Sprintf("    %s %s %-10s %s",
			treeStyle.Render(treeChar),
			selector,
			aStyle.Render(action.label),
			dStyle.Render(action.description),
		)
		lines = append(lines, line)
	}

	return lines
}

func (m *Model) renderBrowserFilter() string {
	prefix := lipgloss.NewStyle().Foreground(colorPurple).Render("/")
	return prefix + " " + m.featureFilter.View()
}

func (m *Model) renderFeatureBrowserEmpty() string {
	msg := lipgloss.NewStyle().Foreground(colorMidGray).Render(
		"no spec-kitty features found")
	hint := lipgloss.NewStyle().Foreground(colorLightGray).Render(
		"press f to create a new feature, or esc to go back")
	helpText := lipgloss.NewStyle().Foreground(colorMidGray).Render("esc back")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		dialogHeaderStyle.Render("browse features"),
		"",
		msg,
		hint,
		"",
		helpText,
	)

	dialog := dialogStyle.Width(min(76, m.width-4)).Render(content)
	return m.renderWithBackdrop(dialog)
}

func (m *Model) updateFeatureBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, m.keys.Back) {
			m.closeFeatureBrowser()
			return m, nil
		}
	}

	return m, nil
}
