package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/internal/platform"
	"github.com/stretchr/testify/assert"
)

func TestDaemonCmd_HasSubcommands(t *testing.T) {
	cmd := NewDaemonCmd()
	subcommands := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subcommands[sub.Name()] = true
	}
	assert.True(t, subcommands["start"], "missing 'start' subcommand")
	assert.True(t, subcommands["stop"], "missing 'stop' subcommand")
	assert.True(t, subcommands["status"], "missing 'status' subcommand")
	assert.True(t, subcommands["add"], "missing 'add' subcommand")
	assert.True(t, subcommands["remove"], "missing 'remove' subcommand")
}

func TestDaemonStatusCmd_NotRunning(t *testing.T) {
	socketPath := "/nonexistent/kas.sock"
	cmd := newDaemonStatusCmd(&socketPath)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	err := cmd.RunE(cmd, nil)
	combined := buf.String()
	if err != nil {
		combined += err.Error()
	}
	assert.Contains(t, combined, "not running")
}

func TestDaemonStartCmd_UsesPlatformSpecificHelpText(t *testing.T) {
	cmd := newDaemonStartCmd()
	manager := platform.ServiceManagerName()

	assert.Contains(t, cmd.Long, manager,
		"Long description should reference the platform service manager (%q)", manager)

	foregroundFlag := cmd.Flags().Lookup("foreground")
	if assert.NotNil(t, foregroundFlag, "expected --foreground flag to be defined") {
		assert.True(t, strings.Contains(foregroundFlag.Usage, manager),
			"--foreground usage should reference the platform service manager (%q), got: %q",
			manager, foregroundFlag.Usage)
	}
}

func TestDaemonStatusCmd_UsesUpdatedSocketFlagValue(t *testing.T) {
	socketPath := "/tmp/initial.sock"
	cmd := newDaemonStatusCmd(&socketPath)
	socketPath = "/nonexistent/override.sock"

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	err := cmd.RunE(cmd, nil)
	combined := buf.String()
	if err != nil {
		combined += err.Error()
	}
	assert.Contains(t, combined, "/nonexistent/override.sock")
}
