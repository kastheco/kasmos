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

	"github.com/user/kasmos/internal/task"
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

// updateFeatureBrowser is the top-level dispatcher for the feature browser overlay.
// It routes messages based on the current browser sub-state:
//   - Filter mode captures all messages (textinput needs non-key msgs too)
//   - Back/Esc is always available regardless of sub-state
//   - Actions mode handles lifecycle sub-menu navigation
//   - List mode handles feature navigation, selection, and filter activation
func (m *Model) updateFeatureBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle filter mode first: textinput needs all message types, not just key events.
	if m.featureFilterActive {
		return m.updateBrowserFilter(msg)
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	// Back/Esc is always available regardless of sub-state.
	if key.Matches(keyMsg, m.keys.Back) || keyMsg.String() == "left" {
		return m.handleBrowserBack()
	}

	// Actions sub-menu mode: j/k/enter navigate the lifecycle action list.
	if m.featureActionsOpen {
		return m.updateBrowserActions(keyMsg)
	}

	// Normal list navigation mode.
	return m.updateBrowserList(keyMsg)
}

// updateBrowserList handles key events in the main feature list.
// j/k navigate the filtered list, enter/right selects, / activates filter,
// and f opens the new-feature dialog when the list is empty.
func (m *Model) updateBrowserList(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "j", "down":
		if m.featureSelectedIdx < len(m.featureFiltered)-1 {
			m.featureSelectedIdx++
		}
		return m, nil

	case "k", "up":
		if m.featureSelectedIdx > 0 {
			m.featureSelectedIdx--
		}
		return m, nil

	case "enter", "right":
		return m.handleFeatureSelect()

	case "/":
		return m.activateBrowserFilter()

	case "f":
		// US4: when the browser is empty (no features exist), f opens the
		// new-feature dialog as a shortcut to start the spec-kitty lifecycle.
		if len(m.featureFiltered) == 0 {
			m.closeFeatureBrowser()
			m.transitionFromLauncher()
			_ = m.openNewDialog()
			return m, m.startNewDialogForm(newDialogTypeFeatureSpec)
		}
		return m, nil

	default:
		return m, nil
	}
}

// handleFeatureSelect routes the Enter/right action based on the selected feature's phase.
// Tasks-ready features load the dashboard directly; non-ready features expand the
// lifecycle sub-menu so the user can advance the feature through the spec-kitty pipeline.
func (m *Model) handleFeatureSelect() (tea.Model, tea.Cmd) {
	if len(m.featureFiltered) == 0 || m.featureSelectedIdx >= len(m.featureFiltered) {
		return m, nil
	}

	entryIdx := m.featureFiltered[m.featureSelectedIdx]
	entry := m.featureEntries[entryIdx]

	if entry.Phase == PhaseTasksReady {
		// Direct dashboard load: detect the task source and swap it in.
		source, err := task.DetectSourceType(entry.Dir)
		if err != nil {
			m.launcherNote = fmt.Sprintf("failed to load %s: %v", entry.Dir, err)
			m.closeFeatureBrowser()
			return m, nil
		}
		m.closeFeatureBrowser()
		m.swapTaskSource(source)
		m.transitionFromLauncher()
		return m, nil
	}

	// Non-ready feature: expand the lifecycle sub-menu.
	actions := actionsForPhase(entry.Phase)
	if len(actions) == 0 {
		// Defensive: no actions defined for this phase, do nothing.
		return m, nil
	}
	m.featureActionsOpen = true
	m.featureActionIdx = 0
	return m, nil
}

// updateBrowserActions handles key events inside the expanded lifecycle sub-menu.
// j/k navigate between actions; enter/right spawns a worker for the selected action.
func (m *Model) updateBrowserActions(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.featureFiltered) == 0 || m.featureSelectedIdx >= len(m.featureFiltered) {
		return m, nil
	}

	entryIdx := m.featureFiltered[m.featureSelectedIdx]
	entry := m.featureEntries[entryIdx]
	actions := actionsForPhase(entry.Phase)

	switch keyMsg.String() {
	case "j", "down":
		if m.featureActionIdx < len(actions)-1 {
			m.featureActionIdx++
		}
		return m, nil

	case "k", "up":
		if m.featureActionIdx > 0 {
			m.featureActionIdx--
		}
		return m, nil

	case "enter", "right":
		if m.featureActionIdx >= len(actions) {
			return m, nil
		}
		action := actions[m.featureActionIdx]
		prompt := fmt.Sprintf(action.promptFmt, entry.Dir)

		m.closeFeatureBrowser()
		m.transitionFromLauncher()
		return m, m.openSpawnDialogWithPrefill(action.role, prompt, nil)

	default:
		return m, nil
	}
}

// activateBrowserFilter switches the browser into filter mode, focusing the textinput
// and collapsing any expanded action sub-menu.
func (m *Model) activateBrowserFilter() (tea.Model, tea.Cmd) {
	m.featureFilterActive = true
	m.featureActionsOpen = false // collapse any expanded sub-menu
	m.featureActionIdx = 0
	return m, m.featureFilter.Focus()
}

// updateBrowserFilter handles all messages while the filter textinput is active.
// Enter confirms the filter (keeps text, returns to nav mode).
// Esc clears the filter and restores the full list.
// All other messages are forwarded to the textinput, which recomputes the filtered list.
func (m *Model) updateBrowserFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			// Confirm filter: keep the current filter text, return to navigation mode.
			m.featureFilterActive = false
			m.featureFilter.Blur()
			return m, nil

		case "esc":
			// Clear filter: empty the textinput, restore the full list, return to nav mode.
			m.featureFilterActive = false
			m.featureFilter.SetValue("")
			m.featureFilter.Blur()
			m.featureFiltered = filterFeatures(m.featureEntries, "")
			m.featureSelectedIdx = 0
			return m, nil
		}
	}

	// Forward all other messages to the textinput so it can handle typing, deletion, etc.
	var cmd tea.Cmd
	m.featureFilter, cmd = m.featureFilter.Update(msg)

	// Recompute the filtered list on every change for real-time filtering.
	m.featureFiltered = filterFeatures(m.featureEntries, m.featureFilter.Value())

	// Clamp selection to prevent out-of-bounds access after the list shrinks.
	if m.featureSelectedIdx >= len(m.featureFiltered) {
		m.featureSelectedIdx = max(0, len(m.featureFiltered)-1)
	}

	return m, cmd
}

// handleBrowserBack implements context-dependent back navigation.
// From the action sub-menu: collapse to the feature list (same feature stays highlighted).
// From the feature list: close the browser and return to the launcher.
func (m *Model) handleBrowserBack() (tea.Model, tea.Cmd) {
	if m.featureActionsOpen {
		// Collapse sub-menu, return to feature list.
		m.featureActionsOpen = false
		m.featureActionIdx = 0
		return m, nil
	}

	// Close browser, return to launcher.
	m.closeFeatureBrowser()
	return m, nil
}
