package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	sessionsdk "github.com/kastheco/kasmos/session/sdk"
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
	// serial: subtests modify repoManagedByDaemon and listDaemonInstances
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

func TestWaitForDaemonPlannerInstanceAcceptsProfileSuffixedTitle(t *testing.T) {
	// serial: mutates daemon instance seam and planner timing globals
	withFastAppTimings(t)
	repoPath := t.TempDir()
	withListDaemonInstances(t, func(project string) ([]api.InstanceStatus, error) {
		require.Equal(t, "test", project)
		return []api.InstanceStatus{{
			Title:   "feature-plan-planner-a",
			Plan:    "feature",
			Role:    session.AgentTypePlanner,
			Active:  true,
			Loading: true,
			Program: "opencode",
		}}, nil
	})

	inst, err := waitForDaemonPlannerInstance("test", session.InstanceData{
		Title:     "feature-plan",
		Path:      repoPath,
		Program:   "opencode",
		TaskFile:  "feature",
		AgentType: session.AgentTypePlanner,
		Status:    session.Loading,
	})
	require.NoError(t, err)
	assert.Equal(t, "feature-plan-planner-a", inst.Title)
}

func TestDaemonSync_DoneTaskDoesNotDeleteInstanceRecord(t *testing.T) {
	t.Parallel()
	const planFile = "feature"

	h, _ := newDaemonSyncTestHome(t, planFile)
	inst := &session.Instance{
		Title:     "feature-W1-T1",
		TaskFile:  planFile,
		AgentType: session.AgentTypeCoder,
		Status:    session.Ready,
	}
	h.addInstanceFinalizer(inst, h.nav.AddInstance(inst))
	h.allInstances = append(h.allInstances, inst)

	require.NoError(t, h.taskState.ForceSetLifecycle(planFile, taskstate.StatusDone, taskstore.ExecutionState{}))
	ps, err := taskstate.Load(h.taskStore, h.taskStoreProject, h.taskStateDir)
	require.NoError(t, err)

	model, _ := h.Update(metadataResultMsg{PlanState: ps})
	updated := model.(*home)

	var foundNav, foundAll bool
	for _, got := range updated.nav.GetInstances() {
		if got.Title == inst.Title {
			foundNav = true
		}
	}
	for _, got := range updated.allInstances {
		if got.Title == inst.Title {
			foundAll = true
		}
	}
	assert.True(t, foundNav, "done-task daemon sync must retain nav instance")
	assert.True(t, foundAll, "done-task daemon sync must retain stored instance")
}

func TestDaemonSync_MetadataTickRebuildsWaveOrchestratorForDaemonWaveTask(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	// serial: modifies repoManagedByDaemon and listDaemonInstances
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
	// serial: modifies repoManagedByDaemon, listDaemonInstances, and restoreInstanceFromData
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

func TestDaemonSync_WaveTaskIndexAndCountPropagatedFromDaemonStatus(t *testing.T) {
	// serial: modifies repoManagedByDaemon, listDaemonInstances, and restoreInstanceFromData
	const planFile = "feature"

	h, dir := newDaemonSyncTestHome(t, planFile)
	require.NoError(t, h.taskState.ForceSetLifecycle(planFile, taskstate.StatusImplementing, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	}))

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
			Title:           "feature-W1-T2",
			Plan:            planFile,
			Role:            session.AgentTypeCoder,
			Active:          true,
			Program:         "opencode",
			TaskNumber:      2,
			WaveNumber:      1,
			WaveTaskIndex:   2,
			WaveTaskCount:   3,
			ResourceProfile: "interactive",
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
			WaveTaskIndex: data.WaveTaskIndex,
			WaveTaskCount: data.WaveTaskCount,
		})
		if err != nil {
			return nil, err
		}
		inst.MarkStartedForTest()
		inst.ResourceProfile = data.ResourceProfile
		inst.SetStatus(session.Running)
		return inst, nil
	}

	// Phase 0: loading placeholder — exercises newDaemonLoadingInstance path.
	_, cmd := h.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)
	msg, ok := cmd().(metadataResultMsg)
	require.True(t, ok)
	require.Len(t, msg.DaemonInstances, 1)

	loading := msg.DaemonInstances[0]
	assert.Equal(t, session.Loading, loading.Status)
	assert.Equal(t, 2, loading.WaveTaskIndex, "loading placeholder must carry WaveTaskIndex")
	assert.Equal(t, 3, loading.WaveTaskCount, "loading placeholder must carry WaveTaskCount")
	assert.Equal(t, "interactive", loading.ResourceProfile, "loading placeholder must carry ResourceProfile")

	model, _ := h.Update(msg)
	updated := model.(*home)
	require.Len(t, updated.nav.GetInstances(), 1)
	placeholder := updated.nav.GetInstances()[0]
	assert.Equal(t, 2, placeholder.WaveTaskIndex)
	assert.Equal(t, 3, placeholder.WaveTaskCount)
	assert.Equal(t, "interactive", placeholder.ResourceProfile)

	// Phase 1: full restore — exercises daemonInstanceData → restoreInstanceFromData path.
	phase = 1
	_, cmd = updated.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)
	msg, ok = cmd().(metadataResultMsg)
	require.True(t, ok)
	require.Len(t, msg.DaemonInstances, 1)

	live := msg.DaemonInstances[0]
	assert.True(t, live.Started())
	assert.Equal(t, session.Running, live.Status)
	assert.Equal(t, 2, live.WaveTaskIndex, "restored instance must carry WaveTaskIndex")
	assert.Equal(t, 3, live.WaveTaskCount, "restored instance must carry WaveTaskCount")
	assert.Equal(t, "interactive", live.ResourceProfile, "restored instance must carry ResourceProfile")
}

