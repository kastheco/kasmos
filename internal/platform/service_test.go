package platform

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── string helper table tests ──────────────────────────────────────────────

func TestServiceManagerName(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{"linux", "systemd"},
		{"darwin", "launchd"},
		{"windows", "service manager"},
		{"freebsd", "service manager"},
	}
	for _, tc := range tests {
		t.Run(tc.goos, func(t *testing.T) {
			assert.Equal(t, tc.want, serviceManagerName(tc.goos))
		})
	}
}

func TestDaemonStartCommand(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{"linux", "systemctl --user start kasmos"},
		{"darwin", "launchctl load -w ~/Library/LaunchAgents/com.kasmos.daemon.plist"},
		{"windows", "kas daemon start --foreground"},
	}
	for _, tc := range tests {
		t.Run(tc.goos, func(t *testing.T) {
			assert.Equal(t, tc.want, daemonStartCommand(tc.goos))
		})
	}
}

func TestRestartServicesCommand(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{"linux", "systemctl --user start kasmosdb kasmos"},
		{"darwin", "launchctl load -w ~/Library/LaunchAgents/com.kasmos.taskstore.plist && launchctl load -w ~/Library/LaunchAgents/com.kasmos.daemon.plist"},
		{"freebsd", "kas daemon start --foreground"},
	}
	for _, tc := range tests {
		t.Run(tc.goos, func(t *testing.T) {
			assert.Equal(t, tc.want, restartServicesCommand(tc.goos))
		})
	}
}

func TestInstallDir(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{"linux", "~/.config/systemd/user"},
		{"darwin", "~/Library/LaunchAgents"},
		{"windows", ""},
	}
	for _, tc := range tests {
		t.Run(tc.goos, func(t *testing.T) {
			assert.Equal(t, tc.want, installDir(tc.goos))
		})
	}
}

// ─── StopServices: darwin tests ─────────────────────────────────────────────

func TestStopServices_Darwin_NoPlistFiles(t *testing.T) {
	home := t.TempDir()
	restore := patchPlatformSeams(t, home, "launchctl", nil)
	defer restore()

	// No plist files in the temp home — unload must never be called.
	var called []string
	platformRunCommand = func(name string, args ...string) error {
		called = append(called, name)
		return nil
	}

	err := stopServices("darwin")
	require.NoError(t, err)
	assert.Empty(t, called, "expected no commands to be run when no plist files exist")
}

func TestStopServices_Darwin_OnePlistFile(t *testing.T) {
	home := t.TempDir()
	laDir := filepath.Join(home, "Library", "LaunchAgents")
	require.NoError(t, os.MkdirAll(laDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(laDir, daemonPlist), []byte(""), 0o644))

	restore := patchPlatformSeams(t, home, "launchctl", nil)
	defer restore()

	var ran [][]string
	platformRunCommand = func(name string, args ...string) error {
		ran = append(ran, append([]string{name}, args...))
		return nil
	}

	require.NoError(t, stopServices("darwin"))
	require.Len(t, ran, 1)
	assert.Equal(t, []string{"launchctl", "unload", "-w", filepath.Join(laDir, daemonPlist)}, ran[0])
}

func TestStopServices_Darwin_BothPlistFiles(t *testing.T) {
	home := t.TempDir()
	laDir := filepath.Join(home, "Library", "LaunchAgents")
	require.NoError(t, os.MkdirAll(laDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(laDir, daemonPlist), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(laDir, taskstorePlist), []byte(""), 0o644))

	restore := patchPlatformSeams(t, home, "launchctl", nil)
	defer restore()

	var ran [][]string
	platformRunCommand = func(name string, args ...string) error {
		ran = append(ran, append([]string{name}, args...))
		return nil
	}

	require.NoError(t, stopServices("darwin"))
	assert.Len(t, ran, 2)
}

func TestStopServices_Darwin_UnloadError(t *testing.T) {
	home := t.TempDir()
	laDir := filepath.Join(home, "Library", "LaunchAgents")
	require.NoError(t, os.MkdirAll(laDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(laDir, daemonPlist), []byte(""), 0o644))

	restore := patchPlatformSeams(t, home, "launchctl", nil)
	defer restore()

	unloadErr := errors.New("exit status 1")
	platformRunCommand = func(name string, args ...string) error {
		return unloadErr
	}

	err := stopServices("darwin")
	require.Error(t, err)
	assert.ErrorContains(t, err, "unload")
}

