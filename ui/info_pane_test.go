package ui

import (
	"strings"
	"time"

	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfoPane_NoInstance(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{HasInstance: false})
	assert.Contains(t, p.String(), "no instance selected")
}

func TestInfoPane_AdHocInstance(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasInstance: true,
		HasPlan:     false,
		Title:       "fix-login-bug",
		Program:     "opencode",
		Branch:      "kas/fix-login-bug",
		Path:        "/home/kas/dev/myapp",
		Created:     "2026-02-25 14:30",
		Status:      "running",
	})
	output := p.String()
	assert.Contains(t, output, "fix-login-bug")
	assert.Contains(t, output, "opencode")
	assert.Contains(t, output, "kas/fix-login-bug")
	assert.Contains(t, output, "running")
	assert.NotContains(t, output, "plan")
}

func TestInfoPane_PlanBoundInstance(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasInstance:     true,
		HasPlan:         true,
		Title:           "my-feature-coder",
		Program:         "claude",
		Branch:          "plan/my-feature",
		Status:          "running",
		PlanName:        "my-feature",
		PlanDescription: "add dark mode toggle",
		PlanStatus:      "implementing",
		ExecutionPhase:  "wave_running",
		ActiveAgentType: "coder",
		ActiveWave:      2,
		PlanTopic:       "ui",
		PlanBranch:      "plan/my-feature",
		PlanCreated:     "2026-02-25",
		AgentType:       "coder",
		WaveNumber:      2,
		TotalWaves:      3,
		TaskNumber:      4,
		TotalTasks:      6,
	})
	output := p.String()
	assert.Contains(t, output, "my-feature")
	assert.Contains(t, output, "add dark mode toggle")
	assert.Contains(t, output, "implementing")
	assert.Contains(t, output, "wave 2 running")
	assert.Contains(t, output, "coder")
}

func TestInfoPane_ArchitectBaselineInstanceUsesLowercaseRole(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasInstance: true,
		HasPlan:     true,
		Title:       "parallel-planner-baseline",
		Status:      "running",
		PlanName:    "parallel-planner-architect",
		AgentType:   "architect-baseline",
	})

	output := p.String()
	plain := stripANSI(output)
	assert.Contains(t, plain, "instance")
	assert.Contains(t, plain, "role")
	assert.Contains(t, plain, "architect baseline")
	assert.NotContains(t, plain, "architect-baseline")
	assert.NotContains(t, plain, "creating blueprint")
	assert.NotContains(t, plain, "architecting")
}

func TestInfoPane_WaveProgress(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasInstance: true,
		HasPlan:     true,
		Title:       "test-coder",
		Program:     "claude",
		Status:      "running",
		PlanName:    "test-plan",
		PlanStatus:  "implementing",
		WaveTasks: []WaveTaskInfo{
			{Number: 1, State: "complete"},
			{Number: 2, State: "running"},
			{Number: 3, State: "pending"},
		},
	})
	output := p.String()
	assert.Contains(t, output, "✓")
	assert.Contains(t, output, "●")
	assert.Contains(t, output, "○")
}

func TestInfoPane_Scrolling(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 5)
	p.SetData(InfoData{
		HasInstance:     true,
		HasPlan:         true,
		Title:           "test",
		Program:         "claude",
		PlanName:        "test",
		PlanDescription: "desc",
		PlanStatus:      "ready",
		WaveTasks: []WaveTaskInfo{
			{Number: 1, State: "complete"},
			{Number: 2, State: "complete"},
			{Number: 3, State: "running"},
			{Number: 4, State: "pending"},
			{Number: 5, State: "pending"},
			{Number: 6, State: "pending"},
			{Number: 7, State: "pending"},
			{Number: 8, State: "pending"},
		},
	})
	before := p.String()
	require.NotEmpty(t, before)
	p.ScrollDown()
	after := p.String()
	assert.NotEqual(t, before, after)
}

