package loop

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessor_ProcessFSMSignals_ImplementFinished(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/my-plan",
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true})
	signals := []taskfsm.Signal{
		{Event: taskfsm.ImplementFinished, TaskFile: "my-plan.md"},
	}

	actions := p.ProcessFSMSignals(signals)
	require.Len(t, actions, 1)
	spawnReviewer, ok := actions[0].(SpawnReviewerAction)
	require.True(t, ok, "expected SpawnReviewerAction, got %T", actions[0])
	assert.Equal(t, "my-plan.md", spawnReviewer.PlanFile)
}

func TestProcessor_ProcessFSMSignals_ReviewApproved(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/my-plan",
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true})
	signals := []taskfsm.Signal{
		{Event: taskfsm.ReviewApproved, TaskFile: "my-plan.md", Body: "LGTM"},
	}

	actions := p.ProcessFSMSignals(signals)

	// ReviewApprovedAction must always be emitted (carries side-effect obligation).
	var foundApproved, foundPR bool
	for _, a := range actions {
		if ra, ok := a.(ReviewApprovedAction); ok {
			assert.Equal(t, "my-plan.md", ra.PlanFile)
			assert.Equal(t, "LGTM", ra.ReviewBody)
			foundApproved = true
		}
		if pr, ok := a.(CreatePRAction); ok {
			assert.Equal(t, "my-plan.md", pr.PlanFile)
			foundPR = true
		}
	}
	assert.True(t, foundApproved, "expected ReviewApprovedAction")
	// Plan has a branch and no PR URL so CreatePRAction should also be emitted.
	assert.True(t, foundPR, "expected CreatePRAction when plan has branch and no PR yet")
}

func TestProcessor_ProcessFSMSignals_ReviewApproved_NoBranch(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
		Branch:   "", // no branch — PR not eligible
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true})
	signals := []taskfsm.Signal{
		{Event: taskfsm.ReviewApproved, TaskFile: "my-plan.md", Body: "LGTM"},
	}

	actions := p.ProcessFSMSignals(signals)

	// ReviewApprovedAction must be emitted even when no PR will be created.
	var foundApproved, foundPR bool
	for _, a := range actions {
		if _, ok := a.(ReviewApprovedAction); ok {
			foundApproved = true
		}
		if _, ok := a.(CreatePRAction); ok {
			foundPR = true
		}
	}
	assert.True(t, foundApproved, "expected ReviewApprovedAction regardless of branch")
	assert.False(t, foundPR, "expected no CreatePRAction when plan has no branch")
}

func TestProcessor_ProcessFSMSignals_ReviewChangesRequested(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/my-plan",
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true})
	feedback := "fix the error handling in handler.go"
	signals := []taskfsm.Signal{
		{Event: taskfsm.ReviewChangesRequested, TaskFile: "my-plan.md", Body: feedback},
	}

	actions := p.ProcessFSMSignals(signals)
	var foundReviewChanges, foundFixer, foundIncrement bool
	for _, a := range actions {
		if rc, ok := a.(ReviewChangesAction); ok {
			assert.Equal(t, feedback, rc.Feedback)
			foundReviewChanges = true
		}
		if sf, ok := a.(SpawnFixerAction); ok {
			assert.Equal(t, feedback, sf.Feedback)
			foundFixer = true
		}
		if _, ok := a.(IncrementReviewCycleAction); ok {
			foundIncrement = true
		}
	}
	assert.True(t, foundReviewChanges, "expected ReviewChangesAction")
	assert.True(t, foundFixer, "expected SpawnFixerAction")
	assert.True(t, foundIncrement, "expected IncrementReviewCycleAction")
}

func TestProcessor_ProcessFSMSignals_InvalidReviewChangesRequested_HasNoActions(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/my-plan",
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true})
	actions := p.ProcessFSMSignals([]taskfsm.Signal{{
		Event:    taskfsm.ReviewChangesRequested,
		TaskFile: "my-plan.md",
		Body:     "stale feedback",
	}})

	assert.Empty(t, actions, "invalid review_changes_requested should not emit side-effect actions")
}

func TestProcessor_ProcessFSMSignals_ReviewChangesRequested_AutoReviewFixDisabled(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/my-plan",
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
	signals := []taskfsm.Signal{{Event: taskfsm.ReviewChangesRequested, TaskFile: "my-plan.md", Body: "fix this"}}

	actions := p.ProcessFSMSignals(signals)
	require.Len(t, actions, 1)
	rc, ok := actions[0].(ReviewChangesAction)
	require.True(t, ok, "expected ReviewChangesAction when auto review-fix is disabled")
	assert.Equal(t, "my-plan.md", rc.PlanFile)
	assert.Equal(t, "fix this", rc.Feedback)
}

func TestProcessor_ProcessFSMSignals_PlanStart_EmitsSpawnPlannerAction(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReady,
	}))

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
	actions := p.ProcessFSMSignals([]taskfsm.Signal{
		{Event: taskfsm.PlanStart, TaskFile: "my-plan.md"},
	})

	require.Len(t, actions, 1, "plan_start must emit a single SpawnPlannerAction")
	spawn, ok := actions[0].(SpawnPlannerAction)
	require.True(t, ok, "expected SpawnPlannerAction, got %T", actions[0])
	assert.Equal(t, "my-plan.md", spawn.PlanFile)
	assert.Equal(t, "planner", spawn.PlannerProfile)
	assert.True(t, spawn.Primary)
	assert.False(t, spawn.DraftMode)

	// FSM must have transitioned ready → planning.
	entry, err := store.Get("test", "my-plan.md")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusPlanning, entry.Status)
}

