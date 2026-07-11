package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskBranchFromFile(t *testing.T) {
	got := TaskBranchFromFile("auth-refactor")
	want := "plan/auth-refactor"
	if got != want {
		t.Fatalf("TaskBranchFromFile() = %q, want %q", got, want)
	}
}

func TestTaskWorktreePath(t *testing.T) {
	repo := "/tmp/repo"
	branch := "plan/auth-refactor"
	got := TaskWorktreePath(repo, branch)
	want := filepath.Join(repo, ".worktrees", "plan-auth-refactor")
	if got != want {
		t.Fatalf("TaskWorktreePath() = %q, want %q", got, want)
	}
}

func TestNewSharedTaskWorktree(t *testing.T) {
	repo := "/tmp/repo"
	branch := "plan/auth-refactor"
	gt := NewSharedTaskWorktree(repo, branch)

	if gt.GetRepoPath() != repo {
		t.Fatalf("repo = %q, want %q", gt.GetRepoPath(), repo)
	}
	if gt.GetWorktreePath() != filepath.Join(repo, ".worktrees", "plan-auth-refactor") {
		t.Fatalf("unexpected worktree path %q", gt.GetWorktreePath())
	}
	if gt.GetBranchName() != branch {
		t.Fatalf("branch = %q, want %q", gt.GetBranchName(), branch)
	}
}

func TestSetupFromExistingBranch_SetsBaseCommitSHA(t *testing.T) {
	repo := initTestRepo(t)

	cmd := exec.Command("git", "-C", repo, "branch", "plan/test-base")
	require.NoError(t, cmd.Run())

	gt := NewSharedTaskWorktree(repo, "plan/test-base")
	require.NoError(t, gt.Setup())
	t.Cleanup(func() { _ = gt.Cleanup() })

	assert.NotEmpty(t, gt.GetBaseCommitSHA(), "baseCommitSHA should be set after Setup")
}

func TestSetupFromExistingBranch_ReusesRegisteredWorktree(t *testing.T) {
	repo := initTestRepo(t)

	cmd := exec.Command("git", "-C", repo, "branch", "plan/test-reuse")
	require.NoError(t, cmd.Run())

	gt := NewSharedTaskWorktree(repo, "plan/test-reuse")
	require.NoError(t, gt.Setup())
	t.Cleanup(func() { _ = gt.Cleanup() })

	marker := filepath.Join(gt.GetWorktreePath(), "marker.txt")
	require.NoError(t, os.WriteFile(marker, []byte("keep\n"), 0o644))

	require.NoError(t, gt.Setup())
	assert.FileExists(t, marker, "matching shared worktree should be reused, not recreated")
	assert.NotEmpty(t, gt.GetBaseCommitSHA(), "baseCommitSHA should remain available after reuse")
}

func TestTaskBranchExists(t *testing.T) {
	repo := initTestRepo(t)
	runGitAt(t, repo, "branch", "plan/local")
	sha := runGitOutput(t, repo, "rev-parse", "HEAD")
	runGitAt(t, repo, "update-ref", "refs/remotes/origin/plan/remote", sha)

	tests := []struct {
		name       string
		branch     string
		wantLocal  bool
		wantRemote bool
	}{
		{name: "local", branch: "plan/local", wantLocal: true},
		{name: "remote", branch: "plan/remote", wantRemote: true},
		{name: "missing", branch: "plan/missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local, remote := TaskBranchExists(repo, tt.branch)
			assert.Equal(t, tt.wantLocal, local)
			assert.Equal(t, tt.wantRemote, remote)
		})
	}
}

