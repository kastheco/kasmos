package config

import (
	"github.com/kastheco/kasmos/log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain runs before all tests to set up the test environment
func TestMain(m *testing.M) {
	log.Initialize(false)
	code := m.Run()
	log.Close()
	os.Exit(code)
}

func TestGetDefaultCommand(t *testing.T) {
	t.Run("finds codex in PATH", func(t *testing.T) {
		tempDir := t.TempDir()
		codexPath := filepath.Join(tempDir, "codex")

		err := os.WriteFile(codexPath, []byte("#!/bin/bash\necho 'mock codex'"), 0755)
		require.NoError(t, err)

		t.Setenv("PATH", tempDir)
		t.Setenv("SHELL", "/bin/sh")

		result, err := GetDefaultCommand()

		assert.NoError(t, err)
		assert.True(t, strings.Contains(result, "codex"))
	})

	t.Run("falls back to opencode when codex is missing", func(t *testing.T) {
		tempDir := t.TempDir()
		opencodePath := filepath.Join(tempDir, "opencode")

		err := os.WriteFile(opencodePath, []byte("#!/bin/bash\necho 'mock opencode'"), 0755)
		require.NoError(t, err)

		t.Setenv("PATH", tempDir)
		t.Setenv("SHELL", "/bin/sh")

		result, err := GetDefaultCommand()

		assert.NoError(t, err)
		assert.True(t, strings.Contains(result, "opencode"))
	})

	t.Run("falls back to claude when codex and opencode are missing", func(t *testing.T) {
		tempDir := t.TempDir()
		claudePath := filepath.Join(tempDir, "claude")

		err := os.WriteFile(claudePath, []byte("#!/bin/bash\necho 'mock claude'"), 0755)
		require.NoError(t, err)

		t.Setenv("PATH", tempDir)
		t.Setenv("SHELL", "/bin/sh")

		result, err := GetDefaultCommand()

		assert.NoError(t, err)
		assert.True(t, strings.Contains(result, "claude"))
	})

	t.Run("handles missing codex, opencode, and claude commands", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("PATH", tempDir)
		t.Setenv("SHELL", "/bin/sh")

		result, err := GetDefaultCommand()

		assert.Error(t, err)
		assert.Equal(t, "", result)
		assert.Contains(t, err.Error(), "none of codex, opencode, or claude command found")
	})

	t.Run("handles empty SHELL environment", func(t *testing.T) {
		tempDir := t.TempDir()
		codexPath := filepath.Join(tempDir, "codex")

		err := os.WriteFile(codexPath, []byte("#!/bin/bash\necho 'mock codex'"), 0755)
		require.NoError(t, err)

		t.Setenv("PATH", tempDir)
		t.Setenv("HOME", tempDir)
		t.Setenv("SHELL", "")

		result, err := GetDefaultCommand()

		assert.NoError(t, err)
		assert.True(t, strings.Contains(result, "codex"))
	})

	t.Run("prefers codex when multiple commands exist", func(t *testing.T) {
		tempDir := t.TempDir()
		codexPath := filepath.Join(tempDir, "codex")
		opencodePath := filepath.Join(tempDir, "opencode")
		claudePath := filepath.Join(tempDir, "claude")

		err := os.WriteFile(codexPath, []byte("#!/bin/bash\necho 'mock codex'"), 0755)
		require.NoError(t, err)
		err = os.WriteFile(opencodePath, []byte("#!/bin/bash\necho 'mock opencode'"), 0755)
		require.NoError(t, err)
		err = os.WriteFile(claudePath, []byte("#!/bin/bash\necho 'mock claude'"), 0755)
		require.NoError(t, err)

		t.Setenv("PATH", tempDir)
		t.Setenv("SHELL", "/bin/sh")

		result, err := GetDefaultCommand()

		assert.NoError(t, err)
		assert.True(t, strings.Contains(result, "codex"))
	})

	t.Run("handles alias parsing", func(t *testing.T) {
		assert.Equal(t, "/usr/local/bin/opencode", parseCommandOutput("opencode: aliased to /usr/local/bin/opencode"))
		assert.Equal(t, "/usr/local/bin/opencode", parseCommandOutput("/usr/local/bin/opencode"))
		assert.Equal(t, "", parseCommandOutput("true: shell built-in command"))
		assert.Equal(t, "", parseCommandOutput("   \n"))
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Run("creates config with default values", func(t *testing.T) {
		config := DefaultConfig()

		assert.NotNil(t, config)
		assert.NotEmpty(t, config.DefaultProgram)
		assert.False(t, config.AutoYes)
		assert.True(t, config.AutoAdvanceWaves)
		assert.True(t, config.AutoAdvance)
		assert.True(t, config.AutoReviewFix)
		assert.Equal(t, 1000, config.DaemonPollInterval)
		assert.NotEmpty(t, config.BranchPrefix)
		assert.True(t, strings.HasSuffix(config.BranchPrefix, "/"))
	})

	t.Run("falls back to codex when command detection fails", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("PATH", tempDir)
		t.Setenv("SHELL", "/bin/sh")

		config := DefaultConfig()

		assert.Equal(t, "codex", config.DefaultProgram)
	})
}

func TestGetConfigDir(t *testing.T) {
	runGit := func(t *testing.T, repo string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	}

	t.Run("returns .kasmos relative to working directory", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)

		configDir, err := GetConfigDir()

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tempDir, ".kasmos"), configDir)
	})

	t.Run("returns .kasmos under repo root from nested git directory", func(t *testing.T) {
		repoDir := t.TempDir()
		t.Setenv("HOME", t.TempDir())

		runGit(t, repoDir, "init", "-b", "main")
		runGit(t, repoDir, "config", "user.email", "test@example.com")
		runGit(t, repoDir, "config", "user.name", "test")
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("init\n"), 0o644))
		runGit(t, repoDir, "add", ".")
		runGit(t, repoDir, "commit", "-m", "initial")

		nestedDir := filepath.Join(repoDir, "internal", "nested")
		require.NoError(t, os.MkdirAll(nestedDir, 0o755))
		t.Chdir(nestedDir)

		configDir, err := GetConfigDir()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(repoDir, ".kasmos"), configDir)
	})

	t.Run("returns .kasmos under main repo root from worktree", func(t *testing.T) {
		repoDir := t.TempDir()
		t.Setenv("HOME", t.TempDir())

		runGit(t, repoDir, "init", "-b", "main")
		runGit(t, repoDir, "config", "user.email", "test@example.com")
		runGit(t, repoDir, "config", "user.name", "test")
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("init\n"), 0o644))
		runGit(t, repoDir, "add", ".")
		runGit(t, repoDir, "commit", "-m", "initial")

		runGit(t, repoDir, "branch", "plan/worktree-config")
		worktreeParent := t.TempDir()
		worktreeDir := filepath.Join(worktreeParent, "worktree-config")
		runGit(t, repoDir, "worktree", "add", worktreeDir, "plan/worktree-config")
		t.Chdir(worktreeDir)

		configDir, err := GetConfigDir()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(repoDir, ".kasmos"), configDir)
	})

	t.Run("migrates supported files from legacy XDG location", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)
		projectDir := t.TempDir()
		t.Chdir(projectDir)

		// Create legacy config at ~/.config/kasmos/
		legacyDir := filepath.Join(tempHome, ".config", "kasmos")
		require.NoError(t, os.MkdirAll(legacyDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(legacyDir, "config.toml"),
			[]byte("[ui]\nanimate_banner = true\n"), 0644))
		require.NoError(t, os.WriteFile(
			filepath.Join(legacyDir, "state.json"),
			[]byte(`{"help_screens_seen":1}`), 0644))

		configDir, err := GetConfigDir()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(projectDir, ".kasmos"), configDir)

		// Config should be copied to new location
		data, err := os.ReadFile(filepath.Join(configDir, "config.toml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "animate_banner")

		// taskstore.db is NOT copied — the global DB at ~/.config/kasmos/taskstore.db is used directly.
		assert.NoFileExists(t, filepath.Join(configDir, "taskstore.db"))

		migratedState, err := os.ReadFile(filepath.Join(configDir, "state.json"))
		require.NoError(t, err)
		assert.JSONEq(t, `{"help_screens_seen":1}`, string(migratedState))

		// Legacy files should still exist (copy, not move)
		assert.FileExists(t, filepath.Join(legacyDir, "config.toml"))
		assert.FileExists(t, filepath.Join(legacyDir, "state.json"))
	})

	t.Run("skips migration when config already exists in .kasmos", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)
		projectDir := t.TempDir()
		t.Chdir(projectDir)

		// Create config in both locations
		kasmosDir := filepath.Join(projectDir, ".kasmos")
		require.NoError(t, os.MkdirAll(kasmosDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(kasmosDir, "config.toml"),
			[]byte("[ui]\nanimate_banner = false\n"), 0644))

		legacyDir := filepath.Join(tempHome, ".config", "kasmos")
		require.NoError(t, os.MkdirAll(legacyDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(legacyDir, "config.toml"),
			[]byte("[ui]\nanimate_banner = true\n"), 0644))

		configDir, err := GetConfigDir()
		require.NoError(t, err)

		// Should use existing .kasmos config, NOT overwrite with legacy
		data, err := os.ReadFile(filepath.Join(configDir, "config.toml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "animate_banner = false")
	})

	t.Run("no-ops when neither location has config", func(t *testing.T) {
		projectDir := t.TempDir()
		t.Chdir(projectDir)
		t.Setenv("HOME", t.TempDir())

		configDir, err := GetConfigDir()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(projectDir, ".kasmos"), configDir)
	})
}

