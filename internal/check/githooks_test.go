package check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckPrePushHook(t *testing.T) {
	cases := []struct {
		name           string
		setup          func(t *testing.T, dir string)
		gitConfig      func(t *testing.T, dir string) string
		gitConfigErr   error
		wantSkipped    bool
		wantConfigured bool
		wantActual     func(t *testing.T, dir string) string
	}{
		{
			name:           "configured correctly",
			setup:          setupKasmosLayout,
			gitConfig:      literalHookConfig("scripts/git-hooks"),
			wantConfigured: true,
			wantActual:     literalHookConfig("scripts/git-hooks"),
		},
		{
			name:           "configured with dot relative path",
			setup:          setupKasmosLayout,
			gitConfig:      literalHookConfig("./scripts/git-hooks/"),
			wantConfigured: true,
			wantActual:     literalHookConfig("./scripts/git-hooks/"),
		},
		{
			name:           "configured with absolute path",
			setup:          setupKasmosLayout,
			gitConfig:      repoHookConfig,
			wantConfigured: true,
			wantActual:     repoHookConfig,
		},
		{
			name:           "absolute path outside repo",
			setup:          setupKasmosLayout,
			gitConfig:      func(t *testing.T, dir string) string { return filepath.Join(t.TempDir(), "scripts", "git-hooks") },
			wantConfigured: false,
		},
		{
			name:           "core.hooksPath unset",
			setup:          setupKasmosLayout,
			gitConfig:      literalHookConfig(""),
			wantConfigured: false,
			wantActual:     literalHookConfig(""),
		},
		{
			name:           "core.hooksPath custom",
			setup:          setupKasmosLayout,
			gitConfig:      literalHookConfig(".husky"),
			wantConfigured: false,
			wantActual:     literalHookConfig(".husky"),
		},
		{
			name:        "no docs-drift-map.yml",
			setup:       func(t *testing.T, dir string) {},
			gitConfig:   literalHookConfig(""),
			wantSkipped: true,
		},
		{
			name:           "hook file missing",
			setup:          setupKasmosLayoutNoHook,
			gitConfig:      literalHookConfig("scripts/git-hooks"),
			wantConfigured: false,
			wantActual:     literalHookConfig("scripts/git-hooks"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(t, dir)
			gitConfig := tc.gitConfig(t, dir)
			prev := SetGitConfigFnForTest(func(string) (string, error) { return gitConfig, tc.gitConfigErr })
			t.Cleanup(func() { SetGitConfigFnForTest(prev) })

			got := CheckPrePushHook(dir)
			assert.Equal(t, tc.wantSkipped, got.Skipped)
			if !tc.wantSkipped {
				wantActual := gitConfig
				if tc.wantActual != nil {
					wantActual = tc.wantActual(t, dir)
				}
				assert.Equal(t, tc.wantConfigured, got.Configured)
				assert.Equal(t, wantActual, got.ActualPath)
			}
		})
	}
}

func literalHookConfig(value string) func(t *testing.T, dir string) string {
	return func(t *testing.T, dir string) string { return value }
}

func repoHookConfig(t *testing.T, dir string) string {
	return filepath.Join(dir, "scripts", "git-hooks")
}

func setupKasmosLayout(t *testing.T, dir string) {
	t.Helper()
	setupKasmosLayoutNoHook(t, dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "scripts", "git-hooks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "git-hooks", "pre-push"), []byte("#!/usr/bin/env bash\n"), 0o755))
}

func setupKasmosLayoutNoHook(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docs", "docs-drift-map.yml"), []byte("[]"), 0o644))
}
