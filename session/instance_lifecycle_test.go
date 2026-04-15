package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/platform"
	"github.com/kastheco/kasmos/session/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "command failed: %s %v\n%s", name, args, string(output))
}

func setupGitRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runCommand(t, repo, "git", "init")
	runCommand(t, repo, "git", "config", "user.email", "test@example.com")
	runCommand(t, repo, "git", "config", "user.name", "test")

	filePath := filepath.Join(repo, "README.md")
	require.NoError(t, os.WriteFile(filePath, []byte("test\n"), 0644))
	runCommand(t, repo, "git", "add", "README.md")
	runCommand(t, repo, "git", "commit", "-m", "initial")

	return repo
}

// newMockTmuxSession returns a tmuxExecutionSession backed by a mock TmuxSession
// with the given dependencies, for use in lifecycle tests.
func newMockTmuxSession(name, program string, ptyFac tmux.PtyFactory, cmdExec cmd_test.MockCmdExec) *tmuxExecutionSession {
	return &tmuxExecutionSession{
		s: tmux.NewTmuxSessionWithDeps(name, program, false, ptyFac, cmdExec),
	}
}

func TestStartTransfersQueuedPromptForOpenCode(t *testing.T) {
	swapProbeMCP(t, func() error { return nil })
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("Ask anything"), nil
		},
	}

	inst := &Instance{
		Title:            "test-transfer",
		Path:             t.TempDir(),
		Program:          "opencode",
		QueuedPrompt:     "Plan auth.",
		executionSession: newMockTmuxSession("test-transfer", "opencode", &testPtyFactory{}, cmdExec),
	}
	inst.SetTmuxSession(tmux.NewTmuxSessionWithDeps("test-transfer", "opencode", false, &testPtyFactory{}, cmdExec))

	// Simulate StartOnMainBranch which is the simplest path.
	err := inst.StartOnMainBranch()
	require.NoError(t, err)

	// QueuedPrompt should be cleared (transferred to initialPrompt).
	assert.Empty(t, inst.QueuedPrompt)
}

func TestStartTransfersQueuedPromptForCodex(t *testing.T) {
	swapProbeMCP(t, func() error { return nil })
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("Ask anything"), nil
		},
	}

	inst := &Instance{
		Title:            "test-codex-transfer",
		Path:             t.TempDir(),
		Program:          "codex",
		QueuedPrompt:     "Implement feature X.",
		executionSession: newMockTmuxSession("test-codex-transfer", "codex", &testPtyFactory{}, cmdExec),
	}
	inst.SetTmuxSession(tmux.NewTmuxSessionWithDeps("test-codex-transfer", "codex", false, &testPtyFactory{}, cmdExec))

	err := inst.StartOnMainBranch()
	require.NoError(t, err)

	// QueuedPrompt should be cleared (transferred to initialPrompt).
	assert.Empty(t, inst.QueuedPrompt)
}

func TestStartKeepsQueuedPromptForAider(t *testing.T) {
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("Open documentation url for more info"), nil
		},
	}

	inst := &Instance{
		Title:            "test-aider",
		Path:             t.TempDir(),
		Program:          "aider --model ollama_chat/gemma3:1b",
		QueuedPrompt:     "Fix the bug.",
		executionSession: newMockTmuxSession("test-aider", "aider --model ollama_chat/gemma3:1b", &testPtyFactory{}, cmdExec),
	}
	inst.SetTmuxSession(tmux.NewTmuxSessionWithDeps("test-aider", "aider --model ollama_chat/gemma3:1b", false, &testPtyFactory{}, cmdExec))

	err := inst.StartOnMainBranch()
	require.NoError(t, err)

	// QueuedPrompt should remain — aider doesn't support CLI prompts.
	assert.Equal(t, "Fix the bug.", inst.QueuedPrompt)
}

func TestRestart_KillsTmuxAndRestartsSession(t *testing.T) {
	swapProbeMCP(t, func() error { return nil })
	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}

	inst := &Instance{
		Title:   "test-restart",
		Path:    t.TempDir(),
		Program: "opencode",
		started: true,
	}
	inst.executionSession = newMockTmuxSession(inst.Title, inst.Program, &testPtyFactory{}, cmdExec)

	err := inst.Restart()
	assert.NoError(t, err)
	assert.Equal(t, Running, inst.Status)
	assert.True(t, inst.started)
}

