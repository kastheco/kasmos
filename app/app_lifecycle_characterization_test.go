package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLifecycleCharacterizationHome(t *testing.T, planFile string, status taskstate.Status) (*home, string) {
	t.Helper()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store, ps, fsm := newSharedStoreForTest(t, plansDir)
	require.NoError(t, ps.Register(planFile, "characterization test", "plan/"+taskstate.DisplayName(planFile), time.Now()))
	seedPlanStatus(t, ps, planFile, status)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             config.DefaultConfig(),
		nav:                   ui.NewNavigationPanel(&sp),
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		overlays:              overlay.NewManager(),
		taskState:             ps,
		taskStore:             store,
		taskStoreProject:      "test",
		taskStateDir:          plansDir,
		fsm:                   fsm,
		pendingReviewFeedback: make(map[string]string),
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
		waveOrchestrators:     make(map[string]*orchestration.WaveOrchestrator),
		instanceFinalizers:    make(map[*session.Instance]func()),
		activeRepoPath:        dir,
		program:               "claude",
	}
	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))
	return h, plansDir
}

func TestFSMSetImplementing_Characterization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		initial  taskstate.Status
		expected taskstate.Status
		planFile string
	}{
		{name: "planning walks through ready", initial: taskstate.StatusPlanning, expected: taskstate.StatusImplementing, planFile: "planning-flow"},
		{name: "ready starts implementing", initial: taskstate.StatusReady, expected: taskstate.StatusImplementing, planFile: "ready-flow"},
		{name: "implementing is stable noop", initial: taskstate.StatusImplementing, expected: taskstate.StatusImplementing, planFile: "already-implementing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, plansDir := newLifecycleCharacterizationHome(t, tt.planFile, tt.initial)
			// planned-ready is required for fsmSetImplementing to accept a ready task;
			// planning and implementing cases handle the phase transition themselves.
			if tt.initial == taskstate.StatusReady {
				require.NoError(t, h.taskState.SetExecutionState(tt.planFile, taskstore.ExecutionState{Phase: "planned"}))
			}

			require.NoError(t, h.fsmSetImplementing(tt.planFile))

			reloaded, err := newTestPlanStateWithStore(t, h.taskStore, plansDir)
			require.NoError(t, err)
			entry, ok := reloaded.Entry(tt.planFile)
			require.True(t, ok)
			assert.Equal(t, tt.expected, entry.Status)
		})
	}
}

func TestFSMSetReviewing_Characterization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		initial  taskstate.Status
		expected taskstate.Status
		planFile string
	}{
		{name: "implementing walks to reviewing", initial: taskstate.StatusImplementing, expected: taskstate.StatusReviewing, planFile: "implementing-to-review"},
		{name: "planning walks all forward stages", initial: taskstate.StatusPlanning, expected: taskstate.StatusReviewing, planFile: "planning-to-review"},
		{name: "reviewing is stable noop", initial: taskstate.StatusReviewing, expected: taskstate.StatusReviewing, planFile: "already-reviewing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, plansDir := newLifecycleCharacterizationHome(t, tt.planFile, tt.initial)

			require.NoError(t, h.fsmSetReviewing(tt.planFile))

			reloaded, err := newTestPlanStateWithStore(t, h.taskStore, plansDir)
			require.NoError(t, err)
			entry, ok := reloaded.Entry(tt.planFile)
			require.True(t, ok)
			assert.Equal(t, tt.expected, entry.Status)
		})
	}
}

func TestExecuteContextAction_StartFixerFromReviewing_Characterization(t *testing.T) {
	t.Parallel()

	const planFile = "manual-fixer"
	const feedback = "preserve the current manual fixer branch"

	h, plansDir := newLifecycleCharacterizationHome(t, planFile, taskstate.StatusReviewing)
	require.NoError(t, h.taskState.SetLatestReviewFeedback(planFile, feedback))

	model, cmd := h.executeContextAction("start_fixer")
	updated := model.(*home)
	require.NotNil(t, cmd, "manual fixer start should still return the fixer startup cmd")

	reloaded, err := newTestPlanStateWithStore(t, updated.taskStore, plansDir)
	require.NoError(t, err)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusImplementing, entry.Status)
	assert.Equal(t, 1, entry.ReviewCycle)

	var fixer *session.Instance
	for _, inst := range updated.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.AgentType == session.AgentTypeFixer {
			fixer = inst
			break
		}
	}
	require.NotNil(t, fixer, "manual fixer start should still spawn a fixer instance")
	assert.Contains(t, fixer.QueuedPrompt, feedback)
	assert.Equal(t, 1, fixer.ReviewCycle)
}
