package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDaemonSyncTestHome(t *testing.T, planFile string) (*home, string) {
	t.Helper()

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store, ps, fsm := newSharedStoreForTest(t, plansDir)
	require.NoError(t, ps.Register(planFile, "stale local metadata", "plan/"+planFile, time.Now()))
	require.NoError(t, ps.ForceSetLifecycle(planFile, taskstate.StatusReady, taskstore.ExecutionState{Phase: "planned"}))

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

	return h, dir
}

func navPlanDisplaysForTest(t *testing.T, nav *ui.NavigationPanel) []ui.PlanDisplay {
	t.Helper()

	field := reflect.ValueOf(nav).Elem().FieldByName("plans")
	require.True(t, field.IsValid(), "navigation panel plans field must exist")

	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface().([]ui.PlanDisplay)
}

func navPlanDisplayForTest(t *testing.T, nav *ui.NavigationPanel, planFile string) ui.PlanDisplay {
	t.Helper()

	for _, plan := range navPlanDisplaysForTest(t, nav) {
		if plan.Filename == planFile {
			return plan
		}
	}

	t.Fatalf("plan %q not found in navigation panel", planFile)
	return ui.PlanDisplay{}
}

func TestDaemonSync_MetadataTickReflectsDaemonTaskStateInSidebar(t *testing.T) {
	tests := []struct {
		name                string
		status              taskstate.Status
		executionState      taskstore.ExecutionState
		reviewCycle         int
		description         string
		topic               string
		expectedSidebarText string
		expectedPlanDisplay ui.PlanDisplay
	}{
		{
			name:                "implementing architecting",
			status:              taskstate.StatusImplementing,
			executionState:      taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseArchitecting), ActiveAgentType: session.AgentTypeElaborator},
			description:         "stale local metadata",
			topic:               "core",
			expectedSidebarText: "feature · architecting",
			expectedPlanDisplay: ui.PlanDisplay{
				Filename:    "feature",
				Status:      string(taskstate.StatusImplementing),
				Description: "stale local metadata",
				Branch:      "plan/feature",
				Topic:       "core",
				Phase:       string(taskfsm.ExecutionPhaseArchitecting),
				AgentType:   session.AgentTypeElaborator,
			},
		},
		{
			name:                "reviewing cycle 2",
			status:              taskstate.StatusReviewing,
			executionState:      taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseReviewing), ActiveAgentType: session.AgentTypeReviewer},
			reviewCycle:         2,
			description:         "stale local metadata",
			topic:               "core",
			expectedSidebarText: "feature · reviewing round 3",
			expectedPlanDisplay: ui.PlanDisplay{
				Filename:    "feature",
				Status:      string(taskstate.StatusReviewing),
				Description: "stale local metadata",
				Branch:      "plan/feature",
				Topic:       "core",
				Phase:       string(taskfsm.ExecutionPhaseReviewing),
				AgentType:   session.AgentTypeReviewer,
				ActiveRound: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const planFile = "feature"

			h, dir := newDaemonSyncTestHome(t, planFile)
			staleEntry, ok := h.taskState.Entry(planFile)
			require.True(t, ok)
			assert.Equal(t, taskstate.StatusReady, staleEntry.Status)
			assert.Equal(t, taskstore.ExecutionState{Phase: "planned"}, staleEntry.ExecutionState)

			staleDisplay := navPlanDisplayForTest(t, h.nav, planFile)
			assert.Equal(t, ui.PlanDisplay{
				Filename:    planFile,
				Status:      string(taskstate.StatusReady),
				Description: "stale local metadata",
				Branch:      "plan/feature",
				Phase:       "planned",
			}, staleDisplay)

			// Simulate daemon writing updated state to the shared SQLite store.
			require.NoError(t, h.taskState.ForceSetLifecycle(planFile, tt.status, tt.executionState))
			if tt.topic != "" {
				require.NoError(t, h.taskState.SetTopic(planFile, tt.topic))
			}
			for i := 0; i < tt.reviewCycle; i++ {
				require.NoError(t, h.taskState.IncrementReviewCycle(planFile))
			}

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

			_, cmd := h.Update(tickUpdateMetadataMessage{})
			require.NotNil(t, cmd)

			msg, ok := cmd().(metadataResultMsg)
			require.True(t, ok)
			assert.True(t, msg.DaemonManagedRepo)
			assert.True(t, msg.DaemonTaskState)
			require.NotNil(t, msg.PlanState)

			entry, ok := msg.PlanState.Entry(planFile)
			require.True(t, ok)
			assert.Equal(t, tt.status, entry.Status)
			assert.Equal(t, tt.executionState, entry.ExecutionState)
			assert.Equal(t, tt.reviewCycle, entry.ReviewCycle)

			model, _ := h.Update(msg)
			updated := model.(*home)

			updatedEntry, ok := updated.taskState.Entry(planFile)
			require.True(t, ok)
			assert.Equal(t, tt.status, updatedEntry.Status)
			assert.Equal(t, tt.executionState, updatedEntry.ExecutionState)
			assert.Equal(t, tt.reviewCycle, updatedEntry.ReviewCycle)

			require.True(t, updated.nav.SelectByID(ui.SidebarPlanPrefix+planFile))
			assert.Contains(t, updated.nav.String(), tt.expectedSidebarText)
			assert.Equal(t, tt.expectedPlanDisplay, navPlanDisplayForTest(t, updated.nav, planFile))
		})
	}
}

