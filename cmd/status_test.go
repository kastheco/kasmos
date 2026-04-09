package cmd

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteStatus_HappyPath(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "test-project"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    "feature-a",
		Status:      taskstore.Status("implementing"),
		Description: "feature a",
		Branch:      "plan/feature-a",
		CreatedAt:   time.Now(),
	}))
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    "feature-b",
		Status:      taskstore.StatusReady,
		Description: "feature b",
		Branch:      "plan/feature-b",
		CreatedAt:   time.Now(),
	}))

	state := newTestStateFromRaw(t, []instanceTestData{
		{Title: "coder-1", Status: 0, Branch: "plan/feature-a", Program: "claude", TaskFile: "feature-a", AgentType: "coder"},
		{Title: "solo-agent", Status: 3, Branch: "main", Program: "claude"},
	})

	epoch := int64(1741084800)
	tmuxOutput := strings.Join([]string{
		fakeTmuxLine("kas_coder-1", epoch, 1, false, 200, 50),
		fakeTmuxLine("kas_orphan-sess", epoch, 1, false, 200, 50),
	}, "\n")
	ex := cmd_test.NewMockExecutor()
	ex.OutputFunc = func(_ *exec.Cmd) ([]byte, error) {
		return []byte(tmuxOutput), nil
	}

	output := executeStatus(state, store, project, ex, "text")

	// tasks section
	assert.Contains(t, output, "tasks:")
	assert.Contains(t, output, "feature-a")
	assert.Contains(t, output, "implementing")
	// instances section
	assert.Contains(t, output, "instances:")
	assert.Contains(t, output, "coder-1")
	assert.Contains(t, output, "running")
	// orphan section
	assert.Contains(t, output, "orphan tmux sessions:")
	assert.Contains(t, output, "kas_orphan-sess")
	// hints: ready task → implement hint, paused instance → resume hint, orphan → tmux hints
	assert.Contains(t, output, "hints:")
	assert.Contains(t, output, "kas task implement <task-name>")
	assert.Contains(t, output, "kas instance resume <title>")
	assert.Contains(t, output, "kas tmux adopt <session> <title>")
	assert.Contains(t, output, "kas tmux kill <session>")
}

func TestExecuteStatus_Empty(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "empty-project"
	state := newTestStateFromRaw(t, []instanceTestData{})
	ex := cmd_test.NewMockExecutor()
	ex.OutputFunc = func(_ *exec.Cmd) ([]byte, error) {
		return nil, errors.New("no tmux")
	}

	output := executeStatus(state, store, project, ex, "text")
	assert.Contains(t, output, "no active tasks")
	assert.Contains(t, output, "no instances")
	assert.Contains(t, output, "no orphan tmux sessions")
}

func TestExecuteStatus_JSON(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "json-project"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:  "plan-x",
		Status:    taskstore.StatusReady,
		Branch:    "plan/plan-x",
		CreatedAt: time.Now(),
	}))
	state := newTestStateFromRaw(t, []instanceTestData{
		{Title: "agent-1", Status: 0, Branch: "plan/plan-x", Program: "claude"},
	})
	ex := cmd_test.NewMockExecutor()
	ex.OutputFunc = func(_ *exec.Cmd) ([]byte, error) {
		return nil, errors.New("no tmux")
	}

	output := executeStatus(state, store, project, ex, "json")
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err)
	assert.Contains(t, parsed, "tasks")
	assert.Contains(t, parsed, "instances")
	assert.Contains(t, parsed, "orphan_sessions")
}

func TestExecuteStatus_ShowsLifecycleStageDetailsAndRecoveryHints(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "lifecycle-project"

	entries := []taskstore.TaskEntry{
		{
			Filename:  "planned-plan",
			Status:    taskstore.StatusReady,
			Branch:    "plan/planned-plan",
			CreatedAt: time.Now(),
			ExecutionState: taskstore.ExecutionState{
				Phase: "planned",
			},
		},
		{
			Filename:  "architect-plan",
			Status:    taskstore.StatusImplementing,
			Branch:    "plan/architect-plan",
			CreatedAt: time.Now(),
			ExecutionState: taskstore.ExecutionState{
				Phase:           "architecting",
				ActiveAgentType: "architect",
			},
		},
		{
			Filename:  "wave-running-plan",
			Status:    taskstore.StatusImplementing,
			Branch:    "plan/wave-running-plan",
			CreatedAt: time.Now(),
			ExecutionState: taskstore.ExecutionState{
				Phase:           "wave_running",
				ActiveAgentType: "coder",
				ActiveWave:      2,
			},
		},
		{
			Filename:  "wave-waiting-plan",
			Status:    taskstore.StatusImplementing,
			Branch:    "plan/wave-waiting-plan",
			CreatedAt: time.Now(),
			ExecutionState: taskstore.ExecutionState{
				Phase:           "wave_waiting",
				ActiveAgentType: "coder",
				ActiveWave:      3,
			},
		},
		{
			Filename:             "fixing-plan",
			Status:               taskstore.StatusImplementing,
			Branch:               "plan/fixing-plan",
			CreatedAt:            time.Now(),
			ReviewCycle:          2,
			LatestReviewFeedback: "fix flaky tests",
			ExecutionState: taskstore.ExecutionState{
				Phase:           "fixing",
				ActiveAgentType: "fixer",
			},
		},
		{
			Filename:             "reviewing-plan",
			Status:               taskstore.StatusReviewing,
			Branch:               "plan/reviewing-plan",
			CreatedAt:            time.Now(),
			ReviewCycle:          2,
			LatestReviewFeedback: "round 2",
			ExecutionState: taskstore.ExecutionState{
				Phase:           "reviewing",
				ActiveAgentType: "reviewer",
			},
		},
	}
	for _, entry := range entries {
		require.NoError(t, store.Create(project, entry))
	}

	state := newTestStateFromRaw(t, nil)
	ex := cmd_test.NewMockExecutor()
	ex.OutputFunc = func(_ *exec.Cmd) ([]byte, error) {
		return nil, errors.New("no tmux")
	}

	output := executeStatus(state, store, project, ex, "text")
	assert.Contains(t, output, "planned")
	assert.Contains(t, output, "architecting")
	assert.Contains(t, output, "wave 2 running")
	assert.Contains(t, output, "waiting for confirmation")
	assert.Contains(t, output, "fixing round 3")
	assert.Contains(t, output, "reviewing round 3")
	assert.Contains(t, output, "kas task recover <task-name> --action architect-finished")
	assert.Contains(t, output, "kas task recover <task-name> --action implement-finished")
	assert.Contains(t, output, "kas task recover <task-name> --action review-changes --feedback")
	assert.Contains(t, output, "kas task recover <task-name> --action advance-review-cycle --feedback")
	assert.Contains(t, output, "yes")
	assert.Contains(t, output, "wave-waiting-plan")
}

