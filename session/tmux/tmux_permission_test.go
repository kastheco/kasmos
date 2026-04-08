package tmux

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendPermissionResponse_AllowAlways(t *testing.T) {
	var ranCmds []string
	exec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, strings.Join(cmd.Args, " "))
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}
	session := NewTmuxSessionWithDeps("test", "opencode", false, &MockPtyFactory{}, exec)

	err := session.SendPermissionResponse(PermissionAllowAlways)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"tmux send-keys -t kas_test Right",
		"tmux send-keys -t kas_test Enter",
		"tmux send-keys -t kas_test Enter",
	}, ranCmds)
}

func TestSendPermissionResponse_AllowOnce(t *testing.T) {
	var ranCmds []string
	exec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, strings.Join(cmd.Args, " "))
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}
	session := NewTmuxSessionWithDeps("test", "opencode", false, &MockPtyFactory{}, exec)

	err := session.SendPermissionResponse(PermissionAllowOnce)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"tmux send-keys -t kas_test Enter",
		"tmux send-keys -t kas_test Enter",
	}, ranCmds)
}

func TestSendPermissionResponse_Reject(t *testing.T) {
	var ranCmds []string
	exec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, strings.Join(cmd.Args, " "))
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}
	session := NewTmuxSessionWithDeps("test", "opencode", false, &MockPtyFactory{}, exec)

	err := session.SendPermissionResponse(PermissionReject)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"tmux send-keys -t kas_test Right",
		"tmux send-keys -t kas_test Right",
		"tmux send-keys -t kas_test Enter",
		"tmux send-keys -t kas_test Enter",
	}, ranCmds)
}

func TestSendPermissionResponse_ClaudeAllowOnce(t *testing.T) {
	var ranCmds []string
	exec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, strings.Join(cmd.Args, " "))
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}
	session := NewTmuxSessionWithDeps("test", "claude", false, &MockPtyFactory{}, exec)

	err := session.SendPermissionResponse(PermissionAllowOnce)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"tmux send-keys -l -t kas_test 1",
		"tmux send-keys -t kas_test Enter",
	}, ranCmds)
}

func TestSendPermissionResponse_ClaudeAllowAlways(t *testing.T) {
	var ranCmds []string
	exec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, strings.Join(cmd.Args, " "))
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}
	session := NewTmuxSessionWithDeps("test", "claude", false, &MockPtyFactory{}, exec)

	err := session.SendPermissionResponse(PermissionAllowAlways)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"tmux send-keys -l -t kas_test 1",
		"tmux send-keys -t kas_test Enter",
	}, ranCmds)
}

func TestSendPermissionResponse_ClaudeReject(t *testing.T) {
	var ranCmds []string
	exec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, strings.Join(cmd.Args, " "))
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}
	session := NewTmuxSessionWithDeps("test", "claude", false, &MockPtyFactory{}, exec)

	err := session.SendPermissionResponse(PermissionReject)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"tmux send-keys -t kas_test Escape",
	}, ranCmds)
}
