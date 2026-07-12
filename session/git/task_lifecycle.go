package git

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kastheco/kasmos/config/taskstate"
)

// ErrBranchNotFound reports that a task branch does not exist. Drift detection
// uses errors.Is to distinguish "merged and deleted, skip" from "git is broken".
var ErrBranchNotFound = errors.New("task branch not found")

// BranchHeadSHA returns the full 40-char commit SHA that branch points at.
func BranchHeadSHA(repoPath, branch string) (string, error) {
	gt := &GitWorktree{repoPath: repoPath, worktreePath: repoPath}
	if _, err := gt.runGitCommand(repoPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", fmt.Errorf("branch head %s: %w", branch, ErrBranchNotFound)
		}
		return "", fmt.Errorf("check branch head %s: %w", branch, err)
	}
	out, err := gt.runGitCommand(repoPath, "rev-parse", "--verify", branch+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve branch head %s: %w", branch, err)
	}
	sha := strings.TrimSpace(out)
	if sha == "" {
		return "", fmt.Errorf("branch head %s: empty commit SHA", branch)
	}
	return sha, nil
}

// RemoteBranchHeadSHA resolves the authoritative origin branch without relying
// on a potentially stale remote-tracking ref.
func RemoteBranchHeadSHA(repoPath, branch string) (string, error) {
	gt := &GitWorktree{repoPath: repoPath, worktreePath: repoPath}
	out, err := gt.runGitCommand(repoPath, "ls-remote", "--heads", "origin", "refs/heads/"+branch)
	if err != nil {
		return "", fmt.Errorf("resolve remote branch %s: %w", branch, err)
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", ErrBranchNotFound
	}
	if len(fields) < 2 || fields[1] != "refs/heads/"+branch {
		return "", fmt.Errorf("unexpected ls-remote output for branch %s", branch)
	}
	return fields[0], nil
}

// RemoteDefaultBranchHeadSHA resolves the default branch on origin.
func RemoteDefaultBranchHeadSHA(repoPath string) (string, error) {
	gt := &GitWorktree{repoPath: repoPath, worktreePath: repoPath}
	ref, err := defaultBranchRef(gt, repoPath)
	if err != nil {
		return "", err
	}
	return RemoteBranchHeadSHA(repoPath, strings.TrimPrefix(ref, "origin/"))
}

// BranchMergeBaseSHA returns the merge-base of the repository's default branch
// and branch. It does not depend on whichever branch the root worktree happens
// to have checked out.
func BranchMergeBaseSHA(repoPath, branch string) (string, error) {
	gt := &GitWorktree{repoPath: repoPath, worktreePath: repoPath}
	base, err := defaultBranchRef(gt, repoPath)
	if err != nil {
		return "", err
	}
	out, err := gt.runGitCommand(repoPath, "merge-base", base, branch)
	if err != nil {
		return "", fmt.Errorf("merge-base %s %s: %w", base, branch, err)
	}
	sha := strings.TrimSpace(out)
	if sha == "" {
		return "", fmt.Errorf("merge-base %s %s: empty commit SHA", base, branch)
	}
	return sha, nil
}

// DefaultBranchHeadSHA returns the current commit of the repository's default
// branch. Verification binds this separately from the task branch head.
func DefaultBranchHeadSHA(repoPath string) (string, error) {
	gt := &GitWorktree{repoPath: repoPath, worktreePath: repoPath}
	base, err := defaultBranchRef(gt, repoPath)
	if err != nil {
		return "", err
	}
	out, err := gt.runGitCommand(repoPath, "rev-parse", "--verify", base+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve default branch head %s: %w", base, err)
	}
	return strings.TrimSpace(out), nil
}

