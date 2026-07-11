package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFSMPlanStart_TransitionsReadyToPlanning verifies that the FSM correctly
// transitions a ready plan to planning via the PlanStart event (replacing the
// deleted setPlanStatus / modify_plan path).
func TestFSMPlanStart_TransitionsReadyToPlanning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	store, ps, fsm := newSharedStoreForTest(t, plansDir)
	_ = store

	planFile := "auth-refactor.md"
	if err := ps.Register(planFile, "auth refactor", "plan/auth-refactor", time.Now()); err != nil {
		t.Fatal(err)
	}

	if err := fsm.Transition(planFile, taskfsm.PlanStart); err != nil {
		t.Fatalf("Transition(PlanStart) error: %v", err)
	}

	reloaded, _ := newTestPlanStateWithStore(t, store, plansDir)
	entry, ok := reloaded.Entry(planFile)
	if !ok {
		t.Fatal("plan entry missing after PlanStart transition")
	}
	if entry.Status != taskstate.StatusPlanning {
		t.Fatalf("status = %q, want %q", entry.Status, taskstate.StatusPlanning)
	}
}

func TestPRTitleSubmitPreparesBodyAsynchronously(t *testing.T) {
	t.Parallel()
	h := newTestHome()
	h.state = statePRTitle
	h.pendingPRSource = &prSource{
		worktree: gitpkg.NewSharedTaskWorktree(t.TempDir(), "task/test"),
		branch:   "task/test",
		title:    "test task",
	}
	h.overlays.Show(&submittedOverlay{result: overlay.Result{
		Dismissed: true,
		Submitted: true,
		Value:     "test task",
	}})

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)

	require.NotNil(t, cmd)
	assert.Equal(t, statePRPreparingBody, updated.state)
	assert.NotEqual(t, statePRBody, updated.state)
}

func TestCompletedTaskPublicationDoesNotRequireLiveInstance(t *testing.T) {
	t.Parallel()
	h, planFile, branch := newCompletedTaskPRHome(t, true)

	model, cmd := h.executeContextAction("create_plan_pr")
	updated := model.(*home)
	require.Nil(t, cmd)
	require.Equal(t, statePRTitle, updated.state)
	require.NotNil(t, updated.pendingPRSource)
	assert.Equal(t, branch, updated.pendingPRSource.branch)
	assert.Equal(t, planFile, updated.pendingPRSource.planFile)

	updated.state = stateDefault
	updated.overlays.Dismiss()
	model, cmd = updated.executeContextAction("push_plan_branch")
	updated = model.(*home)
	assert.Nil(t, cmd)
	assert.Equal(t, stateConfirm, updated.state)
	assert.True(t, updated.overlays.IsActive())
}

func TestPreparePRBodyUsesTaskMetadata(t *testing.T) {
	t.Parallel()
	h, planFile, branch := newCompletedTaskPRHome(t, false)
	require.NoError(t, h.taskStore.SetSubtasks(h.taskStoreProject, planFile, []taskstore.SubtaskEntry{
		{TaskNumber: 1, Title: "preserve completed task flow", Status: taskstore.SubtaskStatusDone},
		{TaskNumber: 2, Title: "persist pull request metadata", Status: taskstore.SubtaskStatusPending},
	}))

	msg := h.preparePRBody(prSource{
		worktree: gitpkg.NewSharedTaskWorktree(h.activeRepoPath, branch),
		planFile: planFile,
		branch:   branch,
	}, "completed task publishing", 1)()
	ready, ok := msg.(prBodyReadyMsg)
	require.True(t, ok, "message = %T", msg)
	assert.NotEmpty(t, ready.body)
	assert.Contains(t, ready.body, "publish a completed task without a live agent")
	assert.Contains(t, ready.body, "keep completed task publishing available")
	assert.Contains(t, ready.body, "preserve completed task flow")
	assert.Contains(t, ready.body, "persist pull request metadata")
}

