package daemon

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration/loop"
	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/stretchr/testify/require"
)

func TestCheckVerificationDrift(t *testing.T) {
	repo := t.TempDir()
	runGit := func(args ...string) {
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
	}
	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("x\n"), 0o644))
	runGit("add", "tracked.txt")
	runGit("commit", "-m", "initial")
	runGit("branch", "plan/drift")
	verified, err := gitpkg.BranchHeadSHA(repo, "plan/drift")
	require.NoError(t, err)

	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("project", taskstore.TaskEntry{Filename: "drift", Status: taskstore.StatusDone}))
	require.NoError(t, store.SetVerification("project", "drift", taskstore.VerificationRecord{SHA: verified}))
	d := &Daemon{logger: slog.Default()}
	e := RepoEntry{Path: repo, Project: "project", Store: store}
	require.Empty(t, d.checkVerificationDrift(context.Background(), e))
	require.Empty(t, d.checkVerificationDrift(context.Background(), e), "unchanged verification must not churn")

	runGit("checkout", "plan/drift")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("y\n"), 0o644))
	runGit("commit", "-am", "drift")
	actions := d.checkVerificationDrift(context.Background(), e)
	require.Len(t, actions, 3)
	require.IsType(t, loop.StaleVerificationAction{}, actions[0])
	require.IsType(t, loop.PausePlanAgentAction{}, actions[1])
	require.IsType(t, loop.SpawnMasterAction{}, actions[2])
}

// TestCheckVerificationDriftIgnoresUnpushedVerifiedHead pins the difference
// between the two ways origin can disagree with the verified SHA.
//
// The remote comparison used to be a bare `remoteHead != VerifiedSHA`, which has
// no direction in it. Agents commit in their worktree and push a moment later, so
// "verified a commit origin has not seen yet" is the normal state of a task for a
// few seconds -- and it was being reported as head_changed_after_verification,
// clearing the verification and sending the task back for another full round. It
// is what put mvp-03 back to verifying at cycle 4 with nothing about the branch
// changed. Origin genuinely *ahead* is real drift and must still fire, so both
// directions are asserted here; a fix that just deleted the check would pass half
// of this test.
func TestCheckVerificationDriftIgnoresUnpushedVerifiedHead(t *testing.T) {
	origin := t.TempDir()
	requireGit := func(dir string, args ...string) {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
	}
	requireGit(origin, "init", "--bare")

	repo := t.TempDir()
	requireGit(repo, "init")
	requireGit(repo, "config", "user.email", "test@example.com")
	requireGit(repo, "config", "user.name", "Test User")
	requireGit(repo, "remote", "add", "origin", origin)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("x\n"), 0o644))
	requireGit(repo, "add", "tracked.txt")
	requireGit(repo, "commit", "-m", "initial")
	requireGit(repo, "checkout", "-b", "plan/drift")
	requireGit(repo, "push", "origin", "plan/drift")

	pushed, err := gitpkg.BranchHeadSHA(repo, "plan/drift")
	require.NoError(t, err)

	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("project", taskstore.TaskEntry{Filename: "drift", Status: taskstore.StatusDone}))
	require.NoError(t, store.SetPRURL("project", "drift", "https://example.test/pr/1"))
	require.NoError(t, store.SetVerification("project", "drift", taskstore.VerificationRecord{SHA: pushed}))

	d := &Daemon{logger: slog.Default()}
	e := RepoEntry{Path: repo, Project: "project", Store: store}
	require.Empty(t, d.checkVerificationDrift(context.Background(), e), "remote in sync must not drift")

	// The mvp-03 shape: verifier approved a commit that exists only locally,
	// because the push has not happened yet. Origin is behind, not different.
	require.NoError(t, os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("y\n"), 0o644))
	requireGit(repo, "commit", "-am", "fix: address review feedback (round 4)")
	unpushed, err := gitpkg.BranchHeadSHA(repo, "plan/drift")
	require.NoError(t, err)
	require.NotEqual(t, pushed, unpushed)
	require.NoError(t, store.SetVerification("project", "drift", taskstore.VerificationRecord{SHA: unpushed}))
	require.Empty(t, d.checkVerificationDrift(context.Background(), e),
		"a verified commit that has not been pushed yet is a pending push, not drift")

	// Control: origin ahead of what was verified is a commit nobody reviewed,
	// and must still reopen the task.
	requireGit(repo, "push", "origin", "plan/drift")
	other := t.TempDir()
	requireGit(other, "clone", origin, other)
	requireGit(other, "config", "user.email", "other@example.com")
	requireGit(other, "config", "user.name", "Other User")
	requireGit(other, "checkout", "plan/drift")
	require.NoError(t, os.WriteFile(filepath.Join(other, "tracked.txt"), []byte("z\n"), 0o644))
	requireGit(other, "commit", "-am", "somebody else pushed after verification")
	requireGit(other, "push", "origin", "plan/drift")

	actions := d.checkVerificationDrift(context.Background(), e)
	require.Len(t, actions, 3, "origin ahead of the verified commit is real drift")
	require.IsType(t, loop.StaleVerificationAction{}, actions[0])
}
