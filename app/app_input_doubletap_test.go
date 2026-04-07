package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureTimeout replaces scheduleDoubleTapTimeout with a synchronous stub that
// records the most recently scheduled timeout message. Restores the original via
// t.Cleanup. The returned getter returns the most-recently recorded message.
func captureTimeout(t *testing.T) (func() doubleTapTimeoutMsg, func()) {
	t.Helper()
	var last doubleTapTimeoutMsg
	orig := scheduleDoubleTapTimeout
	scheduleDoubleTapTimeout = func(_ time.Duration, key string, seq int) tea.Cmd {
		last = doubleTapTimeoutMsg{key: key, seq: seq}
		// Return a Cmd that delivers the recorded message synchronously.
		return func() tea.Msg { return last }
	}
	restore := func() { scheduleDoubleTapTimeout = orig }
	t.Cleanup(restore)
	return func() doubleTapTimeoutMsg { return last }, restore
}

// -- Conflict-free double-tap tests (k, K, u, d) ------------------------------------

// TestHandleKeyPressDoubleTap_KK_DispatchesKill verifies that k+k triggers the kill
// path (returns an async kill command), and that a single k is silently swallowed.
// Note: KeyKill is a direct async kill (no confirmation dialog); KeyAbort uses a
// confirm overlay. Only the presence of a non-nil cmd confirms routing succeeded.
func TestHandleKeyPressDoubleTap_KK_DispatchesKill(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault
	inst, err := newTestInstance("agent-1")
	require.NoError(t, err)
	inst.MarkStartedForTest()
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	// First k: swallowed — nil cmd, no state change.
	h.keySent = true
	result, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m := result.(*home)
	assert.Nil(t, cmd, "first tap must be swallowed (nil cmd)")
	assert.Equal(t, stateDefault, m.state, "state must not change after first k")

	// Second k: dispatch KeyKill — non-nil async kill cmd returned.
	m.keySent = true
	result, cmd = m.handleKeyPress(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m = result.(*home)
	assert.NotNil(t, cmd, "second k must dispatch the async kill command")
	// KeyKill is fire-and-forget async; state stays stateDefault (no confirm dialog).
	assert.Equal(t, stateDefault, m.state, "state stays stateDefault after k+k (async kill)")
}

// TestHandleKeyPressDoubleTap_SingleK_IsNoOp verifies a lone k press does nothing.
func TestHandleKeyPressDoubleTap_SingleK_IsNoOp(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault
	inst, err := newTestInstance("agent-1")
	require.NoError(t, err)
	inst.MarkStartedForTest()
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m := result.(*home)
	assert.Equal(t, stateDefault, m.state, "single k must be a no-op")
	assert.False(t, m.overlays.IsActive())
}

// TestHandleKeyPressDoubleTap_KK_Abort verifies K+K triggers the abort confirmation.
func TestHandleKeyPressDoubleTap_KK_Abort(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault
	inst, err := newTestInstance("agent-1")
	require.NoError(t, err)
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	// First K: swallowed.
	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'K', Text: "K"})
	m := result.(*home)
	assert.Equal(t, stateDefault, m.state, "first K should be swallowed")

	// Second K: abort confirmation.
	m.keySent = true
	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: 'K', Text: "K"})
	m = result.(*home)
	assert.Equal(t, stateConfirm, m.state, "K+K should trigger abort confirmation")
}

// TestHandleKeyPressDoubleTap_UU_HalfPageUp verifies u+u routes through KeyHalfPageUp
// without panicking (scroll with no content is a safe no-op).
func TestHandleKeyPressDoubleTap_UU_HalfPageUp(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault

	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'u', Text: "u"})
	m := result.(*home)
	assert.Equal(t, stateDefault, m.state, "first u swallowed")

	m.keySent = true
	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: 'u', Text: "u"})
	m = result.(*home)
	assert.Equal(t, stateDefault, m.state, "u+u should stay in stateDefault")
}

