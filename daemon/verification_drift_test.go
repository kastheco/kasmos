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