func TestStopServices_Darwin_NoLaunchctl(t *testing.T) {
	home := t.TempDir()
	notFoundErr := errors.New("not found")
	restore := patchPlatformSeams(t, home, "", notFoundErr)
	defer restore()

	var called bool
	platformRunCommand = func(name string, args ...string) error {
		called = true
		return nil
	}

	require.NoError(t, stopServices("darwin"))
	assert.False(t, called)
}

// ─── StopServices: linux tests ───────────────────────────────────────────────

func TestStopServices_Linux_NoSystemctl(t *testing.T) {
	origLook := platformLookPath
	origRun := platformRunCommand
	t.Cleanup(func() {
		platformLookPath = origLook
		platformRunCommand = origRun
	})

	platformLookPath = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	var called bool
	platformRunCommand = func(name string, args ...string) error {
		called = true
		return nil
	}

	require.NoError(t, stopServices("linux"))
	assert.False(t, called)
}

func TestStopServices_Linux_BothInactive(t *testing.T) {
	origLook := platformLookPath
	origRun := platformRunCommand
	t.Cleanup(func() {
		platformLookPath = origLook
		platformRunCommand = origRun
	})

	platformLookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	var stopCalled bool
	platformRunCommand = func(name string, args ...string) error {
		// Detect the stop invocation by looking for the "stop" subcommand.
		for _, arg := range args {
			if arg == "stop" {
				stopCalled = true
				return nil
			}
		}
		// is-active returns non-zero (inactive) for both services.
		return errors.New("inactive")
	}

	require.NoError(t, stopServices("linux"))
	assert.False(t, stopCalled)
}

func TestStopServices_Linux_ActiveServices(t *testing.T) {
	origLook := platformLookPath
	origRun := platformRunCommand
	t.Cleanup(func() {
		platformLookPath = origLook
		platformRunCommand = origRun
	})

	platformLookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	var commands [][]string
	platformRunCommand = func(name string, args ...string) error {
		commands = append(commands, append([]string{name}, args...))
		// is-active for "kasmos" succeeds (active); kasmosdb is inactive.
		if len(args) >= 3 && args[2] == "--quiet" && args[len(args)-1] == "kasmos" {
			return nil // active
		}
		if len(args) >= 3 && args[2] == "--quiet" {
			return errors.New("inactive")
		}
		return nil // stop command succeeds
	}

	require.NoError(t, stopServices("linux"))
	// Last command must be the stop.
	require.NotEmpty(t, commands)
	last := commands[len(commands)-1]
	assert.Equal(t, "systemctl", last[0])
	assert.Contains(t, last, "stop")
}

// ─── StopServices: unsupported OS ────────────────────────────────────────────

func TestStopServices_Unsupported(t *testing.T) {
	origRun := platformRunCommand
	t.Cleanup(func() { platformRunCommand = origRun })

	var called bool
	platformRunCommand = func(name string, args ...string) error {
		called = true
		return nil
	}

	require.NoError(t, stopServices("freebsd"))
	assert.False(t, called)
}

// ─── helpers ────────────────────────────────────────────────────────────────

// patchPlatformSeams replaces the package-level seams for a single test and
// returns a restore function. lookPath returns the faked binary path (or
// lookErr if non-nil). platformUserHomeDir is wired to return home.
func patchPlatformSeams(t *testing.T, home, binaryPath string, lookErr error) func() {
	t.Helper()
	origLook := platformLookPath
	origRun := platformRunCommand
	origHome := platformUserHomeDir

	platformLookPath = func(file string) (string, error) {
		if lookErr != nil {
			return "", lookErr
		}
		return binaryPath, nil
	}
	platformUserHomeDir = func() (string, error) { return home, nil }

	return func() {
		platformLookPath = origLook
		platformRunCommand = origRun
		platformUserHomeDir = origHome
	}
}