func TestConfigFromTOML(t *testing.T) {
	falseVal := false
	zeroCycles := 0
	threshold := 3
	trueVal := true
	result := &TOMLConfigResult{
		DefaultProgram:         "test-cmd",
		AutoYes:                true,
		DaemonPollInterval:     2500,
		BranchPrefix:           "test/",
		NotificationsEnabled:   &falseVal,
		Profiles:               map[string]AgentProfile{"coder": {Program: "opencode", Enabled: true}},
		PhaseRoles:             map[string]string{"implementing": "coder"},
		AnimateBanner:          true,
		AutoAdvanceWaves:       &trueVal,
		AutoAdvance:            &trueVal,
		AutoReviewFix:          &falseVal,
		MaxReviewFixCycles:     &zeroCycles,
		TelemetryEnabled:       &falseVal,
		DatabaseURL:            "https://example.test/store",
		BlueprintSkipThreshold: &threshold,
	}

	cfg := configFromTOML(result)
	require.NotNil(t, cfg)
	assert.Equal(t, "test-cmd", cfg.DefaultProgram)
	assert.True(t, cfg.AutoYes)
	assert.Equal(t, 2500, cfg.DaemonPollInterval)
	assert.Equal(t, "test/", cfg.BranchPrefix)
	require.NotNil(t, cfg.NotificationsEnabled)
	assert.False(t, cfg.AreNotificationsEnabled())
	assert.True(t, cfg.AnimateBanner)
	assert.True(t, cfg.AutoAdvanceWaves)
	assert.True(t, cfg.AutoAdvance)
	assert.False(t, cfg.AutoReviewFix)
	assert.Equal(t, 0, cfg.MaxReviewFixCycles)
	require.NotNil(t, cfg.TelemetryEnabled)
	assert.False(t, cfg.IsTelemetryEnabled())
	assert.Equal(t, "https://example.test/store", cfg.DatabaseURL)
	assert.Equal(t, 3, cfg.BlueprintSkipThreshold())
	assert.Equal(t, "opencode", cfg.Profiles["coder"].Program)
}