func TestRestart_WorksWhenTmuxAlreadyDead(t *testing.T) {
	swapProbeMCP(t, func() error { return nil })
	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}

	inst := &Instance{
		Title:   "test-restart-dead",
		Path:    t.TempDir(),
		Program: "opencode",
		started: true,
		Exited:  true,
	}
	inst.executionSession = newMockTmuxSession(inst.Title, inst.Program, &testPtyFactory{}, cmdExec)

	err := inst.Restart()
	assert.NoError(t, err)
	assert.False(t, inst.Exited, "Exited flag should be cleared after restart")
	assert.Equal(t, Running, inst.Status)
}

func TestRestart_ProbesSharedMCP(t *testing.T) {
	swapProbeMCP(t, func() error { return fmt.Errorf("endpoint down") })
	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}

	inst := &Instance{
		Title:   "test-restart-probe",
		Path:    t.TempDir(),
		Program: "claude",
		started: true,
	}
	inst.executionSession = newMockTmuxSession(inst.Title, inst.Program, &testPtyFactory{}, cmdExec)

	err := inst.Restart()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp endpoint not reachable")
}

func TestRestart_NotStarted_ReturnsError(t *testing.T) {
	inst := &Instance{Title: "never-started", started: false}
	err := inst.Restart()
	assert.Error(t, err)
}

func TestRestart_PausedInstance_ReturnsError(t *testing.T) {
	inst := &Instance{
		Title:   "paused-restart",
		started: true,
		Status:  Paused,
	}
	err := inst.Restart()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "paused")
}

func TestStartOnBranch_SetsFields(t *testing.T) {
	swapProbeMCP(t, func() error { return nil })
	repoPath := setupGitRepo(t)

	inst, err := NewInstance(InstanceOptions{
		Title:   "test-branch",
		Path:    repoPath,
		Program: "opencode",
	})
	require.NoError(t, err)
	assert.Equal(t, "", inst.Branch)

	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("Ask anything"), nil
		},
	}
	inst.executionSession = newMockTmuxSession("test-branch", "opencode", &testPtyFactory{}, cmdExec)

	err = inst.StartOnBranch("feature/task-5")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, inst.Kill())
	})

	assert.Equal(t, "feature/task-5", inst.Branch)
	assert.Equal(t, Running, inst.Status)
	assert.True(t, inst.Started())
	assert.NotEqual(t, "", inst.GetWorktreePath(), fmt.Sprintf("worktree path should be set for %s", inst.Title))
}

func TestKill_PreservesBranch(t *testing.T) {
	swapProbeMCP(t, func() error { return nil })
	repoPath := setupGitRepo(t)
	inst, err := NewInstance(InstanceOptions{
		Title:   "test-safe-kill",
		Path:    repoPath,
		Program: "opencode",
	})
	require.NoError(t, err)

	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}
	inst.executionSession = newMockTmuxSession(inst.Title, inst.Program, &testPtyFactory{}, cmdExec)

	err = inst.StartOnBranch("safe-kill-branch")
	require.NoError(t, err)

	branchName := inst.Branch
	require.NotEmpty(t, branchName)

	err = inst.Kill()
	require.NoError(t, err)

	// Branch must still exist after kill
	out, gitErr := exec.Command("git", "-C", repoPath, "branch", "--list", branchName).CombinedOutput()
	require.NoError(t, gitErr)
	assert.Contains(t, string(out), branchName, "branch should be preserved after Kill()")
}

func TestKill_DirtyWorktreeReturnsError(t *testing.T) {
	swapProbeMCP(t, func() error { return nil })
	repoPath := setupGitRepo(t)
	inst, err := NewInstance(InstanceOptions{
		Title:   "test-dirty-kill",
		Path:    repoPath,
		Program: "opencode",
	})
	require.NoError(t, err)

	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}
	inst.executionSession = newMockTmuxSession(inst.Title, inst.Program, &testPtyFactory{}, cmdExec)

	err = inst.StartOnBranch("dirty-kill-branch")
	require.NoError(t, err)
	t.Cleanup(func() {
		if inst.gitWorktree != nil {
			_ = inst.gitWorktree.Remove()
			_ = inst.gitWorktree.Prune()
		}
	})

	readmePath := filepath.Join(inst.GetWorktreePath(), "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("dirty\n"), 0o644))

	err = inst.Kill()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes")
	assert.Contains(t, err.Error(), "README.md")
}

