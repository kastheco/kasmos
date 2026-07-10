package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteTaskValidateArchitectMeta(t *testing.T) {
	repoRoot := t.TempDir()
	cacheDir := filepath.Join(repoRoot, ".kasmos", "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "planner-architect.json"), []byte(`{
  "schema_version": 1,
  "plan_id": "planner",
  "waves": [{"wave": 1, "parallel": true, "tasks": []}],
  "decision_audit": {
    "schema_version": 1,
    "plan_file": "planner",
    "project": "kasmos",
    "created_at": "2026-07-10T12:00:00Z",
    "baseline_source": "inline",
    "summary": "validated",
    "final_decision": "ship it"
  }
}`), 0o644))

	require.NoError(t, executeTaskValidateArchitectMeta(repoRoot, "kasmos", "planner.md"))

	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "planner-architect.json"), []byte(`{"waves":2}`), 0o644))
	assert.Error(t, executeTaskValidateArchitectMeta(repoRoot, "kasmos", "planner"))
}

func TestNewTaskCmd_RegistersArchitectMetaValidator(t *testing.T) {
	command, _, err := NewTaskCmd().Find([]string{"validate-architect-meta"})
	require.NoError(t, err)
	assert.Equal(t, "validate-architect-meta", command.Name())
}
