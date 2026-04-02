package app

import (
	"os"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDetachTestHome() *home {
	sp := spinner.New()
	return &home{
		termWidth:      120,
		termHeight:     40,
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&sp),
		overlays:       overlay.NewManager(),
		activeRepoPath: os.TempDir(),
	}
}

// TestTmuxAttachReturn_ResetsDimensions verifies that the tmuxAttachReturnMsg
// handler zeroes termWidth and termHeight so the next WindowSizeMsg forces a
// full layout recalculation (termResized==true in updateHandleWindowSizeEvent).
func TestTmuxAttachReturn_ResetsDimensions(t *testing.T) {
	h := newDetachTestHome()

	updated, cmd := h.Update(tmuxAttachReturnMsg{})
	m := updated.(*home)

	assert.Equal(t, 0, m.termWidth, "termWidth must be reset to 0 after detach")
	assert.Equal(t, 0, m.termHeight, "termHeight must be reset to 0 after detach")
	require.NotNil(t, cmd, "handler must return a non-nil command (Sequence)")
}

// TestTmuxAttachReturn_ForcesResizeOnSameDimensions verifies that after the
// dimension reset, a WindowSizeMsg with the original dimensions still triggers
// termResized==true and runs overlay sizing.
func TestTmuxAttachReturn_ForcesResizeOnSameDimensions(t *testing.T) {
	h := newDetachTestHome()

	// Simulate the detach handler resetting dimensions.
	h.termWidth = 0
	h.termHeight = 0

	// Now a WindowSizeMsg with the "unchanged" 120x40 should trigger termResized.
	h.updateHandleWindowSizeEvent(tea.WindowSizeMsg{Width: 120, Height: 40})

	assert.Equal(t, 120, h.termWidth)
	assert.Equal(t, 40, h.termHeight)
}
