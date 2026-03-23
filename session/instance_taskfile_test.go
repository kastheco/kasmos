package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentTypeElaborator_Constant(t *testing.T) {
	// AgentTypeElaborator was renamed from "elaborator" to "architect" to match
	// the opencode config block name after the elaborator→architect role rename.
	assert.Equal(t, "architect", AgentTypeElaborator)
}

func TestNewInstance_SetsPlanFile(t *testing.T) {
	inst, err := NewInstance(InstanceOptions{
		Title:    "plan-worker",
		Path:     ".",
		Program:  "claude",
		TaskFile: "plan-orchestration",
	})
	require.NoError(t, err)
	assert.Equal(t, "plan-orchestration", inst.TaskFile)
}

func TestInstanceData_RoundTripPlanFile(t *testing.T) {
	data := InstanceData{
		Title:    "persisted",
		Path:     "/tmp/repo",
		Branch:   "feature/test",
		Status:   Paused,
		Program:  "claude",
		TaskFile: "plan",
		Worktree: GitWorktreeData{
			RepoPath:      "/tmp/repo",
			WorktreePath:  "/tmp/repo/.worktrees/persisted",
			SessionName:   "persisted",
			BranchName:    "feature/test",
			BaseCommitSHA: "abc123",
		},
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.Equal(t, "plan", inst.TaskFile)

	roundTrip := inst.ToInstanceData()
	assert.Equal(t, "plan", roundTrip.TaskFile)
}

func TestInstanceData_RoundTripImplementationComplete(t *testing.T) {
	data := InstanceData{
		Title:                  "coder-done",
		Path:                   "/tmp/repo",
		Branch:                 "feature/impl",
		Status:                 Paused,
		Program:                "opencode",
		TaskFile:               "plan",
		ImplementationComplete: true,
		Worktree: GitWorktreeData{
			RepoPath:      "/tmp/repo",
			WorktreePath:  "/tmp/repo/.worktrees/coder-done",
			SessionName:   "coder-done",
			BranchName:    "feature/impl",
			BaseCommitSHA: "def456",
		},
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.True(t, inst.ImplementationComplete)

	roundTrip := inst.ToInstanceData()
	assert.True(t, roundTrip.ImplementationComplete)
}

func TestNewInstance_SetsAgentType(t *testing.T) {
	inst, err := NewInstance(InstanceOptions{
		Title:     "planner-worker",
		Path:      ".",
		Program:   "opencode",
		TaskFile:  "auth-refactor",
		AgentType: AgentTypePlanner,
	})
	require.NoError(t, err)
	assert.Equal(t, AgentTypePlanner, inst.AgentType)
}

func TestInstanceData_RoundTripAgentType(t *testing.T) {
	data := InstanceData{
		Title:     "persisted",
		Path:      "/tmp/repo",
		Branch:    "plan/auth-refactor",
		Status:    Paused,
		Program:   "opencode",
		TaskFile:  "auth-refactor",
		AgentType: AgentTypeReviewer,
		Worktree: GitWorktreeData{
			RepoPath:      "/tmp/repo",
			WorktreePath:  "/tmp/repo/.worktrees/plan-auth-refactor",
			SessionName:   "persisted",
			BranchName:    "plan/auth-refactor",
			BaseCommitSHA: "abc123",
		},
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.Equal(t, AgentTypeReviewer, inst.AgentType)

	roundTrip := inst.ToInstanceData()
	assert.Equal(t, AgentTypeReviewer, roundTrip.AgentType)
	assert.False(t, roundTrip.IsReviewer, "deprecated IsReviewer field should not be written for new state")
}

func TestFromInstanceData_MigratesLegacyReviewerFlag(t *testing.T) {
	data := InstanceData{
		Title:      "legacy-reviewer",
		Path:       "/tmp/repo",
		Branch:     "plan/auth-refactor",
		Status:     Paused,
		Program:    "opencode",
		TaskFile:   "auth-refactor",
		IsReviewer: true,
		Worktree: GitWorktreeData{
			RepoPath:      "/tmp/repo",
			WorktreePath:  "/tmp/repo/.worktrees/legacy-reviewer",
			SessionName:   "legacy-reviewer",
			BranchName:    "plan/auth-refactor",
			BaseCommitSHA: "abc123",
		},
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.Equal(t, AgentTypeReviewer, inst.AgentType)
	assert.True(t, inst.IsReviewer, "legacy reviewer flag should still hydrate compatibility mirror")
}

func TestInstanceData_ImplementationCompleteFalseByDefault(t *testing.T) {
	data := InstanceData{
		Title:   "normal-session",
		Path:    "/tmp/repo",
		Branch:  "feature/x",
		Status:  Paused,
		Program: "claude",
		Worktree: GitWorktreeData{
			RepoPath:      "/tmp/repo",
			WorktreePath:  "/tmp/repo/.worktrees/normal-session",
			SessionName:   "normal-session",
			BranchName:    "feature/x",
			BaseCommitSHA: "aaa111",
		},
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.False(t, inst.ImplementationComplete)
}

func TestInstanceData_RoundTripSoloAgent(t *testing.T) {
	inst, err := NewInstance(InstanceOptions{
		Title:   "solo-worker",
		Path:    "/tmp/repo",
		Program: "opencode",
	})
	require.NoError(t, err)
	inst.SoloAgent = true

	data := inst.ToInstanceData()
	restored, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.True(t, restored.SoloAgent)
}

// TestInstanceData_RoundTripExecutionMode verifies that ExecutionMode survives a
// full InstanceData round-trip, and that the empty string normalises to tmux.
func TestInstanceData_RoundTripExecutionMode(t *testing.T) {
	tests := []struct {
		name     string
		input    ExecutionMode
		expected ExecutionMode
	}{
		{"headless preserved", ExecutionModeHeadless, ExecutionModeHeadless},
		{"tmux preserved", ExecutionModeTmux, ExecutionModeTmux},
		{"empty defaults to tmux", "", ExecutionModeTmux},
		{"unknown defaults to tmux", ExecutionMode("unknown"), ExecutionModeTmux},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := InstanceData{
				Title:         "mode-test",
				Path:          "/tmp/repo",
				Branch:        "feature/test",
				Status:        Paused,
				Program:       "claude",
				ExecutionMode: tt.input,
				Worktree: GitWorktreeData{
					RepoPath:      "/tmp/repo",
					WorktreePath:  "/tmp/repo/.worktrees/mode-test",
					SessionName:   "mode-test",
					BranchName:    "feature/test",
					BaseCommitSHA: "abc123",
				},
			}

			inst, err := FromInstanceData(data)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, inst.ExecutionMode)

			roundTrip := inst.ToInstanceData()
			assert.Equal(t, tt.expected, roundTrip.ExecutionMode)
		})
	}
}
