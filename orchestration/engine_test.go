package orchestration

import (
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWavePlan(waves ...[]taskparser.Task) *taskparser.Plan {
	plan := &taskparser.Plan{Waves: make([]taskparser.Wave, 0, len(waves))}
	for i, tasks := range waves {
		plan.Waves = append(plan.Waves, taskparser.Wave{
			Number: i + 1,
			Tasks:  tasks,
		})
	}
	return plan
}

func TestNewWaveOrchestrator(t *testing.T) {
	plan := testWavePlan(
		[]taskparser.Task{
			{Number: 1, Title: "First", Body: "do first"},
			{Number: 2, Title: "Second", Body: "do second"},
		},
		[]taskparser.Task{{Number: 3, Title: "Third", Body: "do third"}},
	)
	plan.Goal = "test"

	orch := NewWaveOrchestrator("plan", plan)
	assert.Equal(t, WaveStateIdle, orch.State())
	assert.Equal(t, 2, orch.TotalWaves())
	assert.Equal(t, 3, orch.TotalTasks())
}

func TestWaveOrchestrator_LoadsArchitectMeta(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, ".kasmos", "cache")
	meta := &ArchitectMeta{
		PlanID: "test-plan",
		Waves: []WaveMeta{{
			Wave: 1,
			Tasks: []TaskMeta{{
				TaskNumber:     1,
				PreferredModel: "openai/gpt-5.3-codex-spark",
				VerifyChecks:   []string{"go test ./..."},
			}},
		}},
	}
	require.NoError(t, SaveArchitectMeta(cacheDir, "test-plan", meta))

	plan := testWavePlan([]taskparser.Task{{Number: 1, Title: "Task 1"}})
	orch := NewWaveOrchestrator("test-plan", plan)
	orch.LoadArchitectMeta(cacheDir)

	got := orch.GetTaskMeta(1)
	require.NotNil(t, got)
	assert.Equal(t, "openai/gpt-5.3-codex-spark", got.PreferredModel)
	assert.Nil(t, orch.GetTaskMeta(99))
}

func TestWaveOrchestrator_NoArchitectMeta(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{{Number: 1, Title: "Task 1"}})
	orch := NewWaveOrchestrator("test-plan", plan)
	orch.LoadArchitectMeta(t.TempDir())

	assert.Nil(t, orch.GetTaskMeta(1))
}

func TestWaveOrchestrator_StartWave(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "First", Body: "do first"},
	})

	orch := NewWaveOrchestrator("plan", plan)
	tasks := orch.StartNextWave()

	assert.Equal(t, WaveStateRunning, orch.State())
	assert.Equal(t, 1, orch.CurrentWaveNumber())
	require.Len(t, tasks, 1)
	assert.Equal(t, "First", tasks[0].Title)
}

func TestWaveOrchestrator_TaskCompleted(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "First", Body: "do first"},
		{Number: 2, Title: "Second", Body: "do second"},
	})

	orch := NewWaveOrchestrator("plan", plan)
	orch.StartNextWave()

	assert.False(t, orch.IsCurrentWaveComplete())

	orch.MarkTaskComplete(1)
	assert.False(t, orch.IsCurrentWaveComplete())

	orch.MarkTaskComplete(2)
	assert.True(t, orch.IsCurrentWaveComplete())
	assert.Equal(t, WaveStateAllComplete, orch.State())
}

func TestWaveOrchestrator_TaskFailed(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "First", Body: "do first"},
		{Number: 2, Title: "Second", Body: "do second"},
	})

	orch := NewWaveOrchestrator("plan", plan)
	orch.StartNextWave()

	orch.MarkTaskFailed(1)
	orch.MarkTaskComplete(2)

	assert.Equal(t, WaveStateAllComplete, orch.State())
	assert.Equal(t, 1, orch.FailedTaskCount())
	assert.Equal(t, 1, orch.CompletedTaskCount())
}

