package app

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/kastheco/klique/config/planstate"
	"github.com/kastheco/klique/session"
	"github.com/kastheco/klique/ui"
	"github.com/kastheco/klique/ui/overlay"
)

func newHomeForPlanActionTests(t *testing.T) *home {
	t.Helper()
	sp := spinner.New()
	return &home{
		ctx:          context.Background(),
		list:         ui.NewList(&sp, false),
		sidebar:      ui.NewSidebar(),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewGitPane()),
		toastManager: overlay.NewToastManager(&sp),
		planState: &planstate.PlanState{
			Dir: t.TempDir(),
			Plans: map[string]planstate.PlanEntry{
				"2026-02-21-alpha.md": {Status: planstate.StatusInProgress},
				"2026-02-21-beta.md":  {Status: planstate.StatusReady},
			},
		},
	}
}

// TestCancelPlan_PendingActionReturnsPlanRefreshMsg verifies that the async
// cancel_plan action (run in a goroutine by confirmAction) returns planRefreshMsg
// so model mutations stay in Update, not in the goroutine.
func TestCancelPlan_PendingActionReturnsPlanRefreshMsg(t *testing.T) {
	h := newHomeForPlanActionTests(t)
	h.updateSidebarPlans()
	h.updateSidebarItems()
	h.sidebar.ClickItem(2) // select alpha plan

	_, _ = h.executeContextAction("cancel_plan")
	if h.pendingConfirmAction == nil {
		t.Fatalf("expected pendingConfirmAction to be set")
	}

	msg := h.pendingConfirmAction()
	if _, ok := msg.(planRefreshMsg); !ok {
		t.Fatalf("pendingConfirmAction returned %T, want planRefreshMsg", msg)
	}
}

// TestExecuteContextAction_KillRunningInstancesInPlan verifies the real async flow:
// pendingConfirmAction does only I/O and returns killPlanInstancesMsg; model
// mutations happen only inside Update.
func TestExecuteContextAction_KillRunningInstancesInPlan(t *testing.T) {
	h := newHomeForPlanActionTests(t)
	h.updateSidebarPlans()

	mk := func(title, planFile string) *session.Instance {
		inst, _ := session.NewInstance(session.InstanceOptions{Title: title, Path: ".", Program: "claude", PlanFile: planFile})
		return inst
	}
	alpha1 := mk("alpha-1", "2026-02-21-alpha.md")
	alpha2 := mk("alpha-2", "2026-02-21-alpha.md")
	beta := mk("beta", "2026-02-21-beta.md")
	h.allInstances = []*session.Instance{alpha1, alpha2, beta}
	_ = h.list.AddInstance(alpha1)
	_ = h.list.AddInstance(alpha2)
	_ = h.list.AddInstance(beta)

	h.updateSidebarItems()
	h.sidebar.ClickItem(2) // alpha plan

	_, _ = h.executeContextAction("kill_running_instances_in_plan")
	if h.confirmationOverlay == nil {
		t.Fatalf("expected confirmation overlay")
	}
	if h.pendingConfirmAction == nil {
		t.Fatalf("expected pendingConfirmAction to be set")
	}

	// Run the async action — must return killPlanInstancesMsg, not mutate model directly.
	msg := h.pendingConfirmAction()
	km, ok := msg.(killPlanInstancesMsg)
	if !ok {
		t.Fatalf("pendingConfirmAction returned %T, want killPlanInstancesMsg", msg)
	}
	if km.planFile != "2026-02-21-alpha.md" {
		t.Fatalf("killPlanInstancesMsg.planFile = %q, want 2026-02-21-alpha.md", km.planFile)
	}

	// Apply the message through Update — this is where model mutations happen.
	newModel, _ := h.Update(km)
	newHome := newModel.(*home)

	if len(newHome.allInstances) != 1 || newHome.allInstances[0].Title != "beta" {
		t.Fatalf("remaining allInstances = %v, want [beta]", instanceTitles(newHome.allInstances))
	}
	if newHome.list.NumInstances() != 1 {
		t.Fatalf("list.NumInstances() = %d, want 1", newHome.list.NumInstances())
	}
}

func instanceTitles(insts []*session.Instance) []string {
	out := make([]string, len(insts))
	for i, inst := range insts {
		out[i] = inst.Title
	}
	return out
}
