package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PaneInfo represents parsed output from tmux list-panes.
type PaneInfo struct {
	ID         string // tmux pane identifier (e.g., "%42")
	PID        int    // process running in the pane
	Dead       bool   // true if the pane process has exited
	DeadStatus int    // exit code of the dead process
}

// SplitOpts configures split-window.
type SplitOpts struct {
	Target     string   // pane/window to split from
	Horizontal bool     // -h flag
	Size       string   // -l flag: "50%" or "80"
	Command    []string // command to run in new pane
	Env        []string // environment variables as "KEY=VALUE" for -e flags
}

// JoinOpts configures join-pane (used for both parking and showing).
type JoinOpts struct {
	Source     string // -s: pane to move
	Target     string // -t: destination window/pane
	Horizontal bool   // -h: horizontal split
	Detached   bool   // -d: don't follow focus (used when parking)
	Size       string // -l: size spec ("50%")
}

// NewWindowOpts configures new-window.
type NewWindowOpts struct {
	Detached bool   // -d: don't switch to it
	Name     string // -n: window name
}

type TmuxCLI interface {
	SplitWindow(ctx context.Context, opts SplitOpts) (string, error)
	KillPane(ctx context.Context, paneID string) error
	SelectPane(ctx context.Context, paneID string) error
	JoinPane(ctx context.Context, opts JoinOpts) error
	NewWindow(ctx context.Context, opts NewWindowOpts) (string, error)
	ListPanes(ctx context.Context, target string) ([]PaneInfo, error)
	CapturePane(ctx context.Context, paneID string) (string, error)
	DisplayMessage(ctx context.Context, format string) (string, error)
	Version(ctx context.Context) (string, error)
	SetEnvironment(ctx context.Context, key, value string) error
	ShowEnvironment(ctx context.Context) (map[string]string, error)
	UnsetEnvironment(ctx context.Context, key string) error
	SetOption(ctx context.Context, key, value string) error
	SetPaneTitle(ctx context.Context, paneID, title string) error
	SetPaneOption(ctx context.Context, paneID, option, value string) error
}

var ErrTmuxNotFound = errors.New("tmux binary not found in PATH")

type TmuxError struct {
	Command []string
	Stderr  string
	Err     error
}

func (e *TmuxError) Error() string {
	if e == nil {
		return "tmux command failed"
	}

	cmd := "tmux"
	if len(e.Command) > 0 {
		cmd = "tmux " + strings.Join(e.Command, " ")
	}

	if e.Stderr == "" {
		return fmt.Sprintf("%s: %v", cmd, e.Err)
	}

	return fmt.Sprintf("%s: %v (stderr: %s)", cmd, e.Err, e.Stderr)
}

func (e *TmuxError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsNotFound(err error) bool {
	return tmuxStderrContains(err, "can't find", "not found", "no such")
}

func IsNoSpace(err error) bool {
	return tmuxStderrContains(err, "no space for new pane")
}

func IsSessionGone(err error) bool {
	return tmuxStderrContains(err, "no server running", "session not found", "no current session")
}

func tmuxStderrContains(err error, needles ...string) bool {
	if err == nil {
		return false
	}

	var tmuxErr *TmuxError
	if !errors.As(err, &tmuxErr) {
		return false
	}

	stderr := strings.ToLower(tmuxErr.Stderr)
	for _, needle := range needles {
		if strings.Contains(stderr, needle) {
			return true
		}
	}

	return false
}

type tmuxExec struct {
	tmuxBin string
}

var _ TmuxCLI = (*tmuxExec)(nil)

func NewTmuxExec() (*tmuxExec, error) {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTmuxNotFound, err)
	}

	return &tmuxExec{tmuxBin: bin}, nil
}

func (t *tmuxExec) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, t.tmuxBin, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", &TmuxError{
			Command: append([]string(nil), args...),
			Stderr:  strings.TrimSpace(stderr.String()),
			Err:     err,
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}

func (t *tmuxExec) SplitWindow(ctx context.Context, opts SplitOpts) (string, error) {
	args := []string{"split-window"}
	if opts.Horizontal {
		args = append(args, "-h")
	}
	if opts.Target != "" {
		args = append(args, "-t", opts.Target)
	}
	if opts.Size != "" {
		args = append(args, "-l", opts.Size)
	}
	for _, env := range opts.Env {
		args = append(args, "-e", env)
	}
	args = append(args, "-P", "-F", "#{pane_id}")
	if len(opts.Command) > 0 {
		args = append(args, opts.Command...)
	}

	return t.run(ctx, args...)
}

