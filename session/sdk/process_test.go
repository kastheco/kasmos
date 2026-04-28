package sdk

import (
	"testing"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/session/resourcecontrol"
	"github.com/stretchr/testify/assert"
)

func TestBuildEnv_IncludesKasmosIdentity(t *testing.T) {
	t.Parallel()

	env := buildEnv(LaunchConfig{
		Name:       "feature-review-1",
		AgentType:  "reviewer",
		Project:    "kasmos",
		TaskNumber: 2,
		WaveNumber: 3,
		PeerCount:  4,
	})

	assert.Contains(t, env, "KASMOS_MANAGED=1")
	assert.Contains(t, env, "KASMOS_INSTANCE_TITLE=feature-review-1")
	assert.Contains(t, env, "KASMOS_AGENT_TYPE=reviewer")
	assert.Contains(t, env, "KASMOS_PROJECT=kasmos")
	assert.Contains(t, env, "KASMOS_TASK=2")
	assert.Contains(t, env, "KASMOS_WAVE=3")
	assert.Contains(t, env, "KASMOS_PEERS=4")
}

// buildProcessEnv mirrors what Process.Start does: apply resource-control
// env on top of the basic kasmos env. Used in tests to avoid a full subprocess.
func buildProcessEnv(cfg LaunchConfig) []string {
	w := resourcecontrol.New(cfg.ResourceControls)
	return w.MergeEnv(buildEnv(cfg))
}

func TestBuildProcessEnv_NormalProfile_NoResourceVars(t *testing.T) {
	t.Parallel()

	env := buildProcessEnv(LaunchConfig{
		Name: "test-session",
		// ResourceControls zero-value: Enabled=false
	})

	for _, kv := range env {
		if len(kv) >= 22 && kv[:22] == "KASMOS_RESOURCE_PROFILE" {
			t.Errorf("normal profile must not include KASMOS_RESOURCE_PROFILE, got %q", kv)
		}
	}
}

func TestBuildProcessEnv_InteractiveProfile_IncludesResourceVars(t *testing.T) {
	t.Parallel()

	env := buildProcessEnv(LaunchConfig{
		Name: "test-session",
		ResourceControls: config.ResolvedResourceControls{
			Enabled:   true,
			Profile:   "interactive",
			BuildJobs: 2,
			Env:       map[string]string{},
		},
	})

	assert.Contains(t, env, "KASMOS_RESOURCE_PROFILE=interactive")
	assert.Contains(t, env, "KASMOS_BUILD_JOBS=2")
}

func TestBuildProcessEnv_ExistingEnvNotOverwritten(t *testing.T) {
	t.Parallel()

	// ExtraEnv explicitly sets KASMOS_BUILD_JOBS=99; resource controls
	// should not overwrite it.
	env := buildProcessEnv(LaunchConfig{
		Name:     "test-session",
		ExtraEnv: []string{"KASMOS_BUILD_JOBS=99"},
		ResourceControls: config.ResolvedResourceControls{
			Enabled:   true,
			Profile:   "interactive",
			BuildJobs: 1,
			Env:       map[string]string{},
		},
	})

	found99 := false
	for _, kv := range env {
		if kv == "KASMOS_BUILD_JOBS=99" {
			found99 = true
		}
		assert.NotEqual(t, "KASMOS_BUILD_JOBS=1", kv,
			"resource control value must not overwrite explicit ExtraEnv value")
	}
	assert.True(t, found99, "ExtraEnv value KASMOS_BUILD_JOBS=99 must be preserved")
}
