package livepreview

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// PaneRunner abstracts external command execution for CapturePane and SendPrompt.
// It is a subset of the CmdRunner interface used by the MCP handlers, allowing
// the HTTP live-preview handler to supply its own lightweight implementation.
type PaneRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ErrSessionGone is returned by CapturePane or SendPrompt when tmux reports
// that the target session, window, or pane no longer exists.
var ErrSessionGone = errors.New("tmux session not found")

// CommandError wraps a non-zero tmux exit that carries useful stderr output.
// It is returned when the exit error is not a recognised session-not-found
// variant, giving the caller enough context to surface a meaningful error body.
type CommandError struct {
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("%v: %s", e.Err, strings.TrimSpace(e.Stderr))
	}
	return e.Err.Error()
}

func (e *CommandError) Unwrap() error { return e.Err }

// runPaneCommand executes tmux with the given args through runner and maps
// *exec.ExitError results to ErrSessionGone or *CommandError, matching the
// classification used by CapturePane.
//
//   - *exec.ExitError whose Stderr contains "can't find pane/window/session" → ErrSessionGone
//   - *exec.ExitError with any other stderr content → *CommandError
//   - any other error → returned as-is
func runPaneCommand(ctx context.Context, runner PaneRunner, args ...string) ([]byte, error) {
	out, err := runner.Output(ctx, "tmux", args...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := string(exitErr.Stderr)
			lower := strings.ToLower(stderr)
			if strings.Contains(lower, "can't find pane") ||
				strings.Contains(lower, "can't find window") ||
				strings.Contains(lower, "can't find session") {
				return nil, ErrSessionGone
			}
			return nil, &CommandError{Stderr: stderr, Err: err}
		}
		return nil, err
	}
	return out, nil
}

// CapturePane invokes tmux capture-pane and returns the pane output verbatim.
// start and end are the optional -S/-E line offsets; empty or whitespace-only
// strings are omitted from the tmux arguments.
//
// Error classification:
//   - *exec.ExitError whose Stderr contains "can't find pane/window/session" → ErrSessionGone
//   - *exec.ExitError with any other stderr content → *CommandError
//   - any other error → returned as-is
func CapturePane(ctx context.Context, runner PaneRunner, rec Record, start, end string) (string, error) {
	args := []string{"capture-pane", "-p", "-e", "-J", "-t", SessionName(rec.Title)}
	if s := strings.TrimSpace(start); s != "" {
		args = append(args, "-S", s)
	}
	if e := strings.TrimSpace(end); e != "" {
		args = append(args, "-E", e)
	}

	out, err := runPaneCommand(ctx, runner, args...)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