// TestHandleKeyPressDoubleTap_DD_HalfPageDown verifies d+d routes through
// KeyHalfPageDown without panicking.
func TestHandleKeyPressDoubleTap_DD_HalfPageDown(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault

	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m := result.(*home)
	assert.Equal(t, stateDefault, m.state, "first d swallowed")

	m.keySent = true
	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = result.(*home)
	assert.Equal(t, stateDefault, m.state, "d+d should stay in stateDefault")
}

// TestHandleKeyPressDoubleTap_DifferentKeyBetweenK_NoFalsePositive verifies that
// pressing an unrelated key between two k presses resets the conflict-free tracker
// so the third k does not accidentally trigger kill.
func TestHandleKeyPressDoubleTap_DifferentKeyBetweenK_NoFalsePositive(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault
	inst, err := newTestInstance("agent-1")
	require.NoError(t, err)
	inst.MarkStartedForTest()
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	// First k (first tap, swallowed).
	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m := result.(*home)

	// Unrelated key 'a' — resets the conflict-free tracker.
	m.keySent = true
	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = result.(*home)

	// Second k after 'a' — should be a fresh first tap, not a double-tap.
	m.keySent = true
	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m = result.(*home)
	assert.Equal(t, stateDefault, m.state, "k after k+a should be a fresh first tap, not kill")
	assert.False(t, m.overlays.IsActive(), "no confirmation overlay: sequence was reset")
}

// -- Debounced double-tap tests (s, space) -------------------------------------------

// TestHandleKeyPressDoubleTap_SS_DispatchesSidebarToggle verifies that s+s dispatches
// KeyToggleSidebar (m.sidebarHidden toggles) and suppresses quick-launch.
func TestHandleKeyPressDoubleTap_SS_DispatchesSidebarToggle(t *testing.T) {
	_, restore := captureTimeout(t)
	defer restore()

	h := newTestHome()
	h.state = stateDefault

	// First s: pending set, timeout scheduled, no sidebar toggle yet.
	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"})
	m := result.(*home)
	assert.Equal(t, "s", m.pendingDoubleTapKey, "pending should be 's' after first tap")
	assert.False(t, m.sidebarHidden, "sidebar must not toggle on the first tap")

	// Second s: double-tap confirmed — dispatch KeyToggleSidebar.
	m.keySent = true
	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = result.(*home)
	assert.Equal(t, "", m.pendingDoubleTapKey, "pending must be cleared after double-tap")
	assert.True(t, m.sidebarHidden, "sidebar should be hidden after s+s")
}

// TestHandleKeyPressDoubleTap_SS_SuppressesQuickLaunch verifies s+s does not create
// a new instance (quick-launch is suppressed in favour of sidebar toggle).
func TestHandleKeyPressDoubleTap_SS_SuppressesQuickLaunch(t *testing.T) {
	_, restore := captureTimeout(t)
	defer restore()

	h := newTestHome()
	h.state = stateDefault
	initialCount := h.nav.NumInstances()

	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"})
	m := result.(*home)

	m.keySent = true
	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = result.(*home)

	assert.True(t, m.sidebarHidden, "sidebar must toggle after s+s")
	assert.Equal(t, initialCount, m.nav.NumInstances(), "quick-launch must be suppressed on s+s")
}

// TestHandleKeyPressDoubleTap_S_TimeoutDispatchesQuickLaunch verifies that a single
// s press followed by the debounce timeout dispatches KeyQuickLaunch (creates an
// instance) and clears the pending state.
func TestHandleKeyPressDoubleTap_S_TimeoutDispatchesQuickLaunch(t *testing.T) {
	getTimeout, restore := captureTimeout(t)
	defer restore()

	h := newTestHome()
	h.state = stateDefault

	// Press s: sets pending + schedules timeout (no action yet).
	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"})
	m := result.(*home)
	require.Equal(t, "s", m.pendingDoubleTapKey, "pending must be set after first s")

	// Fire the captured timeout: dispatches KeyQuickLaunch → instance created.
	timeoutMsg := getTimeout()
	result, _ = m.Update(timeoutMsg)
	m = result.(*home)

	assert.Equal(t, "", m.pendingDoubleTapKey, "pending must be cleared after timeout")
	assert.Greater(t, m.nav.NumInstances(), 0, "quick-launch should have created an instance")
}

