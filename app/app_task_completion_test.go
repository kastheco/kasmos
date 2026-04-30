package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldPromptPushAfterImplementerExit(t *testing.T) {
	t.Parallel()
	entry := taskstate.TaskEntry{Status: taskstate.StatusImplementing, ExecutionState: taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseSingleAgentImplementing), ActiveAgentType: session.AgentTypeCoder}}
	inst := &session.Instance{TaskFile: "p", AgentType: session.AgentTypeCoder}

	if !session.ShouldAutoAdvanceLifecycleImplementer(string(entry.Status), entry.ExecutionState, inst, false) {
		t.Fatal("expected push prompt for exited coder")
	}
}

func TestShouldPromptPushAfterImplementerExit_SDKCoderExited(t *testing.T) {
	t.Parallel()
	entry := taskstate.TaskEntry{Status: taskstate.StatusImplementing, ExecutionState: taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseSingleAgentImplementing), ActiveAgentType: session.AgentTypeCoder}}
	inst := &session.Instance{
		TaskFile:      "p.md",
		AgentType:     session.AgentTypeCoder,
		ExecutionMode: config.ExecutionModeSDK,
		Exited:        true,
	}

	assert.True(t, session.ShouldAutoAdvanceLifecycleImplementer(string(entry.Status), entry.ExecutionState, inst, true))
}

func TestShouldPromptPushAfterImplementerExit_PromptDetectedTriggers(t *testing.T) {
	t.Parallel()
	entry := taskstate.TaskEntry{Status: taskstate.StatusImplementing, ExecutionState: taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseSingleAgentImplementing), ActiveAgentType: session.AgentTypeCoder}}
	inst := &session.Instance{
		TaskFile:              "p",
		AgentType:             session.AgentTypeCoder,
		PromptDetected:        true,
		AwaitingWork:          false,
		CompletionPromptSince: time.Now().Add(-(session.CompletionPromptStabilityWindow + 10*time.Millisecond)),
	}

	// Tmux is still alive but the implementer returned to prompt after finishing
	// its queued work — this covers the "applying fixes" completion path.
	assert.True(t, session.ShouldAutoAdvanceLifecycleImplementer(string(entry.Status), entry.ExecutionState, inst, true),
		"expected push prompt for coder at prompt (PromptDetected && !AwaitingWork)")
}

func TestShouldPromptPushAfterImplementerExit_AwaitingWorkSuppresses(t *testing.T) {
	t.Parallel()
	entry := taskstate.TaskEntry{Status: taskstate.StatusImplementing, ExecutionState: taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseSingleAgentImplementing), ActiveAgentType: session.AgentTypeCoder}}
	inst := &session.Instance{
		TaskFile:       "p",
		AgentType:      session.AgentTypeCoder,
		PromptDetected: true,
		AwaitingWork:   true,
	}

	// Coder is at prompt but still waiting for its queued prompt to be
	// delivered — must NOT trigger push prompt yet.
	assert.False(t, session.ShouldAutoAdvanceLifecycleImplementer(string(entry.Status), entry.ExecutionState, inst, true),
		"must not trigger push prompt while AwaitingWork is true")
}

func TestShouldPromptPushAfterImplementerExit_FixerPromptDetectedTriggers(t *testing.T) {
	t.Parallel()
	entry := taskstate.TaskEntry{Status: taskstate.StatusImplementing, ExecutionState: taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseFixing), ActiveAgentType: session.AgentTypeFixer}}
	inst := &session.Instance{
		TaskFile:              "p",
		AgentType:             session.AgentTypeFixer,
		PromptDetected:        true,
		AwaitingWork:          false,
		CompletionPromptSince: time.Now().Add(-(session.CompletionPromptStabilityWindow + 10*time.Millisecond)),
	}

	assert.True(t, session.ShouldAutoAdvanceLifecycleImplementer(string(entry.Status), entry.ExecutionState, inst, true),
		"expected push prompt for fixer at prompt (PromptDetected && !AwaitingWork)")
}

func TestShouldPromptPushAfterImplementerExit_NoPromptForSoloAgent(t *testing.T) {
	t.Parallel()
	entry := taskstate.TaskEntry{Status: taskstate.StatusImplementing, ExecutionState: taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseSingleAgentImplementing), ActiveAgentType: session.AgentTypeCoder}}
	inst := &session.Instance{TaskFile: "p", AgentType: session.AgentTypeCoder, SoloAgent: true}

	assert.False(t, session.ShouldAutoAdvanceLifecycleImplementer(string(entry.Status), entry.ExecutionState, inst, false),
		"solo agents must not trigger automatic push prompt")
}

func TestShouldPromptPushAfterImplementerExit_NoPromptForReviewer(t *testing.T) {
	t.Parallel()
	entry := taskstate.TaskEntry{Status: taskstate.StatusImplementing, ExecutionState: taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseSingleAgentImplementing), ActiveAgentType: session.AgentTypeCoder}}
	inst := &session.Instance{TaskFile: "p", AgentType: session.AgentTypeReviewer}

	if session.ShouldAutoAdvanceLifecycleImplementer(string(entry.Status), entry.ExecutionState, inst, false) {
		t.Fatal("did not expect push prompt for reviewer")
	}
}

// TestMetadataTickHandler_CoderExitTriggersPrompt verifies that when the metadata
// tick handler processes a coder instance with TmuxAlive=false and plan status
// StatusImplementing, it wires through to promptPushBranchThenAdvance and sets
// the confirmation overlay (proving the push-prompt lifecycle path is connected).
func TestMetadataTickHandler_CoderExitTriggersPrompt(t *testing.T) {
	t.Parallel()
	const planFile = "test-feature"

	// Build a planState with the plan in StatusImplementing.
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "test feature", "plan/test-feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseSingleAgentImplementing), ActiveAgentType: session.AgentTypeCoder}))

	// Build a coder instance (not started — we inject metadata directly).
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:     "test-feature-implement",
		Path:      t.TempDir(),
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeCoder,
	})
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(inst)

	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          list,
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		overlays:     overlay.NewManager(),
		taskState:    ps,
		taskStateDir: plansDir,
		fsm:          newPlanFSMForTest(t, plansDir),
	}

	// Inject a metadataResultMsg with TmuxAlive=false for the coder instance.
	// This simulates the coder's tmux session having exited.
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{
				Title:     inst.Title,
				TmuxAlive: false,
			},
		},
		PlanState: ps,
	}

	model, _ := h.Update(msg)
	updated, ok := model.(*home)
	require.True(t, ok)

	// The push-prompt confirmation overlay must have been set.
	assert.Equal(t, stateConfirm, updated.state,
		"expected stateConfirm after coder exit with StatusImplementing")
	assert.True(t, updated.overlays.IsActive(),
		"expected confirmation overlay to be set for push-prompt")
}

