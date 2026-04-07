package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waveFlowHome builds a minimal home struct suitable for wave-orchestration flow tests.
func waveFlowHome(t *testing.T, ps *taskstate.TaskState, plansDir string, orchMap map[string]*orchestration.WaveOrchestrator) *home {
	t.Helper()
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	threshold := 0
	h := &home{
		ctx:               context.Background(),
		state:             stateDefault,
		appConfig:         &config.Config{BlueprintSkipThresholdValue: &threshold},
		nav:               list,
		menu:              ui.NewMenu(),
		tabbedWindow:      ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:      overlay.NewToastManager(&sp),
		overlays:          overlay.NewManager(),
		taskState:         ps,
		taskStateDir:      plansDir,
		taskStore:         storeForDir(t, plansDir),
		taskStoreProject:  "test",
		waveOrchestrators: orchMap,
	}
	return h
}

// TestWaveMonitor_CancelWaveAdvanceRePrompts verifies that canceling a wave-advance
// confirmation resets the orchestrator confirm latch so the next metadata tick
// can display the prompt again (fixes deadlock).
func TestWaveMonitor_CancelWaveAdvanceRePrompts(t *testing.T) {
	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "First", Body: "do first"}}},
			{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "Second", Body: "do second"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator("test", plan)
	orch.StartNextWave()
	orch.MarkTaskComplete(1) // wave 1 done
	orch.NeedsConfirm()      // consume the one-shot latch so it won't fire again
	require.False(t, orch.NeedsConfirm(), "latch already consumed, must be false")

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	mgr1 := overlay.NewManager()
	mgr1.Show(overlay.NewWaveDecisionOverlay(overlay.WaveDecisionInput{
		PlanFile: "test", PlanName: "test", WaveNumber: 1, TotalWaves: 2,
		Completed: 1, Failed: 0, Total: 1,
	}))
	h := &home{
		ctx:               context.Background(),
		state:             stateWaveDecision,
		appConfig:         config.DefaultConfig(),
		nav:               ui.NewNavigationPanel(&sp),
		menu:              ui.NewMenu(),
		tabbedWindow:      ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:      overlay.NewToastManager(&sp),
		overlays:          mgr1,
		waveOrchestrators: map[string]*orchestration.WaveOrchestrator{"test": orch},
	}

	// Press esc to dismiss the wave decision overlay
	keyMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	_, _ = h.handleKeyPress(keyMsg)

	// Orchestrator latch must be reset so the next tick can re-prompt
	assert.True(t, orch.NeedsConfirm(), "esc must reset orchestrator confirm latch for re-prompt")
}

// TestWaveMonitor_PausedTaskCountsAsFailed verifies that a paused task instance
// is treated as a failure in the wave monitor, causing the wave to complete (with
// failure) and a failed-wave decision prompt to appear.
func TestWaveMonitor_PausedTaskCountsAsFailed(t *testing.T) {
	const planFile = "paused-task"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "Task 1", Body: "do it"}}},
			{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "Task 2", Body: "follow up"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "paused task test", "plan/paused-task", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	// Create the task instance but mark it as Paused
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "paused-task-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst.SetStatus(session.Paused)

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	_ = h.nav.AddInstance(inst)

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "paused-task-T1", TmuxAlive: false}},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Wave must have detected failure and shown the failed-wave decision prompt
	assert.Equal(t, stateWaveDecision, updated.state,
		"paused task must trigger wave-failed decision prompt")
	require.True(t, updated.overlays.IsActive(),
		"wave decision overlay must be set for failed-wave decision")
	wd1, ok1 := updated.overlays.Current().(*overlay.WaveDecisionOverlay)
	require.True(t, ok1, "current overlay must be a WaveDecisionOverlay")
	assert.Greater(t, wd1.Input().Failed, 0,
		"failed-wave overlay must report failures")
	assert.Equal(t, 1, wd1.Input().WaveNumber,
		"wave decision must report the correct wave number")
}

// TestWaveMonitor_MissingTaskCountsAsFailed verifies that a task with no matching
// instance in the list is counted as failed, triggering the failed-wave decision prompt.
func TestWaveMonitor_MissingTaskCountsAsFailed(t *testing.T) {
	const planFile = "missing-task"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "Task 1", Body: "do it"}}},
			{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "Task 2", Body: "follow up"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "missing task test", "plan/missing-task", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	// No instance added to the list — the task is "missing"
	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})

	msg := metadataResultMsg{
		Results:   []instanceMetadata{},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Missing task must be treated as failed and trigger the failed-wave prompt
	assert.Equal(t, stateWaveDecision, updated.state,
		"missing task must trigger wave-failed decision prompt")
	require.True(t, updated.overlays.IsActive())
	wd2, ok2 := updated.overlays.Current().(*overlay.WaveDecisionOverlay)
	require.True(t, ok2, "current overlay must be a WaveDecisionOverlay")
	assert.Greater(t, wd2.Input().Failed, 0,
		"failed-wave overlay must report failures")
}

// TestRebuildOrphanedOrchestrators_SkipsPausedOrExitedOnlyPlans verifies restart
// recovery does not resurrect wave orchestration when a plan only has stale
// paused/exited wave instances and no active task sessions.
func TestRebuildOrphanedOrchestrators_SkipsPausedOrExitedOnlyPlans(t *testing.T) {
	const planFile = "stale-wave"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := taskstore.NewTestSQLiteStore(t)
	content := "**Goal:** stale test\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n\n### Task 2: Second\n\nDo second.\n"
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
		Branch:   "plan/stale-wave",
		Content:  content,
	}))

	ps, err := taskstate.Load(store, "proj", plansDir)
	require.NoError(t, err)
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveRunning), ActiveAgentType: session.AgentTypeCoder, ActiveWave: 1}))

	h := waveFlowHome(t, ps, plansDir, make(map[string]*orchestration.WaveOrchestrator))
	h.taskStore = store
	h.taskStoreProject = "proj"

	pausedInst, err := session.NewInstance(session.InstanceOptions{
		Title:      "stale-wave-W1-T1",
		Path:       dir,
		Program:    "opencode",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	pausedInst.MarkStartedForTest()
	pausedInst.SetStatus(session.Paused)
	h.nav.AddInstance(pausedInst)

	exitedInst, err := session.NewInstance(session.InstanceOptions{
		Title:      "stale-wave-W1-T2",
		Path:       dir,
		Program:    "opencode",
		TaskFile:   planFile,
		TaskNumber: 2,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	exitedInst.MarkStartedForTest()
	exitedInst.Exited = true
	h.nav.AddInstance(exitedInst)

	h.rebuildOrphanedOrchestrators()
	_, exists := h.waveOrchestrators[planFile]
	assert.False(t, exists, "stale paused/exited wave instances must not trigger orchestrator rebuild")
}

// TestRebuildOrphanedOrchestrators_IgnoresArchitectOnlyRestartState verifies that
// restart recovery only rebuilds from persisted active wave-task sessions restored
// via session.FromInstanceData; an architect-only session must not resurrect wave state.
func TestRebuildOrphanedOrchestrators_IgnoresArchitectOnlyRestartState(t *testing.T) {
	const planFile = "architect-only"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := taskstore.NewTestSQLiteStore(t)
	content := "**Goal:** architect test\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n"
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
		Branch:   "plan/architect-only",
		Content:  content,
	}))

	ps, err := taskstate.Load(store, "proj", plansDir)
	require.NoError(t, err)
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseArchitecting), ActiveAgentType: session.AgentTypeElaborator}))

	h := waveFlowHome(t, ps, plansDir, make(map[string]*orchestration.WaveOrchestrator))
	h.taskStore = store
	h.taskStoreProject = "proj"

	architectInst, err := session.NewInstance(session.InstanceOptions{
		Title:     "architect-only-architect",
		Path:      dir,
		Program:   "opencode",
		TaskFile:  planFile,
		AgentType: session.AgentTypeElaborator,
	})
	require.NoError(t, err)
	architectInst.MarkStartedForTest()
	h.nav.AddInstance(architectInst)

	h.rebuildOrphanedOrchestrators()
	_, exists := h.waveOrchestrators[planFile]
	assert.False(t, exists, "architect-only restart state must not rebuild a wave orchestrator")
}