func TestConfigFromTOML_Defaults(t *testing.T) {
	result := &TOMLConfigResult{
		Profiles:   map[string]AgentProfile{},
		PhaseRoles: map[string]string{},
	}

	cfg := configFromTOML(result)
	require.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.DefaultProgram)
	assert.Equal(t, 1000, cfg.DaemonPollInterval)
	assert.NotEmpty(t, cfg.BranchPrefix)
	assert.True(t, cfg.AutoAdvanceWaves)
	assert.True(t, cfg.AutoAdvance)
	assert.True(t, cfg.AutoReviewFix)
	assert.True(t, cfg.AreNotificationsEnabled())
}

func TestLoadConfig(t *testing.T) {
	t.Run("returns default config when file doesn't exist", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)
		t.Setenv("HOME", t.TempDir())

		config := LoadConfig()

		assert.NotNil(t, config)
		assert.NotEmpty(t, config.DefaultProgram)
		assert.False(t, config.AutoYes)
		assert.True(t, config.AutoAdvanceWaves)
		assert.True(t, config.AutoAdvance)
		assert.True(t, config.AutoReviewFix)
		assert.Equal(t, 1000, config.DaemonPollInterval)
		assert.NotEmpty(t, config.BranchPrefix)
		assert.FileExists(t, filepath.Join(tempDir, ".kasmos", TOMLConfigFileName))
	})

	t.Run("loads valid config file", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)
		t.Setenv("HOME", t.TempDir())

		configDir := filepath.Join(tempDir, ".kasmos")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(configDir, TOMLConfigFileName)
		configContent := `default_program = "test-claude"
auto_yes = true
daemon_poll_interval = 2000
branch_prefix = "test/"
`
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		config := LoadConfig()

		assert.NotNil(t, config)
		assert.Equal(t, "test-claude", config.DefaultProgram)
		assert.True(t, config.AutoYes)
		assert.Equal(t, 2000, config.DaemonPollInterval)
		assert.Equal(t, "test/", config.BranchPrefix)
	})

	t.Run("returns default config on invalid TOML", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)
		t.Setenv("HOME", t.TempDir())

		configDir := filepath.Join(tempDir, ".kasmos")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(configDir, TOMLConfigFileName)
		invalidContent := `[invalid toml content`
		err = os.WriteFile(configPath, []byte(invalidContent), 0644)
		require.NoError(t, err)

		config := LoadConfig()

		assert.NotNil(t, config)
		assert.NotEmpty(t, config.DefaultProgram)
		assert.False(t, config.AutoYes)
		assert.True(t, config.AutoAdvanceWaves)
		assert.True(t, config.AutoAdvance)
		assert.True(t, config.AutoReviewFix)
		assert.Equal(t, 1000, config.DaemonPollInterval)
	})

	t.Run("toml false and zero values are respected", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)
		t.Setenv("HOME", t.TempDir())

		configDir := filepath.Join(tempDir, ".kasmos")
		require.NoError(t, os.MkdirAll(configDir, 0755))

		tomlPath := filepath.Join(configDir, TOMLConfigFileName)
		tomlContent := `[ui]
	auto_advance_waves = false
	auto_advance = false
	auto_review_fix = false
	max_review_fix_cycles = 0
`
		require.NoError(t, os.WriteFile(tomlPath, []byte(tomlContent), 0644))

		config := LoadConfig()
		require.NotNil(t, config)
		assert.False(t, config.AutoAdvanceWaves)
		assert.False(t, config.AutoAdvance)
		assert.False(t, config.AutoReviewFix)
		assert.Equal(t, 0, config.MaxReviewFixCycles)
	})
}

