package git

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingGHExecutor struct {
	calls  [][]string
	output []byte
	err    error
	stderr string
}

func (f *recordingGHExecutor) record(cmd *exec.Cmd) {
	f.calls = append(f.calls, append([]string(nil), cmd.Args[1:]...))
}

func (f *recordingGHExecutor) Run(cmd *exec.Cmd) error {
	f.record(cmd)
	return f.err
}

func (f *recordingGHExecutor) Output(cmd *exec.Cmd) ([]byte, error) {
	f.record(cmd)
	if f.stderr != "" {
		_, _ = io.WriteString(cmd.Stderr, f.stderr)
	}
	return f.output, f.err
}

func newPushableWorktree(t *testing.T) *GitWorktree {
	t.Helper()
	repo := t.TempDir()
	bare := filepath.Join(t.TempDir(), "origin.git")
	require.NoError(t, exec.Command("git", "init", "--bare", bare).Run())
	require.NoError(t, exec.Command("git", "init", "-b", "plan/test", repo).Run())
	require.NoError(t, os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("content\n"), 0o644))
	for _, args := range [][]string{{"-C", repo, "add", "tracked.txt"}, {"-C", repo, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "initial"}, {"-C", repo, "remote", "add", "origin", bare}} {
		require.NoError(t, exec.Command("git", args...).Run())
	}
	return NewGitWorktreeFromStorage(repo, repo, "test", "plan/test", "")
}

func TestRequireCleanForPR(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, exec.Command("git", "init", repo).Run())
	worktree := NewGitWorktreeFromStorage(repo, repo, "test", "plan/test", "")

	require.NoError(t, worktree.requireCleanForPR())
	require.NoError(t, os.WriteFile(filepath.Join(repo, "unrelated.txt"), []byte("user edit\n"), 0o644))

	err := worktree.requireCleanForPR()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWorktreeDirty)
	assert.Contains(t, err.Error(), "worktree has uncommitted changes")
	assert.Contains(t, err.Error(), "?? unrelated.txt")
}

func TestCreatePRHeadless(t *testing.T) {
	worktree := newPushableWorktree(t)
	fake := &recordingGHExecutor{output: []byte("https://github.com/org/repo/pull/1\n")}
	t.Cleanup(SetGHExec(fake))

	require.NoError(t, worktree.CreatePR("title", "body"))
	require.Len(t, fake.calls, 1)
	assert.Equal(t, "pr create --title title --body body --head plan/test", strings.Join(fake.calls[0], " "))
	for _, call := range fake.calls {
		assert.NotContains(t, call, "--draft")
		assert.NotContains(t, call, "--web")
	}
}

func TestCreatePRAlreadyExists(t *testing.T) {
	worktree := newPushableWorktree(t)
	fake := &recordingGHExecutor{err: errors.New("exit status 1"), stderr: "a pull request already exists for branch plan/test"}
	t.Cleanup(SetGHExec(fake))

	err := worktree.CreatePR("title", "body")
	assert.ErrorIs(t, err, ErrPRAlreadyExists)
	require.Len(t, fake.calls, 1)
}

func TestCreatePRDirtyDoesNotInvokeGH(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, exec.Command("git", "init", repo).Run())
	require.NoError(t, os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("dirty\n"), 0o644))
	worktree := NewGitWorktreeFromStorage(repo, repo, "test", "plan/test", "")
	fake := &recordingGHExecutor{}
	t.Cleanup(SetGHExec(fake))

	err := worktree.CreatePR("title", "body")
	assert.ErrorIs(t, err, ErrWorktreeDirty)
	assert.Contains(t, err.Error(), "?? dirty.txt")
	assert.Empty(t, fake.calls)
}

func TestQueryPRState(t *testing.T) {
	worktree := NewGitWorktreeFromStorage(t.TempDir(), t.TempDir(), "test", "plan/test", "")
	t.Run("no pull request", func(t *testing.T) {
		fake := &recordingGHExecutor{err: errors.New("exit status 1"), stderr: "no pull requests found for branch"}
		t.Cleanup(SetGHExec(fake))
		state, err := worktree.QueryPRState()
		require.NoError(t, err)
		assert.Zero(t, state)
	})
	t.Run("populated", func(t *testing.T) {
		fake := &recordingGHExecutor{output: []byte(fmt.Sprintf(`{"url":%q,"reviewDecision":"APPROVED","statusCheckRollup":{"state":"SUCCESS"},"isDraft":false,"number":7}`, "https://github.com/org/repo/pull/7"))}
		t.Cleanup(SetGHExec(fake))
		state, err := worktree.QueryPRState()
		require.NoError(t, err)
		assert.Equal(t, PRState{URL: "https://github.com/org/repo/pull/7", ReviewDecision: "APPROVED", CheckStatus: "SUCCESS", Number: 7}, state)
	})
}

func TestParsePRViewJSON(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		wantURL   string
		wantRD    string
		wantCS    string
		wantDraft bool
		wantNum   int
	}{
		{
			name:    "approved with passing checks",
			json:    `{"url":"https://github.com/org/repo/pull/1","reviewDecision":"APPROVED","statusCheckRollup":{"state":"SUCCESS"},"isDraft":false,"number":1}`,
			wantURL: "https://github.com/org/repo/pull/1",
			wantRD:  "APPROVED",
			wantCS:  "SUCCESS",
			wantNum: 1,
		},
		{
			name:      "draft pr",
			json:      `{"url":"https://github.com/org/repo/pull/2","reviewDecision":"","statusCheckRollup":{"state":"PENDING"},"isDraft":true,"number":2}`,
			wantURL:   "https://github.com/org/repo/pull/2",
			wantCS:    "PENDING",
			wantDraft: true,
			wantNum:   2,
		},
		{
			name:    "no status rollup",
			json:    `{"url":"https://github.com/org/repo/pull/3","reviewDecision":"REVIEW_REQUIRED","isDraft":false,"number":3}`,
			wantURL: "https://github.com/org/repo/pull/3",
			wantRD:  "REVIEW_REQUIRED",
			wantNum: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := ParsePRViewJSON([]byte(tt.json))
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, state.URL)
			assert.Equal(t, tt.wantRD, state.ReviewDecision)
			assert.Equal(t, tt.wantCS, state.CheckStatus)
			assert.Equal(t, tt.wantDraft, state.IsDraft)
			assert.Equal(t, tt.wantNum, state.Number)
		})
	}
}

func TestParsePRViewJSON_MalformedJSON(t *testing.T) {
	_, err := ParsePRViewJSON([]byte(`{not valid json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse pr view json")
}
