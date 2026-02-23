package app

import (
	"testing"

	"github.com/kastheco/klique/config/planparser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWaveOrchestrator(t *testing.T) {
	plan := &planparser.Plan{
		Goal: "test",
		Waves: []planparser.Wave{
			{Number: 1, Tasks: []planparser.Task{
				{Number: 1, Title: "First", Body: "do first"},
				{Number: 2, Title: "Second", Body: "do second"},
			}},
			{Number: 2, Tasks: []planparser.Task{
				{Number: 3, Title: "Third", Body: "do third"},
			}},
		},
	}

	orch := NewWaveOrchestrator("plan.md", plan)
	assert.Equal(t, WaveStateIdle, orch.State())
	assert.Equal(t, 2, orch.TotalWaves())
	assert.Equal(t, 3, orch.TotalTasks())
}

func TestWaveOrchestrator_StartWave(t *testing.T) {
	plan := &planparser.Plan{
		Waves: []planparser.Wave{
			{Number: 1, Tasks: []planparser.Task{
				{Number: 1, Title: "First", Body: "do first"},
			}},
		},
	}

	orch := NewWaveOrchestrator("plan.md", plan)
	tasks := orch.StartNextWave()

	assert.Equal(t, WaveStateRunning, orch.State())
	assert.Equal(t, 1, orch.CurrentWaveNumber())
	require.Len(t, tasks, 1)
	assert.Equal(t, "First", tasks[0].Title)
}

func TestWaveOrchestrator_TaskCompleted(t *testing.T) {
	plan := &planparser.Plan{
		Waves: []planparser.Wave{
			{Number: 1, Tasks: []planparser.Task{
				{Number: 1, Title: "First", Body: "do first"},
				{Number: 2, Title: "Second", Body: "do second"},
			}},
		},
	}

	orch := NewWaveOrchestrator("plan.md", plan)
	orch.StartNextWave()

	assert.False(t, orch.IsCurrentWaveComplete())

	orch.MarkTaskComplete(1)
	assert.False(t, orch.IsCurrentWaveComplete())

	orch.MarkTaskComplete(2)
	assert.True(t, orch.IsCurrentWaveComplete())
	assert.Equal(t, WaveStateAllComplete, orch.State())
}

func TestWaveOrchestrator_TaskFailed(t *testing.T) {
	plan := &planparser.Plan{
		Waves: []planparser.Wave{
			{Number: 1, Tasks: []planparser.Task{
				{Number: 1, Title: "First", Body: "do first"},
				{Number: 2, Title: "Second", Body: "do second"},
			}},
		},
	}

	orch := NewWaveOrchestrator("plan.md", plan)
	orch.StartNextWave()

	orch.MarkTaskFailed(1)
	orch.MarkTaskComplete(2)

	assert.Equal(t, WaveStateAllComplete, orch.State())
	assert.Equal(t, 1, orch.FailedTaskCount())
	assert.Equal(t, 1, orch.CompletedTaskCount())
}

func TestWaveOrchestrator_MultiWaveProgression(t *testing.T) {
	plan := &planparser.Plan{
		Waves: []planparser.Wave{
			{Number: 1, Tasks: []planparser.Task{
				{Number: 1, Title: "First", Body: "do first"},
			}},
			{Number: 2, Tasks: []planparser.Task{
				{Number: 2, Title: "Second", Body: "do second"},
			}},
		},
	}

	orch := NewWaveOrchestrator("plan.md", plan)

	// Wave 1
	orch.StartNextWave()
	orch.MarkTaskComplete(1)
	assert.Equal(t, WaveStateWaveComplete, orch.State())

	// Advance to wave 2
	tasks := orch.StartNextWave()
	assert.Equal(t, WaveStateRunning, orch.State())
	assert.Equal(t, 2, orch.CurrentWaveNumber())
	require.Len(t, tasks, 1)

	// Complete wave 2
	orch.MarkTaskComplete(2)
	assert.Equal(t, WaveStateAllComplete, orch.State())
}

func TestWaveOrchestrator_AllComplete(t *testing.T) {
	plan := &planparser.Plan{
		Waves: []planparser.Wave{
			{Number: 1, Tasks: []planparser.Task{
				{Number: 1, Title: "Only", Body: "do it"},
			}},
		},
	}

	orch := NewWaveOrchestrator("plan.md", plan)
	orch.StartNextWave()
	orch.MarkTaskComplete(1)

	// No more waves — should be AllComplete
	assert.Equal(t, WaveStateAllComplete, orch.State())
}
