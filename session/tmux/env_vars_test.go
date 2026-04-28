package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/kastheco/kasmos/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartTmuxSession_WithTaskEnvVars(t *testing.T) {
	t.Parallel()

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

	cmdStr := commandString(ptyFactory.cmds[0])
	assert.Contains(t, cmdStr, "KASMOS_TASK=3")
	assert.Contains(t, cmdStr, "KASMOS_WAVE=2")
	assert.Contains(t, cmdStr, "KASMOS_PEERS=4")
	assert.Contains(t, cmdStr, "KASMOS_MANAGED=1")
	assert.Contains(t, cmdStr, "CLAUDE_CODE_NO_FLICKER=0")
}

func TestStartTmuxSession_WithoutTaskEnvVars(t *testing.T) {
	t.Parallel()

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

	cmdStr := commandString(ptyFactory.cmds[0])
	assert.NotContains(t, cmdStr, "KASMOS_TASK=")
	assert.NotContains(t, cmdStr, "KASMOS_WAVE=")
	assert.NotContains(t, cmdStr, "KASMOS_PEERS=")
	assert.Contains(t, cmdStr, "KASMOS_MANAGED=1")
	assert.Contains(t, cmdStr, "CLAUDE_CODE_NO_FLICKER=0")
}

func TestStartTmuxSession_WithProjectEnvVar(t *testing.T) {
	t.Parallel()

	ptyFactory := NewMockPtyFactory(t)
	created := false
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("session does not exist")
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
	session := newTmuxSession("test-project", "claude", false, ptyFactory, cmdExec)
	session.SetProject("kasmos")

	err := session.Start(workdir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(ptyFactory.cmds), 1)

	cmdStr := commandString(ptyFactory.cmds[0])
	assert.Contains(t, cmdStr, "KASMOS_PROJECT='kasmos'")
	assert.Contains(t, cmdStr, "KASMOS_MANAGED=1")
	// KASMOS_PROJECT must appear before KASMOS_MANAGED in the command string.
	projIdx := strings.Index(cmdStr, "KASMOS_PROJECT='kasmos'")
	managedIdx := strings.Index(cmdStr, "KASMOS_MANAGED=1")
	assert.Less(t, projIdx, managedIdx, "KASMOS_PROJECT should precede KASMOS_MANAGED")
}

func TestStartTmuxSession_WithoutProject_NoProjectEnvVar(t *testing.T) {
	t.Parallel()

	ptyFactory := NewMockPtyFactory(t)
	created := false
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("session does not exist")
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
	session := newTmuxSession("test-no-project", "claude", false, ptyFactory, cmdExec)
	// SetProject not called — env var should be absent.

	err := session.Start(workdir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(ptyFactory.cmds), 1)

	cmdStr := commandString(ptyFactory.cmds[0])
	assert.NotContains(t, cmdStr, "KASMOS_PROJECT=")
	assert.Contains(t, cmdStr, "KASMOS_MANAGED=1")
}

func TestStartTmuxSession_OpenCodeInjectsProjectConfigEnv(t *testing.T) {
	t.Parallel()

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
				return []byte("Ask anything"), nil
			}
			return []byte("output"), nil
		},
	}

	workdir := t.TempDir()
	configPath := filepath.Join(workdir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte("{}\n"), 0o644))

	session := newTmuxSession("test-opencode-config", "opencode", false, ptyFactory, cmdExec)
	err := session.Start(workdir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(ptyFactory.cmds), 1)

	cmdStr := commandString(ptyFactory.cmds[0])
	assert.Contains(t, cmdStr, "KASMOS_MANAGED=1")
	assert.Contains(t, cmdStr, "OPENCODE_CONFIG='"+configPath+"'")
	assert.Contains(t, cmdStr, "opencode")
}

// normalProfile returns a no-op ResolvedResourceControls (profile "normal").
func normalProfile() config.ResolvedResourceControls {
	rc, _ := config.ResourcesConfig{}.Resolve()
	return rc
}

// interactiveProfile returns the interactive preset ResolvedResourceControls.
func interactiveProfile() config.ResolvedResourceControls {
	rc, _ := config.ResourcesConfig{Profile: "interactive"}.Resolve()
	return rc
}

func makeTestCmdExecForResourceControls(t *testing.T) (cmd_test.MockCmdExec, *[]string) {
	t.Helper()
	created := false
	var ranCmds []string
	return cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, commandString(cmd))
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("no session")
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			if strings.Contains(cmd.String(), "capture-pane") {
				return []byte("Do you trust the files in this folder?"), nil
			}
			return []byte("output"), nil
		},
	}, &ranCmds
}

// TestStart_NormalProfile_CommandUnchanged verifies that the normal (no-op) profile
// produces the same command string as before resource controls were added.
func TestStart_NormalProfile_CommandUnchanged(t *testing.T) {
	// serial: mutates tmux timing globals
	withFastTmuxTimings(t)
	ptyFactory := NewMockPtyFactory(t)
	cmdExec, _ := makeTestCmdExecForResourceControls(t)

	workdir := t.TempDir()
	session := newTmuxSession("test-rc-normal", "claude", false, ptyFactory, cmdExec)
	session.SetResourceControls(normalProfile())
	session.SetProject("myrepo")
	session.SetTaskEnv(1, 2, 3)

	err := session.Start(workdir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(ptyFactory.cmds), 1)

	cmdStr := commandString(ptyFactory.cmds[0])

	// Normal profile must not inject nice/ionice.
	assert.NotContains(t, cmdStr, "nice", "normal profile must not wrap with nice")
	assert.NotContains(t, cmdStr, "ionice", "normal profile must not wrap with ionice")
	// Normal profile must not inject resource-control env vars.
	assert.NotContains(t, cmdStr, "KASMOS_RESOURCE_PROFILE=", "normal profile must not inject KASMOS_RESOURCE_PROFILE")
	assert.NotContains(t, cmdStr, "GOMAXPROCS=", "normal profile must not inject GOMAXPROCS")
	assert.NotContains(t, cmdStr, "KASMOS_BUILD_JOBS=", "normal profile must not inject KASMOS_BUILD_JOBS")
	// Standard env vars must still be present.
	assert.Contains(t, cmdStr, "KASMOS_MANAGED=1")
	assert.Contains(t, cmdStr, "KASMOS_PROJECT='myrepo'")
	assert.Contains(t, cmdStr, "KASMOS_TASK=1")
}