func TestInfoPane_PlanSummary(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(60, 30)
	pane.SetData(InfoData{
		IsPlanHeaderSelected: true,
		PlanName:             "my-feature",
		PlanStatus:           "implementing",
		PlanInstanceCount:    3,
		PlanRunningCount:     2,
		PlanReadyCount:       1,
	})

	output := pane.String()
	assert.Contains(t, output, "my-feature")
	assert.Contains(t, output, "implementing")
	assert.Contains(t, output, "3 (2 running, 1 ready)")
	assert.Contains(t, output, "view plan doc")
}

func TestInfoPane_PlanSummaryWithGoalAndLifecycle(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 40)
	now := time.Now()
	pane.SetData(InfoData{
		IsPlanHeaderSelected: true,
		PlanName:             "improved-info-tab",
		PlanStatus:           "implementing",
		PlanBranch:           "plan/improved-info-tab",
		PlanGoal:             "persist subtask statuses and redesign the info pane",
		ExecutionPhase:       "wave_waiting",
		ActiveAgentType:      "coder",
		ActiveWave:           2,
		PlanningAt:           now.Add(-2 * time.Hour),
		ImplementingAt:       now.Add(-1 * time.Hour),
		AllWaveSubtasks: []WaveSubtaskGroup{
			{WaveNumber: 1, Subtasks: []SubtaskDisplay{
				{Number: 1, Title: "schema migration", Status: "complete"},
				{Number: 2, Title: "store methods", Status: "complete"},
			}},
			{WaveNumber: 2, Subtasks: []SubtaskDisplay{
				{Number: 3, Title: "http endpoints", Status: "running"},
				{Number: 4, Title: "UI overhaul", Status: "pending"},
			}},
		},
		CompletedTasks: 2,
		TotalSubtasks:  4,
	})

	output := pane.String()
	assert.Contains(t, output, "persist subtask statuses")
	assert.Contains(t, output, "lifecycle")
	assert.Contains(t, output, "implementing")
	assert.Contains(t, output, "waiting for confirmation")
	assert.Contains(t, output, "active wave")
	assert.Contains(t, output, "2/4")
	assert.Contains(t, output, "schema migration")
	assert.Contains(t, output, "✓")
	assert.Contains(t, output, "●")
	assert.Contains(t, output, "○")
}

func TestInfoPane_InstanceWithTaskAssignment(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance: true,
		HasPlan:     true,
		Title:       "my-feature-coder",
		Status:      "running",
		PlanName:    "my-feature",
		PlanGoal:    "add dark mode toggle",
		PlanStatus:  "implementing",
		AgentType:   "coder",
		TaskNumber:  3,
		TotalTasks:  6,
		TaskTitle:   "http endpoints",
	})

	output := pane.String()
	assert.Contains(t, output, "add dark mode toggle")
	assert.Contains(t, output, "3 of 6: http endpoints")
	assert.Contains(t, output, "view plan doc")
}

func TestInfoPane_InstanceWithResources(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(60, 30)
	pane.SetData(InfoData{
		HasInstance: true,
		Title:       "task 1",
		Status:      "running",
		CPUPercent:  12.5,
		MemMB:       340,
	})

	output := pane.String()
	assert.Contains(t, output, "13%")
	assert.Contains(t, output, "340M")
}

func TestInfoPane_ShowsReviewOutcome(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(60, 40)
	pane.SetData(InfoData{
		IsPlanHeaderSelected: true,
		PlanName:             "test-plan",
		PlanStatus:           "done",
		HasPlan:              true,
		ReviewCycle:          1,
		ReviewOutcome:        "approved",
	})

	output := pane.String()
	assert.Contains(t, output, "review")
	assert.Contains(t, output, "approved")
	assert.Contains(t, output, "cycle")
}

