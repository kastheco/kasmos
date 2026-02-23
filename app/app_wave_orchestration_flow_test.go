package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kastheco/klique/config"
	"github.com/kastheco/klique/config/planparser"
	"github.com/kastheco/klique/config/planstate"
	"github.com/kastheco/klique/session"
	"github.com/kastheco/klique/ui"
	"github.com/kastheco/klique/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waveFlowHome builds a minimal home struct suitable for wave-orchestration flow tests.
func waveFlowHome(t *testing.T, ps *planstate.PlanState, plansDir string, orchMap map[string]*WaveOrchestrator) *home {
	t.Helper()
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewList(&sp, false)
	h := &home{
		ctx:               context.Background(),
		state:             stateDefault,
		appConfig:         config.DefaultConfig(),
		list:              list,
		menu:              ui.NewMenu(),
		sidebar:           ui.NewSidebar(),
		tabbedWindow:      ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewGitPane()),
		toastManager:      overlay.NewToastManager(&sp),
		planState:         ps,
		planStateDir:      plansDir,
		waveOrchestrators: orchMap,
	}
	return h
}

// TestWaveMonitor_CancelWaveAdvanceRePrompts verifies that canceling a wave-advance
// confirmation resets the orchestrator confirm latch so the next metadata tick
// can display the prompt again (fixes deadlock).
func TestWaveMonitor_CancelWaveAdvanceRePrompts(t *testing.T) {
	plan := &planparser.Plan{
		Waves: []planparser.Wave{
			{Number: 1, Tasks: []planparser.Task{{Number: 1, Title: "First", Body: "do first"}}},
			{Number: 2, Tasks: []planparser.Task{{Number: 2, Title: "Second", Body: "do second"}}},
		},
	}
	orch := NewWaveOrchestrator("test.md", plan)
	orch.StartNextWave()
	orch.MarkTaskComplete(1) // wave 1 done
	orch.NeedsConfirm()      // consume the one-shot latch so it won't fire again
	require.False(t, orch.NeedsConfirm(), "latch already consumed, must be false")

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:                        context.Background(),
		state:                      stateConfirm,
		appConfig:                  config.DefaultConfig(),
		list:                       ui.NewList(&sp, false),
		menu:                       ui.NewMenu(),
		sidebar:                    ui.NewSidebar(),
		tabbedWindow:               ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewGitPane()),
		toastManager:               overlay.NewToastManager(&sp),
		waveOrchestrators:          map[string]*WaveOrchestrator{"test.md": orch},
		pendingWaveConfirmPlanFile: "test.md",
		confirmationOverlay:        overlay.NewConfirmationOverlay("Wave 1 complete. Start Wave 2?"),
	}

	// Press 'n' (cancel key = default "n")
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	_, _ = h.handleKeyPress(keyMsg)

	// Orchestrator latch must be reset so the next tick can re-prompt
	assert.True(t, orch.NeedsConfirm(), "cancel must reset orchestrator confirm latch for re-prompt")
}

// TestWaveMonitor_PausedTaskCountsAsFailed verifies that a paused task instance
// is treated as a failure in the wave monitor, causing the wave to complete (with
// failure) and a failed-wave decision prompt to appear.
func TestWaveMonitor_PausedTaskCountsAsFailed(t *testing.T) {
	const planFile = "2026-02-21-paused-task.md"

	plan := &planparser.Plan{
		Waves: []planparser.Wave{
			{Number: 1, Tasks: []planparser.Task{{Number: 1, Title: "Task 1", Body: "do it"}}},
			{Number: 2, Tasks: []planparser.Task{{Number: 2, Title: "Task 2", Body: "follow up"}}},
		},
	}
	orch := NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := planstate.Load(plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "paused task test", "plan/paused-task", time.Now()))
	require.NoError(t, ps.SetStatus(planFile, planstate.StatusImplementing))

	// Create the task instance but mark it as Paused
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "paused-task-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		PlanFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst.SetStatus(session.Paused)

	h := waveFlowHome(t, ps, plansDir, map[string]*WaveOrchestrator{planFile: orch})
	_ = h.list.AddInstance(inst)

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "paused-task-T1", TmuxAlive: false}},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Wave must have detected failure and shown the failed-wave decision prompt
	assert.Equal(t, stateConfirm, updated.state,
		"paused task must trigger wave-failed decision prompt")
	require.NotNil(t, updated.confirmationOverlay,
		"confirmation overlay must be set for failed-wave decision")
	assert.Equal(t, "r", updated.confirmationOverlay.ConfirmKey,
		"failed-wave confirm key must be 'r' (retry)")
	assert.Equal(t, "s", updated.confirmationOverlay.CancelKey,
		"failed-wave cancel key must be 's' (skip/advance)")
}