// TestMetadataTickHandler_CoderPromptDetectedTriggersPrompt verifies that when
// a fixer (spawned by spawnFixerWithFeedback) finishes its work and returns
// to prompt (PromptDetected=true, AwaitingWork=false) while tmux is still alive,
// the push-prompt confirmation overlay is shown. This is the key path that enables
// the review→fix→re-review automation cycle.
func TestMetadataTickHandler_CoderPromptDetectedTriggersPrompt(t *testing.T) {
	t.Parallel()
	const planFile = "test-feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "test feature", "plan/test-feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseFixing), ActiveAgentType: session.AgentTypeFixer}))

	// Build a fixer instance that has finished its queued work and returned to prompt.
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:     "test-feature-implement",
		Path:      t.TempDir(),
		Program:   "opencode",
		TaskFile:  planFile,
		AgentType: session.AgentTypeFixer,
	})
	require.NoError(t, err)
	inst.PromptDetected = true
	inst.AwaitingWork = false
	inst.CompletionPromptSince = time.Now().Add(-(session.CompletionPromptStabilityWindow + 10*time.Millisecond))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(inst)

	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          list,
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		overlays:     overlay.NewManager(),
		taskState:    ps,
		taskStateDir: plansDir,
		fsm:          newPlanFSMForTest(t, plansDir),
	}

	// Tmux is still alive — the coder is at its prompt, not exited.
	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{
				Title:     inst.Title,
				TmuxAlive: true,
			},
		},
		PlanState: ps,
	}

	model, _ := h.Update(msg)
	updated, ok := model.(*home)
	require.True(t, ok)

	assert.Equal(t, stateConfirm, updated.state,
		"expected stateConfirm when coder is at prompt (PromptDetected && !AwaitingWork)")
	assert.True(t, updated.overlays.IsActive(),
		"expected confirmation overlay for push-prompt on prompt-detected fixer")
}

func TestMetadataTickHandler_UpdatedPromptFrameTriggersPrompt(t *testing.T) {
	t.Parallel()
	const planFile = "test-feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "test feature", "plan/test-feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseFixing), ActiveAgentType: session.AgentTypeFixer}))

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:     "test-feature-implement",
		Path:      t.TempDir(),
		Program:   "opencode",
		TaskFile:  planFile,
		AgentType: session.AgentTypeFixer,
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	// Simulate that the prompt has already been stable for longer than the
	// stability window so that ShouldAutoAdvanceLifecycleImplementer returns
	// true on this tick (HasPrompt=true will set PromptDetected during Update).
	inst.CompletionPromptSince = time.Now().Add(-(session.CompletionPromptStabilityWindow + 10*time.Millisecond))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(inst)

	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          list,
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		overlays:     overlay.NewManager(),
		taskState:    ps,
		taskStateDir: plansDir,
		fsm:          newPlanFSMForTest(t, plansDir),
	}

	msg := metadataResultMsg{
		Results: []instanceMetadata{{
			Title:           inst.Title,
			ContentCaptured: true,
			Updated:         true,
			HasPrompt:       true,
			TmuxAlive:       true,
		}},
		PlanState: ps,
	}

	model, _ := h.Update(msg)
	updated, ok := model.(*home)
	require.True(t, ok)

	assert.True(t, inst.PromptDetected,
		"updated prompt frames must still mark the instance as prompt-detected")
	assert.Equal(t, session.Ready, inst.Status,
		"updated prompt frames at a prompt must settle to ready, not keep spinning as running")
	assert.Equal(t, stateConfirm, updated.state,
		"updated prompt frames must still trigger the implementer completion prompt")
	assert.True(t, updated.overlays.IsActive(),
		"updated prompt frames must still show the push/review confirmation overlay")
}

func TestMetadataTickHandler_CoderPromptDeferredInFocusMode(t *testing.T) {
	t.Parallel()
	const planFile = "test-feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "test feature", "plan/test-feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseFixing),
		ActiveAgentType: session.AgentTypeFixer,
	}))

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:     "test-feature-implement",
		Path:      t.TempDir(),
		Program:   "opencode",
		TaskFile:  planFile,
		AgentType: session.AgentTypeFixer,
	})
	require.NoError(t, err)
	inst.PromptDetected = true
	inst.AwaitingWork = false
	inst.CompletionPromptSince = time.Now().Add(-(session.CompletionPromptStabilityWindow + 10*time.Millisecond))
	inst.MarkStartedForTest()

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(inst)

	h := &home{
		ctx:                       context.Background(),
		state:                     stateFocusAgent,
		appConfig:                 config.DefaultConfig(),
		nav:                       list,
		menu:                      ui.NewMenu(),
		tabbedWindow:              ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:              overlay.NewToastManager(&sp),
		overlays:                  overlay.NewManager(),
		taskState:                 ps,
		taskStateDir:              plansDir,
		fsm:                       newPlanFSMForTest(t, plansDir),
		deferredCoderPushToastIDs: make(map[string]string),
	}
	h.tabbedWindow.SetFocusMode(true)
	h.menu.SetFocusMode(true)

	msg := metadataResultMsg{
		Results: []instanceMetadata{{
			Title:     inst.Title,
			TmuxAlive: true,
		}},
		PlanState: ps,
	}

	model, _ := h.Update(msg)
	updated, ok := model.(*home)
	require.True(t, ok)

	assert.Equal(t, stateFocusAgent, updated.state,
		"focus mode must not be interrupted by coder push prompt")
	assert.True(t, updated.tabbedWindow.IsFocusMode(),
		"tabbed window should remain in focus mode")
	assert.Contains(t, updated.deferredCoderPushDialogs, planFile,
		"push prompt should be deferred for later")
}

