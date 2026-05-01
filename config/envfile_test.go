package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(path, []byte(`
# comment
KASMOS_LINEAR_API_KEY=project-token
LINEAR_API_KEY = fallback-token # inline comment
QUOTED_SINGLE='literal # value'
QUOTED_DOUBLE="line\nvalue" # quoted comment
export KASMOS_LINEAR_API_URL=https://linear.example/graphql
`), 0o600))

	values, err := ReadDotEnv(path)
	require.NoError(t, err)

	assert.Equal(t, "project-token", values["KASMOS_LINEAR_API_KEY"])
	assert.Equal(t, "fallback-token", values["LINEAR_API_KEY"])
	assert.Equal(t, "literal # value", values["QUOTED_SINGLE"])
	assert.Equal(t, "line\nvalue", values["QUOTED_DOUBLE"])
	assert.Equal(t, "https://linear.example/graphql", values["KASMOS_LINEAR_API_URL"])
}

func TestLoadDotEnvDoesNotOverrideExistingEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(path, []byte("KASMOS_LINEAR_API_KEY=project-token\nLINEAR_API_KEY=fallback-token\n"), 0o600))
	t.Setenv("KASMOS_LINEAR_API_KEY", "global-token")
	t.Setenv("LINEAR_API_KEY", "")

	require.NoError(t, LoadDotEnv(path))

	assert.Equal(t, "global-token", os.Getenv("KASMOS_LINEAR_API_KEY"))
	assert.Equal(t, "", os.Getenv("LINEAR_API_KEY"))
}

func TestLoadProjectDotEnvUsesRepoRoot(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	runEnvfileGit(t, repo, "init", "-b", "main")

	require.NoError(t, os.WriteFile(filepath.Join(repo, ".env"), []byte("KASMOS_LINEAR_API_KEY=repo-token\n"), 0o600))
	nested := filepath.Join(repo, "internal", "nested")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	t.Chdir(nested)

	require.NoError(t, LoadProjectDotEnv())

	assert.Equal(t, "repo-token", os.Getenv("KASMOS_LINEAR_API_KEY"))
}

func TestReadDotEnvRejectsInvalidLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(path, []byte("not valid\n"), 0o600))

	_, err := ReadDotEnv(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid dotenv line")
}

func runEnvfileGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}