// TestWaveMonitor_MissingTaskCountsAsFailed verifies that a task with no matching
// instance in the list is counted as failed, triggering the failed-wave decision prompt.
func TestWaveMonitor_MissingTaskCountsAsFailed(t *testing.T) {
	const planFile = "2026-02-21-missing-task.md"

	plan := &planparser.Plan{
		Waves: []planparser.Wave{
			{Number: 1, Tasks: []planparser.Task{{Number: 1, Title: "Task 1", Body: "do it"}}},
			{Number: 2, Tasks: []planparser.Task{{Number: 2, Title: "Task 2", Body: "follow up"}}},
		},
	}
	orch := NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := planstate.Load(plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "missing task test", "plan/missing-task", time.Now()))
	require.NoError(t, ps.SetStatus(planFile, planstate.StatusImplementing))

	// No instance added to the list — the task is "missing"
	h := waveFlowHome(t, ps, plansDir, map[string]*WaveOrchestrator{planFile: orch})

	msg := metadataResultMsg{
		Results:   []instanceMetadata{},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Missing task must be treated as failed and trigger the failed-wave prompt
	assert.Equal(t, stateConfirm, updated.state,
		"missing task must trigger wave-failed decision prompt")
	require.NotNil(t, updated.confirmationOverlay)
	assert.Equal(t, "r", updated.confirmationOverlay.ConfirmKey,
		"failed-wave confirm key must be 'r' (retry)")
}

// TestWaveMonitor_AbortKeyDeletesOrchestrator verifies that pressing 'a' on the
// failed-wave decision prompt removes the orchestrator and returns to default state.
func TestWaveMonitor_AbortKeyDeletesOrchestrator(t *testing.T) {
	const planFile = "2026-02-21-abort-test.md"

	plan := &planparser.Plan{
		Waves: []planparser.Wave{
			{Number: 1, Tasks: []planparser.Task{{Number: 1, Title: "Task 1", Body: "do it"}}},
			{Number: 2, Tasks: []planparser.Task{{Number: 2, Title: "Task 2", Body: "follow up"}}},
		},
	}
	orch := NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()
	orch.MarkTaskFailed(1)
	require.Equal(t, WaveStateWaveComplete, orch.State())

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:                        context.Background(),
		state:                      stateConfirm,
		appConfig:                  config.DefaultConfig(),
		list:                       ui.NewList(&sp, false),
		menu:                       ui.NewMenu(),
		sidebar:                    ui.NewSidebar(),
		tabbedWindow:               ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewGitPane()),
		toastManager:               overlay.NewToastManager(&sp),
		waveOrchestrators:          map[string]*WaveOrchestrator{planFile: orch},
		pendingWaveConfirmPlanFile: planFile,
		confirmationOverlay:        overlay.NewConfirmationOverlay("Wave 1 failed. r=retry s=skip a=abort"),
		pendingWaveAbortAction: func() tea.Msg {
			return waveAbortMsg{planFile: planFile}
		},
	}
	// Set confirm key to 'r' as the failed-wave dialog would
	h.confirmationOverlay.ConfirmKey = "r"
	h.confirmationOverlay.CancelKey = "s"

	// Press 'a' for abort
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	model, cmd := h.handleKeyPress(keyMsg)
	updated := model.(*home)

	// State must return to default and abort action must be returned as cmd
	assert.Equal(t, stateDefault, updated.state, "state must return to default after abort")
	assert.Nil(t, updated.confirmationOverlay, "overlay must be cleared after abort")
	assert.Nil(t, updated.pendingWaveAbortAction, "abort action must be cleared")
	assert.NotNil(t, cmd, "abort tea.Cmd must be returned so Update can execute it")
}