func TestWaveOrchestrator_MultiWaveProgression(t *testing.T) {
	plan := testWavePlan(
		[]taskparser.Task{{Number: 1, Title: "First", Body: "do first"}},
		[]taskparser.Task{{Number: 2, Title: "Second", Body: "do second"}},
	)

	orch := NewWaveOrchestrator("plan", plan)

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
	plan := testWavePlan([]taskparser.Task{{Number: 1, Title: "Only", Body: "do it"}})

	orch := NewWaveOrchestrator("plan", plan)
	orch.StartNextWave()
	orch.MarkTaskComplete(1)

	// No more waves — should be AllComplete
	assert.Equal(t, WaveStateAllComplete, orch.State())
}

func TestWaveOrchestrator_ResetConfirmAllowsReprompt(t *testing.T) {
	plan := testWavePlan(
		[]taskparser.Task{{Number: 1, Title: "First", Body: "do first"}},
		[]taskparser.Task{{Number: 2, Title: "Second", Body: "do second"}},
	)

	orch := NewWaveOrchestrator("plan", plan)
	orch.StartNextWave()
	orch.MarkTaskComplete(1) // wave 1 complete

	// First call consumes the one-shot latch
	assert.True(t, orch.NeedsConfirm(), "first call must return true")
	assert.False(t, orch.NeedsConfirm(), "second call must return false (latch consumed)")

	// After reset, NeedsConfirm should fire again
	orch.ResetConfirm()
	assert.True(t, orch.NeedsConfirm(), "after ResetConfirm, must return true again")
}

func TestIsTaskRunning(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{{Number: 1}, {Number: 2}})
	orch := NewWaveOrchestrator("test", plan)
	orch.StartNextWave()

	assert.True(t, orch.IsTaskRunning(1), "task 1 should be running after StartNextWave")
	assert.True(t, orch.IsTaskRunning(2), "task 2 should be running after StartNextWave")

	orch.MarkTaskComplete(1)
	assert.False(t, orch.IsTaskRunning(1), "task 1 should not be running after MarkTaskComplete")
	assert.True(t, orch.IsTaskRunning(2), "task 2 should still be running")

	assert.False(t, orch.IsTaskRunning(99), "unknown task should return false")
}

func TestWaveOrchestrator_TaskStatusQueries(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "Task 1"},
		{Number: 2, Title: "Task 2"},
		{Number: 3, Title: "Task 3"},
	})
	orch := NewWaveOrchestrator("test", plan)
	orch.StartNextWave()

	// All should be running initially.
	assert.True(t, orch.IsTaskRunning(1))
	assert.False(t, orch.IsTaskComplete(1))
	assert.False(t, orch.IsTaskFailed(1))

	orch.MarkTaskComplete(1)
	assert.True(t, orch.IsTaskComplete(1))
	assert.False(t, orch.IsTaskRunning(1))

	orch.MarkTaskFailed(2)
	assert.True(t, orch.IsTaskFailed(2))
	assert.False(t, orch.IsTaskRunning(2))

	// Task 3 still running.
	assert.True(t, orch.IsTaskRunning(3))
}

func TestWaveOrchestrator_RetryFailedTasksRestoresRunning(t *testing.T) {
	plan := testWavePlan(
		[]taskparser.Task{
			{Number: 1, Title: "First", Body: "do first"},
			{Number: 2, Title: "Second", Body: "do second"},
		},
		[]taskparser.Task{{Number: 3, Title: "Third", Body: "do third"}},
	)

	orch := NewWaveOrchestrator("plan", plan)
	orch.StartNextWave()

	// T1 fails, T2 completes — wave done with failure
	orch.MarkTaskFailed(1)
	orch.MarkTaskComplete(2)
	require.Equal(t, WaveStateWaveComplete, orch.State(), "wave must be WaveComplete with failure")
	assert.Equal(t, 1, orch.FailedTaskCount())

	// Retry the failed task
	retried := orch.RetryFailedTasks()

	assert.Equal(t, WaveStateRunning, orch.State(), "state must be Running after retry")
	require.Len(t, retried, 1, "must return only the failed task")
	assert.Equal(t, 1, retried[0].Number, "retried task must be T1")

	// After the retried task completes, wave is done again (with more waves pending)
	orch.MarkTaskComplete(1)
	assert.Equal(t, WaveStateWaveComplete, orch.State(), "wave must be WaveComplete after retry+complete")
	assert.Equal(t, 0, orch.FailedTaskCount(), "no more failures after retry completes")
}