func TestShouldAutoAdvanceLifecycleImplementer(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		state     taskstore.ExecutionState
		inst      *Instance
		tmuxAlive bool
		want      bool
	}{
		{
			name:      "tmux exit advances single coder",
			status:    string(taskfsm.StatusImplementing),
			state:     taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseSingleAgentImplementing), ActiveAgentType: AgentTypeCoder},
			inst:      &Instance{TaskFile: "feature", AgentType: AgentTypeCoder},
			tmuxAlive: false,
			want:      true,
		},
		{
			name:   "prompt return advances fixer after stability window",
			status: string(taskfsm.StatusImplementing),
			state:  taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseFixing), ActiveAgentType: AgentTypeFixer},
			inst: &Instance{
				TaskFile:              "feature",
				AgentType:             AgentTypeFixer,
				PromptDetected:        true,
				CompletionPromptSince: time.Now().Add(-(CompletionPromptStabilityWindow + 10*time.Millisecond)),
			},
			tmuxAlive: true,
			want:      true,
		},
		{
			name:   "permission blocked fixer prompt does not advance",
			status: string(taskfsm.StatusImplementing),
			state:  taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseFixing), ActiveAgentType: AgentTypeFixer},
			inst: &Instance{
				TaskFile:              "feature",
				AgentType:             AgentTypeFixer,
				PromptDetected:        true,
				PermissionBlocked:     true,
				CompletionPromptSince: time.Now().Add(-(CompletionPromptStabilityWindow + 10*time.Millisecond)),
			},
			tmuxAlive: true,
			want:      false,
		},
		{
			name:   "fixer prompt within stability window does not advance",
			status: string(taskfsm.StatusImplementing),
			state:  taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseFixing), ActiveAgentType: AgentTypeFixer},
			inst: &Instance{
				TaskFile:              "feature",
				AgentType:             AgentTypeFixer,
				PromptDetected:        true,
				CompletionPromptSince: time.Now().Add(-10 * time.Millisecond),
			},
			tmuxAlive: true,
			want:      false,
		},
		{
			name:   "fixer prompt with zero CompletionPromptSince does not advance",
			status: string(taskfsm.StatusImplementing),
			state:  taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseFixing), ActiveAgentType: AgentTypeFixer},
			inst: &Instance{
				TaskFile:       "feature",
				AgentType:      AgentTypeFixer,
				PromptDetected: true,
			},
			tmuxAlive: true,
			want:      false,
		},
		{
			name:      "wave task never auto advances",
			status:    string(taskfsm.StatusImplementing),
			state:     taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveRunning), ActiveAgentType: AgentTypeCoder, ActiveWave: 1},
			inst:      &Instance{TaskFile: "feature", AgentType: AgentTypeCoder, TaskNumber: 1},
			tmuxAlive: false,
			want:      false,
		},
		{
			name:      "headless exit advances",
			status:    string(taskfsm.StatusImplementing),
			state:     taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseSingleAgentImplementing), ActiveAgentType: AgentTypeCoder},
			inst:      &Instance{TaskFile: "feature", AgentType: AgentTypeCoder, ExecutionMode: ExecutionModeHeadless, Exited: true},
			tmuxAlive: true,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShouldAutoAdvanceLifecycleImplementer(tt.status, tt.state, tt.inst, tt.tmuxAlive))
		})
	}
}

