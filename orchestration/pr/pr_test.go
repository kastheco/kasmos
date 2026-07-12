package pr

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEligibleIncludesBranchlessDoneTask(t *testing.T) {
	t.Parallel()
	assert.True(t, Eligible(taskstore.TaskEntry{Status: taskstore.StatusDone}))
	assert.False(t, Eligible(taskstore.TaskEntry{Status: taskstore.StatusReviewing}))
	assert.True(t, Eligible(taskstore.TaskEntry{Status: taskstore.StatusDone, PRURL: "https://example.test/pr/1"}))
}

type scriptedGHExecutor struct {
	calls        [][]string
	queryOutput  []byte
	queryErr     error
	queryStderr  string
	createErr    error
	createStderr string
	created      bool
}

func (f *scriptedGHExecutor) record(cmd *exec.Cmd) []string {
	args := append([]string(nil), cmd.Args[1:]...)
	f.calls = append(f.calls, args)
	return args
}

func (f *scriptedGHExecutor) Run(cmd *exec.Cmd) error {
	f.record(cmd)
	return nil
}

func (f *scriptedGHExecutor) Output(cmd *exec.Cmd) ([]byte, error) {
	args := f.record(cmd)
	if len(args) >= 2 && args[0] == "pr" && args[1] == "create" {
		if f.createStderr != "" {
			_, _ = io.WriteString(cmd.Stderr, f.createStderr)
		}
		if f.createErr == nil {
			f.created = true
		}
		return nil, f.createErr
	}
	if len(args) >= 2 && args[0] == "pr" && args[1] == "view" {
		if f.created && f.queryOutput != nil {
			return f.queryOutput, nil
		}
		if f.queryStderr != "" {
			_, _ = io.WriteString(cmd.Stderr, f.queryStderr)
		}
		return f.queryOutput, f.queryErr
	}
	return nil, nil
}

func (f *scriptedGHExecutor) countCall(prefix string) int {
	count := 0
	for _, call := range f.calls {
		if strings.HasPrefix(strings.Join(call, " "), prefix) {
			count++
		}
	}
	return count
}

func newEnsureRepo(t *testing.T, branch string) string {
	t.Helper()
	repo := t.TempDir()
	origin := filepath.Join(t.TempDir(), "origin.git")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, out)
	}
	require.NoError(t, exec.Command("git", "init", "--bare", origin).Run())
	run(repo, "init", "-b", "main")
	run(repo, "config", "user.email", "test@example.com")
	run(repo, "config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "README.md"), []byte("initial\n"), 0o644))
	run(repo, "add", "README.md")
	run(repo, "commit", "-m", "initial")
	run(repo, "remote", "add", "origin", origin)
	run(repo, "push", "-u", "origin", "main")
	run(repo, "branch", branch)
	return repo
}

func createDoneTask(t *testing.T, store taskstore.Store, branch string) Request {
	t.Helper()
	req := Request{RepoPath: newEnsureRepo(t, branch), Project: "test", PlanFile: "plan", Enabled: true}
	require.NoError(t, store.Create(req.Project, taskstore.TaskEntry{Filename: req.PlanFile, Status: taskstore.StatusDone, Branch: branch, Description: "test pr"}))
	head, err := gitpkg.BranchHeadSHA(req.RepoPath, branch)
	require.NoError(t, err)
	base, err := gitpkg.BranchMergeBaseSHA(req.RepoPath, branch)
	require.NoError(t, err)
	require.NoError(t, store.SetVerification(req.Project, req.PlanFile, taskstore.VerificationRecord{SHA: head, BaseSHA: base, By: "master"}))
	return req
}

func prJSON(url, head string) []byte {
	return []byte(`{"url":"` + url + `","reviewDecision":"","statusCheckRollup":{"state":""},"isDraft":false,"number":7,"headRefOid":"` + head + `"}`)
}

func verifiedHead(t *testing.T, store taskstore.Store, req Request) string {
	t.Helper()
	entry, err := store.Get(req.Project, req.PlanFile)
	require.NoError(t, err)
	return entry.VerifiedSHA
}