func TestEnsureTaskWorktree(t *testing.T) {
	t.Run("local branch", func(t *testing.T) {
		repo := initTestRepo(t)
		branch := "plan/local"
		runGitAt(t, repo, "branch", branch)

		wt, err := EnsureTaskWorktree(repo, branch)
		require.NoError(t, err)
		t.Cleanup(func() { _ = wt.Cleanup() })
		assert.DirExists(t, wt.GetWorktreePath())
		assert.NotEmpty(t, wt.GetBaseCommitSHA())
	})

	t.Run("registered worktree is reused", func(t *testing.T) {
		repo := initTestRepo(t)
		branch := "plan/reuse"
		runGitAt(t, repo, "branch", branch)
		first, err := EnsureTaskWorktree(repo, branch)
		require.NoError(t, err)
		t.Cleanup(func() { _ = first.Cleanup() })
		marker := filepath.Join(first.GetWorktreePath(), "marker.txt")
		require.NoError(t, os.WriteFile(marker, []byte("keep\n"), 0o644))

		wt, err := EnsureTaskWorktree(repo, branch)
		require.NoError(t, err)
		assert.FileExists(t, marker)
		assert.NotEmpty(t, wt.GetBaseCommitSHA())
	})

	t.Run("remote-only branch is restored at remote commit", func(t *testing.T) {
		repo := initTestRepo(t)
		branch := "plan/remote"
		runGitAt(t, repo, "checkout", "-b", branch)
		require.NoError(t, os.WriteFile(filepath.Join(repo, "remote.txt"), []byte("remote\n"), 0o644))
		runGitAt(t, repo, "add", "remote.txt")
		runGitAt(t, repo, "commit", "-m", "remote commit")
		remoteSHA := runGitOutput(t, repo, "rev-parse", "HEAD")
		runGitAt(t, repo, "checkout", "-")
		headSHA := runGitOutput(t, repo, "rev-parse", "HEAD")
		require.NotEqual(t, headSHA, remoteSHA)
		runGitAt(t, repo, "remote", "add", "origin", repo)
		runGitAt(t, repo, "update-ref", "refs/remotes/origin/"+branch, remoteSHA)
		runGitAt(t, repo, "branch", "-D", branch)

		wt, err := EnsureTaskWorktree(repo, branch)
		require.NoError(t, err)
		t.Cleanup(func() { _ = wt.Cleanup() })
		assert.Equal(t, remoteSHA, runGitOutput(t, wt.GetWorktreePath(), "rev-parse", "HEAD"))
	})

	t.Run("missing branch is not fabricated", func(t *testing.T) {
		repo := initTestRepo(t)
		branch := "plan/missing"
		path := TaskWorktreePath(repo, branch)

		_, err := EnsureTaskWorktree(repo, branch)
		require.EqualError(t, err, "branch 'plan/missing' no longer exists locally or on origin")
		cmd := exec.Command("git", "-C", repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
		require.Error(t, cmd.Run())
		assert.NoDirExists(t, path)
	})

	t.Run("different registered branch is preserved", func(t *testing.T) {
		repo := initTestRepo(t)
		branch := "plan/target"
		other := "plan/other"
		path := TaskWorktreePath(repo, branch)
		runGitAt(t, repo, "branch", branch)
		runGitAt(t, repo, "branch", other)
		runGitAt(t, repo, "worktree", "add", path, other)
		t.Cleanup(func() { runGitAt(t, repo, "worktree", "remove", "-f", path) })
		marker := filepath.Join(path, "marker.txt")
		require.NoError(t, os.WriteFile(marker, []byte("keep\n"), 0o644))

		_, err := EnsureTaskWorktree(repo, branch)
		require.EqualError(t, err, fmt.Sprintf("worktree path %s is registered to branch 'plan/other'", path))
		assert.FileExists(t, marker)
	})

	t.Run("detached registered worktree is preserved", func(t *testing.T) {
		repo := initTestRepo(t)
		branch := "plan/target"
		path := TaskWorktreePath(repo, branch)
		runGitAt(t, repo, "branch", branch)
		runGitAt(t, repo, "worktree", "add", path, branch)
		t.Cleanup(func() { runGitAt(t, repo, "worktree", "remove", "-f", path) })
		runGitAt(t, path, "checkout", "--detach")
		marker := filepath.Join(path, "marker.txt")
		require.NoError(t, os.WriteFile(marker, []byte("keep\n"), 0o644))

		_, err := EnsureTaskWorktree(repo, branch)
		require.EqualError(t, err, fmt.Sprintf("worktree path %s is registered as 'detached'", path))
		assert.FileExists(t, marker)
	})
}

func runGitAt(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}

func runGitOutput(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	return strings.TrimSpace(string(out))
}

func TestPreflightMergeTaskBranch_BlocksOverlappingDirtyPaths(t *testing.T) {
	repo := initTestRepo(t)
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	}

	runGit("checkout", "-b", "plan/test-merge")
	readmePath := filepath.Join(repo, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("branch change\n"), 0o644))
	runGit("add", "README.md")
	runGit("commit", "-m", "branch change")
	runGit("checkout", "-")
	require.NoError(t, os.WriteFile(readmePath, []byte("local dirty change\n"), 0o644))

	err := PreflightMergeTaskBranch(repo, "plan/test-merge")
	require.Error(t, err)
	assert.ErrorContains(t, err, "uncommitted changes overlap")
	assert.ErrorContains(t, err, "README.md")
}

func TestPreflightMergeTaskBranch_AllowsUnrelatedDirtyPaths(t *testing.T) {
	repo := initTestRepo(t)
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	}

	runGit("checkout", "-b", "plan/test-merge")
	readmePath := filepath.Join(repo, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("branch change\n"), 0o644))
	runGit("add", "README.md")
	runGit("commit", "-m", "branch change")
	runGit("checkout", "-")

	notesPath := filepath.Join(repo, "notes.txt")
	require.NoError(t, os.WriteFile(notesPath, []byte("local dirty note\n"), 0o644))

	require.NoError(t, PreflightMergeTaskBranch(repo, "plan/test-merge"))
}

func initTestRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "test")

	readmePath := filepath.Join(repo, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("initial\n"), 0644))

	runGit("add", "README.md")
	runGit("commit", "-m", "initial commit")

	return repo
}
