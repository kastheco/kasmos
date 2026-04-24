package sdk

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveShell_KnownShellFromEnv_UsesLoginFlag(t *testing.T) {
	for _, name := range []string{"zsh", "bash", "sh"} {
		t.Run(name, func(t *testing.T) {
			path, lookErr := exec.LookPath(name)
			if lookErr != nil {
				t.Skipf("%s not available on PATH", name)
			}
			t.Setenv("SHELL", path)

			shell, flag, err := resolveShell()
			require.NoError(t, err)
			assert.Equal(t, path, shell)
			assert.Equal(t, "-lc", flag)
		})
	}
}

func TestResolveShell_UnknownShellFromEnv_UsesCFlag(t *testing.T) {
	// Point $SHELL at a real binary that is not zsh/bash/sh.
	// Use "env" which is always on PATH.
	path, err := exec.LookPath("env")
	if err != nil {
		t.Skip("env not available on PATH")
	}
	t.Setenv("SHELL", path)

	shell, flag, resolveErr := resolveShell()
	require.NoError(t, resolveErr)
	assert.Equal(t, path, shell)
	assert.Equal(t, "-c", flag)
}

func TestResolveShell_EmptyShellEnv_FallsBackToKnownShell(t *testing.T) {
	t.Setenv("SHELL", "")

	shell, flag, err := resolveShell()
	require.NoError(t, err, "should fall back to a known shell even when $SHELL is empty")
	assert.NotEmpty(t, shell)
	base := filepath.Base(shell)
	assert.Contains(t, []string{"zsh", "bash", "sh"}, base)
	assert.Equal(t, "-lc", flag)
}

func TestResolveShell_NonExistentShellEnv_FallsBack(t *testing.T) {
	t.Setenv("SHELL", "/nonexistent/shell/definitely/not/here")

	shell, flag, err := resolveShell()
	require.NoError(t, err, "should fall back to a known shell when $SHELL does not exist")
	assert.NotEmpty(t, shell)
	base := filepath.Base(shell)
	assert.Contains(t, []string{"zsh", "bash", "sh"}, base)
	assert.Equal(t, "-lc", flag)
}

func TestDefaultShellRunner_UsesWorkDirAndPreservesNonZeroExit(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available on PATH")
	}
	workDir := t.TempDir()

	exitCode, output, truncated, runErr := defaultShellRunner(context.Background(), workDir, shell, []string{"-c", "pwd; exit 7"})

	require.NoError(t, runErr)
	assert.Equal(t, 7, exitCode)
	assert.Contains(t, output, workDir)
	assert.False(t, truncated)
}

func TestDefaultShellRunner_CapsOutputWithoutChangingExitCode(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available on PATH")
	}
	workDir := t.TempDir()

	exitCode, output, truncated, runErr := defaultShellRunner(context.Background(), workDir, shell, []string{"-c", "printf '%70000s' ''; exit 7"})

	require.NoError(t, runErr)
	assert.Equal(t, 7, exitCode)
	assert.Len(t, output, shellOutputCap)
	assert.True(t, truncated)
}
