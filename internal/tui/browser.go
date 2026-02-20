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
	return m.renderWithBackdrop(dialogStyle.Width(70).Render("feature browser (loading...)"))
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
