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
	assert.Equal(t, "ready", ps.Plans["my-plan.md"].Status)
	assert.Equal(t, "done", ps.Plans["done-plan.md"].Status)
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
			"a.md": {Status: "ready"},
			"b.md": {Status: "in_progress"},
			"c.md": {Status: "reviewing"},
			"d.md": {Status: "done"},
		},
	}

	unfinished := ps.Unfinished()
	assert.Len(t, unfinished, 3)
	for _, p := range unfinished {
		assert.NotEqual(t, "d.md", p.Filename)
	}
}

func TestAllTasksDone(t *testing.T) {
	ps := &PlanState{
		Dir: "/tmp",
		Plans: map[string]PlanEntry{
			"a.md": {Status: "done"},
			"b.md": {Status: "done"},
		},
	}

	assert.True(t, ps.AllTasksDone("a.md"))
	ps.Plans["c.md"] = PlanEntry{Status: "in_progress"}
	assert.True(t, ps.AllTasksDone("a.md"))
	assert.False(t, ps.AllTasksDone("missing.md"))
}

func TestPlanLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan-state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"test-plan.md": {"status": "ready"}}`), 0o644))

	ps, err := Load(dir)
	require.NoError(t, err)

	// Coder picks it up
	require.NoError(t, ps.SetStatus("test-plan.md", "in_progress"))
	unfinished := ps.Unfinished()
	require.Len(t, unfinished, 1)
	assert.Equal(t, "in_progress", unfinished[0].Status)
	assert.False(t, ps.AllTasksDone("test-plan.md"))

	// Coder finishes — agent writes "done"
	require.NoError(t, ps.SetStatus("test-plan.md", "done"))
	assert.True(t, ps.AllTasksDone("test-plan.md"))
	assert.Empty(t, ps.Unfinished()) // "done" excluded from unfinished

	// klique transitions to "reviewing" (spawns reviewer session)
	require.NoError(t, ps.SetStatus("test-plan.md", "reviewing"))
	assert.False(t, ps.AllTasksDone("test-plan.md"))
	unfinished = ps.Unfinished()
	require.Len(t, unfinished, 1)
	assert.Equal(t, "reviewing", unfinished[0].Status)

	// Reviewer completes — klique marks done
	require.NoError(t, ps.SetStatus("test-plan.md", "done"))
	assert.True(t, ps.AllTasksDone("test-plan.md"))
	assert.Empty(t, ps.Unfinished())

	// Verify persistence: reload and check final state
	ps2, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "done", ps2.Plans["test-plan.md"].Status)
}

func TestSetStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan-state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"a.md": {"status": "in_progress"}}`), 0o644))

	ps, err := Load(dir)
	require.NoError(t, err)

	require.NoError(t, ps.SetStatus("a.md", "reviewing"))
	assert.Equal(t, "reviewing", ps.Plans["a.md"].Status)

	ps2, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "reviewing", ps2.Plans["a.md"].Status)
}