func TestIsStuck(t *testing.T) {
	tests := []struct {
		name      string
		entry     taskstore.TaskEntry
		inst      *Instance
		tmuxAlive bool
		want      bool
	}{
		{
			name: "architect exit is stuck",
			entry: taskstore.TaskEntry{
				Status: taskstore.StatusImplementing,
				ExecutionState: taskstore.ExecutionState{
					Phase:           string(taskfsm.ExecutionPhaseArchitecting),
					ActiveAgentType: AgentTypeElaborator,
				},
			},
			inst:      &Instance{TaskFile: "feature", AgentType: AgentTypeElaborator},
			tmuxAlive: false,
			want:      true,
		},
		{
			name: "wave wait exit is stuck",
			entry: taskstore.TaskEntry{
				Status: taskstore.StatusImplementing,
				ExecutionState: taskstore.ExecutionState{
					Phase:           string(taskfsm.ExecutionPhaseWaveWaiting),
					ActiveAgentType: AgentTypeCoder,
				},
			},
			inst:      &Instance{TaskFile: "feature", AgentType: AgentTypeCoder, TaskNumber: 2},
			tmuxAlive: false,
			want:      true,
		},
		{
			name: "single agent exit auto advances instead of stuck",
			entry: taskstore.TaskEntry{
				Status: taskstore.StatusImplementing,
				ExecutionState: taskstore.ExecutionState{
					Phase:           string(taskfsm.ExecutionPhaseSingleAgentImplementing),
					ActiveAgentType: AgentTypeCoder,
				},
			},
			inst:      &Instance{TaskFile: "feature", AgentType: AgentTypeCoder},
			tmuxAlive: false,
			want:      false,
		},
		{
			name: "reviewing status is not stuck",
			entry: taskstore.TaskEntry{
				Status: taskstore.StatusReviewing,
				ExecutionState: taskstore.ExecutionState{
					Phase: string(taskfsm.ExecutionPhaseFixing),
				},
			},
			inst:      &Instance{TaskFile: "feature", AgentType: AgentTypeFixer},
			tmuxAlive: false,
			want:      false,
		},
		{
			name: "paused instance is not stuck",
			entry: taskstore.TaskEntry{
				Status: taskstore.StatusImplementing,
				ExecutionState: taskstore.ExecutionState{
					Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
					ActiveAgentType: AgentTypeCoder,
				},
			},
			inst:      &Instance{TaskFile: "feature", AgentType: AgentTypeCoder, Status: Paused},
			tmuxAlive: false,
			want:      false,
		},
		{
			name: "active agent mismatch is not stuck",
			entry: taskstore.TaskEntry{
				Status: taskstore.StatusImplementing,
				ExecutionState: taskstore.ExecutionState{
					Phase:           string(taskfsm.ExecutionPhaseArchitecting),
					ActiveAgentType: AgentTypeElaborator,
				},
			},
			inst:      &Instance{TaskFile: "feature", AgentType: AgentTypeCoder},
			tmuxAlive: false,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsStuck(tt.entry, tt.inst, tt.tmuxAlive))
		})
	}
}

func TestPause_DirtyWorktreeReturnsError(t *testing.T) {
	swapProbeMCP(t, func() error { return nil })
	repoPath := setupGitRepo(t)
	inst, err := NewInstance(InstanceOptions{
		Title:   "test-dirty-pause",
		Path:    repoPath,
		Program: "opencode",
	})
	require.NoError(t, err)

	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}
	inst.executionSession = newMockTmuxSession(inst.Title, inst.Program, &testPtyFactory{}, cmdExec)

	err = inst.StartOnBranch("dirty-pause-branch")
	require.NoError(t, err)
	t.Cleanup(func() {
		if inst.gitWorktree != nil {
			_ = inst.gitWorktree.Remove()
			_ = inst.gitWorktree.Prune()
		}
	})

	readmePath := filepath.Join(inst.GetWorktreePath(), "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("dirty\n"), 0o644))

	err = inst.Pause()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes")
	assert.Contains(t, err.Error(), "README.md")
}

