package app

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/session/sdk"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const infoPaneTestPlanContent = `# Plan

**Goal:** improve the info tab display

## Wave 1

### Task 1: add goal field
populate goal from store

### Task 2: add lifecycle timestamps
populate planning_at, implementing_at

## Wave 2

### Task 3: add subtask progress
show all-wave subtask progress
`

// buildInfoPaneHome creates a home struct wired with taskState and orchestrator for plan "plan.md".
// The plan has 3 subtasks across 2 waves. Task 1 is complete, tasks 2 and 3 are pending.
func buildInfoPaneHome(t *testing.T) (*home, *taskstate.TaskState, taskstore.Store, *orchestration.WaveOrchestrator) {
	t.Helper()

	dir := t.TempDir()
	store := newTestStore(t)

	ps, err := newTestPlanStateWithStore(t, store, dir)
	require.NoError(t, err)

	const planFile = "plan.md"
	require.NoError(t, ps.Create(planFile, "info tab improvements", "plan/info-tab", "", time.Now()))
	require.NoError(t, ps.IngestContent(planFile, infoPaneTestPlanContent))

	// Set lifecycle timestamps via the store directly.
	planningTs := time.Date(2025, 1, 10, 9, 0, 0, 0, time.UTC)
	implementingTs := time.Date(2025, 1, 12, 10, 30, 0, 0, time.UTC)
	require.NoError(t, store.SetPhaseTimestamp("test", planFile, "planning", planningTs))
	require.NoError(t, store.SetPhaseTimestamp("test", planFile, "implementing", implementingTs))

	// Reload so in-memory entry has the timestamps.
	ps2, err := taskstate.Load(store, "test", dir)
	require.NoError(t, err)

	// Mark task 1 complete via store.
	require.NoError(t, store.UpdateSubtaskStatus("test", planFile, 1, taskstore.SubtaskStatusComplete))

	plan, err := taskparser.Parse(infoPaneTestPlanContent)
	require.NoError(t, err)

	orch := orchestration.NewWaveOrchestrator(planFile, plan)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:               context.Background(),
		state:             stateDefault,
		appConfig:         config.DefaultConfig(),
		nav:               ui.NewNavigationPanel(&sp),
		menu:              ui.NewMenu(),
		auditPane:         ui.NewAuditPane(),
		tabbedWindow:      ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:      overlay.NewToastManager(&sp),
		overlays:          overlay.NewManager(),
		activeRepoPath:    os.TempDir(),
		taskState:         ps2,
		taskStore:         store,
		taskStoreProject:  "test",
		waveOrchestrators: map[string]*orchestration.WaveOrchestrator{planFile: orch},
	}

	// Register plan in nav so selection works.
	h.nav.SetData([]ui.PlanDisplay{{Filename: planFile}}, nil, nil, nil, nil)

	return h, ps2, store, orch
}

// TestUpdateInfoPaneForPlanHeader_GoalPopulated verifies that PlanGoal is set.
func TestUpdateInfoPaneForPlanHeader_GoalPopulated(t *testing.T) {
	t.Parallel()
	h, _, _, _ := buildInfoPaneHome(t)

	ok := h.nav.SelectByID(ui.SidebarPlanPrefix + "plan.md")
	require.True(t, ok, "must be able to select plan header")

	h.updateInfoPaneForPlanHeader()

	data := h.tabbedWindow.GetInfoData()
	assert.Equal(t, "improve the info tab display", data.PlanGoal)
}

func TestUpdateInfoPaneForPlanHeader_PopulatesPersistedPRState(t *testing.T) {
	t.Parallel()
	h, _, store, _ := buildInfoPaneHome(t)
	require.NoError(t, store.SetPRURL("test", "plan.md", "https://github.test/pull/42"))
	require.NoError(t, store.SetPRCreateOutcome("test", "plan.md", taskstore.PRCreateOutcome{
		State: "failed",
		Error: "worktree has uncommitted changes",
	}))
	state, err := taskstate.Load(store, "test", h.activeRepoPath)
	require.NoError(t, err)
	h.taskState = state

	ok := h.nav.SelectByID(ui.SidebarPlanPrefix + "plan.md")
	require.True(t, ok)
	h.updateInfoPaneForPlanHeader()

	data := h.tabbedWindow.GetInfoData()
	assert.Equal(t, "https://github.test/pull/42", data.PRURL)
	assert.Equal(t, "failed", data.PRCreateState)
	assert.Equal(t, "worktree has uncommitted changes", data.PRCreateError)
}

// TestUpdateInfoPaneForPlanHeader_LifecycleTimestamps verifies that PlanningAt and ImplementingAt are set.
func TestUpdateInfoPaneForPlanHeader_LifecycleTimestamps(t *testing.T) {
	t.Parallel()
	h, _, _, _ := buildInfoPaneHome(t)

	ok := h.nav.SelectByID(ui.SidebarPlanPrefix + "plan.md")
	require.True(t, ok)

	h.updateInfoPaneForPlanHeader()

	data := h.tabbedWindow.GetInfoData()
	assert.Equal(t, 2025, data.PlanningAt.Year())
	assert.Equal(t, time.Month(1), data.PlanningAt.Month())
	assert.Equal(t, 10, data.PlanningAt.Day())
	assert.Equal(t, 2025, data.ImplementingAt.Year())
	assert.Equal(t, 12, data.ImplementingAt.Day())
	assert.True(t, data.ReviewingAt.IsZero(), "ReviewingAt must be zero — not set")
}

func TestUpdateInfoPaneForPlanHeader_PopulatesExecutionPhaseMetadata(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		status      taskstate.Status
		state       taskstore.ExecutionState
		wantPhase   string
		wantAgent   string
		wantWave    int
		wantRound   int
		prepareHome func(t *testing.T, h *home)
	}{
		{
			name:   "wave waiting",
			status: taskstate.StatusImplementing,
			state: taskstore.ExecutionState{
				Phase:           "wave_waiting",
				ActiveAgentType: session.AgentTypeCoder,
				ActiveWave:      2,
			},
			wantPhase: "wave_waiting",
			wantAgent: session.AgentTypeCoder,
			wantWave:  2,
		},
		{
			name:   "reviewing round",
			status: taskstate.StatusReviewing,
			state: taskstore.ExecutionState{
				Phase:           "reviewing",
				ActiveAgentType: session.AgentTypeReviewer,
			},
			wantPhase: "reviewing",
			wantAgent: session.AgentTypeReviewer,
			wantRound: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _, _ := buildInfoPaneHome(t)
			require.NoError(t, h.taskState.ForceSetLifecycle("plan.md", tt.status, tt.state))
			if tt.prepareHome != nil {
				tt.prepareHome(t, h)
			}

			ok := h.nav.SelectByID(ui.SidebarPlanPrefix + "plan.md")
			require.True(t, ok)

			h.updateInfoPaneForPlanHeader()

			data := h.tabbedWindow.GetInfoData()
			assert.Equal(t, tt.wantPhase, data.ExecutionPhase)
			assert.Equal(t, tt.wantAgent, data.ActiveAgentType)
			assert.Equal(t, tt.wantWave, data.ActiveWave)
			assert.Equal(t, tt.wantRound, data.ActiveRound)
		})
	}
}

// TestUpdateInfoPaneForPlanHeader_SubtaskProgress verifies CompletedTasks, TotalSubtasks, and AllWaveSubtasks.
func TestUpdateInfoPaneForPlanHeader_SubtaskProgress(t *testing.T) {
	t.Parallel()
	h, _, _, _ := buildInfoPaneHome(t)

	ok := h.nav.SelectByID(ui.SidebarPlanPrefix + "plan.md")
	require.True(t, ok)

	h.updateInfoPaneForPlanHeader()

	data := h.tabbedWindow.GetInfoData()
	assert.Equal(t, 3, data.TotalSubtasks, "must count all 3 subtasks")
	assert.Equal(t, 1, data.CompletedTasks, "task 1 is complete")

	require.Len(t, data.AllWaveSubtasks, 2, "must have 2 wave groups")

	wave1 := data.AllWaveSubtasks[0]
	assert.Equal(t, 1, wave1.WaveNumber)
	require.Len(t, wave1.Subtasks, 2)
	assert.Equal(t, 1, wave1.Subtasks[0].Number)
	assert.Equal(t, "add goal field", wave1.Subtasks[0].Title)
	assert.Equal(t, "complete", wave1.Subtasks[0].Status)
	assert.Equal(t, 2, wave1.Subtasks[1].Number)
	assert.Equal(t, "add lifecycle timestamps", wave1.Subtasks[1].Title)

	wave2 := data.AllWaveSubtasks[1]
	assert.Equal(t, 2, wave2.WaveNumber)
	require.Len(t, wave2.Subtasks, 1)
	assert.Equal(t, 3, wave2.Subtasks[0].Number)
	assert.Equal(t, "add subtask progress", wave2.Subtasks[0].Title)
}

// TestUpdateInfoPane_InstanceView_GoalAndLifecycle verifies goal and timestamps populate in instance view.
func TestUpdateInfoPane_InstanceView_GoalAndLifecycle(t *testing.T) {
	t.Parallel()
	h, _, _, _ := buildInfoPaneHome(t)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "coder-T1",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   "plan.md",
		TaskNumber: 1,
		WaveNumber: 1,
		AgentType:  session.AgentTypeCoder,
	})
	require.NoError(t, err)

	h.nav.SetData([]ui.PlanDisplay{{Filename: "plan.md"}}, []*session.Instance{inst}, nil, nil, map[string]ui.TopicStatus{"plan.md": {}})
	ok := h.nav.SelectInstance(inst)
	require.True(t, ok)

	h.updateInfoPane()

	data := h.tabbedWindow.GetInfoData()
	assert.True(t, data.HasInstance)
	assert.Equal(t, "improve the info tab display", data.PlanGoal)
	assert.False(t, data.PlanningAt.IsZero(), "PlanningAt must be set")
	assert.False(t, data.ImplementingAt.IsZero(), "ImplementingAt must be set")
}

func TestUpdateInfoPane_InstanceView_LinearLink(t *testing.T) {
	t.Parallel()
	h, _, _, _ := buildInfoPaneHome(t)
	require.NoError(t, h.taskState.SetLinearLink("plan.md", taskstore.LinearLink{
		LinearIdentifier: "KAS-123",
		LinearURL:        "https://linear.app/kasmos/issue/KAS-123/info-pane",
	}))

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:    "coder-T1",
		Path:     t.TempDir(),
		Program:  "claude",
		TaskFile: "plan.md",
	})
	require.NoError(t, err)

	h.nav.SetData([]ui.PlanDisplay{{Filename: "plan.md"}}, []*session.Instance{inst}, nil, nil, map[string]ui.TopicStatus{"plan.md": {}})
	require.True(t, h.nav.SelectInstance(inst))

	h.updateInfoPane()

	data := h.tabbedWindow.GetInfoData()
	assert.Equal(t, "KAS-123", data.LinearIdentifier)
	assert.Equal(t, "https://linear.app/kasmos/issue/KAS-123/info-pane", data.LinearURL)
}

// TestUpdateInfoPane_InstanceView_TaskTitle verifies TaskTitle is populated from the plan.
func TestUpdateInfoPane_InstanceView_TaskTitle(t *testing.T) {
	t.Parallel()
	h, _, _, _ := buildInfoPaneHome(t)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "coder-T2",
		Path:       t.TempDir(),
		Program:    "claude",
		TaskFile:   "plan.md",
		TaskNumber: 2,
		WaveNumber: 1,
		AgentType:  session.AgentTypeCoder,
	})
	require.NoError(t, err)

	h.nav.SetData([]ui.PlanDisplay{{Filename: "plan.md"}}, []*session.Instance{inst}, nil, nil, map[string]ui.TopicStatus{"plan.md": {}})
	ok := h.nav.SelectInstance(inst)
	require.True(t, ok)

	h.updateInfoPane()

	data := h.tabbedWindow.GetInfoData()
	assert.Equal(t, "add lifecycle timestamps", data.TaskTitle)
}

// TestUpdateInfoPane_InstanceView_SubtaskProgress verifies subtask progress in instance view.
func TestUpdateInfoPane_InstanceView_SubtaskProgress(t *testing.T) {
	t.Parallel()
	h, _, _, _ := buildInfoPaneHome(t)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:    "coder-T1",
		Path:     t.TempDir(),
		Program:  "claude",
		TaskFile: "plan.md",
	})
	require.NoError(t, err)

	h.nav.SetData([]ui.PlanDisplay{{Filename: "plan.md"}}, []*session.Instance{inst}, nil, nil, map[string]ui.TopicStatus{"plan.md": {}})
	ok := h.nav.SelectInstance(inst)
	require.True(t, ok)

	h.updateInfoPane()

	data := h.tabbedWindow.GetInfoData()
	assert.Equal(t, 3, data.TotalSubtasks)
	assert.Equal(t, 1, data.CompletedTasks)
	assert.Len(t, data.AllWaveSubtasks, 2)
}

// TestUpdateInfoPane_SubtaskReadFailure_PreservesSubtaskFields verifies that when GetSubtasks
// fails, prior subtask fields are preserved and not zeroed out.
func TestUpdateInfoPane_SubtaskReadFailure_PreservesSubtaskFields(t *testing.T) {
	t.Parallel()
	h, _, _, _ := buildInfoPaneHome(t)

	// Replace the taskState's store with one that will fail GetSubtasks.
	// We do this by replacing taskState with a version that has no store.
	// Simplest: build a home with a broken store wrapped in taskState that errors on GetSubtasks.
	failStore := &failingSubtaskStore{inner: newTestStore(t)}
	dir := t.TempDir()
	ps, err := taskstate.Load(failStore, "test", dir)
	require.NoError(t, err)

	// Manually insert plan entry into ps so Entry() returns it.
	ps.Plans = map[string]taskstate.TaskEntry{
		"plan.md": {
			Status: taskstate.StatusImplementing,
			Goal:   "test goal",
		},
	}

	h.taskState = ps

	// Pre-set info data with subtask fields via SetInfoData so the home has prior data.
	prior := ui.InfoData{
		HasInstance:     true,
		TotalSubtasks:   5,
		CompletedTasks:  2,
		AllWaveSubtasks: []ui.WaveSubtaskGroup{{WaveNumber: 1, Subtasks: []ui.SubtaskDisplay{{Number: 1, Title: "old", Status: "complete"}}}},
	}
	h.tabbedWindow.SetInfoData(prior)

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:    "coder",
		Path:     t.TempDir(),
		Program:  "claude",
		TaskFile: "plan.md",
	})
	require.NoError(t, err)

	h.nav.SetData([]ui.PlanDisplay{{Filename: "plan.md"}}, []*session.Instance{inst}, nil, nil, map[string]ui.TopicStatus{"plan.md": {}})
	ok := h.nav.SelectInstance(inst)
	require.True(t, ok)

	h.updateInfoPane()

	data := h.tabbedWindow.GetInfoData()
	// Subtask fields must be preserved from prior data, not zeroed.
	assert.Equal(t, 5, data.TotalSubtasks, "TotalSubtasks must be preserved on GetSubtasks error")
	assert.Equal(t, 2, data.CompletedTasks, "CompletedTasks must be preserved on GetSubtasks error")
	assert.Len(t, data.AllWaveSubtasks, 1, "AllWaveSubtasks must be preserved on GetSubtasks error")
}

// failingSubtaskStore wraps a Store and returns an error on GetSubtasks.
type failingSubtaskStore struct {
	inner taskstore.Store
}

func (f *failingSubtaskStore) Create(project string, entry taskstore.TaskEntry) error {
	return f.inner.Create(project, entry)
}
func (f *failingSubtaskStore) Get(project, filename string) (taskstore.TaskEntry, error) {
	return f.inner.Get(project, filename)
}
func (f *failingSubtaskStore) Update(project, filename string, entry taskstore.TaskEntry) error {
	return f.inner.Update(project, filename, entry)
}
func (f *failingSubtaskStore) Rename(project, oldFilename, newFilename string) error {
	return f.inner.Rename(project, oldFilename, newFilename)
}
func (f *failingSubtaskStore) Delete(project, filename string) error {
	return f.inner.Delete(project, filename)
}
func (f *failingSubtaskStore) GetContent(project, filename string) (string, error) {
	return f.inner.GetContent(project, filename)
}
func (f *failingSubtaskStore) SetContent(project, filename, content string) error {
	return f.inner.SetContent(project, filename, content)
}
func (f *failingSubtaskStore) SetSubtasks(project, filename string, subtasks []taskstore.SubtaskEntry) error {
	return f.inner.SetSubtasks(project, filename, subtasks)
}
func (f *failingSubtaskStore) GetSubtasks(project, filename string) ([]taskstore.SubtaskEntry, error) {
	return nil, fmt.Errorf("subtask store unavailable")
}
func (f *failingSubtaskStore) UpdateSubtaskStatus(project, filename string, taskNumber int, status taskstore.SubtaskStatus) error {
	return f.inner.UpdateSubtaskStatus(project, filename, taskNumber, status)
}
func (f *failingSubtaskStore) SetPhaseTimestamp(project, filename, phase string, ts time.Time) error {
	return f.inner.SetPhaseTimestamp(project, filename, phase, ts)
}
func (f *failingSubtaskStore) SetClickUpTaskID(project, filename, taskID string) error {
	return f.inner.SetClickUpTaskID(project, filename, taskID)
}
func (f *failingSubtaskStore) SetLinearLink(project, filename string, link taskstore.LinearLink) error {
	return f.inner.SetLinearLink(project, filename, link)
}
func (f *failingSubtaskStore) SetLinearLinkIfNoActiveDuplicate(project, filename string, link taskstore.LinearLink, statuses ...taskstore.Status) (string, error) {
	return f.inner.SetLinearLinkIfNoActiveDuplicate(project, filename, link, statuses...)
}
func (f *failingSubtaskStore) ClearLinearLink(project, filename string) error {
	return f.inner.ClearLinearLink(project, filename)
}
func (f *failingSubtaskStore) FindLinkedTask(project, issueID string, statuses ...taskstore.Status) (string, error) {
	return f.inner.FindLinkedTask(project, issueID, statuses...)
}
func (f *failingSubtaskStore) IncrementReviewCycle(project, filename string) error {
	return f.inner.IncrementReviewCycle(project, filename)
}
func (f *failingSubtaskStore) SetPlanGoal(project, filename, goal string) error {
	return f.inner.SetPlanGoal(project, filename, goal)
}
func (f *failingSubtaskStore) List(project string) ([]taskstore.TaskEntry, error) {
	return f.inner.List(project)
}
func (f *failingSubtaskStore) ListByStatus(project string, statuses ...taskstore.Status) ([]taskstore.TaskEntry, error) {
	return f.inner.ListByStatus(project, statuses...)
}
func (f *failingSubtaskStore) ListByTopic(project, topic string) ([]taskstore.TaskEntry, error) {
	return f.inner.ListByTopic(project, topic)
}
func (f *failingSubtaskStore) ListTopics(project string) ([]taskstore.TopicEntry, error) {
	return f.inner.ListTopics(project)
}
func (f *failingSubtaskStore) CreateTopic(project string, entry taskstore.TopicEntry) error {
	return f.inner.CreateTopic(project, entry)
}
func (f *failingSubtaskStore) SetPRURL(project, filename, url string) error {
	return f.inner.SetPRURL(project, filename, url)
}
func (f *failingSubtaskStore) SetPRCreateOutcome(project, filename string, outcome taskstore.PRCreateOutcome) error {
	return f.inner.SetPRCreateOutcome(project, filename, outcome)
}
func (f *failingSubtaskStore) ClearPRCreateOutcome(project, filename string) error {
	return f.inner.ClearPRCreateOutcome(project, filename)
}
func (f *failingSubtaskStore) SetPRState(project, filename, reviewDecision, checkStatus string) error {
	return f.inner.SetPRState(project, filename, reviewDecision, checkStatus)
}
func (f *failingSubtaskStore) RecordPRReview(project, filename string, reviewID int, state, body, reviewer string) error {
	return f.inner.RecordPRReview(project, filename, reviewID, state, body, reviewer)
}
func (f *failingSubtaskStore) IsReviewProcessed(project, filename string, reviewID int) bool {
	return f.inner.IsReviewProcessed(project, filename, reviewID)
}
func (f *failingSubtaskStore) MarkReviewReacted(project, filename string, reviewID int) error {
	return f.inner.MarkReviewReacted(project, filename, reviewID)
}
func (f *failingSubtaskStore) MarkReviewFixerDispatched(project, filename string, reviewID int) error {
	return f.inner.MarkReviewFixerDispatched(project, filename, reviewID)
}
func (f *failingSubtaskStore) ListPendingReviews(project, filename string) ([]taskstore.PRReviewEntry, error) {
	return f.inner.ListPendingReviews(project, filename)
}
func (f *failingSubtaskStore) EnqueueLinearTrigger(project string, e taskstore.LinearTriggerEntry) (int64, bool, error) {
	return f.inner.EnqueueLinearTrigger(project, e)
}
func (f *failingSubtaskStore) MarkLinearTriggerDispatched(project string, id int64, targetFilename string) error {
	return f.inner.MarkLinearTriggerDispatched(project, id, targetFilename)
}
func (f *failingSubtaskStore) MarkLinearTriggerRejected(project string, id int64, reason string) error {
	return f.inner.MarkLinearTriggerRejected(project, id, reason)
}
func (f *failingSubtaskStore) MarkLinearTriggerIgnored(project string, id int64, reason string) error {
	return f.inner.MarkLinearTriggerIgnored(project, id, reason)
}
func (f *failingSubtaskStore) MarkLinearTriggerFailed(project string, id int64, reason string) error {
	return f.inner.MarkLinearTriggerFailed(project, id, reason)
}
func (f *failingSubtaskStore) MarkLinearTriggerAck(project string, id int64, ackState string) error {
	return f.inner.MarkLinearTriggerAck(project, id, ackState)
}
func (f *failingSubtaskStore) ListUnprocessedLinearTriggers(project string, limit int) ([]taskstore.LinearTriggerEntry, error) {
	return f.inner.ListUnprocessedLinearTriggers(project, limit)
}
func (f *failingSubtaskStore) RecordLinearWebhookDelivery(project string, d taskstore.LinearWebhookDelivery) (bool, error) {
	return f.inner.RecordLinearWebhookDelivery(project, d)
}
func (f *failingSubtaskStore) UpdateLinearWebhookDelivery(project, deliveryID, status, reason string) error {
	return f.inner.UpdateLinearWebhookDelivery(project, deliveryID, status, reason)
}
func (f *failingSubtaskStore) LinearWebhookDeliveryByID(project, deliveryID string) (taskstore.LinearWebhookDelivery, error) {
	return f.inner.LinearWebhookDeliveryByID(project, deliveryID)
}
func (f *failingSubtaskStore) ListRecentLinearWebhookDeliveries(project string, limit int) ([]taskstore.LinearWebhookDelivery, error) {
	return f.inner.ListRecentLinearWebhookDeliveries(project, limit)
}
func (f *failingSubtaskStore) LinearWebhookStats(project string, since time.Time) (taskstore.LinearWebhookStats, error) {
	return f.inner.LinearWebhookStats(project, since)
}
func (f *failingSubtaskStore) LastSeenCommentAt(project, linearIssueID string) (time.Time, error) {
	return f.inner.LastSeenCommentAt(project, linearIssueID)
}
func (f *failingSubtaskStore) SetLastSeenCommentAt(project, linearIssueID string, at time.Time) error {
	return f.inner.SetLastSeenCommentAt(project, linearIssueID, at)
}
func (f *failingSubtaskStore) Ping() error  { return f.inner.Ping() }
func (f *failingSubtaskStore) Close() error { return f.inner.Close() }

// TestUpdateInfoPane_SDKFastInstance verifies that updateInfoPane sets ExecutionMode
// and SDKSpeedTier on InfoData from a fast-tier sdk instance.
func TestUpdateInfoPane_SDKFastInstance(t *testing.T) {
	t.Parallel()
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:            context.Background(),
		state:          stateDefault,
		appConfig:      config.DefaultConfig(),
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		auditPane:      ui.NewAuditPane(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&sp),
		overlays:       overlay.NewManager(),
		activeRepoPath: t.TempDir(),
	}

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "fast-codex",
		Path:          t.TempDir(),
		Program:       "codex",
		ExecutionMode: session.ExecutionModeSDK,
		SDKSpeedTier:  "fast",
		AgentType:     session.AgentTypeMaster,
	})
	require.NoError(t, err)

	h.nav.SetData(nil, []*session.Instance{inst}, nil, nil, nil)
	ok := h.nav.SelectInstance(inst)
	require.True(t, ok)

	h.updateInfoPane()

	data := h.tabbedWindow.GetInfoData()
	assert.Equal(t, "sdk", data.ExecutionMode, "ExecutionMode must be set from instance")
	assert.Equal(t, "fast", data.SDKSpeedTier, "SDKSpeedTier must be set from instance")

	// Also verify the rendered output shows both rows
	pane := ui.NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(data)
	output := pane.String()
	assert.Contains(t, output, "execution mode")
	assert.Contains(t, output, "speed tier")
	assert.Contains(t, output, "fast")
}

func TestUpdateInfoPane_ResourceProfileFromSelectedInstance(t *testing.T) {
	t.Parallel()
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:            context.Background(),
		state:          stateDefault,
		appConfig:      config.DefaultConfig(),
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		auditPane:      ui.NewAuditPane(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&sp),
		overlays:       overlay.NewManager(),
		activeRepoPath: t.TempDir(),
	}

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:     "profiled-agent",
		Path:      t.TempDir(),
		Program:   "codex",
		AgentType: session.AgentTypeMaster,
		ResourceControls: config.ResolvedResourceControls{
			Enabled: true,
			Profile: "interactive",
		},
	})
	require.NoError(t, err)

	h.nav.SetData(nil, []*session.Instance{inst}, nil, nil, nil)
	require.True(t, h.nav.SelectInstance(inst))

	h.updateInfoPane()

	data := h.tabbedWindow.GetInfoData()
	assert.Equal(t, "interactive", data.ResourceProfile)

	pane := ui.NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(data)
	output := pane.String()
	assert.Contains(t, output, "profile")
	assert.Contains(t, output, "interactive")
}

// TestUpdateInfoPane_RendererStats verifies that updateInfoPane copies RendererStats
// from the selected instance into the InfoData transcript fields.
func TestUpdateInfoPane_RendererStats(t *testing.T) {
	t.Parallel()
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:            context.Background(),
		state:          stateDefault,
		appConfig:      config.DefaultConfig(),
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		auditPane:      ui.NewAuditPane(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&sp),
		overlays:       overlay.NewManager(),
		activeRepoPath: t.TempDir(),
	}

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "sdk-agent",
		Path:          t.TempDir(),
		Program:       "claude",
		ExecutionMode: session.ExecutionModeSDK,
		AgentType:     session.AgentTypeCoder,
	})
	require.NoError(t, err)

	// Inject cached renderer stats directly.
	inst.SetCachedRendererStats(sdk.RendererStats{
		Bytes:         2 << 20,
		Lines:         400,
		EvictedTurns:  3,
		EvictedLines:  60,
		TruncatedRows: 1,
	})

	h.nav.SetData(nil, []*session.Instance{inst}, nil, nil, nil)
	ok := h.nav.SelectInstance(inst)
	require.True(t, ok)

	h.updateInfoPane()

	data := h.tabbedWindow.GetInfoData()
	assert.Equal(t, int64(2<<20), data.TranscriptBytes, "TranscriptBytes must be set")
	assert.Equal(t, int64(400), data.TranscriptLines, "TranscriptLines must be set")
	assert.Equal(t, int64(3), data.TranscriptEvictedTurns, "TranscriptEvictedTurns must be set")
	assert.Equal(t, int64(60), data.TranscriptEvictedLines, "TranscriptEvictedLines must be set")
	assert.Equal(t, int64(1), data.TranscriptTruncatedRows, "TranscriptTruncatedRows must be set")

	// Also verify the rendered pane shows the transcript row.
	pane := ui.NewInfoPane()
	pane.SetSize(70, 30)
	pane.SetData(data)
	output := pane.String()
	assert.Contains(t, output, "transcript", "transcript row must appear in rendered pane")
	assert.Contains(t, output, "evicted")
	assert.Contains(t, output, "truncated")
}

// TestSDKTranscriptRetentionOptions_WithConfig verifies that sdkTranscriptRetentionOptions
// returns the limits from the active app config and sets configured=true.
func TestSDKTranscriptRetentionOptions_WithConfig(t *testing.T) {
	t.Parallel()
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	cfg := config.DefaultConfig()
	cfg.SDK.TranscriptMaxBytes = 1 << 20
	cfg.SDK.TranscriptMaxTurns = 500
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: cfg,
		nav:       ui.NewNavigationPanel(&sp),
		menu:      ui.NewMenu(),
	}

	maxBytes, maxTurns, configured := h.sdkTranscriptRetentionOptions()
	assert.True(t, configured, "configured must be true when appConfig is set")
	assert.Equal(t, int64(1<<20), maxBytes)
	assert.Equal(t, int64(500), maxTurns)
}

// TestSDKTranscriptRetentionOptions_NilConfig verifies that sdkTranscriptRetentionOptions
// returns configured=false when appConfig is nil, without panicking.
func TestSDKTranscriptRetentionOptions_NilConfig(t *testing.T) {
	t.Parallel()
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: nil,
		nav:       ui.NewNavigationPanel(&sp),
		menu:      ui.NewMenu(),
	}

	_, _, configured := h.sdkTranscriptRetentionOptions()
	assert.False(t, configured, "configured must be false when appConfig is nil")
}

// TestWithRetentionOpts_AppliesLimits verifies that withRetentionOpts sets the
// SDK transcript limit fields on InstanceOptions from the app config.
func TestWithRetentionOpts_AppliesLimits(t *testing.T) {
	t.Parallel()
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	cfg := config.DefaultConfig()
	cfg.SDK.TranscriptMaxBytes = 2 << 20
	cfg.SDK.TranscriptMaxTurns = 250
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: cfg,
		nav:       ui.NewNavigationPanel(&sp),
		menu:      ui.NewMenu(),
	}

	opts := h.withRetentionOpts(session.InstanceOptions{
		Title:   "test",
		Program: "claude",
	})
	assert.True(t, opts.SDKTranscriptLimitsSet, "SDKTranscriptLimitsSet must be set")
	assert.Equal(t, int64(2<<20), opts.SDKTranscriptMaxBytes)
	assert.Equal(t, int64(250), opts.SDKTranscriptMaxTurns)
	// Original fields must survive.
	assert.Equal(t, "test", opts.Title)
	assert.Equal(t, "claude", opts.Program)
}

// TestWithRetentionOpts_NilConfig verifies that withRetentionOpts is a no-op
// when appConfig is nil (SDKTranscriptLimitsSet stays false).
func TestWithRetentionOpts_NilConfig(t *testing.T) {
	t.Parallel()
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: nil,
		nav:       ui.NewNavigationPanel(&sp),
		menu:      ui.NewMenu(),
	}

	opts := h.withRetentionOpts(session.InstanceOptions{Title: "t"})
	assert.False(t, opts.SDKTranscriptLimitsSet, "SDKTranscriptLimitsSet must stay false when no config")
}
