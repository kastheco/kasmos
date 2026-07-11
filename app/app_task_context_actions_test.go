package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	gitpkg "github.com/kastheco/kasmos/session/git"
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
