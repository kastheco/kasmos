package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInstance_SoloAgentDefaultsFalse(t *testing.T) {
	inst, err := NewInstance(InstanceOptions{
		Title:   "test",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	assert.False(t, inst.SoloAgent, "SoloAgent must default to false")
}

func TestNewInstance_WaveTaskMetadata(t *testing.T) {
	inst, err := NewInstance(InstanceOptions{
		Title:         "wave-task",
		Path:          t.TempDir(),
		Program:       "claude",
		TaskFile:      "my-plan",
		AgentType:     AgentTypeCoder,
		WaveNumber:    2,
		TaskNumber:    3,
		PeerCount:     4,
		WaveTaskIndex: 2,
		WaveTaskCount: 4,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, inst.WaveTaskIndex, "WaveTaskIndex must be copied from options")
	assert.Equal(t, 4, inst.WaveTaskCount, "WaveTaskCount must be copied from options")
}

func TestNewInstance_WaveTaskMetadataZeroForNonWave(t *testing.T) {
	inst, err := NewInstance(InstanceOptions{
		Title:   "solo",
		Path:    t.TempDir(),
		Program: "opencode",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, inst.WaveTaskIndex, "WaveTaskIndex must default to zero for non-wave instances")
	assert.Equal(t, 0, inst.WaveTaskCount, "WaveTaskCount must default to zero for non-wave instances")
}
