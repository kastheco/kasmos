package daemon

import (
	"log/slog"
	"testing"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/session"
	tmuxpkg "github.com/kastheco/kasmos/session/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGatewayNoopOutcome_Characterization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		signalType string
		status     taskstore.SignalStatus
		result     string
	}{
		{name: "implement finished is suppressed", signalType: "implement_finished", status: taskstore.SignalDone, result: "suppressed implement-finished signal"},
		{name: "wave task failures are explicit", signalType: "implement_task_finished", status: taskstore.SignalFailed, result: "no active orchestrator / wrong wave / already-finished task"},
		{name: "wave start failures are explicit", signalType: "implement_wave", status: taskstore.SignalFailed, result: "processor could not start the requested wave"},
		{name: "architect resume failures are explicit", signalType: "elaborator_finished", status: taskstore.SignalFailed, result: "no active architect pass to resume"},
		{name: "unexpected signals are rejected", signalType: "planner_finished", status: taskstore.SignalFailed, result: "signal rejected by processor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, result := gatewayNoopOutcome(&taskstore.SignalEntry{SignalType: tt.signalType})
			assert.Equal(t, tt.status, status)
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestDaemon_AutoAdvanceCompletedImplementer_Characterization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		entryStatus  taskstore.Status
		execState    taskstore.ExecutionState
		inst         session.Instance
		tmuxAlive    bool
		wantAdvanced bool
		wantPushes   int
	}{
		{
			name:         "coder advances when tmux exits",
			entryStatus:  taskstore.StatusImplementing,
			execState:    taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseSingleAgentImplementing), ActiveAgentType: session.AgentTypeCoder},
			inst:         session.Instance{Title: "feature-coder", TaskFile: "feature.md", AgentType: session.AgentTypeCoder},
			tmuxAlive:    false,
			wantAdvanced: true,
			wantPushes:   1,
		},
		{
			name:         "fixer advances when prompt returns and work is finished",
			entryStatus:  taskstore.StatusImplementing,
			execState:    taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseFixing), ActiveAgentType: session.AgentTypeFixer},
			inst:         session.Instance{Title: "feature-fixer", TaskFile: "feature.md", AgentType: session.AgentTypeFixer, PromptDetected: true},
			tmuxAlive:    true,
			wantAdvanced: true,
			wantPushes:   1,
		},
		{
			name:         "prompt with awaiting work does not advance",
			entryStatus:  taskstore.StatusImplementing,
			execState:    taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseFixing), ActiveAgentType: session.AgentTypeFixer},
			inst:         session.Instance{Title: "feature-fixer", TaskFile: "feature.md", AgentType: session.AgentTypeFixer, PromptDetected: true, AwaitingWork: true},
			tmuxAlive:    true,
			wantAdvanced: false,
			wantPushes:   0,
		},
		{
			name:         "solo agents are ignored",
			entryStatus:  taskstore.StatusImplementing,
			execState:    taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseSingleAgentImplementing), ActiveAgentType: session.AgentTypeCoder},
			inst:         session.Instance{Title: "feature-solo", TaskFile: "feature.md", AgentType: session.AgentTypeCoder, SoloAgent: true},
			tmuxAlive:    false,
			wantAdvanced: false,
			wantPushes:   0,
		},
		{
			name:         "wave task agents are ignored",
			entryStatus:  taskstore.StatusImplementing,
			execState:    taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveRunning), ActiveAgentType: session.AgentTypeCoder, ActiveWave: 1},
			inst:         session.Instance{Title: "feature-W1-T1", TaskFile: "feature.md", AgentType: session.AgentTypeCoder, TaskNumber: 1},
			tmuxAlive:    false,
			wantAdvanced: false,
			wantPushes:   0,
		},
		{
			name:         "non implementing plans are ignored",
			entryStatus:  taskstore.StatusReviewing,
			execState:    taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseReviewing), ActiveAgentType: session.AgentTypeReviewer},
			inst:         session.Instance{Title: "feature-fixer", TaskFile: "feature.md", AgentType: session.AgentTypeFixer, PromptDetected: true},
			tmuxAlive:    true,
			wantAdvanced: false,
			wantPushes:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := taskstore.NewTestStore(t)
			const project = "proj"
			require.NoError(t, store.Create(project, taskstore.TaskEntry{
				Filename:       tt.inst.TaskFile,
				Status:         tt.entryStatus,
				Branch:         "plan/feature",
				ExecutionState: tt.execState,
			}))

			pushes := 0
			d := &Daemon{
				logger:      slog.Default(),
				broadcaster: api.NewEventBroadcaster(),
				pushBranch: func(*session.Instance) error {
					pushes++
					return nil
				},
			}

			advanced, err := d.autoAdvanceCompletedImplementer(RepoEntry{Project: project, Store: store}, &tt.inst, tt.tmuxAlive)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAdvanced, advanced)
			assert.Equal(t, tt.wantPushes, pushes)

			entry, err := store.Get(project, tt.inst.TaskFile)
			require.NoError(t, err)
			if tt.wantAdvanced {
				assert.Equal(t, taskstore.StatusReviewing, entry.Status)
				assert.Equal(t, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseReviewing), ActiveAgentType: session.AgentTypeReviewer}, entry.ExecutionState)
			} else {
				assert.Equal(t, tt.entryStatus, entry.Status)
				assert.Equal(t, tt.execState, entry.ExecutionState)
			}
		})
	}
}