func TestProcessor_ProcessFSMSignals_PlanStart_DraftModeEmitsClearAndMultiplePlanners(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReady,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner", "planner-alt"},
	})
	actions := p.ProcessFSMSignals([]taskfsm.Signal{
		{Event: taskfsm.PlanStart, TaskFile: "my-plan.md"},
	})

	// Expect: ClearPlannerDraftsAction + SpawnPlannerAction for each profile.
	require.Len(t, actions, 3)
	clear, ok := actions[0].(ClearPlannerDraftsAction)
	require.True(t, ok, "expected ClearPlannerDraftsAction first, got %T", actions[0])
	assert.Equal(t, "my-plan.md", clear.PlanFile)

	spawnPrimary, ok := actions[1].(SpawnPlannerAction)
	require.True(t, ok, "expected SpawnPlannerAction second, got %T", actions[1])
	assert.Equal(t, "my-plan.md", spawnPrimary.PlanFile)
	assert.Equal(t, "planner", spawnPrimary.PlannerProfile)
	assert.True(t, spawnPrimary.Primary, "first profile must be primary")
	assert.True(t, spawnPrimary.DraftMode)

	spawnAlt, ok := actions[2].(SpawnPlannerAction)
	require.True(t, ok, "expected SpawnPlannerAction third, got %T", actions[2])
	assert.Equal(t, "my-plan.md", spawnAlt.PlanFile)
	assert.Equal(t, "planner-alt", spawnAlt.PlannerProfile)
	assert.False(t, spawnAlt.Primary, "non-first profile must not be primary")
	assert.True(t, spawnAlt.DraftMode)
}

func TestProcessor_ProcessFSMSignals_PlanStart_PreAppliedRunsWhenAlreadyPlanning(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
	// HTTP-originated signal where the FSM transition was already applied by
	// the taskactions handler — processor must still emit SpawnPlannerAction
	// so the daemon runs the side effect.
	actions := p.ProcessFSMSignals([]taskfsm.Signal{
		{Event: taskfsm.PlanStart, TaskFile: "my-plan.md", PreApplied: true},
	})

	require.Len(t, actions, 1)
	spawn, ok := actions[0].(SpawnPlannerAction)
	assert.True(t, ok, "pre-applied plan_start must still emit SpawnPlannerAction, got %T", actions[0])
	assert.Equal(t, "planner", spawn.PlannerProfile)
	assert.True(t, spawn.Primary)
	assert.False(t, spawn.DraftMode)
}

func TestProcessor_ProcessFSMSignals_PlanStart_PreAppliedDraftModeEmitsClearAndMultiplePlanners(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner", "planner-alt"},
	})
	actions := p.ProcessFSMSignals([]taskfsm.Signal{
		{Event: taskfsm.PlanStart, TaskFile: "my-plan.md", PreApplied: true},
	})

	require.Len(t, actions, 3)
	assert.IsType(t, ClearPlannerDraftsAction{}, actions[0])
	spawnPrimary, ok := actions[1].(SpawnPlannerAction)
	require.True(t, ok)
	assert.True(t, spawnPrimary.Primary)
	assert.True(t, spawnPrimary.DraftMode)
	spawnAlt, ok := actions[2].(SpawnPlannerAction)
	require.True(t, ok)
	assert.False(t, spawnAlt.Primary)
	assert.True(t, spawnAlt.DraftMode)
}

func TestProcessor_ProcessFSMSignals_PlannerFinished(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
	signals := []taskfsm.Signal{
		{Event: taskfsm.PlannerFinished, TaskFile: "my-plan.md"},
	}

	actions := p.ProcessFSMSignals(signals)
	var found bool
	for _, a := range actions {
		if pc, ok := a.(PlannerCompleteAction); ok {
			assert.Equal(t, "my-plan.md", pc.PlanFile)
			found = true
		}
	}
	assert.True(t, found, "expected PlannerCompleteAction")
}

func TestProcessor_ProcessFSMSignals_PlannerFinished_AutoAdvance(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoAdvance: true})
	actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.PlannerFinished, TaskFile: "my-plan.md"}})

	require.Len(t, actions, 2)
	plannerComplete, ok := actions[0].(PlannerCompleteAction)
	require.True(t, ok, "expected PlannerCompleteAction first, got %T", actions[0])
	assert.Equal(t, "my-plan.md", plannerComplete.PlanFile)

	autoImplement, ok := actions[1].(AutoImplementAction)
	require.True(t, ok, "expected AutoImplementAction second, got %T", actions[1])
	assert.Equal(t, "my-plan.md", autoImplement.PlanFile)
}

func TestProcessor_ProcessFSMSignals_SkipIfWaveOrchestratorActive(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusImplementing,
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
	p.SetWaveOrchestratorActive("my-plan.md", true)

	signals := []taskfsm.Signal{
		{Event: taskfsm.ImplementFinished, TaskFile: "my-plan.md"},
	}
	actions := p.ProcessFSMSignals(signals)
	assert.Empty(t, actions, "implement-finished should be suppressed when wave orchestrator active")
}

func TestProcessor_ProcessTaskSignals(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/my-plan",
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
	p.RegisterOrchestrator("my-plan.md", 1, []int{1, 2})

	taskSignals := []taskfsm.TaskSignal{
		{TaskFile: "my-plan.md", TaskNumber: 1, WaveNumber: 1},
	}

	actions := p.ProcessTaskSignals(taskSignals)
	var found bool
	for _, a := range actions {
		if tc, ok := a.(TaskCompleteAction); ok {
			assert.Equal(t, 1, tc.TaskNumber)
			assert.Equal(t, 0, tc.RetryGeneration)
			found = true
		}
	}
	assert.True(t, found, "expected TaskCompleteAction")
}

