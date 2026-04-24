package app

import (
	"testing"
	"time"

	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/require"
)

func TestQuickLaunch_TitleSyncCmdReturnsConversationTitle(t *testing.T) {
	// serial: mutates app timing globals
	withFastAppTimings(t)
	h := newTestHome()

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "agent-1",
		Path:    t.TempDir(),
		Program: "opencode",
	})
	require.NoError(t, err)

	originalRead := readQuickLaunchSessionTitle
	readQuickLaunchSessionTitle = func(workDir string, afterTime time.Time) (string, error) {
		return "Ship Auth UI", nil
	}
	t.Cleanup(func() {
		readQuickLaunchSessionTitle = originalRead
	})

	cmd := h.quickLaunchTitleSyncCmd(inst)
	require.NotNil(t, cmd)

	msg := cmd()
	require.IsType(t, instanceTitleSyncMsg{}, msg)
	require.Equal(t, "Ship Auth UI", msg.(instanceTitleSyncMsg).newTitle)
	require.Same(t, inst, msg.(instanceTitleSyncMsg).instance)
}
