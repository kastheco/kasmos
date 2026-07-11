package livestatus

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveWavesInvariant(t *testing.T) {
	content := "## Wave 1\n### Task 1: first\n## Wave 2\n### Task 2: second\n### Task 3: missing\n## Wave 3\n### Task 4: fourth\n"
	subtasks := []taskstore.SubtaskEntry{{TaskNumber: 4, Status: taskstore.SubtaskStatusComplete}, {TaskNumber: 1, Status: taskstore.SubtaskStatusDone}, {TaskNumber: 2, Status: taskstore.SubtaskStatusRunning}}
	waves := DeriveWaves(content, subtasks, 2)
	require.Len(t, waves, 3)
	assert.Equal(t, []WaveTask{{Number: 1, Title: "first", Status: "done"}}, waves[0].Tasks)
	assert.Equal(t, []WaveTask{{Number: 2, Title: "second", Status: "running"}, {Number: 3, Title: "missing", Status: "pending"}}, waves[1].Tasks)
	assert.Equal(t, []WaveTask{{Number: 4, Title: "fourth", Status: "complete"}}, waves[2].Tasks)
	assert.False(t, waves[0].Active)
	assert.True(t, waves[1].Active)
	assert.False(t, waves[2].Active)
}

func TestDeriveWavesWithoutHeadersReturnsNil(t *testing.T) {
	assert.Nil(t, DeriveWaves("### Task 1: flat", nil, 0))
}
