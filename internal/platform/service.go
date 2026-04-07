package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// launchd plist filenames used by kasmos on macOS.
const (
	daemonPlist    = "com.kasmos.daemon.plist"
	taskstorePlist = "com.kasmos.taskstore.plist"
)

// Package-level seams that tests can replace to stay hermetic.
var (
	platformLookPath   = exec.LookPath
	platformRunCommand = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
	platformCommandOutput = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
	platformUserHomeDir = os.UserHomeDir
)

// ServiceManagerName returns the name of the service manager for the current OS.
func ServiceManagerName() string { return serviceManagerName(runtime.GOOS) }

// DaemonStartCommand returns the shell command that starts the kasmos daemon.
func DaemonStartCommand() string { return daemonStartCommand(runtime.GOOS) }

// RestartServicesCommand returns the shell command that (re)starts all kasmos services.
func RestartServicesCommand() string { return restartServicesCommand(runtime.GOOS) }

// InstallDir returns the directory where service unit/plist files should be placed.
func InstallDir() string { return installDir(runtime.GOOS) }

// StopServices stops kasmos services using the appropriate service manager.
func StopServices() error { return stopServices(runtime.GOOS) }

// serviceManagerName returns the service manager label for the given GOOS value.
func serviceManagerName(goos string) string {
	switch goos {
	case "linux":
		return "systemd"
	case "darwin":
		return "launchd"
	default:
		return "service manager"
	}
}

// daemonStartCommand returns the command to start only the kasmos daemon.
func daemonStartCommand(goos string) string {
	switch goos {
	case "linux":
		return "systemctl --user start kasmos"
	case "darwin":
		return "launchctl load -w ~/Library/LaunchAgents/" + daemonPlist
	default:
		return "kas daemon start --foreground"
	}
}

// restartServicesCommand returns the command to start all kasmos services.
func restartServicesCommand(goos string) string {
	switch goos {
	case "linux":
		return "systemctl --user start kasmosdb kasmos"
	case "darwin":
		return "launchctl load -w ~/Library/LaunchAgents/" + taskstorePlist +
			" && launchctl load -w ~/Library/LaunchAgents/" + daemonPlist
	default:
		return "kas daemon start --foreground"
	}
}

// installDir returns the directory where service files should be installed.
func installDir(goos string) string {
	switch goos {
	case "linux":
		return "~/.config/systemd/user"
	case "darwin":
		return "~/Library/LaunchAgents"
	default:
		return ""
	}
}

// launchAgentPath returns the full path for a given plist filename under the
// user's LaunchAgents directory. home must be the resolved home directory.
func launchAgentPath(home, filename string) string {
	return filepath.Join(home, "Library", "LaunchAgents", filename)
}

// stopServices stops kasmos services using the service manager for goos.
func stopServices(goos string) error {
	switch goos {
	case "linux":
		return stopServicesLinux()
	case "darwin":
		return stopServicesDarwin()
	default:
		return nil
	}
}

func stopServicesLinux() error {
	if _, err := platformLookPath("systemctl"); err != nil {
		// systemctl not available — nothing to stop.
		return nil
	}

	// No-op if both units are already inactive.
	active1 := platformRunCommand("systemctl", "--user", "is-active", "--quiet", "kasmos")
	active2 := platformRunCommand("systemctl", "--user", "is-active", "--quiet", "kasmosdb")
	if active1 != nil && active2 != nil {
		return nil
	}

	return platformRunCommand("systemctl", "--user", "stop", "kasmos", "kasmosdb")
}

func stopServicesDarwin() error {
	home, err := platformUserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	if _, err := platformLookPath("launchctl"); err != nil {
		// launchctl not available — nothing to stop.
		return nil
	}

	plists := []string{daemonPlist, taskstorePlist}
	var firstErr error
	for _, name := range plists {
		path := launchAgentPath(home, name)
		if _, err := os.Stat(path); err != nil {
			// plist does not exist — skip.
			continue
		}
		if err := platformRunCommand("launchctl", "unload", "-w", path); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("unload %s: %w", name, err)
			}
		}
	}
	return firstErr
}
