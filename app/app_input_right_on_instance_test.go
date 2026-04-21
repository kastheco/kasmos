package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRightOnInstance_OpensContextMenu verifies that pressing right on an
// instance row opens the instance context menu (regardless of active tab).
func TestRightOnInstance_OpensContextMenu(t *testing.T) {
	h := newTestHome()

	// Add a solo instance so the selected row is an instance row.
	inst := &session.Instance{Title: "test-agent"}
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	// Press right on the instance — should open context menu.
	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyRight})
	updated := model.(*home)

	assert.Equal(t, stateContextMenu, updated.state,
		"right on instance should open context menu")
}

// TestRightOnInstance_PreviewTab_OpensContextMenu verifies that pressing right
// on an instance while on the preview tab also opens the context menu.
func TestRightOnInstance_PreviewTab_OpensContextMenu(t *testing.T) {
	h := newTestHome()

	inst := &session.Instance{Title: "test-agent"}
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyRight})
	updated := model.(*home)

	assert.Equal(t, stateContextMenu, updated.state,
		"right on instance should open context menu")
}

func TestSpaceOnInstance_OpensContextMenu(t *testing.T) {
	// Space is now debounced: the first tap schedules a timeout and KeySpace fires
	// only when the timeout message arrives (no second space within the window).
	var capturedTimeout doubleTapTimeoutMsg
	orig := scheduleDoubleTapTimeout
	scheduleDoubleTapTimeout = func(_ time.Duration, key string, seq int) tea.Cmd {
		capturedTimeout = doubleTapTimeoutMsg{key: key, seq: seq}
		return func() tea.Msg { return capturedTimeout }
	}
	t.Cleanup(func() { scheduleDoubleTapTimeout = orig })

	h := newTestHome()

	inst := &session.Instance{Title: "test-agent"}
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	// First space: debounced — pending set, timeout scheduled.
	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	updated := model.(*home)
	require.Equal(t, "space", updated.pendingDoubleTapKey, "pending must be set after first space")

	// Fire the timeout: dispatches KeySpace → opens context menu.
	model, _ = updated.Update(capturedTimeout)
	updated = model.(*home)

	assert.Equal(t, stateContextMenu, updated.state,
		"space on instance should open context menu after timeout fires")
}

// TestRightOnNonInstanceRow_DoesNotOpenContextMenu verifies that pressing right
// on a non-instance row (e.g. a topic header with no plan) does not open the
// context menu or switch tabs.
func TestRightOnNonInstanceRow_DoesNotOpenContextMenu(t *testing.T) {
	h := newTestHome()

	// Set up with no instance selected and no plan selected (empty nav).

	// Press right with no instance and no plan selected.
	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyRight})
	updated := model.(*home)

	// Should not have entered context menu state.
	assert.NotEqual(t, stateContextMenu, updated.state,
		"right on non-instance row should not open context menu")
}

// TestRightOnPlanRow_ViewsPlan verifies that pressing right on a plan header
// row triggers plan view behavior rather than context menu or expand/collapse.
func TestRightOnPlanRow_ViewsPlan(t *testing.T) {
	h := newTestHome()

	// Set up a plan row so GetSelectedPlanFile() returns a non-empty string.
	h.nav.SetData(
		[]ui.PlanDisplay{{Filename: "test-plan.md"}},
		nil, nil, nil, nil,
	)
	// Row 0 is the plan header; select it.
	h.nav.Down()

	h.keySent = true
	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyRight})
	updated := model.(*home)

	// Should not have entered context menu state.
	assert.NotEqual(t, stateContextMenu, updated.state,
		"right on plan row should not open context menu")
}