func TestClaimWaveOutcome_SameGenerationBlocksDuplicate(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{{Number: 1, Title: "first"}})
	orch := NewWaveOrchestrator("plan", plan)
	orch.StartNextWave()
	orch.MarkTaskComplete(1)

	assert.True(t, orch.ClaimWaveOutcome())
	assert.False(t, orch.ClaimWaveOutcome())
	assert.Equal(t, 0, orch.RetryGeneration())
}

func TestClaimWaveOutcome_RetryAllowsEmission(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{{Number: 1, Title: "first"}})
	orch := NewWaveOrchestrator("plan", plan)
	orch.StartNextWave()
	orch.MarkTaskFailed(1)

	assert.True(t, orch.ClaimWaveOutcome())
	retried := orch.RetryFailedTasks()
	require.Len(t, retried, 1)
	assert.Equal(t, 1, orch.RetryGeneration())
	orch.MarkTaskFailed(1)
	assert.True(t, orch.ClaimWaveOutcome())
	assert.False(t, orch.ClaimWaveOutcome())
}

func TestClaimWaveOutcome_RetryGenerationResetsOnNextWave(t *testing.T) {
	plan := testWavePlan(
		[]taskparser.Task{{Number: 1, Title: "first"}},
		[]taskparser.Task{{Number: 2, Title: "second"}},
	)
	orch := NewWaveOrchestrator("plan", plan)
	orch.StartNextWave()
	orch.MarkTaskFailed(1)

	assert.True(t, orch.ClaimWaveOutcome())
	require.Len(t, orch.RetryFailedTasks(), 1)
	assert.Equal(t, 1, orch.RetryGeneration())
	orch.MarkTaskComplete(1)
	assert.True(t, orch.ClaimWaveOutcome())
	assert.False(t, orch.ClaimWaveOutcome())

	tasks := orch.StartNextWave()
	require.Len(t, tasks, 1)
	assert.Equal(t, 2, orch.CurrentWaveNumber())
	assert.Equal(t, 0, orch.RetryGeneration())

	orch.MarkTaskComplete(2)
	assert.True(t, orch.ClaimWaveOutcome())
	assert.False(t, orch.ClaimWaveOutcome())
	assert.Equal(t, 0, orch.RetryGeneration())
}

func TestUpdatePlan_ResetsOutcomeClaims(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{{Number: 1, Title: "first"}})
	orch := NewWaveOrchestrator("plan", plan)
	orch.StartNextWave()
	orch.MarkTaskFailed(1)
	assert.True(t, orch.ClaimWaveOutcome())
	require.Len(t, orch.RetryFailedTasks(), 1)
	assert.Equal(t, 1, orch.RetryGeneration())

	orch.UpdatePlan(plan)

	assert.Equal(t, 0, orch.RetryGeneration())
	assert.True(t, orch.ClaimWaveOutcome())
	assert.False(t, orch.ClaimWaveOutcome())
}

func TestRestoreToWave(t *testing.T) {
	plan := testWavePlan(
		[]taskparser.Task{{Number: 1}, {Number: 2}},
		[]taskparser.Task{{Number: 3}},
	)
	orch := NewWaveOrchestrator("plan", plan)
	orch.RestoreToWave(2, []int{3})
	assert.Equal(t, WaveStateAllComplete, orch.State())
	assert.Equal(t, 2, orch.CurrentWaveNumber())
}

func TestRestoreToWave_PartialCompletion(t *testing.T) {
	plan := testWavePlan(
		[]taskparser.Task{{Number: 1}},
		[]taskparser.Task{{Number: 2}, {Number: 3}},
	)
	orch := NewWaveOrchestrator("plan", plan)
	orch.RestoreToWave(2, []int{2}) // task 3 still running
	assert.Equal(t, WaveStateRunning, orch.State())
	assert.True(t, orch.IsTaskComplete(2))
	assert.True(t, orch.IsTaskRunning(3))
}

