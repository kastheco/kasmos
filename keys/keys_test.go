package keys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalKeyStringsMap_ViewPlanHasPAlias(t *testing.T) {
	if got, ok := GlobalKeyStringsMap["p"]; !ok || got != KeyViewPlan {
		t.Fatalf("GlobalKeyStringsMap[\"p\"] = (%v, %v), want (%v, true)", got, ok, KeyViewPlan)
	}
}

func TestSpawnAgentKeyInGlobalMap(t *testing.T) {
	name, ok := GlobalKeyStringsMap["S"]
	assert.True(t, ok, "'S' must be in GlobalKeyStringsMap")
	assert.Equal(t, KeySpawnAgent, name)
}

func TestQuickLaunchKeyInGlobalMap(t *testing.T) {
	quickLaunchName, ok := GlobalKeyStringsMap["s"]
	assert.True(t, ok, "'s' must be in GlobalKeyStringsMap")
	assert.Equal(t, KeyQuickLaunch, quickLaunchName)

	spawnAgentName, ok := GlobalKeyStringsMap["S"]
	assert.True(t, ok, "'S' must still be in GlobalKeyStringsMap")
	assert.Equal(t, KeySpawnAgent, spawnAgentName)
	assert.NotEqual(t, quickLaunchName, spawnAgentName)

	assert.Equal(t, "quick launch", GlobalkeyBindings[KeyQuickLaunch].Help().Desc)
}

func TestRemovedSingleKeyBindings(t *testing.T) {
	assert.NotContains(t, GlobalKeyStringsMap, "c")
	assert.NotContains(t, GlobalKeyStringsMap, "g")
	assert.NotContains(t, GlobalKeyStringsMap, "#")
	assert.NotContains(t, GlobalKeyStringsMap, "T")
	assert.NotContains(t, GlobalKeyStringsMap, "k")
	assert.NotContains(t, GlobalKeyStringsMap, "K")
}

func TestDestructiveBindingsRequireCtrl(t *testing.T) {
	assert.Equal(t, KeyKill, GlobalKeyStringsMap["ctrl+k"])
	assert.Equal(t, KeyAbort, GlobalKeyStringsMap["ctrl+shift+k"])
}

func TestInfoTabKeyInGlobalMap(t *testing.T) {
	name, ok := GlobalKeyStringsMap["I"]
	assert.True(t, ok, "'I' must be in GlobalKeyStringsMap")
	assert.Equal(t, KeyInfoTab, name)
}

func TestGlobalKeyBindings_UpdatedStatusLineLabels(t *testing.T) {
	if got := GlobalkeyBindings[KeyEnter].Help().Desc; got != "select" {
		t.Fatalf("KeyEnter help desc = %q, want %q", got, "select")
	}
	if got := GlobalkeyBindings[KeySpaceExpand].Help().Desc; got != "toggle" {
		t.Fatalf("KeySpaceExpand help desc = %q, want %q", got, "toggle")
	}
	if got := GlobalkeyBindings[KeyViewPlan].Help().Desc; got != "view plan" {
		t.Fatalf("KeyViewPlan help desc = %q, want %q", got, "view plan")
	}
	if got := GlobalkeyBindings[KeyCommandLauncher].Help().Desc; got != "commands" {
		t.Fatalf("KeyCommandLauncher help desc = %q, want %q", got, "commands")
	}
}

func TestYesKeyInGlobalMap(t *testing.T) {
	name, ok := GlobalKeyStringsMap["y"]
	assert.True(t, ok, "'y' must be in GlobalKeyStringsMap")
	assert.Equal(t, KeySendYes, name)
}

func TestGlobalKeyBindings_YesLabel(t *testing.T) {
	if got := GlobalkeyBindings[KeySendYes].Help().Desc; got != "yes" {
		t.Fatalf("KeySendYes help desc = %q, want %q", got, "yes")
	}
}

func TestGlobalKeyBindings_PageLabels(t *testing.T) {
	assert.Equal(t, "page up", GlobalkeyBindings[KeyPageUp].Help().Desc)
	assert.Equal(t, "page down", GlobalkeyBindings[KeyPageDown].Help().Desc)
}