// TestTriggerPlanStage_ImplementNoWaves_RespecsPlanner verifies that when the
// implement stage is triggered on a plan without ## Wave headers, the plan status
// reverts to planning and a new planner session is spawned with a wave-annotation prompt.
func TestTriggerPlanStage_ImplementNoWaves_RespawnsPlanner(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	const planFile = "2026-02-21-no-waves.md"
	// Plan content without ## Wave headers (has tasks but no waves)
	content := "# Plan\n\n**Goal:** Test\n\n### Task 1: Something\n\nDo it.\n"
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, planFile), []byte(content), 0o644))

	ps, err := planstate.Load(plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "no waves test", "plan/no-waves", time.Now()))
	require.NoError(t, ps.SetStatus(planFile, planstate.StatusPlanning))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewList(&sp, false)
	h := &home{
		ctx:               context.Background(),
		state:             stateDefault,
		appConfig:         config.DefaultConfig(),
		planState:         ps,
		planStateDir:      plansDir,
		activeRepoPath:    dir,
		program:           "opencode",
		list:              list,
		menu:              ui.NewMenu(),
		sidebar:           ui.NewSidebar(),
		tabbedWindow:      ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewGitPane()),
		toastManager:      overlay.NewToastManager(&sp),
		waveOrchestrators: make(map[string]*WaveOrchestrator),
	}

	_, _ = h.triggerPlanStage(planFile, "implement")

	// Plan status must have reverted to planning (parse failed, no StatusImplementing set)
	entry, ok := ps.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, planstate.StatusPlanning, entry.Status,
		"plan status must revert to planning when wave headers are missing")

	// A new planner instance must have been added to the list
	instances := list.GetInstances()
	require.NotEmpty(t, instances, "a planner instance must be spawned after parse failure")
	plannerInst := instances[len(instances)-1]
	assert.Equal(t, session.AgentTypePlanner, plannerInst.AgentType,
		"spawned instance must be a planner")
	assert.Contains(t, plannerInst.QueuedPrompt, "Wave",
		"planner prompt must mention Wave headers")
}

// ---------------------------------------------------------------------------
// Planner-exit detection and confirmation flow tests
// ---------------------------------------------------------------------------

// TestPlannerExit_ShowsImplementConfirm verifies that when a planner session's
// tmux pane dies and the plan status is ready, a confirmation dialog appears.
func TestPlannerExit_ShowsImplementConfirm(t *testing.T) {
	const planFile = "2026-02-22-planner-exit.md"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := planstate.Load(plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "planner exit test", "plan/planner-exit", time.Now()))
	// Default status after Register is StatusReady — exactly what we need.

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "planner-exit-inst",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	inst.AgentType = session.AgentTypePlanner
	inst.PlanFile = planFile

	h := waveFlowHome(t, ps, plansDir, make(map[string]*WaveOrchestrator))
	h.plannerPrompted = make(map[string]bool)
	_ = h.list.AddInstance(inst)

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "planner-exit-inst", TmuxAlive: false}},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.Equal(t, stateConfirm, updated.state,
		"state must be stateConfirm when planner dies and plan is ready")
	require.NotNil(t, updated.confirmationOverlay,
		"confirmation overlay must be set for planner-exit prompt")
	assert.Equal(t, "planner-exit-inst", updated.pendingPlannerInstanceTitle,
		"pendingPlannerInstanceTitle must be set to the planner instance title")
}

// TestPlannerExit_NoRePromptAfterAnswer verifies that once the user has answered
// the planner-exit prompt (plannerPrompted[planFile] = true), the dialog doesn't reappear.
func TestPlannerExit_NoRePromptAfterAnswer(t *testing.T) {
	const planFile = "2026-02-22-no-reprompt.md"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := planstate.Load(plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "no reprompt test", "plan/no-reprompt", time.Now()))

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "planner-no-reprompt",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	inst.AgentType = session.AgentTypePlanner
	inst.PlanFile = planFile

	h := waveFlowHome(t, ps, plansDir, make(map[string]*WaveOrchestrator))
	h.plannerPrompted = map[string]bool{planFile: true} // already answered
	_ = h.list.AddInstance(inst)

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "planner-no-reprompt", TmuxAlive: false}},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.Equal(t, stateDefault, updated.state,
		"state must remain stateDefault when plannerPrompted is already true")
}

