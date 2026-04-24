package sdk

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// shellOutputCap bounds the bytes captured from a shell subprocess so a runaway
// command cannot balloon the renderer's in-memory transcript.
const shellOutputCap = 64 * 1024

// errShellNotAvailable is returned when no usable shell can be found.
var errShellNotAvailable = errors.New("no usable shell found in PATH ($SHELL, zsh, bash, sh)")

// shellRunner is the test seam for command execution. Tests override it to
// avoid spawning real processes.
type shellRunner func(ctx context.Context, workDir, shell string, args []string) (exitCode int, output string, truncated bool, err error)

// defaultShellRunner spawns the selected shell with the requested flags and
// captures up to shellOutputCap bytes of combined stdout+stderr. Non-zero
// exit codes are NOT returned as errors — they flow through exitCode so the
// caller can render them as a status row.
func defaultShellRunner(ctx context.Context, workDir, shell string, args []string) (int, string, bool, error) {
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Dir = workDir

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return -1, "", false, err
	}

	var buf bytes.Buffer
	copyDone := make(chan struct{})
	go func() {
		defer close(copyDone)
		io.CopyN(&buf, pr, shellOutputCap+1) //nolint:errcheck
		pr.Close()
	}()

	runErr := cmd.Wait()
	pw.Close()
	<-copyDone

	var exitCode int
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
			// non-zero exit is not an infrastructure error
		} else {
			return -1, "", false, runErr
		}
	}

	truncated := false
	out := buf.String()
	if len(out) > shellOutputCap {
		out = out[:shellOutputCap]
		truncated = true
	}

	return exitCode, out, truncated, nil
}

// resolveShell returns the shell command and flag ("-lc" for known shells,
// "-c" for unknown) to run a single command string. Lookup order:
// $SHELL (validated on PATH), then zsh, bash, sh.
func resolveShell() (shell string, flag string, err error) {
	// Try $SHELL first.
	if envShell := os.Getenv("SHELL"); envShell != "" {
		if path, lookErr := exec.LookPath(envShell); lookErr == nil {
			base := filepath.Base(path)
			if base == "zsh" || base == "bash" || base == "sh" {
				return path, "-lc", nil
			}
			return path, "-c", nil
		}
	}

	// Fallback list.
	for _, s := range []string{"zsh", "bash", "sh"} {
		if path, lookErr := exec.LookPath(s); lookErr == nil {
			return path, "-lc", nil
		}
	}

	return "", "", errShellNotAvailable
}