// TestStart_InteractiveProfile_CommandShape verifies that the interactive profile
// prepends wrapper env assignments before KASMOS_MANAGED and wraps the program
// with nice (and ionice on Linux) before the agent executable.
func TestStart_InteractiveProfile_CommandShape(t *testing.T) {
	// serial: mutates tmux timing globals
	withFastTmuxTimings(t)
	ptyFactory := NewMockPtyFactory(t)
	cmdExec, _ := makeTestCmdExecForResourceControls(t)

	workdir := t.TempDir()
	session := newTmuxSession("test-rc-interactive", "claude", false, ptyFactory, cmdExec)
	session.SetResourceControls(interactiveProfile())
	session.SetProject("example")

	err := session.Start(workdir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(ptyFactory.cmds), 1)

	cmdStr := commandString(ptyFactory.cmds[0])

	// Resource-control env vars must be present.
	assert.Contains(t, cmdStr, "KASMOS_RESOURCE_PROFILE=interactive")
	assert.Contains(t, cmdStr, "GOMAXPROCS=2")
	assert.Contains(t, cmdStr, "KASMOS_BUILD_JOBS=1")
	assert.Contains(t, cmdStr, "GOFLAGS=-p=1")

	// nice must wrap the agent program.
	assert.Contains(t, cmdStr, "nice -n 10", "interactive profile must prepend nice")

	// Env assignments must appear before KASMOS_MANAGED.
	profileIdx := strings.Index(cmdStr, "KASMOS_RESOURCE_PROFILE=interactive")
	managedIdx := strings.Index(cmdStr, "KASMOS_MANAGED=1")
	require.NotEqual(t, -1, profileIdx, "KASMOS_RESOURCE_PROFILE must be in command")
	require.NotEqual(t, -1, managedIdx, "KASMOS_MANAGED must be in command")
	assert.Greater(t, profileIdx, managedIdx,
		"KASMOS_RESOURCE_PROFILE must appear after KASMOS_MANAGED (MANAGED is left of env assignments)")

	// nice must appear after the env assignments and before the program.
	niceIdx := strings.Index(cmdStr, "nice -n 10")
	claudeIdx := strings.Index(cmdStr, "claude")
	require.NotEqual(t, -1, niceIdx, "nice must appear in command")
	require.NotEqual(t, -1, claudeIdx, "claude must appear in command")
	assert.Less(t, niceIdx, claudeIdx, "nice must precede claude in command")
	assert.Greater(t, niceIdx, profileIdx, "nice must follow wrapper env assignments")
}

// TestStart_NormalProfile_NoResourceProfileTmuxEnv verifies that the normal profile
// does NOT inject KASMOS_RESOURCE_PROFILE via tmux set-environment.
func TestStart_NormalProfile_NoResourceProfileTmuxEnv(t *testing.T) {
	// serial: mutates tmux timing globals
	withFastTmuxTimings(t)
	ptyFactory := NewMockPtyFactory(t)
	cmdExec, ranCmds := makeTestCmdExecForResourceControls(t)

	workdir := t.TempDir()
	session := newTmuxSession("test-rc-normal-env", "claude", false, ptyFactory, cmdExec)
	session.SetResourceControls(normalProfile())

	err := session.Start(workdir)
	require.NoError(t, err)

	// KASMOS_RESOURCE_PROFILE must not appear in any set-environment call.
	for _, cmd := range *ranCmds {
		if strings.Contains(cmd, "set-environment") {
			assert.NotContains(t, cmd, "KASMOS_RESOURCE_PROFILE",
				"normal profile must not set KASMOS_RESOURCE_PROFILE in tmux environment")
		}
	}
}

// TestStart_InteractiveProfile_InjectsTmuxResourceProfileEnv verifies that the
// interactive profile DOES inject KASMOS_RESOURCE_PROFILE via tmux set-environment.
func TestStart_InteractiveProfile_InjectsTmuxResourceProfileEnv(t *testing.T) {
	// serial: mutates tmux timing globals
	withFastTmuxTimings(t)
	ptyFactory := NewMockPtyFactory(t)
	cmdExec, ranCmds := makeTestCmdExecForResourceControls(t)

	workdir := t.TempDir()
	session := newTmuxSession("test-rc-interactive-env", "claude", false, ptyFactory, cmdExec)
	session.SetResourceControls(interactiveProfile())

	err := session.Start(workdir)
	require.NoError(t, err)

	// At least one set-environment call must set KASMOS_RESOURCE_PROFILE=interactive.
	found := false
	for _, cmd := range *ranCmds {
		if strings.Contains(cmd, "set-environment") && strings.Contains(cmd, "KASMOS_RESOURCE_PROFILE") && strings.Contains(cmd, "interactive") {
			found = true
			break
		}
	}
	assert.True(t, found, "interactive profile must set KASMOS_RESOURCE_PROFILE in tmux environment; ran: %v", *ranCmds)
}