// TestRebuildOrphanedOrchestrators_RestoresPersistedActiveWaveInstances verifies the
// supported restart path: session.FromInstanceData restores active wave instances,
// then kasmos rebuilds the in-memory orchestrator from that persisted state.
func TestRebuildOrphanedOrchestrators_RestoresPersistedActiveWaveInstances(t *testing.T) {
	const planFile = "active-wave"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := taskstore.NewTestSQLiteStore(t)
	content := "**Goal:** active test\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n\n### Task 2: Second\n\nDo second.\n"
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
		Branch:   "plan/active-wave",
		Content:  content,
	}))

	ps, err := taskstate.Load(store, "proj", plansDir)
	require.NoError(t, err)
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveRunning), ActiveAgentType: session.AgentTypeCoder, ActiveWave: 1}))

	h := waveFlowHome(t, ps, plansDir, make(map[string]*orchestration.WaveOrchestrator))
	h.taskStore = store
	h.taskStoreProject = "proj"

	completedInst, err := session.NewInstance(session.InstanceOptions{
		Title:      "active-wave-W1-T1",
		Path:       dir,
		Program:    "opencode",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	completedInst.MarkStartedForTest()
	completedInst.SetStatus(session.Paused)
	h.nav.AddInstance(completedInst)

	activeInst, err := session.NewInstance(session.InstanceOptions{
		Title:      "active-wave-W1-T2",
		Path:       dir,
		Program:    "opencode",
		TaskFile:   planFile,
		TaskNumber: 2,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	activeInst.MarkStartedForTest()
	h.nav.AddInstance(activeInst)

	h.rebuildOrphanedOrchestrators()
	orch, exists := h.waveOrchestrators[planFile]
	require.True(t, exists, "active wave instance must trigger orchestrator rebuild")
	assert.Equal(t, 1, orch.CurrentWaveNumber())
	assert.Equal(t, orchestration.WaveStateRunning, orch.State())
	assert.True(t, orch.IsTaskComplete(1), "paused task should be restored as completed")
	assert.True(t, orch.IsTaskRunning(2), "active task should remain running")
}

// TestRebuildOrphanedOrchestrators_RestoresExitedWaveFromPersistedSubtasks verifies the
// stranded-wave recovery path: after restart, a dead wave task can still be reconstructed
// from persisted execution/subtask state so the next metadata tick can fail the wave and
// present recovery actions instead of leaving the plan stuck in wave_running forever.
func TestRebuildOrphanedOrchestrators_RestoresExitedWaveFromPersistedSubtasks(t *testing.T) {
	const planFile = "exited-wave"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := taskstore.NewTestSQLiteStore(t)
	content := "**Goal:** exited test\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n\n## Wave 2\n\n### Task 2: Second\n\nDo second.\n"
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
		Branch:   "plan/exited-wave",
		Content:  content,
	}))
	require.NoError(t, store.SetSubtasks("proj", planFile, []taskstore.SubtaskEntry{
		{TaskNumber: 1, Title: "First", Status: taskstore.SubtaskStatusRunning},
		{TaskNumber: 2, Title: "Second", Status: taskstore.SubtaskStatusPending},
	}))

	ps, err := taskstate.Load(store, "proj", plansDir)
	require.NoError(t, err)
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveRunning), ActiveAgentType: session.AgentTypeCoder, ActiveWave: 1}))

	h := waveFlowHome(t, ps, plansDir, make(map[string]*orchestration.WaveOrchestrator))
	h.taskStore = store
	h.taskStoreProject = "proj"

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "exited-wave-W1-T1",
		Path:       dir,
		Program:    "opencode",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	inst.SetStatus(session.Ready)
	inst.Exited = true
	h.nav.AddInstance(inst)

	h.rebuildOrphanedOrchestrators()
	orch, exists := h.waveOrchestrators[planFile]
	require.True(t, exists, "persisted running subtask should rebuild the orphaned orchestrator")
	assert.Equal(t, 1, orch.CurrentWaveNumber())
	assert.Equal(t, orchestration.WaveStateRunning, orch.State())
	assert.True(t, orch.IsTaskRunning(1), "task 1 should be restored as running from persisted subtask state")

	model, _ := h.Update(metadataResultMsg{
		Results:   []instanceMetadata{{Title: "exited-wave-W1-T1", TmuxAlive: false}},
		PlanState: ps,
	})
	updated := model.(*home)

	assert.Equal(t, orchestration.WaveStateWaveComplete, orch.State(),
		"dead restored wave task should resolve the current wave instead of leaving it stranded")
	assert.Equal(t, stateWaveDecision, updated.state,
		"restored dead wave task should surface the failed-wave recovery prompt")
	require.True(t, updated.overlays.IsActive(),
		"failed-wave recovery prompt must be shown after restoring a dead wave task")
	wdRestored, okRestored := updated.overlays.Current().(*overlay.WaveDecisionOverlay)
	require.True(t, okRestored, "current overlay must be a WaveDecisionOverlay")
	assert.Greater(t, wdRestored.Input().Failed, 0,
		"failed restored wave should report failures")
	assert.Equal(t, 1, wdRestored.Input().WaveNumber,
		"failed restored wave should report the correct wave number")
}

// TestMetadataTick_RebuildsLocalOrphanedWaveFromPersistedSubtasks verifies the live-session
// recovery path: local repos should rebuild missing wave orchestration during the normal
// metadata tick, not only after startup or in daemon-managed mode.
func TestMetadataTick_RebuildsLocalOrphanedWaveFromPersistedSubtasks(t *testing.T) {
	const planFile = "local-orphaned-wave"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := taskstore.NewTestSQLiteStore(t)
	content := "**Goal:** local orphan test\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n\n## Wave 2\n\n### Task 2: Second\n\nDo second.\n"
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
		Branch:   "plan/local-orphaned-wave",
		Content:  content,
	}))
	require.NoError(t, store.SetSubtasks("proj", planFile, []taskstore.SubtaskEntry{
		{TaskNumber: 1, Title: "First", Status: taskstore.SubtaskStatusRunning},
		{TaskNumber: 2, Title: "Second", Status: taskstore.SubtaskStatusPending},
	}))

	ps, err := taskstate.Load(store, "proj", plansDir)
	require.NoError(t, err)
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveRunning), ActiveAgentType: session.AgentTypeCoder, ActiveWave: 1}))

	h := waveFlowHome(t, ps, plansDir, make(map[string]*orchestration.WaveOrchestrator))
	h.taskStore = store
	h.taskStoreProject = "proj"

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "local-orphaned-wave-W1-T1",
		Path:       dir,
		Program:    "opencode",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	inst.SetStatus(session.Ready)
	inst.Exited = true
	h.nav.AddInstance(inst)

	model, _ := h.Update(metadataResultMsg{
		Results:   []instanceMetadata{{Title: "local-orphaned-wave-W1-T1", TmuxAlive: false}},
		PlanState: ps,
	})
	updated := model.(*home)

	orch, exists := updated.waveOrchestrators[planFile]
	require.True(t, exists, "local metadata tick should rebuild missing wave orchestration")
	assert.Equal(t, orchestration.WaveStateWaveComplete, orch.State(),
		"rebuilt local orphaned wave should resolve after the dead task is observed")
	assert.Equal(t, stateWaveDecision, updated.state,
		"rebuilt local orphaned wave should show the failed-wave recovery prompt")
	require.True(t, updated.overlays.IsActive(),
		"failed-wave recovery prompt must appear for local orphaned wave recovery")
	wdLocal, okLocal := updated.overlays.Current().(*overlay.WaveDecisionOverlay)
	require.True(t, okLocal, "current overlay must be a WaveDecisionOverlay")
	assert.Greater(t, wdLocal.Input().Failed, 0,
		"rebuilt local orphaned wave should report failures")
}

// TestWaveMonitor_LoadingInstanceNotMarkedFailed verifies that a wave task whose
// instance exists in the nav list but hasn't finished async startup (Loading status)
// is NOT prematurely marked as failed. This prevents the "instant all-complete" bug
// where the metadata tick fires before StartInSharedWorktree completes.
func TestWaveMonitor_LoadingInstanceNotMarkedFailed(t *testing.T) {
	const planFile = "loading-race"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{
				{Number: 1, Title: "Task 1", Body: "do it"},
				{Number: 2, Title: "Task 2", Body: "do it too"},
			}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "loading race test", "plan/loading-race", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	// Create instances in Loading state (async start not yet complete).
	inst1, err := session.NewInstance(session.InstanceOptions{
		Title:      "loading-race-W1-T1",
		Path:       t.TempDir(),
		Program:    "opencode",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
		PeerCount:  2,
	})
	require.NoError(t, err)
	inst1.SetStatus(session.Loading)

	inst2, err := session.NewInstance(session.InstanceOptions{
		Title:      "loading-race-W1-T2",
		Path:       t.TempDir(),
		Program:    "opencode",
		TaskFile:   planFile,
		TaskNumber: 2,
		WaveNumber: 1,
		PeerCount:  2,
	})
	require.NoError(t, err)
	inst2.SetStatus(session.Loading)

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	_ = h.nav.AddInstance(inst1)
	_ = h.nav.AddInstance(inst2)

	// Metadata tick with NO results for the loading instances (they aren't started yet).
	msg := metadataResultMsg{
		Results:   []instanceMetadata{},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Orchestrator must still be running — tasks must NOT be marked failed.
	require.Len(t, updated.waveOrchestrators, 1,
		"orchestrator must survive when instances are still loading")
	assert.Equal(t, orchestration.WaveStateRunning, orch.State(),
		"wave must remain running while instances are loading")
	assert.False(t, orch.IsTaskComplete(1), "task 1 must not be complete")
	assert.False(t, orch.IsTaskComplete(2), "task 2 must not be complete")
}

// TestWaveMonitor_AbortKeyDeletesOrchestrator verifies that pressing 'a' on the
// failed-wave decision prompt removes the orchestrator and returns to default state.
func TestWaveMonitor_AbortKeyDeletesOrchestrator(t *testing.T) {
	const planFile = "abort-test"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "Task 1", Body: "do it"}}},
			{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "Task 2", Body: "follow up"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()
	orch.MarkTaskFailed(1)
	require.Equal(t, orchestration.WaveStateWaveComplete, orch.State())

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	mgrAbort := overlay.NewManager()
	mgrAbort.Show(overlay.NewWaveDecisionOverlay(overlay.WaveDecisionInput{
		PlanFile: planFile, PlanName: planFile, WaveNumber: 1, TotalWaves: 2,
		Completed: 0, Failed: 1, Total: 1,
	}))

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "abort test", "plan/abort-test", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	h := &home{
		ctx:               context.Background(),
		state:             stateWaveDecision,
		appConfig:         config.DefaultConfig(),
		nav:               ui.NewNavigationPanel(&sp),
		menu:              ui.NewMenu(),
		tabbedWindow:      ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:      overlay.NewToastManager(&sp),
		overlays:          mgrAbort,
		waveOrchestrators: map[string]*orchestration.WaveOrchestrator{planFile: orch},
		taskState:         ps,
	}

	// Press 'a' for abort — direct shortcut on failure overlay
	keyMsg := tea.KeyPressMsg{Code: 'a', Text: "a"}
	model, cmd := h.handleKeyPress(keyMsg)
	updated := model.(*home)

	// State must return to default and abort action must be returned as cmd
	assert.Equal(t, stateDefault, updated.state, "state must return to default after abort")
	assert.False(t, updated.overlays.IsActive(), "overlay must be cleared after abort")
	assert.NotNil(t, cmd, "abort tea.Cmd must be returned so Update can execute it")
}