func TestShouldPostWaveCompleteComment(t *testing.T) {
	single := testWavePlan([]taskparser.Task{{Number: 1}})
	multi := testWavePlan(
		[]taskparser.Task{{Number: 1}},
		[]taskparser.Task{{Number: 2}},
	)
	assert.False(t, NewWaveOrchestrator("s", single).ShouldPostWaveCompleteComment())
	assert.True(t, NewWaveOrchestrator("m", multi).ShouldPostWaveCompleteComment())

	// nil receiver safety
	var nilOrch *WaveOrchestrator
	assert.False(t, nilOrch.ShouldPostWaveCompleteComment())
}

func TestWaveOrchestrator_ElaboratingState(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{{Number: 1, Title: "First", Body: "do first"}})

	orch := NewWaveOrchestrator("plan", plan)
	orch.SetElaborating()
	assert.Equal(t, WaveStateElaborating, orch.State())

	// StartNextWave should be blocked while elaborating
	tasks := orch.StartNextWave()
	assert.Nil(t, tasks, "must not start waves while elaborating")
	assert.Equal(t, WaveStateElaborating, orch.State())
}

func TestWaveOrchestrator_UpdatePlan(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{{Number: 1, Title: "First", Body: "terse body"}})
	plan.Goal = "original"

	orch := NewWaveOrchestrator("plan", plan)
	orch.SetElaborating()

	updated := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "First", Body: "detailed body with signatures and patterns"},
	})
	updated.Goal = "original"
	orch.UpdatePlan(updated)

	// Should transition back to Idle so StartNextWave works
	assert.Equal(t, WaveStateIdle, orch.State())

	// Verify the plan was replaced
	tasks := orch.StartNextWave()
	require.Len(t, tasks, 1)
	assert.Contains(t, tasks[0].Body, "detailed body")
}

func TestBuildTaskPrompt_Method(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "First", Body: "do first"},
		{Number: 2, Title: "Second", Body: "do second"},
	})
	plan.Goal = "Test goal"
	orch := NewWaveOrchestrator("plan", plan)
	orch.SetStore(nil, "testproject")
	orch.StartNextWave()
	prompt := orch.BuildTaskPrompt(plan.Waves[0].Tasks[0], 2)
	assert.Contains(t, prompt, "Task 1")
	assert.Contains(t, prompt, "Test goal")
	assert.Contains(t, prompt, "Wave 1 of 1")
	assert.Contains(t, prompt, "parallel") // peerCount > 1
	assert.Contains(t, prompt, `project: "testproject"`)
}

func TestWaveOrchestrator_PreferredModelForTask(t *testing.T) {
	cacheDir := t.TempDir()
	meta := &ArchitectMeta{
		PlanID: "model-plan",
		Waves: []WaveMeta{{
			Wave: 1,
			Tasks: []TaskMeta{
				{TaskNumber: 1, PreferredModel: "openai/gpt-5.3-codex-spark", FallbackModel: "openai/gpt-5.4"},
				{TaskNumber: 2},
			},
		}},
	}
	require.NoError(t, SaveArchitectMeta(cacheDir, "model-plan", meta))

	plan := testWavePlan([]taskparser.Task{{Number: 1}, {Number: 2}})
	orch := NewWaveOrchestrator("model-plan", plan)
	orch.LoadArchitectMeta(cacheDir)

	assert.Equal(t, "openai/gpt-5.3-codex-spark", orch.PreferredModelForTask(1))
	assert.Equal(t, "openai/gpt-5.4", orch.FallbackModelForTask(1))
	assert.Empty(t, orch.PreferredModelForTask(2))
	assert.Empty(t, orch.FallbackModelForTask(2))
	assert.Empty(t, orch.PreferredModelForTask(99))
}

