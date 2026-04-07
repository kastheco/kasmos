package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunRepoReset_UsesPlatformRestartInstruction verifies that the restart
// instruction line shown at the end of runRepoReset is sourced from the
// injected repoResetRestartServicesCommand helper rather than a hardcoded string.
func TestRunRepoReset_UsesPlatformRestartInstruction(t *testing.T) {
	backupRoot := t.TempDir()
	t.Setenv("BACKUP_ROOT", backupRoot)

	repo := makeFakeRepo(t)

	// Inject a deterministic restart command string.
	origCmd := repoResetRestartServicesCommand
	t.Cleanup(func() { repoResetRestartServicesCommand = origCmd })
	repoResetRestartServicesCommand = func() string { return "test-restart-cmd" }

	// Prevent actual service calls during the test.
	origStop := repoResetStopServices
	t.Cleanup(func() { repoResetStopServices = origStop })
	repoResetStopServices = func(_ repoResetOptions) error { return nil }

	var out bytes.Buffer
	err := runRepoReset(nil, repoResetOptions{
		DryRun: true,
		Yes:    true,
		Stdout: &out,
		Stderr: &out,
		Stdin:  strings.NewReader(repo + "\n"),
	})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "restart services with: test-restart-cmd")
	assert.NotContains(t, out.String(), "systemctl")
}

// TestDefaultRepoResetStopServices_DryRun ensures that neither stop helper is
// called in dry-run mode.
func TestDefaultRepoResetStopServices_DryRun(t *testing.T) {
	origStop := repoResetStopUserServices
	t.Cleanup(func() { repoResetStopUserServices = origStop })
	stopCalled := false
	repoResetStopUserServices = func() error {
		stopCalled = true
		return nil
	}

	origPID := repoResetStopDaemonByPID
	t.Cleanup(func() { repoResetStopDaemonByPID = origPID })
	pidCalled := false
	repoResetStopDaemonByPID = func(_ string) error {
		pidCalled = true
		return nil
	}

	var out bytes.Buffer
	err := defaultRepoResetStopServices(repoResetOptions{
		DryRun: true,
		Stdout: &out,
		Stderr: &out,
	})
	require.NoError(t, err)
	assert.False(t, stopCalled, "service stop should not be called in dry-run")
	assert.False(t, pidCalled, "pid stop should not be called in dry-run")
	assert.Contains(t, out.String(), "[dry-run] would stop daemon and user services if running")
}

// TestDefaultRepoResetStopServices_ServiceStopFailure verifies that a failure
// from repoResetStopUserServices only emits a warning and returns nil.
func TestDefaultRepoResetStopServices_ServiceStopFailure(t *testing.T) {
	origStop := repoResetStopUserServices
	t.Cleanup(func() { repoResetStopUserServices = origStop })
	repoResetStopUserServices = func() error {
		return errors.New("systemctl exploded")
	}

	// Ensure no PID file exists so PID fallback is skipped.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var stdout, stderr bytes.Buffer
	err := defaultRepoResetStopServices(repoResetOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err, "service stop failure should not propagate")
	assert.Contains(t, stderr.String(), "warning: failed to stop user services")
}

// TestDefaultRepoResetStopServices_NoPIDFile verifies that the PID stop
// fallback is silently skipped when no PID file exists.
func TestDefaultRepoResetStopServices_NoPIDFile(t *testing.T) {
	origStop := repoResetStopUserServices
	t.Cleanup(func() { repoResetStopUserServices = origStop })
	repoResetStopUserServices = func() error { return nil }

	origPID := repoResetStopDaemonByPID
	t.Cleanup(func() { repoResetStopDaemonByPID = origPID })
	pidCalled := false
	repoResetStopDaemonByPID = func(_ string) error {
		pidCalled = true
		return nil
	}

	// HOME points at an empty temp dir — no PID file present.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var stderr bytes.Buffer
	err := defaultRepoResetStopServices(repoResetOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
	})
	require.NoError(t, err)
	assert.False(t, pidCalled, "pid stop should not be called when no PID file exists")
	// No noisy "failed to stop daemon" warning expected.
	assert.NotContains(t, stderr.String(), "failed to stop daemon")
}

// TestDefaultRepoResetStopServices_ExistingPIDFile verifies that the PID stop
// fallback is invoked when a PID file is present.
func TestDefaultRepoResetStopServices_ExistingPIDFile(t *testing.T) {
	origStop := repoResetStopUserServices
	t.Cleanup(func() { repoResetStopUserServices = origStop })
	repoResetStopUserServices = func() error { return nil }

	origPID := repoResetStopDaemonByPID
	t.Cleanup(func() { repoResetStopDaemonByPID = origPID })
	var receivedPath string
	repoResetStopDaemonByPID = func(path string) error {
		receivedPath = path
		return nil
	}

	// Create a fake PID file at the location daemonPIDPath() resolves to.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	pidPath := daemonPIDPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(pidPath), 0o755))
	require.NoError(t, os.WriteFile(pidPath, []byte("12345\n"), 0o644))

	err := defaultRepoResetStopServices(repoResetOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(receivedPath, "daemon.pid"), "pid stop should be called with the PID file path")
}