func TestMetadataTick_TaskFinishedSignalMarksWaveTaskComplete(t *testing.T) {
	t.Parallel()
	const planFile = "task-finished.md"

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{{
			Number: 1,
			Tasks:  []taskparser.Task{{Number: 1, Title: "Only task", Body: "do it"}},
		}},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.StartNextWave()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "task finished test", "plan/task-finished", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "task-finished-W1-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   planFile,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(inst)

	h := &home{
		ctx:               context.Background(),
		state:             stateDefault,
		appConfig:         config.DefaultConfig(),
		nav:               list,
		menu:              ui.NewMenu(),
		tabbedWindow:      ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:      overlay.NewToastManager(&sp),
		overlays:          overlay.NewManager(),
		taskState:         ps,
		taskStateDir:      plansDir,
		fsm:               newPlanFSMForTest(t, plansDir),
		waveOrchestrators: map[string]*orchestration.WaveOrchestrator{planFile: orch},
	}

	msg := metadataResultMsg{
		Results:   []instanceMetadata{{Title: inst.Title, TmuxAlive: true}},
		PlanState: ps,
		TaskSignals: []taskfsm.TaskSignal{{
			WaveNumber: 1,
			TaskNumber: 1,
			TaskFile:   planFile,
		}},
	}

	model, _ := h.Update(msg)
	updated, ok := model.(*home)
	require.True(t, ok)

	assert.Empty(t, updated.waveOrchestrators, "orchestrator must be deleted after final task signal")
	assert.True(t, inst.ImplementationComplete, "task instance must be marked complete")
	assert.Equal(t, session.Ready, inst.Status, "task instance should be left ready for wave completion handling")
	assert.Equal(t, stateConfirm, updated.state, "final task signal should trigger the review prompt")
	assert.True(t, updated.overlays.IsActive(), "review prompt should be shown after final task signal")
}

// TestPromptPushBranchThenAdvance_SetStatusErrorPropagates verifies that when
// SetStatus fails inside the push-action closure, the error is returned as a
// tea.Msg rather than being silently swallowed with _ =.
//
// TestPromptPushBranchThenAdvance_ReturnsCoderCompleteMsg verifies that the
// confirm action returns a coderCompleteMsg so the Update handler can perform
// the FSM transition and spawn a reviewer.
func TestPromptPushBranchThenAdvance_ReturnsCoderCompleteMsg(t *testing.T) {
	t.Parallel()
	const planFile = "test-feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "test feature", "plan/test-feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	inst := &session.Instance{
		TaskFile:  planFile,
		AgentType: session.AgentTypeCoder,
	}

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		taskState:    ps,
		taskStateDir: plansDir,
		fsm:          newPlanFSMForTest(t, plansDir),
		toastManager: overlay.NewToastManager(&sp),
		overlays:     overlay.NewManager(),
	}

	// Call promptPushBranchThenAdvance — this sets pendingConfirmAction.
	_ = h.promptPushBranchThenAdvance(inst)

	require.NotNil(t, h.pendingConfirmAction,
		"pendingConfirmAction must be set after promptPushBranchThenAdvance")

	msg := h.pendingConfirmAction()

	ccMsg, ok := msg.(coderCompleteMsg)
	assert.True(t, ok,
		"push action must return coderCompleteMsg, got %T: %v", msg, msg)
	assert.Equal(t, planFile, ccMsg.planFile,
		"coderCompleteMsg must carry the correct plan file")
}

// TestMetadataTickHandler_NoRepromptWhenConfirmPending verifies that when the
// app is already in stateConfirm (a confirmation overlay is showing), a second
// metadata tick does NOT re-trigger promptPushBranchThenAdvance and overwrite
// the existing overlay. Without this guard the modal re-appears every tick.
func TestMetadataTickHandler_NoRepromptWhenConfirmPending(t *testing.T) {
	t.Parallel()
	const planFile = "test-feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "test feature", "plan/test-feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	// ShouldAutoAdvanceLifecycleImplementer requires a single-agent phase; without
	// this the coder-exit check returns false and stateConfirm is never set.
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseSingleAgentImplementing),
		ActiveAgentType: session.AgentTypeCoder,
	}))

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:     "test-feature-implement",
		Path:      t.TempDir(),
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeCoder,
	})
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(inst)

	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          list,
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		overlays:     overlay.NewManager(),
		taskState:    ps,
		taskStateDir: plansDir,
		fsm:          newPlanFSMForTest(t, plansDir),
	}

	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: inst.Title, TmuxAlive: false},
		},
		PlanState: ps,
	}

	// First tick: should set stateConfirm and the overlay.
	model1, _ := h.Update(msg)
	updated1, ok := model1.(*home)
	require.True(t, ok)
	require.Equal(t, stateConfirm, updated1.state, "first tick must set stateConfirm")
	firstOverlay := updated1.overlays.Current()
	require.NotNil(t, firstOverlay)

	// Second tick while stateConfirm is active: must NOT overwrite the overlay.
	model2, _ := updated1.Update(msg)
	updated2, ok := model2.(*home)
	require.True(t, ok)
	assert.Equal(t, stateConfirm, updated2.state, "state must remain stateConfirm")
	assert.Same(t, firstOverlay, updated2.overlays.Current(),
		"second tick must not replace the existing confirmation overlay")
}

func TestFullPlanLifecycle_StateTransitions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(
		"auth-refactor",
		"Refactor JWT auth",
		"plan/auth-refactor",
		time.Date(2026, 2, 21, 10, 0, 0, 0, time.UTC),
	))

	seedPlanStatus(t, ps, "auth-refactor", taskstate.StatusPlanning)
	seedPlanStatus(t, ps, "auth-refactor", taskstate.StatusImplementing)
	seedPlanStatus(t, ps, "auth-refactor", taskstate.StatusReviewing)
	seedPlanStatus(t, ps, "auth-refactor", taskstate.StatusDone)

	entry, ok := ps.Entry("auth-refactor")
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusDone, entry.Status)
	assert.Equal(t, "plan/auth-refactor", entry.Branch)
}