func TestWaveOrchestrator_DetectFileConflicts(t *testing.T) {
	cacheDir := t.TempDir()
	meta := &ArchitectMeta{
		PlanID: "conflict-plan",
		Waves: []WaveMeta{{
			Wave: 1,
			Tasks: []TaskMeta{
				{TaskNumber: 1, FilesToModify: []string{"shared.go", "task1.go"}},
				{TaskNumber: 2, FilesToModify: []string{"shared.go", "task2.go"}},
			},
		}},
	}
	require.NoError(t, SaveArchitectMeta(cacheDir, "conflict-plan", meta))

	plan := testWavePlan([]taskparser.Task{{Number: 1}, {Number: 2}})
	orch := NewWaveOrchestrator("conflict-plan", plan)
	orch.LoadArchitectMeta(cacheDir)

	conflicts := orch.DetectFileConflicts(1)
	require.Len(t, conflicts, 1)
	assert.Equal(t, "shared.go", conflicts[0].File)
	assert.ElementsMatch(t, []int{1, 2}, conflicts[0].TaskNumbers)
}

func TestWaveOrchestrator_DetectFileConflicts_NoMeta(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{{Number: 1}})
	orch := NewWaveOrchestrator("no-meta", plan)

	conflicts := orch.DetectFileConflicts(1)
	assert.Empty(t, conflicts)
}

func TestWaveOrchestrator_PreferredModelForTask_NoMeta(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{{Number: 1}})
	orch := NewWaveOrchestrator("no-meta", plan)
	assert.Empty(t, orch.PreferredModelForTask(1))
}

func TestWaveOrchestrator_PersistsSubtaskStatus(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "proj"
	planFile := "plan"

	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: planFile, Status: taskstore.StatusImplementing}))
	require.NoError(t, store.SetSubtasks(project, planFile, []taskstore.SubtaskEntry{
		{TaskNumber: 1, Title: "first", Status: taskstore.SubtaskStatusPending},
		{TaskNumber: 2, Title: "second", Status: taskstore.SubtaskStatusPending},
	}))

	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "first"},
		{Number: 2, Title: "second"},
	})
	orch := NewWaveOrchestrator(planFile, plan)
	orch.SetStore(store, project)

	orch.StartNextWave()
	subtasks, err := store.GetSubtasks(project, planFile)
	require.NoError(t, err)
	require.Len(t, subtasks, 2)
	assert.Equal(t, taskstore.SubtaskStatusRunning, subtasks[0].Status)
	assert.Equal(t, taskstore.SubtaskStatusRunning, subtasks[1].Status)

	orch.MarkTaskComplete(1)
	orch.MarkTaskFailed(2)

	subtasks, err = store.GetSubtasks(project, planFile)
	require.NoError(t, err)
	require.Len(t, subtasks, 2)
	assert.Equal(t, taskstore.SubtaskStatusComplete, subtasks[0].Status)
	assert.Equal(t, taskstore.SubtaskStatusFailed, subtasks[1].Status)
}

// TestStartNextWaveLimited_ZeroLimit delegates to unlimited behavior.
func TestStartNextWaveLimited_ZeroLimit(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "T1"},
		{Number: 2, Title: "T2"},
		{Number: 3, Title: "T3"},
	})
	orch := NewWaveOrchestrator("plan", plan)
	tasks, err := orch.StartNextWaveLimited(0)
	require.NoError(t, err)
	assert.Len(t, tasks, 3, "limit=0 must start all tasks")
	assert.Equal(t, WaveStateRunning, orch.State())
	assert.True(t, orch.IsTaskRunning(1))
	assert.True(t, orch.IsTaskRunning(2))
	assert.True(t, orch.IsTaskRunning(3))
}

// TestStartNextWaveLimited_LimitOne only starts first task, others pending.
func TestStartNextWaveLimited_LimitOne(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "T1"},
		{Number: 2, Title: "T2"},
		{Number: 3, Title: "T3"},
	})
	orch := NewWaveOrchestrator("plan", plan)
	tasks, err := orch.StartNextWaveLimited(1)
	require.NoError(t, err)
	require.Len(t, tasks, 1, "limit=1 must start exactly one task")
	assert.Equal(t, 1, tasks[0].Number)
	assert.Equal(t, WaveStateRunning, orch.State())

	assert.True(t, orch.IsTaskRunning(1))
	assert.False(t, orch.IsTaskRunning(2))
	assert.False(t, orch.IsTaskRunning(3))
	assert.False(t, orch.IsTaskComplete(2))
	assert.False(t, orch.IsTaskFailed(2))
	assert.Equal(t, 1, orch.ActiveTaskCount())
}

