package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/planfsm"
	"github.com/kastheco/kasmos/config/planstate"
)

// TestFSMPlanStart_TransitionsReadyToPlanning verifies that the FSM correctly
// transitions a ready plan to planning via the PlanStart event (replacing the
// deleted setPlanStatus / modify_plan path).
func TestFSMPlanStart_TransitionsReadyToPlanning(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ps, err := planstate.Load(plansDir)
	if err != nil {
		t.Fatal(err)
	}
	planFile := "2026-02-21-auth-refactor.md"
	if err := ps.Register(planFile, "auth refactor", "plan/auth-refactor", time.Now()); err != nil {
		t.Fatal(err)
	}

	fsm := planfsm.New(plansDir)
	if err := fsm.Transition(planFile, planfsm.PlanStart); err != nil {
		t.Fatalf("Transition(PlanStart) error: %v", err)
	}

	reloaded, _ := planstate.Load(plansDir)
	entry, ok := reloaded.Entry(planFile)
	if !ok {
		t.Fatal("plan entry missing after PlanStart transition")
	}
	if entry.Status != planstate.StatusPlanning {
		t.Fatalf("status = %q, want %q", entry.Status, planstate.StatusPlanning)
	}
}