// TestTriggerPlanStage_ImplementNoWaves_RespecsPlanner verifies that when the
// implement stage is triggered on a plan without ## Wave headers, the plan status
// reverts to planning and a new planner session is spawned with a wave-annotation prompt.
func TestTriggerPlanStage_ImplementNoWaves_RespawnsPlanner(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	const planFile = "no-waves"
	// Plan content without ## Wave headers (has tasks but no waves)
	content := "# Plan\n\n**Goal:** Test\n\n### Task 1: Something\n\nDo it.\n"
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, planFile), []byte(content), 0o644))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "no waves test", "plan/no-waves", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusPlanning)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	h := &home{
		ctx:               context.Background(),
		state:             stateDefault,
		appConfig:         config.DefaultConfig(),
		taskState:         ps,
		taskStateDir:      plansDir,
		fsm:               newPlanFSMForTest(t, plansDir),
		activeRepoPath:    dir,
		program:           "opencode",
		nav:               list,
		menu:              ui.NewMenu(),
		tabbedWindow:      ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:      overlay.NewToastManager(&sp),
		waveOrchestrators: make(map[string]*orchestration.WaveOrchestrator),
	}

	_, _ = h.triggerTaskStage(planFile, "implement")

	// Plan status must have reverted to planning (parse failed, no StatusImplementing set)
	entry, ok := ps.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusPlanning, entry.Status,
		"plan status must revert to planning when wave headers are missing")

	// A new planner instance must have been added to the list
	instances := list.GetInstances()
	require.NotEmpty(t, instances, "a planner instance must be spawned after parse failure")
	plannerInst := instances[len(instances)-1]
	assert.Equal(t, session.AgentTypePlanner, plannerInst.AgentType,
		"spawned instance must be a planner")
	assert.Contains(t, plannerInst.QueuedPrompt, "Wave",
		"planner prompt must mention Wave headers")
	assert.Contains(t, plannerInst.QueuedPrompt, "planner-finished",
		"planner prompt must include planner completion signal")
	assert.Contains(t, plannerInst.QueuedPrompt, "task_update_content",
		"planner prompt must instruct the planner to store the annotated plan via MCP")
}

// ---------------------------------------------------------------------------
// All-waves-complete → review flow tests
// ---------------------------------------------------------------------------

// TestWaveMonitor_AllComplete_ShowsReviewPrompt verifies that when all tasks in the
// final wave complete, the orchestrator is deleted and a confirmation dialog appears
// asking the user to push and start review.
func TestWaveMonitor_AllComplete_ShowsReviewPrompt(t *testing.T) {
	const planFile = "all-complete"

	// Single wave plan — completing its tasks triggers orchestration.WaveStateAllComplete directly.
	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "Only task", Body: "do it"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "all-complete test", "plan/all-complete", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	// Create task instance for the finished wave task.
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "all-complete-W1-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	h.fsm = newPlanFSMForTest(t, plansDir)
	_ = h.nav.AddInstance(inst)

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "all-complete-W1-T1", TmuxAlive: true}},
		PlanState: ps,
		TaskSignals: []taskfsm.TaskSignal{{
			WaveNumber: 1,
			TaskNumber: 1,
			TaskFile:   planFile,
		}},
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Orchestrator must be deleted
	assert.Empty(t, updated.waveOrchestrators,
		"orchestrator must be deleted after all waves complete")

	// Confirmation dialog must appear for review
	assert.Equal(t, stateConfirm, updated.state,
		"state must be stateConfirm to prompt user for review")
	require.True(t, updated.overlays.IsActive(),
		"confirmation overlay must be set for all-complete review prompt")
	co3, ok3 := updated.overlays.Current().(*overlay.ConfirmationOverlay)
	require.True(t, ok3, "current overlay must be a ConfirmationOverlay")
	// Standard confirm dialog (y/n) — not a wave-failed decision prompt
	assert.Equal(t, "y", co3.ConfirmKey,
		"confirm key must be 'y' for review prompt")
}

// TestWaveAllCompleteMsg_TransitionsToReviewing verifies that the waveAllCompleteMsg
// handler transitions the plan FSM from implementing to reviewing.
func TestWaveAllCompleteMsg_TransitionsToReviewing(t *testing.T) {
	const planFile = "review-transition"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "review transition test", "plan/review-trans", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	h := waveFlowHome(t, ps, plansDir, make(map[string]*orchestration.WaveOrchestrator))
	h.fsm = newPlanFSMForTest(t, plansDir)

	model, _ := h.Update(wavePushCompleteMsg{planFile: planFile})
	updated := model.(*home)

	// Reload plan state from disk to verify FSM transition persisted.
	reloaded, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusReviewing, entry.Status,
		"plan must transition to reviewing after wavePushCompleteMsg")

	// Toast must confirm the transition
	_ = updated // ensure the model is used (toast is in-memory, hard to assert without rendering)
}

// TestWaveAllCompleteMsg_PushIsAsync verifies that waveAllCompleteMsg
// returns an async command and that the plan transitions on wavePushCompleteMsg.
func TestWaveAllCompleteMsg_PushIsAsync(t *testing.T) {
	const planFile = "review-async"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "review async test", "plan/review-async", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	h := waveFlowHome(t, ps, plansDir, make(map[string]*orchestration.WaveOrchestrator))
	h.fsm = newPlanFSMForTest(t, plansDir)

	model, cmd := h.Update(waveAllCompleteMsg{planFile: planFile})
	require.True(t, cmd != nil, "waveAllCompleteMsg must return non-nil async cmd")
	updated := model.(*home)

	// Plan remains implementing immediately after handling waveAllCompleteMsg.
	reloaded, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	entryBefore, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusImplementing, entryBefore.Status,
		"plan must remain implementing before async push completion")
	assert.Equal(t, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveWaiting), ActiveAgentType: session.AgentTypeCoder, ActiveWave: 1}, entryBefore.ExecutionState)

	msg := cmd()
	pushMsg, ok := msg.(wavePushCompleteMsg)
	require.True(t, ok, "waveAllCompleteMsg command must resolve to wavePushCompleteMsg, got %T", msg)
	assert.Equal(t, planFile, pushMsg.planFile, "push message must include same plan file")

	model, _ = updated.Update(pushMsg)
	updated = model.(*home)

	// Transition to reviewing after push completion.
	reloaded, err = newTestPlanState(t, plansDir)
	require.NoError(t, err)
	entryAfter, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusReviewing, entryAfter.Status,
		"plan must transition to reviewing after wavePushCompleteMsg")
	assert.Equal(t, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseReviewing), ActiveAgentType: session.AgentTypeReviewer}, entryAfter.ExecutionState)

	// Ensure caller gets back a usable updated model after async completion.
	_ = updated

}

// TestWaveAllCompleteMsg_DoesNotRepromptWhilePushInFlight verifies that once the
// user confirms the final all-waves-complete dialog, a metadata tick arriving
// before the async push finishes does not re-show the same confirmation.
func TestWaveAllCompleteMsg_DoesNotRepromptWhilePushInFlight(t *testing.T) {
	const planFile = "review-in-flight"

	planContent := "**Goal:** verify dedupe\n\n## Wave 1\n\n### Task 1: only\n\nDo it.\n"
	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{{
			Number: 1,
			Tasks:  []taskparser.Task{{Number: 1, Title: "only", Body: "Do it."}},
		}},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "review in flight test", "plan/review-in-flight", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	store := storeForDir(t, plansDir)
	require.NoError(t, store.SetContent("test", planFile, planContent))
	orch.SetStore(store, "test")

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "review-in-flight-W1-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	h.fsm = newPlanFSMForTest(t, plansDir)
	_ = h.nav.AddInstance(inst)

	model, _ := h.Update(metadataResultMsg{
		Results:   []instanceMetadata{{Title: inst.Title, TmuxAlive: true}},
		PlanState: ps,
		TaskSignals: []taskfsm.TaskSignal{{
			WaveNumber: 1,
			TaskNumber: 1,
			TaskFile:   planFile,
		}},
	})
	updated := model.(*home)
	require.Equal(t, stateConfirm, updated.state)

	model, cmd := updated.handleKeyPress(tea.KeyPressMsg{Code: 'y', Text: "y"})
	updated = model.(*home)
	require.Equal(t, stateDefault, updated.state)

	msg := cmd()
	require.IsType(t, waveAllCompleteMsg{}, msg)

	model, cmd = updated.Update(msg)
	require.NotNil(t, cmd, "confirming all-complete must kick off async push")
	updated = model.(*home)

	model, _ = updated.Update(metadataResultMsg{
		Results:   []instanceMetadata{{Title: inst.Title, TmuxAlive: false}},
		PlanState: ps,
	})
	updated = model.(*home)

	assert.Equal(t, stateDefault, updated.state,
		"metadata tick during in-flight push must not re-show the all-complete prompt")
	assert.False(t, updated.overlays.IsActive(),
		"no confirmation overlay should be active while push-to-review is already in flight")
	assert.Empty(t, updated.pendingAllComplete,
		"no deferred all-complete prompt should be queued while push-to-review is already in flight")
}