func TestDaemonSync_SDKPlaceholderCachesPresentationFromDaemon(t *testing.T) {
	// serial: modifies repoManagedByDaemon and listDaemonInstances
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
			Title:           "sdk-agent",
			Plan:            planFile,
			Role:            session.AgentTypeMaster,
			Active:          true,
			Program:         "codex",
			ExecutionMode:   string(session.ExecutionModeSDK),
			ResourceProfile: "interactive",
		}}, nil
	}

	rawTurns, err := json.Marshal([]*sessionsdk.PresentationTurn{{
		ID:     "t1",
		Number: 1,
		Rows: []sessionsdk.PresentationRow{
			{Kind: sessionsdk.RowResponse},
			{Kind: sessionsdk.RowProse, Text: "daemon structured output"},
		},
	}})
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/repos/test/instances/sdk-agent/presentation", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(api.PresentationResponse{
			Supported:  true,
			Turns:      rawTurns,
			CapturedAt: time.Now().UTC(),
		}))
	})
	startTestDaemonSocketServer(t, mux)

	_, cmd := h.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)
	msg, ok := cmd().(metadataResultMsg)
	require.True(t, ok)

	model, _ := h.Update(msg)
	updated := model.(*home)
	require.Len(t, updated.nav.GetInstances(), 1)
	placeholder := updated.nav.GetInstances()[0]
	assert.False(t, placeholder.Started())
	assert.Equal(t, session.ExecutionModeSDK, session.NormalizeExecutionMode(placeholder.ExecutionMode))
	assert.Equal(t, "interactive", placeholder.ResourceProfile)
	assert.Nil(t, placeholder.CapturePresentation(), "first tick restores the placeholder before metadata is available")

	_, cmd = updated.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)
	msg, ok = cmd().(metadataResultMsg)
	require.True(t, ok)
	require.Len(t, msg.Results, 1)
	assert.True(t, msg.Results[0].PresentationCached)
	require.Len(t, msg.Results[0].PresentationTurns, 1)

	model, _ = updated.Update(msg)
	updated = model.(*home)
	require.Len(t, updated.nav.GetInstances(), 1)
	placeholder = updated.nav.GetInstances()[0]

	turns := placeholder.CapturePresentation()
	require.Len(t, turns, 1)
	require.Len(t, turns[0].Rows, 2)
	assert.Equal(t, "daemon structured output", turns[0].Rows[1].Text)
}