func defaultBranchRef(gt *GitWorktree, repoPath string) (string, error) {
	base := ""
	if out, err := gt.runGitCommand(repoPath, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		base = strings.TrimSpace(out)
	}
	if base == "" {
		for _, candidate := range []string{"main", "master"} {
			if _, err := gt.runGitCommand(repoPath, "rev-parse", "--verify", candidate+"^{commit}"); err == nil {
				base = candidate
				break
			}
		}
	}
	if base == "" {
		return "", fmt.Errorf("resolve default branch: neither origin/HEAD, main, nor master exists")
	}
	return base, nil
}

// ShortSHA returns the first 7 chars of sha, or "" when sha is empty.
func ShortSHA(sha string) string {
	if len(sha) < 7 {
		return sha
	}
	return sha[:7]
}

// TaskBranchFromFile derives the git branch name from a plan filename.
// "auth-refactor" → "plan/auth-refactor"
func TaskBranchFromFile(planFile string) string {
	name := taskstate.DisplayName(planFile)
	name = sanitizeBranchName(name)
	if name == "" {
		name = "plan"
	}
	return "plan/" + name
}

// TaskWorktreePath returns the worktree path for a plan branch.
// The branch separator "/" is replaced with "-" to form a valid directory name.
func TaskWorktreePath(repoPath, branch string) string {
	safe := strings.ReplaceAll(branch, "/", "-")
	return filepath.Join(repoPath, ".worktrees", safe)
}

// NewSharedTaskWorktree constructs a GitWorktree for the shared plan worktree
// (used by coder and reviewer sessions that share the same branch).
func NewSharedTaskWorktree(repoPath, branch string) *GitWorktree {
	return NewGitWorktreeFromStorage(
		repoPath,
		TaskWorktreePath(repoPath, branch),
		"plan-shared",
		branch,
		"",
	)
}