func TestLoadConfig_MigratesJSON(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	t.Setenv("HOME", t.TempDir())

	configDir := filepath.Join(tempDir, ".kasmos")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	jsonContent := `{
		"default_program": "migrated-claude",
		"auto_yes": true,
		"daemon_poll_interval": 3000,
		"branch_prefix": "migrated/",
		"auto_advance_waves": true,
		"auto_review_fix": false,
		"max_review_fix_cycles": 0,
		"notifications_enabled": false
	}`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), []byte(jsonContent), 0o644))

	cfg := LoadConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "migrated-claude", cfg.DefaultProgram)
	assert.True(t, cfg.AutoYes)
	assert.Equal(t, 3000, cfg.DaemonPollInterval)
	assert.Equal(t, "migrated/", cfg.BranchPrefix)
	assert.True(t, cfg.AutoAdvanceWaves)
	assert.False(t, cfg.AutoReviewFix)
	assert.Equal(t, 0, cfg.MaxReviewFixCycles)
	require.NotNil(t, cfg.NotificationsEnabled)
	assert.False(t, cfg.AreNotificationsEnabled())
	assert.NoFileExists(t, filepath.Join(configDir, "config.json"))
	assert.FileExists(t, filepath.Join(configDir, "config.json.migrated"))
	assert.FileExists(t, filepath.Join(configDir, TOMLConfigFileName))

	written, err := os.ReadFile(filepath.Join(configDir, TOMLConfigFileName))
	require.NoError(t, err)
	assert.Contains(t, string(written), `default_program = "migrated-claude"`)
	assert.Contains(t, string(written), `auto_advance_waves = true`)
	assert.Contains(t, string(written), `auto_review_fix = false`)
	assert.Contains(t, string(written), `max_review_fix_cycles = 0`)
	assert.Contains(t, string(written), `notifications_enabled = false`)
	// Legacy parallel_planner_architect key must not appear in migrated TOML.
	assert.NotContains(t, string(written), `parallel_planner_architect`)
}

func TestLoadConfig_MigratesJSONLegacyPPAIgnored(t *testing.T) {
	// parallel_planner_architect in legacy JSON is ignored during migration;
	// the migrated TOML should not contain it.
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	t.Setenv("HOME", t.TempDir())

	configDir := filepath.Join(tempDir, ".kasmos")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	jsonContent := `{
		"default_program": "migrated-claude",
		"parallel_planner_architect": false
	}`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), []byte(jsonContent), 0o644))

	cfg := LoadConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "migrated-claude", cfg.DefaultProgram)

	written, err := os.ReadFile(filepath.Join(configDir, TOMLConfigFileName))
	require.NoError(t, err)
	assert.NotContains(t, string(written), `parallel_planner_architect`)
}

func TestAutoReadinessReviewConfig(t *testing.T) {
	t.Run("explicit false round-trips through configFromTOML", func(t *testing.T) {
		falseVal := false
		result := &TOMLConfigResult{
			Profiles:            map[string]AgentProfile{},
			PhaseRoles:          map[string]string{},
			AutoReadinessReview: &falseVal,
		}
		cfg := configFromTOML(result)
		assert.False(t, cfg.AutoReadinessReview)
	})

	t.Run("explicit true round-trips through configFromTOML", func(t *testing.T) {
		trueVal := true
		result := &TOMLConfigResult{
			Profiles:            map[string]AgentProfile{},
			PhaseRoles:          map[string]string{},
			AutoReadinessReview: &trueVal,
		}
		cfg := configFromTOML(result)
		assert.True(t, cfg.AutoReadinessReview)
	})

	t.Run("nil AutoReadinessReview defaults to true (opt-out default)", func(t *testing.T) {
		result := &TOMLConfigResult{
			Profiles:   map[string]AgentProfile{},
			PhaseRoles: map[string]string{},
		}
		cfg := configFromTOML(result)
		assert.True(t, cfg.AutoReadinessReview)
	})

	t.Run("DefaultConfig enables readiness review", func(t *testing.T) {
		cfg := DefaultConfig()
		assert.True(t, cfg.AutoReadinessReview)
	})
}

func TestResolveProfile_ReadinessReviewAlias(t *testing.T) {
	masterProfile := AgentProfile{Program: "opencode", Enabled: true, ExecutionMode: ExecutionModeTmux}

	t.Run("readiness_review resolves via master_review alias", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{"master_review": "master"},
			Profiles:   map[string]AgentProfile{"master": masterProfile},
		}
		profile := cfg.ResolveProfile("readiness_review", "claude")
		assert.Equal(t, "opencode", profile.Program)
	})

	t.Run("master_review resolves via readiness_review alias", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{"readiness_review": "master"},
			Profiles:   map[string]AgentProfile{"master": masterProfile},
		}
		profile := cfg.ResolveProfile("master_review", "claude")
		assert.Equal(t, "opencode", profile.Program)
	})

	t.Run("direct readiness_review mapping takes precedence when both keys present", func(t *testing.T) {
		directProfile := AgentProfile{Program: "claude", Enabled: true, ExecutionMode: ExecutionModeTmux}
		legacyProfile := AgentProfile{Program: "codex", Enabled: true, ExecutionMode: ExecutionModeTmux}
		cfg := &Config{
			PhaseRoles: map[string]string{
				"readiness_review": "direct_master",
				"master_review":    "legacy_master",
			},
			Profiles: map[string]AgentProfile{
				"direct_master": directProfile,
				"legacy_master": legacyProfile,
			},
		}
		profile := cfg.ResolveProfile("readiness_review", "fallback")
		assert.Equal(t, "claude", profile.Program)
	})

	t.Run("neither alias present falls back to default", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{"implementing": "coder"},
			Profiles:   map[string]AgentProfile{"coder": {Program: "opencode", Enabled: true}},
		}
		profile := cfg.ResolveProfile("readiness_review", "fallback")
		assert.Equal(t, "fallback", profile.Program)
	})
}