func TestProcessor_ProcessTaskSignals_RestoresOrchestratorFromStore(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/my-plan",
	}))
	require.NoError(t, store.SetContent("test", "my-plan.md", "# Plan\n\n**Goal:** test\n\n**Architecture:** test\n\n**Tech Stack:** go\n\n**Size:** Small\n\n---\n\n## Wave 1\n\n### Task 1: First\n\nDo the first thing.\n\n### Task 2: Second\n\nDo the second thing."))
	require.NoError(t, store.SetSubtasks("test", "my-plan.md", []taskstore.SubtaskEntry{
		{TaskNumber: 1, Title: "First", Status: taskstore.SubtaskStatusComplete},
		{TaskNumber: 2, Title: "Second", Status: taskstore.SubtaskStatusRunning},
	}))

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
	actions := p.ProcessTaskSignals([]taskfsm.TaskSignal{{
		TaskFile:   "my-plan.md",
		TaskNumber: 2,
		WaveNumber: 1,
	}})

	require.Len(t, actions, 1)
	taskAction, ok := actions[0].(TaskCompleteAction)
	require.True(t, ok)
	assert.Equal(t, 2, taskAction.TaskNumber)

	orch := p.WaveOrchestrator("my-plan.md")
	require.NotNil(t, orch)
	assert.Equal(t, orchestration.WaveStateAllComplete, orch.State())
	assert.True(t, orch.IsTaskComplete(1))
	assert.True(t, orch.IsTaskComplete(2))
}

func TestProcessor_ProcessWaveSignals(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/my-plan",
	})
	store.SetContent("test", "my-plan.md", "# Plan\n\n**Goal:** test\n\n**Architecture:** test\n\n**Tech Stack:** go\n\n**Size:** Small\n\n---\n\n## Wave 1\n\n### Task 1: Test\n\nDo the thing.")

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
	waveSignals := []taskfsm.WaveSignal{
		{TaskFile: "my-plan.md", WaveNumber: 1},
	}

	actions := p.ProcessWaveSignals(waveSignals)
	var found bool
	for _, a := range actions {
		if aw, ok := a.(AdvanceWaveAction); ok {
			assert.Equal(t, 1, aw.Wave)
			found = true
		}
	}
	assert.True(t, found, "expected AdvanceWaveAction")
}

func TestProcessor_ProcessFSMSignals_ReviewCycleLimitReached(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("proj", taskstore.TaskEntry{
		Filename: "test.md",
		Status:   taskstore.StatusReviewing,
	})
	// Increment review_cycle to 2 (the limit).
	store.IncrementReviewCycle("proj", "test.md")
	store.IncrementReviewCycle("proj", "test.md")

	p := NewProcessor(ProcessorConfig{
		AutoReviewFix:      true,
		Store:              store,
		Project:            "proj",
		MaxReviewFixCycles: 2,
	})

	signals := []taskfsm.Signal{
		{Event: taskfsm.ReviewChangesRequested, TaskFile: "test.md", Body: "fix this"},
	}

	actions := p.ProcessFSMSignals(signals)

	var foundLimit, foundIncrement bool
	for _, a := range actions {
		if lim, ok := a.(ReviewCycleLimitAction); ok {
			assert.Equal(t, "test.md", lim.PlanFile)
			assert.Equal(t, 3, lim.Cycle) // current(2) + 1 for the pending increment
			assert.Equal(t, 2, lim.Limit)
			foundLimit = true
		}
		if _, ok := a.(IncrementReviewCycleAction); ok {
			foundIncrement = true
		}
	}
	assert.True(t, foundLimit, "expected ReviewCycleLimitAction when cycle limit reached")
	assert.False(t, foundIncrement, "should not increment review cycle when cycle limit reached")

	// Should NOT have SpawnCoderAction
	for _, a := range actions {
		if _, ok := a.(SpawnCoderAction); ok {
			t.Fatal("should not emit SpawnCoderAction when cycle limit reached")
		}
	}
}

func TestProcessor_HooksAttachedToFSM(t *testing.T) {
	store := taskstore.NewTestStore(t)

	// Build a registry with a single notify hook.
	hookCfgs := []taskfsm.HookConfig{
		{Type: "notify"},
	}
	registry := taskfsm.BuildHookRegistry(hookCfgs)
	require.NotNil(t, registry)
	require.Equal(t, 1, registry.Len())

	// Pass the registry through ProcessorConfig — ensures startup wiring compiles
	// and the field is accepted without error.
	p := NewProcessor(ProcessorConfig{
		Store:   store,
		Project: "test",
		Hooks:   registry,
	})
	require.NotNil(t, p)
	// The FSM should have hooks attached; we verify indirectly: Processor was
	// successfully constructed (no panic) and the FSM field is non-nil.
	assert.NotNil(t, p.fsm, "expected non-nil FSM")
}

func TestProcessor_ProcessFSMSignals_ReviewCycleBelowLimit(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("proj", taskstore.TaskEntry{
		Filename: "test.md",
		Status:   taskstore.StatusReviewing,
	})
	// review_cycle = 0 (below limit of 3)

	p := NewProcessor(ProcessorConfig{
		AutoReviewFix:      true,
		Store:              store,
		Project:            "proj",
		MaxReviewFixCycles: 3,
	})

	signals := []taskfsm.Signal{
		{Event: taskfsm.ReviewChangesRequested, TaskFile: "test.md", Body: "fix this"},
	}

	actions := p.ProcessFSMSignals(signals)

	var foundFixer bool
	for _, a := range actions {
		if _, ok := a.(SpawnFixerAction); ok {
			foundFixer = true
		}
	}
	assert.True(t, foundFixer, "expected SpawnFixerAction when below cycle limit")
}

func TestProcessor_AutoReadinessReview_ReviewApprovedIntercepted(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/my-plan",
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReadinessReview: true})
	signals := []taskfsm.Signal{
		{Event: taskfsm.ReviewApproved, TaskFile: "my-plan.md", Body: "LGTM"},
	}

	actions := p.ProcessFSMSignals(signals)

	// Must emit ReviewApprovedAction first (reviewer side-effects), then SpawnMasterAction.
	require.Len(t, actions, 2)
	ra, ok := actions[0].(ReviewApprovedAction)
	require.True(t, ok, "expected ReviewApprovedAction first, got %T", actions[0])
	assert.Equal(t, "my-plan.md", ra.PlanFile)
	master, ok := actions[1].(SpawnMasterAction)
	require.True(t, ok, "expected SpawnMasterAction second, got %T", actions[1])
	assert.Equal(t, "my-plan.md", master.PlanFile)
}

