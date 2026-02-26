package ui

import (
	"strings"
	"testing"
)

func stripMenuANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
		}
		if !inEsc {
			b.WriteRune(r)
		}
		if inEsc && r == 'm' {
			inEsc = false
		}
	}
	return b.String()
}

func TestMenu_SidebarEmptyHidesNewPlanAndUsesUpdatedLabels(t *testing.T) {
	m := NewMenu()
	m.SetSize(140, 1)
	m.SetState(StateEmpty)
	m.SetFocusSlot(MenuSlotSidebar)

	out := stripMenuANSI(m.String())

	if strings.Contains(out, "new plan") {
		t.Fatalf("menu should hide 'new plan' hint when fallback logo is showing; got: %q", out)
	}
	if !strings.Contains(out, "↵/o select") {
		t.Fatalf("menu should label enter as select; got: %q", out)
	}
	if !strings.Contains(out, "space toggle") {
		t.Fatalf("menu should label space as toggle; got: %q", out)
	}
	if !strings.Contains(out, "v preview") {
		t.Fatalf("menu should label v as preview; got: %q", out)
	}
}