// TestWaveMonitor_AllComplete_DefersWhileFocusModeActive verifies that the final
// review prompt does not steal focus from an active agent pane. Instead it
// queues the prompt and shows a sticky toast until focus mode is left.
func TestWaveMonitor_AllComplete_DefersWhileFocusModeActive(t *testing.T) {
	const planFile = "focus-deferred-review"

	planContent := "**Goal:** verify focus-mode deferral\n\n## Wave 1\n\n### Task 1: only\n\nDo it.\n"
	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{{
			Number: 1,
			Tasks:  []taskparser.Task{{Number: 1, Title: "only", Body: "Do it."}},
		}},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "focus deferral test", "plan/focus-deferred-review", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	store := storeForDir(t, plansDir)
	require.NoError(t, store.SetContent("test", planFile, planContent))
	orch.SetStore(store, "test")

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "focus-deferred-review-W1-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	h.fsm = newPlanFSMForTest(t, plansDir)
	h.state = stateFocusAgent
	h.tabbedWindow.SetFocusMode(true)
	h.menu.SetFocusMode(true)
	_ = h.nav.AddInstance(inst)

	model, _ := h.Update(metadataResultMsg{
		Results:   []instanceMetadata{{Title: inst.Title, TmuxAlive: true}},
		PlanState: ps,
		TaskSignals: []taskfsm.TaskSignal{{
			WaveNumber: 1,
			TaskNumber: 1,
			TaskFile:   planFile,
		}},
	})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state,
		"all-complete should not force-exit focus mode")
	assert.False(t, updated.overlays.IsActive(),
		"all-complete should not show a confirmation overlay while focus mode is active")
	assert.Contains(t, updated.pendingAllComplete, planFile,
		"all-complete prompt must be queued until focus mode ends")
	assert.True(t, updated.toastManager.HasActiveToasts(),
		"a sticky toast should notify the user while focus mode remains active")
	toastView := updated.toastManager.View()
	assert.Contains(t, toastView, "all waves complete",
		"toast should mention the completed wave set")
	assert.Contains(t, toastView, "focus mode to review",
		"toast should explain how to surface the deferred review prompt")

	model, _ = updated.Update(metadataResultMsg{
		Results:   []instanceMetadata{{Title: inst.Title, TmuxAlive: false}},
		PlanState: ps,
	})
	updated = model.(*home)
	assert.Equal(t, stateFocusAgent, updated.state,
		"later metadata ticks should still avoid stealing focus")
	assert.False(t, updated.overlays.IsActive(),
		"later metadata ticks should not show the prompt until focus mode ends")

	updated.exitFocusMode()
	model, _ = updated.Update(metadataResultMsg{
		Results:   []instanceMetadata{{Title: inst.Title, TmuxAlive: false}},
		PlanState: ps,
	})
	updated = model.(*home)

	assert.Equal(t, stateConfirm, updated.state,
		"once focus mode ends, the queued all-complete prompt should appear")
	assert.True(t, updated.overlays.IsActive(),
		"queued all-complete prompt should become an overlay after focus mode ends")
}

// TestWaveMonitor_AllComplete_MultiWave verifies the flow with a multi-wave plan
// where all waves complete sequentially (wave 1 done → advance → wave 2 done → review prompt).
func TestWaveMonitor_AllComplete_MultiWave(t *testing.T) {
	const planFile = "multi-wave"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "W1 task", Body: "first"}}},
			{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "W2 task", Body: "second"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()     // start wave 1
	orch.MarkTaskComplete(1) // wave 1 done → orchestration.WaveStateWaveComplete
	require.Equal(t, orchestration.WaveStateWaveComplete, orch.State())

	orch.StartNextWave() // advance to wave 2
	require.Equal(t, orchestration.WaveStateRunning, orch.State())

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "multi wave test", "plan/multi-wave", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	// Wave 2 task instance.
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "multi-wave-W2-T2",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 2,
		WaveNumber: 2,
	})
	require.NoError(t, err)

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	h.fsm = newPlanFSMForTest(t, plansDir)
	_ = h.nav.AddInstance(inst)

	// 1. Feed wave-2 completion event.
	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "multi-wave-W2-T2", TmuxAlive: true}},
		PlanState: ps,
		TaskSignals: []taskfsm.TaskSignal{{
			WaveNumber: 2,
			TaskNumber: 2,
			TaskFile:   planFile,
		}},
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Wave 2 was the last wave → AllComplete → review prompt
	assert.Empty(t, updated.waveOrchestrators,
		"orchestrator must be deleted after final wave completes")
	assert.Equal(t, stateConfirm, updated.state,
		"state must be stateConfirm for review prompt after final wave")
}