func TestDaemonSync_TUIStartedLoadingInstanceNotExpired(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// TestDaemonSync_SDKPresentation_PreservesNestedFieldsAfterSync asserts that
// daemon placeholder sync preserves tool_diff, tool_preview, and activity
// fields after SetCachedPresentation() and that later reads via
// CapturePresentation() do not alias the cached copy.
func TestDaemonSync_SDKPresentation_PreservesNestedFieldsAfterSync(t *testing.T) {
	// serial: modifies repoManagedByDaemon and listDaemonInstances
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
			Title:         "sdk-agent",
			Plan:          planFile,
			Role:          session.AgentTypeCoder,
			Active:        true,
			Program:       "codex",
			ExecutionMode: string(session.ExecutionModeSDK),
		}}, nil
	}

	oldN, newN := 1, 1
	turns := []*sessionsdk.PresentationTurn{{
		ID:     "t1",
		Number: 1,
		Activity: &sessionsdk.TurnActivity{
			Kind:  "tool",
			Label: "Edit foo.go",
		},
		Rows: []sessionsdk.PresentationRow{
			{
				Kind:     sessionsdk.RowToolDiff,
				ToolName: "Edit",
				ToolDiff: &sessionsdk.ToolDiffPayload{
					Path: "foo.go",
					Lines: []sessionsdk.ToolDiffLine{
						{Kind: sessionsdk.DiffLineRemoved, OldNumber: &oldN, OldText: "old"},
						{Kind: sessionsdk.DiffLineAdded, NewNumber: &newN, NewText: "new"},
					},
				},
			},
			{
				Kind:     sessionsdk.RowToolPreview,
				ToolName: "Bash",
				ToolPreview: &sessionsdk.ToolPreviewPayload{
					Lines: []string{"result output"},
				},
			},
		},
	}}

	rawTurns, err := json.Marshal(turns)
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/repos/test/instances/sdk-agent/presentation", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(api.PresentationResponse{
			Supported:  true,
			Turns:      rawTurns,
			CapturedAt: time.Now().UTC(),
		}))
	})
	startTestDaemonSocketServer(t, mux)

	// First tick: placeholder rehydrated; presentation not yet fetched.
	_, cmd := h.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)
	msg, ok := cmd().(metadataResultMsg)
	require.True(t, ok)
	model, _ := h.Update(msg)
	updated := model.(*home)

	// Second tick: presentation is fetched from the daemon and cached.
	_, cmd = updated.Update(tickUpdateMetadataMessage{})
	require.NotNil(t, cmd)
	msg, ok = cmd().(metadataResultMsg)
	require.True(t, ok)
	require.Len(t, msg.Results, 1)
	require.True(t, msg.Results[0].PresentationCached,
		"second tick must cache presentation from daemon")
	require.Len(t, msg.Results[0].PresentationTurns, 1)

	model, _ = updated.Update(msg)
	updated = model.(*home)
	require.Len(t, updated.nav.GetInstances(), 1)
	placeholder := updated.nav.GetInstances()[0]

	// Verify nested fields survive the full daemon sync round-trip.
	cachedTurns := placeholder.CapturePresentation()
	require.Len(t, cachedTurns, 1)
	turn := cachedTurns[0]

	require.NotNil(t, turn.Activity, "activity must be preserved through daemon sync")
	assert.Equal(t, "tool", turn.Activity.Kind)
	assert.Equal(t, "Edit foo.go", turn.Activity.Label)

	require.Len(t, turn.Rows, 2)
	diffRow := turn.Rows[0]
	require.NotNil(t, diffRow.ToolDiff, "tool_diff must survive daemon sync round-trip")
	assert.Equal(t, "foo.go", diffRow.ToolDiff.Path)
	require.Len(t, diffRow.ToolDiff.Lines, 2)
	assert.Equal(t, sessionsdk.DiffLineRemoved, diffRow.ToolDiff.Lines[0].Kind)
	assert.Equal(t, sessionsdk.DiffLineAdded, diffRow.ToolDiff.Lines[1].Kind)

	previewRow := turn.Rows[1]
	require.NotNil(t, previewRow.ToolPreview, "tool_preview must survive daemon sync round-trip")
	require.Len(t, previewRow.ToolPreview.Lines, 1)
	assert.Equal(t, "result output", previewRow.ToolPreview.Lines[0])

	// Verify no aliasing: mutating the returned slice must not affect a
	// subsequent CapturePresentation() call.
	cachedTurns[0].Rows[0].ToolDiff.Path = "mutated.go"
	cachedTurns[0].Rows[1].ToolPreview.Lines[0] = "mutated"

	fresh := placeholder.CapturePresentation()
	require.Len(t, fresh, 1)
	assert.Equal(t, "foo.go", fresh[0].Rows[0].ToolDiff.Path,
		"CapturePresentation must return independent copies, not aliases of the cache")
	assert.Equal(t, "result output", fresh[0].Rows[1].ToolPreview.Lines[0],
		"CapturePresentation must return independent copies, not aliases of the cache")
}

// TestNewDaemonSDKInstance_CopiesSoloAgentAndSpeedTier verifies that the
// SoloAgent flag and SDKSpeedTier from the daemon's InstanceStatus are
// preserved on the constructed placeholder so TUI restore shows the correct
// nav label and fast-tier info-pane row.
func TestNewDaemonSDKInstance_CopiesSoloAgentAndSpeedTier(t *testing.T) {
	t.Parallel()
	status := api.InstanceStatus{
		Title:         "my-solo",
		Program:       "codex",
		Active:        true,
		ExecutionMode: "sdk",
		SoloAgent:     true,
		SDKSpeedTier:  "fast",
	}
	inst, err := newDaemonSDKInstance(t.TempDir(), status)
	require.NoError(t, err)
	assert.True(t, inst.SoloAgent, "SoloAgent must be copied from daemon status")
	assert.Equal(t, "fast", inst.SDKSpeedTier, "SDKSpeedTier must be copied from daemon status")
}