// TestInstance_DefaultExecutionModeIsTmux verifies that instances created without
// an explicit ExecutionMode use tmux by default, and that this survives a round-trip
// through InstanceData.
func TestInstance_DefaultExecutionModeIsTmux(t *testing.T) {
	inst, err := NewInstance(InstanceOptions{
		Title:   "default-mode",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	assert.Equal(t, ExecutionModeTmux, inst.ExecutionMode, "default execution mode should be tmux")

	// Round-trip through InstanceData.
	data := inst.ToInstanceData()
	assert.Equal(t, ExecutionModeTmux, data.ExecutionMode, "InstanceData should persist tmux mode")

	// Restoring with empty ExecutionMode should also default to tmux.
	emptyModeData := InstanceData{
		Title:   "empty-mode",
		Path:    "/tmp/repo",
		Branch:  "feature/test",
		Status:  Paused,
		Program: "claude",
		Worktree: GitWorktreeData{
			RepoPath:      "/tmp/repo",
			WorktreePath:  "/tmp/repo/.worktrees/empty-mode",
			SessionName:   "empty-mode",
			BranchName:    "feature/test",
			BaseCommitSHA: "abc123",
		},
	}
	restored, err := FromInstanceData(emptyModeData)
	require.NoError(t, err)
	assert.Equal(t, ExecutionModeTmux, restored.ExecutionMode, "empty ExecutionMode should restore as tmux")
}

// TestInstance_AttachReturnsErrorForHeadlessExecution verifies that Attach() returns
// an error for headless instances, and that unstarted instances return the standard
// "not started" error regardless of execution mode.
func TestInstance_AttachReturnsErrorForHeadlessExecution(t *testing.T) {
	// An unstarted instance should always return the "not started" error.
	unstarted := &Instance{
		Title:         "headless-unstarted",
		ExecutionMode: ExecutionModeHeadless,
		started:       false,
	}
	_, err := unstarted.Attach()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot attach instance that has not been started")

	// A started headless instance should return ErrInteractiveOnly (or an error
	// wrapping it) from the headless execution session's Attach implementation.
	headlessInst := &Instance{
		Title:            "headless-started",
		ExecutionMode:    ExecutionModeHeadless,
		started:          true,
		executionSession: NewExecutionSession(ExecutionModeHeadless, "headless-started", "sh", false),
	}
	_, err = headlessInst.Attach()
	require.Error(t, err, "Attach on headless instance should return an error")
	// The error originates from the headless session, which reports interactive-only.
	assert.Contains(t, err.Error(), "interactive")
}

func TestResume_MainBranchPaused_UsesRepoPathAndClearsEphemeralState(t *testing.T) {
	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}

	inst := &Instance{
		Title:                 "test-resume-main",
		Path:                  t.TempDir(),
		Program:               "opencode",
		Status:                Paused,
		started:               true,
		gitWorktree:           nil, // main-branch instance
		Exited:                true,
		PromptDetected:        true,
		HasWorked:             true,
		AwaitingWork:          true,
		Notified:              true,
		CachedContentSet:      true,
		CachedContent:         "stale",
		PermissionBlocked:     true,
		CompletionPromptSince: time.Now().Add(-time.Second),
	}
	inst.executionSession = newMockTmuxSession(inst.Title, inst.Program, &testPtyFactory{}, cmdExec)

	err := inst.Resume()
	require.NoError(t, err)
	assert.Equal(t, Running, inst.Status)
	assert.False(t, inst.Exited)
	assert.False(t, inst.PromptDetected)
	assert.False(t, inst.HasWorked)
	assert.False(t, inst.AwaitingWork)
	assert.False(t, inst.Notified)
	assert.False(t, inst.CachedContentSet)
	assert.Empty(t, inst.CachedContent)
	assert.False(t, inst.PermissionBlocked, "PermissionBlocked should be cleared on resume")
	assert.True(t, inst.CompletionPromptSince.IsZero(), "CompletionPromptSince should be zeroed on resume")
}

func TestResume_SharedWorktree_ReusesExistingPath(t *testing.T) {
	repoPath := setupGitRepo(t)

	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}

	// Create a branch so we can reference it
	runCommand(t, repoPath, "git", "branch", "plan/feature")

	inst := &Instance{
		Title:          "test-resume-shared",
		Path:           repoPath,
		Branch:         "plan/feature",
		Program:        "opencode",
		Status:         Paused,
		started:        true,
		sharedWorktree: true,
	}
	inst.BindSharedTaskWorktree(repoPath, "plan/feature")
	inst.executionSession = newMockTmuxSession(inst.Title, inst.Program, &testPtyFactory{}, cmdExec)

	err := inst.Resume()
	require.NoError(t, err)
	assert.Equal(t, Running, inst.Status)
}

