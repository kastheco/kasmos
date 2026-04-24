package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initLifecycleE2ERepo(t *testing.T, dir string) {
	t.Helper()
	for _, cmd := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	} {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			t.Skipf("git setup failed (%v): %s", err, out)
		}
	}
}

func withDaemonManagedRepo(t *testing.T, dir string) {
	t.Helper()
	oldManaged := repoManagedByDaemon
	repoManagedByDaemon = func(repoPath string) bool {
		return filepath.Clean(repoPath) == filepath.Clean(dir)
	}
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
	})
}

func newLifecycleE2EHome(t *testing.T, dir, plansDir string, store taskstore.Store, ps *taskstate.TaskState, fsm *taskfsm.TaskStateMachine) *home {
	t.Helper()
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	threshold := 0
	cfg := config.DefaultConfig()
	cfg.BlueprintSkipThresholdValue = &threshold

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             cfg,
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
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{ready: true}
		},
	}
	h.updateSidebarTasks()
	return h
}

func lifecycleEntryForTest(t *testing.T, store taskstore.Store, plansDir, planFile string) taskstate.TaskEntry {
	t.Helper()
	reloaded, err := newTestPlanStateWithStore(t, store, plansDir)
	require.NoError(t, err)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	return entry
}

func TestLifecycleE2E_DaemonManagedHappyPath(t *testing.T) {
	// serial: modifies repoManagedByDaemon via withDaemonManagedRepo
	const planFile = "daemon-managed-happy-path"

	dir := t.TempDir()
	initLifecycleE2ERepo(t, dir)
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	withDaemonManagedRepo(t, dir)

	store, ps, fsm := newSharedStoreForTest(t, plansDir)
	require.NoError(t, ps.Create(planFile, "daemon managed happy path", "plan/daemon-managed-happy-path", "lifecycle", time.Now()))
	require.NoError(t, store.SetContent("test", planFile, strings.Join([]string{
		"# happy path",
		"",
		"**Goal:** verify the daemon-managed lifecycle path.",
		"**Architecture:** test",
		"**Tech Stack:** Go",
		"",
		"## Wave 1",
		"",
		"### Task 1: Implement lifecycle fix",
		"",
		"Make the lifecycle transitions persist through SQLite.",
		"",
	}, "\n")))

	h := newLifecycleE2EHome(t, dir, plansDir, store, ps, fsm)
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	model, planCmd := h.executeTaskStage(planFile, "plan")
	updated := model.(*home)
	require.NotNil(t, planCmd)

	entry := lifecycleEntryForTest(t, store, plansDir, planFile)
	assert.Equal(t, taskstate.StatusPlanning, entry.Status)

	model, implementCmd := updated.executeTaskStage(planFile, "implement_direct")
	updated = model.(*home)
	require.NotNil(t, implementCmd)

	entry = lifecycleEntryForTest(t, store, plansDir, planFile)
	assert.Equal(t, taskstate.StatusImplementing, entry.Status)
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	}, entry.ExecutionState)

	model, reviewerCmd := updated.Update(wavePushCompleteMsg{planFile: planFile})
	updated = model.(*home)
	require.NotNil(t, reviewerCmd)

	entry = lifecycleEntryForTest(t, store, plansDir, planFile)
	assert.Equal(t, taskstate.StatusReviewing, entry.Status)
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseReviewing),
		ActiveAgentType: session.AgentTypeReviewer,
	}, entry.ExecutionState)

	model, doneCmd := updated.executeTaskStage(planFile, "finished")
	updated = model.(*home)
	assert.Nil(t, doneCmd)

	entry = lifecycleEntryForTest(t, store, plansDir, planFile)
	assert.Equal(t, taskstate.StatusDone, entry.Status)
	assert.Equal(t, taskstore.ExecutionState{}, entry.ExecutionState)
	assert.NotNil(t, updated)
}

func TestLifecycleE2E_DaemonManagedReviewFixLoop(t *testing.T) {
	// serial: modifies repoManagedByDaemon via withDaemonManagedRepo
	const planFile = "daemon-managed-review-fix-loop"
	const feedback = "please persist the latest review feedback before respawning the fixer"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	withDaemonManagedRepo(t, dir)

	store, ps, fsm := newSharedStoreForTest(t, plansDir)
	require.NoError(t, ps.Create(planFile, "daemon managed review fix loop", "plan/daemon-managed-review-fix-loop", "lifecycle", time.Now()))
	require.NoError(t, ps.ForceSetLifecycle(planFile, taskstate.StatusReviewing, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseReviewing),
		ActiveAgentType: session.AgentTypeReviewer,
	}))
	require.NoError(t, ps.SetLatestReviewFeedback(planFile, feedback))

	h := newLifecycleE2EHome(t, dir, plansDir, store, ps, fsm)
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	model, fixerCmd := h.executeContextAction("start_fixer")
	updated := model.(*home)
	require.NotNil(t, fixerCmd)

	entry := lifecycleEntryForTest(t, store, plansDir, planFile)
	assert.Equal(t, taskstate.StatusImplementing, entry.Status)
	assert.Equal(t, 1, entry.ReviewCycle)
	assert.Equal(t, feedback, entry.LatestReviewFeedback)
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseFixing),
		ActiveAgentType: session.AgentTypeFixer,
	}, entry.ExecutionState)

	model, reviewerCmd := updated.Update(coderCompleteMsg{planFile: planFile})
	updated = model.(*home)
	require.NotNil(t, reviewerCmd)

	entry = lifecycleEntryForTest(t, store, plansDir, planFile)
	assert.Equal(t, taskstate.StatusReviewing, entry.Status)
	assert.Equal(t, 1, entry.ReviewCycle)
	assert.Equal(t, feedback, entry.LatestReviewFeedback)
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseReviewing),
		ActiveAgentType: session.AgentTypeReviewer,
	}, entry.ExecutionState)

	model, doneCmd := updated.executeTaskStage(planFile, "finished")
	updated = model.(*home)
	assert.Nil(t, doneCmd)

	entry = lifecycleEntryForTest(t, store, plansDir, planFile)
	assert.Equal(t, taskstate.StatusDone, entry.Status)
	assert.Equal(t, taskstore.ExecutionState{}, entry.ExecutionState)
	assert.NotNil(t, updated)
}