// TestHandleKeyPressDoubleTap_S_ResendDoesNotFalseTrigger drives the actual
// handleMenuHighlighting resend path to ensure the re-sent 's' does NOT
// count as a second tap and accidentally trigger the sidebar toggle.
func TestHandleKeyPressDoubleTap_S_ResendDoesNotFalseTrigger(t *testing.T) {
	_, restore := captureTimeout(t)
	defer restore()

	h := newTestHome()
	h.state = stateDefault
	// h.keySent is intentionally left false to drive the menu-highlight resend path.

	// First call: handleMenuHighlighting fires, sets keySent, re-sends msg.
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"})
	m := result.(*home)
	require.True(t, m.keySent, "keySent must be set on the first pass for resend")
	require.Equal(t, "", m.pendingDoubleTapKey, "pending must NOT be set before the resend")
	assert.False(t, m.sidebarHidden, "no sidebar toggle before resend completes")

	// Second call (simulates the auto-resend): keySent=true → clears it → first-tap processing.
	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = result.(*home)
	assert.False(t, m.keySent, "keySent must be cleared after the resend pass")
	assert.Equal(t, "s", m.pendingDoubleTapKey, "first real tap: pending must now be set")
	assert.False(t, m.sidebarHidden, "sidebar must NOT toggle on the first tap alone")
}

// TestHandleKeyPressDoubleTap_SpaceSpace_DispatchesExitFocus verifies that ␣+␣ in
// stateDefault dispatches the same KeyExitFocus handler path as ctrl+space.
// With no running instance the handler is a no-op, but the route is verified.
func TestHandleKeyPressDoubleTap_SpaceSpace_DispatchesExitFocus(t *testing.T) {
	_, restore := captureTimeout(t)
	defer restore()

	h := newTestHome()
	h.state = stateDefault

	// First space: pending 'space' set, timeout scheduled.
	h.keySent = true
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m := result.(*home)
	assert.Equal(t, "space", m.pendingDoubleTapKey, "pending should be 'space' after first tap")

	// Second space: double-tap → dispatches KeyExitFocus.
	m.keySent = true
	result, _ = m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = result.(*home)
	assert.Equal(t, "", m.pendingDoubleTapKey, "pending must be cleared after double-space")
	// KeyExitFocus with no running instance is a safe no-op: stateDefault preserved.
	assert.Equal(t, stateDefault, m.state, "stateDefault preserved when no running instance")
}

// TestHandleKeyPressDoubleTap_SpaceSpace_FocusAgentStateUnaffected verifies that
// plain space in stateFocusAgent is forwarded to the PTY path and never touches
// the double-tap pending state (the stateFocusAgent block returns early).
func TestHandleKeyPressDoubleTap_SpaceSpace_FocusAgentStateUnaffected(t *testing.T) {
	_, restore := captureTimeout(t)
	defer restore()

	h := newTestHome()
	inst, err := newTestInstance("agent-1")
	require.NoError(t, err)
	inst.MarkStartedForTest()
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.state = stateFocusAgent

	// Space in stateFocusAgent: early return in the stateFocusAgent block.
	// The double-tap logic must never run.
	result, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m := result.(*home)
	assert.Equal(t, "", m.pendingDoubleTapKey, "double-tap pending must not be set in stateFocusAgent")
	assert.Equal(t, stateFocusAgent, m.state, "state must remain stateFocusAgent")
}

// -- Stale timeout tests -------------------------------------------------------------

// TestHandleKeyPressDoubleTap_StaleTimeout_WrongSeqIgnored verifies a timeout whose
// sequence number does not match the current pending seq is silently dropped.
func TestHandleKeyPressDoubleTap_StaleTimeout_WrongSeqIgnored(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault
	h.pendingDoubleTapKey = "s"
	h.pendingDoubleTapAction = keys.KeyQuickLaunch
	h.pendingDoubleTapSeq = 2

	stale := doubleTapTimeoutMsg{key: "s", seq: 1} // seq mismatch
	result, cmd := h.Update(stale)
	m := result.(*home)
	assert.Nil(t, cmd, "stale timeout (wrong seq) must produce no cmd")
	assert.Equal(t, "s", m.pendingDoubleTapKey, "pending must remain after stale timeout")
}