// TestMetadataResultMsg_SignalDoesNotClobberFreshPlanState verifies that when
// signals are present in a metadataResultMsg, the stale msg.PlanState (loaded
// by the goroutine before signals were scanned) does not overwrite the fresh
// planState that loadTaskState() sets after FSM transitions are applied.
//
// Regression test for: sentinel processed → disk updated → loadTaskState() →
// m.taskState="ready", then m.taskState=msg.PlanState → m.taskState="planning"
// (stale), causing the sidebar to show the wrong status for ~500ms.
func TestMetadataResultMsg_SignalDoesNotClobberFreshPlanState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	const planFile = "feature"
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	// Planner is running — status is "planning"
	seedPlanStatus(t, ps, planFile, taskstate.StatusPlanning)

	// Simulate the goroutine snapshot: loaded "planning" before sentinel was seen.
	stalePlanState, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	assert.Equal(t, taskstate.StatusPlanning, stalePlanState.Plans[planFile].Status)

	// Build a minimal home with FSM wired up.
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		taskState:             stalePlanState, // starts with stale state
		taskStateDir:          plansDir,
		taskStore:             storeForDir(t, plansDir),
		taskStoreProject:      "test",
		fsm:                   newPlanFSMForTest(t, plansDir),
		pendingReviewFeedback: make(map[string]string),
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		overlays:              overlay.NewManager(),
		nav:                   ui.NewNavigationPanel(&sp),
	}

	// Construct a metadataResultMsg as the goroutine would: stale PlanState +
	// a PlannerFinished signal (sentinel written after the goroutine loaded state).
	signal := taskfsm.Signal{
		Event:    taskfsm.PlannerFinished,
		TaskFile: planFile,
	}
	// We can't set the private filePath, so we pre-delete the sentinel file
	// (ConsumeSignal would normally delete it — safe to skip deletion here).

	msg := metadataResultMsg{
		PlanState: stalePlanState, // goroutine's stale snapshot
		Signals:   []taskfsm.Signal{signal},
	}

	// Feed the message through Update.
	_, _ = h.Update(msg)

	// After Update, h.taskState must reflect the FSM transition (planning→ready),
	// NOT the stale msg.PlanState snapshot.
	require.NotNil(t, h.taskState)
	entry, ok := h.taskState.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusReady, entry.Status,
		"planState must show 'ready' after PlannerFinished signal — stale msg.PlanState must not overwrite it")
}

func TestMetadataResultMsg_StaleSnapshotDoesNotClobberPlanning(t *testing.T) {
	t.Parallel()
	const planFile = "feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store, ps, fsm := newSharedStoreForTest(t, plansDir)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	require.NoError(t, ps.ForceSetLifecycle(planFile, taskstate.StatusReady, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhasePlanned)}))

	stalePlanState, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)

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
		waveOrchestrators:     make(map[string]*orchestration.WaveOrchestrator),
		pendingReviewFeedback: make(map[string]string),
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
	}

	require.NoError(t, h.fsm.Transition(planFile, taskfsm.PlanStart))
	h.loadTaskState()

	current, ok := h.taskState.Entry(planFile)
	require.True(t, ok)
	require.Equal(t, taskstate.StatusPlanning, current.Status)
	require.NotZero(t, h.taskStateLoadedAt)

	_, _ = h.Update(metadataResultMsg{
		PlanState:         stalePlanState,
		PlanStateLoadedAt: h.taskStateLoadedAt.Add(-time.Second),
	})

	entry, ok := h.taskState.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusPlanning, entry.Status,
		"an older metadata snapshot must not revert a locally-started planner back to ready/planned")
	assert.Equal(t, "", strings.TrimSpace(entry.ExecutionState.Phase),
		"stale ready/planned execution state must not overwrite the current planning lifecycle")
}

// TestImplementFinishedSignal_SpawnsReviewer verifies that when an
// implement-finished sentinel is processed, a reviewer instance is added to the
// list and a start cmd is returned. This is the sentinel-driven equivalent of
// the old checkPlanCompletion → transitionToReview path.
func TestImplementFinishedSignal_SpawnsReviewer(t *testing.T) {
	t.Parallel()
	const planFile = "feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	// Create a coder instance bound to this plan.
	coderInst, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-implement",
		Path:      dir,
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeCoder,
	})
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(coderInst)

	appCfg := config.DefaultConfig()
	appCfg.AutoReviewFix = true

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             appCfg,
		nav:                   list,
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		taskState:             ps,
		taskStateDir:          plansDir,
		fsm:                   newPlanFSMForTest(t, plansDir),
		pendingReviewFeedback: make(map[string]string),
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
		activeRepoPath:        dir,
		program:               "claude",
	}

	signal := taskfsm.Signal{
		Event:    taskfsm.ImplementFinished,
		TaskFile: planFile,
	}
	msg := metadataResultMsg{
		PlanState: ps,
		Signals:   []taskfsm.Signal{signal},
	}

	_, _ = h.Update(msg)

	// A reviewer instance must have been added to the list.
	var foundReviewer bool
	for _, inst := range h.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.IsReviewer {
			foundReviewer = true
			break
		}
	}
	assert.True(t, foundReviewer,
		"implement-finished signal must spawn a reviewer instance")

	// Plan status must be "reviewing" on disk.
	reloaded, _ := newTestPlanState(t, plansDir)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusReviewing, entry.Status)
}

// TestReviewChangesSignal_RespawnsCoder verifies that when a review-changes
// sentinel is processed, the plan transitions back to implementing and a new
// coder instance is added with the reviewer's feedback in its prompt.
func TestReviewChangesSignal_RespawnsFixer(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	const feedback = "Fix the error handling in auth.go"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	// Write a minimal plan file so spawnTaskAgent can read it.
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, planFile), []byte("# Plan\n## Wave 1\n- Task 1\n"), 0o644))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReviewing)
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseReviewing), ActiveAgentType: session.AgentTypeReviewer}))

	// Create a reviewer instance bound to this plan.
	reviewerInst, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-review",
		Path:      dir,
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	})
	require.NoError(t, err)
	reviewerInst.IsReviewer = true

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(reviewerInst)

	appCfg := config.DefaultConfig()
	appCfg.AutoReviewFix = true

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             appCfg,
		nav:                   list,
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		taskState:             ps,
		taskStateDir:          plansDir,
		fsm:                   newPlanFSMForTest(t, plansDir),
		pendingReviewFeedback: make(map[string]string),
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
		activeRepoPath:        dir,
		program:               "claude",
	}

	signal := taskfsm.Signal{
		Event:    taskfsm.ReviewChangesRequested,
		TaskFile: planFile,
		Body:     feedback,
	}
	msg := metadataResultMsg{
		PlanState: ps,
		Signals:   []taskfsm.Signal{signal},
	}

	_, _ = h.Update(msg)

	// A fixer instance must have been added with feedback in its prompt.
	var foundFixer bool
	for _, inst := range h.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.AgentType == session.AgentTypeFixer {
			foundFixer = true
			assert.Contains(t, inst.QueuedPrompt, feedback,
				"fixer prompt must contain reviewer feedback")
			assert.Contains(t, inst.QueuedPrompt, "not an implementer",
				"fixer prompt must keep the agent in fix-only mode")
			assert.NotContains(t, inst.QueuedPrompt, "execute all tasks sequentially",
				"fixer prompt must not reuse the broad implement prompt")
			break
		}
	}
	assert.True(t, foundFixer,
		"review-changes signal must spawn a fixer instance")

	// Plan status must be "implementing" on disk.
	reloaded, _ := newTestPlanState(t, plansDir)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusImplementing, entry.Status)
}

