package overlay

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func successInput() WaveDecisionInput {
	return WaveDecisionInput{
		PlanFile:   "myplan",
		PlanName:   "My Plan",
		WaveNumber: 1,
		TotalWaves: 2,
		Completed:  3,
		Failed:     0,
		Total:      3,
	}
}

func failureInput() WaveDecisionInput {
	return WaveDecisionInput{
		PlanFile:   "myplan",
		PlanName:   "My Plan",
		WaveNumber: 1,
		TotalWaves: 2,
		Completed:  2,
		Failed:     1,
		Total:      3,
	}
}

// TestWaveDecisionOverlay_InputAccessor verifies Input() returns the original input.
func TestWaveDecisionOverlay_InputAccessor(t *testing.T) {
	input := successInput()
	w := NewWaveDecisionOverlay(input)
	assert.Equal(t, input, w.Input())
}

// TestWaveDecisionOverlay_SuccessShortcutY verifies 'y' advances immediately on success.
func TestWaveDecisionOverlay_SuccessShortcutY(t *testing.T) {
	w := NewWaveDecisionOverlay(successInput())
	result := w.HandleKey(tea.KeyPressMsg{Code: 'y', Text: "y"})
	assert.True(t, result.Dismissed)
	assert.True(t, result.Submitted)
	assert.Equal(t, "advance", result.Action)
}

// TestWaveDecisionOverlay_FailureShortcutR verifies 'r' retries on failure.
func TestWaveDecisionOverlay_FailureShortcutR(t *testing.T) {
	w := NewWaveDecisionOverlay(failureInput())
	result := w.HandleKey(tea.KeyPressMsg{Code: 'r', Text: "r"})
	assert.True(t, result.Dismissed)
	assert.True(t, result.Submitted)
	assert.Equal(t, "retry", result.Action)
}

// TestWaveDecisionOverlay_FailureShortcutN verifies 'n' advances on failure.
func TestWaveDecisionOverlay_FailureShortcutN(t *testing.T) {
	w := NewWaveDecisionOverlay(failureInput())
	result := w.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	assert.True(t, result.Dismissed)
	assert.True(t, result.Submitted)
	assert.Equal(t, "advance", result.Action)
}

// TestWaveDecisionOverlay_FailureShortcutA verifies 'a' aborts on failure.
func TestWaveDecisionOverlay_FailureShortcutA(t *testing.T) {
	w := NewWaveDecisionOverlay(failureInput())
	result := w.HandleKey(tea.KeyPressMsg{Code: 'a', Text: "a"})
	assert.True(t, result.Dismissed)
	assert.True(t, result.Submitted)
	assert.Equal(t, "abort", result.Action)
}

// TestWaveDecisionOverlay_EscDismissesWithNoAction verifies esc dismisses without submitting.
func TestWaveDecisionOverlay_EscDismissesWithNoAction(t *testing.T) {
	for _, name := range []string{"success", "failure"} {
		t.Run(name, func(t *testing.T) {
			var w *WaveDecisionOverlay
			if name == "success" {
				w = NewWaveDecisionOverlay(successInput())
			} else {
				w = NewWaveDecisionOverlay(failureInput())
			}
			result := w.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
			assert.True(t, result.Dismissed)
			assert.False(t, result.Submitted)
			assert.Equal(t, "", result.Action)
		})
	}
}

// TestWaveDecisionOverlay_EnterConfirmsDefault verifies enter confirms the default button.
func TestWaveDecisionOverlay_EnterConfirmsDefault(t *testing.T) {
	// Success: default idx 0 = advance
	w := NewWaveDecisionOverlay(successInput())
	result := w.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, result.Dismissed)
	assert.True(t, result.Submitted)
	assert.Equal(t, "advance", result.Action)
}

// TestWaveDecisionOverlay_EnterConfirmsFailureDefault verifies enter confirms retry (idx 0) on failure.
func TestWaveDecisionOverlay_EnterConfirmsFailureDefault(t *testing.T) {
	w := NewWaveDecisionOverlay(failureInput())
	result := w.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, result.Dismissed)
	assert.True(t, result.Submitted)
	assert.Equal(t, "retry", result.Action)
}

