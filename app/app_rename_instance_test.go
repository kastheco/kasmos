package app

import (
	"testing"

	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui/overlay"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type submittedOverlay struct {
	result overlay.Result
}

func (o *submittedOverlay) HandleKey(tea.KeyPressMsg) overlay.Result { return o.result }
func (o *submittedOverlay) View() string                             { return "" }
func (o *submittedOverlay) SetSize(_, _ int)                         {}

func TestRenameInstance_SubmitPreservesStableTitle(t *testing.T) {
	h := newTestHome()
	h.nav.SetSize(80, 20)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "agent-1",
		Path:    t.TempDir(),
		Program: "opencode",
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()

	h.nav.AddInstance(inst)()
	h.nav.SelectInstance(inst)
	h.allInstances = append(h.allInstances, inst)
	h.tabbedWindow.SetInstance(inst)
	h.previewTerminalInstance = inst.Title
	h.populateInstanceTabs()
	h.updateInfoPane()

	h.state = stateRenameInstance
	h.overlays.Show(&submittedOverlay{result: overlay.Result{
		Dismissed: true,
		Submitted: true,
		Value:     "Ship Auth UI",
	}})

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)

	require.NotNil(t, cmd)
	assert.Equal(t, stateDefault, updated.state)
	assert.False(t, updated.overlays.IsActive())
	assert.Equal(t, "agent-1", inst.Title)
	assert.Equal(t, "ship-auth-ui", inst.DisplayTitle)
	assert.Equal(t, "agent-1", updated.previewTerminalInstance)
	assert.Equal(t, "ship-auth-ui", updated.nav.GetSelectedInstance().DisplayName())
	assert.Equal(t, "ship-auth-ui", updated.tabbedWindow.GetInfoData().Title)
}