func TestInfoPane_NoReviewSectionWhenOutcomeEmpty(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(60, 40)
	pane.SetData(InfoData{
		IsPlanHeaderSelected: true,
		PlanName:             "test-plan",
		PlanStatus:           "implementing",
		HasPlan:              true,
		ReviewOutcome:        "",
	})

	output := pane.String()
	// Review section must not appear when ReviewOutcome is empty.
	assert.NotContains(t, output, "approved")
}

func TestInfoPane_ShowsCurrentReviewRound(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(60, 40)
	pane.SetData(InfoData{
		IsPlanHeaderSelected: true,
		PlanName:             "test-plan",
		PlanStatus:           "reviewing",
		ExecutionPhase:       "reviewing",
		ActiveAgentType:      "reviewer",
		ActiveRound:          3,
	})

	output := pane.String()
	assert.Contains(t, output, "reviewing round 3")
	assert.Contains(t, output, "reviewer")
	assert.Contains(t, output, "round")
}

// TestRenderCompact_ShowsPlanMetadata verifies that compact output surfaces
// runtime fields (phase, agent, wave, task counters) and does NOT include
// static metadata that is already visible in the sidebar (plan name, branch).
func TestRenderCompact_ShowsPlanMetadata(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		IsPlanHeaderSelected: true,
		PlanName:             "my-feature",
		PlanStatus:           "implementing",
		PlanBranch:           "plan/my-feature",
		ExecutionPhase:       "wave_running",
		ActiveAgentType:      "coder",
		ActiveWave:           1,
		TotalWaves:           3,
	})

	compact := p.RenderCompact(80)
	assert.NotEmpty(t, compact)
	assert.True(t, lipgloss.Width(compact) > 0)
	// Runtime fields must be present.
	assert.Contains(t, compact, "wave 1 running")
	assert.Contains(t, compact, "coder")
	assert.Contains(t, compact, "wave 1/3")
	// Static sidebar fields must be absent.
	assert.NotContains(t, stripANSI(compact), "my-feature")
	assert.NotContains(t, stripANSI(compact), "plan/my-feature")
	assert.NotContains(t, stripANSI(compact), "implementing")
}

func TestRenderCompact_EmptyWhenNoData(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{})

	compact := p.RenderCompact(80)
	assert.Empty(t, compact)
}

// TestRenderCompact_ShowsInstanceTitle verifies that compact output for an
// instance selection contains runtime fields (phase, agent, counters) and does
// NOT contain static fields already visible in the sidebar (title, status).
func TestRenderCompact_ShowsInstanceTitle(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasInstance:     true,
		Title:           "my-plan-coder-1",
		Status:          "running",
		ExecutionPhase:  "fixing",
		ActiveAgentType: "fixer",
		ActiveRound:     2,
		WaveNumber:      1,
		TotalWaves:      2,
		TaskNumber:      3,
		TotalTasks:      5,
	})

	compact := p.RenderCompact(80)
	assert.NotEmpty(t, compact)
	// Runtime fields present.
	assert.Contains(t, compact, "fixing round 2")
	assert.Contains(t, compact, "fixer")
	assert.Contains(t, compact, "wave 1/2")
	assert.Contains(t, compact, "task 3/5")
	// Static sidebar fields absent.
	assert.NotContains(t, stripANSI(compact), "my-plan-coder-1")
	assert.NotContains(t, stripANSI(compact), "running")
}

// TestRenderCompact_ReviewingRoundNoDuplicate verifies that "reviewing round 2"
// appears exactly once when ExecutionPhase="reviewing" and ActiveRound=2 — the
// standalone round fragment must not be emitted alongside the phase label.
func TestRenderCompact_ReviewingRoundNoDuplicate(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		IsPlanHeaderSelected: true,
		ExecutionPhase:       "reviewing",
		ActiveRound:          2,
	})

	compact := stripANSI(p.RenderCompact(80))
	count := strings.Count(compact, "round 2")
	assert.Equal(t, 1, count, "\"round 2\" must appear exactly once in compact output")
}