// TestStartNextWaveLimited_PendingDoesNotCompleteWave ensures wave stays running while pending tasks exist.
func TestStartNextWaveLimited_PendingDoesNotCompleteWave(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "T1"},
		{Number: 2, Title: "T2"},
		{Number: 3, Title: "T3"},
	})
	orch := NewWaveOrchestrator("plan", plan)
	tasks, err := orch.StartNextWaveLimited(1)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Completing task 1 should NOT complete the wave — tasks 2 and 3 are pending.
	orch.MarkTaskComplete(1)
	assert.Equal(t, WaveStateRunning, orch.State(), "wave must stay running while pending tasks remain")
	assert.False(t, orch.IsCurrentWaveComplete())
}

// TestStartPendingTasks_LaunchesUpToCapacity verifies that pending tasks are launched as capacity opens.
func TestStartPendingTasks_LaunchesUpToCapacity(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "T1"},
		{Number: 2, Title: "T2"},
		{Number: 3, Title: "T3"},
	})
	orch := NewWaveOrchestrator("plan", plan)
	_, err := orch.StartNextWaveLimited(1)
	require.NoError(t, err)
	assert.Equal(t, 1, orch.ActiveTaskCount())

	// Complete task 1 — now capacity opens.
	orch.MarkTaskComplete(1)
	assert.Equal(t, WaveStateRunning, orch.State())

	pending, err := orch.StartPendingTasks(1)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, 2, pending[0].Number)
	assert.True(t, orch.IsTaskRunning(2))
	assert.Equal(t, 1, orch.ActiveTaskCount())
}

// TestStartPendingTasks_FullDrainWithLimit1 simulates a 3-task wave with limit=1 draining sequentially.
func TestStartPendingTasks_FullDrainWithLimit1(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "T1"},
		{Number: 2, Title: "T2"},
		{Number: 3, Title: "T3"},
	})
	orch := NewWaveOrchestrator("plan", plan)
	launched, err := orch.StartNextWaveLimited(1)
	require.NoError(t, err)
	require.Len(t, launched, 1)

	// T1 completes → T2 launched
	orch.MarkTaskComplete(1)
	require.Equal(t, WaveStateRunning, orch.State())
	pending, err := orch.StartPendingTasks(1)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, 2, pending[0].Number)

	// T2 completes → T3 launched
	orch.MarkTaskComplete(2)
	require.Equal(t, WaveStateRunning, orch.State())
	pending, err = orch.StartPendingTasks(1)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, 3, pending[0].Number)

	// T3 completes → wave done
	orch.MarkTaskComplete(3)
	assert.Equal(t, WaveStateAllComplete, orch.State())
}

// TestStartPendingTasks_NoOpWhenNotRunning returns nil outside WaveStateRunning.
func TestStartPendingTasks_NoOpWhenNotRunning(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{{Number: 1, Title: "T1"}})
	orch := NewWaveOrchestrator("plan", plan)
	// State is Idle — StartPendingTasks must be a no-op.
	pending, err := orch.StartPendingTasks(1)
	require.NoError(t, err)
	assert.Empty(t, pending)
}

// TestStartPendingTasks_FullyAtCapacity returns nil when no slots are free.
func TestStartPendingTasks_FullyAtCapacity(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "T1"},
		{Number: 2, Title: "T2"},
	})
	orch := NewWaveOrchestrator("plan", plan)
	launched, err := orch.StartNextWaveLimited(1)
	require.NoError(t, err)
	require.Len(t, launched, 1)

	// Active = 1, limit = 1 → no capacity.
	pending, err := orch.StartPendingTasks(1)
	require.NoError(t, err)
	assert.Empty(t, pending, "no slots free while task 1 is running")
}

