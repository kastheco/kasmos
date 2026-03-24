package cmd

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetAndRestoreCommandsExist(t *testing.T) {
	root := NewRootCmd()

	resetCmd, _, err := root.Find([]string{"reset"})
	require.NoError(t, err)
	assert.Equal(t, "reset", resetCmd.Name())

	restoreCmd, _, err := root.Find([]string{"restore"})
	require.NoError(t, err)
	assert.Equal(t, "restore", restoreCmd.Name())
}

func TestPromptForRepo_All(t *testing.T) {
	ok, all, err := promptForRepo(bufio.NewReader(strings.NewReader("a\n")), ioDiscard{}, "/tmp/repo")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.True(t, all)
}

func TestResetCmd_ReadsReposFromStdinDryRun(t *testing.T) {
	backupRoot := t.TempDir()
	t.Setenv("BACKUP_ROOT", backupRoot)

	repo1 := makeFakeRepo(t)
	repo2 := makeFakeRepo(t)

	cmd := NewResetCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(repo1 + "\n# comment\n" + repo2 + "\n"))
	cmd.SetArgs([]string{"--dry-run"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), repo1)
	assert.Contains(t, out.String(), repo2)
	assert.Contains(t, out.String(), "[dry-run] would create backup")
	assert.Contains(t, out.String(), "backups root: "+backupRoot)
}

func TestResetCmd_SkipsWorktreesWithoutFlag(t *testing.T) {
	backupRoot := t.TempDir()
	t.Setenv("BACKUP_ROOT", backupRoot)

	repo := makeFakeRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".worktrees"), 0o755))

	cmd := NewResetCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(repo + "\n"))
	cmd.SetArgs([]string{"--dry-run"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "rerun with --ignore-worktrees")
}

func TestCreateAndExtractManagedBackup_RoundTrip(t *testing.T) {
	repo := makeFakeRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".kasmos", "config.toml"), []byte("foo = \"bar\"\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".claude", "agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".claude", "agents", "coder.md"), []byte("coder"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".mcp.json"), []byte("{}\n"), 0o644))

	backupFile := filepath.Join(t.TempDir(), "repo-backup.tar.gz")
	require.NoError(t, createManagedBackup(repo, backupFile))

	require.NoError(t, os.RemoveAll(filepath.Join(repo, ".kasmos")))
	require.NoError(t, os.RemoveAll(filepath.Join(repo, ".claude")))
	require.NoError(t, os.Remove(filepath.Join(repo, ".mcp.json")))

	require.NoError(t, extractManagedBackup(repo, backupFile))

	content, err := os.ReadFile(filepath.Join(repo, ".kasmos", "config.toml"))
	require.NoError(t, err)
	assert.Equal(t, "foo = \"bar\"\n", string(content))

	content, err = os.ReadFile(filepath.Join(repo, ".claude", "agents", "coder.md"))
	require.NoError(t, err)
	assert.Equal(t, "coder", string(content))

	content, err = os.ReadFile(filepath.Join(repo, ".mcp.json"))
	require.NoError(t, err)
	assert.Equal(t, "{}\n", string(content))
}

func TestRestoreCmd_DryRun(t *testing.T) {
	backupRoot := t.TempDir()
	t.Setenv("BACKUP_ROOT", backupRoot)

	repo := makeFakeRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".kasmos", "config.toml"), []byte("foo = \"bar\"\n"), 0o644))

	backupFile := filepath.Join(t.TempDir(), "repo-backup.tar.gz")
	require.NoError(t, createManagedBackup(repo, backupFile))

	cmd := NewRestoreCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dry-run", backupFile, repo})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "[dry-run] would create pre-restore backup")
	assert.Contains(t, out.String(), backupFile)
	assert.Contains(t, out.String(), repo)
}

func makeFakeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: /tmp/fake\n"), 0o644))
	return repo
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func TestResolveBackupRoot_WithPrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root, err := resolveBackupRoot("restore-", func() time.Time {
		return time.Date(2026, 3, 23, 12, 34, 56, 0, time.UTC)
	})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "kasmos-backups", "restore-20260323-123456"), root)
}
