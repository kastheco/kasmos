package livepreview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrActionInstanceNotFound is returned by ApplyAction when no instance with the
// given title is found in the state file.
var ErrActionInstanceNotFound = errors.New("instance not found")

// ErrActionInvalidState is returned by ApplyAction when the requested action is
// not valid for the instance's current state.
var ErrActionInvalidState = errors.New("invalid instance state for action")

// CommandRunner abstracts external command execution for standalone instance
// actions. Only Run (fire-and-forget) is needed — standalone actions do not
// capture command output.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

// ExecCommandRunner is the real CommandRunner backed by os/exec.
type ExecCommandRunner struct{}

// Run implements CommandRunner using os/exec.
func (r *ExecCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

// ApplyAction loads the full state.json at repoRoot, finds the instance by
// title, validates and executes action, then persists the updated state back to
// disk. The top-level state envelope (including help_screens_seen and any
// future unknown fields) is preserved verbatim.
//
// Actions:
//   - pause:   validate state, best-effort kill tmux session, best-effort
//              remove/prune owned worktree, persist StatusPaused.
//   - resume:  recreate worktree and tmux session from stored metadata, persist
//              StatusRunning. Returns an error when worktree metadata is absent.
//   - restart: reject paused rows (ErrActionInvalidState), best-effort stop old
//              session, start fresh session with same metadata, persist StatusRunning.
//   - kill:    best-effort stop tmux session, best-effort worktree cleanup,
//              remove the record from the instances slice entirely.
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
		// Best-effort: kill the tmux session.
		_ = runner.Run(ctx, "tmux", "kill-session", "-t", SessionName(rec.Title))
		// Best-effort: remove and prune the owned worktree.
		if rec.Worktree.RepoPath != "" && rec.Worktree.WorktreePath != "" {
			_ = runner.Run(ctx, "git", "-C", rec.Worktree.RepoPath,
				"worktree", "remove", "--force", rec.Worktree.WorktreePath)
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
		_ = runner.Run(ctx, "tmux", "kill-session", "-t", SessionName(rec.Title))
		if rec.Worktree.RepoPath != "" && rec.Worktree.WorktreePath != "" {
			_ = runner.Run(ctx, "git", "-C", rec.Worktree.RepoPath,
				"worktree", "remove", "--force", rec.Worktree.WorktreePath)
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

// standaloneResumeProgram reconstructs the full tmux command string for a
// standalone instance. It mirrors buildResumeProgram from
// internal/mcpserver/instancetools/resume.go without depending on that package.
func standaloneResumeProgram(rec Record, worktreePath string) string {
	program := rec.Program

	if rec.SkipPermissions && strings.HasSuffix(program, "claude") {
		program += " --permission-mode bypassPermissions"
	}
	if rec.AgentType != "" && !strings.Contains(program, "--agent") {
		program += " --agent " + rec.AgentType
	}
	if strings.HasSuffix(rec.Program, "opencode") {
		logDir := filepath.Join(worktreePath, ".kasmos", "logs")
		if err := os.MkdirAll(logDir, 0o755); err == nil {
			logFile := filepath.Join(logDir, SessionName(rec.Title)+".log")
			program += " --print-logs 2>>'" + logFile + "'"
		}
	}

	program = "KASMOS_MANAGED=1 " + program

	if project := standaloneProject(rec); project != "" {
		program = "KASMOS_PROJECT='" + strings.ReplaceAll(project, "'", "'\\''") + "' " + program
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