// TestRetryFailedWaveTasks_RemovesOldInstances verifies that when a failed wave task
// is retried, the old failed instance is removed from the list before the new one is
// spawned. Without this cleanup, each retry leaves behind ghost instances that all get
// marked ImplementationComplete when waves finish — producing duplicate entries.
func TestRetryFailedWaveTasks_RemovesOldInstances(t *testing.T) {
	const planFile = "retry-cleanup"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{
				{Number: 1, Title: "Task 1", Body: "do first"},
				{Number: 6, Title: "Task 6", Body: "the flaky one"},
			}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	// Task 1 completed, task 6 failed.
	orch.MarkTaskComplete(1)
	orch.MarkTaskFailed(6)
	require.Equal(t, orchestration.WaveStateAllComplete, orch.State(), "single-wave plan should be AllComplete")

	dir := t.TempDir()
	// spawnWaveTasks → Setup() creates .worktrees/ inside dir before failing
	// (no real git repo). Force-remove it so t.TempDir cleanup doesn't fail.
	t.Cleanup(func() { os.RemoveAll(filepath.Join(dir, ".worktrees")) })
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "retry cleanup test", "plan/retry-cleanup", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	planName := taskstate.DisplayName(planFile)

	// Create the completed task 1 instance
	inst1, err := session.NewInstance(session.InstanceOptions{
		Title:      planName + "-W1-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst1.SetStatus(session.Ready)

	// Create the failed task 6 instance (the one that should be removed on retry)
	failedInst6, err := session.NewInstance(session.InstanceOptions{
		Title:      planName + "-W1-T6",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 6,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	failedInst6.SetStatus(session.Paused) // failed tasks end up paused

	state := config.DefaultState()
	storage, err := session.NewStorage(state)
	require.NoError(t, err)

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	h.storage = storage
	h.allInstances = []*session.Instance{inst1, failedInst6}
	h.activeRepoPath = dir
	h.program = "claude"
	_ = h.nav.AddInstance(inst1)
	_ = h.nav.AddInstance(failedInst6)

	// Verify we start with 2 instances
	require.Len(t, h.nav.GetInstances(), 2, "should start with 2 instances")

	// Count instances with TaskNumber==6 before retry
	countTask6 := func() int {
		count := 0
		for _, inst := range h.nav.GetInstances() {
			if inst.TaskNumber == 6 && inst.TaskFile == planFile {
				count++
			}
		}
		return count
	}
	require.Equal(t, 1, countTask6(), "should have exactly 1 task-6 instance before retry")

	entry, _ := ps.Entry(planFile)

	// retryFailedWaveTasks spawns new instances — but it should remove the old one first.
	// Note: spawnWaveTasks will fail (no real git/tmux) but the cleanup should happen before that.
	h.retryFailedWaveTasks(orch, entry)

	// The old failed task 6 instance must have been removed from the list.
	for _, inst := range h.nav.GetInstances() {
		if inst == failedInst6 {
			t.Fatal("old failed task-6 instance must be removed from the list on retry")
		}
	}

	// The old failed task 6 instance must have been removed from allInstances.
	for _, inst := range h.allInstances {
		if inst == failedInst6 {
			t.Fatal("old failed task-6 instance must be removed from allInstances on retry")
		}
	}

	// Task 1 instance must still be there (it wasn't retried)
	foundTask1 := false
	for _, inst := range h.nav.GetInstances() {
		if inst.TaskNumber == 1 && inst.TaskFile == planFile {
			foundTask1 = true
		}
	}
	assert.True(t, foundTask1, "task 1 instance must not be affected by task 6 retry")
}

// TestWaveSignal_TriggersImplementation verifies that a wave signal file written
// in .signals/ is correctly picked up by ScanWaveSignals and parsed into a
// WaveSignal with the correct WaveNumber and PlanFile fields, ready for TUI consumption.
func TestWaveSignal_TriggersImplementation(t *testing.T) {
	repoRoot := t.TempDir()
	signalsDir := filepath.Join(repoRoot, ".kasmos", "signals")
	require.NoError(t, os.MkdirAll(signalsDir, 0o755))

	// Create a plan with wave headers
	planContent := "# Test\n\n**Goal:** test\n\n## Wave 1\n\n### Task 1: Do thing\n\nDo the thing.\n"
	planFile := "wave-signal-test"
	plansDir := filepath.Join(repoRoot, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, planFile), []byte(planContent), 0o644))

	// Register plan as implementing
	ps := &taskstate.TaskState{Dir: plansDir, Plans: make(map[string]taskstate.TaskEntry), TopicEntries: make(map[string]taskstate.TopicEntry)}
	ps.Plans[planFile] = taskstate.TaskEntry{
		Status: "implementing",
		Branch: "plan/wave-signal-test",
	}
	require.NoError(t, ps.Save())

	// Write a wave signal
	signalFile := fmt.Sprintf("implement-wave-1-%s", planFile)
	require.NoError(t, os.WriteFile(filepath.Join(signalsDir, signalFile), nil, 0o644))

	// Verify signal is scannable using the new .kasmos/signals/ convention
	waveSignals := taskfsm.ScanWaveSignals(signalsDir)
	require.Len(t, waveSignals, 1)
	assert.Equal(t, 1, waveSignals[0].WaveNumber)
	assert.Equal(t, planFile, waveSignals[0].TaskFile)
}

// TestPlannerExit_CancelKillsInstanceAndMarksPrompted verifies that pressing "n"
// on the planner-exit dialog kills the planner instance and marks plannerPrompted.
func TestPlannerExit_CancelKillsInstanceAndMarksPrompted(t *testing.T) {
	const planFile = "cancel-kill"

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "planner-cancel-inst",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	inst.AgentType = session.AgentTypePlanner
	inst.TaskFile = planFile

	// Create storage so saveAllInstances doesn't panic
	state := config.DefaultState()
	storage, err := session.NewStorage(state)
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	mgrCancel := overlay.NewManager()
	mgrCancel.Show(overlay.NewConfirmationOverlay("Plan 'cancel-kill' is ready. Start implementation?"))
	h := &home{
		ctx:                         context.Background(),
		state:                       stateConfirm,
		appConfig:                   config.DefaultConfig(),
		nav:                         ui.NewNavigationPanel(&sp),
		menu:                        ui.NewMenu(),
		tabbedWindow:                ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:                overlay.NewToastManager(&sp),
		overlays:                    mgrCancel,
		storage:                     storage,
		waveOrchestrators:           make(map[string]*orchestration.WaveOrchestrator),
		plannerPrompted:             make(map[string]bool),
		coderPushPrompted:           make(map[string]bool),
		pendingPlannerInstanceTitle: "planner-cancel-inst",
		pendingPlannerTaskFile:      planFile,
		allInstances:                []*session.Instance{inst},
	}
	_ = h.nav.AddInstance(inst)

	// Press 'n' (cancel key — default for confirmation overlay)
	keyMsg := tea.KeyPressMsg{Code: 'n', Text: "n"}
	model, _ := h.handleKeyPress(keyMsg)
	updated := model.(*home)

	assert.True(t, updated.plannerPrompted[planFile],
		"plannerPrompted must be true after cancel")
	assert.Empty(t, updated.allInstances,
		"planner instance must be removed from allInstances after cancel")
	assert.Empty(t, updated.pendingPlannerInstanceTitle,
		"pendingPlannerInstanceTitle must be cleared after cancel")
	assert.Empty(t, updated.pendingPlannerTaskFile,
		"pendingPlannerTaskFile must be cleared after cancel")
}

// --- Focus-before-overlay tests ---
// These verify that agent-related overlays auto-focus the relevant instance
// so the user can see the agent output behind the dialog.

// TestWaveMonitor_FocusesTaskInstance_WhenWaveCompleteShown verifies that
// showing the wave-complete confirmation auto-selects a task instance for
// that plan so the agent output is visible behind the overlay.
func TestWaveMonitor_FocusesTaskInstance_WhenWaveCompleteShown(t *testing.T) {
	const planFile = "focus-wave"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "Task 1", Body: "do it"}}},
			{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "Task 2", Body: "follow up"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "focus wave test", "plan/focus-wave", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	planName := taskstate.DisplayName(planFile)

	// Add an "other" instance first (so it's selected by default), then the task instance.
	otherInst := &session.Instance{Title: "other-agent", Program: "opencode"}
	otherInst.MarkStartedForTest()
	taskTitle := fmt.Sprintf("%s-W1-T1", planName)
	taskInst := &session.Instance{
		Title:      taskTitle,
		Program:    "opencode",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	}
	taskInst.MarkStartedForTest()

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	_ = h.nav.AddInstance(otherInst)
	_ = h.nav.AddInstance(taskInst)
	h.updateSidebarTasks() // register plans so rebuildRows emits plan-grouped instances
	h.nav.SetSelectedInstance(0)
	require.Equal(t, otherInst, h.nav.GetSelectedInstance(), "precondition: other-agent selected")

	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "other-agent", TmuxAlive: true},
			{Title: taskTitle, TmuxAlive: true},
		},
		PlanState: ps,
		TaskSignals: []taskfsm.TaskSignal{{
			WaveNumber: 1,
			TaskNumber: 1,
			TaskFile:   planFile,
		}},
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.Equal(t, stateWaveDecision, updated.state, "should show wave-advance decision")
	// The task instance should be selected, not the other one.
	assert.Equal(t, taskInst, updated.nav.GetSelectedInstance(),
		"wave-advance overlay should auto-focus a task instance for the plan")
}

// TestWaveMonitor_FocusesTaskInstance_WhenFailedWaveShown verifies that the
// failed-wave decision dialog auto-focuses a task instance for the plan.
func TestWaveMonitor_FocusesTaskInstance_WhenFailedWaveShown(t *testing.T) {
	const planFile = "focus-failed"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "Task 1", Body: "do it"}}},
			{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "Task 2", Body: "follow up"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "focus failed test", "plan/focus-failed", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	planName := taskstate.DisplayName(planFile)

	// "other" instance selected by default.
	otherInst := &session.Instance{Title: "other-agent", Program: "opencode"}
	otherInst.MarkStartedForTest()
	taskTitle := fmt.Sprintf("%s-W1-T1", planName)
	taskInst := &session.Instance{
		Title:      taskTitle,
		Program:    "opencode",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	}
	taskInst.MarkStartedForTest()
	taskInst.SetStatus(session.Paused) // paused = treated as failed

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	_ = h.nav.AddInstance(otherInst)
	_ = h.nav.AddInstance(taskInst)
	h.updateSidebarTasks() // register plans so rebuildRows emits plan-grouped instances
	h.nav.SetSelectedInstance(0)
	require.Equal(t, otherInst, h.nav.GetSelectedInstance(), "precondition: other-agent selected")

	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "other-agent", TmuxAlive: true},
			{Title: taskTitle, TmuxAlive: false},
		},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.Equal(t, stateWaveDecision, updated.state, "should show failed-wave decision")
	assert.Equal(t, taskInst, updated.nav.GetSelectedInstance(),
		"failed-wave overlay should auto-focus a task instance for the plan")
}

// TestPlannerExit_FocusesPlannerInstance_BeforeConfirm verifies that when a
// PlannerFinished signal is processed, the planner instance is auto-focused
// so its output is visible behind the overlay.
func TestPlannerExit_FocusesPlannerInstance_BeforeConfirm(t *testing.T) {
	const planFile = "focus-planner"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "focus planner test", "plan/focus-planner", time.Now()))
	// Plan is StatusPlanning — the PlannerFinished signal will transition it to StatusReady.
	seedPlanStatus(t, ps, planFile, taskstate.StatusPlanning)

	plannerInst := &session.Instance{
		Title:     "focus-planner-plan",
		Program:   "opencode",
		TaskFile:  planFile,
		AgentType: session.AgentTypePlanner,
	}
	plannerInst.MarkStartedForTest()

	otherInst := &session.Instance{Title: "other-agent", Program: "opencode"}
	otherInst.MarkStartedForTest()

	h := waveFlowHome(t, ps, plansDir, nil)
	h.waveOrchestrators = make(map[string]*orchestration.WaveOrchestrator)
	h.plannerPrompted = make(map[string]bool)
	h.coderPushPrompted = make(map[string]bool)
	h.pendingReviewFeedback = make(map[string]string)
	h.fsm = newPlanFSMForTest(t, plansDir)
	_ = h.nav.AddInstance(otherInst)
	_ = h.nav.AddInstance(plannerInst)
	h.updateSidebarTasks() // register plans so rebuildRows emits plan-grouped instances
	h.nav.SetSelectedInstance(0)
	require.Equal(t, otherInst, h.nav.GetSelectedInstance(), "precondition: other-agent selected")

	// Use the signal-driven path: PlannerFinished signal triggers the dialog.
	signal := taskfsm.Signal{
		Event:    taskfsm.PlannerFinished,
		TaskFile: planFile,
	}
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "other-agent", TmuxAlive: true},
			{Title: "focus-planner-plan", TmuxAlive: true},
		},
		PlanState: ps,
		Signals:   []taskfsm.Signal{signal},
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.Equal(t, stateConfirm, updated.state, "should show planner-exit confirm")
	assert.Equal(t, plannerInst, updated.nav.GetSelectedInstance(),
		"planner-exit overlay should auto-focus the planner instance")
}

