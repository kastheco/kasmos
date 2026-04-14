package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteTaskTransition_LifecycleMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		initial     taskstore.Status
		event       string
		wantStatus  string
		wantErrText string
	}{
		{name: "plan start", initial: taskstore.StatusReady, event: "plan_start", wantStatus: "planning"},
		{name: "planner finished", initial: taskstore.StatusPlanning, event: "planner_finished", wantStatus: "ready"},
		{name: "implement finished", initial: taskstore.StatusImplementing, event: "implement_finished", wantStatus: "reviewing"},
		{name: "review approved", initial: taskstore.StatusReviewing, event: "review_approved", wantStatus: "verifying"},
		{name: "verify approved", initial: taskstore.StatusVerifying, event: "verify_approved", wantStatus: "done"},
		{name: "verify failed", initial: taskstore.StatusVerifying, event: "verify_failed", wantStatus: "implementing"},
		{name: "review changes alias", initial: taskstore.StatusReviewing, event: "review_changes", wantStatus: "implementing"},
		{name: "review changes requested canonical alias", initial: taskstore.StatusReviewing, event: "review_changes_requested", wantStatus: "implementing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := taskstore.NewTestSQLiteStore(t)
			const project = "proj"
			require.NoError(t, store.Create(project, taskstore.TaskEntry{
				Filename: "feature",
				Status:   tt.initial,
			}))

			status, err := executeTaskTransition(project, "feature", tt.event, store)
			if tt.wantErrText != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrText)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

func TestExecuteTaskImplement_WalksPlanningBeforeWritingWaveSignal(t *testing.T) {
	t.Parallel()

	store := taskstore.NewTestSQLiteStore(t)
	repoRoot := t.TempDir()
	const project = "proj"
	const planFile = "planning-plan"
	signalsDir := filepath.Join(repoRoot, ".kasmos", "signals")
	require.NoError(t, os.MkdirAll(signalsDir, 0o755))
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusPlanning,
	}))

	require.NoError(t, executeTaskImplement(repoRoot, project, planFile, 2, store))

	ps, err := taskstate.Load(store, project, "")
	require.NoError(t, err)
	entry, ok := ps.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusImplementing, entry.Status)

	_, err = os.Stat(filepath.Join(signalsDir, "implement-wave-2-"+planFile))
	assert.NoError(t, err)
}
