package app

import (
	"fmt"
	"os/exec"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/session/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleKeyPress_ResumeOnNonPausedInstanceIsNoOp(t *testing.T) {
	for _, status := range []session.Status{session.Running, session.Ready} {
		t.Run(fmt.Sprintf("status-%d", status), func(t *testing.T) {
			h := newTestHomeWithToast()
			inst, err := newTestInstance(fmt.Sprintf("resume-noop-%d", status))
			require.NoError(t, err)
			inst.Status = status
			inst.MarkStartedForTest()
			_ = h.nav.AddInstance(inst)
			h.nav.SelectInstance(inst)

			h.keySent = true
			_, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'r', Text: "r"})

			assert.Nil(t, cmd, "resume key should no-op on %s instance", status)
			assert.Equal(t, status, inst.Status, "status should not change")
		})
	}
}

func TestHandleKeyPress_ResumeOnNilSelectedIsNoOp(t *testing.T) {
	h := newTestHomeWithToast()

	h.keySent = true
	_, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'r', Text: "r"})

	assert.Nil(t, cmd, "resume key should no-op when no instance selected")
}

func TestExecuteLauncherAction_ResumeOnNonPausedInstanceIsNoOp(t *testing.T) {
	for _, status := range []session.Status{session.Running, session.Ready} {
		t.Run(fmt.Sprintf("status-%d", status), func(t *testing.T) {
			h := newTestHomeWithToast()
			inst, err := newTestInstance(fmt.Sprintf("launcher-noop-%d", status))
			require.NoError(t, err)
			inst.Status = status
			inst.MarkStartedForTest()
			_ = h.nav.AddInstance(inst)
			h.nav.SelectInstance(inst)

			_, cmd := h.executeLauncherAction("resume")

			assert.Nil(t, cmd, "launcher resume should no-op on %s instance", status)
			assert.Equal(t, status, inst.Status, "status should not change")
		})
	}
}

func TestExecuteLauncherAction_ResumeOnNilSelectedIsNoOp(t *testing.T) {
	h := newTestHomeWithToast()

	_, cmd := h.executeLauncherAction("resume")

	assert.Nil(t, cmd, "launcher resume should no-op when no instance selected")
}

func TestHandleKeyPress_ResumePausedInstanceCallsResume(t *testing.T) {
	// Verify that the key handler dispatches to Resume() for a paused instance.
	// Resume() will fail because we don't have a real tmux, but the key should
	// produce an error-handling command (not nil), proving the guard passed.
	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(_ *exec.Cmd) error { return fmt.Errorf("no tmux") },
		OutputFunc: func(_ *exec.Cmd) ([]byte, error) { return nil, fmt.Errorf("no tmux") },
	}

	h := newTestHomeWithToast()
	inst, err := newTestInstance("resume-paused")
	require.NoError(t, err)
	inst.Status = session.Paused
	inst.MarkStartedForTest()
	// Main-branch path: nil gitWorktree, mock tmux session that always fails
	inst.SetTmuxSession(tmux.NewTmuxSessionWithDeps("resume-paused", "opencode", false, &noopPtyFactory{}, cmdExec))
	_ = h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	h.keySent = true
	_, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'r', Text: "r"})

	// Resume() was called (evidenced by the error-handling cmd), proving the
	// Paused guard allows dispatch. The cmd handles the error toast.
	assert.NotNil(t, cmd, "resume key on paused instance should dispatch to Resume and return error cmd")
}
