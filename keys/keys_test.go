package keys

import "testing"

func TestGlobalKeyStringsMap_ViewPlanHasPAlias(t *testing.T) {
	if got, ok := GlobalKeyStringsMap["p"]; !ok || got != KeyViewPlan {
		t.Fatalf("GlobalKeyStringsMap[\"p\"] = (%v, %v), want (%v, true)", got, ok, KeyViewPlan)
	}
}

func TestGlobalKeyStringsMap_NoSidebarFocusShortcutOnS(t *testing.T) {
	if _, ok := GlobalKeyStringsMap["s"]; ok {
		t.Fatalf("GlobalKeyStringsMap should not map 's' to a keybind")
	}
}

func TestGlobalKeyBindings_UpdatedStatusLineLabels(t *testing.T) {
	if got := GlobalkeyBindings[KeyEnter].Help().Desc; got != "select" {
		t.Fatalf("KeyEnter help desc = %q, want %q", got, "select")
	}
	if got := GlobalkeyBindings[KeySpaceExpand].Help().Desc; got != "toggle" {
		t.Fatalf("KeySpaceExpand help desc = %q, want %q", got, "toggle")
	}
	if got := GlobalkeyBindings[KeyViewPlan].Help().Desc; got != "preview" {
		t.Fatalf("KeyViewPlan help desc = %q, want %q", got, "preview")
	}
}
