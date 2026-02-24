package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/kastheco/kasmos/config/planstate"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
)

func TestBuildPlanPrompt(t *testing.T) {
	prompt := buildPlanPrompt("Auth Refactor", "Refactor JWT auth")
	if !strings.Contains(prompt, "Plan Auth Refactor") {
		t.Fatalf("prompt missing title")
	}
	if !strings.Contains(prompt, "Goal: Refactor JWT auth") {
		t.Fatalf("prompt missing goal")
	}
}

func TestBuildImplementPrompt(t *testing.T) {
	prompt := buildImplementPrompt("2026-02-21-auth-refactor.md")
	if !strings.Contains(prompt, "Implement docs/plans/2026-02-21-auth-refactor.md") {
		t.Fatalf("prompt missing plan path")
	}
}

func TestAgentTypeForSubItem(t *testing.T) {
	tests := map[string]string{
		"plan":      session.AgentTypePlanner,
		"implement": session.AgentTypeCoder,
		"review":    session.AgentTypeReviewer,
	}
	for action, want := range tests {
		got, ok := agentTypeForSubItem(action)
		if !ok {
			t.Fatalf("agentTypeForSubItem(%q) returned ok=false", action)
		}
		if got != want {
			t.Fatalf("agentTypeForSubItem(%q) = %q, want %q", action, got, want)
		}
	}
}

// TestSpawnPlanAgent_ReviewerSetsIsReviewer verifies that spawnPlanAgent sets
// IsReviewer=true on the created instance when the action is "review", so that
// the reviewer completion check in the metadata tick handler (which gates on
// inst.IsReviewer) can detect when the reviewer session exits.
//
// This is a regression test for the bug where spawnPlanAgent set AgentType but
// not IsReviewer, causing sidebar-spawned reviewers to never trigger plan completion.
func TestSpawnPlanAgent_ReviewerSetsIsReviewer(t *testing.T) {
	dir := t.TempDir()

	// Set up a minimal git repo so shared.Setup() can open it.
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

	plansDir := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ps, err := planstate.Load(plansDir)
	if err != nil {
		t.Fatal(err)
	}
	planFile := "2026-02-21-test.md"
	if err := ps.Register(planFile, "test plan", "plan/test", time.Now()); err != nil {
		t.Fatal(err)
	}

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewList(&sp, false)
	h := &home{
		planState:      ps,
		activeRepoPath: dir,
		program:        "opencode",
		list:           list,
		menu:           ui.NewMenu(),
		sidebar:        ui.NewSidebar(),
	}

	h.spawnPlanAgent(planFile, "review", "review prompt")

	instances := list.GetInstances()
	if len(instances) == 0 {
		t.Fatal("expected instance to be added to list after spawnPlanAgent(review)")
	}
	inst := instances[len(instances)-1]
	if inst.AgentType != session.AgentTypeReviewer {
		t.Fatalf("AgentType = %q, want %q", inst.AgentType, session.AgentTypeReviewer)
	}
	if !inst.IsReviewer {
		t.Fatal("spawnPlanAgent(review) must set IsReviewer=true on the created instance")
	}
}

// TestSpawnPlanAgent_PlannerUsesMainBranch verifies that spawnPlanAgent for the
// "plan" action does NOT create a git worktree — the planner runs on main and
// commits plan files there directly.
func TestSpawnPlanAgent_PlannerUsesMainBranch(t *testing.T) {
	dir := t.TempDir()

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

	plansDir := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ps, err := planstate.Load(plansDir)
	if err != nil {
		t.Fatal(err)
	}
	planFile := "2026-02-23-test-planner.md"
	if err := ps.Register(planFile, "test plan", "plan/test-planner", time.Now()); err != nil {
		t.Fatal(err)
	}

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewList(&sp, false)
	h := &home{
		planState:      ps,
		activeRepoPath: dir,
		program:        "opencode",
		list:           list,
		menu:           ui.NewMenu(),
		sidebar:        ui.NewSidebar(),
	}

	h.spawnPlanAgent(planFile, "plan", "plan prompt")

	instances := list.GetInstances()
	if len(instances) == 0 {
		t.Fatal("expected instance to be added to list after spawnPlanAgent(plan)")
	}
	inst := instances[len(instances)-1]
	if inst.AgentType != session.AgentTypePlanner {
		t.Fatalf("AgentType = %q, want %q", inst.AgentType, session.AgentTypePlanner)
	}
	// Planner should have no branch assigned — it runs on main, not a worktree branch.
	if inst.Branch != "" {
		t.Fatalf("planner instance must have empty Branch (runs on main), got %q", inst.Branch)
	}
}

// TestFSM_PlanLifecycleStages verifies that the FSM produces the correct status for
// each stage in the plan lifecycle (plan→implement→review→done).
func TestFSM_PlanLifecycleStages(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ps, err := planstate.Load(plansDir)
	if err != nil {
		t.Fatal(err)
	}
	planFile := "2026-02-21-test.md"
	if err := ps.Register(planFile, "test plan", "plan/test", time.Now()); err != nil {
		t.Fatal(err)
	}

	f := newFSMForTest(plansDir)

	stages := []struct {
		event      string
		wantStatus planstate.Status
	}{
		{"plan_start", planstate.StatusPlanning},
		{"planner_finished", planstate.StatusReady},
		{"implement_start", planstate.StatusImplementing},
		{"implement_finished", planstate.StatusReviewing},
		{"review_approved", planstate.StatusDone},
	}

	for _, tc := range stages {
		if err := f.TransitionByName(planFile, tc.event); err != nil {
			t.Fatalf("TransitionByName(%q, %q) error: %v", planFile, tc.event, err)
		}
		reloaded, _ := planstate.Load(plansDir)
		entry, ok := reloaded.Entry(planFile)
		if !ok {
			t.Fatalf("plan entry missing after %q", tc.event)
		}
		if entry.Status != tc.wantStatus {
			t.Errorf("after %q: got status %q, want %q", tc.event, entry.Status, tc.wantStatus)
		}
	}
}
