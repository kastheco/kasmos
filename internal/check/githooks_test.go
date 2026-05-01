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
		gitConfig      string
		gitConfigErr   error
		wantSkipped    bool
		wantConfigured bool
		wantActual     string
	}{
		{name: "configured correctly", setup: setupKasmosLayout, gitConfig: "scripts/git-hooks", wantConfigured: true, wantActual: "scripts/git-hooks"},
		{name: "core.hooksPath unset", setup: setupKasmosLayout, gitConfig: "", wantConfigured: false, wantActual: ""},
		{name: "core.hooksPath custom", setup: setupKasmosLayout, gitConfig: ".husky", wantConfigured: false, wantActual: ".husky"},
		{name: "no docs-drift-map.yml", setup: func(t *testing.T, dir string) {}, gitConfig: "", wantSkipped: true},
		{name: "hook file missing", setup: setupKasmosLayoutNoHook, gitConfig: "scripts/git-hooks", wantConfigured: false, wantActual: "scripts/git-hooks"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(t, dir)
			prev := SetGitConfigFnForTest(func(string) (string, error) { return tc.gitConfig, tc.gitConfigErr })
			t.Cleanup(func() { SetGitConfigFnForTest(prev) })

			got := CheckPrePushHook(dir)
			assert.Equal(t, tc.wantSkipped, got.Skipped)
			if !tc.wantSkipped {
				assert.Equal(t, tc.wantConfigured, got.Configured)
				assert.Equal(t, tc.wantActual, got.ActualPath)
			}
		})
	}
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
