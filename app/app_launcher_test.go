package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// launcherHasAction reports whether any item in the list carries the given action.
func launcherHasAction(items []overlay.LauncherItem, action string) bool {
	for _, it := range items {
		if it.Action == action {
			return true
		}
	}
	return false
}

func TestShiftSpaceOpensCommandLauncher(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault

	// Shift+space should open the launcher.
	// handleKeyPress requires keySent=false to not trigger menu highlighting.
	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Text: " ", Mod: tea.ModShift})
	m := result.(*home)
	assert.Equal(t, stateLauncher, m.state)
	assert.True(t, m.overlays.IsActive())
	_, ok := m.overlays.Current().(*overlay.CommandLauncherOverlay)
	require.True(t, ok, "expected CommandLauncherOverlay")
}

func TestSpaceDoesNotOpenCommandLauncher(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault

	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m := result.(*home)
	assert.Equal(t, stateDefault, m.state)
	assert.False(t, m.overlays.IsActive())
}

func TestQuestionMarkOpensKeybindBrowser(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault

	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: '?', Text: "?"})
	m := result.(*home)
	assert.Equal(t, stateKeybindBrowser, m.state)
	assert.True(t, m.overlays.IsActive())
	_, ok := m.overlays.Current().(*overlay.CommandLauncherOverlay)
	require.True(t, ok, "expected CommandLauncherOverlay for keybind browser")
}

func TestLauncherEscReturnToDefault(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault

	// Open the launcher
	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Text: " ", Mod: tea.ModShift})
	m := result.(*home)
	require.Equal(t, stateLauncher, m.state)

	// Press Esc to dismiss
	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = result.(*home)
	assert.Equal(t, stateDefault, m.state)
	assert.False(t, m.overlays.IsActive())
}

func TestLauncherViewKeybindsOpensKeybindBrowser(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault

	// Open the launcher
	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Text: " ", Mod: tea.ModShift})
	m := result.(*home)
	require.Equal(t, stateLauncher, m.state)

	// Select "view keybinds" (first item)
	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(*home)
	assert.Equal(t, stateKeybindBrowser, m.state)
	assert.True(t, m.overlays.IsActive())
}

func TestKeybindBrowserEscReturnToDefault(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault

	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: '?', Text: "?"})
	m := result.(*home)
	require.Equal(t, stateKeybindBrowser, m.state)

	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = result.(*home)
	assert.Equal(t, stateDefault, m.state)
	assert.False(t, m.overlays.IsActive())
}

func TestBuildKeybindBrowserItems_HidesSubmitNameAndRemovedBindings(t *testing.T) {
	items := buildKeybindBrowserItems()

	var foundInfoTab bool
	for _, item := range items {
		assert.NotEqual(t, "submit name", item.Label)
		assert.NotEqual(t, "checkout", item.Label)
		assert.NotEqual(t, "right sidebar", item.Label)
		if item.Label == "toggle info header" {
			foundInfoTab = true
			assert.Equal(t, "I", item.Hint)
		}
	}

	require.True(t, foundInfoTab, "toggle info header should still be listed in the keybind browser")
}

// TestBuildLauncherItems_NoSelection_OnlyGlobalActions verifies that when nothing
// is selected in the nav panel, buildLauncherItems returns only the global items
// and none of the selection-dependent instance or task actions.
func TestBuildLauncherItems_NoSelection_OnlyGlobalActions(t *testing.T) {
	h := newTestHome()
	// No plans, no instances — nothing selected.

	items := h.buildLauncherItems()

	// Must include every global action.
	for _, want := range []string{
		"view_keybinds", "new_plan", "new_instance", "spawn_agent",
		"quick_launch", "search", "tmux_browser", "toggle_sidebar",
		"toggle_audit", "audit_cursor", "info_tab", "quit",
	} {
		assert.True(t, launcherHasAction(items, want), "global action %q must be present", want)
	}

	// Must NOT include instance-only or task-only actions.
	for _, absent := range []string{
		"kill_instance", "restart_instance", "open_instance", "resume_instance",
		"send_yes", "start_implement", "view_plan",
	} {
		assert.False(t, launcherHasAction(items, absent), "selection-dependent action %q must be absent with no selection", absent)
	}
}