func TestDaemon_RecoverSessions_DeduplicatesAndAdoptsKnownTitles(t *testing.T) {
	t.Parallel()

	const project = "proj"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    "feature",
		Status:      taskstore.StatusReviewing,
		Branch:      "plan/feature",
		ReviewCycle: 2,
	}))

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	d.repos.repos = []RepoEntry{{
		Path:    "/tmp/proj",
		Project: project,
		Store:   store,
	}}
	d.spawner.discoverOrphans = func(_ []string) ([]tmuxpkg.SessionInfo, error) {
		return []tmuxpkg.SessionInfo{
			{Title: "feature-architect"},
			{Title: "feature-architect"},
			{Title: "feature-fix-2"},
			{Title: "feature-review-3"},
			{Title: "feature-review-4"},
		}, nil
	}

	var restored []session.InstanceData
	d.spawner.restoreInstance = func(data session.InstanceData) (*session.Instance, error) {
		restored = append(restored, data)
		return &session.Instance{Title: data.Title, Path: data.Path, TaskFile: data.TaskFile, AgentType: data.AgentType}, nil
	}

	recovered, err := d.RecoverSessions()
	require.NoError(t, err)
	assert.Equal(t, 3, recovered, "duplicate and unknown orphan titles should be suppressed")

	gotTitles := make([]string, 0, len(restored))
	for _, data := range restored {
		gotTitles = append(gotTitles, data.Title)
	}
	assert.ElementsMatch(t, []string{"feature-architect", "feature-fix-2", "feature-review-3"}, gotTitles)

	running := d.spawner.RunningInstances()
	require.Len(t, running, 3)
}

func TestDaemon_RecoverSessions_UsesPersistedWaveStateForWaveTasks(t *testing.T) {
	t.Parallel()

	const project = "proj"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "feature",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/feature",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
			ActiveAgentType: session.AgentTypeCoder,
			ActiveWave:      2,
		},
		Content: "# Plan\n\n**Goal:** test\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n\n## Wave 2\n\n### Task 2: Second\n\nDo second.\n",
	}))

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	d.repos.repos = []RepoEntry{{
		Path:    "/tmp/proj",
		Project: project,
		Store:   store,
	}}
	d.spawner.discoverOrphans = func(_ []string) ([]tmuxpkg.SessionInfo, error) {
		return []tmuxpkg.SessionInfo{{Title: "feature-W2-T2"}}, nil
	}

	var restored []session.InstanceData
	d.spawner.restoreInstance = func(data session.InstanceData) (*session.Instance, error) {
		restored = append(restored, data)
		return &session.Instance{Title: data.Title, Path: data.Path, TaskFile: data.TaskFile, AgentType: data.AgentType, TaskNumber: data.TaskNumber, WaveNumber: data.WaveNumber}, nil
	}

	recovered, err := d.RecoverSessions()
	require.NoError(t, err)
	assert.Equal(t, 1, recovered)
	require.Len(t, restored, 1)
	assert.Equal(t, "feature-W2-T2", restored[0].Title)
	assert.Equal(t, 2, restored[0].TaskNumber)
	assert.Equal(t, 2, restored[0].WaveNumber)
}