func TestMetadataResultMsg_DaemonManagedRepoIgnoresReviewChangesSignal(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	const feedback = "Fix the error handling in auth.go"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, planFile), []byte("# Plan\n## Wave 1\n- Task 1\n"), 0o644))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReviewing)

	reviewerInst, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-review",
		Path:      dir,
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	})
	require.NoError(t, err)
	reviewerInst.IsReviewer = true

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(reviewerInst)

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             config.DefaultConfig(),
		nav:                   list,
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		overlays:              overlay.NewManager(),
		taskState:             ps,
		taskStateDir:          plansDir,
		fsm:                   newPlanFSMForTest(t, plansDir),
		pendingReviewFeedback: make(map[string]string),
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
		activeRepoPath:        dir,
		program:               "claude",
	}

	_, _ = h.Update(metadataResultMsg{
		PlanState:         ps,
		Signals:           []taskfsm.Signal{{Event: taskfsm.ReviewChangesRequested, TaskFile: planFile, Body: feedback}},
		DaemonManagedRepo: true,
	})

	for _, inst := range h.nav.GetInstances() {
		assert.NotEqual(t, session.AgentTypeFixer, inst.AgentType, "daemon-managed review signals must not spawn a local fixer")
	}

	reloaded, _ := newTestPlanState(t, plansDir)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusReviewing, entry.Status)
}

func TestMetadataResultMsg_ProcessorReviewChangesSignalSpawnsFixer(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	const feedback = "Fix the error handling in auth.go"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, planFile), []byte("# Plan\n## Wave 1\n- Task 1\n"), 0o644))

	store := storeForDir(t, plansDir)
	ps, err := newTestPlanStateWithStore(t, store, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReviewing)

	reviewerInst, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-review",
		Path:      dir,
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	})
	require.NoError(t, err)
	reviewerInst.IsReviewer = true

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(reviewerInst)

	appCfg := config.DefaultConfig()
	appCfg.AutoReviewFix = true

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             appCfg,
		nav:                   list,
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		overlays:              overlay.NewManager(),
		taskState:             ps,
		taskStore:             store,
		taskStoreProject:      "test",
		taskStateDir:          plansDir,
		fsm:                   newPlanFSMForTestWithStore(t, store, plansDir),
		pendingReviewFeedback: make(map[string]string),
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
		activeRepoPath:        dir,
		program:               "claude",
	}

	_, _ = h.Update(metadataResultMsg{
		PlanState: ps,
		Signals: []taskfsm.Signal{{
			Event:    taskfsm.ReviewChangesRequested,
			TaskFile: planFile,
			Body:     feedback,
		}},
	})

	var foundFixer bool
	for _, inst := range h.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.AgentType == session.AgentTypeFixer {
			foundFixer = true
			assert.Contains(t, inst.QueuedPrompt, feedback)
			break
		}
	}
	assert.True(t, foundFixer, "processor-backed review-changes signal must spawn a fixer instance")

	reloaded, err := newTestPlanStateWithStore(t, store, plansDir)
	require.NoError(t, err)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusImplementing, entry.Status)
	assert.Equal(t, 1, entry.ReviewCycle)
}

func TestTickUpdateMetadata_DaemonManagedRepoSkipsFilesystemReviewSignals(t *testing.T) {
	// serial: modifies repoManagedByDaemon
	dir := t.TempDir()
	signalsDir := filepath.Join(dir, ".kasmos", "signals")
	require.NoError(t, os.MkdirAll(signalsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(signalsDir, "review-approved-feature.md"), []byte("Approved."), 0o644))

	old := repoManagedByDaemon
	repoManagedByDaemon = func(repoPath string) bool {
		return filepath.Clean(repoPath) == filepath.Clean(dir)
	}
	t.Cleanup(func() {
		repoManagedByDaemon = old
	})

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		state:            stateDefault,
		nav:              ui.NewNavigationPanel(&sp),
		menu:             ui.NewMenu(),
		tabbedWindow:     ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:     overlay.NewToastManager(&sp),
		overlays:         overlay.NewManager(),
		activeRepoPath:   dir,
		signalsDir:       signalsDir,
		taskStoreProject: "test",
	}

	_, cmd := h.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)

	msg, ok := cmd().(metadataResultMsg)
	require.True(t, ok)
	assert.True(t, msg.DaemonManagedRepo)
	assert.Empty(t, msg.Signals)
	assert.Empty(t, msg.TaskSignals)
	assert.Empty(t, msg.WaveSignals)
	assert.Empty(t, msg.ElaborationSignals)
}

func TestTickUpdateMetadata_DaemonManagedRepoLoadsTaskStateFromStore(t *testing.T) {
	// serial: modifies repoManagedByDaemon and listDaemonInstances
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	// Set up a real store with task state (simulating daemon writes to SQLite).
	store, ps, _ := newSharedStoreForTest(t, plansDir)
	require.NoError(t, ps.Create("feature", "feature", "plan/feature", "core", time.Now()))
	require.NoError(t, ps.ForceSetLifecycle("feature", taskstate.StatusImplementing,
		taskstore.ExecutionState{Phase: "wave_running", ActiveAgentType: session.AgentTypeCoder, ActiveWave: 2}))
	require.NoError(t, ps.IncrementReviewCycle("feature"))

	oldManaged := repoManagedByDaemon
	oldListInstances := listDaemonInstances
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
		listDaemonInstances = oldListInstances
	})

	repoManagedByDaemon = func(repoPath string) bool {
		return filepath.Clean(repoPath) == filepath.Clean(dir)
	}
	listDaemonInstances = func(project string) ([]api.InstanceStatus, error) {
		require.Equal(t, "test", project)
		return nil, nil
	}

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		state:            stateDefault,
		nav:              ui.NewNavigationPanel(&sp),
		menu:             ui.NewMenu(),
		tabbedWindow:     ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:     overlay.NewToastManager(&sp),
		overlays:         overlay.NewManager(),
		activeRepoPath:   dir,
		taskStateDir:     plansDir,
		taskStore:        store,
		taskStoreProject: "test",
	}

	_, cmd := h.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)

	msg, ok := cmd().(metadataResultMsg)
	require.True(t, ok)
	assert.True(t, msg.DaemonManagedRepo)
	assert.True(t, msg.DaemonTaskState)
	require.NotNil(t, msg.PlanState)

	entry, ok := msg.PlanState.Entry("feature")
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusImplementing, entry.Status)
	assert.Equal(t, taskstore.ExecutionState{Phase: "wave_running", ActiveAgentType: session.AgentTypeCoder, ActiveWave: 2}, entry.ExecutionState)
	assert.Equal(t, 1, entry.ReviewCycle)

	topics := msg.PlanState.Topics()
	require.Len(t, topics, 1)
	assert.Equal(t, "core", topics[0].Name)
}