// TestRenderCompact_PlanHeaderWaveCounter verifies that a plan-header selection
// with ActiveWave set renders "wave N/M" and never emits "active wave N".
func TestRenderCompact_PlanHeaderWaveCounter(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		IsPlanHeaderSelected: true,
		ActiveWave:           2,
		TotalWaves:           3,
	})

	compact := stripANSI(p.RenderCompact(80))
	assert.Contains(t, compact, "wave 2/3")
	assert.NotContains(t, compact, "active wave 2")
}

// TestInfoPane_WaveLocalTaskCounters verifies that compact output uses wave-local
// task counters (WaveTaskIndex/WaveTaskCount) when both are set.
func TestInfoPane_WaveLocalTaskCounters(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasInstance:   true,
		HasPlan:       true,
		Title:         "my-feature-coder-1",
		Status:        "running",
		PlanName:      "my-feature",
		WaveNumber:    2,
		TotalWaves:    2,
		TaskNumber:    3,
		TotalTasks:    6,
		WaveTaskIndex: 1,
		WaveTaskCount: 2,
	})

	compact := p.RenderCompact(80)
	// wave-local counter preferred over global
	assert.Contains(t, compact, "task 1/2")
	// global counter must not appear
	assert.NotContains(t, compact, "task 3/6")
}

// TestInfoPane_SuppressActiveWaveWhenInstanceHasWaveCounter checks that
// "active wave N" is not emitted alongside an instance-level "wave N/M".
func TestInfoPane_SuppressActiveWaveWhenInstanceHasWaveCounter(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasInstance:   true,
		HasPlan:       true,
		Title:         "my-feature-coder-1",
		Status:        "running",
		PlanName:      "my-feature",
		ActiveWave:    2,
		WaveNumber:    2,
		TotalWaves:    3,
		WaveTaskIndex: 1,
		WaveTaskCount: 2,
	})

	compact := p.RenderCompact(80)
	// wave N/M should appear for the instance
	assert.Contains(t, compact, "wave 2/3")
	// "active wave N" is redundant and must be suppressed
	assert.NotContains(t, compact, "active wave 2")
}

// TestInfoPane_ClampedWaveDenominator verifies the denominator is clamped when
// TotalWaves is stale and smaller than WaveNumber.
func TestInfoPane_ClampedWaveDenominator(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasInstance: true,
		HasPlan:     true,
		Title:       "my-feature-coder",
		Status:      "running",
		PlanName:    "my-feature",
		WaveNumber:  2,
		TotalWaves:  1, // stale — must be clamped to 2
	})

	compact := p.RenderCompact(80)
	assert.Contains(t, compact, "wave 2/2")
	assert.NotContains(t, compact, "wave 2/1")
}

// TestInfoPane_WaveTotalUnknown verifies that when TotalWaves is 0 (unknown),
// the wave counter is shown without a denominator to avoid misleading "wave 2/2".
func TestInfoPane_WaveTotalUnknown(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasInstance: true,
		HasPlan:     true,
		Title:       "my-feature-coder",
		Status:      "running",
		PlanName:    "my-feature",
		WaveNumber:  2,
		TotalWaves:  0, // unknown
	})

	compact := p.RenderCompact(80)
	assert.Contains(t, compact, "wave 2")
	assert.NotContains(t, compact, "wave 2/2")
	assert.NotContains(t, compact, "wave 2/0")
}

// TestInfoPane_TaskTotalUnknown verifies that when TotalTasks is 0 (unknown) and
// WaveTaskIndex is also unknown, only the numerator is shown to avoid a misleading
// "task 3/0" display. This is tested for both the compact and full rendering paths.
func TestInfoPane_TaskTotalUnknown(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 30)
	p.SetData(InfoData{
		HasInstance:   true,
		HasPlan:       true,
		Title:         "my-feature-coder",
		Status:        "running",
		PlanName:      "my-feature",
		TaskNumber:    3,
		TotalTasks:    0, // unknown
		WaveTaskIndex: 0,
		WaveTaskCount: 0,
	})

	// compact path
	compact := p.RenderCompact(80)
	assert.NotContains(t, compact, "task 3/0")
	assert.NotContains(t, compact, "3/0")

	// full rendering path
	full := p.String()
	assert.NotContains(t, full, "3 of 0")
	assert.NotContains(t, full, "3/0")
}

