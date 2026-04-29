package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plannerSignalHome builds a minimal home with a planner instance registered
// in StatusPlanning (so the FSM transition PlannerFinished → StatusReady succeeds).
func plannerSignalHome(t *testing.T, planFile string) (*home, *taskstate.TaskState, string, *session.Instance) {
	t.Helper()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store, ps, fsm := newSharedStoreForTest(t, plansDir)
	require.NoError(t, ps.Register(planFile, "test plan", "plan/test", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusPlanning)

	plannerInst, err := session.NewInstance(session.InstanceOptions{
		Title:     "test-plan-planner",
		Path:      dir,
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypePlanner,
	})
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(plannerInst)

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             config.DefaultConfig(),
		nav:                   list,
		allInstances:          []*session.Instance{plannerInst},
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		overlays:              overlay.NewManager(),
		taskState:             ps,
		taskStateDir:          plansDir,
		taskStore:             store,
		taskStoreProject:      "test",
		fsm:                   fsm,
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
		pendingReviewFeedback: make(map[string]string),
		waveOrchestrators:     make(map[string]*orchestration.WaveOrchestrator),
		instanceFinalizers:    make(map[*session.Instance]func()),
		activeRepoPath:        dir,
		program:               "claude",
	}

	return h, ps, plansDir, plannerInst
}

// TestPlannerFinishedSignal_ShowsConfirmDialog verifies that when a PlannerFinished
// signal is processed, the app enters stateConfirm with a confirmation overlay
// and pendingPlannerTaskFile is set.
func TestPlannerFinishedSignal_ShowsConfirmDialog(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, ps, _, _ := plannerSignalHome(t, planFile)

	// Explicitly disable auto-advance so this test exercises the dialog path.
	h.appConfig.AutoAdvance = false

	signal := taskfsm.Signal{
		Event:    taskfsm.PlannerFinished,
		TaskFile: planFile,
	}
	msg := metadataResultMsg{
		PlanState: ps,
		Signals:   []taskfsm.Signal{signal},
	}

	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.Equal(t, stateConfirm, updated.state,
		"PlannerFinished signal must set stateConfirm")
	assert.True(t, updated.overlays.IsActive(),
		"PlannerFinished signal must show confirmation overlay")
	assert.Equal(t, planFile, updated.pendingPlannerTaskFile,
		"pendingPlannerTaskFile must be set to the plan file from the signal")
}

func TestPlanStartDraftModeGatewayRespawnKillsStalePlanners(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, ps, _, oldPlanner := plannerSignalHome(t, planFile)
	seedPlanStatus(t, ps, planFile, taskstate.StatusReady)
	h.taskStateDir = ""
	h.appConfig = &config.Config{
		Planners: []string{"planner_a", "planner_b"},
		Profiles: map[string]config.AgentProfile{
			"planner_a": {Enabled: true, Program: "opencode"},
			"planner_b": {Enabled: true, Program: "opencode"},
		},
	}

	model, _ := h.Update(metadataResultMsg{
		PlanState: ps,
		Signals: []taskfsm.Signal{
			{Event: taskfsm.PlanStart, TaskFile: planFile},
		},
	})
	updated := model.(*home)

	var titles []string
	var profiles []string
	for _, inst := range updated.nav.GetInstances() {
		titles = append(titles, inst.Title)
		if inst.AgentType == session.AgentTypePlanner {
			profiles = append(profiles, inst.PlannerProfile)
		}
	}
	assert.NotContains(t, titles, oldPlanner.Title)
	assert.ElementsMatch(t, []string{"planner_a", "planner_b"}, profiles)

	for _, inst := range updated.allInstances {
		assert.NotEqual(t, oldPlanner.Title, inst.Title)
	}
}

