package planstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docs", "plans", "plan-state.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`{
		"my-plan.md": {"status": "ready"},
		"done-plan.md": {"status": "done", "implemented": "2026-02-20"}
	}`), 0o644))

	ps, err := Load(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, ps.Plans, 2)
	assert.Equal(t, StatusReady, ps.Plans["my-plan.md"].Status)
	assert.Equal(t, StatusDone, ps.Plans["done-plan.md"].Status)
}

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	ps, err := Load(dir)
	require.NoError(t, err)
	assert.Empty(t, ps.Plans)
}

func TestUnfinished(t *testing.T) {
	ps := &PlanState{
		Dir: "/tmp",
		Plans: map[string]PlanEntry{
			"a.md": {Status: StatusReady},
			"b.md": {Status: StatusInProgress},
			"c.md": {Status: StatusReviewing},
			"d.md": {Status: StatusDone},
			"e.md": {Status: StatusCompleted},
		},
	}

	unfinished := ps.Unfinished()
	// done and completed are both excluded
	assert.Len(t, unfinished, 3)
	for _, p := range unfinished {
		assert.NotEqual(t, "d.md", p.Filename, "done should be excluded")
		assert.NotEqual(t, "e.md", p.Filename, "completed should be excluded")
	}
}

func TestIsDone(t *testing.T) {
	ps := &PlanState{
		Dir: "/tmp",
		Plans: map[string]PlanEntry{
			"a.md": {Status: StatusDone},
			"b.md": {Status: StatusDone},
		},
	}

	assert.True(t, ps.IsDone("a.md"))
	ps.Plans["c.md"] = PlanEntry{Status: StatusInProgress}
	assert.True(t, ps.IsDone("a.md"))
	assert.False(t, ps.IsDone("missing.md"))

	// completed is NOT done — this is the key invariant that breaks the spawn loop
	ps.Plans["comp.md"] = PlanEntry{Status: StatusCompleted}
	assert.False(t, ps.IsDone("comp.md"), "completed should not be treated as done")
}

func TestPlanLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan-state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"test-plan.md": {"status": "ready"}}`), 0o644))

	ps, err := Load(dir)
	require.NoError(t, err)

	// Coder picks it up
	require.NoError(t, ps.SetStatus("test-plan.md", StatusInProgress))
	unfinished := ps.Unfinished()
	require.Len(t, unfinished, 1)
	assert.Equal(t, StatusInProgress, unfinished[0].Status)
	assert.False(t, ps.IsDone("test-plan.md"))

	// Coder finishes — agent writes "done"
	require.NoError(t, ps.SetStatus("test-plan.md", StatusDone))
	assert.True(t, ps.IsDone("test-plan.md"))
	assert.Empty(t, ps.Unfinished()) // "done" excluded from unfinished

	// klique transitions to "reviewing" (spawns reviewer session)
	require.NoError(t, ps.SetStatus("test-plan.md", StatusReviewing))
	assert.False(t, ps.IsDone("test-plan.md"))
	unfinished = ps.Unfinished()
	require.Len(t, unfinished, 1)
	assert.Equal(t, StatusReviewing, unfinished[0].Status)

	// Reviewer completes — klique marks completed (terminal, not done)
	require.NoError(t, ps.SetStatus("test-plan.md", StatusCompleted))
	assert.False(t, ps.IsDone("test-plan.md"))
	assert.Empty(t, ps.Unfinished())

	// Verify persistence: reload and check final state
	ps2, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, ps2.Plans["test-plan.md"].Status)
}

// TestFullLifecycleNoRespawnLoop walks the complete orchestration state machine and
// asserts that the terminal `completed` status cannot re-trigger a reviewer session.
//
// The bug this tests for: after a reviewer exits, klique wrote "done" which caused
// IsDone() to return true again, spawning another reviewer — forever. Now klique
// writes "completed" instead, and IsDone() only matches "done", breaking the cycle.
func TestFullLifecycleNoRespawnLoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan-state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"feature.md": {"status": "ready"}}`), 0o644))

	ps, err := Load(dir)
	require.NoError(t, err)

	// Step 1: ready → coder picks it up
	require.NoError(t, ps.SetStatus("feature.md", StatusInProgress))
	assert.False(t, ps.IsDone("feature.md"))
	assert.Len(t, ps.Unfinished(), 1)

	// Step 2: coder writes "done"
	require.NoError(t, ps.SetStatus("feature.md", StatusDone))
	assert.True(t, ps.IsDone("feature.md"), "IsDone must be true so reviewer gets spawned")
	assert.Empty(t, ps.Unfinished(), "done should not appear in sidebar")

	// Step 3: klique spawns reviewer, marks "reviewing"
	require.NoError(t, ps.SetStatus("feature.md", StatusReviewing))
	assert.False(t, ps.IsDone("feature.md"), "reviewing is not done")
	assert.Len(t, ps.Unfinished(), 1, "reviewing should appear in sidebar")

	// Step 4: reviewer exits — klique marks "completed" (the fix)
	require.NoError(t, ps.SetStatus("feature.md", StatusCompleted))

	// Critical invariants that break the respawn loop:
	assert.False(t, ps.IsDone("feature.md"),
		"completed must NOT satisfy IsDone — otherwise a new reviewer would be spawned")
	assert.Empty(t, ps.Unfinished(),
		"completed must not appear in sidebar unfinished list")

	// Verify persistence
	ps2, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, ps2.Plans["feature.md"].Status)
	assert.False(t, ps2.IsDone("feature.md"))
	assert.Empty(t, ps2.Unfinished())
}

func TestSetStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan-state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"a.md": {"status": "in_progress"}}`), 0o644))

	ps, err := Load(dir)
	require.NoError(t, err)

	require.NoError(t, ps.SetStatus("a.md", StatusReviewing))
	assert.Equal(t, StatusReviewing, ps.Plans["a.md"].Status)

	ps2, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, StatusReviewing, ps2.Plans["a.md"].Status)
}