// TestInfoPane_InstanceSectionWaveLocalCounter verifies that the full instance
// section uses wave-local counters instead of the global task position.
func TestInfoPane_InstanceSectionWaveLocalCounter(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:   true,
		HasPlan:       true,
		Title:         "my-feature-coder",
		Status:        "running",
		PlanName:      "my-feature",
		AgentType:     "coder",
		TaskNumber:    3,
		TotalTasks:    6,
		TaskTitle:     "http endpoints",
		WaveTaskIndex: 2,
		WaveTaskCount: 3,
	})

	output := pane.String()
	// wave-local counter should be displayed
	assert.Contains(t, output, "2 of 3: http endpoints")
	// global counter must not appear
	assert.NotContains(t, output, "3 of 6")
}

// TestInfoPane_FallbackToGlobalCounterWhenNoWaveFields verifies legacy instances
// (WaveTaskIndex == 0) still show a sensible global counter.
func TestInfoPane_FallbackToGlobalCounterWhenNoWaveFields(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:   true,
		HasPlan:       true,
		Title:         "my-feature-coder",
		Status:        "running",
		PlanName:      "my-feature",
		TaskNumber:    3,
		TotalTasks:    6,
		TaskTitle:     "http endpoints",
		WaveTaskIndex: 0, // legacy — not set
		WaveTaskCount: 0,
	})

	output := pane.String()
	assert.Contains(t, output, "3 of 6: http endpoints")
}

// TestInfoPane_ReadinessReviewPhase verifies that a task in the readiness_reviewing
// sub-phase shows "readiness review" as the phase label, "master" as the active agent,
// and suppresses the review round counter entirely.
func TestInfoPane_ReadinessReviewPhase(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 40)
	pane.SetData(InfoData{
		IsPlanHeaderSelected: true,
		PlanName:             "auth-feature",
		PlanStatus:           "reviewing",
		ExecutionPhase:       "readiness_reviewing",
		ActiveAgentType:      "master",
		// ActiveRound is set to simulate a non-zero value that must be suppressed.
		ActiveRound: 2,
	})

	output := pane.String()
	assert.Contains(t, output, "readiness review", "phase label must be readiness review")
	assert.Contains(t, output, "master", "active agent must be master")
	// The round counter must NOT appear for readiness review.
	assert.NotContains(t, output, "round", "round counter must be suppressed for readiness review")
}

// TestInfoPane_ReadinessReviewPhaseLabel exercises the infoPhaseLabel helper directly.
func TestInfoPane_ReadinessReviewPhaseLabel(t *testing.T) {
	assert.Equal(t, "readiness review", infoPhaseLabel("readiness_reviewing", 0, 0))
	// activeWave and activeRound are ignored for readiness review
	assert.Equal(t, "readiness review", infoPhaseLabel("readiness_reviewing", 3, 2))
}

// TestInfoPane_VerifyingAtTimeline verifies that VerifyingAt appears between
// reviewing and done in the lifecycle timeline when set.
func TestInfoPane_VerifyingAtTimeline(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 40)

	reviewingTs := time.Date(2025, 3, 10, 10, 0, 0, 0, time.UTC)
	verifyingTs := time.Date(2025, 3, 11, 9, 0, 0, 0, time.UTC)

	pane.SetData(InfoData{
		IsPlanHeaderSelected: true,
		PlanName:             "auth-feature",
		PlanStatus:           "verifying",
		ReviewingAt:          reviewingTs,
		VerifyingAt:          verifyingTs,
	})

	output := pane.String()
	plain := stripANSI(output)
	assert.Contains(t, plain, "verifying", "verifying phase must appear in lifecycle timeline")
	assert.Contains(t, plain, "reviewing", "reviewing phase must appear in lifecycle timeline")

	// The timeline uses "●" for reached phases. Both reviewing and verifying are reached.
	// Find the last occurrence of "reviewing" (timeline row) and last "verifying" (timeline row).
	// Since verifying is listed after reviewing in phases, its last occurrence must be after reviewing's last occurrence.
	lastReviewing := strings.LastIndex(plain, "reviewing")
	lastVerifying := strings.LastIndex(plain, "verifying")
	assert.Greater(t, lastVerifying, lastReviewing,
		"verifying timeline row must appear after reviewing timeline row")
}

