package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWaveMonitor_AutoDetectSetsImplementationComplete verifies that when the
// wave monitor auto-detects task completion (HasWorked + PromptDetected), the
// instance's ImplementationComplete flag is set to true. This prevents the
// sidebar from reverting to a spinner after attach/detach cycles.
func TestWaveMonitor_AutoDetectSetsImplementationComplete(t *testing.T) {
	t.Parallel()
	const planFile = "autodetect-complete"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{
				{Number: 1, Title: "Task 1", Body: "do it"},
				{Number: 2, Title: "Task 2", Body: "also do it"},
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
	require.NoError(t, ps.Register(planFile, "autodetect test", "plan/autodetect-complete", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	planName := taskstate.DisplayName(planFile)
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      planName + "-W1-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst.HasWorked = true
	inst.PromptDetected = true
	inst.AwaitingWork = false

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	_ = h.nav.AddInstance(inst)

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: inst.Title, TmuxAlive: true, ContentCaptured: true}},
		PlanState: ps,
	}
	h.Update(msg)

	assert.True(t, inst.ImplementationComplete,
		"wave auto-detect must set ImplementationComplete so sidebar shows checkmark")
	assert.True(t, orch.IsTaskComplete(1),
		"task must be marked complete in orchestrator")
}

// TestWaveMonitor_TmuxDeathSetsImplementationComplete verifies that when a
// tmux session dies after the agent did real work, the instance gets
// ImplementationComplete=true alongside the orchestrator's MarkTaskComplete.
func TestWaveMonitor_TmuxDeathSetsImplementationComplete(t *testing.T) {
	t.Parallel()
	const planFile = "tmuxdeath-complete"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{
				{Number: 1, Title: "Task 1", Body: "do it"},
				{Number: 2, Title: "Task 2", Body: "also do it"},
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
	require.NoError(t, ps.Register(planFile, "tmux death test", "plan/tmuxdeath-complete", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	planName := taskstate.DisplayName(planFile)
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      planName + "-W1-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst.HasWorked = true

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	_ = h.nav.AddInstance(inst)

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: inst.Title, TmuxAlive: false, ContentCaptured: true}},
		PlanState: ps,
	}
	h.Update(msg)

	assert.True(t, inst.ImplementationComplete,
		"tmux death with HasWorked must set ImplementationComplete")
	assert.True(t, orch.IsTaskComplete(1),
		"task must be marked complete in orchestrator")
}

// TestMarkTaskComplete_SetsImplementationComplete verifies that the manual
// "mark complete" context menu action sets ImplementationComplete on the
// selected instance, not just the orchestrator.
func TestMarkTaskComplete_SetsImplementationComplete(t *testing.T) {
	t.Parallel()
	const planFile = "manual-complete"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{
				{Number: 1, Title: "Task 1", Body: "do it"},
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
	require.NoError(t, ps.Register(planFile, "manual complete test", "plan/manual-complete", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "manual-complete-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)

	h := waveFlowHome(t, ps, plansDir, map[string]*orchestration.WaveOrchestrator{planFile: orch})
	h.activeRepoPath = t.TempDir()
	_ = h.nav.AddInstance(inst)
	h.updateSidebarTasks()
	selected := h.nav.SelectInstance(inst)
	require.True(t, selected, "instance must be selectable after sidebar rebuild")

	h.executeContextAction("mark_task_complete")

	assert.True(t, inst.ImplementationComplete,
		"mark_task_complete must set ImplementationComplete so sidebar shows checkmark")
	assert.True(t, orch.IsTaskComplete(1),
		"task must be marked complete in orchestrator")
}

// TestPersistedSubtaskCompletionSetsImplementationComplete verifies that when a
// daemon-managed repo has already marked a wave subtask complete in the store,
// the sidebar instance picks up the completed glyph on the next metadata tick.
func TestPersistedSubtaskCompletionSetsImplementationComplete(t *testing.T) {
	t.Parallel()
	const planFile = "persisted-complete"

	content := `# Plan

**Goal:** show completed task glyphs

## Wave 1

### Task 1: first

Finish the first task.

### Task 2: second

Finish the second task.
`

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Create(planFile, "persisted completion", "plan/persisted-complete", "", time.Now()))
	require.NoError(t, ps.IngestContent(planFile, content))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	require.NoError(t, ps.UpdateSubtaskStatus(planFile, 1, taskstore.SubtaskStatusComplete))

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         taskstate.DisplayName(planFile) + "-W1-T1",
		Path:          t.TempDir(),
		Program:       "claude",
		TaskFile:      planFile,
		TaskNumber:    1,
		WaveNumber:    1,
		WaveTaskIndex: 1,
		WaveTaskCount: 2,
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	inst.SetStatus(session.Running)

	h := waveFlowHome(t, ps, plansDir, nil)
	_ = h.nav.AddInstance(inst)

	model, _ := h.Update(metadataResultMsg{
		Results:           []instanceMetadata{{Title: inst.Title, TmuxAlive: true}},
		PlanState:         ps,
		DaemonManagedRepo: true,
	})
	_ = model.(*home)

	assert.True(t, inst.ImplementationComplete,
		"persisted complete subtask must set ImplementationComplete so the sidebar shows a checkmark")
}
