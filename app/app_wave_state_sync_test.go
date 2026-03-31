package app

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const daemonWaveStateSyncPlanContent = `# Plan

**Goal:** keep sidebar, header, and info pane wave state aligned

**Architecture:** daemon-managed task state is the authoritative plan snapshot

**Tech Stack:** Go, Bubble Tea, SQLite

## Wave 1

### Task 1: sync sidebar
keep sidebar wave labels current

### Task 2: sync status bar
keep header wave labels current

## Wave 2

### Task 3: sync info pane
keep info-pane execution metadata current
`

type waveSyncOrchestratorPosition string

const (
	waveSyncPositionWave1Running waveSyncOrchestratorPosition = "wave1_running"
	waveSyncPositionWave1Waiting waveSyncOrchestratorPosition = "wave1_waiting"
	waveSyncPositionWave2Running waveSyncOrchestratorPosition = "wave2_running"
)

type daemonWaveSurfaceExpectation struct {
	planStatus       string
	waveLabel        string
	wantTaskGlyphs   bool
	phase            string
	agentType        string
	activeWave       int
	activeRound      int
	wantWaveTaskRows bool
	sidebarText      string
}

func newDaemonWaveStateSyncHome(t *testing.T) (*home, string) {
	t.Helper()

	const planFile = "feature"
	h, _ := newDaemonSyncTestHome(t, planFile)
	require.NoError(t, h.taskState.IngestContent(planFile, daemonWaveStateSyncPlanContent))
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))
	h.updateInfoPane()

	return h, planFile
}

func newDaemonWaveStateSyncOrchestrator(t *testing.T, planFile string, position waveSyncOrchestratorPosition) *orchestration.WaveOrchestrator {
	t.Helper()

	plan, err := taskparser.Parse(daemonWaveStateSyncPlanContent)
	require.NoError(t, err)

	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	switch position {
	case waveSyncPositionWave1Running:
		orch.StartNextWave()
	case waveSyncPositionWave1Waiting:
		orch.StartNextWave()
		orch.MarkTaskComplete(1)
		orch.MarkTaskComplete(2)
	case waveSyncPositionWave2Running:
		orch.StartNextWave()
		orch.MarkTaskComplete(1)
		orch.MarkTaskComplete(2)
		orch.StartNextWave()
	default:
		t.Fatalf("unknown orchestrator position %q", position)
	}

	return orch
}

func daemonWaveStatePlanState(t *testing.T, h *home, planFile string, status taskstate.Status, state taskstore.ExecutionState, reviewCycle int) *taskstate.TaskState {
	t.Helper()

	require.NoError(t, h.taskState.ForceSetLifecycle(planFile, status, state))
	currentReviewCycle, err := h.taskState.ReviewCycle(planFile)
	require.NoError(t, err)
	for currentReviewCycle < reviewCycle {
		require.NoError(t, h.taskState.IncrementReviewCycle(planFile))
		currentReviewCycle++
	}

	ps, err := taskstate.Load(h.taskStore, h.taskStoreProject, h.taskStateDir)
	require.NoError(t, err)
	return ps
}

func ensureWaveTaskInstances(t *testing.T, h *home, planFile string, orch *orchestration.WaveOrchestrator) {
	t.Helper()

	existing := make(map[string]bool, len(h.nav.GetInstances()))
	for _, inst := range h.nav.GetInstances() {
		existing[inst.Title] = true
	}

	planName := taskstate.DisplayName(planFile)
	for _, task := range orch.CurrentWaveTasks() {
		title := orchestration.BuildWaveTaskTitle(planName, orch.CurrentWaveNumber(), task.Number)
		if existing[title] {
			continue
		}
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:      title,
			Path:       h.activeRepoPath,
			Program:    h.program,
			TaskFile:   planFile,
			AgentType:  session.AgentTypeCoder,
			TaskNumber: task.Number,
			WaveNumber: orch.CurrentWaveNumber(),
		})
		require.NoError(t, err)
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		h.nav.AddInstance(inst)
	}
}

func applyDaemonMetadata(t *testing.T, h *home, msg metadataResultMsg) *home {
	t.Helper()

	model, _ := h.Update(msg)
	updated := model.(*home)
	updated.updateInfoPane()
	return updated
}

func assertDaemonWaveSurfaceState(t *testing.T, updated *home, planFile string, want daemonWaveSurfaceExpectation) {
	t.Helper()

	status := updated.computeStatusBarData()
	assert.Equal(t, want.planStatus, status.PlanStatus)
	assert.Equal(t, want.waveLabel, status.WaveLabel)
	if want.wantTaskGlyphs {
		assert.NotEmpty(t, status.TaskGlyphs)
	} else {
		assert.Empty(t, status.TaskGlyphs)
	}

	planDisplay := navPlanDisplayForTest(t, updated.nav, planFile)
	assert.Equal(t, want.phase, planDisplay.Phase)
	assert.Equal(t, want.agentType, planDisplay.AgentType)
	assert.Equal(t, want.activeWave, planDisplay.ActiveWave)
	assert.Equal(t, want.activeRound, planDisplay.ActiveRound)

	info := updated.tabbedWindow.GetInfoData()
	assert.Equal(t, want.phase, info.ExecutionPhase)
	assert.Equal(t, want.agentType, info.ActiveAgentType)
	assert.Equal(t, want.activeWave, info.ActiveWave)
	assert.Equal(t, want.activeRound, info.ActiveRound)
	if want.wantWaveTaskRows {
		assert.NotEmpty(t, info.WaveTasks)
	} else {
		assert.Empty(t, info.WaveTasks)
	}

	if want.sidebarText != "" {
		assert.Contains(t, updated.nav.String(), want.sidebarText)
	}
}