func TestMetadataResultMsg_DaemonManagedRepoAddsMissingDaemonWaveTask(t *testing.T) {
	// serial: modifies repoManagedByDaemon, listDaemonInstances, and restoreInstanceFromData
	dir := t.TempDir()

	oldManaged := repoManagedByDaemon
	oldList := listDaemonInstances
	oldRestore := restoreInstanceFromData
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
		listDaemonInstances = oldList
		restoreInstanceFromData = oldRestore
	})

	repoManagedByDaemon = func(repoPath string) bool {
		return filepath.Clean(repoPath) == filepath.Clean(dir)
	}
	listDaemonInstances = func(project string) ([]api.InstanceStatus, error) {
		require.Equal(t, "test", project)
		return []api.InstanceStatus{{
			Title:      "feature-W1-T1",
			Plan:       "feature",
			Role:       session.AgentTypeCoder,
			Branch:     "plan/feature",
			Program:    "opencode",
			WaveNumber: 1,
			TaskNumber: 1,
			Active:     true,
		}}, nil
	}
	restoreInstanceFromData = func(data session.InstanceData) (*session.Instance, error) {
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:         data.Title,
			Path:          data.Path,
			Program:       data.Program,
			ExecutionMode: data.ExecutionMode,
			TaskFile:      data.TaskFile,
			AgentType:     data.AgentType,
			TaskNumber:    data.TaskNumber,
			WaveNumber:    data.WaveNumber,
			ReviewCycle:   data.ReviewCycle,
		})
		if err != nil {
			return nil, err
		}
		inst.Branch = data.Branch
		if data.Branch != "" {
			inst.BindSharedTaskWorktree(data.Path, data.Branch)
		}
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return inst, nil
	}

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		state:            stateDefault,
		nav:              ui.NewNavigationPanel(&sp),
		menu:             ui.NewMenu(),
		tabbedWindow:     ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:     overlay.NewToastManager(&sp),
		overlays:         overlay.NewManager(),
		activeRepoPath:   dir,
		taskStoreProject: "test",
	}

	_, cmd := h.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)
	msg, ok := cmd().(metadataResultMsg)
	require.True(t, ok)
	require.Len(t, msg.DaemonInstances, 1)

	updatedModel, _ := h.Update(msg)
	updated := updatedModel.(*home)
	instances := updated.nav.GetInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, "feature-W1-T1", instances[0].Title)
	assert.Equal(t, "feature", instances[0].TaskFile)
	assert.Equal(t, 1, instances[0].WaveNumber)
	assert.Equal(t, 1, instances[0].TaskNumber)
	assert.Equal(t, session.AgentTypeCoder, instances[0].AgentType)
}

// TestReviewerTmuxDeath_DoesNotAutoApprove verifies that when a reviewer's tmux
// session dies (e.g. killed manually), the plan is NOT automatically transitioned
// to done. Approval must come exclusively from an explicit review-approved sentinel.
//
// Note: session.Instance.Started() is not settable from outside the session package
// without real tmux, so this test uses a non-started instance. The guard catches any
// reimplementation that drops the started check — a started instance would require
// an integration test. The behavioral contract is: no auto-approve on reviewer death.
func TestReviewerTmuxDeath_DoesNotAutoApprove(t *testing.T) {
	t.Parallel()
	const planFile = "feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReviewing)

	reviewerInst, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-review",
		Path:      dir,
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	})
	require.NoError(t, err)
	reviewerInst.IsReviewer = true

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(reviewerInst)

	appCfg := config.DefaultConfig()
	appCfg.AutoReviewFix = true

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             appCfg,
		nav:                   list,
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		taskState:             ps,
		taskStateDir:          plansDir,
		fsm:                   newPlanFSMForTest(t, plansDir),
		pendingReviewFeedback: make(map[string]string),
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
	}

	msg := metadataResultMsg{
		Results: []instanceMetadata{
			{Title: reviewerInst.Title, TmuxAlive: false},
		},
		PlanState: ps,
	}

	_, _ = h.Update(msg)

	// Plan must remain in reviewing — tmux death is not an approval signal.
	reloaded, _ := newTestPlanState(t, plansDir)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusReviewing, entry.Status,
		"reviewer tmux death must not auto-approve — plan must stay reviewing")
}

