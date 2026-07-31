package git

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/log"
)

var (
	// ErrPRAlreadyExists reports that gh refused to create a PR because one
	// already exists for the head branch. Callers should adopt it, not retry.
	ErrPRAlreadyExists = errors.New("pull request already exists for branch")
	// ErrWorktreeDirty reports uncommitted changes blocking PR creation.
	// Terminal: a human must commit or stash.
	ErrWorktreeDirty = errors.New("worktree has uncommitted changes")
)

// PRState holds the current state of a GitHub pull request as returned by
// `gh pr view --json url,reviewDecision,statusCheckRollup,isDraft,number,headRefOid`.
type PRState struct {
	URL            string
	ReviewDecision string
	CheckStatus    string
	IsDraft        bool
	Number         int
	HeadSHA        string
}

// ParsePRViewJSON parses the JSON output of `gh pr view --json ...` into a PRState.
// A missing statusCheckRollup is not an error — CheckStatus will be empty.
func ParsePRViewJSON(data []byte) (PRState, error) {
	var raw struct {
		URL               string `json:"url"`
		ReviewDecision    string `json:"reviewDecision"`
		StatusCheckRollup *struct {
			State string `json:"state"`
		} `json:"statusCheckRollup"`
		IsDraft bool   `json:"isDraft"`
		Number  int    `json:"number"`
		HeadSHA string `json:"headRefOid"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return PRState{}, fmt.Errorf("parse pr view json: %w", err)
	}
	state := PRState{
		URL:            raw.URL,
		ReviewDecision: raw.ReviewDecision,
		IsDraft:        raw.IsDraft,
		Number:         raw.Number,
		HeadSHA:        raw.HeadSHA,
	}
	if raw.StatusCheckRollup != nil {
		state.CheckStatus = raw.StatusCheckRollup.State
	}
	return state, nil
}

// QueryPRState queries GitHub for the current state of the pull request
// associated with the worktree's branch. It returns a zero PRState (and nil
// error) when no pull request exists for the branch.
//
// The command runs inside repoPath rather than worktreePath: gh pr view only
// needs any valid git repo directory, and the branch-specific worktree may
// have been cleaned up after the plan completed.
func (g *GitWorktree) QueryPRState() (PRState, error) {
	out, err := ghOutput(g.repoPath, "pr", "view", g.branchName,
		"--json", "url,reviewDecision,statusCheckRollup,isDraft,number,headRefOid")
	if err != nil {
		if strings.Contains(err.Error(), "no pull requests found") {
			return PRState{}, nil
		}
		return PRState{}, fmt.Errorf("failed to query PR state: %w", err)
	}
	return ParsePRViewJSON(out)
}

// PostGitHubReview posts a review comment (or approval) to the pull request
// identified by prNumber. When approve is true the review is submitted as an
// approval; otherwise it is posted as a plain comment.
func (g *GitWorktree) PostGitHubReview(prNumber int, body string, approve bool) error {
	if prNumber <= 0 {
		return fmt.Errorf("invalid pr number: %d", prNumber)
	}
	numStr := fmt.Sprintf("%d", prNumber)
	if approve {
		if err := ghRun(g.worktreePath, "pr", "review", numStr, "--approve", "-b", body); err != nil {
			return fmt.Errorf("failed to post github review: %w", err)
		}
		return nil
	}
	if err := ghRun(g.worktreePath, "pr", "review", numStr, "--comment", "-b", body); err != nil {
		return fmt.Errorf("failed to post github review: %w", err)
	}
	return nil
}

// runGitCommand executes a git command with the given path as the working directory.
// It builds the invocation as: git -C <path> <args...>, captures combined stdout+stderr,
// and wraps any error with the captured output for context.
func (g *GitWorktree) runGitCommand(path string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", path}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %s (%w)", out, err)
	}
	return string(out), nil
}

// PushChanges stages, commits, and pushes the worktree's current state to the remote.
// It requires the GitHub CLI (gh) to be present and authenticated.
// When open is true the branch URL is opened in the browser after a successful push.
func (g *GitWorktree) PushChanges(commitMessage string, open bool) error {
	if err := checkGHCLI(); err != nil {
		return err
	}
	if err := g.CommitChanges(commitMessage); err != nil {
		return err
	}
	return g.Push(open)
}

// Push pushes the current branch to origin without committing first.
// If open is true it attempts to open the remote branch URL; any error from
// that step is logged but not returned.
func (g *GitWorktree) Push(open bool) error {
	if _, err := g.runGitCommand(g.worktreePath, "push", "-u", "origin", g.branchName); err != nil {
		return fmt.Errorf("failed to push branch %s: %w", g.branchName, err)
	}
	if open {
		if err := g.OpenBranchURL(); err != nil {
			log.ErrorLog.Printf("failed to open branch URL: %v", err)
		}
	}
	return nil
}

// GeneratePRBody builds a markdown pull-request description that summarises
// the files changed, the commit history, and diff statistics since baseCommitSHA.
// It returns an error if no base commit SHA is available.
func (g *GitWorktree) GeneratePRBody() (string, error) {
	base := g.GetBaseCommitSHA()
	if base == "" {
		return "", fmt.Errorf("no base commit SHA available")
	}

	var sections []string

	// List of files that changed relative to the base commit.
	if files, err := g.runGitCommand(g.worktreePath, "diff", "--name-only", base); err == nil {
		if trimmed := strings.TrimSpace(files); trimmed != "" {
			sections = append(sections, "## Changes\n\n"+trimmed)
		}
	}

	// One-line commit log from base to HEAD.
	if commits, err := g.runGitCommand(g.worktreePath, "log", "--oneline", base+"..HEAD"); err == nil {
		if trimmed := strings.TrimSpace(commits); trimmed != "" {
			sections = append(sections, "## Commits\n\n"+trimmed)
		}
	}

	// Summary statistics of insertions/deletions.
	if stats, err := g.runGitCommand(g.worktreePath, "diff", "--stat", base); err == nil {
		if trimmed := strings.TrimSpace(stats); trimmed != "" {
			sections = append(sections, "## Stats\n\n"+trimmed)
		}
	}

	if len(sections) == 0 {
		return "", nil
	}
	return strings.Join(sections, "\n\n"), nil
}

// CreatePR pushes the current branch and opens a pull request on GitHub.
func (g *GitWorktree) CreatePR(title, body string) error {
	if err := g.requireCleanForPR(); err != nil {
		return err
	}
	head, err := BranchHeadSHA(g.repoPath, g.branchName)
	if err != nil {
		return err
	}
	return g.CreatePRAtSHA(title, body, head)
}

// CreatePRAtSHA pushes exactly expectedSHA and refuses to create the PR if the
// local task branch moved after verification.
func (g *GitWorktree) CreatePRAtSHA(title, body, expectedSHA string) error {
	if err := g.requireCleanForPR(); err != nil {
		return err
	}
	current, err := BranchHeadSHA(g.repoPath, g.branchName)
	if err != nil {
		return err
	}
	if !strings.EqualFold(current, expectedSHA) {
		return fmt.Errorf("branch %s moved after verification: expected %s, current %s", g.branchName, ShortSHA(expectedSHA), ShortSHA(current))
	}
	if _, err := g.runGitCommand(g.worktreePath, "push", "-u", "origin", expectedSHA+":refs/heads/"+g.branchName); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	args := []string{"pr", "create", "--title", title, "--body", body, "--head", g.branchName}
	if base := prBaseBranch(g.repoPath); base != "" {
		args = append(args, "--base", base)
	}

	_, err = ghOutput(g.worktreePath, args...)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("%w: %s", ErrPRAlreadyExists, err)
		}
		return fmt.Errorf("failed to create PR: %w", err)
	}
	return nil
}

// prBaseBranch returns the repo's configured `pr_base_branch`, or "" to leave
// --base off the gh invocation entirely so gh falls back to the GitHub default
// branch (the behavior before this setting existed).
//
// It reads repoPath's own .kasmos/config.toml rather than calling
// config.LoadConfig(), which resolves its directory from os.Getwd(). The daemon
// serves several repos from one process and its cwd is none of them, so a
// cwd-relative load would silently apply the wrong repo's setting — or no
// setting at all.
func prBaseBranch(repoPath string) string {
	if repoPath == "" {
		return ""
	}
	result, err := config.LoadTOMLConfigFrom(filepath.Join(repoPath, ".kasmos", config.TOMLConfigFileName))
	if err != nil || result == nil {
		// Missing or malformed config is not a reason to block a PR; fall back
		// to the historical default-branch behavior.
		return ""
	}
	return strings.TrimSpace(result.PRBaseBranch)
}

// requireCleanForPR prevents PR finalization from absorbing unrelated or
// pre-existing worktree edits. Lifecycle agents must commit their own scoped
// files before signaling completion; PR creation only publishes that history.
func (g *GitWorktree) requireCleanForPR() error {
	status, err := g.runGitCommand(g.worktreePath, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to check worktree before PR creation: %w", err)
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("%w; commit only the intended files and preserve unrelated edits before creating a PR:\n%s", ErrWorktreeDirty, strings.TrimSpace(status))
	}
	return nil
}

// CommitChanges stages all changes and creates a commit with the given message.
// It is a no-op when the worktree is clean.
func (g *GitWorktree) CommitChanges(commitMessage string) error {
	dirty, err := g.IsDirty()
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}
	if !dirty {
		return nil
	}
	if _, err := g.runGitCommand(g.worktreePath, "add", "."); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to stage changes: %w", err)
	}
	if _, err := g.runGitCommand(g.worktreePath, "commit", "-m", commitMessage, "--no-verify"); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	return nil
}

// IsDirty reports whether the worktree contains uncommitted changes.
func (g *GitWorktree) IsDirty() (bool, error) {
	out, err := g.runGitCommand(g.worktreePath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check worktree status: %w", err)
	}
	return len(out) > 0, nil
}

// IsBranchCheckedOut reports whether the configured branch is the currently
// checked-out branch in the repository (not in the worktree).
func (g *GitWorktree) IsBranchCheckedOut() (bool, error) {
	out, err := g.runGitCommand(g.repoPath, "branch", "--show-current")
	if err != nil {
		return false, fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(out) == g.branchName, nil
}

// OpenBranchURL opens the remote branch page in the default browser using the
// GitHub CLI. It requires gh to be present and authenticated.
func (g *GitWorktree) OpenBranchURL() error {
	if err := checkGHCLI(); err != nil {
		return err
	}
	if err := ghRun(g.worktreePath, "browse", "--branch", g.branchName); err != nil {
		return fmt.Errorf("failed to open branch URL: %w", err)
	}
	return nil
}