func TestResume_OwnedWorktree_RecreatesWorktree(t *testing.T) {
	swapProbeMCP(t, func() error { return nil })
	repoPath := setupGitRepo(t)

	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}

	inst, err := NewInstance(InstanceOptions{
		Title:   "test-resume-owned",
		Path:    repoPath,
		Program: "opencode",
	})
	require.NoError(t, err)
	inst.executionSession = newMockTmuxSession(inst.Title, inst.Program, &testPtyFactory{}, cmdExec)

	err = inst.StartOnBranch("resume-owned-branch")
	require.NoError(t, err)

	// Pause removes the worktree
	err = inst.Pause()
	require.NoError(t, err)
	assert.Equal(t, Paused, inst.Status)

	// Resume should recreate it
	err = inst.Resume()
	require.NoError(t, err)
	assert.Equal(t, Running, inst.Status)
	assert.NotEmpty(t, inst.GetWorktreePath())
	_, statErr := os.Stat(inst.GetWorktreePath())
	assert.NoError(t, statErr, "worktree path should exist after resume")

	// Cleanup
	t.Cleanup(func() {
		if inst.gitWorktree != nil {
			_ = inst.gitWorktree.Remove()
			_ = inst.gitWorktree.Prune()
		}
	})
}

func TestResume_NotStarted_ReturnsError(t *testing.T) {
	inst := &Instance{Title: "never-started", started: false}
	err := inst.Resume()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not been started")
}

func TestResume_NotPaused_ReturnsError(t *testing.T) {
	inst := &Instance{Title: "running-resume", started: true, Status: Running}
	err := inst.Resume()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "paused")
}

// swapProbeMCP replaces probeMCPFunc for the duration of the test and restores
// it on cleanup.
func swapProbeMCP(t *testing.T, fn func() error) {
	t.Helper()
	orig := probeMCPFunc
	probeMCPFunc = fn
	t.Cleanup(func() { probeMCPFunc = orig })
}

func TestUsesManagedKasmosMCP(t *testing.T) {
	tests := []struct {
		program string
		want    bool
	}{
		{"claude", true},
		{"/usr/local/bin/claude", true},
		{"opencode", true},
		{"/usr/bin/opencode", true},
		{"codex", true},
		{"/opt/bin/codex", true},
		{"aider", false},
		{"aider --model ollama_chat/gemma3:1b", false},
		{"", false},
		{"   ", false},
		{"\t\n", false},
		{"/usr/bin/sh", false},
		{"gemini", false},
	}
	for _, tt := range tests {
		t.Run(tt.program, func(t *testing.T) {
			assert.Equal(t, tt.want, usesManagedKasmosMCP(tt.program))
		})
	}
}

func TestEnsureSharedKasmosMCP_FailedProbeBlocksStartOnMainBranch(t *testing.T) {
	swapProbeMCP(t, func() error { return fmt.Errorf("endpoint down") })

	inst := &Instance{
		Title:   "test-probe-fail-main",
		Path:    t.TempDir(),
		Program: "claude",
	}

	err := inst.StartOnMainBranch()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp endpoint not reachable")
}

func TestEnsureSharedKasmosMCP_FailedProbeBlocksStart(t *testing.T) {
	repoPath := setupGitRepo(t)
	swapProbeMCP(t, func() error { return fmt.Errorf("endpoint down") })

	inst, err := NewInstance(InstanceOptions{
		Title:   "test-probe-fail-start",
		Path:    repoPath,
		Program: "opencode",
	})
	require.NoError(t, err)

	err = inst.Start(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp endpoint not reachable")
}

// Start(false) is the restore-only attachment path used by FromInstanceData when
// reattaching a live session. A down shared endpoint must not block restore —
// the harness process is already running and we only reconnect to its tmux/pty.
func TestEnsureSharedKasmosMCP_RestoreSkipsProbe(t *testing.T) {
	probeCalls := 0
	swapProbeMCP(t, func() error {
		probeCalls++
		return fmt.Errorf("endpoint down")
	})

	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}

	inst := &Instance{
		Title:            "test-probe-restore",
		Path:             t.TempDir(),
		Program:          "claude",
		executionSession: newMockTmuxSession("test-probe-restore", "claude", &testPtyFactory{}, cmdExec),
	}

	require.NoError(t, inst.Start(false))
	assert.Equal(t, 0, probeCalls, "restore path must not probe the shared endpoint")
}

