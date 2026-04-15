package livepreview

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCommandRunner records commands for test verification.
type mockCommandRunner struct {
	calls [][]string
	err   map[string]error // key is command name
}

func (r *mockCommandRunner) Run(_ context.Context, name string, args ...string) error {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.err != nil {
		if err, ok := r.err[name]; ok {
			return err
		}
	}
	return nil
}

func writeFullStateJSON(t *testing.T, dir string, records ...Record) {
	t.Helper()
	kasmosDir := filepath.Join(dir, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
	raw, err := json.Marshal(records)
	require.NoError(t, err)
	state := map[string]json.RawMessage{
		"help_screens_seen": json.RawMessage("0"),
		"instances":         json.RawMessage(raw),
	}
	data, err := json.Marshal(state)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "state.json"), data, 0o644))
}

func TestApplyAction_Pause_MarksAsPausedAndKillsTmux(t *testing.T) {
	root := t.TempDir()
	writeFullStateJSON(t, root,
		Record{Title: "my-agent", Status: StatusRunning, Program: "claude",
			Worktree: Worktree{RepoPath: "/repo", WorktreePath: "/repo/wt"}},
	)
	runner := &mockCommandRunner{}

	err := ApplyAction(context.Background(), root, "my-agent", "pause", runner)
	require.NoError(t, err)

	// Verify state was updated.
	records, rerr := LoadRecordsFromRepoRoot(root)
	require.NoError(t, rerr)
	require.Len(t, records, 1)
	assert.Equal(t, StatusPaused, records[0].Status)
	assert.Empty(t, records[0].Worktree.WorktreePath)

	// Verify tmux was killed (best-effort).
	require.NotEmpty(t, runner.calls)
	assert.Equal(t, "tmux", runner.calls[0][0])
}

func TestApplyAction_Kill_RemovesRecord(t *testing.T) {
	root := t.TempDir()
	writeFullStateJSON(t, root,
		Record{Title: "agent-a", Status: StatusRunning},
		Record{Title: "agent-b", Status: StatusRunning},
	)
	runner := &mockCommandRunner{}

	err := ApplyAction(context.Background(), root, "agent-a", "kill", runner)
	require.NoError(t, err)

	records, rerr := LoadRecordsFromRepoRoot(root)
	require.NoError(t, rerr)
	require.Len(t, records, 1)
	assert.Equal(t, "agent-b", records[0].Title)
}

func TestApplyAction_InstanceNotFound(t *testing.T) {
	root := t.TempDir()
	writeFullStateJSON(t, root, Record{Title: "other", Status: StatusRunning})
	runner := &mockCommandRunner{}

	err := ApplyAction(context.Background(), root, "missing", "kill", runner)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrActionInstanceNotFound)
}

func TestApplyAction_InvalidState_PausePaused(t *testing.T) {
	root := t.TempDir()
	writeFullStateJSON(t, root, Record{Title: "paused-agent", Status: StatusPaused})
	runner := &mockCommandRunner{}

	err := ApplyAction(context.Background(), root, "paused-agent", "pause", runner)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrActionInvalidState)
}

func TestApplyAction_Resume_MissingWorktreeMetadata(t *testing.T) {
	root := t.TempDir()
	writeFullStateJSON(t, root, Record{Title: "paused-agent", Status: StatusPaused})
	runner := &mockCommandRunner{}

	err := ApplyAction(context.Background(), root, "paused-agent", "resume", runner)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worktree metadata")
}

func TestApplyAction_PreservesHelpScreensSeen(t *testing.T) {
	root := t.TempDir()
	kasmosDir := filepath.Join(root, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))

	// Write a state with a non-zero help_screens_seen.
	raw, _ := json.Marshal([]Record{{Title: "agent", Status: StatusRunning}})
	state := map[string]json.RawMessage{
		"help_screens_seen": json.RawMessage("42"),
		"instances":         json.RawMessage(raw),
	}
	data, _ := json.Marshal(state)
	require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "state.json"), data, 0o644))

	err := ApplyAction(context.Background(), root, "agent", "kill", &mockCommandRunner{})
	require.NoError(t, err)

	// Verify help_screens_seen is preserved.
	updated, _ := os.ReadFile(filepath.Join(kasmosDir, "state.json"))
	var readBack map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(updated, &readBack))
	assert.Equal(t, json.RawMessage("42"), readBack["help_screens_seen"])
}