// TestInfoPane_VerifyingStatus_SuppressesRoundCounter verifies that the round
// counter is suppressed when the plan is in verifying status.
func TestInfoPane_VerifyingStatus_SuppressesRoundCounter(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 40)
	pane.SetData(InfoData{
		IsPlanHeaderSelected: true,
		PlanName:             "auth-feature",
		PlanStatus:           "verifying",
		ActiveRound:          2,
	})

	output := pane.String()
	assert.NotContains(t, output, "round", "round counter must be suppressed for verifying status")
}

// TestInfoPhaseLabel_TerminalAttempt exercises the infoPhaseLabel helper for
// the terminal-attempt variant of the readiness_reviewing phase.
func TestInfoPhaseLabel_TerminalAttempt(t *testing.T) {
	// Without the optional flag the label is unchanged.
	assert.Equal(t, "readiness review", infoPhaseLabel("readiness_reviewing", 0, 0))
	assert.Equal(t, "readiness review", infoPhaseLabel("readiness_reviewing", 3, 2, false))

	// With terminalAttempt = true the label gains the suffix.
	assert.Equal(t, "readiness review (terminal attempt)", infoPhaseLabel("readiness_reviewing", 0, 0, true))
	// activeWave and activeRound are irrelevant for readiness_reviewing.
	assert.Equal(t, "readiness review (terminal attempt)", infoPhaseLabel("readiness_reviewing", 3, 2, true))

	// Other phases are not affected by the flag.
	assert.Equal(t, "reviewing", infoPhaseLabel("reviewing", 0, 0, true))
	assert.Equal(t, "reviewing round 2", infoPhaseLabel("reviewing", 0, 2, true))
}

// TestInfoPane_TerminalVerifyAttempt_Label verifies that the pane renders
// "readiness review (terminal attempt)" when VerifyRound >= ReadinessMaxVerifyCycles.
func TestInfoPane_TerminalVerifyAttempt_Label(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 40)
	pane.SetData(InfoData{
		IsPlanHeaderSelected:     true,
		PlanName:                 "auth-feature",
		PlanStatus:               "verifying",
		ExecutionPhase:           "readiness_reviewing",
		ActiveAgentType:          "master",
		VerifyRound:              2,
		ReadinessMaxVerifyCycles: 2,
	})

	output := pane.String()
	assert.Contains(t, output, "readiness review (terminal attempt)",
		"pane must show terminal-attempt label when VerifyRound >= cap")
	// Round counter must still be suppressed.
	assert.NotContains(t, output, "round", "round counter must be suppressed in readiness_reviewing phase")
}

