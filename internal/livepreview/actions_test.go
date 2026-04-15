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
//
// The optional output map is keyed by the first argument of the recorded
// command after name (for example "status" for `git ... status --porcelain`).
// Tests use it to simulate a dirty or clean worktree in the pause/kill gate.
type mockCommandRunner struct {
	calls  [][]string
	err    map[string]error // key is command name
	output map[string][]byte
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

// Output records the invocation and returns a test-configured byte slice.
// It keys on the first non-flag argument after name (e.g. "status" for
// `git -C <path> status --porcelain`), falling back to name-only lookup so
// simple tests can pass output keyed by `"git"`.
func (r *mockCommandRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.err != nil {
		if err, ok := r.err["output:"+name]; ok {
			return nil, err
		}
	}
	if r.output != nil {
		for _, a := range args {
			if v, ok := r.output[a]; ok {
				return v, nil
			}
		}
		if v, ok := r.output[name]; ok {
			return v, nil
		}
	}
	return nil, nil
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

	// Verify tmux was killed (best-effort). The dirty check runs first, so
	// tmux won't be the first recorded call — just assert it was invoked.
	require.NotEmpty(t, runner.calls)
	var sawTmux bool
	for _, c := range runner.calls {
		if len(c) > 0 && c[0] == "tmux" {
			sawTmux = true
			break
		}
	}
	assert.True(t, sawTmux, "expected tmux kill-session to be invoked, got calls: %v", runner.calls)
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

func TestApplyAction_Headless_RejectsAllLifecycleActions(t *testing.T) {
	// Headless rows must be refused by ValidateAction for pause/resume/restart/
	// kill because the web path cannot safely manage a headless child process.
	for _, action := range []string{"pause", "resume", "restart", "kill"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			// StatusPaused so "resume" would otherwise be valid; StatusRunning
			// wouldn't exercise the resume branch.
			status := StatusRunning
			if action == "resume" {
				status = StatusPaused
			}
			writeFullStateJSON(t, root,
				Record{
					Title:         "headless-agent",
					Status:        status,
					Program:       "claude",
					ExecutionMode: "headless",
					Worktree:      Worktree{RepoPath: "/repo", WorktreePath: "/repo/wt", BranchName: "b"},
				},
			)
			runner := &mockCommandRunner{}

			err := ApplyAction(context.Background(), root, "headless-agent", action, runner)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrActionInvalidState,
				"headless %s must return ErrActionInvalidState", action)
			// State must be unchanged.
			records, rerr := LoadRecordsFromRepoRoot(root)
			require.NoError(t, rerr)
			require.Len(t, records, 1)
			assert.Equal(t, status, records[0].Status)
		})
	}
}

func TestApplyAction_Pause_RejectsDirtyWorktree(t *testing.T) {
	root := t.TempDir()
	writeFullStateJSON(t, root,
		Record{Title: "dirty-agent", Status: StatusRunning, Program: "claude",
			Worktree: Worktree{RepoPath: "/repo", WorktreePath: "/repo/wt"}},
	)
	runner := &mockCommandRunner{
		output: map[string][]byte{
			// Any git status call returns dirty output.
			"status": []byte(" M README.md\n"),
		},
	}

	err := ApplyAction(context.Background(), root, "dirty-agent", "pause", runner)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes")
	assert.ErrorIs(t, err, ErrActionInvalidState,
		"dirty worktree must wrap ErrActionInvalidState so the HTTP layer maps it to 409")

	// State must not be mutated: status still running, worktree path preserved.
	records, rerr := LoadRecordsFromRepoRoot(root)
	require.NoError(t, rerr)
	require.Len(t, records, 1)
	assert.Equal(t, StatusRunning, records[0].Status)
	assert.Equal(t, "/repo/wt", records[0].Worktree.WorktreePath)

	// The dirty check must have run — but no destructive tmux/worktree commands.
	for _, c := range runner.calls {
		if len(c) > 0 && c[0] == "tmux" {
			t.Errorf("tmux command should not have been run on dirty pause, got: %v", c)
		}
		// git worktree remove must never appear.
		for i, a := range c {
			if a == "worktree" && i+1 < len(c) && c[i+1] == "remove" {
				t.Errorf("git worktree remove should not have been run on dirty pause, got: %v", c)
			}
		}
	}
}

func TestApplyAction_Kill_RejectsDirtyWorktree(t *testing.T) {
	root := t.TempDir()
	writeFullStateJSON(t, root,
		Record{Title: "dirty-agent", Status: StatusRunning, Program: "claude",
			Worktree: Worktree{RepoPath: "/repo", WorktreePath: "/repo/wt"}},
		Record{Title: "other-agent", Status: StatusRunning},
	)
	runner := &mockCommandRunner{
		output: map[string][]byte{
			"status": []byte("?? new.txt\n"),
		},
	}

	err := ApplyAction(context.Background(), root, "dirty-agent", "kill", runner)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes")

	// Record must still be present — kill must not have removed it.
	records, rerr := LoadRecordsFromRepoRoot(root)
	require.NoError(t, rerr)
	require.Len(t, records, 2)

	for _, c := range runner.calls {
		if len(c) > 0 && c[0] == "tmux" {
			t.Errorf("tmux command should not have been run on dirty kill, got: %v", c)
		}
		for i, a := range c {
			if a == "worktree" && i+1 < len(c) && c[i+1] == "remove" {
				t.Errorf("git worktree remove should not have been run on dirty kill, got: %v", c)
			}
		}
	}
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

// TestStandaloneResumeProgram_CodexSkipPermissions verifies that resumed codex
// instances on the standalone web path receive --dangerously-bypass-approvals-and-sandbox
// when SkipPermissions is true, and that the unsupported --agent flag is never
// injected. Mirrors session/tmux/tmux_session.go:Start.
func TestStandaloneResumeProgram_CodexSkipPermissions(t *testing.T) {
	rec := Record{
		Title:           "my-codex",
		Program:         "codex",
		AgentType:       "coder",
		SkipPermissions: true,
	}
	got := standaloneResumeProgram(rec, "/worktrees/my-codex")
	assert.Contains(t, got, "--dangerously-bypass-approvals-and-sandbox")
	assert.NotContains(t, got, "--agent")
	assert.NotContains(t, got, "--permission-mode bypassPermissions")
}

// TestStandaloneResumeProgram_CodexNoSkipPermissions verifies that resumed codex
// instances omit the bypass flag when SkipPermissions is false, and still
// suppress --agent (codex does not recognise it).
func TestStandaloneResumeProgram_CodexNoSkipPermissions(t *testing.T) {
	rec := Record{
		Title:     "my-codex",
		Program:   "codex",
		AgentType: "planner",
	}
	got := standaloneResumeProgram(rec, "/worktrees/my-codex")
	assert.NotContains(t, got, "--dangerously-bypass-approvals-and-sandbox")
	assert.NotContains(t, got, "--agent")
}