func TestAgentProfile_ResolveSkipPermissions(t *testing.T) {
	tests := []struct {
		name              string
		defaultSkip       bool
		permissionDefault string
		want              bool
	}{
		{"inherit keeps true default", true, PermissionDefaultInherit, true},
		{"inherit keeps false default", false, PermissionDefaultInherit, false},
		{"prompt overrides true default", true, PermissionDefaultPrompt, false},
		{"prompt keeps false default", false, PermissionDefaultPrompt, false},
		{"bypass keeps true default", true, PermissionDefaultBypass, true},
		{"bypass overrides false default", false, PermissionDefaultBypass, true},
		{"invalid overrides true default as prompt", true, "never", false},
		{"invalid keeps false default as prompt", false, "never", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := AgentProfile{PermissionDefault: tt.permissionDefault}
			assert.Equal(t, tt.want, profile.ResolveSkipPermissions(tt.defaultSkip))
		})
	}
}

func TestEnforcementRoundTrip(t *testing.T) {
	t.Run("nil Enforcement survives configFromTOML and configToTOML", func(t *testing.T) {
		result := &TOMLConfigResult{
			Profiles:   map[string]AgentProfile{},
			PhaseRoles: map[string]string{},
		}
		cfg := configFromTOML(result)
		assert.Nil(t, cfg.Enforcement)

		tc := configToTOML(cfg)
		assert.Nil(t, tc.Enforcement)
	})

	t.Run("explicit false entry survives configFromTOML and configToTOML", func(t *testing.T) {
		result := &TOMLConfigResult{
			Profiles:    map[string]AgentProfile{},
			PhaseRoles:  map[string]string{},
			Enforcement: map[string]bool{"codex": false},
		}
		cfg := configFromTOML(result)
		require.NotNil(t, cfg.Enforcement)
		assert.False(t, cfg.Enforcement["codex"])
		assert.False(t, IsEnforcementEnabled(cfg.Enforcement, "codex"))

		tc := configToTOML(cfg)
		require.NotNil(t, tc.Enforcement)
		assert.False(t, tc.Enforcement["codex"])
	})

	t.Run("explicit true entry survives configFromTOML and configToTOML", func(t *testing.T) {
		result := &TOMLConfigResult{
			Profiles:    map[string]AgentProfile{},
			PhaseRoles:  map[string]string{},
			Enforcement: map[string]bool{"claude": true},
		}
		cfg := configFromTOML(result)
		require.NotNil(t, cfg.Enforcement)
		assert.True(t, cfg.Enforcement["claude"])

		tc := configToTOML(cfg)
		require.NotNil(t, tc.Enforcement)
		assert.True(t, tc.Enforcement["claude"])
	})

	t.Run("mixed entries survive round-trip", func(t *testing.T) {
		result := &TOMLConfigResult{
			Profiles:    map[string]AgentProfile{},
			PhaseRoles:  map[string]string{},
			Enforcement: map[string]bool{"codex": false, "claude": true},
		}
		cfg := configFromTOML(result)
		require.NotNil(t, cfg.Enforcement)
		assert.False(t, cfg.Enforcement["codex"])
		assert.True(t, cfg.Enforcement["claude"])

		tc := configToTOML(cfg)
		require.NotNil(t, tc.Enforcement)
		assert.False(t, tc.Enforcement["codex"])
		assert.True(t, tc.Enforcement["claude"])
	})
}

func intPtr(i int) *int { return &i }

func boolPtr(b bool) *bool { return &b }

func int64Ptr(i int64) *int64 { return &i }

func TestDefaultConfig_SDK(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, int64(4<<20), cfg.SDK.TranscriptMaxBytes, "default TranscriptMaxBytes should be 4 MiB")
	assert.Equal(t, int64(2000), cfg.SDK.TranscriptMaxTurns, "default TranscriptMaxTurns should be 2000")
}

func TestConfigFromTOML_SDKOmittedTable(t *testing.T) {
	// No [sdk] section at all — both fields must fall back to runtime defaults.
	result := &TOMLConfigResult{
		Profiles:   map[string]AgentProfile{},
		PhaseRoles: map[string]string{},
		// SDK is zero-value TOMLSDKConfig: both pointers nil
	}
	cfg := configFromTOML(result)
	assert.Equal(t, int64(4<<20), cfg.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(2000), cfg.SDK.TranscriptMaxTurns)
}