func TestExecuteStatus_JSONUsesOperatorLifecycleLabels(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "json-lifecycle-project"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:  "wave-plan",
		Status:    taskstore.StatusImplementing,
		Branch:    "plan/wave-plan",
		CreatedAt: time.Now(),
		ExecutionState: taskstore.ExecutionState{
			Phase:           "wave_running",
			ActiveAgentType: "coder",
			ActiveWave:      4,
		},
	}))

	state := newTestStateFromRaw(t, nil)
	ex := cmd_test.NewMockExecutor()
	ex.OutputFunc = func(_ *exec.Cmd) ([]byte, error) {
		return nil, errors.New("no tmux")
	}

	output := executeStatus(state, store, project, ex, "json")
	var parsed statusData
	require.NoError(t, json.Unmarshal([]byte(output), &parsed))
	require.Len(t, parsed.Tasks, 1)
	assert.Equal(t, "wave 4 running", parsed.Tasks[0].Stage)
	assert.Equal(t, "coder", parsed.Tasks[0].ActiveAgentType)
	assert.Equal(t, 4, parsed.Tasks[0].ActiveWave)
}

func TestExecuteStatus_VerifyingPhase(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "verifying-project"

	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    "master-check",
		Status:      taskstore.StatusVerifying,
		Branch:      "plan/master-check",
		CreatedAt:   time.Now(),
		ReviewCycle: 1,
		ExecutionState: taskstore.ExecutionState{
			ActiveAgentType: "master",
		},
	}))

	state := newTestStateFromRaw(t, nil)
	ex := cmd_test.NewMockExecutor()
	ex.OutputFunc = func(_ *exec.Cmd) ([]byte, error) {
		return nil, errors.New("no tmux")
	}

	t.Run("text output shows verifying status without round counter", func(t *testing.T) {
		output := executeStatus(state, store, project, ex, "text")
		assert.Contains(t, output, "verifying", "status label must be verifying")
		// review round must not be shown for verifying state
		assert.NotContains(t, output, "round", "round counter must not appear for verifying state")
	})

	t.Run("json output has correct stage and no review cycle", func(t *testing.T) {
		output := executeStatus(state, store, project, ex, "json")
		var parsed statusData
		require.NoError(t, json.Unmarshal([]byte(output), &parsed))
		require.Len(t, parsed.Tasks, 1)
		task := parsed.Tasks[0]
		assert.Equal(t, "verifying", task.Stage)
		assert.Equal(t, "master", task.ActiveAgentType)
		assert.Equal(t, 0, task.ReviewCycle, "review cycle must be 0 for verifying state")
	})

	t.Run("recovery hints show verify-approved and verify-failed", func(t *testing.T) {
		output := executeStatus(state, store, project, ex, "text")
		assert.Contains(t, output, "kas task recover <task-name> --action verify-approved")
		assert.Contains(t, output, "kas task recover <task-name> --action verify-failed --feedback")
		// standard review hints must not appear for verifying state
		assert.NotContains(t, output, "--action review-approved")
		assert.NotContains(t, output, "--action review-changes")
	})
}

func TestExecuteStatus_NilStore(t *testing.T) {
	state := newTestStateFromRaw(t, []instanceTestData{
		{Title: "solo", Status: 0, Branch: "main", Program: "claude"},
	})
	ex := cmd_test.NewMockExecutor()
	ex.OutputFunc = func(_ *exec.Cmd) ([]byte, error) {
		return nil, errors.New("no tmux")
	}

	output := executeStatus(state, nil, "test-project", ex, "text")
	assert.Contains(t, output, "no active tasks")
	assert.Contains(t, output, "solo")
}
