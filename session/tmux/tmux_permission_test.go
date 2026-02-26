package tmux

import (
	"os/exec"
	"testing"

	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendPermissionResponse_AllowAlways(t *testing.T) {
	var ranCmds []string
	exec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, cmd.String())
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}
	session := NewTmuxSessionWithDeps("test", "opencode", false, &MockPtyFactory{}, exec)

	err := session.SendPermissionResponse(PermissionAllowAlways)
	require.NoError(t, err)

	// Should send: Right, Enter, Enter
	assert.GreaterOrEqual(t, len(ranCmds), 3)
}

func TestSendPermissionResponse_AllowOnce(t *testing.T) {
	var ranCmds []string
	exec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, cmd.String())
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}
	session := NewTmuxSessionWithDeps("test", "opencode", false, &MockPtyFactory{}, exec)

	err := session.SendPermissionResponse(PermissionAllowOnce)
	require.NoError(t, err)

	// Should send: Enter
	assert.GreaterOrEqual(t, len(ranCmds), 1)
}

func TestSendPermissionResponse_Reject(t *testing.T) {
	var ranCmds []string
	exec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, cmd.String())
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}
	session := NewTmuxSessionWithDeps("test", "opencode", false, &MockPtyFactory{}, exec)

	err := session.SendPermissionResponse(PermissionReject)
	require.NoError(t, err)

	// Should send: Right, Right, Enter
	assert.GreaterOrEqual(t, len(ranCmds), 3)
}