func TestDaemonSync_MetadataTickRebuildsWaveOrchestratorForDaemonWaveTask(t *testing.T) {
	const planFile = "feature"

	h, _ := newDaemonSyncTestHome(t, planFile)
	const content = "# Plan\n\n**Goal:** daemon wave restore\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n\n### Task 2: Second\n\nDo second.\n"
	require.NoError(t, h.taskState.IngestContent(planFile, content))
	require.NoError(t, h.taskState.ForceSetLifecycle(planFile, taskstate.StatusImplementing, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	}))

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:      "feature-W1-T1",
		Path:       h.activeRepoPath,
		Program:    "opencode",
		TaskFile:   planFile,
		AgentType:  session.AgentTypeCoder,
		TaskNumber: 1,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	inst.SetStatus(session.Running)

	inst2, err := session.NewInstance(session.InstanceOptions{
		Title:      "feature-W1-T2",
		Path:       h.activeRepoPath,
		Program:    "opencode",
		TaskFile:   planFile,
		AgentType:  session.AgentTypeCoder,
		TaskNumber: 2,
		WaveNumber: 1,
	})
	require.NoError(t, err)
	inst2.MarkStartedForTest()
	inst2.SetStatus(session.Running)

	model, _ := h.Update(metadataResultMsg{
		Results:           []instanceMetadata{{Title: inst.Title, TmuxAlive: true}, {Title: inst2.Title, TmuxAlive: true}},
		PlanState:         h.taskState,
		DaemonTaskState:   true,
		DaemonManagedRepo: true,
		DaemonInstances:   []*session.Instance{inst, inst2},
	})
	updated := model.(*home)

	orch, ok := updated.waveOrchestrators[planFile]
	require.True(t, ok, "daemon-managed wave task should rebuild an orphaned orchestrator")
	assert.Equal(t, 1, orch.CurrentWaveNumber())
	assert.Equal(t, orchestration.WaveStateRunning, orch.State())
	assert.True(t, orch.IsTaskRunning(1), "live daemon wave task should restore as running")
	assert.True(t, orch.IsTaskRunning(2), "other tasks in the active wave should remain runnable")
}

