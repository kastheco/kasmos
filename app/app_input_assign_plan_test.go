package app

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kastheco/klique/config/planstate"
	"github.com/kastheco/klique/session"
	"github.com/kastheco/klique/ui"
	"github.com/kastheco/klique/ui/overlay"
	zone "github.com/lrstanley/bubblezone"
)

func newHomeForInputTests(t *testing.T) *home {
	t.Helper()
	sp := spinner.New()
	h := &home{
		ctx:            context.Background(),
		state:          stateDefault,
		program:        "claude",
		activeRepoPath: ".",
		list:           ui.NewList(&sp, false),
		menu:           ui.NewMenu(),
		sidebar:        ui.NewSidebar(),
		planState: &planstate.PlanState{Plans: map[string]planstate.PlanEntry{
			"2026-02-21-alpha.md": {Status: planstate.StatusReady},
		}},
	}
	h.updateSidebarPlans()
	h.updateSidebarItems()
	return h
}

// newHomeForViewTests creates a home with all fields View() needs so it won't panic.
func newHomeForViewTests(t *testing.T) *home {
	t.Helper()
	zone.NewGlobal()
	sp := spinner.New()
	h := &home{
		ctx:            context.Background(),
		state:          stateDefault,
		program:        "claude",
		activeRepoPath: ".",
		list:           ui.NewList(&sp, false),
		menu:           ui.NewMenu(),
		sidebar:        ui.NewSidebar(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewGitPane()),
		toastManager:   overlay.NewToastManager(&sp),
		planState: &planstate.PlanState{Plans: map[string]planstate.PlanEntry{
			"2026-02-21-alpha.md": {Status: planstate.StatusReady},
		}},
	}
	h.updateSidebarPlans()
	h.updateSidebarItems()
	return h
}

// TestStateMoveTo_ViewRendersPickerOverlay verifies that View() actually renders
// the pickerOverlay content when state == stateMoveTo (not falling through to default).
func TestStateMoveTo_ViewRendersPickerOverlay(t *testing.T) {
	h := newHomeForViewTests(t)
	h.state = stateMoveTo
	h.pickerOverlay = overlay.NewPickerOverlay("Assign to plan", []string{"alpha", "beta"})

	rendered := h.View()

	if !strings.Contains(rendered, "Assign to plan") {
		t.Fatalf("View() with stateMoveTo did not render picker overlay; output does not contain 'Assign to plan'")
	}
}

// pressKey bypasses the two-pass menu-highlight mechanism and invokes the real
// key handler directly. The first call to handleKeyPress normally intercepts to
// update the menu; pre-setting keySent=true skips that first pass.
func pressKey(h *home, msg tea.KeyMsg) *home {
	h.keySent = true
	model, _ := h.handleKeyPress(msg)
	if model == nil {
		return h
	}
	return model.(*home)
}

// TestKeyNew_InheritsSelectedPlanFile verifies that pressing 'n' to create a new
// instance inherits the PlanFile from the currently selected plan in the sidebar.
func TestKeyNew_InheritsSelectedPlanFile(t *testing.T) {
	h := newHomeForInputTests(t)
	h.sidebar.ClickItem(2) // select alpha plan

	h = pressKey(h, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if h.newInstance == nil {
		t.Fatalf("newInstance not set after KeyNew")
	}
	if h.newInstance.PlanFile != "2026-02-21-alpha.md" {
		t.Fatalf("newInstance.PlanFile = %q, want 2026-02-21-alpha.md", h.newInstance.PlanFile)
	}
}

// TestKeyPrompt_InheritsSelectedPlanFile verifies that 'N' (prompt-first new instance)
// also inherits the selected plan.
func TestKeyPrompt_InheritsSelectedPlanFile(t *testing.T) {
	h := newHomeForInputTests(t)
	h.sidebar.ClickItem(2) // select alpha plan

	h = pressKey(h, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})

	if h.newInstance == nil {
		t.Fatalf("newInstance not set after KeyPrompt")
	}
	if h.newInstance.PlanFile != "2026-02-21-alpha.md" {
		t.Fatalf("newInstance.PlanFile = %q, want 2026-02-21-alpha.md", h.newInstance.PlanFile)
	}
}

// TestKeyNewSkipPermissions_InheritsSelectedPlanFile verifies that 'S'
// (skip-permissions new instance) also inherits the selected plan.
func TestKeyNewSkipPermissions_InheritsSelectedPlanFile(t *testing.T) {
	h := newHomeForInputTests(t)
	h.sidebar.ClickItem(2) // select alpha plan

	h = pressKey(h, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})

	if h.newInstance == nil {
		t.Fatalf("newInstance not set after KeyNewSkipPermissions")
	}
	if h.newInstance.PlanFile != "2026-02-21-alpha.md" {
		t.Fatalf("newInstance.PlanFile = %q, want 2026-02-21-alpha.md", h.newInstance.PlanFile)
	}
}

func TestKeyMoveTo_OpensAssignPlanPicker(t *testing.T) {
	h := newHomeForInputTests(t)
	inst, _ := session.NewInstance(session.InstanceOptions{Title: "w", Path: ".", Program: "claude"})
	_ = h.list.AddInstance(inst)
	h.list.SetSelectedInstance(0)

	_, _ = h.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})

	if h.state != stateMoveTo {
		t.Fatalf("state = %v, want stateMoveTo", h.state)
	}
	if h.pickerOverlay == nil {
		t.Fatalf("pickerOverlay should be initialized")
	}
	if got := h.pickerOverlay.Render(); got == "" {
		t.Fatalf("picker render should not be empty")
	}
}

func TestStateMoveTo_SubmitAssignsPlanFile(t *testing.T) {
	h := newHomeForInputTests(t)
	inst, _ := session.NewInstance(session.InstanceOptions{Title: "w", Path: ".", Program: "claude"})
	_ = h.list.AddInstance(inst)
	h.list.SetSelectedInstance(0)

	h.state = stateMoveTo
	h.planPickerMap = map[string]string{
		"(Ungrouped)": "",
		"alpha":       "2026-02-21-alpha.md",
	}
	h.pickerOverlay = overlay.NewPickerOverlay("Assign to plan", []string{"(Ungrouped)", "alpha"})

	// move selection to "alpha", then submit
	h.pickerOverlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = h.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})

	if inst.PlanFile != "2026-02-21-alpha.md" {
		t.Fatalf("PlanFile = %q, want 2026-02-21-alpha.md", inst.PlanFile)
	}
}
