package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

// TestResourcesConfigResolve covers the full validation and preset expansion matrix.
func TestResourcesConfigResolve(t *testing.T) {
	tests := []struct {
		name      string
		cfg       ResourcesConfig
		wantErr   string // substring; empty means no error expected
		wantCheck func(t *testing.T, r ResolvedResourceControls)
	}{
		{
			name: "omitted block resolves to normal disabled",
			cfg:  ResourcesConfig{},
			wantCheck: func(t *testing.T, r ResolvedResourceControls) {
				t.Helper()
				assert.False(t, r.Enabled)
				assert.Equal(t, "normal", r.Profile)
				assert.Equal(t, 0, r.Nice)
				assert.Equal(t, "", r.IoniceClass)
				assert.Equal(t, 0, r.BuildJobs)
				assert.Equal(t, 0, r.MaxParallelWaveTasks)
			},
		},
		{
			name: "profile normal explicit resolves to disabled",
			cfg:  ResourcesConfig{Profile: "normal"},
			wantCheck: func(t *testing.T, r ResolvedResourceControls) {
				t.Helper()
				assert.False(t, r.Enabled)
				assert.Equal(t, "normal", r.Profile)
			},
		},
		{
			name: "interactive preset defaults",
			cfg:  ResourcesConfig{Profile: "interactive"},
			wantCheck: func(t *testing.T, r ResolvedResourceControls) {
				t.Helper()
				assert.True(t, r.Enabled)
				assert.Equal(t, "interactive", r.Profile)
				assert.Equal(t, 10, r.Nice)
				assert.Equal(t, "best-effort", r.IoniceClass)
				assert.Equal(t, 7, r.IoniceLevel)
				assert.Equal(t, 1, r.BuildJobs)
				assert.Equal(t, 1, r.GoPackageParallelism)
				assert.Equal(t, 2, r.GOMAXPROCS)
				assert.Equal(t, 1, r.MaxParallelWaveTasks)
				assert.NotNil(t, r.Env)
			},
		},
		{
			name: "interactive with explicit overrides",
			cfg: ResourcesConfig{
				Profile:              "interactive",
				Nice:                 intPtr(5),
				BuildJobs:            intPtr(2),
				GoPackageParallelism: intPtr(4),
				MaxParallelWaveTasks: intPtr(3),
			},
			wantCheck: func(t *testing.T, r ResolvedResourceControls) {
				t.Helper()
				assert.True(t, r.Enabled)
				assert.Equal(t, "interactive", r.Profile)
				// overridden
				assert.Equal(t, 5, r.Nice)
				assert.Equal(t, 2, r.BuildJobs)
				assert.Equal(t, 4, r.GoPackageParallelism)
				assert.Equal(t, 3, r.MaxParallelWaveTasks)
				// preset values preserved for non-overridden fields
				assert.Equal(t, "best-effort", r.IoniceClass)
				assert.Equal(t, 7, r.IoniceLevel)
				assert.Equal(t, 2, r.GOMAXPROCS)
			},
		},
		{
			name: "custom valid with build_jobs only",
			cfg: ResourcesConfig{
				Profile:   "custom",
				BuildJobs: intPtr(4),
			},
			wantCheck: func(t *testing.T, r ResolvedResourceControls) {
				t.Helper()
				assert.True(t, r.Enabled)
				assert.Equal(t, "custom", r.Profile)
				assert.Equal(t, 4, r.BuildJobs)
				assert.Equal(t, 0, r.Nice)
				assert.Equal(t, "", r.IoniceClass)
			},
		},
		{
			name:    "custom with no control keys is invalid",
			cfg:     ResourcesConfig{Profile: "custom"},
			wantErr: "non-zero/non-empty control key",
		},
		{
			name:    "invalid profile name",
			cfg:     ResourcesConfig{Profile: "aggressive"},
			wantErr: "not a recognised profile",
		},
		{
			name:    "negative nice rejected",
			cfg:     ResourcesConfig{Profile: "interactive", Nice: intPtr(-1)},
			wantErr: "resources.nice",
		},
		{
			name:    "nice out of range high",
			cfg:     ResourcesConfig{Profile: "interactive", Nice: intPtr(20)},
			wantErr: "resources.nice",
		},
		{
			name:    "realtime ionice class rejected",
			cfg:     ResourcesConfig{Profile: "interactive", IoniceClass: strPtr("realtime")},
			wantErr: "realtime",
		},
		{
			name:    "unknown ionice class rejected",
			cfg:     ResourcesConfig{Profile: "interactive", IoniceClass: strPtr("fifo")},
			wantErr: "resources.ionice_class",
		},
		{
			name: "ionice_level valid for best-effort",
			cfg: ResourcesConfig{
				Profile:     "interactive",
				IoniceClass: strPtr("best-effort"),
				IoniceLevel: intPtr(3),
			},
			wantCheck: func(t *testing.T, r ResolvedResourceControls) {
				t.Helper()
				assert.Equal(t, 3, r.IoniceLevel)
			},
		},
		{
			name: "interactive ionice_level override uses preset best-effort class",
			cfg: ResourcesConfig{
				Profile:     "interactive",
				IoniceLevel: intPtr(3),
			},
			wantCheck: func(t *testing.T, r ResolvedResourceControls) {
				t.Helper()
				assert.Equal(t, "best-effort", r.IoniceClass)
				assert.Equal(t, 3, r.IoniceLevel)
			},
		},
		{
			name: "ionice_level 0 on non-best-effort class is ok",
			cfg: ResourcesConfig{
				Profile:     "interactive",
				IoniceClass: strPtr("idle"),
				IoniceLevel: intPtr(0),
			},
			wantCheck: func(t *testing.T, r ResolvedResourceControls) {
				t.Helper()
				assert.Equal(t, "idle", r.IoniceClass)
				assert.Equal(t, 0, r.IoniceLevel)
			},
		},
		{
			name:    "ionice_level non-zero on idle class rejected",
			cfg:     ResourcesConfig{Profile: "interactive", IoniceClass: strPtr("idle"), IoniceLevel: intPtr(3)},
			wantErr: "ionice_level is only valid for ionice_class",
		},
		{
			name:    "negative build_jobs rejected",
			cfg:     ResourcesConfig{Profile: "custom", BuildJobs: intPtr(-1)},
			wantErr: "resources.build_jobs",
		},
		{
			name:    "negative go_package_parallelism rejected",
			cfg:     ResourcesConfig{Profile: "custom", GoPackageParallelism: intPtr(-2)},
			wantErr: "resources.go_package_parallelism",
		},
		{
			name:    "negative gomaxprocs rejected",
			cfg:     ResourcesConfig{Profile: "custom", GOMAXPROCS: intPtr(-1)},
			wantErr: "resources.gomaxprocs",
		},
		{
			name:    "negative max_parallel_wave_tasks rejected",
			cfg:     ResourcesConfig{Profile: "custom", MaxParallelWaveTasks: intPtr(-1)},
			wantErr: "resources.max_parallel_wave_tasks",
		},
		{
			name: "custom with only zero values is invalid",
			cfg: ResourcesConfig{
				Profile:              "custom",
				BuildJobs:            intPtr(0),
				GoPackageParallelism: intPtr(0),
				GOMAXPROCS:           intPtr(0),
				MaxParallelWaveTasks: intPtr(0),
			},
			wantErr: "non-zero/non-empty control key",
		},
		{
			name: "env with valid key is accepted",
			cfg: ResourcesConfig{
				Profile: "interactive",
				Env:     map[string]string{"MY_CUSTOM_VAR": "hello"},
			},
			wantCheck: func(t *testing.T, r ResolvedResourceControls) {
				t.Helper()
				assert.Equal(t, "hello", r.Env["MY_CUSTOM_VAR"])
			},
		},
		{
			name:    "env key with invalid characters rejected",
			cfg:     ResourcesConfig{Profile: "interactive", Env: map[string]string{"invalid-key": "val"}},
			wantErr: "not a valid shell environment variable name",
		},
		{
			name:    "env key overwriting KASMOS_MANAGED rejected",
			cfg:     ResourcesConfig{Profile: "interactive", Env: map[string]string{"KASMOS_MANAGED": "0"}},
			wantErr: "kasmos-managed variable",
		},
		{
			name:    "env key overwriting KASMOS_PROJECT rejected",
			cfg:     ResourcesConfig{Profile: "interactive", Env: map[string]string{"KASMOS_PROJECT": "other"}},
			wantErr: "kasmos-managed variable",
		},
		{
			name:    "env key overwriting KASMOS_RESOURCE_PROFILE rejected",
			cfg:     ResourcesConfig{Profile: "interactive", Env: map[string]string{"KASMOS_RESOURCE_PROFILE": "spoofed"}},
			wantErr: "kasmos-managed variable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := tc.cfg.Resolve()
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.True(t, strings.Contains(err.Error(), tc.wantErr),
					"expected error to contain %q, got: %v", tc.wantErr, err)
				return
			}
			require.NoError(t, err)
			if tc.wantCheck != nil {
				tc.wantCheck(t, r)
			}
		})
	}
}