// TaskBranchExists reports whether branch resolves locally (refs/heads/<branch>)
// or on the remote (refs/remotes/origin/<branch>). Local refs only; no network.
func TaskBranchExists(repoPath, branch string) (local bool, remote bool) {
	gt := &GitWorktree{repoPath: repoPath, worktreePath: repoPath}
	_, localErr := gt.runGitCommand(repoPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	_, remoteErr := gt.runGitCommand(repoPath, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	return localErr == nil, remoteErr == nil
}

// EnsureTaskWorktree returns a ready shared task worktree for branch, with its
// base commit SHA resolved. It never fabricates a branch: if branch exists only
// on origin, the local ref is created from origin/<branch> first, because
// GitWorktree.Setup() detects branches through refs/heads/<branch> only and would
// otherwise cut a new branch off HEAD (see setupNewWorktree).
func EnsureTaskWorktree(repoPath, branch string) (*GitWorktree, error) {
	if branch == "" {
		return nil, fmt.Errorf("no branch to create a worktree from")
	}

	local, remote := TaskBranchExists(repoPath, branch)
	if !local && !remote {
		return nil, fmt.Errorf("branch '%s' no longer exists locally or on origin", branch)
	}

	gt := &GitWorktree{repoPath: repoPath, worktreePath: repoPath}
	if !local {
		if _, err := gt.runGitCommand(repoPath, "branch", "--track", branch, "origin/"+branch); err != nil {
			return nil, fmt.Errorf("restore branch '%s' from origin: %w", branch, err)
		}
	}

	path := TaskWorktreePath(repoPath, branch)
	out, err := gt.runGitCommand(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("inspect worktree registrations: %w", err)
	}
	var currentPath string
	var registration string
	checkRegistration := func() error {
		if currentPath != filepath.Clean(path) {
			return nil
		}
		if registration != "branch refs/heads/"+branch {
			if strings.HasPrefix(registration, "branch refs/heads/") {
				registeredBranch := strings.TrimPrefix(registration, "branch refs/heads/")
				return fmt.Errorf("worktree path %s is registered to branch '%s'", path, registeredBranch)
			}
			return fmt.Errorf("worktree path %s is registered as '%s'", path, registration)
		}
		return nil
	}
	for _, line := range strings.Split(out, "\n") {
		switch {
		case line == "":
			if err := checkRegistration(); err != nil {
				return nil, err
			}
			currentPath = ""
			registration = ""
		case strings.HasPrefix(line, "worktree "):
			currentPath = filepath.Clean(strings.TrimSpace(strings.TrimPrefix(line, "worktree ")))
		case strings.HasPrefix(line, "branch "):
			registration = strings.TrimSpace(line)
		case line == "detached" || line == "bare":
			registration = line
		}
	}

	wt := NewSharedTaskWorktree(repoPath, branch)
	if err := wt.Setup(); err != nil {
		return nil, fmt.Errorf("set up task worktree for branch '%s': %w", branch, err)
	}
	return wt, nil
}

// EnsureTaskBranch creates the plan branch off the current HEAD if it doesn't
// already exist. It is idempotent.
func EnsureTaskBranch(repoPath, branch string) error {
	gt := &GitWorktree{repoPath: repoPath, worktreePath: repoPath}
	if _, err := gt.runGitCommand(repoPath, "rev-parse", "--verify", branch); err == nil {
		return nil // already exists
	}
	if _, err := gt.runGitCommand(repoPath, "branch", branch); err != nil {
		return fmt.Errorf("create plan branch %s: %w", branch, err)
	}
	return nil
}

// MergeTaskBranch merges the plan branch into the current branch (typically main),
// removes the worktree, and deletes the plan branch.
func MergeTaskBranch(repoPath, branch string) error {
	head, err := BranchHeadSHA(repoPath, branch)
	if err != nil {
		return err
	}
	base, err := DefaultBranchHeadSHA(repoPath)
	if err != nil {
		return err
	}
	return MergeTaskBranchAtSHA(repoPath, branch, head, base)
}

// MergeTaskBranchAtSHA merges exactly expectedSHA and refuses the operation if
// either the task branch or target branch moved after verification.
func MergeTaskBranchAtSHA(repoPath, branch, expectedSHA, expectedBaseSHA string) error {
	gt := &GitWorktree{repoPath: repoPath, worktreePath: repoPath}
	worktreePath := TaskWorktreePath(repoPath, branch)
	targetHead, err := gt.runGitCommand(repoPath, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return fmt.Errorf("resolve current merge target: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(targetHead), expectedBaseSHA) {
		return fmt.Errorf("current merge target differs from verified base: expected %s, current %s", ShortSHA(expectedBaseSHA), ShortSHA(strings.TrimSpace(targetHead)))
	}

	// Remove worktree first so the branch isn't "checked out" elsewhere.
	_, _ = gt.runGitCommand(repoPath, "worktree", "remove", "-f", worktreePath)
	_, _ = gt.runGitCommand(repoPath, "worktree", "prune")

	// Ensure the local branch exists — worktree removal may have deleted it.
	// If a remote tracking branch exists, recreate the local one from it.
	if _, err := gt.runGitCommand(repoPath, "rev-parse", "--verify", branch); err != nil {
		remote := "origin/" + branch
		if _, remoteErr := gt.runGitCommand(repoPath, "rev-parse", "--verify", remote); remoteErr == nil {
			if _, brErr := gt.runGitCommand(repoPath, "branch", branch, remote); brErr != nil {
				return fmt.Errorf("recreate local branch %s from remote: %w", branch, brErr)
			}
		} else {
			return fmt.Errorf("branch %s not found locally or on remote", branch)
		}
	}
	current, err := BranchHeadSHA(repoPath, branch)
	if err != nil {
		return err
	}
	if !strings.EqualFold(current, expectedSHA) {
		return fmt.Errorf("branch %s moved after verification: expected %s, current %s", branch, ShortSHA(expectedSHA), ShortSHA(current))
	}
	base, err := DefaultBranchHeadSHA(repoPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(base, expectedBaseSHA) {
		return fmt.Errorf("default branch moved after verification: expected %s, current %s", ShortSHA(expectedBaseSHA), ShortSHA(base))
	}

	// Merge the verified commit object, not a movable branch name.
	if _, err := gt.runGitCommand(repoPath, "merge", expectedSHA, "--no-ff", "-m",
		fmt.Sprintf("merge plan branch %s", branch)); err != nil {
		return fmt.Errorf("merge %s: %w", branch, err)
	}

	// Delete the plan branch after successful merge.
	if _, err := gt.runGitCommand(repoPath, "branch", "-d", branch); err != nil {
		// Non-fatal: merge succeeded, branch cleanup is best-effort.
		_ = err
	}

	return nil
}

// PreflightMergeTaskBranch checks whether the current branch can safely merge
// the task branch without clobbering local uncommitted changes in the repo
// worktree. It only blocks when dirty paths overlap files changed by the
// incoming branch.
func PreflightMergeTaskBranch(repoPath, branch string) error {
	gt := &GitWorktree{repoPath: repoPath, worktreePath: repoPath}

	statusOut, err := gt.runGitCommand(repoPath, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("preflight merge %s: check worktree status: %w", branch, err)
	}

	dirtyPaths := dirtyPathsFromPorcelain(statusOut)
	if len(dirtyPaths) == 0 {
		return nil
	}

	changedOut, err := gt.runGitCommand(repoPath, "diff", "--name-only", "HEAD..."+branch)
	if err != nil {
		return fmt.Errorf("preflight merge %s: list changed files: %w", branch, err)
	}

	changedPaths := pathSetFromLines(changedOut)
	if len(changedPaths) == 0 {
		return nil
	}

	overlap := intersectPathSets(dirtyPaths, changedPaths)
	if len(overlap) == 0 {
		return nil
	}

	preview := overlap
	if len(preview) > 5 {
		preview = preview[:5]
	}
	suffix := ""
	if len(overlap) > len(preview) {
		suffix = fmt.Sprintf(" (+%d more)", len(overlap)-len(preview))
	}

	return fmt.Errorf(
		"cannot merge %s: uncommitted changes overlap with incoming branch (%s%s); commit or stash first",
		branch,
		strings.Join(preview, ", "),
		suffix,
	)
}

func dirtyPathsFromPorcelain(statusOut string) map[string]struct{} {
	paths := make(map[string]struct{})
	for _, line := range strings.Split(statusOut, "\n") {
		if line == "" {
			continue
		}
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if arrow := strings.Index(path, " -> "); arrow >= 0 {
			path = path[arrow+4:]
		}
		if path != "" {
			paths[path] = struct{}{}
		}
	}
	return paths
}

func pathSetFromLines(out string) map[string]struct{} {
	paths := make(map[string]struct{})
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			paths[line] = struct{}{}
		}
	}
	return paths
}

func intersectPathSets(a, b map[string]struct{}) []string {
	var overlap []string
	for path := range a {
		if _, ok := b[path]; ok {
			overlap = append(overlap, path)
		}
	}
	sort.Strings(overlap)
	return overlap
}

// ResetTaskBranch removes the plan worktree (if any), deletes the branch, and
// recreates it from the current HEAD. Used by "start over".
func ResetTaskBranch(repoPath, branch string) error {
	gt := &GitWorktree{repoPath: repoPath, worktreePath: repoPath}
	worktreePath := TaskWorktreePath(repoPath, branch)
	_, _ = gt.runGitCommand(repoPath, "worktree", "remove", "-f", worktreePath)
	_, _ = gt.runGitCommand(repoPath, "branch", "-D", branch)
	if _, err := gt.runGitCommand(repoPath, "branch", branch); err != nil {
		return fmt.Errorf("recreate plan branch %s: %w", branch, err)
	}
	if _, err := gt.runGitCommand(repoPath, "worktree", "prune"); err != nil {
		return fmt.Errorf("prune worktrees: %w", err)
	}
	return nil
}