// TestInfoPane_NonTerminalVerifyAttempt_Label verifies the label is plain
// "readiness review" when VerifyRound is below the cap or cap is 0.
func TestInfoPane_NonTerminalVerifyAttempt_Label(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 40)

	// Round 1 of cap 2 — not terminal yet.
	pane.SetData(InfoData{
		IsPlanHeaderSelected:     true,
		PlanName:                 "auth-feature",
		PlanStatus:               "verifying",
		ExecutionPhase:           "readiness_reviewing",
		ActiveAgentType:          "master",
		VerifyRound:              1,
		ReadinessMaxVerifyCycles: 2,
	})
	output := pane.String()
	assert.Contains(t, output, "readiness review",
		"pane must show readiness review label")
	assert.NotContains(t, output, "terminal attempt",
		"non-terminal round must not show terminal-attempt suffix")

	// Cap 0 (unlimited) — never terminal.
	pane.SetData(InfoData{
		IsPlanHeaderSelected:     true,
		PlanName:                 "auth-feature",
		PlanStatus:               "verifying",
		ExecutionPhase:           "readiness_reviewing",
		ActiveAgentType:          "master",
		VerifyRound:              5,
		ReadinessMaxVerifyCycles: 0,
	})
	output = pane.String()
	assert.Contains(t, output, "readiness review")
	assert.NotContains(t, output, "terminal attempt")
}

// TestInfoPane_SDKFastInstance verifies that execution mode and speed tier are
// rendered for an sdk-fast instance, and that speed tier is suppressed when
// the execution mode is not sdk.
func TestInfoPane_SDKFastInstance(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:   true,
		Title:         "my-codex-agent",
		Program:       "codex",
		Status:        "running",
		ExecutionMode: "sdk",
		SDKSpeedTier:  "fast",
	})

	output := pane.String()
	assert.Contains(t, output, "execution mode", "execution mode label must appear")
	assert.Contains(t, output, "sdk", "execution mode value must appear")
	assert.Contains(t, output, "speed tier", "speed tier label must appear for sdk-fast")
	assert.Contains(t, output, "fast", "speed tier value must appear")
}

// TestInfoPane_SDKInstance_NoSpeedTierRow verifies that when ExecutionMode is sdk
// but SDKSpeedTier is empty, the speed tier row is not rendered.
func TestInfoPane_SDKInstance_NoSpeedTierRow(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:   true,
		Title:         "my-sdk-agent",
		Program:       "claude",
		Status:        "running",
		ExecutionMode: "sdk",
		SDKSpeedTier:  "",
	})

	output := pane.String()
	assert.Contains(t, output, "execution mode")
	assert.NotContains(t, output, "speed tier", "speed tier must not appear when SDKSpeedTier is empty")
}

// TestInfoPane_TmuxInstance_NoSpeedTierRow verifies that tmux instances do not show speed tier.
func TestInfoPane_TmuxInstance_NoSpeedTierRow(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:   true,
		Title:         "my-tmux-agent",
		Program:       "claude",
		Status:        "running",
		ExecutionMode: "tmux",
		SDKSpeedTier:  "",
	})

	output := pane.String()
	assert.NotContains(t, output, "speed tier", "speed tier must not appear for tmux instances")
}

// TestInfoPane_TranscriptRow_HiddenWhenZero verifies that transcript row is not
// shown when TranscriptBytes is zero.
func TestInfoPane_TranscriptRow_HiddenWhenZero(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:     true,
		Title:           "sdk-agent",
		Status:          "running",
		ExecutionMode:   "sdk",
		TranscriptBytes: 0,
	})

	output := pane.String()
	assert.NotContains(t, output, "transcript", "transcript row must be hidden when bytes is zero")
}

// TestInfoPane_TranscriptRow_HiddenForTmux verifies that the transcript row is
// suppressed for tmux instances even when stats are non-zero.
func TestInfoPane_TranscriptRow_HiddenForTmux(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:     true,
		Title:           "tmux-agent",
		Status:          "running",
		ExecutionMode:   "tmux",
		TranscriptBytes: 1 << 20,
		TranscriptLines: 100,
	})

	output := pane.String()
	assert.NotContains(t, output, "transcript", "transcript row must be hidden for tmux instances")
}

