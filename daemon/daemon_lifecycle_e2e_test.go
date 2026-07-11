package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/linearreceipt"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	prsvc "github.com/kastheco/kasmos/orchestration/pr"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemon_LifecycleE2E_DaemonPath(t *testing.T) {
	t.Parallel()

	store, err := taskstore.NewSQLiteStore(filepath.Join(t.TempDir(), "taskstore.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	const (
		project  = "proj"
		planFile = "verify-every-fsm-transition"
		branch   = "plan/verify-every-fsm-transition"
		prURL    = "https://github.com/kastheco/kasmos/pull/10"
	)

	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:         planFile,
		Status:           taskstore.StatusReady,
		Branch:           branch,
		Description:      "verify every fsm transition",
		Topic:            "lifecycle",
		LinearIssueID:    "issue-1",
		LinearIdentifier: "KAS-123",
	}))
	require.NoError(t, store.SetContent(project, planFile, `# Plan

**Goal:** characterize the daemon lifecycle end to end

**Architecture:** daemon-routed lifecycle with architect handoff and multi-wave coding

**Tech Stack:** go, sqlite

**Size:** Large

---

## Wave 1

### Task 1: Wire processor handoff

Ensure the planner handoff starts the architect path.

### Task 2: Start the first wave

Launch the first parallel coder tasks.

## Wave 2

### Task 3: Finalize review handoff

Complete the last implementation task and transition into review.
`))

	linearClient := &daemonLinearClient{}
	receiptCfg := daemonLinearReceiptConfig(taskfsm.ImplementFinished)
	receiptHook := linearreceipt.NewHook(receiptCfg, store, linearClient, nil, project)
	hooks := taskfsm.NewHookRegistry()
	hooks.Add(receiptHook, []taskfsm.Event{taskfsm.ImplementFinished})

	proc := loop.NewProcessor(loop.ProcessorConfig{
		Store:       store,
		Project:     project,
		AutoAdvance: true,
		Hooks:       hooks,
	})

	repo := RepoEntry{
		Path:      t.TempDir(),
		Project:   project,
		Store:     store,
		Processor: proc,
		Hooks:     hooks,
	}
	var d *Daemon

	entryState := func() taskstore.TaskEntry {
		entry, getErr := store.Get(project, planFile)
		require.NoError(t, getErr)
		return entry
	}
	assertState := func(wantStatus taskstore.Status, wantState taskstore.ExecutionState) {
		t.Helper()
		entry := entryState()
		assert.Equal(t, wantStatus, entry.Status)
		assert.Equal(t, wantState, entry.ExecutionState)
	}
	execActions := func(actions []loop.Action) {
		t.Helper()
		for _, action := range actions {
			require.NoError(t, d.executeAction(context.Background(), repo, action), "action=%s", action.Kind())
		}
	}

	var (
		mu       sync.Mutex
		events   []string
		phases   = map[string]taskstore.ExecutionState{}
		recorder = func(name string) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, name)
			phases[name] = entryState().ExecutionState
		}
		recorderWithState = func(name string, state taskstore.ExecutionState) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, name)
			phases[name] = state
		}
	)

	d = &Daemon{
		cfg:         &DaemonConfig{AutoAdvance: true, AutoAdvanceWaves: true},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		killAgent: func(repoPath, taskFile, agentType string) error {
			// Pre-kill hook invoked by SpawnPlannerAction before spawning a
			// new planner. Must be a no-op when no existing planner is present.
			assert.Equal(t, repo.Path, repoPath)
			assert.Equal(t, planFile, taskFile)
			assert.Equal(t, session.AgentTypePlanner, agentType)
			recorder("kill:planner:pre-spawn")
			return nil
		},
		spawnPlanner: func(_ context.Context, opts loop.SpawnOpts) error {
			assert.Equal(t, planFile, opts.PlanFile)
			assert.Equal(t, repo.Path, opts.RepoPath)
			assert.Equal(t, project, opts.Project)
			assert.NotEmpty(t, opts.Prompt, "planner spawn must carry a non-empty prompt")
			recorder("spawn:planner")
			return nil
		},
		spawnElaborator: func(_ context.Context, opts loop.SpawnOpts) error {
			assert.Equal(t, planFile, opts.PlanFile)
			assert.Equal(t, repo.Path, opts.RepoPath)
			assert.Equal(t, project, opts.Project)
			recorder("spawn:architect")
			return nil
		},
		spawnWaveTask: func(_ context.Context, opts loop.SpawnOpts, task taskparser.Task, prompt string, _ int, peerCount int) error {
			assert.Equal(t, planFile, opts.PlanFile)
			assert.Equal(t, repo.Path, opts.RepoPath)
			assert.Equal(t, project, opts.Project)
			assert.Equal(t, branch, opts.Branch)
			assert.NotEmpty(t, prompt)
			assert.Greater(t, peerCount, 0)
			recorderWithState(fmt.Sprintf("spawn:wave-%d:task-%d", opts.Wave, task.Number), taskstore.ExecutionState{
				Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
				ActiveAgentType: session.AgentTypeCoder,
				ActiveWave:      opts.Wave,
			})
			return nil
		},
		killWaveAgents: func(repoPath, taskFile string, wave int) error {
			assert.Equal(t, repo.Path, repoPath)
			assert.Equal(t, planFile, taskFile)
			recorder(fmt.Sprintf("kill:wave-%d", wave))
			return nil
		},
		spawnReviewer: func(_ context.Context, opts loop.SpawnOpts) error {
			assert.Equal(t, planFile, opts.PlanFile)
			assert.Equal(t, repo.Path, opts.RepoPath)
			assert.Equal(t, project, opts.Project)
			assert.Equal(t, branch, opts.Branch)
			recorder("spawn:reviewer")
			return nil
		},
		createPR: func(e RepoEntry, gotPlanFile, reviewBody string) (prsvc.Result, error) {
			assert.Equal(t, repo.Project, e.Project)
			assert.Equal(t, planFile, gotPlanFile)
			assert.Equal(t, "lgtm", reviewBody)
			recorder("create:pr")
			return prsvc.Result{Outcome: prsvc.OutcomeCreated, URL: prURL}, store.SetPRURL(project, planFile, prURL)
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })

	planStartActions := proc.ProcessFSMSignals([]taskfsm.Signal{{TaskFile: planFile, Event: taskfsm.PlanStart}})
	require.Len(t, planStartActions, 1, "plan_start must emit SpawnPlannerAction")
	_, isSpawnPlanner := planStartActions[0].(loop.SpawnPlannerAction)
	assert.True(t, isSpawnPlanner, "plan_start action must be SpawnPlannerAction, got %T", planStartActions[0])
	assertState(taskstore.StatusPlanning, taskstore.ExecutionState{})
	execActions(planStartActions)

	plannerFinishedActions := proc.ProcessFSMSignals([]taskfsm.Signal{{TaskFile: planFile, Event: taskfsm.PlannerFinished}})
	require.Len(t, plannerFinishedActions, 2)
	assert.Equal(t, "planner_complete", plannerFinishedActions[0].Kind())
	assert.Equal(t, "auto_implement", plannerFinishedActions[1].Kind())
	assertState(taskstore.StatusReady, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhasePlanned)})

	execActions(plannerFinishedActions[:1])
	assertState(taskstore.StatusReady, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhasePlanned)})

	execActions(plannerFinishedActions[1:])
	assertState(taskstore.StatusImplementing, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseArchitecting),
		ActiveAgentType: session.AgentTypeElaborator,
	})
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseArchitecting),
		ActiveAgentType: session.AgentTypeElaborator,
	}, phases["spawn:architect"])

	elaborationActions := proc.ProcessElaborationSignals([]taskfsm.ElaborationSignal{{TaskFile: planFile}})
	require.Len(t, elaborationActions, 1)
	assert.Equal(t, "advance_wave", elaborationActions[0].Kind())
	execActions(elaborationActions)
	assertState(taskstore.StatusImplementing, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	})
	require.NotNil(t, proc.WaveOrchestrator(planFile))
	assert.Equal(t, orchestration.WaveStateRunning, proc.WaveOrchestrator(planFile).State())
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	}, phases["spawn:wave-1:task-1"])
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	}, phases["spawn:wave-1:task-2"])

	waveOneTaskOne := proc.ProcessTaskSignals([]taskfsm.TaskSignal{{TaskFile: planFile, WaveNumber: 1, TaskNumber: 1}})
	require.Len(t, waveOneTaskOne, 1)
	assert.Equal(t, "task_complete", waveOneTaskOne[0].Kind())
	execActions(waveOneTaskOne)
	assertState(taskstore.StatusImplementing, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	})

	waveOneTaskTwo := proc.ProcessTaskSignals([]taskfsm.TaskSignal{{TaskFile: planFile, WaveNumber: 1, TaskNumber: 2}})
	require.Len(t, waveOneTaskTwo, 1)
	execActions(waveOneTaskTwo)
	assertState(taskstore.StatusImplementing, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      2,
	})
	require.NotNil(t, proc.WaveOrchestrator(planFile))
	assert.Equal(t, 2, proc.WaveOrchestrator(planFile).CurrentWaveNumber())
	assert.Equal(t, orchestration.WaveStateRunning, proc.WaveOrchestrator(planFile).State())
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	}, phases["kill:wave-1"])
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      2,
	}, phases["spawn:wave-2:task-3"])

	waveTwoTaskThree := proc.ProcessTaskSignals([]taskfsm.TaskSignal{{TaskFile: planFile, WaveNumber: 2, TaskNumber: 3}})
	require.Len(t, waveTwoTaskThree, 1)
	execActions(waveTwoTaskThree)
	assertState(taskstore.StatusReviewing, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseReviewing),
		ActiveAgentType: session.AgentTypeReviewer,
	})
	assert.Nil(t, proc.WaveOrchestrator(planFile))
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      2,
	}, phases["kill:wave-2"])
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseReviewing),
		ActiveAgentType: session.AgentTypeReviewer,
	}, phases["spawn:reviewer"])

	beforeApproval := entryState()
	assert.Empty(t, beforeApproval.PRURL)

	reviewApprovedActions := proc.ProcessFSMSignals([]taskfsm.Signal{{TaskFile: planFile, Event: taskfsm.ReviewApproved, Body: "lgtm"}})
	require.Len(t, reviewApprovedActions, 3)
	assert.Equal(t, "review_approved", reviewApprovedActions[0].Kind())
	assert.Equal(t, "verify_approved", reviewApprovedActions[1].Kind())
	assert.Equal(t, "create_pr", reviewApprovedActions[2].Kind())

	execActions(reviewApprovedActions[:1])
	// ReviewApprovedAction is lightweight — it does not clear execution state.
	// The FSM transitions (reviewing→verifying→done, chained by the processor
	// for !AutoReadinessReview) already zero the execution state.
	assertState(taskstore.StatusDone, taskstore.ExecutionState{})
	assert.Empty(t, entryState().PRURL)

	execActions(reviewApprovedActions[1:])
	finalEntry := entryState()
	assert.Equal(t, taskstore.StatusDone, finalEntry.Status)
	assert.Equal(t, taskstore.ExecutionState{}, finalEntry.ExecutionState)
	assert.Equal(t, prURL, finalEntry.PRURL)
	assert.Equal(t, taskstore.ExecutionState{}, phases["create:pr"])

	require.Len(t, events, 10)
	// plan_start now dispatches SpawnPlannerAction which first kills any stale
	// planner (kill:planner:pre-spawn) then spawns the planner (spawn:planner).
	// planner_finished transitions to architect without killing that planner session.
	assert.Equal(t, []string{"kill:planner:pre-spawn", "spawn:planner", "spawn:architect"}, events[:3])
	assert.ElementsMatch(t, []string{"spawn:wave-1:task-1", "spawn:wave-1:task-2"}, events[3:5])
	assert.Equal(t, []string{"kill:wave-1", "spawn:wave-2:task-3", "kill:wave-2", "spawn:reviewer", "create:pr"}, events[5:])
	require.Eventually(t, func() bool {
		return linearClient.createCount() == 1
	}, time.Second, 10*time.Millisecond)
	assert.Contains(t, linearClient.firstComment().body, "kasmos status receipt")
	assert.Contains(t, linearClient.firstComment().body, "event: implement_finished")
}