func TestDaemonSync_MetadataResultShowsAllLoadingWaveTasksImmediately(t *testing.T) {
	const planFile = "feature"

	h, _ := newDaemonSyncTestHome(t, planFile)
	const content = "# Plan\n\n**Goal:** daemon wave visibility\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n\n### Task 2: Second\n\nDo second.\n\n### Task 3: Third\n\nDo third.\n"
	require.NoError(t, h.taskState.IngestContent(planFile, content))
	require.NoError(t, h.taskState.ForceSetLifecycle(planFile, taskstate.StatusImplementing, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	}))

	var daemonInstances []*session.Instance
	for _, taskNumber := range []int{1, 2, 3} {
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:      fmt.Sprintf("feature-W1-T%d", taskNumber),
			Path:       h.activeRepoPath,
			Program:    "opencode",
			TaskFile:   planFile,
			AgentType:  session.AgentTypeCoder,
			TaskNumber: taskNumber,
			WaveNumber: 1,
			PeerCount:  3,
		})
		require.NoError(t, err)
		inst.SetStatus(session.Loading)
		daemonInstances = append(daemonInstances, inst)
	}

	model, _ := h.Update(metadataResultMsg{
		PlanState:         h.taskState,
		DaemonTaskState:   true,
		DaemonManagedRepo: true,
		DaemonInstances:   daemonInstances,
	})
	updated := model.(*home)

	assert.Len(t, updated.nav.GetInstances(), 3, "all daemon-tracked loading tasks should be rehydrated immediately")
	require.True(t, updated.nav.SelectByID(ui.SidebarPlanPrefix+planFile))
	rendered := updated.nav.String()
	assert.Contains(t, rendered, "wave 1 · task 1")
	assert.Contains(t, rendered, "wave 1 · task 2")
	assert.Contains(t, rendered, "wave 1 · task 3")
}

func TestDaemonSync_TickSkipsInactiveMissingInstances(t *testing.T) {
	const planFile = "feature"

	h, dir := newDaemonSyncTestHome(t, planFile)

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
		return []api.InstanceStatus{{
			Title:  "feature-planning",
			Plan:   planFile,
			Role:   session.AgentTypePlanner,
			Active: false,
		}}, nil
	}

	_, cmd := h.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)

	msg, ok := cmd().(metadataResultMsg)
	require.True(t, ok)
	assert.True(t, msg.DaemonManagedRepo)
	assert.Equal(t, []string{"feature-planning"}, msg.DaemonTitles)
	assert.Empty(t, msg.DaemonInstances, "inactive daemon instances should not be rehydrated into the sidebar")
}

func TestDaemonSync_TickUpgradesLoadingPlaceholderWhenSessionAppears(t *testing.T) {
	const planFile = "feature"

	h, dir := newDaemonSyncTestHome(t, planFile)

	oldManaged := repoManagedByDaemon
	oldListInstances := listDaemonInstances
	oldRestore := restoreInstanceFromData
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
		listDaemonInstances = oldListInstances
		restoreInstanceFromData = oldRestore
	})

	repoManagedByDaemon = func(repoPath string) bool {
		return filepath.Clean(repoPath) == filepath.Clean(dir)
	}

	phase := 0
	listDaemonInstances = func(project string) ([]api.InstanceStatus, error) {
		require.Equal(t, "test", project)
		status := api.InstanceStatus{
			Title:   "feature-plan",
			Plan:    planFile,
			Role:    session.AgentTypePlanner,
			Active:  true,
			Program: "opencode",
		}
		if phase == 0 {
			status.Loading = true
		}
		return []api.InstanceStatus{status}, nil
	}

	restoreInstanceFromData = func(data session.InstanceData) (*session.Instance, error) {
		if data.Status == session.Loading {
			return nil, fmt.Errorf("tmux not live yet")
		}
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
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return inst, nil
	}

	_, cmd := h.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)
	msg, ok := cmd().(metadataResultMsg)
	require.True(t, ok)
	require.Len(t, msg.DaemonInstances, 1)
	assert.Equal(t, session.Loading, msg.DaemonInstances[0].Status)
	assert.False(t, msg.DaemonInstances[0].Started())

	model, _ := h.Update(msg)
	updated := model.(*home)
	require.Len(t, updated.nav.GetInstances(), 1)
	placeholder := updated.nav.GetInstances()[0]
	assert.Equal(t, session.Loading, placeholder.Status)
	assert.False(t, placeholder.Started())
	assert.False(t, placeholder.Exited)

	phase = 1
	_, cmd = updated.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)
	msg, ok = cmd().(metadataResultMsg)
	require.True(t, ok)
	require.Len(t, msg.DaemonInstances, 1)
	assert.True(t, msg.DaemonInstances[0].Started())
	assert.Equal(t, session.Running, msg.DaemonInstances[0].Status)

	model, _ = updated.Update(msg)
	updated = model.(*home)
	require.Len(t, updated.nav.GetInstances(), 1)
	live := updated.nav.GetInstances()[0]
	assert.True(t, live.Started())
	assert.Equal(t, session.Running, live.Status)
	assert.False(t, live.Exited)
}