func (t *tmuxExec) KillPane(ctx context.Context, paneID string) error {
	_, err := t.run(ctx, "kill-pane", "-t", paneID)
	if IsNotFound(err) {
		return nil
	}

	return err
}

func (t *tmuxExec) SelectPane(ctx context.Context, paneID string) error {
	_, err := t.run(ctx, "select-pane", "-t", paneID)
	return err
}

func (t *tmuxExec) JoinPane(ctx context.Context, opts JoinOpts) error {
	args := []string{"join-pane"}
	if opts.Source != "" {
		args = append(args, "-s", opts.Source)
	}
	if opts.Target != "" {
		args = append(args, "-t", opts.Target)
	}
	if opts.Horizontal {
		args = append(args, "-h")
	}
	if opts.Detached {
		args = append(args, "-d")
	}
	if opts.Size != "" {
		args = append(args, "-l", opts.Size)
	}

	_, err := t.run(ctx, args...)
	return err
}

func (t *tmuxExec) NewWindow(ctx context.Context, opts NewWindowOpts) (string, error) {
	args := []string{"new-window"}
	if opts.Detached {
		args = append(args, "-d")
	}
	if opts.Name != "" {
		args = append(args, "-n", opts.Name)
	}
	args = append(args, "-P", "-F", "#{window_id}")

	return t.run(ctx, args...)
}

func (t *tmuxExec) ListPanes(ctx context.Context, target string) ([]PaneInfo, error) {
	format := "#{pane_id} #{pane_pid} #{pane_dead} #{pane_dead_status}"
	args := []string{"list-panes"}
	if target == "-s" {
		args = append(args, "-s")
	} else if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, "-F", format)

	out, err := t.run(ctx, args...)
	if err != nil {
		return nil, err
	}

	return parsePaneList(out)
}

func parsePaneList(output string) ([]PaneInfo, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	panes := make([]PaneInfo, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		panePID, err := strconv.Atoi(fields[1])
		if err != nil {
			panePID = 0
		}

		deadStatus := -1
		if len(fields) > 3 {
			status, err := strconv.Atoi(fields[3])
			if err == nil {
				deadStatus = status
			}
		}

		panes = append(panes, PaneInfo{
			ID:         fields[0],
			PID:        panePID,
			Dead:       fields[2] == "1",
			DeadStatus: deadStatus,
		})
	}

	return panes, nil
}

func (t *tmuxExec) CapturePane(ctx context.Context, paneID string) (string, error) {
	return t.run(ctx, "capture-pane", "-p", "-t", paneID, "-S", "-")
}

func (t *tmuxExec) DisplayMessage(ctx context.Context, format string) (string, error) {
	return t.run(ctx, "display-message", "-p", format)
}

func (t *tmuxExec) Version(ctx context.Context) (string, error) {
	return t.run(ctx, "-V")
}

func (t *tmuxExec) SetEnvironment(ctx context.Context, key, value string) error {
	_, err := t.run(ctx, "set-environment", key, value)
	return err
}

func (t *tmuxExec) ShowEnvironment(ctx context.Context) (map[string]string, error) {
	out, err := t.run(ctx, "show-environment")
	if err != nil {
		return nil, err
	}

	return parseEnvironment(out)
}

func parseEnvironment(output string) (map[string]string, error) {
	env := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			continue
		}

		env[key] = value
	}

	return env, nil
}

func (t *tmuxExec) UnsetEnvironment(ctx context.Context, key string) error {
	_, err := t.run(ctx, "set-environment", "-u", key)
	return err
}

func (t *tmuxExec) SetOption(ctx context.Context, key, value string) error {
	_, err := t.run(ctx, "set-option", key, value)
	return err
}

func (t *tmuxExec) SetPaneTitle(ctx context.Context, paneID, title string) error {
	_, err := t.run(ctx, "select-pane", "-t", paneID, "-T", title)
	return err
}

func (t *tmuxExec) SetPaneOption(ctx context.Context, paneID, option, value string) error {
	_, err := t.run(ctx, "set-option", "-p", "-t", paneID, option, value)
	return err
}
