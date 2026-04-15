package livepreview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kastheco/kasmos/config"
)

// ErrActionInstanceNotFound is returned by ApplyAction when no instance with the
// given title is found in the state file.
var ErrActionInstanceNotFound = errors.New("instance not found")

// ErrActionInvalidState is returned by ApplyAction when the requested action is
// not valid for the instance's current state.
var ErrActionInvalidState = errors.New("invalid instance state for action")

// CommandRunner abstracts external command execution for standalone instance
// actions. Run is fire-and-forget; Output captures stdout for the dirty-state
// worktree check that gates destructive pause/kill cleanup.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecCommandRunner is the real CommandRunner backed by os/exec.
type ExecCommandRunner struct{}

// Run implements CommandRunner using os/exec.
func (r *ExecCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

// Output implements CommandRunner using os/exec, returning the command's
// standard output.
func (r *ExecCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// ApplyAction loads the full state.json at repoRoot, finds the instance by
// title, validates and executes action, then persists the updated state back to
// disk. The top-level state envelope (including help_screens_seen and any
// future unknown fields) is preserved verbatim.
//
// Actions (tmux execution mode only — headless rows are rejected in
// ValidateAction because the web path cannot safely manage a headless child
// process it does not own):
//   - pause:   gate on worktree dirty check, best-effort kill tmux session,
//     remove/prune owned worktree, persist StatusPaused.
//   - resume:  recreate worktree and tmux session from stored metadata, persist
//     StatusRunning. Returns an error when worktree metadata is absent.
//   - restart: reject paused rows (ErrActionInvalidState), best-effort stop old
//     session, start fresh session with same metadata, persist StatusRunning.
//   - kill:    gate on worktree dirty check, best-effort stop tmux session,
//     remove owned worktree, remove the record from the instances slice entirely.
//
// pause and kill refuse to proceed when the owned worktree contains
// uncommitted changes, matching the safety gate in session.Instance.Pause() /
// session.Instance.Kill() (session/instance_lifecycle.go). They no longer pass
// --force to `git worktree remove` because the dirty check already rejected
// destructive cases.
func ApplyAction(ctx context.Context, repoRoot, title, action string, runner CommandRunner) error {
	path := StateFilePath(repoRoot)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrActionInstanceNotFound, title)
		}
		return fmt.Errorf("read state: %w", err)
	}

	// Parse to a generic map so all top-level fields survive the round-trip.
	var rawState map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawState); err != nil {
		return fmt.Errorf("parse state: %w", err)
	}

	var records []Record
	if raw, ok := rawState["instances"]; ok && raw != nil {
		if err := json.Unmarshal(raw, &records); err != nil {
			return fmt.Errorf("parse instances: %w", err)
		}
	}

	idx := -1
	for i, r := range records {
		if r.Title == title {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("%w: %s", ErrActionInstanceNotFound, title)
	}
	rec := records[idx]

	// Validate the action against the current status.
	if err := ValidateAction(rec, action); err != nil {
		return fmt.Errorf("%w: %v", ErrActionInvalidState, err)
	}

	switch action {
	case "pause":
		// Refuse if the owned worktree has uncommitted changes — we must not
		// destroy work on the operator's behalf.
		if err := checkWorktreeClean(ctx, runner, rec, "pause"); err != nil {
			return err
		}
		// Best-effort: kill the tmux session.
		_ = runner.Run(ctx, "tmux", "kill-session", "-t", SessionName(rec.Title))
		// Remove and prune the owned worktree (dirty check above guards this).
		if rec.Worktree.RepoPath != "" && rec.Worktree.WorktreePath != "" {
			_ = runner.Run(ctx, "git", "-C", rec.Worktree.RepoPath,
				"worktree", "remove", rec.Worktree.WorktreePath)
			_ = runner.Run(ctx, "git", "-C", rec.Worktree.RepoPath, "worktree", "prune")
		}
		records[idx].Status = StatusPaused
		records[idx].Worktree.WorktreePath = ""

	case "resume":
		if rec.Worktree.RepoPath == "" || rec.Worktree.BranchName == "" {
			return fmt.Errorf("no stored worktree metadata for %q: cannot resume", rec.Title)
		}
		worktreePath := rec.Worktree.WorktreePath
		if worktreePath == "" {
			worktreePath = rec.Path
		}
		if err := runner.Run(ctx, "git", "-C", rec.Worktree.RepoPath,
			"worktree", "add", worktreePath, rec.Worktree.BranchName); err != nil {
			return fmt.Errorf("recreate worktree for %q: %w", rec.Title, err)
		}
		program := standaloneResumeProgram(rec, worktreePath)
		if err := runner.Run(ctx, "tmux", "new-session", "-d", "-s", SessionName(rec.Title),
			"-c", worktreePath, program); err != nil {
			return fmt.Errorf("start tmux session for %q: %w", rec.Title, err)
		}
		records[idx].Status = StatusRunning
		records[idx].Worktree.WorktreePath = worktreePath

	case "restart":
		// ValidateAction already rejected paused instances.
		_ = runner.Run(ctx, "tmux", "kill-session", "-t", SessionName(rec.Title))
		worktreePath := rec.Worktree.WorktreePath
		if worktreePath == "" {
			worktreePath = rec.Path
		}
		program := standaloneResumeProgram(rec, worktreePath)
		if err := runner.Run(ctx, "tmux", "new-session", "-d", "-s", SessionName(rec.Title),
			"-c", worktreePath, program); err != nil {
			return fmt.Errorf("restart tmux session for %q: %w", rec.Title, err)
		}
		records[idx].Status = StatusRunning

	case "kill":
		if err := checkWorktreeClean(ctx, runner, rec, "kill"); err != nil {
			return err
		}
		_ = runner.Run(ctx, "tmux", "kill-session", "-t", SessionName(rec.Title))
		if rec.Worktree.RepoPath != "" && rec.Worktree.WorktreePath != "" {
			_ = runner.Run(ctx, "git", "-C", rec.Worktree.RepoPath,
				"worktree", "remove", rec.Worktree.WorktreePath)
			_ = runner.Run(ctx, "git", "-C", rec.Worktree.RepoPath, "worktree", "prune")
		}
		records = append(records[:idx], records[idx+1:]...)
	}

	// Marshal updated records back and write the full state envelope.
	updatedInstances, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("marshal instances: %w", err)
	}
	rawState["instances"] = json.RawMessage(updatedInstances)

	updated, err := json.MarshalIndent(rawState, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// checkWorktreeClean returns an error when the record owns a worktree that
// contains uncommitted changes. It is the safety gate that lets pause/kill
// drop the `--force` flag on `git worktree remove` without destroying work.
// Records with no owned worktree (RepoPath/WorktreePath unset) skip the
// check. If the status command itself fails (for example the worktree
// directory is missing), we refuse the action rather than silently proceed.
func checkWorktreeClean(ctx context.Context, runner CommandRunner, rec Record, action string) error {
	if rec.Worktree.RepoPath == "" || rec.Worktree.WorktreePath == "" {
		return nil
	}
	// Defence in depth: the web path must never touch a headless agent's
	// worktree. ValidateAction already rejects headless rows for the four
	// lifecycle actions, but keep this assertion local so future callers of
	// checkWorktreeClean stay safe by default.
	if config.NormalizeExecutionMode(rec.ExecutionMode) == config.ExecutionModeHeadless {
		return fmt.Errorf("%w: cannot %s a headless instance", ErrActionInvalidState, action)
	}
	out, err := runner.Output(ctx, "git", "-C", rec.Worktree.WorktreePath, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("check worktree %q for %s: %w", rec.Worktree.WorktreePath, action, err)
	}
	if len(bytes.TrimSpace(out)) > 0 {
		// Map to ErrActionInvalidState so the HTTP layer returns 409 Conflict.
		// A dirty worktree is an expected precondition failure the user should
		// see as a normal rejection, not an internal server error.
		return fmt.Errorf("%w: worktree %q has uncommitted changes; commit or stash before %sing",
			ErrActionInvalidState, rec.Worktree.WorktreePath, action)
	}
	return nil
}

// shellSingleQuote wraps s in POSIX single quotes, escaping any embedded
// single quotes with the standard close-escape-reopen pattern.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// standaloneResumeProgram reconstructs the full tmux command string for a
// standalone instance. It mirrors buildResumeProgram from
// internal/mcpserver/instancetools/resume.go without depending on that package.
func standaloneResumeProgram(rec Record, worktreePath string) string {
	program := rec.Program

	if rec.SkipPermissions && resumeProgramBase(program) == "claude" {
		program += " --permission-mode bypassPermissions"
	}
	// Append --dangerously-bypass-approvals-and-sandbox for codex when SkipPermissions
	// is enabled, mirroring session/tmux/tmux_session.go:Start.
	if rec.SkipPermissions && resumeProgramBase(program) == "codex" {
		program += " --dangerously-bypass-approvals-and-sandbox"
	}
	// codex does not accept --agent, so skip it for codex programs.
	if rec.AgentType != "" && !strings.Contains(program, "--agent") && resumeProgramBase(rec.Program) != "codex" {
		program += " --agent " + rec.AgentType
	}
	if resumeProgramBase(rec.Program) == "opencode" {
		logDir := filepath.Join(worktreePath, ".kasmos", "logs")
		if err := os.MkdirAll(logDir, 0o755); err == nil {
			logFile := filepath.Join(logDir, SessionName(rec.Title)+".log")
			program += " --print-logs 2>>" + shellSingleQuote(logFile)
		}
	}

	program = "KASMOS_MANAGED=1 " + program

	if project := standaloneProject(rec); project != "" {
		program = "KASMOS_PROJECT=" + shellSingleQuote(project) + " " + program
	}
	if rec.TaskNumber > 0 {
		program = fmt.Sprintf("KASMOS_TASK=%d KASMOS_WAVE=%d KASMOS_PEERS=%d %s",
			rec.TaskNumber, rec.WaveNumber, rec.PeerCount, program)
	}
	return program
}

// standaloneProject derives the project name from the instance record.
func standaloneProject(rec Record) string {
	if rec.Worktree.RepoPath != "" {
		return filepath.Base(filepath.Clean(rec.Worktree.RepoPath))
	}
	if rec.Path != "" {
		return filepath.Base(filepath.Clean(rec.Path))
	}
	return ""
}

func resumeProgramBase(program string) string {
	fields := strings.Fields(strings.TrimSpace(program))
	if len(fields) == 0 {
		return ""
	}
	return filepath.Base(fields[0])
}