func TestEnsureSharedKasmosMCP_FailedProbeBlocksStartOnBranch(t *testing.T) {
	repoPath := setupGitRepo(t)
	swapProbeMCP(t, func() error { return fmt.Errorf("endpoint down") })

	inst, err := NewInstance(InstanceOptions{
		Title:   "test-probe-fail-branch",
		Path:    repoPath,
		Program: "codex",
	})
	require.NoError(t, err)

	err = inst.StartOnBranch("feature/probe-test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp endpoint not reachable")
}

func TestEnsureSharedKasmosMCP_NonManagedSkipsProbe(t *testing.T) {
	probeCallCount := 0
	swapProbeMCP(t, func() error {
		probeCallCount++
		return fmt.Errorf("should not be called")
	})

	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}

	inst := &Instance{
		Title:            "test-probe-skip",
		Path:             t.TempDir(),
		Program:          "aider",
		executionSession: newMockTmuxSession("test-probe-skip", "aider", &testPtyFactory{}, cmdExec),
	}

	err := inst.StartOnMainBranch()
	require.NoError(t, err)
	assert.Equal(t, 0, probeCallCount, "probe should not be called for non-managed programs")
}

func TestEnsureSharedKasmosMCP_SuccessfulProbeAllowsStart(t *testing.T) {
	swapProbeMCP(t, func() error { return nil })

	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}

	inst := &Instance{
		Title:            "test-probe-ok",
		Path:             t.TempDir(),
		Program:          "claude",
		executionSession: newMockTmuxSession("test-probe-ok", "claude", &testPtyFactory{}, cmdExec),
	}

	err := inst.StartOnMainBranch()
	require.NoError(t, err)
	assert.Equal(t, Running, inst.Status)
}

func TestEnsureSharedKasmosMCP_FailedProbeBlocksStartInSharedWorktree(t *testing.T) {
	swapProbeMCP(t, func() error { return fmt.Errorf("endpoint down") })

	inst := &Instance{
		Title:   "test-probe-shared",
		Path:    t.TempDir(),
		Program: "opencode",
	}

	err := inst.StartInSharedWorktree(nil, "plan/shared")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp endpoint not reachable")
}

// Resume's fresh-start fallback (triggered when no prior tmux session exists)
// must probe the shared endpoint before spawning a new managed harness.
func TestEnsureSharedKasmosMCP_FailedProbeBlocksResumeFreshStart(t *testing.T) {
	swapProbeMCP(t, func() error { return fmt.Errorf("endpoint down") })

	// Mock a tmux session that reports as non-existent so Resume takes the
	// fresh-start branch rather than the restore branch.
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			// has-session returns exit code 1 when the session is absent.
			return &exec.ExitError{}
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}

	inst := &Instance{
		Title:            "test-probe-resume",
		Path:             t.TempDir(),
		Program:          "claude",
		Status:           Paused,
		started:          true,
		executionSession: newMockTmuxSession("test-probe-resume", "claude", &testPtyFactory{}, cmdExec),
	}

	err := inst.Resume()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp endpoint not reachable")
}

// TestEnsureSharedKasmosMCP_ErrorUsesRestartServicesCommand pins the remediation
// command embedded in the failed-probe error to platform.RestartServicesCommand().
// DaemonStartCommand only starts the kasmos daemon and does not start kasmosdb /
// kas serve, which is what hosts the shared MCP endpoint this probe targets.
func TestEnsureSharedKasmosMCP_ErrorUsesRestartServicesCommand(t *testing.T) {
	swapProbeMCP(t, func() error { return fmt.Errorf("dial tcp: connection refused") })

	inst := &Instance{
		Title:   "test-probe-error-text",
		Path:    t.TempDir(),
		Program: "claude",
	}

	err := inst.StartOnMainBranch()
	require.Error(t, err)
	assert.Contains(t, err.Error(), platform.RestartServicesCommand(),
		"error must include RestartServicesCommand (kasmosdb + kasmos), not DaemonStartCommand")
	assert.Contains(t, err.Error(), "shared mcp host",
		"error phrasing must reference the shared mcp host, not just the daemon")
}