// TestReviewCycle_InstanceTitlesIncludeCycleNumber verifies that review cycle
// numbers are embedded in instance titles. After the first ReviewChangesRequested
// signal the spawned coder gets title "feature-fix-1"; after the subsequent
// ImplementFinished the spawned reviewer gets "feature-review-2".
func TestReviewCycle_InstanceTitlesIncludeCycleNumber(t *testing.T) {
	t.Parallel()
	const planFile = "feature"
	const feedback = "Fix the error handling in auth.go"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	// Write a minimal plan file so spawnTaskAgent helpers can read it.
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, planFile), []byte("# Plan\n## Wave 1\n- Task 1\n"), 0o644))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReviewing)

	// Create a reviewer instance — title uses the old format (no cycle suffix).
	reviewerInst, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-review",
		Path:      dir,
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	})
	require.NoError(t, err)
	reviewerInst.IsReviewer = true

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	_ = list.AddInstance(reviewerInst)

	appCfg := config.DefaultConfig()
	appCfg.AutoReviewFix = true

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             appCfg,
		nav:                   list,
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		taskState:             ps,
		taskStateDir:          plansDir,
		fsm:                   newPlanFSMForTest(t, plansDir),
		pendingReviewFeedback: make(map[string]string),
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
		activeRepoPath:        dir,
		program:               "claude",
	}

	// === Part 1: ReviewChangesRequested → fixer with cycle suffix ===
	signal1 := taskfsm.Signal{
		Event:    taskfsm.ReviewChangesRequested,
		TaskFile: planFile,
		Body:     feedback,
	}
	_, _ = h.Update(metadataResultMsg{
		PlanState: ps,
		Signals:   []taskfsm.Signal{signal1},
	})

	var fixerTitle string
	for _, inst := range h.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.AgentType == session.AgentTypeFixer {
			fixerTitle = inst.Title
			break
		}
	}
	assert.Equal(t, "feature-fix-1", fixerTitle,
		"fixer spawned after first ReviewChangesRequested must have title 'feature-fix-1'")

	// === Part 2: ImplementFinished → reviewer with next cycle suffix ===
	// At this point m.taskState has ReviewCycle=1 in-memory (incremented by part 1).
	// The FSM store has status=implementing (transitioned by part 1).
	signal2 := taskfsm.Signal{
		Event:    taskfsm.ImplementFinished,
		TaskFile: planFile,
	}
	_, _ = h.Update(metadataResultMsg{
		PlanState: h.taskState,
		Signals:   []taskfsm.Signal{signal2},
	})

	// Find the newly spawned reviewer (the old one was killed and removed).
	var reviewerTitle string
	for _, inst := range h.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.IsReviewer {
			reviewerTitle = inst.Title
		}
	}
	assert.Equal(t, "feature-review-2", reviewerTitle,
		"reviewer spawned after ImplementFinished with ReviewCycle=1 must have title 'feature-review-2'")
}

// TestReviewCycle_InstanceStructHasCycleSet verifies that the spawned reviewer
// instance has ReviewCycle set from planstate so the field is available for
// display in instance titles and opencode session labels.
// With review_cycle=0 in planstate (initial), the first reviewer gets ReviewCycle=1
// (1-indexed for humans: display value = stored cycle + 1).
func TestReviewCycle_InstanceStructHasCycleSet(t *testing.T) {
	t.Parallel()
	const planFile = "feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	// review_cycle starts at 0 for a new plan — no IncrementReviewCycle call needed.
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             config.DefaultConfig(),
		nav:                   list,
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		taskState:             ps,
		taskStateDir:          plansDir,
		fsm:                   newPlanFSMForTest(t, plansDir),
		pendingReviewFeedback: make(map[string]string),
		plannerPrompted:       make(map[string]bool),
		coderPushPrompted:     make(map[string]bool),
		activeRepoPath:        dir,
		program:               "claude",
	}

	// Transition to reviewing so spawnReviewer can run.
	require.NoError(t, h.fsm.Transition(planFile, taskfsm.ImplementFinished))

	// Call spawnReviewer directly to obtain the created instance.
	_ = h.spawnReviewer(planFile)

	// Find the reviewer instance.
	var reviewerInst *session.Instance
	for _, inst := range h.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.IsReviewer {
			reviewerInst = inst
			break
		}
	}
	require.NotNil(t, reviewerInst, "spawnReviewer must create a reviewer instance")

	// ReviewCycle must be set to cycle+1 (1-indexed display value).
	// With review_cycle=0 in planstate, the first reviewer gets ReviewCycle=1.
	assert.Equal(t, 1, reviewerInst.ReviewCycle,
		"reviewer instance must have ReviewCycle=1 for first review cycle (cycle=0 → display=1)")
}

// TestIsLocked_FinishedLockedWhenDone verifies that the "finished" stage is
// locked when the plan is already done, preventing a spurious FSM error.
func TestIsLocked_FinishedLockedWhenDone(t *testing.T) {
	t.Parallel()
	assert.True(t, isLocked(taskstate.StatusDone, "finished"),
		"finished stage must be locked when plan is already done")
	// Still unlocked for reviewing (the valid trigger).
	assert.False(t, isLocked(taskstate.StatusReviewing, "finished"),
		"finished stage must be unlocked when plan is reviewing")
	// Also unlocked for verifying — master agent may still need to apply verify_approved.
	assert.False(t, isLocked(taskstate.StatusVerifying, "finished"),
		"finished stage must be unlocked when plan is verifying")
}

// TestReviewApproved_PausesReviewerInsteadOfKilling verifies that when a
// ReviewApproved signal is processed, the reviewer instance is kept in the nav
// panel but transitioned to Paused status (not killed/removed), and the plan
// status transitions to done.
func TestReviewApproved_PausesReviewerInsteadOfKilling(t *testing.T) {
	t.Parallel()
	const planFile = "feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReviewing)

	reviewer, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-review-1",
		Path:      dir,
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	})
	require.NoError(t, err)
	reviewer.IsReviewer = true
	reviewer.SetStatus(session.Running)

	master, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-master-1",
		Path:      dir,
		Program:   "opencode",
		TaskFile:  planFile,
		AgentType: session.AgentTypeMaster,
	})
	require.NoError(t, err)
	master.SetStatus(session.Running)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	nav := ui.NewNavigationPanel(&sp)
	_ = nav.AddInstance(reviewer)
	_ = nav.AddInstance(master)

	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          nav,
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		taskState:    ps,
		taskStateDir: plansDir,
		fsm:          newPlanFSMForTest(t, plansDir),
	}

	_, _ = h.Update(metadataResultMsg{
		PlanState: ps,
		Signals: []taskfsm.Signal{{
			Event:    taskfsm.ReviewApproved,
			TaskFile: planFile,
		}},
	})

	var reviewerAfter *session.Instance
	for _, inst := range h.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.IsReviewer {
			reviewerAfter = inst
			break
		}
	}
	require.NotNil(t, reviewerAfter, "reviewer must remain in nav after approval")
	assert.Equal(t, session.Paused, reviewerAfter.Status)

	var masterAfter *session.Instance
	for _, inst := range h.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.AgentType == session.AgentTypeMaster {
			masterAfter = inst
			break
		}
	}
	require.NotNil(t, masterAfter, "master must remain in nav after approval")
	assert.Equal(t, session.Paused, masterAfter.Status)

	reloaded, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusDone, entry.Status)
	assert.Equal(t, taskstore.ExecutionState{}, entry.ExecutionState)
}