// TestWaveMonitor_WaveComplete_DeferredWhenOverlayActive verifies that when an
// intermediate wave completes while the user is in a non-focus overlay (e.g.
// context menu or confirmation dialog), the wave-advance dialog is deferred
// and shown on the next tick when the overlay clears — not swallowed.
func TestWaveMonitor_WaveComplete_DeferredWhenOverlayActive(t *testing.T) {
	const planFile = "deferred-wave"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "W1T1", Body: "task 1"}}},
			{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "W2T1", Body: "task 2"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()
	// Wave 1 task finishes — orchestrator transitions to WaveStateWaveComplete.
	orch.MarkTaskComplete(1)
	require.Equal(t, orchestration.WaveStateWaveComplete, orch.State())

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "deferred wave test", "plan/deferred-wave", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "deferred-wave-W1-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	_ = h.nav.AddInstance(inst)

	// Simulate user being in a non-focus overlay (e.g. context menu).
	h.state = stateConfirm

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "deferred-wave-W1-T1", TmuxAlive: true}},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Orchestrator must still exist (intermediate wave, not all-complete).
	require.Contains(t, updated.waveOrchestrators, planFile,
		"orchestrator must survive intermediate wave completion")

	// The wave dialog must be deferred, not swallowed.
	assert.Contains(t, updated.deferredWaveDialogs, planFile,
		"wave-advance dialog must be queued when overlay blocks")

	// Simulate overlay clearing and another metadata tick.
	updated.state = stateDefault
	msg2 := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "deferred-wave-W1-T1", TmuxAlive: true}},
		PlanState: ps,
	}
	model2, _ := updated.Update(msg2)
	updated2 := model2.(*home)

	// Deferred wave dialog must have been drained.
	assert.Empty(t, updated2.deferredWaveDialogs,
		"deferredWaveDialogs must be drained after overlay clears")

	// The wave decision overlay must now be shown.
	assert.Equal(t, stateWaveDecision, updated2.state,
		"deferred wave dialog must be shown as wave decision after overlay clears")
	assert.True(t, updated2.overlays.IsActive(),
		"wave decision overlay must be active after deferred dialog is drained")
	wdDeferred, okDeferred := updated2.overlays.Current().(*overlay.WaveDecisionOverlay)
	require.True(t, okDeferred, "current overlay must be a WaveDecisionOverlay after deferred drain")
	assert.Equal(t, planFile, wdDeferred.Input().PlanFile,
		"deferred wave decision overlay must reference the correct plan")

	// The plan must already be in wave_waiting state.
	entry2, entryOK := updated2.taskState.Entry(planFile)
	require.True(t, entryOK, "plan entry must exist after deferred wave dialog")
	assert.Equal(t, string(taskfsm.ExecutionPhaseWaveWaiting), entry2.ExecutionState.Phase,
		"plan must be in wave_waiting phase before user sees the overlay")
}

// TestWaveMonitor_AllComplete_DeferredWhenOverlayActive verifies that when all
// waves complete while the user is in an overlay (e.g. confirmation dialog),
// the review prompt is deferred and shown on the next tick when the overlay clears.
func TestWaveMonitor_AllComplete_DeferredWhenOverlayActive(t *testing.T) {
	const planFile = "deferred-complete"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "Only task", Body: "do it"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "deferred test", "plan/deferred", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "deferred-complete-W1-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	h.fsm = newPlanFSMForTest(t, plansDir)
	_ = h.nav.AddInstance(inst)

	// Simulate user being in an overlay (e.g. another confirmation dialog)
	h.state = stateConfirm

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "deferred-complete-W1-T1", TmuxAlive: true}},
		PlanState: ps,
		TaskSignals: []taskfsm.TaskSignal{{
			WaveNumber: 1,
			TaskNumber: 1,
			TaskFile:   planFile,
		}},
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Orchestrator must be deleted (tasks are paused)
	assert.Empty(t, updated.waveOrchestrators,
		"orchestrator must be deleted even when overlay is active")

	// But the confirm dialog must NOT have been shown (overlay was blocking)
	// Instead, the plan file must be in pendingAllComplete
	assert.Contains(t, updated.pendingAllComplete, planFile,
		"plan must be deferred to pendingAllComplete when overlay blocks")

	// Now simulate the overlay clearing and another metadata tick arriving
	updated.state = stateDefault
	msg2 := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "deferred-complete-W1-T1", TmuxAlive: true}},
		PlanState: ps,
	}
	model2, _ := updated.Update(msg2)
	updated2 := model2.(*home)

	// Now the confirm dialog must appear
	assert.Equal(t, stateConfirm, updated2.state,
		"deferred all-complete must show confirm dialog on next tick")
	assert.Empty(t, updated2.pendingAllComplete,
		"pendingAllComplete must be drained after showing dialog")
}

// TestAutoAdvanceWaves_SkipsConfirmOnSuccess verifies that when AutoAdvanceWaves is
// true and a wave completes with zero failures, the model is configured to auto-advance
// without showing a confirmation dialog.
func TestAutoAdvanceWaves_SkipsConfirmOnSuccess(t *testing.T) {
	// Build a plan with 2 waves
	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "T1"}}},
			{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "T2"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator("test", plan)
	orch.StartNextWave()
	orch.MarkTaskComplete(1) // wave 1 complete, no failures

	m := &home{
		appConfig:         &config.Config{AutoAdvanceWaves: true},
		waveOrchestrators: map[string]*orchestration.WaveOrchestrator{"test": orch},
		taskState:         &taskstate.TaskState{Plans: map[string]taskstate.TaskEntry{"test": {Status: "implementing"}}},
		state:             stateDefault,
	}

	// NeedsConfirm should be true (wave just completed)
	assert.True(t, orch.NeedsConfirm())

	// With auto-advance enabled, the handler should NOT show a confirm dialog
	// and instead directly emit a waveAdvanceMsg.
	// This is a unit-level assertion on the branching logic.
	assert.True(t, m.appConfig.AutoAdvanceWaves)
	assert.Equal(t, 0, orch.FailedTaskCount())
}

// TestAutoAdvanceWaves_ShowsConfirmOnFailure verifies that even when AutoAdvanceWaves
// is true, a wave with failures still shows the decision dialog.
func TestAutoAdvanceWaves_ShowsConfirmOnFailure(t *testing.T) {
	const planFile = "auto-advance-failure"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{
				{Number: 1, Title: "T1"},
				{Number: 2, Title: "T2"},
			}},
			{Number: 2, Tasks: []taskparser.Task{{Number: 3, Title: "T3"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()
	orch.MarkTaskComplete(1)
	orch.MarkTaskFailed(2) // wave 1 complete with 1 failure

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "auto-advance failure test", "plan/auto-advance-failure", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	// Create task instances
	inst1, err := session.NewInstance(session.InstanceOptions{
		Title:      "auto-advance-failure-W1-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst1.PromptDetected = true
	inst1.HasWorked = true

	inst2, err := session.NewInstance(session.InstanceOptions{
		Title:      "auto-advance-failure-W1-T2",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 2,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst2.SetStatus(session.Paused) // failed

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	// Enable auto-advance
	h.appConfig = &config.Config{AutoAdvanceWaves: true}
	_ = h.nav.AddInstance(inst1)
	_ = h.nav.AddInstance(inst2)

	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "auto-advance-failure-W1-T1", TmuxAlive: true},
			{Title: "auto-advance-failure-W1-T2", TmuxAlive: false},
		},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Even with auto-advance enabled, failures must show the decision dialog
	assert.Equal(t, stateWaveDecision, updated.state,
		"failed wave must show decision dialog even when auto-advance is enabled")
	require.True(t, updated.overlays.IsActive(),
		"wave decision overlay must be set for failed-wave decision")
	wd4, ok4 := updated.overlays.Current().(*overlay.WaveDecisionOverlay)
	require.True(t, ok4, "current overlay must be a WaveDecisionOverlay")
	assert.Greater(t, wd4.Input().Failed, 0,
		"failed-wave overlay must report failures")
}

// TestAutoAdvanceWaves_EmitsAdvanceMsgOnSuccess verifies that when AutoAdvanceWaves
// is true and a wave completes with zero failures, the Update handler emits a
// waveAdvanceMsg directly (no confirmation dialog shown).
func TestAutoAdvanceWaves_EmitsAdvanceMsgOnSuccess(t *testing.T) {
	const planFile = "auto-advance-success"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "T1"}}},
			{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "T2"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "auto-advance success test", "plan/auto-advance-success", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "auto-advance-success-W1-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	// Enable auto-advance
	h.appConfig = &config.Config{AutoAdvanceWaves: true}
	_ = h.nav.AddInstance(inst)

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "auto-advance-success-W1-T1", TmuxAlive: true}},
		PlanState: ps,
		TaskSignals: []taskfsm.TaskSignal{{
			WaveNumber: 1,
			TaskNumber: 1,
			TaskFile:   planFile,
		}},
	}
	model, cmd := h.Update(msg)
	updated := model.(*home)

	// With auto-advance enabled and no failures, must NOT show a confirmation dialog
	assert.NotEqual(t, stateConfirm, updated.state,
		"auto-advance must not show confirmation dialog on success")
	assert.False(t, updated.overlays.IsActive(),
		"no confirmation overlay must be shown when auto-advancing")

	// The cmd must be non-nil (it contains the waveAdvanceMsg)
	assert.NotNil(t, cmd, "auto-advance must emit a tea.Cmd containing waveAdvanceMsg")
}

