package initcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptionsDefaults(t *testing.T) {
	opts := Options{}
	assert.False(t, opts.Force)
	assert.False(t, opts.Clean)
}

// TestWritePhase verifies the post-wizard write path: TOML config is written
// and can be loaded back correctly. Does not run the interactive wizard.
func TestWritePhase(t *testing.T) {
	// Set up a project dir and chdir into it so GetConfigDir returns .kasmos under it
	projectDir := t.TempDir()
	t.Chdir(projectDir)

	// Set HOME to an empty temp dir to avoid accidental migration from real config
	t.Setenv("HOME", t.TempDir())

	// GetConfigDir will create .kasmos under projectDir
	configDir := filepath.Join(projectDir, ".kasmos")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	// Simulate wizard output
	temp := 0.7
	tc := &config.TOMLConfig{
		Phases: map[string]string{
			"implementing": "coder",
			"planning":     "planner",
		},
		Agents: map[string]config.TOMLAgent{
			"coder": {
				Enabled:     true,
				Program:     "opencode",
				Model:       "anthropic/claude-sonnet-4-6",
				Temperature: &temp,
				Effort:      "high",
				Flags:       []string{},
			},
			"planner": {
				Enabled: true,
				Program: "claude",
				Model:   "claude-opus-4-6",
				Flags:   []string{},
			},
		},
	}

	// Write TOML config
	err := config.SaveTOMLConfig(tc)
	require.NoError(t, err)

	// Verify TOML file exists under .kasmos in the project dir
	tomlPath := filepath.Join(projectDir, ".kasmos", "config.toml")
	assert.FileExists(t, tomlPath)

	// Verify it can be loaded back
	result, err := config.LoadTOMLConfigFrom(tomlPath)
	require.NoError(t, err)
	assert.Equal(t, "coder", result.PhaseRoles["implementing"])
	assert.Equal(t, "opencode", result.Profiles["coder"].Program)
}

func TestInitCreatesConfigWithMasterPhase(t *testing.T) {
	projectDir := t.TempDir()
	t.Chdir(projectDir)
	t.Setenv("HOME", t.TempDir())

	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".kasmos"), 0o755))

	temp := 0.2
	tc := &config.TOMLConfig{
		Phases: map[string]string{
			"implementing":  "coder",
			"planning":      "planner",
			"master_review": "master",
		},
		Agents: map[string]config.TOMLAgent{
			"master": {
				Enabled:     true,
				Program:     "opencode",
				Model:       "openai/gpt-5.4",
				Temperature: &temp,
				Effort:      "high",
				Flags:       []string{},
			},
		},
	}

	require.NoError(t, config.SaveTOMLConfig(tc))

	result, err := config.LoadTOMLConfigFrom(filepath.Join(projectDir, ".kasmos", "config.toml"))
	require.NoError(t, err)

	assert.Equal(t, "master", result.PhaseRoles["master_review"])
	assert.Equal(t, "opencode", result.Profiles["master"].Program)
	assert.Equal(t, "openai/gpt-5.4", result.Profiles["master"].Model)
}

func TestPreserveExistingEnforcement(t *testing.T) {
	t.Run("no-op when existing is nil", func(t *testing.T) {
		tc := &config.TOMLConfig{}
		preserveExistingEnforcement(tc, nil)
		assert.Nil(t, tc.Enforcement)
	})

	t.Run("no-op when existing enforcement is empty", func(t *testing.T) {
		tc := &config.TOMLConfig{}
		existing := &config.TOMLConfigResult{Enforcement: map[string]bool{}}
		preserveExistingEnforcement(tc, existing)
		assert.Nil(t, tc.Enforcement)
	})

	t.Run("copies missing keys from existing into tc", func(t *testing.T) {
		tc := &config.TOMLConfig{}
		existing := &config.TOMLConfigResult{
			Enforcement: map[string]bool{"codex": false, "claude": true},
		}
		preserveExistingEnforcement(tc, existing)
		require.NotNil(t, tc.Enforcement)
		assert.Equal(t, false, tc.Enforcement["codex"])
		assert.Equal(t, true, tc.Enforcement["claude"])
	})

	t.Run("does not overwrite keys already set in tc", func(t *testing.T) {
		tc := &config.TOMLConfig{
			Enforcement: map[string]bool{"claude": false},
		}
		existing := &config.TOMLConfigResult{
			Enforcement: map[string]bool{"claude": true, "codex": false},
		}
		preserveExistingEnforcement(tc, existing)
		// claude was already false in tc — must not be overwritten
		assert.Equal(t, false, tc.Enforcement["claude"])
		// codex was absent in tc — copied from existing
		assert.Equal(t, false, tc.Enforcement["codex"])
	})
}