func TestEnsureCreatedOnceThenRecordedURLSkipped(t *testing.T) {
	store := taskstore.NewTestStore(t)
	req := createDoneTask(t, store, "plan/test")
	fake := &scriptedGHExecutor{queryOutput: prJSON("https://example.test/pr/7", verifiedHead(t, store, req)), queryErr: errors.New("exit status 1"), queryStderr: "no pull requests found"}
	t.Cleanup(gitpkg.SetGHExec(fake))

	first, err := Ensure(context.Background(), store, req)
	require.NoError(t, err)
	assert.Equal(t, OutcomeCreated, first.Outcome)
	createdEntry, err := store.Get(req.Project, req.PlanFile)
	require.NoError(t, err)
	assert.Equal(t, string(OutcomeCreated), createdEntry.PRCreateState)
	assert.Empty(t, createdEntry.PRCreateError)
	assert.Equal(t, 1, createdEntry.PRCreateAttempts)
	second, err := Ensure(context.Background(), store, req)
	require.NoError(t, err)
	assert.Equal(t, OutcomeSkipped, second.Outcome)
	assert.Equal(t, "pr already recorded", second.Reason)
	assert.Equal(t, first.URL, second.URL)
	assert.Equal(t, 1, fake.countCall("pr create"))
	entry, err := store.Get(req.Project, req.PlanFile)
	require.NoError(t, err)
	assert.Equal(t, first.URL, entry.PRURL)
	assert.Equal(t, string(OutcomeSkipped), entry.PRCreateState)
}

func TestEnsureAdoptsExistingPRWithoutCreatingWorktree(t *testing.T) {
	store := taskstore.NewTestStore(t)
	req := createDoneTask(t, store, "plan/adopt")
	fake := &scriptedGHExecutor{queryOutput: prJSON("https://example.test/pr/7", verifiedHead(t, store, req))}
	t.Cleanup(gitpkg.SetGHExec(fake))

	res, err := Ensure(context.Background(), store, req)
	require.NoError(t, err)
	assert.Equal(t, OutcomeAdopted, res.Outcome)
	assert.Equal(t, 0, fake.countCall("pr create"))
	assert.NoDirExists(t, gitpkg.TaskWorktreePath(req.RepoPath, "plan/adopt"))
	entry, err := store.Get(req.Project, req.PlanFile)
	require.NoError(t, err)
	assert.Equal(t, res.URL, entry.PRURL)
	assert.Equal(t, string(OutcomeAdopted), entry.PRCreateState)
	assert.Empty(t, entry.PRCreateError)
	assert.Equal(t, 1, entry.PRCreateAttempts)
}

