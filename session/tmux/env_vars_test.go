package tmux

import (
	"fmt"
	cmd2 "github.com/kastheco/kasmos/cmd"
	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/kastheco/kasmos/log"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartTmuxSession_WithTaskEnvVars(t *testing.T) {
	log.Initialize(false)
	defer log.Close()

	ptyFactory := NewMockPtyFactory(t)
	created := false
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("session already exists")
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			if strings.Contains(cmd.String(), "capture-pane") {
				return []byte("Do you trust the files in this folder?"), nil
			}
			return []byte("output"), nil
		},
	}

	workdir := t.TempDir()
	session := newTmuxSession("test-task", "claude", false, ptyFactory, cmdExec)
	session.SetTaskEnv(3, 2, 4)

	err := session.Start(workdir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(ptyFactory.cmds), 1)

	cmdStr := cmd2.ToString(ptyFactory.cmds[0])
	assert.Contains(t, cmdStr, "KASMOS_TASK=3")
	assert.Contains(t, cmdStr, "KASMOS_WAVE=2")
	assert.Contains(t, cmdStr, "KASMOS_PEERS=4")
	assert.Contains(t, cmdStr, "KASMOS_MANAGED=1")
}

func TestStartTmuxSession_WithoutTaskEnvVars(t *testing.T) {
	log.Initialize(false)
	defer log.Close()

	ptyFactory := NewMockPtyFactory(t)
	created := false
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("session already exists")
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			if strings.Contains(cmd.String(), "capture-pane") {
				return []byte("Do you trust the files in this folder?"), nil
			}
			return []byte("output"), nil
		},
	}

	workdir := t.TempDir()
	session := newTmuxSession("test-solo", "claude", false, ptyFactory, cmdExec)

	err := session.Start(workdir)
	require.NoError(t, err)

	cmdStr := cmd2.ToString(ptyFactory.cmds[0])
	assert.NotContains(t, cmdStr, "KASMOS_TASK=")
	assert.NotContains(t, cmdStr, "KASMOS_WAVE=")
	assert.NotContains(t, cmdStr, "KASMOS_PEERS=")
	assert.Contains(t, cmdStr, "KASMOS_MANAGED=1")
}
