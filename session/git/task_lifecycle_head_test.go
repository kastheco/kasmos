package git

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBranchHeadSHA(t *testing.T) {
	dir := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	runGit("commit", "--allow-empty", "-m", "initial")
	runGit("branch", "plan/example")
	want := runGit("rev-parse", "plan/example^{commit}")

	got, err := BranchHeadSHA(dir, "plan/example")
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Len(t, got, 40)

	_, err = BranchHeadSHA(dir, "plan/missing")
	require.ErrorIs(t, err, ErrBranchNotFound)

	_, err = BranchHeadSHA(filepath.Join(dir, "missing-repo"), "plan/example")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrBranchNotFound)
}

func TestBranchMergeBaseSHA(t *testing.T) {
	dir := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	runGit("commit", "--allow-empty", "-m", "base")
	base := runGit("rev-parse", "HEAD")
	baseBranch := runGit("branch", "--show-current")
	runGit("checkout", "-b", "plan/example")
	runGit("commit", "--allow-empty", "-m", "task head")
	runGit("checkout", "-b", "unrelated")
	runGit("commit", "--allow-empty", "-m", "unrelated head")
	runGit("checkout", baseBranch)
	runGit("checkout", "unrelated")

	got, err := BranchMergeBaseSHA(dir, "plan/example")
	require.NoError(t, err)
	require.Equal(t, base, got)
}

func TestValidateVerificationDetectsDefaultBranchMovement(t *testing.T) {
	dir := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	runGit("init", "-b", "main")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	runGit("commit", "--allow-empty", "-m", "base")
	base := runGit("rev-parse", "HEAD")
	runGit("checkout", "-b", "plan/example")
	runGit("commit", "--allow-empty", "-m", "task")
	head := runGit("rev-parse", "HEAD")
	runGit("checkout", "main")
	runGit("commit", "--allow-empty", "-m", "main moved")

	_, _, reason, err := ValidateVerification(dir, "plan/example", head, base)
	require.NoError(t, err)
	require.Contains(t, reason, "base_changed_after_verification")
}

func TestMergeTaskBranchAtSHARejectsMovedBranch(t *testing.T) {
	dir := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	runGit("init", "-b", "main")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	runGit("commit", "--allow-empty", "-m", "base")
	runGit("checkout", "-b", "plan/example")
	runGit("commit", "--allow-empty", "-m", "reviewed")
	expected := runGit("rev-parse", "HEAD")
	runGit("commit", "--allow-empty", "-m", "moved")
	runGit("checkout", "main")

	base, err := DefaultBranchHeadSHA(dir)
	require.NoError(t, err)
	err = MergeTaskBranchAtSHA(dir, "plan/example", expected, base)
	require.ErrorContains(t, err, "moved after verification")
	require.Equal(t, runGit("rev-parse", "main"), runGit("rev-parse", "HEAD"))
}

func TestShortSHA(t *testing.T) {
	require.Equal(t, "", ShortSHA(""))
	require.Equal(t, "abc", ShortSHA("abc"))
	require.Equal(t, "1234567", ShortSHA("1234567890"))
}

func TestErrBranchNotFoundSupportsErrorsIs(t *testing.T) {
	require.True(t, errors.Is(ErrBranchNotFound, ErrBranchNotFound))
}