func TestProcessor_AutoReadinessReview_DisabledPassesThrough(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/my-plan",
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReadinessReview: false})
	signals := []taskfsm.Signal{
		{Event: taskfsm.ReviewApproved, TaskFile: "my-plan.md", Body: "LGTM"},
	}

	actions := p.ProcessFSMSignals(signals)

	var foundApproved bool
	for _, a := range actions {
		if _, ok := a.(ReviewApprovedAction); ok {
			foundApproved = true
		}
	}
	assert.True(t, foundApproved, "expected ReviewApprovedAction when readiness review is disabled")
}

func TestProcessor_VerifyApprovedDroppedOutsideVerifyingStatus(t *testing.T) {
	// verify_approved arriving when the task is not in verifying state must be
	// rejected by the FSM and produce no actions.
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/my-plan",
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReadinessReview: true})
	signals := []taskfsm.Signal{
		{Event: taskfsm.VerifyApproved, TaskFile: "my-plan.md", Body: "ready"},
	}

	actions := p.ProcessFSMSignals(signals)
	assert.Empty(t, actions, "verify_approved outside verifying status must be silently dropped by FSM")
}

func TestProcessor_VerifyApprovedProcessedInVerifyingStatus(t *testing.T) {
	// verify_approved when the task is in verifying state transitions to done.
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusVerifying,
		Branch:   "plan/my-plan",
		ExecutionState: taskstore.ExecutionState{
			ActiveAgentType: session.AgentTypeMaster,
		},
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReadinessReview: true})
	signals := []taskfsm.Signal{
		{Event: taskfsm.VerifyApproved, TaskFile: "my-plan.md", Body: "ready"},
	}

	actions := p.ProcessFSMSignals(signals)

	var foundApproved bool
	for _, a := range actions {
		if _, ok := a.(VerifyApprovedAction); ok {
			foundApproved = true
		}
	}
	assert.True(t, foundApproved, "verify_approved in verifying status must produce VerifyApprovedAction")
}

func TestProcessor_VerifyFailedDroppedOutsideVerifyingStatus(t *testing.T) {
	// verify_failed arriving when the task is not in verifying state must be
	// rejected by the FSM.
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/my-plan",
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
	signals := []taskfsm.Signal{
		{Event: taskfsm.VerifyFailed, TaskFile: "my-plan.md", Body: "issues found"},
	}

	actions := p.ProcessFSMSignals(signals)
	assert.Empty(t, actions, "verify_failed outside verifying status must be silently dropped by FSM")
}

func TestProcessor_VerifyFailedEmitsVerifyFailedAction(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusVerifying,
		Branch:   "plan/my-plan",
		ExecutionState: taskstore.ExecutionState{
			ActiveAgentType: session.AgentTypeMaster,
		},
	})

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true})
	signals := []taskfsm.Signal{
		{Event: taskfsm.VerifyFailed, TaskFile: "my-plan.md", Body: "issues found"},
	}

	actions := p.ProcessFSMSignals(signals)
	var foundVerifyFailed, foundFixer, foundIncrement bool
	for _, a := range actions {
		if vf, ok := a.(VerifyFailedAction); ok {
			assert.Equal(t, "my-plan.md", vf.PlanFile)
			assert.Equal(t, "issues found", vf.Feedback)
			foundVerifyFailed = true
		}
		if sf, ok := a.(SpawnFixerAction); ok {
			assert.Equal(t, "issues found", sf.Feedback)
			foundFixer = true
		}
		if _, ok := a.(IncrementReviewCycleAction); ok {
			foundIncrement = true
		}
	}
	assert.True(t, foundVerifyFailed, "expected VerifyFailedAction")
	assert.True(t, foundFixer, "expected SpawnFixerAction")
	assert.True(t, foundIncrement, "expected IncrementReviewCycleAction")
}

// TestProcessor_VerifyFailed_ReadinessLoopCapForcePromotes verifies that when
// the readiness verify-loop cap is reached, a VerifyFailed signal is promoted
// to VerifyApproved before the FSM transition — so the task moves verifying→
// done instead of verifying→implementing, and no fixer is spawned.
func TestProcessor_VerifyFailed_ReadinessLoopCapForcePromotes(t *testing.T) {
	cases := []struct {
		name        string
		cap         int
		reviewCycle int
		wantPromote bool
	}{
		{"first attempt below cap", 2, 0, false},
		{"cap reached on first attempt", 1, 0, true},
		{"cap reached on second attempt", 2, 1, true},
		{"cap exceeded on third attempt", 2, 2, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := taskstore.NewTestStore(t)
			store.Create("test", taskstore.TaskEntry{
				Filename: "my-plan.md",
				Status:   taskstore.StatusVerifying,
				Branch:   "plan/my-plan",
				ExecutionState: taskstore.ExecutionState{
					ActiveAgentType: session.AgentTypeMaster,
				},
				ReviewCycle: tc.reviewCycle,
			})

			p := NewProcessor(ProcessorConfig{
				Store:                    store,
				Project:                  "test",
				AutoReviewFix:            true,
				AutoReadinessReview:      true,
				ReadinessMaxVerifyCycles: tc.cap,
			})
			actions := p.ProcessFSMSignals([]taskfsm.Signal{
				{Event: taskfsm.VerifyFailed, TaskFile: "my-plan.md", Body: "issues found"},
			})

			var (
				foundApproved, foundFailed, foundFixer, foundIncrement bool
				approved                                               VerifyApprovedAction
			)
			for _, a := range actions {
				switch act := a.(type) {
				case VerifyApprovedAction:
					foundApproved = true
					approved = act
				case VerifyFailedAction:
					foundFailed = true
				case SpawnFixerAction:
					foundFixer = true
				case IncrementReviewCycleAction:
					foundIncrement = true
				}
			}

			if tc.wantPromote {
				assert.True(t, foundApproved, "expected VerifyApprovedAction on cap promotion")
				assert.True(t, approved.ForcePromoted, "promoted action must set ForcePromoted=true")
				assert.False(t, foundFailed, "must not emit VerifyFailedAction on promotion")
				assert.False(t, foundFixer, "must not spawn fixer on promotion")
				assert.False(t, foundIncrement, "must not increment review cycle on promotion")

				entry, err := store.Get("test", "my-plan.md")
				require.NoError(t, err)
				assert.Equal(t, taskstore.StatusDone, entry.Status, "task must transition to done")
			} else {
				assert.False(t, foundApproved, "must not promote below cap")
				assert.True(t, foundFailed, "expected VerifyFailedAction below cap")
				assert.True(t, foundFixer, "expected SpawnFixerAction below cap")
				assert.True(t, foundIncrement, "expected IncrementReviewCycleAction below cap")

				entry, err := store.Get("test", "my-plan.md")
				require.NoError(t, err)
				assert.Equal(t, taskstore.StatusImplementing, entry.Status, "task must transition to implementing")
			}
		})
	}
}