func TestPlanStartDraftModeGatewaySpawnFailureCleansPartialFanout(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, ps, _, _ := plannerSignalHome(t, planFile)
	seedPlanStatus(t, ps, planFile, taskstate.StatusReady)
	h.taskStateDir = ""
	h.appConfig = &config.Config{
		Planners: []string{"planner_a", "missing_profile", "planner_c"},
		Profiles: map[string]config.AgentProfile{
			"planner_a": {Enabled: true, Program: "opencode"},
			"planner_c": {Enabled: true, Program: "opencode"},
		},
	}

	model, _ := h.Update(metadataResultMsg{
		PlanState: ps,
		Signals: []taskfsm.Signal{
			{Event: taskfsm.PlanStart, TaskFile: planFile},
		},
	})
	updated := model.(*home)

	for _, inst := range updated.nav.GetInstances() {
		assert.NotEqual(t, session.AgentTypePlanner, inst.AgentType)
	}
}

func TestMetadataTickProcessesPlanStartBeforePlannerDraftRows(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, ps, _, _ := plannerSignalHome(t, planFile)
	h.taskStateDir = ""
	h.appConfig = &config.Config{
		Planners: []string{"planner_a", "planner_b"},
		Profiles: map[string]config.AgentProfile{
			"planner_a": {Enabled: true, Program: "opencode"},
			"planner_b": {Enabled: true, Program: "opencode"},
		},
	}

	proc := h.ensureProcessor()
	require.NotNil(t, proc)
	require.Empty(t, proc.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: planFile, PlannerID: "planner_a"},
	}))
	require.NotEmpty(t, proc.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: planFile, PlannerID: "planner_b"},
	}))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReady)

	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })
	h.signalGateway = gw
	require.NoError(t, gw.Create("test", taskstore.SignalEntry{
		PlanFile:   planFile,
		SignalType: "plan_start",
	}))
	require.NoError(t, gw.Create("test", taskstore.SignalEntry{
		PlanFile:   planFile,
		SignalType: "planner_draft_finished",
		Payload:    `{"planner_id":"planner_a"}`,
	}))

	var scan loop.ScanResult
	claimed := make([]*taskstore.SignalEntry, 0, 2)
	for range 2 {
		entry, err := gw.Claim("test", "app-test")
		require.NoError(t, err)
		require.NotNil(t, entry)
		require.NoError(t, loop.ConvertSignalEntry(entry, &scan))
		claimed = append(claimed, entry)
	}
	require.Len(t, scan.FSMSignals, 1)
	require.Len(t, scan.PlannerDraftSignals, 1)

	model, _ := h.Update(metadataResultMsg{
		PlanState:            ps,
		Signals:              scan.FSMSignals,
		PlannerDraftSignals:  scan.PlannerDraftSignals,
		GatewaySignalEntries: claimed,
	})
	updated := model.(*home)

	done, err := gw.List("test", taskstore.SignalDone)
	require.NoError(t, err)
	require.Len(t, done, 2)

	actions := updated.processor.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: planFile, PlannerID: "planner_b"},
	})
	require.NotEmpty(t, actions, "same-tick planner_a draft must be recorded after plan_start resets stale aggregation")
	assert.Equal(t, "planner_complete", actions[0].Kind())
}

func TestGatewayAckClassifiesRowsIndividuallyForSamePlan(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, ps, _, _ := plannerSignalHome(t, planFile)
	h.appConfig.AutoAdvance = false

	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })
	h.signalGateway = gw

	require.NoError(t, gw.Create("test", taskstore.SignalEntry{
		PlanFile:   planFile,
		SignalType: "planner_finished",
	}))
	require.NoError(t, gw.Create("test", taskstore.SignalEntry{
		PlanFile:   planFile,
		SignalType: "verify_approved",
	}))

	var scan loop.ScanResult
	claimed := make([]*taskstore.SignalEntry, 0, 2)
	for range 2 {
		entry, err := gw.Claim("test", "app-test")
		require.NoError(t, err)
		require.NotNil(t, entry)
		require.NoError(t, loop.ConvertSignalEntry(entry, &scan))
		claimed = append(claimed, entry)
	}

	model, _ := h.Update(metadataResultMsg{
		PlanState:            ps,
		Signals:              scan.FSMSignals,
		GatewaySignalEntries: claimed,
	})
	_ = model.(*home)

	done, err := gw.List("test", taskstore.SignalDone)
	require.NoError(t, err)
	require.Len(t, done, 1)
	assert.Equal(t, "planner_finished", done[0].SignalType)
	assert.Empty(t, done[0].Result)

	failed, err := gw.List("test", taskstore.SignalFailed)
	require.NoError(t, err)
	require.Len(t, failed, 1)
	assert.Equal(t, "verify_approved", failed[0].SignalType)
	assert.Contains(t, failed[0].Result, "outside verifying")
}