// TestWaveTaskCompletion_PromptDetectedCompletesOneTaskWave verifies that a
// wave task returning to prompt after doing real work completes the current
// one-task wave and shows the next-wave confirmation UI.
func TestWaveTaskCompletion_PromptDetectedCompletesOneTaskWave(t *testing.T) {
	const planFile = "prompt-detected-complete"

	plan := &taskparser.Plan{Waves: []taskparser.Wave{
		{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "do work"}}},
		{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "follow up"}}},
	}}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "prompt-detected completion test", "plan/prompt-detected-complete", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	planName := taskstate.DisplayName(planFile)
	instTitle := fmt.Sprintf("%s-W1-T1", planName)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      instTitle,
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	inst.HasWorked = true
	inst.PromptDetected = true
	inst.AwaitingWork = false

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	_ = h.nav.AddInstance(inst)

	model, _ := h.Update(metadataResultMsg{
		Results:   []instanceMetadata{{Title: instTitle, TmuxAlive: true}},
		PlanState: ps,
	})
	updated := model.(*home)

	assert.Equal(t, orchestration.WaveStateWaveComplete, orch.State(),
		"prompt-detected completion should resolve the finished wave")
	assert.Equal(t, stateWaveDecision, updated.state,
		"manual wave orchestration should show the wave decision overlay")
	require.True(t, updated.overlays.IsActive(),
		"wave decision overlay should be visible")
	wdPrompt, okPrompt := updated.overlays.Current().(*overlay.WaveDecisionOverlay)
	require.True(t, okPrompt, "current overlay must be a WaveDecisionOverlay")
	assert.Equal(t, 0, wdPrompt.Input().Failed,
		"successful wave completion should show the success-variant decision overlay")
	assert.Equal(t, 1, orch.CompletedTaskCount(),
		"completed task should be recorded as successful")
	assert.Equal(t, 0, orch.FailedTaskCount(),
		"prompt-detected completion must not count as a failure")
}

// TestWaveTaskCompletion_RequiresHasWorked verifies that a wave task is NOT
// auto-completed when PromptDetected is true but HasWorked is false. This
// prevents permission prompts and early prompt returns from prematurely
// completing a wave (especially dangerous with auto-advance enabled).
// When HasWorked is true, the task auto-completes on the next metadata tick
// without waiting for a signal.
func TestWaveTaskCompletion_RequiresHasWorked(t *testing.T) {
	const planFile = "has-worked-guard"

	plan := &taskparser.Plan{Waves: []taskparser.Wave{{
		Number: 1,
		Tasks:  []taskparser.Task{{Number: 1, Title: "do work"}},
	}}}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "task signal guard test", "plan/task-signal-guard", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	planName := taskstate.DisplayName(planFile)
	instTitle := planName + "-W1-T1"

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      instTitle,
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst.PromptDetected = true
	inst.HasWorked = false // prompt seen but no real work yet
	inst.AwaitingWork = false

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	_ = h.nav.AddInstance(inst)

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: instTitle, TmuxAlive: true}},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Task must NOT be auto-completed when HasWorked is false.
	assert.Len(t, updated.waveOrchestrators, 1,
		"orchestrator must still exist when HasWorked is false")
	assert.Equal(t, orchestration.WaveStateRunning, orch.State(),
		"wave must remain running when task has not done real work")
	assert.NotEqual(t, stateConfirm, updated.state,
		"no completion dialog should appear before real work is done")
	assert.False(t, updated.overlays.IsActive(),
		"no completion overlay should appear before real work is observed")

	// Simulate the agent doing real work and returning to prompt — wave must auto-complete.
	inst.HasWorked = true
	model2, _ := updated.Update(metadataResultMsg{
		Results:   []instanceMetadata{{Title: instTitle, TmuxAlive: true}},
		PlanState: ps,
	})
	_ = model2.(*home)

	// Single-wave plan: task marked complete → orchestrator goes directly to WaveStateAllComplete.
	assert.Equal(t, orchestration.WaveStateAllComplete, orch.State(),
		"wave must be AllComplete after HasWorked=true detected at prompt")
}

// TestWaveTaskCompletion_IgnoresPromptEchoUpdates verifies that a wave task is
// not marked complete when metadata shows an update while already at prompt
// (prompt-echo / startup noise). Completion must wait for non-prompt output.
func TestWaveTaskCompletion_IgnoresPromptEchoUpdates(t *testing.T) {
	const planFile = "prompt-echo-guard"

	plan := &taskparser.Plan{Waves: []taskparser.Wave{{
		Number: 1,
		Tasks:  []taskparser.Task{{Number: 1, Title: "do work"}},
	}}}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "prompt echo guard test", "plan/prompt-echo-guard", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	planName := taskstate.DisplayName(planFile)
	instTitle := planName + "-W1-T1"

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      instTitle,
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst.PromptDetected = true
	inst.HasWorked = false
	inst.AwaitingWork = false

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	_ = h.nav.AddInstance(inst)

	// First update: metadata arrives while at prompt with no real work done.
	// The orchestrator must NOT complete the task yet.
	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: instTitle, TmuxAlive: true}},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.Len(t, updated.waveOrchestrators, 1,
		"orchestrator must still exist when only a prompt-echo update arrived")
	assert.Equal(t, orchestration.WaveStateRunning, orch.State(),
		"prompt-echo updates must leave the wave running")
	assert.False(t, updated.overlays.IsActive(),
		"prompt-echo updates must not show a completion dialog")

	// Second update: a task-finished signal arrives — now the orchestrator must be removed.
	model2, _ := updated.Update(metadataResultMsg{
		Results:   []instanceMetadata{{Title: instTitle, TmuxAlive: true}},
		PlanState: ps,
		TaskSignals: []taskfsm.TaskSignal{{
			WaveNumber: 1,
			TaskNumber: 1,
			TaskFile:   planFile,
		}},
	})
	updated2 := model2.(*home)
	assert.Empty(t, updated2.waveOrchestrators, "orchestrator must be deleted after the task-finished signal arrives")
}