// TestProcessor_VerifyFailed_LoopCapDisabledWhenReadinessOff verifies that
// ReadinessMaxVerifyCycles has no effect when AutoReadinessReview is off —
// the task should follow the normal VerifyFailed → fixer path regardless of
// the configured cap.
func TestProcessor_VerifyFailed_LoopCapDisabledWhenReadinessOff(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename:    "my-plan.md",
		Status:      taskstore.StatusVerifying,
		Branch:      "plan/my-plan",
		ReviewCycle: 5, // well past any reasonable cap
	})

	p := NewProcessor(ProcessorConfig{
		Store:                    store,
		Project:                  "test",
		AutoReviewFix:            true,
		AutoReadinessReview:      false, // readiness gate disabled
		ReadinessMaxVerifyCycles: 2,
	})
	actions := p.ProcessFSMSignals([]taskfsm.Signal{
		{Event: taskfsm.VerifyFailed, TaskFile: "my-plan.md", Body: "issues found"},
	})

	var foundApproved, foundFailed bool
	for _, a := range actions {
		if _, ok := a.(VerifyApprovedAction); ok {
			foundApproved = true
		}
		if _, ok := a.(VerifyFailedAction); ok {
			foundFailed = true
		}
	}
	assert.False(t, foundApproved, "readiness cap must be ignored when AutoReadinessReview is off")
	assert.True(t, foundFailed, "expected VerifyFailedAction when readiness cap is disabled")
}

// TestProcessor_VerifyFailed_LoopCapZeroDisabled verifies that
// ReadinessMaxVerifyCycles = 0 disables the cap entirely.
func TestProcessor_VerifyFailed_LoopCapZeroDisabled(t *testing.T) {
	store := taskstore.NewTestStore(t)
	store.Create("test", taskstore.TaskEntry{
		Filename:    "my-plan.md",
		Status:      taskstore.StatusVerifying,
		Branch:      "plan/my-plan",
		ReviewCycle: 10,
	})

	p := NewProcessor(ProcessorConfig{
		Store:                    store,
		Project:                  "test",
		AutoReviewFix:            true,
		AutoReadinessReview:      true,
		ReadinessMaxVerifyCycles: 0,
	})
	actions := p.ProcessFSMSignals([]taskfsm.Signal{
		{Event: taskfsm.VerifyFailed, TaskFile: "my-plan.md", Body: "issues found"},
	})

	var foundApproved, foundFailed bool
	for _, a := range actions {
		if _, ok := a.(VerifyApprovedAction); ok {
			foundApproved = true
		}
		if _, ok := a.(VerifyFailedAction); ok {
			foundFailed = true
		}
	}
	assert.False(t, foundApproved, "cap=0 must disable force-promotion")
	assert.True(t, foundFailed, "expected VerifyFailedAction when cap is 0")
}

func TestProcessor_SetReadinessReviewConfig(t *testing.T) {
	store := taskstore.NewTestStore(t)
	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
	assert.False(t, p.config.AutoReadinessReview)

	p.SetReadinessReviewConfig(true)
	assert.True(t, p.config.AutoReadinessReview)

	p.SetReadinessReviewConfig(false)
	assert.False(t, p.config.AutoReadinessReview)
}