func TestDaemonSync_TUIStartedLoadingInstanceNotExpired(t *testing.T) {
	h := newTestHome()
	h.activeRepoPath = t.TempDir()

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:     "kasmos-agent-1",
		Path:      h.activeRepoPath,
		Program:   "claude",
		AgentType: session.AgentTypeFixer,
	})
	require.NoError(t, err)
	inst.SetStatus(session.Loading)
	// Backdate CreatedAt so the instance looks stale (>30s old).
	inst.CreatedAt = time.Now().Add(-60 * time.Second)

	h.nav.AddInstance(inst)
	h.allInstances = append(h.allInstances, inst)
	require.Equal(t, 1, h.nav.TotalInstances())

	// Simulate a daemon sync where the daemon doesn't know about this instance.
	model, _ := h.Update(metadataResultMsg{
		DaemonManagedRepo: true,
		DaemonTitles:      []string{},
		DaemonInstances:   nil,
	})
	updated := model.(*home)
	assert.Equal(t, 1, updated.nav.TotalInstances(),
		"TUI-started loading instance must not be expired just because daemon doesn't know about it")
}

func TestDaemonSync_DeleteDismissedDeadInstanceDoesNotReappear(t *testing.T) {
	h := newTestHome()
	h.nav.SetTopicsAndPlans(nil, []ui.PlanDisplay{{Filename: "feature", Status: string(taskstate.StatusPlanning)}}, nil)
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-planning",
		Path:      t.TempDir(),
		Program:   "opencode",
		TaskFile:  "feature",
		AgentType: session.AgentTypePlanner,
	})
	require.NoError(t, err)
	inst.Exited = true
	inst.SetStatus(session.Ready)
	_ = h.nav.AddInstance(inst)
	require.True(t, h.nav.SelectInstance(inst), "dead plan instance should be selectable before deletion")
	h.allInstances = append(h.allInstances, inst)

	_, _ = h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyDelete})
	assert.Equal(t, 0, h.nav.TotalInstances(), "delete should remove the dead instance immediately")
	assert.True(t, h.isInstanceTitleDismissed(inst.Title), "delete should tombstone the title against daemon re-sync")

	model, _ := h.Update(metadataResultMsg{
		DaemonManagedRepo: true,
		DaemonTitles:      []string{inst.Title},
		DaemonInstances:   []*session.Instance{inst},
	})
	updated := model.(*home)
	assert.Equal(t, 0, updated.nav.TotalInstances(), "dismissed dead instance must not be re-added by daemon sync")

	model, _ = updated.Update(metadataResultMsg{DaemonManagedRepo: true})
	updated = model.(*home)
	assert.False(t, updated.isInstanceTitleDismissed(inst.Title), "tombstone should clear once the daemon stops reporting the title")
	assert.Equal(t, 0, updated.nav.TotalInstances())
}