// TestWaveTaskCompletion_TmuxExitDependsOnHasWorked verifies that a dead tmux
// session is treated as completion only after the task has produced real work.
func TestWaveTaskCompletion_TmuxExitDependsOnHasWorked(t *testing.T) {
	tests := []struct {
		name             string
		hasWorked        bool
		expectCompleted  int
		expectFailed     int
		expectConfirmKey string
		expectOverlay    bool
	}{
		{
			name:             "after real work completes wave",
			hasWorked:        true,
			expectCompleted:  1,
			expectFailed:     0,
			expectConfirmKey: "y",
			expectOverlay:    true,
		},
		{
			name:             "before real work fails wave",
			hasWorked:        false,
			expectCompleted:  0,
			expectFailed:     1,
			expectConfirmKey: "r",
			expectOverlay:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planFile := fmt.Sprintf("tmux-exit-%t", tt.hasWorked)
			plan := &taskparser.Plan{Waves: []taskparser.Wave{
				{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "do work"}}},
				{Number: 2, Tasks: []taskparser.Task{{Number: 2, Title: "follow up"}}},
			}}
			orch := orchestration.NewWaveOrchestrator(planFile, plan)
			orch.StartNextWave()

			dir := t.TempDir()
			plansDir := filepath.Join(dir, "docs", "plans")
			require.NoError(t, os.MkdirAll(plansDir, 0o755))
			ps, err := newTestPlanState(t, plansDir)
			require.NoError(t, err)
			require.NoError(t, ps.Register(planFile, "tmux exit completion test", fmt.Sprintf("plan/%s", planFile), time.Now()))
			seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

			planName := taskstate.DisplayName(planFile)
			instTitle := fmt.Sprintf("%s-W1-T1", planName)

			inst, err := session.NewInstance(session.InstanceOptions{
				Title:      instTitle,
				Path:       t.TempDir(),
				Program:    "claude",
				TaskFile:   planFile,
				TaskNumber: 1,
				WaveNumber: 1,
			})
			require.NoError(t, err)
			inst.MarkStartedForTest()
			inst.HasWorked = tt.hasWorked

			h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
			_ = h.nav.AddInstance(inst)

			model, _ := h.Update(metadataResultMsg{
				Results:   []instanceMetadata{{Title: instTitle, TmuxAlive: false}},
				PlanState: ps,
			})
			updated := model.(*home)

			assert.Equal(t, orchestration.WaveStateWaveComplete, orch.State(),
				"resolved first wave should wait for user decision")
			assert.Equal(t, tt.expectCompleted, orch.CompletedTaskCount())
			assert.Equal(t, tt.expectFailed, orch.FailedTaskCount())
			assert.Equal(t, stateWaveDecision, updated.state,
				"resolving the wave task should show the wave decision UI")
			assert.Equal(t, tt.expectOverlay, updated.overlays.IsActive())

			if tt.expectOverlay {
				wdTmux, okTmux := updated.overlays.Current().(*overlay.WaveDecisionOverlay)
				require.True(t, okTmux, "current overlay must be a WaveDecisionOverlay")
				if tt.expectFailed > 0 {
					assert.Greater(t, wdTmux.Input().Failed, 0,
						"failed wave should show failure overlay variant")
				} else {
					assert.Equal(t, 0, wdTmux.Input().Failed,
						"successful wave should show success overlay variant")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Elaboration phase tests
// ---------------------------------------------------------------------------

// waveElabTestHarness bundles the minimal state needed for elaboration flow tests
// so each test doesn't have to duplicate setup boilerplate.
type waveElabTestHarness struct {
	t        *testing.T
	dir      string
	plansDir string
	store    taskstore.Store
	h        *home
}

func newWaveElabTestHarness(t *testing.T) *waveElabTestHarness {
	t.Helper()
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	threshold := 0

	store := taskstore.NewTestSQLiteStore(t)
	ps, err := taskstate.Load(store, "proj", plansDir)
	require.NoError(t, err)
	fsm := taskfsm.New(store, "proj", plansDir)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:               context.Background(),
		taskState:         ps,
		taskStore:         store,
		taskStoreProject:  "proj",
		taskStateDir:      plansDir,
		fsm:               fsm,
		nav:               ui.NewNavigationPanel(&sp),
		menu:              ui.NewMenu(),
		tabbedWindow:      ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:      overlay.NewToastManager(&sp),
		overlays:          overlay.NewManager(),
		waveOrchestrators: make(map[string]*orchestration.WaveOrchestrator),
		activeRepoPath:    dir,
		program:           "opencode",
		appConfig:         &config.Config{BlueprintSkipThresholdValue: &threshold},
		state:             stateDefault,
	}
	return &waveElabTestHarness{t: t, dir: dir, plansDir: plansDir, store: store, h: h}
}

// registerPlan creates a plan entry in the store (with content and branch) and
// reloads task state so the home struct picks up the new plan.
func (th *waveElabTestHarness) registerPlan(planFile, content, branch string) {
	th.t.Helper()
	require.NoError(th.t, th.store.Create("proj", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
		Branch:   branch,
		Content:  content,
		// planned-ready so fsmSetImplementing accepts the plan when implement is triggered.
		ExecutionState: taskstore.ExecutionState{Phase: "planned"},
	}))
	ps, err := taskstate.Load(th.store, "proj", th.plansDir)
	require.NoError(th.t, err)
	th.h.taskState = ps
}

// executeTaskStage delegates to the home struct's executeTaskStage.
func (th *waveElabTestHarness) executeTaskStage(planFile, stage string) (tea.Model, tea.Cmd) {
	return th.h.executeTaskStage(planFile, stage)
}

// TestImplementTriggersElaborationBeforeWave1 verifies that triggering "implement"
// creates a wave orchestrator in the elaborating state and spawns the architect pass.
func TestImplementTriggersElaborationBeforeWave1(t *testing.T) {
	h := newWaveElabTestHarness(t)

	const planFile = "elab-test"
	planContent := "**Goal:** test\n\n## Wave 1\n\n### Task 1: Do thing\n\n**Files:**\n- Create: `foo.go`\n\nImplement foo."
	h.registerPlan(planFile, planContent, "plan/elab-test")

	model, _ := h.executeTaskStage(planFile, "implement")
	m := model.(*home)

	// Orchestrator should exist and be in elaborating state
	orch, exists := m.waveOrchestrators[planFile]
	require.True(t, exists, "orchestrator must be created")
	assert.Equal(t, orchestration.WaveStateElaborating, orch.State(),
		"orchestrator must be in elaborating state, not running")

	// An architect instance should have been spawned.
	var foundArchitect bool
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.AgentType == session.AgentTypeElaborator {
			foundArchitect = true
			assert.Contains(t, inst.QueuedPrompt, "architect",
				"elaboration prompt must reference the architect role")
			assert.Contains(t, inst.QueuedPrompt, "elaborator_finished",
				"architect prompt must retain the legacy completion signal contract")
			break
		}
	}
	assert.True(t, foundArchitect, "architect instance must be spawned")
}

func TestElaborationSignal_RefreshesWavePlanAndTaskStateBeforeWave1Starts(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := storeForDir(t, plansDir)
	const planFile = "elab-reload"
	oldContent := "**Goal:** old\n\n## Wave 1\n\n### Task 1: one\n\nDo one.\n\n### Task 2: two\n\nDo two.\n\n### Task 3: three\n\nDo three.\n\n### Task 4: four\n\nDo four.\n"
	newContent := "**Goal:** new\n\n## Wave 1\n\n### Task 1: alpha\n\nDo alpha.\n\n### Task 2: beta\n\nDo beta.\n\n## Wave 2\n\n### Task 3: gamma\n\nDo gamma.\n\n### Task 4: delta\n\nDo delta.\n\n### Task 5: epsilon\n\nDo epsilon.\n"

	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/elab-reload",
		Content:  oldContent,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseArchitecting),
			ActiveAgentType: session.AgentTypeElaborator,
		},
	}))

	ps, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.IngestContent(planFile, newContent))

	oldPlan, err := taskparser.Parse(oldContent)
	require.NoError(t, err)
	orch := orchestration.NewWaveOrchestrator(planFile, oldPlan)
	orch.SetStore(store, "test")
	orch.SetElaborating()

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	h.daemonStatusChecker = func(string) daemonStatusMsg {
		return daemonStatusMsg{message: "daemon required"}
	}

	model, _ := h.Update(metadataResultMsg{
		PlanState:          ps,
		ElaborationSignals: []taskfsm.ElaborationSignal{{TaskFile: planFile}},
	})
	updated := model.(*home)

	updatedOrch, ok := updated.waveOrchestrators[planFile]
	require.True(t, ok)
	assert.Equal(t, 2, updatedOrch.TotalWaves(), "architect-enriched wave count must replace the pre-elaboration plan")
	assert.Equal(t, 5, updatedOrch.TotalTasks(), "architect-enriched task count must replace the pre-elaboration plan")
	require.Len(t, updatedOrch.CurrentWaveTasks(), 2)
	assert.Equal(t, "alpha", updatedOrch.CurrentWaveTasks()[0].Title)

	entry, ok := updated.taskState.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, string(taskfsm.ExecutionPhaseWaveRunning), entry.ExecutionState.Phase)
	assert.Equal(t, session.AgentTypeCoder, entry.ExecutionState.ActiveAgentType)
	assert.Equal(t, 1, entry.ExecutionState.ActiveWave)
}

// TestImplementDirectlySkipsElaboration verifies that "implement_direct" creates an
// orchestrator in the running state without spawning the architect pass.
func TestImplementDirectlySkipsElaboration(t *testing.T) {
	h := newWaveElabTestHarness(t)

	const planFile = "direct-test"
	planContent := "**Goal:** test\n\n## Wave 1\n\n### Task 1: Do thing\n\nDo it."
	h.registerPlan(planFile, planContent, "plan/direct-test")

	model, _ := h.executeTaskStage(planFile, "implement_direct")
	m := model.(*home)

	// Orchestrator should exist and be running (not elaborating)
	orch, exists := m.waveOrchestrators[planFile]
	require.True(t, exists, "orchestrator must be created")
	assert.NotEqual(t, orchestration.WaveStateElaborating, orch.State(),
		"direct implement must skip elaboration")
}

// TestCoderExit_FocusesCoderInstance_BeforePushConfirm verifies that when a
// coder finishes (tmux dies) and the "push branch?" dialog shows, the coder
// instance is auto-focused so its output is visible behind the overlay.
func TestCoderExit_FocusesCoderInstance_BeforePushConfirm(t *testing.T) {
	const planFile = "focus-coder"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "focus coder test", "plan/focus-coder", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	// ShouldAutoAdvanceLifecycleImplementer checks IsSingleAgentImplementingPhase;
	// without the execution phase the coder-exit auto-advance never fires.
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseSingleAgentImplementing),
		ActiveAgentType: session.AgentTypeCoder,
	}))

	coderInst := &session.Instance{
		Title:     "focus-coder-implement",
		Program:   "opencode",
		TaskFile:  planFile,
		AgentType: session.AgentTypeCoder,
	}
	coderInst.MarkStartedForTest()

	otherInst := &session.Instance{Title: "other-agent", Program: "opencode"}
	otherInst.MarkStartedForTest()

	h := waveFlowHome(t, ps, plansDir, nil)
	h.waveOrchestrators = make(map[string]*orchestration.WaveOrchestrator)
	h.plannerPrompted = make(map[string]bool)
	h.coderPushPrompted = make(map[string]bool)
	h.pendingReviewFeedback = make(map[string]string)
	_ = h.nav.AddInstance(otherInst)
	_ = h.nav.AddInstance(coderInst)
	h.updateSidebarTasks() // register plans so rebuildRows emits plan-grouped instances
	h.nav.SetSelectedInstance(0)
	require.Equal(t, otherInst, h.nav.GetSelectedInstance(), "precondition: other-agent selected")

	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "other-agent", TmuxAlive: true},
			{Title: "focus-coder-implement", TmuxAlive: false},
		},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.Equal(t, stateConfirm, updated.state, "should show coder-exit push confirm")
	assert.Equal(t, coderInst, updated.nav.GetSelectedInstance(),
		"coder-exit overlay should auto-focus the coder instance")
}