// TestBuildLauncherItems_PlannedReadyTask verifies that when a planned-ready task
// is selected, start_implement appears and instance-only actions are absent.
func TestBuildLauncherItems_PlannedReadyTask(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store, ps, _ := newSharedStoreForTest(t, plansDir)
	const planFile = "my-feature.md"
	require.NoError(t, ps.Register(planFile, "my feature", "plan/my-feature", time.Now()))
	// Force the task to planned-ready status (StatusReady + Phase=planned).
	require.NoError(t, ps.ForceSetLifecycle(planFile, taskstate.StatusReady, taskstore.ExecutionState{Phase: taskstate.ManualOverridePlanned}))

	h := newLifecycleE2EHome(t, dir, plansDir, store, ps, newPlanFSMForTestWithStore(t, store, plansDir))
	h.updateSidebarTasks()

	// Select the plan header row in the nav panel.
	h.nav.SetPlans([]ui.PlanDisplay{{Filename: planFile, Status: "ready", Phase: "planned"}})
	h.nav.ClickItem(0)
	require.Equal(t, planFile, h.nav.GetSelectedPlanFile(), "plan must be selected")

	items := h.buildLauncherItems()

	assert.True(t, launcherHasAction(items, "start_implement"), "start_implement must be present for planned-ready task")
	// Instance-only actions must be absent when a plan (not an instance) is selected.
	assert.False(t, launcherHasAction(items, "kill_instance"), "kill_instance must be absent for task selection")
	assert.False(t, launcherHasAction(items, "restart_instance"), "restart_instance must be absent for task selection")
	assert.False(t, launcherHasAction(items, "send_yes"), "send_yes must be absent for task selection")
	// Global actions must still be present.
	assert.True(t, launcherHasAction(items, "new_plan"), "global new_plan must still be present")
}

// TestBuildLauncherItems_RunningInstance verifies that when a started, non-paused
// instance is selected, kill_instance and restart_instance are present.
// open_instance requires TmuxAlive() which MarkStartedForTest does not provide,
// so it must be absent in this scenario.
func TestBuildLauncherItems_RunningInstance(t *testing.T) {
	h := newTestHome()
	inst, err := newTestInstance("running-agent")
	require.NoError(t, err)
	inst.MarkStartedForTest() // simulate a running session (no real tmux session)
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	items := h.buildLauncherItems()

	// open_instance requires TmuxAlive(); MarkStartedForTest does not create a real
	// tmux session, so open_instance must be absent here.
	assert.False(t, launcherHasAction(items, "open_instance"), "open_instance must be absent when TmuxAlive is false")
	assert.True(t, launcherHasAction(items, "kill_instance"), "kill_instance must be present for running instance")
	assert.True(t, launcherHasAction(items, "restart_instance"), "restart_instance must be present for running instance")
	// resume_instance must be absent (not paused).
	assert.False(t, launcherHasAction(items, "resume_instance"), "resume_instance must be absent for non-paused instance")
	// send_yes must be absent (PromptDetected is false by default).
	assert.False(t, launcherHasAction(items, "send_yes"), "send_yes must be absent when PromptDetected is false")
}

// TestBuildLauncherItems_PausedInstance verifies that resume_instance is present
// and open_instance is absent when the selected instance is paused.
func TestBuildLauncherItems_PausedInstance(t *testing.T) {
	h := newTestHome()
	inst, err := newTestInstance("paused-agent")
	require.NoError(t, err)
	inst.SetStatus(session.Paused)
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	items := h.buildLauncherItems()

	assert.True(t, launcherHasAction(items, "resume_instance"), "resume_instance must be present for paused instance")
	assert.False(t, launcherHasAction(items, "open_instance"), "open_instance must be absent for paused instance")
	assert.True(t, launcherHasAction(items, "kill_instance"), "kill_instance must always be present")
	assert.True(t, launcherHasAction(items, "restart_instance"), "restart_instance must always be present")
}

// TestBuildKeybindBrowserItems_DoubleTapHints verifies that GlobalkeyBindings
// now carries double-tap hint text for every action that gained an alternative
// key sequence.  buildKeybindBrowserItems surfaces these via the Hint field so
// users can discover them via the keybind browser (?).
func TestBuildKeybindBrowserItems_DoubleTapHints(t *testing.T) {
	items := buildKeybindBrowserItems()

	// Build a label → hint map for convenient assertions.
	hints := make(map[string]string)
	for _, item := range items {
		hints[item.Label] = item.Hint
	}

	cases := []struct {
		label   string
		wantSub string // substring expected in the Hint field
	}{
		{"kill", "k+k"},
		{"abort", "K+K"},
		{"toggle sidebar", "s+s"},
		{"exit focus", "␣+␣"},
		{"half-page up", "u+u"},
		{"half-page down", "d+d"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			hint, ok := hints[tc.label]
			require.True(t, ok, "binding %q must appear in keybind browser", tc.label)
			assert.Contains(t, hint, tc.wantSub, "hint for %q must include double-tap key %q", tc.label, tc.wantSub)
		})
	}
}

// TestBuildLauncherItems_PromptDetected verifies that send_yes appears when the
// selected instance is started, not paused, and has PromptDetected set.
func TestBuildLauncherItems_PromptDetected(t *testing.T) {
	h := newTestHome()
	inst, err := newTestInstance("prompt-agent")
	require.NoError(t, err)
	inst.MarkStartedForTest()
	inst.PromptDetected = true
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	items := h.buildLauncherItems()

	assert.True(t, launcherHasAction(items, "send_yes"), "send_yes must be present when PromptDetected is true")
}