func TestGatewayAckDoesNotAttributeFilesystemSignalToGatewayRow(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, ps, _, _ := plannerSignalHome(t, planFile)
	h.appConfig.AutoAdvance = false

	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })
	h.signalGateway = gw

	require.NoError(t, gw.Create("test", taskstore.SignalEntry{
		PlanFile:   planFile,
		SignalType: "planner_finished",
	}))

	entry, err := gw.Claim("test", "app-test")
	require.NoError(t, err)
	require.NotNil(t, entry)
	var scan loop.ScanResult
	require.NoError(t, loop.ConvertSignalEntry(entry, &scan))
	require.Len(t, scan.FSMSignals, 1)

	model, _ := h.Update(metadataResultMsg{
		PlanState: ps,
		Signals: append([]taskfsm.Signal{{
			Event:    taskfsm.PlannerFinished,
			TaskFile: planFile,
		}}, scan.FSMSignals...),
		GatewaySignalEntries: []*taskstore.SignalEntry{entry},
	})
	_ = model.(*home)

	done, err := gw.List("test", taskstore.SignalDone)
	require.NoError(t, err)
	assert.Empty(t, done)

	failed, err := gw.List("test", taskstore.SignalFailed)
	require.NoError(t, err)
	require.Len(t, failed, 1)
	assert.Equal(t, entry.ID, failed[0].ID)
	assert.Equal(t, "planner_finished", failed[0].SignalType)
	assert.Contains(t, failed[0].Result, "signal rejected by processor")
}

// TestPlannerFinishedSignal_ConfirmKeepsPlannerAndTriggersImplement verifies that
// after the user confirms (plannerCompleteMsg), the planner instance is kept,
// plannerPrompted is set, and triggerTaskStage("implement") is called.
func TestPlannerFinishedSignal_ConfirmKeepsPlannerAndTriggersImplement(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, ps, plansDir, plannerInst := plannerSignalHome(t, planFile)

	// Set up the state as if the confirm dialog was shown after a PlannerFinished signal.
	h.state = stateConfirm
	h.pendingPlannerInstanceTitle = plannerInst.Title
	h.pendingPlannerTaskFile = planFile

	// Write a multi-task plan so triggerTaskStage uses the architect path.
	planContent := "# Plan\n\n## Wave 1\n\n### Task 1: First\n\nDo it.\n\n### Task 2: Second\n\nDo it.\n\n### Task 3: Third\n\nDo it.\n"
	require.NoError(t, ps.SetContent(planFile, planContent))
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, planFile), []byte(planContent), 0o644))

	// Advance the FSM to StatusReady so triggerTaskStage can proceed to "implement".
	require.NoError(t, h.fsm.Transition(planFile, taskfsm.PlannerFinished))
	h.loadTaskState()

	// Send the confirm message.
	_, _ = h.Update(plannerCompleteMsg{planFile: planFile})

	assert.True(t, h.plannerPrompted[planFile],
		"plannerPrompted must be true after confirm")
	assert.Empty(t, h.pendingPlannerInstanceTitle,
		"pendingPlannerInstanceTitle must be cleared after confirm")
	assert.Empty(t, h.pendingPlannerTaskFile,
		"pendingPlannerTaskFile must be cleared after confirm")

	// Planner instance must remain visible while the architect handoff starts.
	foundInNav := false
	for _, inst := range h.nav.GetInstances() {
		if inst.Title == plannerInst.Title {
			foundInNav = true
			break
		}
	}
	assert.True(t, foundInNav, "planner instance must remain in nav after confirm")

	foundInAllInstances := false
	for _, inst := range h.allInstances {
		if inst.Title == plannerInst.Title {
			foundInAllInstances = true
			break
		}
	}
	assert.True(t, foundInAllInstances, "planner instance must remain in allInstances after confirm")
}

