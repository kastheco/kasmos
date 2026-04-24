package sdk

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// shellOutputCap bounds the bytes captured from a shell subprocess so a runaway
// command cannot balloon the renderer's in-memory transcript.
const shellOutputCap = 64 * 1024

// errShellNotAvailable is returned when no usable shell can be found.
var errShellNotAvailable = errors.New("no usable shell found in PATH ($SHELL, zsh, bash, sh)")

// shellRunner is the test seam for command execution. Tests override it to
// avoid spawning real processes.
type shellRunner func(ctx context.Context, workDir, shell string, args []string) (exitCode int, output string, truncated bool, err error)

type cappedOutputBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *cappedOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.limit <= 0 {
		b.truncated = b.truncated || len(p) > 0
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			_, _ = b.buf.Write(p[:remaining])
			b.truncated = true
		} else {
			_, _ = b.buf.Write(p)
		}
	} else if len(p) > 0 {
		b.truncated = true
	}
	return len(p), nil
}

func (b *cappedOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *cappedOutputBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}

// defaultShellRunner spawns the selected shell with the requested flags and
// captures up to shellOutputCap bytes of combined stdout+stderr. Non-zero
// exit codes are NOT returned as errors — they flow through exitCode so the
// caller can render them as a status row.
func defaultShellRunner(ctx context.Context, workDir, shell string, args []string) (int, string, bool, error) {
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Dir = workDir

	output := &cappedOutputBuffer{limit: shellOutputCap}
	cmd.Stdout = output
	cmd.Stderr = output
	runErr := cmd.Run()

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

	return exitCode, output.String(), output.Truncated(), nil
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
