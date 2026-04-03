package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleKeyPress_DownKeyAlwaysFocusesNav(t *testing.T) {
	// Up/Down always refocus the sidebar and navigate it, regardless of which
	// pane was previously focused (when no document/scroll mode is active).
	for _, slot := range []int{slotAgent} {
		t.Run(fmt.Sprintf("from slot %d", slot), func(t *testing.T) {
			spin := spinner.New(spinner.WithSpinner(spinner.Dot))
			h := &home{
				ctx:          context.Background(),
				state:        stateDefault,
				appConfig:    config.DefaultConfig(),
				nav:          ui.NewNavigationPanel(&spin),
				menu:         ui.NewMenu(),
				tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
				focusSlot:    slot,
				keySent:      true,
			}

			model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyDown})
			homeModel, ok := model.(*home)
			require.True(t, ok)
			assert.Equal(t, slotNav, homeModel.focusSlot, "Down must focus nav")
		})
	}
}

func TestHandleKeyPress_UpKeyAlwaysFocusesNav(t *testing.T) {
	for _, slot := range []int{slotAgent} {
		t.Run(fmt.Sprintf("from slot %d", slot), func(t *testing.T) {
			spin := spinner.New(spinner.WithSpinner(spinner.Dot))
			h := &home{
				ctx:          context.Background(),
				state:        stateDefault,
				appConfig:    config.DefaultConfig(),
				nav:          ui.NewNavigationPanel(&spin),
				menu:         ui.NewMenu(),
				tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
				focusSlot:    slot,
				keySent:      true,
			}

			model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyUp})
			homeModel, ok := model.(*home)
			require.True(t, ok)
			assert.Equal(t, slotNav, homeModel.focusSlot, "Up must focus nav")
		})
	}
}

func appTestDocumentLines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		if i > 1 {
			b.WriteByte('\n')
		}
		_, _ = fmt.Fprintf(&b, "line %d", i)
	}
	return b.String()
}

func TestHandleMouseWheel_DocumentModeScrollsWithoutSelectedInstance(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&spin),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		keySent:      true,
	}

	h.tabbedWindow.SetSize(100, 16)
	h.tabbedWindow.SetActiveTab(ui.PreviewTab)
	h.tabbedWindow.SetDocumentContent(appTestDocumentLines(120))

	before := h.tabbedWindow.String()

	model, cmd := h.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	require.Equal(t, h, model)
	assert.Nil(t, cmd)

	after := h.tabbedWindow.String()
	assert.NotEqual(t, before, after, "mouse wheel should scroll plan document in preview tab")
}

func TestHandleMouseWheel_FocusModeForwardsToEmbeddedTerminal(t *testing.T) {
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "focus-wheel",
		Path:    t.TempDir(),
		Program: "opencode",
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.tabbedWindow.SetInstance(inst)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.Title
	h.state = stateFocusAgent
	h.tabbedWindow.SetFocusMode(true)
	h.menu.SetFocusMode(true)
	h.termWidth = 120
	h.termHeight = 40
	h.navWidth = 36
	h.tabsWidth = 84
	h.contentHeight = 38
	h.toastManager.SetSize(h.termWidth, h.termHeight)
	h.nav.SetSize(h.navWidth, h.contentHeight)
	h.tabbedWindow.SetSize(h.tabsWidth, h.contentHeight)
	h.menu.SetSize(h.termWidth, 1)
	t.Cleanup(func() { zone.Clear(ui.ZoneAgentPane) })
	_ = safeZoneScan(h.tabbedWindow.String())

	require.Eventually(t, func() bool {
		return !zone.Get(ui.ZoneAgentPane).IsZero()
	}, time.Second, 10*time.Millisecond, "agent pane zone must be registered after rendering")
	agentPane := zone.Get(ui.ZoneAgentPane)

	msg := tea.MouseWheelMsg{
		X:      agentPane.StartX + 2,
		Y:      agentPane.StartY + 2,
		Button: tea.MouseWheelDown,
	}

	model, cmd := h.handleMouseWheel(msg)
	require.Nil(t, cmd)
	updated := model.(*home)

	require.Len(t, updated.previewTerminal.SentKeys(), 1)
	assert.Equal(t,
		[]byte(ansi.MouseSgr(ansi.EncodeMouseButton(ansi.MouseWheelDown, false, false, false, false), 1, 2, false)),
		updated.previewTerminal.SentKeys()[0],
	)
	assert.False(t, updated.tabbedWindow.IsPreviewInScrollMode(),
		"focus-mode wheel input should be forwarded to the embedded terminal instead of entering kasmos scroll mode")
	assert.Equal(t, stateFocusAgent, updated.state,
		"forwarding wheel input should keep focus mode active")
}
