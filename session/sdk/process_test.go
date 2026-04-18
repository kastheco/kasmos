package sdk

import (
	"testing"

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