// TestProcessor_ProcessFSMSignals_PreAppliedHTTPSignals covers the case where
// the HTTP admin handler applied the FSM transition before emitting a gateway
// signal. Each signal-bearing event must still produce its downstream actions
// even though p.fsm.Transition returns "invalid transition" (the task is
// already in the post-event state). See reviewer feedback on
// orchestration/loop/processor.go alreadyApplied path.
func TestProcessor_ProcessFSMSignals_PreAppliedHTTPSignals(t *testing.T) {
	t.Run("implement_finished pre-applied spawns reviewer", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{
			Filename: "my-plan.md",
			Status:   taskstore.StatusReviewing, // already advanced by HTTP handler
			Branch:   "plan/my-plan",
		}))

		p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{
			Event:      taskfsm.ImplementFinished,
			TaskFile:   "my-plan.md",
			PreApplied: true,
		}})

		require.Len(t, actions, 1, "HTTP-applied implement_finished must still spawn the reviewer")
		_, ok := actions[0].(SpawnReviewerAction)
		assert.True(t, ok, "expected SpawnReviewerAction, got %T", actions[0])
	})

	t.Run("review_approved pre-applied emits reviewer side-effects and chains verify_approved", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{
			Filename: "my-plan.md",
			Status:   taskstore.StatusVerifying, // already advanced by HTTP handler
			Branch:   "plan/my-plan",
		}))

		p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{
			Event:      taskfsm.ReviewApproved,
			TaskFile:   "my-plan.md",
			Body:       "LGTM",
			PreApplied: true,
		}})

		var foundApproved, foundVerify, foundPR bool
		for _, a := range actions {
			switch v := a.(type) {
			case ReviewApprovedAction:
				assert.Equal(t, "my-plan.md", v.PlanFile)
				assert.Equal(t, "LGTM", v.ReviewBody)
				foundApproved = true
			case VerifyApprovedAction:
				foundVerify = true
			case CreatePRAction:
				foundPR = true
			}
		}
		assert.True(t, foundApproved, "expected ReviewApprovedAction for pre-applied review_approved")
		assert.True(t, foundVerify, "expected VerifyApprovedAction after chained verify_approved transition from verifying")
		assert.True(t, foundPR, "expected CreatePRAction since branch is set and no PR URL exists yet")

		// Chained FSM transition must have advanced the task from verifying to done.
		updated, err := store.Get("test", "my-plan.md")
		require.NoError(t, err)
		assert.Equal(t, taskstore.StatusDone, updated.Status, "chained verify_approved must move task to done")
	})

	t.Run("review_approved pre-applied routes through readiness gate", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{
			Filename: "my-plan.md",
			Status:   taskstore.StatusVerifying, // HTTP-applied reviewing → verifying
			Branch:   "plan/my-plan",
		}))

		p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReadinessReview: true})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{
			Event:      taskfsm.ReviewApproved,
			TaskFile:   "my-plan.md",
			Body:       "LGTM",
			PreApplied: true,
		}})

		require.Len(t, actions, 2)
		_, firstOK := actions[0].(ReviewApprovedAction)
		assert.True(t, firstOK, "expected ReviewApprovedAction first")
		_, secondOK := actions[1].(SpawnMasterAction)
		assert.True(t, secondOK, "expected SpawnMasterAction second under readiness gate")
	})

	t.Run("review_changes_requested pre-applied spawns fixer", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{
			Filename: "my-plan.md",
			Status:   taskstore.StatusImplementing, // HTTP-applied reviewing → implementing
			Branch:   "plan/my-plan",
		}))

		p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{
			Event:      taskfsm.ReviewChangesRequested,
			TaskFile:   "my-plan.md",
			Body:       "fix the nits",
			PreApplied: true,
		}})

		var foundChanges, foundFixer, foundIncrement bool
		for _, a := range actions {
			switch v := a.(type) {
			case ReviewChangesAction:
				assert.Equal(t, "fix the nits", v.Feedback)
				foundChanges = true
			case SpawnFixerAction:
				assert.Equal(t, "fix the nits", v.Feedback)
				foundFixer = true
			case IncrementReviewCycleAction:
				foundIncrement = true
			}
		}
		assert.True(t, foundChanges, "expected ReviewChangesAction")
		assert.True(t, foundFixer, "expected SpawnFixerAction")
		assert.True(t, foundIncrement, "expected IncrementReviewCycleAction")
	})

	t.Run("verify_approved pre-applied emits verify-approved action and PR", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{
			Filename: "my-plan.md",
			Status:   taskstore.StatusDone, // HTTP-applied verifying → done
			Branch:   "plan/my-plan",
		}))

		p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{
			Event:      taskfsm.VerifyApproved,
			TaskFile:   "my-plan.md",
			Body:       "ready",
			PreApplied: true,
		}})

		var foundApproved, foundPR bool
		for _, a := range actions {
			if _, ok := a.(VerifyApprovedAction); ok {
				foundApproved = true
			}
			if _, ok := a.(CreatePRAction); ok {
				foundPR = true
			}
		}
		assert.True(t, foundApproved, "expected VerifyApprovedAction")
		assert.True(t, foundPR, "expected CreatePRAction when branch is set and no PR URL exists")
	})

	t.Run("verify_failed pre-applied spawns fixer", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{
			Filename: "my-plan.md",
			Status:   taskstore.StatusImplementing, // HTTP-applied verifying → implementing
			Branch:   "plan/my-plan",
		}))

		p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{
			Event:      taskfsm.VerifyFailed,
			TaskFile:   "my-plan.md",
			Body:       "issues found",
			PreApplied: true,
		}})

		var foundVerifyFailed, foundFixer, foundIncrement bool
		for _, a := range actions {
			switch v := a.(type) {
			case VerifyFailedAction:
				assert.Equal(t, "issues found", v.Feedback)
				foundVerifyFailed = true
			case SpawnFixerAction:
				assert.Equal(t, "issues found", v.Feedback)
				foundFixer = true
			case IncrementReviewCycleAction:
				foundIncrement = true
			}
		}
		assert.True(t, foundVerifyFailed, "expected VerifyFailedAction")
		assert.True(t, foundFixer, "expected SpawnFixerAction")
		assert.True(t, foundIncrement, "expected IncrementReviewCycleAction")
	})

	t.Run("verify_failed pre-applied at cap must not force-promote", func(t *testing.T) {
		// Regression: when a verify_failed signal is PreApplied (the HTTP handler
		// already moved verifying → implementing), force-promotion must be
		// skipped. Otherwise the processor would emit VerifyApprovedAction side
		// effects on a task that is persisted in StatusImplementing.
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{
			Filename:    "my-plan.md",
			Status:      taskstore.StatusImplementing, // HTTP-applied verifying → implementing
			Branch:      "plan/my-plan",
			ReviewCycle: 5, // well past the cap
		}))

		p := NewProcessor(ProcessorConfig{
			Store:                    store,
			Project:                  "test",
			AutoReviewFix:            true,
			AutoReadinessReview:      true,
			ReadinessMaxVerifyCycles: 2,
		})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{
			Event:      taskfsm.VerifyFailed,
			TaskFile:   "my-plan.md",
			Body:       "issues found",
			PreApplied: true,
		}})

		var foundApproved, foundFailed bool
		for _, a := range actions {
			if _, ok := a.(VerifyApprovedAction); ok {
				foundApproved = true
			}
			if _, ok := a.(VerifyFailedAction); ok {
				foundFailed = true
			}
		}
		assert.False(t, foundApproved, "must not promote when signal is PreApplied")
		assert.True(t, foundFailed, "expected VerifyFailedAction for PreApplied verify_failed")

		entry, err := store.Get("test", "my-plan.md")
		require.NoError(t, err)
		assert.Equal(t, taskstore.StatusImplementing, entry.Status, "task must remain in implementing")
	})

	t.Run("planner_finished pre-applied spawns architect under auto-advance", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{
			Filename: "my-plan.md",
			Status:   taskstore.StatusReady, // HTTP-applied planning → ready
		}))

		p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoAdvance: true})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{
			Event:      taskfsm.PlannerFinished,
			TaskFile:   "my-plan.md",
			PreApplied: true,
		}})

		require.Len(t, actions, 2)
		_, firstOK := actions[0].(PlannerCompleteAction)
		assert.True(t, firstOK, "expected PlannerCompleteAction first")
		_, secondOK := actions[1].(AutoImplementAction)
		assert.True(t, secondOK, "expected AutoImplementAction second under AutoAdvance")
	})
}