// TestHandleKeyPressDoubleTap_StaleTimeout_WrongKeyIgnored verifies a timeout for a
// different key than the one currently pending is silently dropped.
func TestHandleKeyPressDoubleTap_StaleTimeout_WrongKeyIgnored(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault
	h.pendingDoubleTapKey = "s"
	h.pendingDoubleTapAction = keys.KeyQuickLaunch
	h.pendingDoubleTapSeq = 1

	stale := doubleTapTimeoutMsg{key: "space", seq: 1} // key mismatch
	result, cmd := h.Update(stale)
	m := result.(*home)
	assert.Nil(t, cmd, "stale timeout (wrong key) must produce no cmd")
	assert.Equal(t, "s", m.pendingDoubleTapKey, "pending must remain after wrong-key timeout")
}

// TestHandleKeyPressDoubleTap_StaleTimeout_EmptyPendingIgnored verifies a timeout
// arriving with no pending state is silently dropped.
func TestHandleKeyPressDoubleTap_StaleTimeout_EmptyPendingIgnored(t *testing.T) {
	h := newTestHome()
	h.state = stateDefault
	// No pending state.

	stale := doubleTapTimeoutMsg{key: "s", seq: 99}
	result, cmd := h.Update(stale)
	m := result.(*home)
	assert.Nil(t, cmd, "timeout with empty pending state must produce no cmd")
	assert.Equal(t, stateDefault, m.state)
}

// TestHandleKeyPressDoubleTap_StaleTimeout_NonDefaultStateClearsPending verifies
// that a timeout arriving while the model is not in stateDefault clears the
// pending state so it doesn't linger and fire unexpectedly on the next key press.
func TestHandleKeyPressDoubleTap_StaleTimeout_NonDefaultStateClearsPending(t *testing.T) {
	h := newTestHome()
	h.state = stateConfirm // not stateDefault
	h.pendingDoubleTapKey = "s"
	h.pendingDoubleTapAction = keys.KeyQuickLaunch
	h.pendingDoubleTapSeq = 1

	msg := doubleTapTimeoutMsg{key: "s", seq: 1}
	result, cmd := h.Update(msg)
	m := result.(*home)
	assert.Nil(t, cmd, "timeout in non-default state must not dispatch")
	assert.Empty(t, m.pendingDoubleTapKey, "pending must be cleared to avoid stale fire")
	assert.Zero(t, m.pendingDoubleTapAction, "pending action must be cleared")
}

// -- Helper: canonicalDoubleTapKey ---------------------------------------------------

func TestCanonicalDoubleTapKey(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.KeyPressMsg
		want string
	}{
		{"plain s", tea.KeyPressMsg{Code: 's', Text: "s"}, "s"},
		{"plain k", tea.KeyPressMsg{Code: 'k', Text: "k"}, "k"},
		{"uppercase K (shift+k)", tea.KeyPressMsg{Code: 'K', Text: "K", Mod: tea.ModShift}, "K"},
		{"space", tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, "space"},
		{"ctrl+k", tea.KeyPressMsg{Code: 'k', Text: "k", Mod: tea.ModCtrl}, ""},
		{"alt+s", tea.KeyPressMsg{Code: 's', Text: "s", Mod: tea.ModAlt}, ""},
		{"shift+space (launcher)", tea.KeyPressMsg{Code: tea.KeySpace, Text: " ", Mod: tea.ModShift}, "space"},
		{"enter (no text)", tea.KeyPressMsg{Code: tea.KeyEnter}, ""},
		{"up arrow (no text)", tea.KeyPressMsg{Code: tea.KeyUp}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalDoubleTapKey(tc.msg)
			assert.Equal(t, tc.want, got)
		})
	}
}