// TestPlannerFinishedSignal_CancelKillsPlannerAndLeavesReady verifies that after
// the user cancels (no), the planner instance is removed, plannerPrompted is set,
// and the plan stays at StatusReady.
func TestPlannerFinishedSignal_CancelKillsPlannerAndLeavesReady(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, _, _, plannerInst := plannerSignalHome(t, planFile)

	// Advance FSM to StatusReady (as the signal handler would do).
	require.NoError(t, h.fsm.Transition(planFile, taskfsm.PlannerFinished))
	h.loadTaskState()

	// Set up state as if the confirm dialog was shown.
	h.state = stateConfirm
	h.pendingPlannerInstanceTitle = plannerInst.Title
	h.pendingPlannerTaskFile = planFile
	h.overlays.Show(overlay.NewConfirmationOverlay("plan is ready. start implementation?"))
	h.pendingConfirmAction = func() tea.Msg {
		return plannerCompleteMsg{planFile: planFile}
	}

	// Press "n" (cancel).
	keyMsg := tea.KeyPressMsg{Code: 'n', Text: "n"}
	model, _ := h.handleKeyPress(keyMsg)
	updated := model.(*home)

	assert.True(t, updated.plannerPrompted[planFile],
		"plannerPrompted must be true after cancel")
	assert.Empty(t, updated.pendingPlannerInstanceTitle,
		"pendingPlannerInstanceTitle must be cleared after cancel")
	assert.Empty(t, updated.pendingPlannerTaskFile,
		"pendingPlannerTaskFile must be cleared after cancel")

	// Planner instance must be removed.
	for _, inst := range updated.nav.GetInstances() {
		assert.NotEqual(t, plannerInst.Title, inst.Title,
			"planner instance must be removed from nav after cancel")
	}

	// Plan must still be at StatusReady (not advanced to implementing).
	entry, ok := updated.taskState.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusReady, entry.Status,
		"plan must remain at StatusReady after cancel — user declined implementation")
}

// TestPlannerFinishedSignal_SkipsWhenAlreadyPrompted verifies that when
// plannerPrompted[planFile] is already true, no dialog is shown.
func TestPlannerFinishedSignal_SkipsWhenAlreadyPrompted(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, ps, _, _ := plannerSignalHome(t, planFile)

	// Mark already prompted.
	h.plannerPrompted[planFile] = true

	signal := taskfsm.Signal{
		Event:    taskfsm.PlannerFinished,
		TaskFile: planFile,
	}
	msg := metadataResultMsg{
		PlanState: ps,
		Signals:   []taskfsm.Signal{signal},
	}

	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.NotEqual(t, stateConfirm, updated.state,
		"no confirm dialog when plannerPrompted is already true")
	assert.False(t, updated.overlays.IsActive(),
		"no confirmation overlay when plannerPrompted is already true")
}

// TestPlannerTmuxDeath_NoFallbackDialog verifies that when a planner pane dies
// but NO sentinel was written (plan still in StatusPlanning), NO confirmation
// dialog is shown. The plan must remain in StatusPlanning.
//
// This is the definitive regression test for the removed tmux-death fallback:
// spurious transitions must not occur just because the planner process died.
func TestPlannerTmuxDeath_NoFallbackDialog(t *testing.T) {
	t.Parallel()
	const planFile = "no-fallback"
	h, ps, _, _ := plannerSignalHome(t, planFile)

	// No sentinel written — plan stays in StatusPlanning.
	// The planner pane dies (TmuxAlive: false), but there are no signals.
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: "test-plan-planner", TmuxAlive: false},
		},
		PlanState: ps,
	}

	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.NotEqual(t, stateConfirm, updated.state,
		"tmux death without sentinel must NOT show a confirm dialog")
	assert.False(t, updated.overlays.IsActive(),
		"tmux death without sentinel must NOT show confirmation overlay")

	// Plan must remain in StatusPlanning (not advanced).
	entry, ok := updated.taskState.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusPlanning, entry.Status,
		"plan must remain StatusPlanning when planner pane dies without a sentinel")
}