func TestConfigFromTOML_SDKOmittedIndividualKeys(t *testing.T) {
	// [sdk] present but only one key set — the other keeps its default.
	result := &TOMLConfigResult{
		Profiles:   map[string]AgentProfile{},
		PhaseRoles: map[string]string{},
		SDK: TOMLSDKConfig{
			TranscriptMaxBytes: int64Ptr(1 << 20), // 1 MiB explicit
			// TranscriptMaxTurns absent (nil) → default 2000
		},
	}
	cfg := configFromTOML(result)
	assert.Equal(t, int64(1<<20), cfg.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(2000), cfg.SDK.TranscriptMaxTurns)

	result2 := &TOMLConfigResult{
		Profiles:   map[string]AgentProfile{},
		PhaseRoles: map[string]string{},
		SDK: TOMLSDKConfig{
			// TranscriptMaxBytes absent (nil) → default 4 MiB
			TranscriptMaxTurns: int64Ptr(500),
		},
	}
	cfg2 := configFromTOML(result2)
	assert.Equal(t, int64(4<<20), cfg2.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(500), cfg2.SDK.TranscriptMaxTurns)
}

func TestConfigFromTOML_SDKExplicitValues(t *testing.T) {
	result := &TOMLConfigResult{
		Profiles:   map[string]AgentProfile{},
		PhaseRoles: map[string]string{},
		SDK: TOMLSDKConfig{
			TranscriptMaxBytes: int64Ptr(8 << 20),
			TranscriptMaxTurns: int64Ptr(1000),
		},
	}
	cfg := configFromTOML(result)
	assert.Equal(t, int64(8<<20), cfg.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(1000), cfg.SDK.TranscriptMaxTurns)
}

func TestConfigFromTOML_SDKExplicitZeros(t *testing.T) {
	// Explicit zero values must be preserved (operator intends to disable that dimension).
	result := &TOMLConfigResult{
		Profiles:   map[string]AgentProfile{},
		PhaseRoles: map[string]string{},
		SDK: TOMLSDKConfig{
			TranscriptMaxBytes: int64Ptr(0),
			TranscriptMaxTurns: int64Ptr(0),
		},
	}
	cfg := configFromTOML(result)
	assert.Equal(t, int64(0), cfg.SDK.TranscriptMaxBytes, "explicit zero disables byte limit")
	assert.Equal(t, int64(0), cfg.SDK.TranscriptMaxTurns, "explicit zero disables turn limit")
}

func TestConfigFromTOML_SDKNegativeClamp(t *testing.T) {
	// Negative values must be clamped to zero with a warning (not used as-is).
	result := &TOMLConfigResult{
		Profiles:   map[string]AgentProfile{},
		PhaseRoles: map[string]string{},
		SDK: TOMLSDKConfig{
			TranscriptMaxBytes: int64Ptr(-1024),
			TranscriptMaxTurns: int64Ptr(-50),
		},
	}
	cfg := configFromTOML(result)
	assert.Equal(t, int64(0), cfg.SDK.TranscriptMaxBytes, "negative TranscriptMaxBytes clamped to 0")
	assert.Equal(t, int64(0), cfg.SDK.TranscriptMaxTurns, "negative TranscriptMaxTurns clamped to 0")
}

func TestConfigToTOML_SDKRoundTrip(t *testing.T) {
	// configToTOML must always write effective SDK values.
	cfg := DefaultConfig()
	tc := configToTOML(cfg)
	require.NotNil(t, tc.SDK.TranscriptMaxBytes)
	require.NotNil(t, tc.SDK.TranscriptMaxTurns)
	assert.Equal(t, int64(4<<20), *tc.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(2000), *tc.SDK.TranscriptMaxTurns)

	// Also verify non-default explicit values round-trip.
	cfg.SDK.TranscriptMaxBytes = 1 << 20
	cfg.SDK.TranscriptMaxTurns = 500
	tc2 := configToTOML(cfg)
	require.NotNil(t, tc2.SDK.TranscriptMaxBytes)
	require.NotNil(t, tc2.SDK.TranscriptMaxTurns)
	assert.Equal(t, int64(1<<20), *tc2.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(500), *tc2.SDK.TranscriptMaxTurns)
}

func TestConfigToTOML_SDKExplicitZeroRoundTrip(t *testing.T) {
	// Explicit zeros must survive the configToTOML round-trip.
	cfg := DefaultConfig()
	cfg.SDK.TranscriptMaxBytes = 0
	cfg.SDK.TranscriptMaxTurns = 0
	tc := configToTOML(cfg)
	require.NotNil(t, tc.SDK.TranscriptMaxBytes)
	require.NotNil(t, tc.SDK.TranscriptMaxTurns)
	assert.Equal(t, int64(0), *tc.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(0), *tc.SDK.TranscriptMaxTurns)
}

func TestIsTelemetryEnabled(t *testing.T) {
	tests := []struct {
		name     string
		field    *bool
		expected bool
	}{
		{"nil defaults to true", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{TelemetryEnabled: tt.field}
			assert.Equal(t, tt.expected, cfg.IsTelemetryEnabled())
		})
	}
}