// TestDefaultResourcesConfig verifies the zero-value default.
func TestDefaultResourcesConfig(t *testing.T) {
	def := DefaultResourcesConfig()
	assert.Equal(t, "", def.Profile)
	assert.Nil(t, def.Nice)
	assert.Nil(t, def.IoniceClass)
	assert.Nil(t, def.BuildJobs)
	assert.Nil(t, def.Env)

	resolved, err := def.Resolve()
	require.NoError(t, err)
	assert.False(t, resolved.Enabled)
	assert.Equal(t, "normal", resolved.Profile)
}

// TestResourcesTOMLRoundTrip verifies TOML load and that an absent [resources] block
// preserves default config and a present block maps every field.
func TestResourcesTOMLRoundTrip(t *testing.T) {
	t.Run("absent resources block keeps normal default", func(t *testing.T) {
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

		cfg := configFromTOML(result)
		assert.Equal(t, "", cfg.Resources.Profile)

		resolved, err := cfg.Resources.Resolve()
		require.NoError(t, err)
		assert.False(t, resolved.Enabled)
		assert.Equal(t, "normal", resolved.Profile)
	})

	t.Run("present resources block maps all fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		tomlPath := filepath.Join(tmpDir, "config.toml")
		content := `
[resources]
profile                 = "interactive"
nice                    = 5
ionice_class            = "best-effort"
ionice_level            = 3
build_jobs              = 2
go_package_parallelism  = 2
gomaxprocs              = 4
max_parallel_wave_tasks = 2

[resources.env]
MYVAR = "hello"
`
		err := os.WriteFile(tomlPath, []byte(content), 0o644)
		require.NoError(t, err)

		result, err := LoadTOMLConfigFrom(tomlPath)
		require.NoError(t, err)

		cfg := configFromTOML(result)
		r := cfg.Resources
		assert.Equal(t, "interactive", r.Profile)
		require.NotNil(t, r.Nice)
		assert.Equal(t, 5, *r.Nice)
		require.NotNil(t, r.IoniceClass)
		assert.Equal(t, "best-effort", *r.IoniceClass)
		require.NotNil(t, r.IoniceLevel)
		assert.Equal(t, 3, *r.IoniceLevel)
		require.NotNil(t, r.BuildJobs)
		assert.Equal(t, 2, *r.BuildJobs)
		require.NotNil(t, r.GoPackageParallelism)
		assert.Equal(t, 2, *r.GoPackageParallelism)
		require.NotNil(t, r.GOMAXPROCS)
		assert.Equal(t, 4, *r.GOMAXPROCS)
		require.NotNil(t, r.MaxParallelWaveTasks)
		assert.Equal(t, 2, *r.MaxParallelWaveTasks)
		assert.Equal(t, "hello", r.Env["MYVAR"])

		resolved, err := r.Resolve()
		require.NoError(t, err)
		assert.True(t, resolved.Enabled)
		assert.Equal(t, "interactive", resolved.Profile)
		assert.Equal(t, 5, resolved.Nice)
		assert.Equal(t, 3, resolved.IoniceLevel)
		assert.Equal(t, 2, resolved.BuildJobs)
		assert.Equal(t, 2, resolved.MaxParallelWaveTasks)
		assert.Equal(t, "hello", resolved.Env["MYVAR"])
	})
}
