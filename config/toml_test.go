package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTOMLConfig(t *testing.T) {
	t.Run("parses valid TOML with agents and phases", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")

		content := `
[phases]
implementing = "coder"
spec_review = "reviewer"
quality_review = "reviewer"
planning = "planner"
master_review = "master"

	[agents.coder]
	enabled = true
	program = "opencode"
	execution_mode = "headless"
	tier = "default"
	model = "anthropic/claude-sonnet-4-6"
	temperature = 0.7
	effort = "high"
	permission_default = "bypass"
	flags = []

	[agents.reviewer]
	enabled = true
	program = "claude"
	tier = "FAST"
	model = "claude-opus-4-6"
	effort = "high"
	flags = ["--agent", "reviewer"]

[agents.planner]
enabled = false
program = "codex"
model = "gpt-5.3-codex"
flags = []
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)

		tc, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)

		// Verify phases
		assert.Equal(t, "coder", tc.PhaseRoles["implementing"])
		assert.Equal(t, "reviewer", tc.PhaseRoles["spec_review"])
		assert.Equal(t, "planner", tc.PhaseRoles["planning"])
		assert.Equal(t, "master", tc.PhaseRoles["master_review"])

		// Verify agent profiles
		coder, ok := tc.Profiles["coder"]
		require.True(t, ok)
		assert.Equal(t, "opencode", coder.Program)
		assert.Equal(t, "anthropic/claude-sonnet-4-6", coder.Model)
		assert.NotNil(t, coder.Temperature)
		assert.InDelta(t, 0.7, *coder.Temperature, 0.001)
		assert.Equal(t, "high", coder.Effort)
		assert.Equal(t, PermissionDefaultBypass, coder.PermissionDefault)
		// "headless" in config is a legacy alias normalised to "sdk".
		assert.Equal(t, ExecutionModeSDK, coder.ExecutionMode)
		assert.Equal(t, "flex", coder.Tier)
		assert.True(t, coder.Enabled)

		// Verify disabled agent
		planner, ok := tc.Profiles["planner"]
		require.True(t, ok)
		assert.False(t, planner.Enabled)

		// Verify flags preserved
		reviewer, ok := tc.Profiles["reviewer"]
		require.True(t, ok)
		assert.Equal(t, "fast", reviewer.Tier)
		assert.Equal(t, []string{"--agent", "reviewer"}, reviewer.Flags)
	})

	t.Run("normalizes invalid execution_mode in TOML profile", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")

		content := `
		[phases]
		implementing = "coder"

		[agents.coder]
		enabled = true
		program = "opencode"
		execution_mode = "invalid"
		`
		require.NoError(t, os.WriteFile(tomlPath, []byte(content), 0o644))

		tc, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)

		coder, ok := tc.Profiles["coder"]
		require.True(t, ok)
		// Invalid execution_mode falls back to the conservative tmux default.
		assert.Equal(t, ExecutionModeTmux, coder.ExecutionMode)
	})

	t.Run("normalizes invalid tier in TOML profile", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")

		content := `
			[phases]
			implementing = "coder"

			[agents.coder]
			enabled = true
			program = "codex"
			execution_mode = "sdk"
			tier = "priority"
			`
		require.NoError(t, os.WriteFile(tomlPath, []byte(content), 0o644))

		tc, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)

		coder, ok := tc.Profiles["coder"]
		require.True(t, ok)
		assert.Empty(t, coder.Tier)
	})

	t.Run("returns error on missing file", func(t *testing.T) {
		_, err := LoadTOMLConfigFrom("/nonexistent/config.toml")
		assert.Error(t, err)
	})

	t.Run("returns error on invalid TOML", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		err := os.WriteFile(tomlPath, []byte("[invalid toml\n"), 0o644)
		require.NoError(t, err)

		_, err = LoadTOMLConfigFrom(tomlPath)
		assert.Error(t, err)
	})

	t.Run("parses accent color from ui section", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[ui]
accent_color = "#112233"
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)

		result, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		assert.Equal(t, "#112233", result.AccentColor)
	})
}

func TestSaveTOMLConfig(t *testing.T) {
	t.Run("round-trips through save and load", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")

		temp := 0.5
		original := &TOMLConfig{
			Phases: map[string]string{
				"implementing": "coder",
				"planning":     "planner",
			},
			Agents: map[string]TOMLAgent{
				"coder": {
					Enabled:       true,
					Program:       "opencode",
					ExecutionMode: "headless", // legacy alias, normalises to "sdk" on load
					Model:         "anthropic/claude-sonnet-4-6",
					Temperature:   &temp,
					Effort:        "high",
					Flags:         []string{},
				},
			},
		}
		original.UI.AccentColor = "#112233"

		err := SaveTOMLConfigTo(original, tomlPath)
		require.NoError(t, err)

		loaded, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)

		assert.Equal(t, original.Phases, loaded.PhaseRoles)
		coder := loaded.Profiles["coder"]
		assert.Equal(t, "opencode", coder.Program)
		// "headless" in the original config.toml is normalised to "sdk" on load.
		assert.Equal(t, ExecutionModeSDK, coder.ExecutionMode)
		assert.Equal(t, "anthropic/claude-sonnet-4-6", coder.Model)
		assert.InDelta(t, 0.5, *coder.Temperature, 0.001)
		assert.Equal(t, "#112233", loaded.AccentColor)
	})
}

func TestNormalizePermissionDefault(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"inherit", ""},
		{"PROMPT", "prompt"},
		{"  bypass  ", "bypass"},
		{"never", "prompt"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, NormalizePermissionDefault(tc.in))
		})
	}
}

func TestPermissionDefaultRoundTrip(t *testing.T) {
	result := &TOMLConfigResult{
		Profiles: map[string]AgentProfile{
			"coder": {
				Program:           "opencode",
				Enabled:           true,
				PermissionDefault: PermissionDefaultBypass,
			},
		},
		PhaseRoles: map[string]string{"implementing": "coder"},
	}

	cfg := configFromTOML(result)
	tc := configToTOML(cfg)

	assert.Equal(t, PermissionDefaultBypass, tc.Agents["coder"].PermissionDefault)
}

func TestResolveProfile_ExecutionMode(t *testing.T) {
	t.Run("defaults to tmux when unset", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{"implementing": "coder"},
			Profiles: map[string]AgentProfile{
				"coder": {Program: "opencode", Enabled: true},
			},
		}

		profile := cfg.ResolveProfile("implementing", "claude")
		assert.Equal(t, ExecutionModeTmux, profile.ExecutionMode)
	})

	t.Run("headless mode normalises to sdk", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{"implementing": "coder"},
			Profiles: map[string]AgentProfile{
				"coder": {Program: "opencode", Enabled: true, ExecutionMode: "headless"}, // legacy alias
			},
		}

		profile := cfg.ResolveProfile("implementing", "claude")
		// "headless" is a legacy alias that normalises to "sdk".
		assert.Equal(t, ExecutionModeSDK, profile.ExecutionMode)
	})

	t.Run("sdk mode preserved", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{"implementing": "coder"},
			Profiles: map[string]AgentProfile{
				"coder": {Program: "opencode", Enabled: true, ExecutionMode: ExecutionModeSDK},
			},
		}

		profile := cfg.ResolveProfile("implementing", "claude")
		assert.Equal(t, ExecutionModeSDK, profile.ExecutionMode)
	})

	t.Run("tmux mode preserved", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{"implementing": "coder"},
			Profiles: map[string]AgentProfile{
				"coder": {Program: "opencode", Enabled: true, ExecutionMode: ExecutionModeTmux},
			},
		}

		profile := cfg.ResolveProfile("implementing", "claude")
		assert.Equal(t, ExecutionModeTmux, profile.ExecutionMode)
	})

	t.Run("invalid mode normalises to tmux", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{"implementing": "coder"},
			Profiles: map[string]AgentProfile{
				"coder": {Program: "opencode", Enabled: true, ExecutionMode: "invalid"},
			},
		}

		profile := cfg.ResolveProfile("implementing", "claude")
		assert.Equal(t, ExecutionModeTmux, profile.ExecutionMode)
	})
}

func TestAutoReviewFixConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[ui]
auto_review_fix = true
max_review_fix_cycles = 5
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	result, err := LoadTOMLConfigFrom(path)
	require.NoError(t, err)
	require.NotNil(t, result.AutoReviewFix)
	require.NotNil(t, result.MaxReviewFixCycles)
	assert.True(t, *result.AutoReviewFix)
	assert.Equal(t, 5, *result.MaxReviewFixCycles)
}

func TestAutoAdvanceWaves(t *testing.T) {
	t.Run("parses auto_advance_waves from UI section", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[ui]
animate_banner = true
auto_advance_waves = true
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)
		tc, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		require.NotNil(t, tc.AutoAdvanceWaves)
		assert.True(t, *tc.AutoAdvanceWaves)
	})
}

func TestAutoAdvance(t *testing.T) {
	t.Run("parses auto_advance from UI section", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[ui]
auto_advance = true
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)
		tc, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		require.NotNil(t, tc.AutoAdvance)
		assert.True(t, *tc.AutoAdvance)
	})

	t.Run("parses auto_advance false from UI section", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[ui]
auto_advance = false
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)
		tc, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		require.NotNil(t, tc.AutoAdvance)
		assert.False(t, *tc.AutoAdvance)
	})
}

func TestLoadTOMLConfig_Hooks(t *testing.T) {
	tmpDir := t.TempDir()
	tomlPath := filepath.Join(tmpDir, "config.toml")

	content := `
[[hooks]]
type = "webhook"
url = "https://example.com/hook"
events = ["plan_start", "implement_finished"]

[hooks.headers]
X-Token = "secret123"
Authorization = "Bearer tok"

[[hooks]]
type = "notify"
events = ["review_approved"]

[[hooks]]
type = "command"
command = "echo done"
events = ["review_approved"]
`
	err := os.WriteFile(tomlPath, []byte(content), 0o644)
	require.NoError(t, err)

	result, err := LoadTOMLConfigFrom(tomlPath)
	require.NoError(t, err)
	require.Len(t, result.Hooks, 3, "expected three hook entries")

	// First: webhook
	webhook := result.Hooks[0]
	assert.Equal(t, "webhook", webhook.Type)
	assert.Equal(t, "https://example.com/hook", webhook.URL)
	assert.Equal(t, []string{"plan_start", "implement_finished"}, webhook.Events)
	require.NotNil(t, webhook.Headers)
	assert.Equal(t, "secret123", webhook.Headers["X-Token"])
	assert.Equal(t, "Bearer tok", webhook.Headers["Authorization"])

	// Second: notify
	notify := result.Hooks[1]
	assert.Equal(t, "notify", notify.Type)
	assert.Equal(t, []string{"review_approved"}, notify.Events)

	// Third: command
	command := result.Hooks[2]
	assert.Equal(t, "command", command.Type)
	assert.Equal(t, "echo done", command.Command)
	assert.Equal(t, []string{"review_approved"}, command.Events)
}

func TestKeybindsDoubleTapThreshold(t *testing.T) {
	t.Run("parses double_tap_threshold_ms from keybinds section", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[keybinds]
double_tap_threshold_ms = 250
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)

		result, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		require.NotNil(t, result.DoubleTapThresholdMS)
		assert.Equal(t, 250, *result.DoubleTapThresholdMS)
	})

	t.Run("absent keybinds section leaves DoubleTapThresholdMS nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[ui]
animate_banner = false
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)

		result, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		assert.Nil(t, result.DoubleTapThresholdMS)
	})

	t.Run("round-trips keybinds through SaveTOMLConfigTo and LoadTOMLConfigFrom", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")

		ms := 400
		tc := &TOMLConfig{
			Keybinds: TOMLKeybindsConfig{DoubleTapThresholdMS: &ms},
		}

		err := SaveTOMLConfigTo(tc, tomlPath)
		require.NoError(t, err)

		loaded, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		require.NotNil(t, loaded.DoubleTapThresholdMS)
		assert.Equal(t, 400, *loaded.DoubleTapThresholdMS)
	})

	t.Run("default config writes double_tap_threshold_ms to TOML", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")

		def := DefaultConfig()
		err := SaveTOMLConfigTo(configToTOML(def), tomlPath)
		require.NoError(t, err)

		data, err := os.ReadFile(tomlPath)
		require.NoError(t, err)
		assert.Contains(t, string(data), "double_tap_threshold_ms")

		loaded, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		require.NotNil(t, loaded.DoubleTapThresholdMS)
		assert.Equal(t, 300, *loaded.DoubleTapThresholdMS)
	})
}

func TestAutoReadinessReview(t *testing.T) {
	t.Run("explicit false round-trips cleanly", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[ui]
auto_readiness_review = false
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)

		result, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		require.NotNil(t, result.AutoReadinessReview)
		assert.False(t, *result.AutoReadinessReview)
	})

	t.Run("explicit true is loaded correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[ui]
auto_readiness_review = true
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)

		result, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		require.NotNil(t, result.AutoReadinessReview)
		assert.True(t, *result.AutoReadinessReview)
	})

	t.Run("absent key leaves AutoReadinessReview nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[ui]
animate_banner = false
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)

		result, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		assert.Nil(t, result.AutoReadinessReview)
	})
}

func TestMasterReviewBackwardCompat(t *testing.T) {
	t.Run("config with only master_review phase loads correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[phases]
implementing = "coder"
master_review = "master"
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)

		result, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		assert.Equal(t, "master", result.PhaseRoles["master_review"])
		assert.Empty(t, result.PhaseRoles["readiness_review"])
	})

	t.Run("new config uses readiness_review as canonical key", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[phases]
implementing = "coder"
readiness_review = "master"
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)

		result, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)
		assert.Equal(t, "master", result.PhaseRoles["readiness_review"])
		assert.Empty(t, result.PhaseRoles["master_review"])
	})
}

func TestEnforcement(t *testing.T) {
	t.Run("absent [enforcement] section leaves Enforcement nil and all harnesses enabled", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		require.NoError(t, os.WriteFile(path, []byte("[ui]\nanimate_banner = false\n"), 0o644))

		result, err := LoadTOMLConfigFrom(path)
		require.NoError(t, err)
		assert.Nil(t, result.Enforcement)
		assert.True(t, result.IsEnforcementEnabled("claude"))
		assert.True(t, result.IsEnforcementEnabled("codex"))
	})

	t.Run("explicit false disables enforcement for the named harness", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := `
[enforcement]
codex = false
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		result, err := LoadTOMLConfigFrom(path)
		require.NoError(t, err)
		require.NotNil(t, result.Enforcement)
		assert.False(t, result.IsEnforcementEnabled("codex"))
		assert.True(t, result.IsEnforcementEnabled("claude")) // absent key → enabled
	})

	t.Run("explicit true enables enforcement for the named harness", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := `
[enforcement]
claude = true
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		result, err := LoadTOMLConfigFrom(path)
		require.NoError(t, err)
		require.NotNil(t, result.Enforcement)
		assert.True(t, result.IsEnforcementEnabled("claude"))
	})

	t.Run("round-trips [enforcement] through SaveTOMLConfigTo and LoadTOMLConfigFrom", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")

		tc := &TOMLConfig{
			Enforcement: map[string]bool{
				"codex":  false,
				"claude": true,
			},
		}
		require.NoError(t, SaveTOMLConfigTo(tc, path))

		loaded, err := LoadTOMLConfigFrom(path)
		require.NoError(t, err)
		require.NotNil(t, loaded.Enforcement)
		assert.False(t, loaded.IsEnforcementEnabled("codex"))
		assert.True(t, loaded.IsEnforcementEnabled("claude"))
	})

	t.Run("absent [enforcement] section omitted on save (nil map)", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")

		tc := &TOMLConfig{}
		require.NoError(t, SaveTOMLConfigTo(tc, path))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "enforcement")
	})
}

func TestIsEnforcementEnabled(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]bool
		harness  string
		want     bool
	}{
		{"nil map → enabled", nil, "claude", true},
		{"absent key → enabled", map[string]bool{"codex": false}, "claude", true},
		{"explicit false → disabled", map[string]bool{"codex": false}, "codex", false},
		{"explicit true → enabled", map[string]bool{"claude": true}, "claude", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsEnforcementEnabled(tt.settings, tt.harness))
		})
	}
}

func TestSDKConfig_TOMLOmittedTable(t *testing.T) {
	// No [sdk] section — both TOML pointer fields must be nil.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("[ui]\nanimate_banner = false\n"), 0o644))

	result, err := LoadTOMLConfigFrom(path)
	require.NoError(t, err)
	assert.Nil(t, result.SDK.TranscriptMaxBytes, "absent [sdk] table → TranscriptMaxBytes nil")
	assert.Nil(t, result.SDK.TranscriptMaxTurns, "absent [sdk] table → TranscriptMaxTurns nil")
}

func TestSDKConfig_TOMLOmittedIndividualKeys(t *testing.T) {
	// [sdk] present but only one key — the other must be nil.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[sdk]
transcript_max_bytes = 1048576
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	result, err := LoadTOMLConfigFrom(path)
	require.NoError(t, err)
	require.NotNil(t, result.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(1<<20), *result.SDK.TranscriptMaxBytes)
	assert.Nil(t, result.SDK.TranscriptMaxTurns, "absent transcript_max_turns → nil")
}

func TestSDKConfig_TOMLExplicitValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[sdk]
transcript_max_bytes = 8388608
transcript_max_turns = 1000
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	result, err := LoadTOMLConfigFrom(path)
	require.NoError(t, err)
	require.NotNil(t, result.SDK.TranscriptMaxBytes)
	require.NotNil(t, result.SDK.TranscriptMaxTurns)
	assert.Equal(t, int64(8<<20), *result.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(1000), *result.SDK.TranscriptMaxTurns)
}

func TestSDKConfig_TOMLExplicitZeros(t *testing.T) {
	// Explicit zeros must arrive as non-nil pointers to zero.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[sdk]
transcript_max_bytes = 0
transcript_max_turns = 0
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	result, err := LoadTOMLConfigFrom(path)
	require.NoError(t, err)
	require.NotNil(t, result.SDK.TranscriptMaxBytes)
	require.NotNil(t, result.SDK.TranscriptMaxTurns)
	assert.Equal(t, int64(0), *result.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(0), *result.SDK.TranscriptMaxTurns)
}

func TestSDKConfig_SaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	maxBytes := int64(2 << 20)
	maxTurns := int64(500)
	tc := &TOMLConfig{
		SDK: TOMLSDKConfig{
			TranscriptMaxBytes: &maxBytes,
			TranscriptMaxTurns: &maxTurns,
		},
	}

	require.NoError(t, SaveTOMLConfigTo(tc, path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "[sdk]")
	assert.Contains(t, string(data), "transcript_max_bytes")
	assert.Contains(t, string(data), "transcript_max_turns")

	loaded, err := LoadTOMLConfigFrom(path)
	require.NoError(t, err)
	require.NotNil(t, loaded.SDK.TranscriptMaxBytes)
	require.NotNil(t, loaded.SDK.TranscriptMaxTurns)
	assert.Equal(t, int64(2<<20), *loaded.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(500), *loaded.SDK.TranscriptMaxTurns)
}

func TestSDKConfig_DefaultConfigWritesSDK(t *testing.T) {
	// SaveTOMLConfigTo(configToTOML(DefaultConfig())) must include [sdk] with defaults.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	def := DefaultConfig()
	require.NoError(t, SaveTOMLConfigTo(configToTOML(def), path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "[sdk]", "default config should write [sdk] section")
	assert.Contains(t, string(data), "transcript_max_bytes")
	assert.Contains(t, string(data), "transcript_max_turns")

	loaded, err := LoadTOMLConfigFrom(path)
	require.NoError(t, err)
	require.NotNil(t, loaded.SDK.TranscriptMaxBytes)
	require.NotNil(t, loaded.SDK.TranscriptMaxTurns)
	assert.Equal(t, int64(4<<20), *loaded.SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(2000), *loaded.SDK.TranscriptMaxTurns)
}

func TestLoadHooksForRepo(t *testing.T) {
	t.Run("returns nil when config.toml absent", func(t *testing.T) {
		repoDir := t.TempDir()
		hooks, err := LoadHooksForRepo(repoDir)
		require.NoError(t, err)
		assert.Nil(t, hooks)
	})

	t.Run("returns hooks from repo-local config.toml", func(t *testing.T) {
		repoDir := t.TempDir()
		kasmosDir := filepath.Join(repoDir, ".kasmos")
		require.NoError(t, os.MkdirAll(kasmosDir, 0o755))

		content := `
[[hooks]]
type = "webhook"
url = "https://example.com/hook"
events = ["plan_start"]

[[hooks]]
type = "notify"
`
		require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(content), 0o644))

		hooks, err := LoadHooksForRepo(repoDir)
		require.NoError(t, err)
		require.Len(t, hooks, 2)
		assert.Equal(t, "webhook", hooks[0].Type)
		assert.Equal(t, "https://example.com/hook", hooks[0].URL)
		assert.Equal(t, "notify", hooks[1].Type)
	})

	t.Run("returns error on invalid TOML", func(t *testing.T) {
		repoDir := t.TempDir()
		kasmosDir := filepath.Join(repoDir, ".kasmos")
		require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte("[bad toml\n"), 0o644))

		_, err := LoadHooksForRepo(repoDir)
		assert.Error(t, err)
	})
}

func TestLoadTOMLConfigFrom_RuntimeFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
default_program = "/usr/bin/claude"
auto_yes = true
daemon_poll_interval = 2000
branch_prefix = "dev/"
notifications_enabled = false

[phases]
plan = "planner"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	result, err := LoadTOMLConfigFrom(path)
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/claude", result.DefaultProgram)
	assert.True(t, result.AutoYes)
	assert.Equal(t, 2000, result.DaemonPollInterval)
	assert.Equal(t, "dev/", result.BranchPrefix)
	require.NotNil(t, result.NotificationsEnabled)
	assert.False(t, *result.NotificationsEnabled)
	assert.Equal(t, "planner", result.PhaseRoles["plan"])
}

func TestPlannersTOML(t *testing.T) {
	t.Run("absent planners key leaves Planners nil", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		require.NoError(t, os.WriteFile(path, []byte("[orchestration]\n"), 0o644))

		result, err := LoadTOMLConfigFrom(path)
		require.NoError(t, err)
		assert.Nil(t, result.Planners)
	})

	t.Run("empty planners list parses as empty slice", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := `
[orchestration]
planners = []
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		result, err := LoadTOMLConfigFrom(path)
		require.NoError(t, err)
		assert.Empty(t, result.Planners)
	})

	t.Run("single explicit planner parses correctly", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := `
[orchestration]
planners = ["planner-a"]
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		result, err := LoadTOMLConfigFrom(path)
		require.NoError(t, err)
		require.Len(t, result.Planners, 1)
		assert.Equal(t, "planner-a", result.Planners[0])
	})

	t.Run("multiple planners preserve order", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := `
[orchestration]
planners = ["planner-a", "planner-b", "planner-c"]
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		result, err := LoadTOMLConfigFrom(path)
		require.NoError(t, err)
		require.Len(t, result.Planners, 3)
		assert.Equal(t, []string{"planner-a", "planner-b", "planner-c"}, result.Planners)
	})

	t.Run("planners round-trip through SaveTOMLConfigTo and LoadTOMLConfigFrom", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		tc := &TOMLConfig{
			Orchestration: TOMLOrchestrationConfig{
				Planners: []string{"planner-a", "planner-b"},
			},
		}

		require.NoError(t, SaveTOMLConfigTo(tc, path))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(data), "[orchestration]")
		assert.Contains(t, string(data), "planners")
		assert.NotContains(t, string(data), "parallel_planner_architect")

		loaded, err := LoadTOMLConfigFrom(path)
		require.NoError(t, err)
		assert.Equal(t, []string{"planner-a", "planner-b"}, loaded.Planners)
	})

	t.Run("legacy parallel_planner_architect key loads without error and is ignored", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := `
[orchestration]
parallel_planner_architect = true
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		// Should not error; legacy key is silently dropped with a warning.
		result, err := LoadTOMLConfigFrom(path)
		require.NoError(t, err)
		// Planners is not set by the legacy key.
		assert.Nil(t, result.Planners)
	})

	t.Run("saved config never includes parallel_planner_architect", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")

		def := DefaultConfig()
		require.NoError(t, SaveTOMLConfigTo(configToTOML(def), path))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "parallel_planner_architect")
	})
}

func TestResolveProfileWithDisabledAgent(t *testing.T) {
	t.Run("disabled agent falls back to default", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{"planning": "planner"},
			Profiles: map[string]AgentProfile{
				"planner": {Program: "codex", Enabled: false},
			},
		}
		profile := cfg.ResolveProfile("planning", "claude")
		assert.Equal(t, "claude", profile.Program)
	})

	t.Run("enabled agent resolves normally", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{"implementing": "coder"},
			Profiles: map[string]AgentProfile{
				"coder": {Program: "opencode", Enabled: true},
			},
		}
		profile := cfg.ResolveProfile("implementing", "claude")
		assert.Equal(t, "opencode", profile.Program)
	})

	t.Run("spec_review phase resolves to reviewer profile", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{
				"implementing": "coder",
				"spec_review":  "reviewer",
			},
			Profiles: map[string]AgentProfile{
				"coder":    {Program: "opencode", Enabled: true},
				"reviewer": {Program: "claude", Enabled: true, Flags: []string{"--model", "opus"}},
			},
		}
		profile := cfg.ResolveProfile("spec_review", "opencode")
		assert.Equal(t, "claude", profile.Program)
		assert.Equal(t, "claude --model opus", profile.BuildCommand())
	})

	t.Run("spec_review falls back when no reviewer configured", func(t *testing.T) {
		cfg := &Config{
			PhaseRoles: map[string]string{"implementing": "coder"},
			Profiles: map[string]AgentProfile{
				"coder": {Program: "opencode", Enabled: true},
			},
		}
		profile := cfg.ResolveProfile("spec_review", "opencode")
		assert.Equal(t, "opencode", profile.Program)
	})
}
