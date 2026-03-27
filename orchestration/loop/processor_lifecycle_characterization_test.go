package loop

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessor_LifecycleSignalMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		entry      taskstore.TaskEntry
		signal     taskfsm.Signal
		autoFix    bool
		wantStatus taskstore.Status
		wantKinds  []string
	}{
		{
			name:       "planner finished",
			entry:      taskstore.TaskEntry{Filename: "plan.md", Status: taskstore.StatusPlanning},
			signal:     taskfsm.Signal{TaskFile: "plan.md", Event: taskfsm.PlannerFinished},
			wantStatus: taskstore.StatusReady,
			wantKinds:  []string{"planner_complete"},
		},
		{
			name:       "implement finished",
			entry:      taskstore.TaskEntry{Filename: "plan.md", Status: taskstore.StatusImplementing, Branch: "plan/plan"},
			signal:     taskfsm.Signal{TaskFile: "plan.md", Event: taskfsm.ImplementFinished},
			wantStatus: taskstore.StatusReviewing,
			wantKinds:  []string{"spawn_reviewer"},
		},
		{
			name:       "review approved",
			entry:      taskstore.TaskEntry{Filename: "plan.md", Status: taskstore.StatusReviewing, Branch: "plan/plan"},
			signal:     taskfsm.Signal{TaskFile: "plan.md", Event: taskfsm.ReviewApproved, Body: "lgtm"},
			wantStatus: taskstore.StatusDone,
			wantKinds:  []string{"review_approved", "create_pr"},
		},
		{
			name:       "review changes requested",
			entry:      taskstore.TaskEntry{Filename: "plan.md", Status: taskstore.StatusReviewing, Branch: "plan/plan"},
			signal:     taskfsm.Signal{TaskFile: "plan.md", Event: taskfsm.ReviewChangesRequested, Body: "fix the tests"},
			autoFix:    true,
			wantStatus: taskstore.StatusImplementing,
			wantKinds:  []string{"review_changes", "increment_review_cycle", "spawn_fixer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := taskstore.NewSQLiteStore(":memory:")
			require.NoError(t, err)
			defer store.Close()

			const project = "proj"
			require.NoError(t, store.Create(project, tt.entry))
			p := NewProcessor(ProcessorConfig{Store: store, Project: project, AutoReviewFix: tt.autoFix})

			actions := p.ProcessFSMSignals([]taskfsm.Signal{tt.signal})
			gotKinds := make([]string, 0, len(actions))
			for _, action := range actions {
				gotKinds = append(gotKinds, action.Kind())
			}
			assert.Equal(t, tt.wantKinds, gotKinds)

			entry, err := store.Get(project, tt.entry.Filename)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, entry.Status)
		})
	}
}

func TestProcessor_ProcessFSMSignals_SuppressesImplementFinishedWhenWaveOwnershipExists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(*Processor)
	}{
		{
			name: "active orchestrator flag suppresses signal",
			setup: func(p *Processor) {
				p.SetWaveOrchestratorActive("plan.md", true)
			},
		},
		{
			name: "registered orchestrator suppresses signal",
			setup: func(p *Processor) {
				p.RegisterOrchestrator("plan.md", 1, []int{1})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := taskstore.NewSQLiteStore(":memory:")
			require.NoError(t, err)
			defer store.Close()

			const project = "proj"
			require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: "plan.md", Status: taskstore.StatusImplementing}))
			p := NewProcessor(ProcessorConfig{Store: store, Project: project})
			tt.setup(p)

			actions := p.ProcessFSMSignals([]taskfsm.Signal{{TaskFile: "plan.md", Event: taskfsm.ImplementFinished}})
			assert.Empty(t, actions)

			entry, err := store.Get(project, "plan.md")
			require.NoError(t, err)
			assert.Equal(t, taskstore.StatusImplementing, entry.Status)
		})
	}
}

func TestProcessor_ProcessElaborationSignals_ResumesArchitectWaveOne(t *testing.T) {
	t.Parallel()

	store, err := taskstore.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	const (
		project  = "proj"
		planFile = "plan.md"
	)
	content := "# Plan\n\n**Goal:** test\n\n## Wave 1\n\n### Task 1: First\n\nDo it.\n"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: planFile, Status: taskstore.StatusImplementing}))
	require.NoError(t, store.SetContent(project, planFile, content))

	plan, err := taskparser.Parse(content)
	require.NoError(t, err)
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.SetStore(store, project)
	orch.SetElaborating()

	p := NewProcessor(ProcessorConfig{Store: store, Project: project})
	p.waveOrchestrators[planFile] = orch

	actions := p.ProcessElaborationSignals([]taskfsm.ElaborationSignal{{TaskFile: planFile}})
	require.Len(t, actions, 1)
	advance, ok := actions[0].(AdvanceWaveAction)
	require.True(t, ok)
	assert.Equal(t, planFile, advance.PlanFile)
	assert.Equal(t, 1, advance.Wave)
	assert.Equal(t, orchestration.WaveStateRunning, orch.State())
	assert.Equal(t, 1, orch.CurrentWaveNumber())
	require.NotEmpty(t, orch.CurrentWaveTasks())
}