func TestEnforcementHarnessNames(t *testing.T) {
	registry := harness.NewRegistry()

	t.Run("returns selected harnesses in stable order", func(t *testing.T) {
		names := enforcementHarnessNames([]string{"claude", "opencode"}, nil, registry)
		assert.Equal(t, []string{"claude", "opencode"}, names)
	})

	t.Run("includes explicit enforcement keys not in selected", func(t *testing.T) {
		enforcement := map[string]bool{"codex": false}
		names := enforcementHarnessNames([]string{"claude"}, enforcement, registry)
		assert.Contains(t, names, "claude")
		assert.Contains(t, names, "codex")
	})

	t.Run("deduplicates when selected and enforcement overlap", func(t *testing.T) {
		enforcement := map[string]bool{"claude": false}
		names := enforcementHarnessNames([]string{"claude", "opencode"}, enforcement, registry)
		claudeCount := 0
		for _, n := range names {
			if n == "claude" {
				claudeCount++
			}
		}
		assert.Equal(t, 1, claudeCount, "claude must appear exactly once")
	})

	t.Run("excludes unknown harness names", func(t *testing.T) {
		enforcement := map[string]bool{"unknown-harness": false}
		names := enforcementHarnessNames([]string{"claude"}, enforcement, registry)
		for _, n := range names {
			assert.NotEqual(t, "unknown-harness", n)
		}
	})

	t.Run("returns stable sorted order", func(t *testing.T) {
		enforcement := map[string]bool{"codex": false, "opencode": true}
		names := enforcementHarnessNames([]string{"claude"}, enforcement, registry)
		// Result must be sorted
		sorted := make([]string, len(names))
		copy(sorted, names)
		for i := 1; i < len(sorted); i++ {
			assert.LessOrEqual(t, sorted[i-1], sorted[i])
		}
	})
}

func TestInitCreatesConfigWithReadinessReviewPhase(t *testing.T) {
	projectDir := t.TempDir()
	t.Chdir(projectDir)
	t.Setenv("HOME", t.TempDir())

	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".kasmos"), 0o755))

	trueVal := true
	temp := 0.2
	tc := &config.TOMLConfig{
		Phases: map[string]string{
			"implementing":     "coder",
			"planning":         "planner",
			"readiness_review": "master",
		},
		Agents: map[string]config.TOMLAgent{
			"master": {
				Enabled:     true,
				Program:     "opencode",
				Model:       "openai/gpt-5.4",
				Temperature: &temp,
				Effort:      "high",
				Flags:       []string{},
			},
		},
		UI: config.TOMLUIConfig{
			AutoReadinessReview: &trueVal,
		},
	}

	require.NoError(t, config.SaveTOMLConfig(tc))

	result, err := config.LoadTOMLConfigFrom(filepath.Join(projectDir, ".kasmos", "config.toml"))
	require.NoError(t, err)

	// Canonical key is readiness_review
	assert.Equal(t, "master", result.PhaseRoles["readiness_review"])
	assert.Empty(t, result.PhaseRoles["master_review"])
	assert.Equal(t, "opencode", result.Profiles["master"].Program)
	assert.Equal(t, "openai/gpt-5.4", result.Profiles["master"].Model)
	// auto_readiness_review written explicitly as true
	require.NotNil(t, result.AutoReadinessReview)
	assert.True(t, *result.AutoReadinessReview)
}