// TestProcessor_ProcessFSMSignals_UnmarkedStaleSignalsAreDropped confirms that
// signals which land when the task happens to be in the post-event state but
// do NOT carry the fsm_applied marker (filesystem bridge, MCP signal_create)
// are still dropped. This preserves the existing drop behavior for stale /
// out-of-order signals — the PreApplied gate is the single switch that
// distinguishes HTTP-originated from other signal sources.
func TestProcessor_ProcessFSMSignals_UnmarkedStaleSignalsAreDropped(t *testing.T) {
	cases := []struct {
		name   string
		event  taskfsm.Event
		status taskstore.Status
	}{
		{"stale planner_finished in ready", taskfsm.PlannerFinished, taskstore.StatusReady},
		{"stale implement_finished in reviewing", taskfsm.ImplementFinished, taskstore.StatusReviewing},
		{"stale review_approved in verifying", taskfsm.ReviewApproved, taskstore.StatusVerifying},
		{"stale review_changes_requested in implementing", taskfsm.ReviewChangesRequested, taskstore.StatusImplementing},
		{"stale verify_approved in done", taskfsm.VerifyApproved, taskstore.StatusDone},
		{"stale verify_failed in implementing", taskfsm.VerifyFailed, taskstore.StatusImplementing},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := taskstore.NewTestStore(t)
			require.NoError(t, store.Create("test", taskstore.TaskEntry{
				Filename: "my-plan.md",
				Status:   tc.status,
				Branch:   "plan/my-plan",
			}))

			p := NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReviewFix: true, AutoAdvance: true})
			actions := p.ProcessFSMSignals([]taskfsm.Signal{{
				Event:    tc.event,
				TaskFile: "my-plan.md",
				// PreApplied intentionally left false
			}})

			assert.Empty(t, actions, "unmarked stale signal must be dropped even when task is in the post-event target state")
		})
	}
}

// TestProcessor_ProcessPlannerDraftSignals_LegacyModeIgnoresSignals verifies
// that when PlannerDraftMode is false, planner_draft_finished signals are ignored.
func TestProcessor_ProcessPlannerDraftSignals_LegacyModeIgnoresSignals(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{Store: store, Project: "test"})
	actions := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})
	assert.Empty(t, actions, "draft signals must be ignored in legacy mode")
}

// TestProcessor_ProcessPlannerDraftSignals_EmptyProfilesIgnoresSignals verifies
// that when PlannerProfiles is empty (even with DraftMode), signals are ignored.
func TestProcessor_ProcessPlannerDraftSignals_EmptyProfilesIgnoresSignals(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{}, // empty
	})
	actions := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})
	assert.Empty(t, actions, "draft signals must be ignored when no profiles configured")
}

// TestProcessor_ProcessPlannerDraftSignals_UnknownProfileIsIgnored verifies that
// a signal from an unrecognized planner profile produces no actions.
func TestProcessor_ProcessPlannerDraftSignals_UnknownProfileIsIgnored(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner", "planner-alt"},
	})
	actions := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "unknown-planner"},
	})
	assert.Empty(t, actions, "unknown profile must not produce actions")
}

// TestProcessor_ProcessPlannerDraftSignals_DuplicateProfileIsIgnored verifies
// that a duplicate signal for an already-received profile produces no actions.
func TestProcessor_ProcessPlannerDraftSignals_DuplicateProfileIsIgnored(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner", "planner-alt"},
	})

	// First signal — not yet complete (still waiting for planner-alt).
	actions := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})
	assert.Empty(t, actions, "first signal alone must not trigger synthesis")

	// Duplicate signal — must be ignored.
	actions = p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})
	assert.Empty(t, actions, "duplicate signal must produce no actions")
}

