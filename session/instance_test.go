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