// TestPlannerFinishedSignal_AutoAdvance_SkipsDialog verifies that when
// appConfig.AutoAdvance is true, a PlannerFinished signal skips the confirmation
// dialog entirely: plannerPrompted is set and no overlay is shown.
func TestPlannerFinishedSignal_AutoAdvance_SkipsDialog(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, ps, _, _ := plannerSignalHome(t, planFile)

	// Enable auto-advance: skip the confirmation dialog.
	h.appConfig.AutoAdvance = true

	signal := taskfsm.Signal{
		Event:    taskfsm.PlannerFinished,
		TaskFile: planFile,
	}
	msg := metadataResultMsg{
		PlanState: ps,
		Signals:   []taskfsm.Signal{signal},
	}

	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.True(t, updated.plannerPrompted[planFile],
		"plannerPrompted must be set when auto-advance skips the dialog")
	assert.NotEqual(t, stateConfirm, updated.state,
		"auto-advance must NOT enter stateConfirm")
	assert.False(t, updated.overlays.IsActive(),
		"auto-advance must NOT show a confirmation overlay")
}

// TestPlannerFinishedSignal_DeferredWhenOverlayActive verifies that when the
// PlannerFinished signal arrives while an overlay is active, the dialog is NOT
// lost — it is deferred and shown on the next metadata tick once the overlay clears.
func TestPlannerFinishedSignal_DeferredWhenOverlayActive(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, ps, _, _ := plannerSignalHome(t, planFile)

	// Explicitly disable auto-advance so this test exercises the deferred dialog path.
	h.appConfig.AutoAdvance = false

	// Simulate an active overlay (e.g. new-plan form is open).
	existingOverlay := overlay.NewConfirmationOverlay("unrelated question?")
	h.state = stateConfirm
	h.overlays.Show(existingOverlay)

	signal := taskfsm.Signal{
		Event:    taskfsm.PlannerFinished,
		TaskFile: planFile,
	}
	msg := metadataResultMsg{
		PlanState: ps,
		Signals:   []taskfsm.Signal{signal},
	}

	model, _ := h.Update(msg)
	updated := model.(*home)

	// Overlay must be untouched — we must NOT clobber it.
	assert.Same(t, existingOverlay, updated.overlays.Current(),
		"existing overlay must not be replaced while active")

	// The plan file must be queued for deferred dialog.
	assert.Contains(t, updated.deferredPlannerDialogs, planFile,
		"plan file must be queued in deferredPlannerDialogs when overlay was active")

	// Now simulate the overlay clearing (state returns to default).
	updated.state = stateDefault
	updated.overlays.Dismiss()

	// Send an empty metadata tick — deferred dialog should fire.
	emptyMsg := metadataResultMsg{PlanState: ps}
	model2, _ := updated.Update(emptyMsg)
	updated2 := model2.(*home)

	assert.Equal(t, stateConfirm, updated2.state,
		"deferred PlannerFinished dialog must show on next tick after overlay clears")
	assert.True(t, updated2.overlays.IsActive(),
		"confirmation overlay must be set for deferred dialog")
	assert.Empty(t, updated2.deferredPlannerDialogs,
		"deferredPlannerDialogs must be cleared after showing the dialog")
	assert.Equal(t, planFile, updated2.pendingPlannerTaskFile,
		"pendingPlannerTaskFile must be set for the deferred plan")
}

// TestPlannerFinishedSignal_SkipsWhenConfirmActive verifies that when
// state == stateConfirm, no new dialog is shown (avoids clobbering an active overlay).
func TestPlannerFinishedSignal_SkipsWhenConfirmActive(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	h, ps, _, _ := plannerSignalHome(t, planFile)

	// Pre-existing confirm overlay.
	existingOverlay := overlay.NewConfirmationOverlay("existing question?")
	h.state = stateConfirm
	h.overlays.Show(existingOverlay)

	signal := taskfsm.Signal{
		Event:    taskfsm.PlannerFinished,
		TaskFile: planFile,
	}
	msg := metadataResultMsg{
		PlanState: ps,
		Signals:   []taskfsm.Signal{signal},
	}

	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.Equal(t, stateConfirm, updated.state,
		"state must remain stateConfirm")
	assert.Same(t, existingOverlay, updated.overlays.Current(),
		"existing overlay must not be replaced when confirm is already active")
	assert.Empty(t, updated.pendingPlannerTaskFile,
		"pendingPlannerTaskFile must not be set when confirm is already active")
}