// TestProcessor_ProcessPlannerDraftSignals_AllProfilesReceivedSynthesizesPlannerFinished
// verifies that when all configured profiles emit draft signals, a synthesized
// planner_finished action is produced.
func TestProcessor_ProcessPlannerDraftSignals_AllProfilesReceivedSynthesizesPlannerFinished(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner", "planner-alt"},
	})

	// First profile — incomplete.
	actions := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})
	assert.Empty(t, actions, "only first profile received — must not synthesize yet")

	// Second (last) profile — should trigger synthesis.
	actions = p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner-alt"},
	})
	require.NotEmpty(t, actions, "all profiles received — must synthesize planner_finished")

	var foundPlannerComplete bool
	for _, a := range actions {
		if _, ok := a.(PlannerCompleteAction); ok {
			foundPlannerComplete = true
		}
	}
	assert.True(t, foundPlannerComplete, "expected PlannerCompleteAction from synthesized planner_finished")

	// FSM must have transitioned planning → ready.
	entry, err := store.Get("test", "my-plan.md")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, entry.Status)
}

// TestProcessor_ProcessPlannerDraftSignals_SingleProfileSynthesesImmediately verifies
// that a single configured profile synthesizes planner_finished on the first signal.
func TestProcessor_ProcessPlannerDraftSignals_SingleProfileSynthesesImmediately(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner"},
	})

	actions := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})
	require.NotEmpty(t, actions, "single profile: first signal must trigger synthesis")

	var foundPlannerComplete bool
	for _, a := range actions {
		if _, ok := a.(PlannerCompleteAction); ok {
			foundPlannerComplete = true
		}
	}
	assert.True(t, foundPlannerComplete, "expected PlannerCompleteAction")
}

// TestProcessor_ProcessPlannerDraftSignals_SignalAfterCompletionIgnored verifies
// that a signal for an already-completed plan produces no actions.
func TestProcessor_ProcessPlannerDraftSignals_SignalAfterCompletionIgnored(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner"},
	})

	// Trigger synthesis with the single expected profile.
	p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})

	// A subsequent signal for the same plan must be ignored.
	actions := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})
	assert.Empty(t, actions, "signal after completion must produce no actions")
}

// TestProcessor_ProcessPlannerDraftSignals_AutoAdvanceSynthesis verifies that
// when AutoAdvance is enabled, the synthesized planner_finished also emits
// AutoImplementAction.
func TestProcessor_ProcessPlannerDraftSignals_AutoAdvanceSynthesis(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner"},
		AutoAdvance:      true,
	})

	actions := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})

	require.Len(t, actions, 2, "expected PlannerCompleteAction + AutoImplementAction")
	_, isPlannerComplete := actions[0].(PlannerCompleteAction)
	assert.True(t, isPlannerComplete, "expected PlannerCompleteAction first, got %T", actions[0])
	_, isAutoImpl := actions[1].(AutoImplementAction)
	assert.True(t, isAutoImpl, "expected AutoImplementAction second, got %T", actions[1])
}

// TestProcessor_ResetPlannerDraftAgg_AllowsNewSynthesis verifies the public
// reset is honored by ProcessPlannerDraftSignals on the next signal — this is
// the seam UI replan flows (which bypass ProcessFSMSignals(PlanStart)) use to
// avoid the stale agg.done drop documented in the FSM-path test below.
func TestProcessor_ResetPlannerDraftAgg_AllowsNewSynthesis(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner"},
	})

	// First fan-out completes (single-profile synthesizes immediately).
	first := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})
	require.NotEmpty(t, first)

	// Without reset, a follow-up signal would be dropped on agg.done.
	p.ResetPlannerDraftAgg("my-plan.md")

	// Plan must be in StatusPlanning for the synthesized planner_finished
	// transition to succeed; the UI flow performs PlanStart on the FSM
	// (ready→planning) before calling ResetPlannerDraftAgg.
	require.NoError(t, p.fsm.Transition("my-plan.md", taskfsm.PlanStart))

	second := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})
	require.NotEmpty(t, second, "post-reset signal must be honored")
}

// TestProcessor_ProcessFSMSignals_PlanStart_DraftModeResetsCompletedAggregation
// verifies that re-issuing PlanStart for a plan whose previous draft
// aggregation already completed clears the in-memory state so subsequent
// planner_draft_finished signals are honored instead of dropped.
func TestProcessor_ProcessFSMSignals_PlanStart_DraftModeResetsCompletedAggregation(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner"},
	})

	// First fan-out completes (single profile synthesizes immediately).
	// Synthesis transitions the FSM planning → ready.
	first := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})
	require.NotEmpty(t, first, "first draft signal should synthesize planner_finished")

	// Restart planning: ready → planning, which must clear the stale
	// completed aggregation so the next signal isn't dropped.
	_ = p.ProcessFSMSignals([]taskfsm.Signal{
		{Event: taskfsm.PlanStart, TaskFile: "my-plan.md"},
	})

	// New draft signal must NOT be dropped by stale agg.done.
	second := p.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{
		{TaskFile: "my-plan.md", PlannerID: "planner"},
	})
	require.NotEmpty(t, second, "draft signal after replanning must synthesize planner_finished again")
}

// TestProcessor_ProcessFSMSignals_PlanStart_DraftModeWithSingleProfile verifies
// that draft mode with a single profile emits clear + one planner spawn.
func TestProcessor_ProcessFSMSignals_PlanStart_DraftModeWithSingleProfile(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "my-plan.md",
		Status:   taskstore.StatusReady,
	}))

	p := NewProcessor(ProcessorConfig{
		Store:            store,
		Project:          "test",
		PlannerDraftMode: true,
		PlannerProfiles:  []string{"planner"},
	})
	actions := p.ProcessFSMSignals([]taskfsm.Signal{
		{Event: taskfsm.PlanStart, TaskFile: "my-plan.md"},
	})

	require.Len(t, actions, 2, "draft mode with single profile: clear + one spawn")
	assert.IsType(t, ClearPlannerDraftsAction{}, actions[0])
	spawn, ok := actions[1].(SpawnPlannerAction)
	require.True(t, ok)
	assert.Equal(t, "planner", spawn.PlannerProfile)
	assert.True(t, spawn.Primary)
	assert.True(t, spawn.DraftMode)
}