func TestPRPreparingBodyBlocksInputAndIgnoresStaleResponses(t *testing.T) {
	h := newTestHome()
	h.state = statePRPreparingBody
	h.pendingPRRequestID = 2
	h.pendingPRSource = &prSource{branch: "plan/current", title: "current"}

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'P', Text: "P"})
	updated := model.(*home)
	require.Nil(t, cmd)
	assert.Equal(t, statePRPreparingBody, updated.state)
	assert.Equal(t, uint64(2), updated.pendingPRRequestID)
	assert.Equal(t, "plan/current", updated.pendingPRSource.branch)

	model, cmd = updated.Update(prBodyReadyMsg{requestID: 1, title: "stale", body: "stale"})
	updated = model.(*home)
	require.Nil(t, cmd)
	assert.Equal(t, statePRPreparingBody, updated.state)
	assert.Equal(t, "plan/current", updated.pendingPRSource.branch)
}

func TestAutomaticPRCompletionDoesNotClearManualPRState(t *testing.T) {
	h, planFile, _ := newCompletedTaskPRHome(t, false)
	h.state = statePRPreparingBody
	h.pendingPRSource = &prSource{branch: "plan/manual", title: "manual"}
	h.pendingPRToastID = h.toastManager.Loading("preparing manual pr...")
	manualToastID := h.pendingPRToastID

	model, cmd := h.Update(prCreatedForPlanMsg{planFile: planFile, url: "https://github.test/pr/1"})
	updated := model.(*home)

	require.NotNil(t, cmd)
	require.NotNil(t, updated.pendingPRSource)
	assert.Equal(t, "plan/manual", updated.pendingPRSource.branch)
	assert.Equal(t, manualToastID, updated.pendingPRToastID)
	assert.Equal(t, statePRPreparingBody, updated.state)
}

func TestCreatePRKeyOnTaskRowUsesTaskMetadata(t *testing.T) {
	t.Parallel()
	h, _, branch := newCompletedTaskPRHome(t, false)
	h.keySent = true // bypass the first-pass menu highlight replay

	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'P', Text: "P"})
	updated := model.(*home)
	require.Equal(t, statePRTitle, updated.state)
	require.NotNil(t, updated.pendingPRSource)
	assert.Equal(t, branch, updated.pendingPRSource.branch)
	assert.Equal(t, "publish a completed task without a live agent", updated.pendingPRSource.title)
}

func TestUnresolvableTaskPRFailsClosed(t *testing.T) {
	t.Parallel()
	h, planFile, _ := newCompletedTaskPRHome(t, false)
	entry, err := h.taskStore.Get(h.taskStoreProject, planFile)
	require.NoError(t, err)
	entry.Branch = ""
	require.NoError(t, h.taskStore.Update(h.taskStoreProject, planFile, entry))
	h.taskState.Plans[planFile] = taskstate.TaskEntry{
		Status:      taskstate.StatusDone,
		Description: entry.Description,
		Goal:        entry.Goal,
	}

	model, cmd := h.executeContextAction("create_plan_pr")
	updated := model.(*home)
	require.NotNil(t, cmd)
	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, updated.pendingPRSource)
	assert.False(t, updated.overlays.IsActive())
}

func newCompletedTaskPRHome(t *testing.T, placeholder bool) (*home, string, string) {
	t.Helper()
	const (
		planFile = "completed-task"
		branch   = "plan/completed-task"
	)
	dir := t.TempDir()
	store, ps, _ := newSharedStoreForTest(t, dir)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename:    planFile,
		Status:      taskstore.StatusDone,
		Description: "publish a completed task without a live agent",
		Goal:        "keep completed task publishing available",
		Branch:      branch,
	}))
	ps.Plans[planFile] = taskstate.TaskEntry{
		Status:      taskstate.StatusDone,
		Description: "publish a completed task without a live agent",
		Goal:        "keep completed task publishing available",
		Branch:      branch,
	}
	h := newTestHome()
	h.activeRepoPath = dir
	h.taskStore = store
	h.taskStoreProject = "test"
	h.taskState = ps
	// Keep the task selected in the active rows while the authoritative store
	// entry remains done; publication behavior reads status and branch from there.
	h.nav.SetPlans([]ui.PlanDisplay{{Filename: planFile, Status: string(taskstate.StatusReady)}})
	if placeholder {
		inst, err := session.NewInstance(session.InstanceOptions{Title: "completed task placeholder", Program: "opencode", TaskFile: planFile})
		require.NoError(t, err)
		inst.BindSharedTaskWorktree(dir, branch)
		h.nav.AddInstance(inst)
	}
	h.nav.ClickItem(0)
	require.Equal(t, planFile, h.nav.GetSelectedPlanFile())
	return h, planFile, branch
}