// TestPausedReviewer_RetainedOnNavigateAway verifies that when the user navigates
// away from a paused reviewer whose plan is done, the reviewer instance remains
// inspectable and is only de-emphasised by presentation state.
func TestPausedReviewer_RetainedOnNavigateAway(t *testing.T) {
	t.Parallel()
	const planFile = "feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusDone)

	reviewer, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-review-1",
		Path:      dir,
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	})
	require.NoError(t, err)
	reviewer.IsReviewer = true
	reviewer.SetStatus(session.Paused)

	other, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-coder",
		Path:      dir,
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeCoder,
	})
	require.NoError(t, err)
	other.SetStatus(session.Running)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	nav := ui.NewNavigationPanel(&sp)
	_ = nav.AddInstance(reviewer)
	_ = nav.AddInstance(other)
	// Register the plan so instances appear in nav rows (required for SelectInstance).
	nav.SetPlans([]ui.PlanDisplay{{Filename: planFile, Status: string(taskstate.StatusDone)}})
	require.True(t, nav.SelectInstance(reviewer))

	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          nav,
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		taskState:    ps,
		allInstances: []*session.Instance{reviewer, other},
	}

	// Simulate user navigating away from paused reviewer.
	require.True(t, h.nav.SelectInstance(other))
	_ = h.instanceChanged()

	var foundNav, foundAll bool
	for _, inst := range h.nav.GetInstances() {
		if inst.Title == "feature-review-1" {
			foundNav = true
		}
	}
	for _, inst := range h.allInstances {
		if inst.Title == "feature-review-1" {
			foundAll = true
		}
	}
	assert.True(t, foundNav)
	assert.True(t, foundAll)
	entry, ok := ps.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, ui.PresentationRetired, ui.DeriveInstancePresentation(reviewer, string(entry.Status), true))
}

func TestDeriveInstancePresentation_StartedReadyDoneSoloAgentStaysActive(t *testing.T) {
	t.Parallel()

	inst := &session.Instance{Title: "solo", Status: session.Ready, SoloAgent: true}
	inst.MarkStartedForTest()

	assert.Equal(t, ui.PresentationActive, ui.DeriveInstancePresentation(inst, string(taskstate.StatusDone), true))
	assert.Equal(t, ui.PresentationActive, ui.DeriveInstancePresentation(inst, string(taskstate.StatusCancelled), true))
}

func TestReviewCycleLimitAction_Kind(t *testing.T) {
	t.Parallel()
	action := loop.ReviewCycleLimitAction{
		PlanFile: "test.md",
		Cycle:    3,
		Limit:    3,
	}
	assert.Equal(t, "review_cycle_limit", action.Kind())
	assert.Equal(t, "test.md", action.PlanFile)
	assert.Equal(t, 3, action.Cycle)
	assert.Equal(t, 3, action.Limit)
}

// TestReviewApproved_NoReviewerNoPanic verifies that a ReviewApproved signal
// with no matching reviewer instance in nav still transitions the FSM to done
// without panicking.
func TestReviewApproved_NoReviewerNoPanic(t *testing.T) {
	t.Parallel()
	const planFile = "feature"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReviewing)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	nav := ui.NewNavigationPanel(&sp)

	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          nav,
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		taskState:    ps,
		taskStateDir: plansDir,
		fsm:          newPlanFSMForTest(t, plansDir),
	}

	// Should not panic even with no reviewer instance in nav.
	require.NotPanics(t, func() {
		_, _ = h.Update(metadataResultMsg{
			PlanState: ps,
			Signals: []taskfsm.Signal{{
				Event:    taskfsm.ReviewApproved,
				TaskFile: planFile,
			}},
		})
	})

	// Plan status must still transition to done.
	reloaded, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusDone, entry.Status)
}

// TestMetadataResultMsg_DeferredDialogReleasedOnMenuDismissal verifies that
// queued deferred dialogs (e.g. planner-ready prompts) are drained on the same
// metadata tick that dismisses a stale context menu. This prevents a one-tick
// delay between menu dismissal and the follow-up prompt appearing.
func TestMetadataResultMsg_DeferredDialogReleasedOnMenuDismissal(t *testing.T) {
	t.Parallel()
	const planFile = "planner-ready-task"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := newTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
	}))
	ps, err := newTestPlanStateWithStore(t, store, plansDir)
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:              context.Background(),
		state:            stateContextMenu,
		appConfig:        config.DefaultConfig(),
		taskState:        ps,
		taskStore:        store,
		taskStoreProject: "test",
		taskStateDir:     plansDir,
		nav:              ui.NewNavigationPanel(&sp),
		menu:             ui.NewMenu(),
		tabbedWindow:     ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:     overlay.NewToastManager(&sp),
		overlays:         overlay.NewManager(),
		activeRepoPath:   dir,
		// tracking fields: menu was opened for a ready task
		contextMenuTaskFile:   planFile,
		contextMenuTaskStatus: taskstate.StatusReady,
		contextMenuTaskPhase:  "",
		// pre-queue a planner-ready dialog so drainDeferredDialogs has work to do
		deferredPlannerDialogs:  []string{planFile},
		plannerPrompted:         make(map[string]bool),
		deferredPlannerToastIDs: make(map[string]string),
	}
	h.overlays.Show(overlay.NewContextMenu([]overlay.ContextMenuItem{
		{Label: "start planning", Action: "start_plan"},
	}))

	// Deliver fresh state where the task has advanced to implementing; this
	// triggers the stale-menu dismissal.
	require.NoError(t, store.Update("test", planFile, taskstore.TaskEntry{
		Filename:       planFile,
		Status:         taskstore.StatusImplementing,
		ExecutionState: taskstore.ExecutionState{Phase: "planned"},
	}))
	freshPs, err := newTestPlanStateWithStore(t, store, plansDir)
	require.NoError(t, err)

	model, _ := h.Update(metadataResultMsg{PlanState: freshPs})
	updated := model.(*home)

	// The stale menu must be dismissed on this tick.
	assert.Contains(t, updated.toastManager.View(), "menu dismissed",
		"stale menu must be dismissed")

	// drainDeferredDialogs fires on the same tick (after dismissal sets
	// state=stateDefault so isUserInOverlay returns false). showPlannerDialog
	// calls confirmAction which sets state=stateConfirm and shows an overlay.
	assert.Equal(t, stateConfirm, updated.state,
		"planner dialog must surface on the same tick as menu dismissal")
	assert.True(t, updated.overlays.IsActive(),
		"confirmation overlay from planner dialog must be active on the same tick")
}
