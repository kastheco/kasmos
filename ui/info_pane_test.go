package ui

import (
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

func TestRenderCompact_ShowsPlanMetadata(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasPlan:        true,
		PlanName:       "my-feature",
		PlanStatus:     "implementing",
		PlanBranch:     "plan/my-feature",
		ExecutionPhase: "planned",
	})

	compact := p.RenderCompact(80)
	assert.NotEmpty(t, compact)
	assert.True(t, lipgloss.Width(compact) > 0)
	assert.Contains(t, compact, "view plan [p]")
	assert.Contains(t, compact, "planned")
}

func TestRenderCompact_EmptyWhenNoData(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{})

	compact := p.RenderCompact(80)
	assert.Empty(t, compact)
}

func TestRenderCompact_ShowsInstanceTitle(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasInstance: true,
		Title:       "my-plan-coder-1",
		Status:      "running",
	})

	compact := p.RenderCompact(80)
	assert.NotEmpty(t, compact)
}

func TestRenderCompact_ShowsViewPlanHintForInstance(t *testing.T) {
	p := NewInfoPane()
	p.SetSize(80, 24)
	p.SetData(InfoData{
		HasInstance: true,
		HasPlan:     true,
		Title:       "my-plan-coder-1",
		Status:      "running",
		PlanName:    "my-plan",
	})

	compact := p.RenderCompact(80)
	assert.Contains(t, compact, "view plan [p]")
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