func TestDoubleTapThreshold(t *testing.T) {
	tests := []struct {
		name     string
		field    *int
		expected time.Duration
	}{
		{"nil uses default 300ms", nil, 300 * time.Millisecond},
		{"zero uses default 300ms", intPtr(0), 300 * time.Millisecond},
		{"negative uses default 300ms", intPtr(-1), 300 * time.Millisecond},
		{"explicit 200ms", intPtr(200), 200 * time.Millisecond},
		{"explicit 500ms", intPtr(500), 500 * time.Millisecond},
		{"explicit 300ms (same as default)", intPtr(300), 300 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{DoubleTapThresholdMS: tt.field}
			assert.Equal(t, tt.expected, cfg.DoubleTapThreshold())
		})
	}
}

func TestDefaultConfig_DoubleTapThreshold(t *testing.T) {
	cfg := DefaultConfig()
	require.NotNil(t, cfg.DoubleTapThresholdMS)
	assert.Equal(t, 300, *cfg.DoubleTapThresholdMS)
	assert.Equal(t, 300*time.Millisecond, cfg.DoubleTapThreshold())
}

func TestConfigFromTOML_DoubleTapThreshold(t *testing.T) {
	t.Run("explicit threshold round-trips through configFromTOML", func(t *testing.T) {
		ms := 150
		result := &TOMLConfigResult{
			Profiles:             map[string]AgentProfile{},
			PhaseRoles:           map[string]string{},
			DoubleTapThresholdMS: &ms,
		}
		cfg := configFromTOML(result)
		require.NotNil(t, cfg.DoubleTapThresholdMS)
		assert.Equal(t, 150, *cfg.DoubleTapThresholdMS)
		assert.Equal(t, 150*time.Millisecond, cfg.DoubleTapThreshold())
	})

	t.Run("nil threshold from TOML falls back to default in helper", func(t *testing.T) {
		result := &TOMLConfigResult{
			Profiles:   map[string]AgentProfile{},
			PhaseRoles: map[string]string{},
		}
		cfg := configFromTOML(result)
		// DoubleTapThresholdMS is nil (not set in TOML); helper uses 300ms default.
		assert.Equal(t, 300*time.Millisecond, cfg.DoubleTapThreshold())
	})
}

func TestPlannerProfileNames(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		var c *Config
		assert.Nil(t, c.PlannerProfileNames())
	})

	t.Run("nil Planners returns nil (legacy mode)", func(t *testing.T) {
		cfg := &Config{}
		assert.Nil(t, cfg.PlannerProfileNames())
	})

	t.Run("empty Planners returns empty slice", func(t *testing.T) {
		cfg := &Config{Planners: []string{}}
		assert.Empty(t, cfg.PlannerProfileNames())
	})

	t.Run("configured planners returned in order", func(t *testing.T) {
		cfg := &Config{Planners: []string{"planner-a", "planner-b"}}
		assert.Equal(t, []string{"planner-a", "planner-b"}, cfg.PlannerProfileNames())
	})

	t.Run("DefaultConfig has nil Planners (legacy mode)", func(t *testing.T) {
		assert.Nil(t, DefaultConfig().Planners)
	})

	t.Run("absent planners in configFromTOML leaves Planners nil", func(t *testing.T) {
		cfg := configFromTOML(&TOMLConfigResult{
			Profiles:   map[string]AgentProfile{},
			PhaseRoles: map[string]string{},
		})
		assert.Nil(t, cfg.PlannerProfileNames())
	})

	t.Run("explicit planners list round-trips through configFromTOML", func(t *testing.T) {
		cfg := configFromTOML(&TOMLConfigResult{
			Profiles:   map[string]AgentProfile{},
			PhaseRoles: map[string]string{},
			Planners:   []string{"planner-a", "planner-b"},
		})
		assert.Equal(t, []string{"planner-a", "planner-b"}, cfg.PlannerProfileNames())
	})

	t.Run("configToTOML omits planners when nil", func(t *testing.T) {
		tc := configToTOML(&Config{})
		assert.Nil(t, tc.Orchestration.Planners)
	})

	t.Run("configToTOML writes planners when set", func(t *testing.T) {
		tc := configToTOML(&Config{Planners: []string{"planner-a"}})
		assert.Equal(t, []string{"planner-a"}, tc.Orchestration.Planners)
	})
}

