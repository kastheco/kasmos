package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