func TestDaemon_LifecycleE2E_LegacyPlanStartSpawnsOnlyPlanner(t *testing.T) {
	// When planner profiles are unset, plan_start emits exactly one legacy
	// SpawnPlannerAction and no planner draft cache action.
	store := taskstore.NewTestStore(t)
	project := "test-project-false"
	planFile := "serial-plan.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    planFile,
		Status:      taskstore.StatusReady,
		Description: "serial planning run",
	}))

	proc := loop.NewProcessor(loop.ProcessorConfig{
		Store:       store,
		Project:     project,
		AutoAdvance: true,
	})

	planStartActions := proc.ProcessFSMSignals([]taskfsm.Signal{{TaskFile: planFile, Event: taskfsm.PlanStart}})
	require.Len(t, planStartActions, 1, "legacy plan_start must emit only SpawnPlannerAction")
	spawn, isSpawnPlanner := planStartActions[0].(loop.SpawnPlannerAction)
	require.True(t, isSpawnPlanner, "expected SpawnPlannerAction, got %T", planStartActions[0])
	assert.False(t, spawn.DraftMode)
	assert.True(t, spawn.Primary)
}

func TestDaemon_LifecycleE2E_PlannerDraftsAggregateToPlannerCompletion(t *testing.T) {
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "parallel-plan.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    planFile,
		Status:      taskstore.StatusReady,
		Description: "ship parallel planning",
	}))

	proc := loop.NewProcessor(loop.ProcessorConfig{
		Store:            store,
		Project:          project,
		AutoAdvance:      true,
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner-a", "planner-b"},
	})

	planStartActions := proc.ProcessFSMSignals([]taskfsm.Signal{{TaskFile: planFile, Event: taskfsm.PlanStart}})
	require.Len(t, planStartActions, 3)
	assert.Equal(t, "clear_planner_drafts", planStartActions[0].Kind())
	assert.Equal(t, "spawn_planner", planStartActions[1].Kind())
	assert.Equal(t, "spawn_planner", planStartActions[2].Kind())

	entry, err := store.Get(project, planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusPlanning, entry.Status)
	assert.Equal(t, taskstore.ExecutionState{}, entry.ExecutionState)

	firstDraftActions := proc.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{{TaskFile: planFile, PlannerID: "planner-a"}})
	assert.Empty(t, firstDraftActions)

	duplicateDraftActions := proc.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{{TaskFile: planFile, PlannerID: "planner-a"}})
	assert.Empty(t, duplicateDraftActions)

	plannerFinishedActions := proc.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{{TaskFile: planFile, PlannerID: "planner-b"}})
	require.Len(t, plannerFinishedActions, 2)
	assert.Equal(t, "planner_complete", plannerFinishedActions[0].Kind())
	assert.Equal(t, "auto_implement", plannerFinishedActions[1].Kind())
}