func TestDaemonWaveState_StatusBarSidebarInfoPaneStayInSync(t *testing.T) {
	h, planFile := newDaemonWaveStateSyncHome(t)

	steps := []struct {
		name                 string
		status               taskstate.Status
		executionState       taskstore.ExecutionState
		reviewCycle          int
		orchestratorPosition waveSyncOrchestratorPosition
		want                 daemonWaveSurfaceExpectation
	}{
		{
			name:                 "wave_running_on_wave_1",
			status:               taskstate.StatusImplementing,
			executionState:       taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveRunning), ActiveAgentType: session.AgentTypeCoder, ActiveWave: 1},
			orchestratorPosition: waveSyncPositionWave1Running,
			want: daemonWaveSurfaceExpectation{
				planStatus:       string(taskstate.StatusImplementing),
				waveLabel:        "wave 1/2",
				wantTaskGlyphs:   true,
				phase:            string(taskfsm.ExecutionPhaseWaveRunning),
				agentType:        session.AgentTypeCoder,
				activeWave:       1,
				wantWaveTaskRows: true,
				sidebarText:      "wave 1 running",
			},
		},
		{
			name:                 "wave_waiting_on_wave_1",
			status:               taskstate.StatusImplementing,
			executionState:       taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveWaiting), ActiveAgentType: session.AgentTypeCoder, ActiveWave: 1},
			orchestratorPosition: waveSyncPositionWave1Waiting,
			want: daemonWaveSurfaceExpectation{
				planStatus:       string(taskstate.StatusImplementing),
				waveLabel:        "wave 1/2",
				wantTaskGlyphs:   true,
				phase:            string(taskfsm.ExecutionPhaseWaveWaiting),
				agentType:        session.AgentTypeCoder,
				activeWave:       1,
				wantWaveTaskRows: true,
				sidebarText:      "waiting for confirmation",
			},
		},
		{
			name:                 "wave_running_on_wave_2_after_later_metadata_update",
			status:               taskstate.StatusImplementing,
			executionState:       taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveRunning), ActiveAgentType: session.AgentTypeCoder, ActiveWave: 2},
			orchestratorPosition: waveSyncPositionWave2Running,
			want: daemonWaveSurfaceExpectation{
				planStatus:       string(taskstate.StatusImplementing),
				waveLabel:        "wave 2/2",
				wantTaskGlyphs:   true,
				phase:            string(taskfsm.ExecutionPhaseWaveRunning),
				agentType:        session.AgentTypeCoder,
				activeWave:       2,
				wantWaveTaskRows: true,
				sidebarText:      "wave 2 running",
			},
		},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			h.waveOrchestrators[planFile] = newDaemonWaveStateSyncOrchestrator(t, planFile, step.orchestratorPosition)
			ensureWaveTaskInstances(t, h, planFile, h.waveOrchestrators[planFile])
			msg := metadataResultMsg{
				PlanState:         daemonWaveStatePlanState(t, h, planFile, step.status, step.executionState, step.reviewCycle),
				DaemonTaskState:   true,
				DaemonManagedRepo: true,
			}

			updated := applyDaemonMetadata(t, h, msg)
			assertDaemonWaveSurfaceState(t, updated, planFile, step.want)
			h = updated
		})
	}
}

func TestDaemonWaveState_StaleOrchestratorDoesNotOverrideDaemonSnapshot(t *testing.T) {
	h, planFile := newDaemonWaveStateSyncHome(t)
	h.waveOrchestrators[planFile] = newDaemonWaveStateSyncOrchestrator(t, planFile, waveSyncPositionWave1Running)
	ensureWaveTaskInstances(t, h, planFile, h.waveOrchestrators[planFile])

	msg := metadataResultMsg{
		PlanState: daemonWaveStatePlanState(
			t,
			h,
			planFile,
			taskstate.StatusImplementing,
			taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveRunning), ActiveAgentType: session.AgentTypeCoder, ActiveWave: 2},
			0,
		),
		DaemonTaskState:   true,
		DaemonManagedRepo: true,
	}

	updated := applyDaemonMetadata(t, h, msg)
	assertDaemonWaveSurfaceState(t, updated, planFile, daemonWaveSurfaceExpectation{
		planStatus:       string(taskstate.StatusImplementing),
		waveLabel:        "wave 2/2",
		wantTaskGlyphs:   false,
		phase:            string(taskfsm.ExecutionPhaseWaveRunning),
		agentType:        session.AgentTypeCoder,
		activeWave:       2,
		wantWaveTaskRows: false,
		sidebarText:      "wave 2 running",
	})

	updated.waveOrchestrators[planFile] = newDaemonWaveStateSyncOrchestrator(t, planFile, waveSyncPositionWave2Running)
	ensureWaveTaskInstances(t, updated, planFile, updated.waveOrchestrators[planFile])
	updated = applyDaemonMetadata(t, updated, msg)
	assertDaemonWaveSurfaceState(t, updated, planFile, daemonWaveSurfaceExpectation{
		planStatus:       string(taskstate.StatusImplementing),
		waveLabel:        "wave 2/2",
		wantTaskGlyphs:   true,
		phase:            string(taskfsm.ExecutionPhaseWaveRunning),
		agentType:        session.AgentTypeCoder,
		activeWave:       2,
		wantWaveTaskRows: true,
	})
}