// TestInfoPane_TranscriptRow_BasicStats verifies that transcript row renders
// bytes and lines when evictions and truncations are zero.
func TestInfoPane_TranscriptRow_BasicStats(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:     true,
		Title:           "sdk-agent",
		Status:          "running",
		ExecutionMode:   "sdk",
		TranscriptBytes: 1258291, // ~1.2M
		TranscriptLines: 348,
	})

	output := pane.String()
	plain := stripANSI(output)
	assert.Contains(t, plain, "transcript", "transcript label must appear")
	assert.Contains(t, plain, "lines", "lines count must appear")
	assert.NotContains(t, plain, "evicted", "evicted must not appear when zero")
	assert.NotContains(t, plain, "truncated", "truncated must not appear when zero")
}

// TestInfoPane_TranscriptRow_WithEvictions verifies that the eviction count appears
// when TranscriptEvictedTurns is non-zero.
func TestInfoPane_TranscriptRow_WithEvictions(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:            true,
		Title:                  "sdk-agent",
		Status:                 "running",
		ExecutionMode:          "sdk",
		TranscriptBytes:        2 << 20,
		TranscriptLines:        500,
		TranscriptEvictedTurns: 7,
	})

	output := pane.String()
	plain := stripANSI(output)
	assert.Contains(t, plain, "transcript")
	assert.Contains(t, plain, "7 evicted", "eviction count must appear")
	assert.NotContains(t, plain, "truncated", "truncated must not appear when zero")
}

// TestInfoPane_TranscriptRow_WithTruncations verifies that truncated count
// appears inside the suffix when TranscriptTruncatedRows is non-zero.
func TestInfoPane_TranscriptRow_WithTruncations(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:             true,
		Title:                   "sdk-agent",
		Status:                  "running",
		ExecutionMode:           "sdk",
		TranscriptBytes:         3 << 20,
		TranscriptLines:         600,
		TranscriptEvictedTurns:  5,
		TranscriptTruncatedRows: 2,
	})

	output := pane.String()
	plain := stripANSI(output)
	assert.Contains(t, plain, "transcript")
	assert.Contains(t, plain, "5 evicted")
	assert.Contains(t, plain, "2 truncated")
}

// TestInfoPane_ResourceProfile_Interactive verifies that the "profile" row is
// rendered when ResourceProfile is "interactive".
func TestInfoPane_ResourceProfile_Interactive(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:     true,
		Title:           "my-agent",
		Program:         "claude",
		Status:          "running",
		ExecutionMode:   "sdk",
		ResourceProfile: "interactive",
	})

	output := pane.String()
	plain := stripANSI(output)
	assert.Contains(t, plain, "profile", "profile label must appear for non-normal profile")
	assert.Contains(t, plain, "interactive", "profile value must appear")
}

// TestInfoPane_ResourceProfile_HiddenForNormal verifies that the "profile" row
// is suppressed when ResourceProfile is "normal".
func TestInfoPane_ResourceProfile_HiddenForNormal(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:     true,
		Title:           "my-agent",
		Program:         "claude",
		Status:          "running",
		ExecutionMode:   "sdk",
		ResourceProfile: "normal",
	})

	output := pane.String()
	assert.NotContains(t, output, "profile", "profile row must not appear for normal profile")
}

// TestInfoPane_ResourceProfile_HiddenWhenEmpty verifies that the "profile" row
// is suppressed when ResourceProfile is empty (the default / zero-value).
func TestInfoPane_ResourceProfile_HiddenWhenEmpty(t *testing.T) {
	pane := NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(InfoData{
		HasInstance:   true,
		Title:         "my-agent",
		Program:       "claude",
		Status:        "running",
		ExecutionMode: "sdk",
	})

	output := pane.String()
	assert.NotContains(t, output, "profile", "profile row must not appear when ResourceProfile is empty")
}

// TestFormatBytesShort exercises the byte formatter helper.
func TestFormatBytesShort(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1K"},
		{1536, "2K"},
		{1 << 20, "1.0M"},
		{1258291, "1.2M"},
		{4 << 20, "4.0M"},
	}
	for _, tt := range tests {
		got := formatBytesShort(tt.n)
		assert.Equal(t, tt.want, got, "formatBytesShort(%d)", tt.n)
	}
}
