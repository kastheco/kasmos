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

// TestHalfPageKeysInGlobalMap asserts ctrl+u / ctrl+d are registered.
func TestHalfPageKeysInGlobalMap(t *testing.T) {
	assert.Equal(t, KeyHalfPageUp, GlobalKeyStringsMap["ctrl+u"], "ctrl+u must map to KeyHalfPageUp")
	assert.Equal(t, KeyHalfPageDown, GlobalKeyStringsMap["ctrl+d"], "ctrl+d must map to KeyHalfPageDown")
}

// TestHalfPageBindingsHaveDoubleTapHint asserts help text mentions the double-tap alternative.
func TestHalfPageBindingsHaveDoubleTapHint(t *testing.T) {
	upDesc := GlobalkeyBindings[KeyHalfPageUp].Help().Desc
	downDesc := GlobalkeyBindings[KeyHalfPageDown].Help().Desc
	assert.Equal(t, "half-page up", upDesc)
	assert.Equal(t, "half-page down", downDesc)
	assert.Contains(t, GlobalkeyBindings[KeyHalfPageUp].Help().Key, "ctrl+u")
	assert.Contains(t, GlobalkeyBindings[KeyHalfPageDown].Help().Key, "ctrl+d")
}

// TestDestructiveBindingsHaveDoubleTapHint checks that kill/abort help text
// exposes the double-tap hint so the key-bind browser shows it.
func TestDestructiveBindingsHaveDoubleTapHint(t *testing.T) {
	killKey := GlobalkeyBindings[KeyKill].Help().Key
	abortKey := GlobalkeyBindings[KeyAbort].Help().Key
	assert.Contains(t, killKey, "k+k", "KeyKill help key should mention k+k double-tap")
	assert.Contains(t, killKey, "k+k+k", "KeyKill help key should mention k+k+k triple-tap")
	assert.Contains(t, abortKey, "K+K", "KeyAbort help key should mention K+K double-tap")
}

// TestTripleTapMapEntries asserts the triple-tap escalation entries.
func TestTripleTapMapEntries(t *testing.T) {
	assert.Equal(t, KeyKillAndRemove, TripleTapMap["k"], "k must triple-tap to KeyKillAndRemove")
	assert.NotContains(t, TripleTapMap, "K", "K must not be in TripleTapMap (uppercase only in DoubleTapMap)")

	// Every key in TripleTapMap must also be present in DoubleTapMap.
	for k := range TripleTapMap {
		_, ok := DoubleTapMap[k]
		assert.True(t, ok, "TripleTapMap key %q must also exist in DoubleTapMap", k)
	}
}

// TestDoubleTapMapEntries asserts the four conflict-free double-tap entries.
func TestDoubleTapMapEntries(t *testing.T) {
	assert.Equal(t, KeyKill, DoubleTapMap["k"])
	assert.Equal(t, KeyAbort, DoubleTapMap["K"])
	assert.Equal(t, KeyHalfPageUp, DoubleTapMap["u"])
	assert.Equal(t, KeyHalfPageDown, DoubleTapMap["d"])
}

// TestDoubleTapMapDoesNotContainBoundKeys asserts that keys with existing
// single-press bindings are NOT in DoubleTapMap (they belong in DebouncedDoubleTapMap).
func TestDoubleTapMapDoesNotContainBoundKeys(t *testing.T) {
	assert.NotContains(t, DoubleTapMap, "s", "s has a single-press binding; use DebouncedDoubleTapMap")
	assert.NotContains(t, DoubleTapMap, "space", "space has a single-press binding; use DebouncedDoubleTapMap")
}

// TestDebouncedDoubleTapMapEntries asserts the app-layer debounced entries.
func TestDebouncedDoubleTapMapEntries(t *testing.T) {
	assert.Equal(t, KeyToggleSidebar, DebouncedDoubleTapMap["s"])
	assert.Equal(t, KeyExitFocus, DebouncedDoubleTapMap["space"])
	// Canonical form is "space", not " ".
	assert.NotContains(t, DebouncedDoubleTapMap, " ", `store "space" not " " to keep the map canonical`)
}

// TestRemovedSingleKeyBindings_StillAbsent re-asserts existing guarantee that
// destructive keys (k, K) remain absent from GlobalKeyStringsMap.
func TestRemovedSingleKeyBindings_StillAbsent(t *testing.T) {
	assert.NotContains(t, GlobalKeyStringsMap, "k", "k must not have a single-press binding")
	assert.NotContains(t, GlobalKeyStringsMap, "K", "K must not have a single-press binding")
	assert.NotContains(t, GlobalKeyStringsMap, "u", "u must not have a single-press binding")
	assert.NotContains(t, GlobalKeyStringsMap, "d", "d must not have a single-press binding")
}