// TestWaveDecisionOverlay_ArrowNavigationSuccess verifies up/down navigation on success overlay.
func TestWaveDecisionOverlay_ArrowNavigationSuccess(t *testing.T) {
	w := NewWaveDecisionOverlay(successInput())

	// Default idx 0 (advance). Down → idx 1 (cancel). Enter → dismiss only.
	result := w.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, Result{}, result, "arrow should return empty result")

	result = w.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, result.Dismissed)
	assert.False(t, result.Submitted, "cancel button should dismiss without submit")
}

// TestWaveDecisionOverlay_ArrowNavigationFailure verifies navigation through failure buttons.
func TestWaveDecisionOverlay_ArrowNavigationFailure(t *testing.T) {
	w := NewWaveDecisionOverlay(failureInput())

	// Default idx 0 (retry). Down → 1 (advance). Down → 2 (abort). Enter → abort.
	w.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	w.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	result := w.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, result.Dismissed)
	assert.True(t, result.Submitted)
	assert.Equal(t, "abort", result.Action)
}

// TestWaveDecisionOverlay_ArrowNavigationWraps verifies up/down clamps at boundaries.
func TestWaveDecisionOverlay_ArrowNavigationWraps(t *testing.T) {
	w := NewWaveDecisionOverlay(successInput())

	// Already at idx 0; Up should stay at 0.
	w.HandleKey(tea.KeyPressMsg{Code: tea.KeyUp})
	result := w.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, "advance", result.Action, "up at idx 0 should stay on advance")

	// Go to end and try Down again.
	w2 := NewWaveDecisionOverlay(successInput())
	w2.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	w2.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown}) // extra down, should clamp
	result2 := w2.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, result2.Submitted, "clamped-at-end button is still cancel (no submit)")
}

// TestWaveDecisionOverlay_SuccessViewText verifies the success view contains expected text.
func TestWaveDecisionOverlay_SuccessViewText(t *testing.T) {
	w := NewWaveDecisionOverlay(successInput())
	view := w.View()
	assert.True(t, len(view) > 0)
	// Plan name and wave info must appear
	assert.True(t, strings.Contains(view, "My Plan"), "view must contain plan name")
	assert.True(t, strings.Contains(view, "wave 1"), "view must contain wave number")
	assert.True(t, strings.Contains(view, "3/3"), "view must contain completed/total counts")
	assert.True(t, strings.Contains(view, "y"), "view must show 'y' shortcut hint")
}

// TestWaveDecisionOverlay_FailureViewText verifies the failure view contains expected text.
func TestWaveDecisionOverlay_FailureViewText(t *testing.T) {
	w := NewWaveDecisionOverlay(failureInput())
	view := w.View()
	assert.True(t, len(view) > 0)
	assert.True(t, strings.Contains(view, "My Plan"), "view must contain plan name")
	assert.True(t, strings.Contains(view, "wave 1"), "view must contain wave number")
	assert.True(t, strings.Contains(view, "r"), "view must show 'r' shortcut for retry")
	assert.True(t, strings.Contains(view, "n"), "view must show 'n' shortcut for next wave")
	assert.True(t, strings.Contains(view, "a"), "view must show 'a' shortcut for abort")
}

// TestWaveDecisionOverlay_SetSize verifies SetSize doesn't panic.
func TestWaveDecisionOverlay_SetSize(t *testing.T) {
	w := NewWaveDecisionOverlay(successInput())
	require.NotPanics(t, func() { w.SetSize(80, 24) })
	view := w.View()
	assert.True(t, len(view) > 0)
}

// TestWaveDecisionOverlay_SuccessYNotActiveOnFailure verifies 'y' does nothing in failure mode.
func TestWaveDecisionOverlay_SuccessYNotActiveOnFailure(t *testing.T) {
	w := NewWaveDecisionOverlay(failureInput())
	result := w.HandleKey(tea.KeyPressMsg{Code: 'y', Text: "y"})
	// 'y' is not a shortcut for failure overlay
	assert.Equal(t, Result{}, result)
}

// TestWaveDecisionOverlay_FailureShortcutsNotActiveOnSuccess verifies r/n/a do nothing on success overlay.
func TestWaveDecisionOverlay_FailureShortcutsNotActiveOnSuccess(t *testing.T) {
	w := NewWaveDecisionOverlay(successInput())
	for _, key := range []string{"r", "n", "a"} {
		result := w.HandleKey(tea.KeyPressMsg{Code: rune(key[0]), Text: key})
		assert.Equal(t, Result{}, result, "key %q should be no-op on success overlay", key)
	}
}