func TestResolveNamedProfile(t *testing.T) {
	t.Run("returns false for nil config", func(t *testing.T) {
		var c *Config
		_, ok := c.ResolveNamedProfile("planner", "claude")
		assert.False(t, ok)
	})

	t.Run("returns false when profiles map is nil", func(t *testing.T) {
		cfg := &Config{}
		_, ok := cfg.ResolveNamedProfile("planner", "claude")
		assert.False(t, ok)
	})

	t.Run("returns false when profile not found", func(t *testing.T) {
		cfg := &Config{Profiles: map[string]AgentProfile{"coder": {Program: "opencode", Enabled: true}}}
		_, ok := cfg.ResolveNamedProfile("planner", "claude")
		assert.False(t, ok)
	})

	t.Run("returns false when profile is disabled", func(t *testing.T) {
		cfg := &Config{Profiles: map[string]AgentProfile{
			"planner": {Program: "codex", Enabled: false},
		}}
		_, ok := cfg.ResolveNamedProfile("planner", "claude")
		assert.False(t, ok)
	})

	t.Run("returns false when profile has empty program", func(t *testing.T) {
		cfg := &Config{Profiles: map[string]AgentProfile{
			"planner": {Program: "", Enabled: true},
		}}
		_, ok := cfg.ResolveNamedProfile("planner", "claude")
		assert.False(t, ok)
	})

	t.Run("returns normalised profile when found and enabled", func(t *testing.T) {
		cfg := &Config{Profiles: map[string]AgentProfile{
			"planner": {Program: "codex", Enabled: true, ExecutionMode: "headless", Tier: "default", PermissionDefault: "bypass"},
		}}
		profile, ok := cfg.ResolveNamedProfile("planner", "claude")
		require.True(t, ok)
		assert.Equal(t, "codex", profile.Program)
		assert.Equal(t, ExecutionModeSDK, profile.ExecutionMode) // headless → sdk
		assert.Equal(t, "flex", profile.Tier)                    // default → flex
		assert.Equal(t, PermissionDefaultBypass, profile.PermissionDefault)
	})
}

func TestValidatePlannerProfiles(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		var c *Config
		assert.NoError(t, c.ValidatePlannerProfiles())
	})

	t.Run("empty planners returns nil", func(t *testing.T) {
		cfg := &Config{}
		assert.NoError(t, cfg.ValidatePlannerProfiles())
	})

	t.Run("valid single planner passes", func(t *testing.T) {
		cfg := &Config{
			Planners: []string{"planner-a"},
			Profiles: map[string]AgentProfile{
				"planner-a": {Program: "codex", Enabled: true},
			},
		}
		assert.NoError(t, cfg.ValidatePlannerProfiles())
	})

	t.Run("valid multiple planners passes", func(t *testing.T) {
		cfg := &Config{
			Planners: []string{"planner-a", "planner-b"},
			Profiles: map[string]AgentProfile{
				"planner-a": {Program: "codex", Enabled: true},
				"planner-b": {Program: "claude", Enabled: true},
			},
		}
		assert.NoError(t, cfg.ValidatePlannerProfiles())
	})

	t.Run("empty name rejected", func(t *testing.T) {
		cfg := &Config{
			Planners: []string{""},
			Profiles: map[string]AgentProfile{},
		}
		err := cfg.ValidatePlannerProfiles()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "[orchestration].planners")
	})

	t.Run("name with slash rejected", func(t *testing.T) {
		cfg := &Config{
			Planners: []string{"plan/ner"},
			Profiles: map[string]AgentProfile{},
		}
		err := cfg.ValidatePlannerProfiles()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "plan/ner")
		assert.Contains(t, err.Error(), "[orchestration].planners")
	})

	t.Run("name with backslash rejected", func(t *testing.T) {
		cfg := &Config{
			Planners: []string{`plan\ner`},
			Profiles: map[string]AgentProfile{},
		}
		err := cfg.ValidatePlannerProfiles()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "[orchestration].planners")
	})

	t.Run("name with dotdot rejected", func(t *testing.T) {
		cfg := &Config{
			Planners: []string{"../evil"},
			Profiles: map[string]AgentProfile{},
		}
		err := cfg.ValidatePlannerProfiles()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "[orchestration].planners")
	})

	t.Run("duplicate name rejected after trimming", func(t *testing.T) {
		cfg := &Config{
			Planners: []string{"planner-a", "planner-a"},
			Profiles: map[string]AgentProfile{
				"planner-a": {Program: "codex", Enabled: true},
			},
		}
		err := cfg.ValidatePlannerProfiles()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate")
		assert.Contains(t, err.Error(), "planner-a")
		assert.Contains(t, err.Error(), "[orchestration].planners")
	})

	t.Run("unknown profile rejected", func(t *testing.T) {
		cfg := &Config{
			Planners: []string{"unknown-planner"},
			Profiles: map[string]AgentProfile{
				"coder": {Program: "codex", Enabled: true},
			},
		}
		err := cfg.ValidatePlannerProfiles()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown-planner")
		assert.Contains(t, err.Error(), "[orchestration].planners")
	})

	t.Run("disabled profile rejected", func(t *testing.T) {
		cfg := &Config{
			Planners: []string{"planner-a"},
			Profiles: map[string]AgentProfile{
				"planner-a": {Program: "codex", Enabled: false},
			},
		}
		err := cfg.ValidatePlannerProfiles()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "planner-a")
		assert.Contains(t, err.Error(), "disabled")
		assert.Contains(t, err.Error(), "[orchestration].planners")
	})

	t.Run("nil profiles map with non-empty planners rejected", func(t *testing.T) {
		cfg := &Config{
			Planners: []string{"planner-a"},
		}
		err := cfg.ValidatePlannerProfiles()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "planner-a")
		assert.Contains(t, err.Error(), "[orchestration].planners")
	})
}