// TestPlannerExit_NoPromptWhileAlive verifies that while the planner tmux pane
// is still alive, no confirmation prompt appears.
func TestPlannerExit_NoPromptWhileAlive(t *testing.T) {
	const planFile = "2026-02-22-still-alive.md"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := planstate.Load(plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "still alive test", "plan/still-alive", time.Now()))

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "planner-alive",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	inst.AgentType = session.AgentTypePlanner
	inst.PlanFile = planFile

	h := waveFlowHome(t, ps, plansDir, make(map[string]*WaveOrchestrator))
	h.plannerPrompted = make(map[string]bool)
	_ = h.list.AddInstance(inst)

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: "planner-alive", TmuxAlive: true}},
		PlanState: ps,
	}
	model, _ := h.Update(msg)
	updated := model.(*home)

	assert.Equal(t, stateDefault, updated.state,
		"state must remain stateDefault while planner tmux pane is alive")
}

// TestPlannerExit_EscPreservesForRePrompt verifies that pressing esc on the
// planner-exit dialog does NOT mark plannerPrompted, allowing re-prompt next tick.
func TestPlannerExit_EscPreservesForRePrompt(t *testing.T) {
	const planFile = "2026-02-22-esc-reprompt.md"

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "planner-esc-inst",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	inst.AgentType = session.AgentTypePlanner
	inst.PlanFile = planFile

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:                         context.Background(),
		state:                       stateConfirm,
		appConfig:                   config.DefaultConfig(),
		list:                        ui.NewList(&sp, false),
		menu:                        ui.NewMenu(),
		sidebar:                     ui.NewSidebar(),
		tabbedWindow:                ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewGitPane()),
		toastManager:                overlay.NewToastManager(&sp),
		waveOrchestrators:           make(map[string]*WaveOrchestrator),
		plannerPrompted:             make(map[string]bool),
		pendingPlannerInstanceTitle: "planner-esc-inst",
		confirmationOverlay:         overlay.NewConfirmationOverlay("Plan 'esc-reprompt' is ready. Start implementation?"),
	}
	_ = h.list.AddInstance(inst)

	// Press esc
	keyMsg := tea.KeyMsg{Type: tea.KeyEscape}
	model, _ := h.handleKeyPress(keyMsg)
	updated := model.(*home)

	assert.Equal(t, stateDefault, updated.state,
		"state must return to default after esc")
	assert.Empty(t, updated.plannerPrompted,
		"plannerPrompted must NOT be set after esc — allows re-prompt")
	assert.Empty(t, updated.pendingPlannerInstanceTitle,
		"pendingPlannerInstanceTitle must be cleared after esc")
}

// TestPlannerExit_CancelKillsInstanceAndMarksPrompted verifies that pressing "n"
// on the planner-exit dialog kills the planner instance and marks plannerPrompted.
func TestPlannerExit_CancelKillsInstanceAndMarksPrompted(t *testing.T) {
	const planFile = "2026-02-22-cancel-kill.md"

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "planner-cancel-inst",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	inst.AgentType = session.AgentTypePlanner
	inst.PlanFile = planFile

	// Create storage so saveAllInstances doesn't panic
	state := config.DefaultState()
	storage, err := session.NewStorage(state)
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:                         context.Background(),
		state:                       stateConfirm,
		appConfig:                   config.DefaultConfig(),
		list:                        ui.NewList(&sp, false),
		menu:                        ui.NewMenu(),
		sidebar:                     ui.NewSidebar(),
		tabbedWindow:                ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewGitPane()),
		toastManager:                overlay.NewToastManager(&sp),
		storage:                     storage,
		waveOrchestrators:           make(map[string]*WaveOrchestrator),
		plannerPrompted:             make(map[string]bool),
		pendingPlannerInstanceTitle: "planner-cancel-inst",
		confirmationOverlay:         overlay.NewConfirmationOverlay("Plan 'cancel-kill' is ready. Start implementation?"),
		allInstances:                []*session.Instance{inst},
	}
	_ = h.list.AddInstance(inst)

	// Press 'n' (cancel key — default for confirmation overlay)
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	model, _ := h.handleKeyPress(keyMsg)
	updated := model.(*home)

	assert.True(t, updated.plannerPrompted[planFile],
		"plannerPrompted must be true after cancel")
	assert.Empty(t, updated.allInstances,
		"planner instance must be removed from allInstances after cancel")
	assert.Empty(t, updated.pendingPlannerInstanceTitle,
		"pendingPlannerInstanceTitle must be cleared after cancel")
}