// TestDaemonInstanceData_CopiesSoloAgentAndSpeedTier verifies that the
// SoloAgent flag and SDKSpeedTier are preserved in the InstanceData returned
// by daemonInstanceData so that restoreInstanceFromData produces matching fields.
func TestDaemonInstanceData_CopiesSoloAgentAndSpeedTier(t *testing.T) {
	t.Parallel()
	status := api.InstanceStatus{
		Title:         "my-solo",
		Program:       "codex",
		Active:        true,
		ExecutionMode: "sdk",
		SoloAgent:     true,
		SDKSpeedTier:  "fast",
	}
	data := daemonInstanceData(t.TempDir(), status)
	assert.True(t, data.SoloAgent, "SoloAgent must be propagated to InstanceData")
	assert.Equal(t, "fast", data.SDKSpeedTier, "SDKSpeedTier must be propagated to InstanceData")
}

// TestDaemonSync_RendererStats_StoredOnInstance verifies that renderer stats
// returned by CapturePresentationFull are stored on the placeholder instance
// via SetCachedRendererStats when processing a metadataResultMsg.
//
// This test uses an in-process fake daemon server so it exercises the full
// CapturePresentationFull → metadataResultMsg → SetCachedRendererStats path.
func TestDaemonSync_RendererStats_StoredOnInstance(t *testing.T) {
	t.Parallel()

	// Build a fake daemon HTTP server that returns a presentation response with stats.
	statsPayload := api.RendererStats{
		Bytes:        2048,
		Lines:        30,
		Turns:        3,
		MaxBytes:     4 << 20,
		MaxTurns:     2000,
		EvictedTurns: 2,
		EvictedBytes: 512,
	}
	turns := json.RawMessage(`[]`)
	presResp := api.PresentationResponse{
		Supported: true,
		Turns:     turns,
		Stats:     &statsPayload,
	}

	srv := newFakePresServer(t, presResp)
	defer srv.Close()

	// Inject a placeholder SDK instance (no local execution session).
	repoDir := t.TempDir()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "sdk-agent",
		Path:          repoDir,
		Program:       "opencode",
		ExecutionMode: session.ExecutionModeSDK,
	})
	require.NoError(t, err)

	// Simulate the metadata update that the polling goroutine produces.
	// stats come from the daemon API response.
	md := instanceMetadata{
		Title:              inst.Title,
		PresentationTurns:  nil,
		PresentationCached: true,
		RendererStats:      rendererStatsFromDaemon(&statsPayload),
		TmuxAlive:          true,
	}
	_ = srv // fake server created for realistic environment; we inject md directly

	h := newTestHome()
	h.activeRepoPath = repoDir
	h.allInstances = append(h.allInstances, inst)
	_ = h.nav.AddInstance(inst)

	// Process the metadata result which should call SetCachedRendererStats.
	_, _ = h.Update(metadataResultMsg{Results: []instanceMetadata{md}})

	// The instance should now have the renderer stats cached.
	assert.Equal(t, int64(2048), inst.RendererStats.Bytes)
	assert.Equal(t, int64(30), inst.RendererStats.Lines)
	assert.Equal(t, int64(3), inst.RendererStats.Turns)
	assert.Equal(t, int64(4<<20), inst.RendererStats.MaxBytes)
	assert.Equal(t, int64(2000), inst.RendererStats.MaxTurns)
	assert.Equal(t, int64(2), inst.RendererStats.EvictedTurns)
	assert.Equal(t, int64(512), inst.RendererStats.EvictedBytes)
}

func TestMetadataResult_LocalRendererStats_StoredOnInstance(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "local-sdk-agent",
		Path:          repoDir,
		Program:       "opencode",
		ExecutionMode: session.ExecutionModeSDK,
	})
	require.NoError(t, err)

	stats := sessionsdk.RendererStats{
		Bytes:         4096,
		Lines:         80,
		Turns:         6,
		MaxBytes:      8 << 20,
		MaxTurns:      3000,
		EvictedTurns:  4,
		EvictedLines:  12,
		EvictedBytes:  1024,
		TruncatedRows: 2,
	}

	h := newTestHome()
	h.activeRepoPath = repoDir
	h.allInstances = append(h.allInstances, inst)
	_ = h.nav.AddInstance(inst)

	_, _ = h.Update(metadataResultMsg{Results: []instanceMetadata{{
		Title:         inst.Title,
		RendererStats: &stats,
		TmuxAlive:     true,
	}}})

	assert.Equal(t, stats, inst.RendererStats)
}

// newFakePresServer creates a test HTTP server that serves the given presentation
// response on any request. Callers must call srv.Close().
func newFakePresServer(t *testing.T, resp api.PresentationResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}