func TestEnsureRejectsExistingPRAtUnverifiedHead(t *testing.T) {
	store := taskstore.NewTestStore(t)
	req := createDoneTask(t, store, "plan/adopt-stale")
	fake := &scriptedGHExecutor{queryOutput: prJSON("https://example.test/pr/7", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")}
	t.Cleanup(gitpkg.SetGHExec(fake))

	res, err := Ensure(context.Background(), store, req)
	require.NoError(t, err)
	assert.Equal(t, OutcomeBlocked, res.Outcome)
	assert.Contains(t, res.Reason, "pull request head moved")
	entry, getErr := store.Get(req.Project, req.PlanFile)
	require.NoError(t, getErr)
	assert.Empty(t, entry.VerifiedSHA)
}

func TestEnsureRejectsRemoteOnlyBranchAdvance(t *testing.T) {
	store := taskstore.NewTestStore(t)
	req := createDoneTask(t, store, "plan/remote-stale")
	run := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", req.RepoPath}, args...)...).CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, out)
	}
	run("checkout", "-b", "remote-advance", "plan/remote-stale")
	run("commit", "--allow-empty", "-m", "external advance")
	run("push", "origin", "HEAD:refs/heads/plan/remote-stale")
	run("checkout", "main")
	fake := &scriptedGHExecutor{}
	t.Cleanup(gitpkg.SetGHExec(fake))

	res, err := Ensure(context.Background(), store, req)
	require.NoError(t, err)
	assert.Equal(t, OutcomeBlocked, res.Outcome)
	assert.Contains(t, res.Reason, "remote branch moved")
	assert.Empty(t, fake.calls, "GitHub must not be queried after remote drift")
}

func TestEnsureClassifiesTransientDirtyAndEmptyURL(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, Request)
		fake    *scriptedGHExecutor
		outcome Outcome
		reason  string
	}{
		{name: "transient", fake: &scriptedGHExecutor{queryErr: errors.New("exit status 1"), queryStderr: "no pull requests found", createErr: errors.New("network unavailable")}, outcome: OutcomeFailed, reason: "network unavailable"},
		{name: "dirty", prepare: func(t *testing.T, req Request) {
			wt, err := gitpkg.EnsureTaskWorktree(req.RepoPath, "plan/test")
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(wt.GetWorktreePath(), "dirty.txt"), []byte("dirty\n"), 0o644))
		}, fake: &scriptedGHExecutor{queryErr: errors.New("exit status 1"), queryStderr: "no pull requests found"}, outcome: OutcomeBlocked, reason: "worktree has uncommitted changes"},
		{name: "empty url", fake: &scriptedGHExecutor{queryErr: errors.New("exit status 1"), queryStderr: "no pull requests found"}, outcome: OutcomeFailed, reason: "empty pr url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := taskstore.NewTestStore(t)
			req := createDoneTask(t, store, "plan/test")
			if tt.prepare != nil {
				tt.prepare(t, req)
			}
			t.Cleanup(gitpkg.SetGHExec(tt.fake))
			res, err := Ensure(context.Background(), store, req)
			require.NoError(t, err)
			assert.Equal(t, tt.outcome, res.Outcome)
			assert.Contains(t, res.Reason, tt.reason)
			entry, getErr := store.Get(req.Project, req.PlanFile)
			require.NoError(t, getErr)
			assert.Equal(t, string(tt.outcome), entry.PRCreateState)
			assert.NotEmpty(t, entry.PRCreateError)
			assert.Equal(t, 1, entry.PRCreateAttempts)
		})
	}
}

func TestEnsureManualOverridesDisabledConfig(t *testing.T) {
	store := taskstore.NewTestStore(t)
	req := createDoneTask(t, store, "plan/manual")
	req.Enabled = false
	req.Manual = true
	req.BodyOverride = "# edited body\n\nkeep this verbatim"
	fake := &scriptedGHExecutor{queryOutput: prJSON("https://example.test/pr/7", verifiedHead(t, store, req)), queryErr: errors.New("exit status 1"), queryStderr: "no pull requests found"}
	t.Cleanup(gitpkg.SetGHExec(fake))

	res, err := Ensure(context.Background(), store, req)
	require.NoError(t, err)
	assert.Equal(t, OutcomeCreated, res.Outcome)
	assert.Equal(t, 1, fake.countCall("pr create"))
	var createCall []string
	for _, call := range fake.calls {
		if len(call) >= 2 && call[0] == "pr" && call[1] == "create" {
			createCall = call
			break
		}
	}
	require.NotEmpty(t, createCall)
	assert.Contains(t, strings.Join(createCall, "\x00"), "--body\x00# edited body\n\nkeep this verbatim")
}

func TestEnsurePersistsNonSuccessOutcomes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		enabled bool
		outcome Outcome
		reason  string
	}{
		{name: "disabled", enabled: false, outcome: OutcomeSkipped, reason: "auto pr disabled by config"},
		{name: "missing branch", enabled: true, outcome: OutcomeBlocked, reason: "no branch recorded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := taskstore.NewTestStore(t)
			require.NoError(t, store.Create("test", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusDone}))
			res, err := Ensure(context.Background(), store, Request{RepoPath: t.TempDir(), Project: "test", PlanFile: "plan", Enabled: tt.enabled})
			require.NoError(t, err)
			assert.Equal(t, tt.outcome, res.Outcome)
			assert.Contains(t, res.Reason, tt.reason)
			entry, getErr := store.Get("test", "plan")
			require.NoError(t, getErr)
			assert.Equal(t, string(tt.outcome), entry.PRCreateState)
			assert.NotEmpty(t, entry.PRCreateError)
			assert.Equal(t, 1, entry.PRCreateAttempts)
		})
	}
}