// TestStartPendingTasks_FailureStillOpensCapacity confirms a failed task releases a slot.
func TestStartPendingTasks_FailureStillOpensCapacity(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "T1"},
		{Number: 2, Title: "T2"},
		{Number: 3, Title: "T3"},
	})
	orch := NewWaveOrchestrator("plan", plan)
	launched, err := orch.StartNextWaveLimited(1)
	require.NoError(t, err)
	require.Len(t, launched, 1)

	// T1 fails → capacity opens despite not being a success.
	orch.MarkTaskFailed(1)
	require.Equal(t, WaveStateRunning, orch.State(), "wave stays running while pending tasks exist")

	pending, err := orch.StartPendingTasks(1)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, 2, pending[0].Number)
}

// TestStartNextWaveLimited_DoesNotAdvanceToNextWaveWhilePending verifies that
// completing all running tasks is not enough to declare the wave complete when
// pending tasks remain.
func TestStartNextWaveLimited_DoesNotAdvanceToNextWaveWhilePending(t *testing.T) {
	plan := testWavePlan(
		[]taskparser.Task{
			{Number: 1, Title: "T1"},
			{Number: 2, Title: "T2"},
		},
		[]taskparser.Task{{Number: 3, Title: "T3"}},
	)
	orch := NewWaveOrchestrator("plan", plan)
	_, err := orch.StartNextWaveLimited(1)
	require.NoError(t, err)

	// Complete the only running task — pending T2 means wave is not done.
	orch.MarkTaskComplete(1)
	assert.Equal(t, WaveStateRunning, orch.State(), "wave must not advance while pending tasks remain")
	assert.Equal(t, 1, orch.CurrentWaveNumber(), "must still be on wave 1")
}

// TestActiveTaskCount tracks running count across state transitions.
func TestActiveTaskCount(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "T1"},
		{Number: 2, Title: "T2"},
	})
	orch := NewWaveOrchestrator("plan", plan)
	assert.Equal(t, 0, orch.ActiveTaskCount())

	orch.StartNextWave()
	assert.Equal(t, 2, orch.ActiveTaskCount())

	orch.MarkTaskComplete(1)
	assert.Equal(t, 1, orch.ActiveTaskCount())

	orch.MarkTaskFailed(2)
	assert.Equal(t, 0, orch.ActiveTaskCount())
}

// TestApplyParallelismLimit_TrimsRunningToLimit verifies that excess running tasks
// are moved back to pending so only `limit` tasks remain active.
func TestApplyParallelismLimit_TrimsRunningToLimit(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "T1"},
		{Number: 2, Title: "T2"},
		{Number: 3, Title: "T3"},
	})
	orch := NewWaveOrchestrator("plan", plan)
	orch.StartNextWave() // marks all three as running

	assert.Equal(t, 3, orch.ActiveTaskCount())

	launched := orch.ApplyParallelismLimit(1)
	require.Len(t, launched, 1)
	assert.Equal(t, 1, launched[0].Number)
	assert.Equal(t, 1, orch.ActiveTaskCount())
	assert.True(t, orch.IsTaskRunning(1))
	// T2 and T3 should be pending — not running, not complete, not failed.
	assert.False(t, orch.IsTaskRunning(2))
	assert.False(t, orch.IsTaskComplete(2))
	assert.False(t, orch.IsTaskFailed(2))
}

// TestApplyParallelismLimit_NoOpWhenZeroLimit returns all running tasks unchanged.
func TestApplyParallelismLimit_NoOpWhenZeroLimit(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{
		{Number: 1, Title: "T1"},
		{Number: 2, Title: "T2"},
	})
	orch := NewWaveOrchestrator("plan", plan)
	orch.StartNextWave()

	launched := orch.ApplyParallelismLimit(0)
	assert.Len(t, launched, 2)
	assert.Equal(t, 2, orch.ActiveTaskCount())
}

func TestApplyParallelismLimit_NoOpWhenAllCompleteAndWaveExhausted(t *testing.T) {
	plan := testWavePlan([]taskparser.Task{{Number: 1, Title: "T1"}})
	orch := NewWaveOrchestrator("plan", plan)
	orch.state = WaveStateAllComplete
	orch.currentWave = len(plan.Waves)

	require.NotPanics(t, func() {
		launched := orch.ApplyParallelismLimit(1)
		assert.Empty(t, launched)
	})
}
