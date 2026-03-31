package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
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

const daemonWaveSidebarDriftPlanContent = `# Plan

**Goal:** keep daemon-managed wave state consistent across tui surfaces

**Architecture:** task-state snapshots drive sidebar, status bar, and info pane

**Tech Stack:** Go, Bubble Tea, SQLite

## Wave 1

### Task 1: prepare state refresh
seed the initial wave state

## Wave 2

### Task 2: apply daemon metadata
advance the authoritative snapshot

## Wave 3

### Task 3: render consistent wave state
show the final advanced wave everywhere
`

func newWaveSidebarDriftTestHome(t *testing.T) (*home, string) {
	t.Helper()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store, ps, fsm := newSharedStoreForTest(t, plansDir)

	const planFile = "feature"
	require.NoError(t, ps.Register(planFile, "wave drift regression", "plan/feature", time.Now()))
	require.NoError(t, ps.IngestContent(planFile, daemonWaveSidebarDriftPlanContent))
	require.NoError(t, ps.ForceSetLifecycle(planFile, taskstate.StatusImplementing, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	}))

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
		program:               "opencode",
	}
	h.nav.SetSize(80, 40)
	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))
	h.updateInfoPane()

	return h, planFile
}

func newWaveSidebarDriftOrchestrator(t *testing.T, planFile string) *orchestration.WaveOrchestrator {
	t.Helper()

	plan, err := taskparser.Parse(daemonWaveSidebarDriftPlanContent)
	require.NoError(t, err)

	return orchestration.NewWaveOrchestrator(planFile, plan)
}

func daemonWaveSidebarDriftPlanState(
	t *testing.T,
	h *home,
	planFile string,
	executionState taskstore.ExecutionState,
) *taskstate.TaskState {
	t.Helper()

	ps, err := taskstate.Load(h.taskStore, h.taskStoreProject, h.taskStateDir)
	require.NoError(t, err)
	require.NoError(t, ps.ForceSetLifecycle(planFile, taskstate.StatusImplementing, executionState))

	return ps
}

func TestDaemonMetadataRefresh_UpdatesSidebarStatusBarAndInfoPaneToAdvancedWave(t *testing.T) {
	h, planFile := newWaveSidebarDriftTestHome(t)

	staleDisplay := navPlanDisplayForTest(t, h.nav, planFile)
	assert.Equal(t, 1, staleDisplay.ActiveWave)

	orch := newWaveSidebarDriftOrchestrator(t, planFile)
	orch.RestoreToWave(3, nil)
	h.waveOrchestrators[planFile] = orch

	msg := metadataResultMsg{
		PlanState: daemonWaveSidebarDriftPlanState(t, h, planFile, taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
			ActiveAgentType: session.AgentTypeCoder,
			ActiveWave:      3,
		}),
		DaemonTaskState:   true,
		DaemonManagedRepo: true,
	}

	model, _ := h.Update(msg)
	updated := model.(*home)

	require.True(t, updated.nav.SelectByID(ui.SidebarPlanPrefix+planFile))
	assert.Equal(t, 3, navPlanDisplayForTest(t, updated.nav, planFile).ActiveWave)

	status := updated.computeStatusBarData()
	assert.Contains(t, status.WaveLabel, "wave 3")

	updated.updateInfoPane()
	assert.Equal(t, 3, updated.tabbedWindow.GetInfoData().ActiveWave)
}

func TestComputeStatusBarData_StaleOrchestratorSuppressesGlyphsInsteadOfOverridingWave(t *testing.T) {
	h, planFile := newWaveSidebarDriftTestHome(t)

	staleDisplay := navPlanDisplayForTest(t, h.nav, planFile)
	assert.Equal(t, 1, staleDisplay.ActiveWave)

	staleOrch := newWaveSidebarDriftOrchestrator(t, planFile)
	staleOrch.StartNextWave()
	h.waveOrchestrators[planFile] = staleOrch

	msg := metadataResultMsg{
		PlanState: daemonWaveSidebarDriftPlanState(t, h, planFile, taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
			ActiveAgentType: session.AgentTypeCoder,
			ActiveWave:      3,
		}),
		DaemonTaskState:   true,
		DaemonManagedRepo: true,
	}

	model, _ := h.Update(msg)
	updated := model.(*home)

	require.True(t, updated.nav.SelectByID(ui.SidebarPlanPrefix+planFile))
	assert.Equal(t, 3, navPlanDisplayForTest(t, updated.nav, planFile).ActiveWave)

	status := updated.computeStatusBarData()
	assert.Contains(t, status.WaveLabel, "wave 3")
	assert.Empty(t, status.TaskGlyphs)

	updated.updateInfoPane()
	assert.Equal(t, 3, updated.tabbedWindow.GetInfoData().ActiveWave)

	updated.waveOrchestrators[planFile].MarkTaskComplete(1)
	updated.waveOrchestrators[planFile].StartNextWave()
	updated.waveOrchestrators[planFile].MarkTaskComplete(2)
	updated.waveOrchestrators[planFile].StartNextWave()

	resyncedStatus := updated.computeStatusBarData()
	assert.Contains(t, resyncedStatus.WaveLabel, "wave 3")
	assert.NotEmpty(t, resyncedStatus.TaskGlyphs)
}
