package session

import (
	"fmt"
	"runtime"
	"testing"

	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentTypeElaborator_Constant(t *testing.T) {
	// AgentTypeElaborator was renamed from "elaborator" to "architect" to match
	// the opencode config block name after the elaborator→architect role rename.
	assert.Equal(t, "architect", AgentTypeElaborator)
}

func TestAgentTypeArchitectBaseline_Constant(t *testing.T) {
	assert.Equal(t, "architect-baseline", AgentTypeArchitectBaseline)
	assert.NotEqual(t, AgentTypeElaborator, AgentTypeArchitectBaseline)
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

func TestInstanceData_RoundTripArchitectBaselineAgentType(t *testing.T) {
	data := InstanceData{
		Title:     "feature-architect-baseline",
		Path:      "/tmp/repo",
		Status:    Paused,
		Program:   "opencode",
		TaskFile:  "feature",
		AgentType: AgentTypeArchitectBaseline,
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.Equal(t, "feature", inst.TaskFile)
	assert.Equal(t, AgentTypeArchitectBaseline, inst.AgentType)
	assert.False(t, inst.sharedWorktree)

	roundTrip := inst.ToInstanceData()
	assert.Equal(t, "feature", roundTrip.TaskFile)
	assert.Equal(t, AgentTypeArchitectBaseline, roundTrip.AgentType)
}

func TestFromInstanceData_RestoresSharedTaskWorktree(t *testing.T) {
	data := InstanceData{
		Title:       "feature-review-2",
		Path:        "/tmp/repo",
		Branch:      "plan/feature",
		Status:      Paused,
		Program:     "opencode",
		TaskFile:    "feature",
		AgentType:   AgentTypeReviewer,
		ReviewCycle: 2,
		Worktree: GitWorktreeData{
			RepoPath:     "/tmp/repo",
			WorktreePath: gitpkg.TaskWorktreePath("/tmp/repo", "plan/feature"),
			SessionName:  "feature-review-2",
			BranchName:   "plan/feature",
		},
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.True(t, inst.sharedWorktree)
	assert.Equal(t, "plan/feature", inst.Branch)
	assert.Equal(t, 2, inst.ReviewCycle)
}

func TestBindSharedTaskWorktree(t *testing.T) {
	inst, err := NewInstance(InstanceOptions{Title: "feature-W2-T2", Path: "/tmp/repo", Program: "opencode", TaskFile: "feature", AgentType: AgentTypeCoder, WaveNumber: 2, TaskNumber: 2})
	require.NoError(t, err)

	inst.BindSharedTaskWorktree("/tmp/repo", "plan/feature")

	assert.True(t, inst.sharedWorktree)
	assert.Equal(t, "plan/feature", inst.Branch)
	require.NotNil(t, inst.gitWorktree)
	assert.Equal(t, gitpkg.TaskWorktreePath("/tmp/repo", "plan/feature"), inst.gitWorktree.GetWorktreePath())
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

func TestInstanceData_RoundTripDisplayTitle(t *testing.T) {
	inst, err := NewInstance(InstanceOptions{
		Title:   "agent-1",
		Path:    "/tmp/repo",
		Program: "opencode",
	})
	require.NoError(t, err)
	inst.DisplayTitle = "ship-auth-ui"

	data := inst.ToInstanceData()
	assert.Equal(t, "ship-auth-ui", data.DisplayTitle)

	restored, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.Equal(t, "ship-auth-ui", restored.DisplayTitle)
	assert.Equal(t, "ship-auth-ui", restored.DisplayName())
}

func TestFromInstanceData_DeadSessionRestoreDoesNotNotifyAgain(t *testing.T) {
	origEnabled := NotificationsEnabled
	origLookPath := notifyLookPath
	origStart := notifyStart
	NotificationsEnabled = true
	defer func() {
		NotificationsEnabled = origEnabled
		notifyLookPath = origLookPath
		notifyStart = origStart
	}()

	started := false
	notifyLookPath = func(file string) (string, error) {
		if runtime.GOOS == "linux" {
			return "/usr/bin/notify-send", nil
		}
		return "", fmt.Errorf("lookup not used on %s", runtime.GOOS)
	}
	notifyStart = func(name string, args ...string) error {
		started = true
		return nil
	}

	data := InstanceData{
		Title:         "dead-planner",
		Path:          "/tmp/repo",
		Branch:        "feature/test",
		Status:        Running,
		Program:       "opencode",
		ExecutionMode: ExecutionModeSDK,
		TaskFile:      "my-plan",
		AgentType:     AgentTypePlanner,
		Worktree: GitWorktreeData{
			RepoPath:      "/tmp/repo",
			WorktreePath:  "/tmp/repo/.worktrees/dead-planner",
			SessionName:   "dead-planner",
			BranchName:    "feature/test",
			BaseCommitSHA: "abc123",
		},
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.True(t, inst.started)
	assert.True(t, inst.Exited)
	assert.Equal(t, Ready, inst.Status)
	assert.True(t, inst.Notified)
	assert.False(t, started, "restoring a dead saved instance must not re-send desktop notifications")
}

// TestInstanceData_RoundTripExecutionMode verifies that ExecutionMode survives a
// full InstanceData round-trip. ResolveExecutionMode is used so the actual process
// host is always stored (e.g. legacy "headless" normalises to "sdk" for claude).
func TestInstanceData_RoundTripExecutionMode(t *testing.T) {
	tests := []struct {
		name     string
		input    ExecutionMode
		expected ExecutionMode
	}{
		// "headless" is a legacy config string that normalises to "sdk"; claude supports SDK.
		{"headless maps to sdk for claude", ExecutionMode("headless"), ExecutionModeSDK},
		{"sdk preserved for claude", ExecutionModeSDK, ExecutionModeSDK},
		{"tmux preserved", ExecutionModeTmux, ExecutionModeTmux},
		// Empty/unknown defaults to tmux (session layer conservative default).
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

func TestInstanceData_RoundTripWaveTaskMetadata(t *testing.T) {
	data := InstanceData{
		Title:         "wave-coder-2",
		Path:          "/tmp/repo",
		Branch:        "plan/feature",
		Status:        Paused,
		Program:       "claude",
		TaskFile:      "feature",
		AgentType:     AgentTypeCoder,
		WaveNumber:    3,
		TaskNumber:    5,
		PeerCount:     6,
		WaveTaskIndex: 2,
		WaveTaskCount: 6,
		Worktree: GitWorktreeData{
			RepoPath:      "/tmp/repo",
			WorktreePath:  "/tmp/repo/.worktrees/wave-coder-2",
			SessionName:   "wave-coder-2",
			BranchName:    "plan/feature",
			BaseCommitSHA: "abc123",
		},
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.Equal(t, 2, inst.WaveTaskIndex, "WaveTaskIndex must be restored from persisted data")
	assert.Equal(t, 6, inst.WaveTaskCount, "WaveTaskCount must be restored from persisted data")

	roundTrip := inst.ToInstanceData()
	assert.Equal(t, 2, roundTrip.WaveTaskIndex, "WaveTaskIndex must survive ToInstanceData round-trip")
	assert.Equal(t, 6, roundTrip.WaveTaskCount, "WaveTaskCount must survive ToInstanceData round-trip")
}

func TestInstanceData_WaveTaskMetadataZeroFromOldState(t *testing.T) {
	// Simulate old persisted state that has no wave_task_index / wave_task_count keys.
	data := InstanceData{
		Title:   "old-instance",
		Path:    "/tmp/repo",
		Branch:  "feature/legacy",
		Status:  Paused,
		Program: "opencode",
		Worktree: GitWorktreeData{
			RepoPath:      "/tmp/repo",
			WorktreePath:  "/tmp/repo/.worktrees/old-instance",
			SessionName:   "old-instance",
			BranchName:    "feature/legacy",
			BaseCommitSHA: "aaa111",
		},
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.Equal(t, 0, inst.WaveTaskIndex, "missing wave_task_index in old state must restore as zero")
	assert.Equal(t, 0, inst.WaveTaskCount, "missing wave_task_count in old state must restore as zero")
}

func TestFromInstanceData_PausedMainBranchLeavesWorktreeNil(t *testing.T) {
	data := InstanceData{
		Title:   "planner-main",
		Path:    "/tmp/repo",
		Status:  Paused,
		Program: "opencode",
		// Empty Worktree block — main-branch instance.
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.True(t, inst.Started(), "paused instances should be marked started")
	assert.Nil(t, inst.gitWorktree, "main-branch instance should have nil gitWorktree")
	assert.NotNil(t, inst.executionSession, "execution session should be prepared")
	assert.Equal(t, Paused, inst.Status)

	// Round-trip: the empty worktree should stay empty.
	roundTrip := inst.ToInstanceData()
	assert.Empty(t, roundTrip.Worktree.RepoPath)
	assert.Empty(t, roundTrip.Worktree.BranchName)

	// Restore again and confirm nil is preserved.
	inst2, err := FromInstanceData(roundTrip)
	require.NoError(t, err)
	assert.Nil(t, inst2.gitWorktree)
}

// TestNewInstance_SDKSpeedTier_GatedOnCodexSDK verifies that speed tiers are only
// stored when ExecutionMode is SDK and the program is Codex.
func TestNewInstance_SDKSpeedTier_GatedOnCodexSDK(t *testing.T) {
	tests := []struct {
		name          string
		program       string
		mode          ExecutionMode
		tier          string
		wantSpeedTier string
	}{
		{
			name:          "fast codex sdk stores fast",
			program:       "codex",
			mode:          ExecutionModeSDK,
			tier:          "fast",
			wantSpeedTier: "fast",
		},
		{
			name:          "flex codex sdk stores flex",
			program:       "codex",
			mode:          ExecutionModeSDK,
			tier:          "flex",
			wantSpeedTier: "flex",
		},
		{
			name:          "default alias stores flex",
			program:       "codex",
			mode:          ExecutionModeSDK,
			tier:          "default",
			wantSpeedTier: "flex",
		},
		{
			name:          "fast claude sdk is ignored (claude has no tier)",
			program:       "claude",
			mode:          ExecutionModeSDK,
			tier:          "fast",
			wantSpeedTier: "",
		},
		{
			name:          "fast codex tmux is ignored (not sdk)",
			program:       "codex",
			mode:          ExecutionModeTmux,
			tier:          "fast",
			wantSpeedTier: "",
		},
		{
			name:          "unknown tier normalises to empty",
			program:       "codex",
			mode:          ExecutionModeSDK,
			tier:          "priority",
			wantSpeedTier: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inst, err := NewInstance(InstanceOptions{
				Title:         "test-speed-tier",
				Path:          ".",
				Program:       tc.program,
				ExecutionMode: tc.mode,
				SDKSpeedTier:  tc.tier,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.wantSpeedTier, inst.SDKSpeedTier)
		})
	}
}

// TestInstanceData_SDKSpeedTier_FromInstanceDataRoundTrip verifies that a fast-tier
// codex instance survives the InstanceData → Instance → InstanceData cycle.
func TestInstanceData_SDKSpeedTier_FromInstanceDataRoundTrip(t *testing.T) {
	data := InstanceData{
		Title:         "fast-codex-rt",
		Path:          "/tmp/repo",
		Branch:        "plan/fast-rt",
		Status:        Paused,
		Program:       "codex",
		ExecutionMode: ExecutionModeSDK,
		SDKSpeedTier:  "fast",
		Worktree: GitWorktreeData{
			RepoPath:     "/tmp/repo",
			WorktreePath: "/tmp/repo/.worktrees/fast-codex-rt",
			SessionName:  "fast-codex-rt",
			BranchName:   "plan/fast-rt",
		},
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.Equal(t, "fast", inst.SDKSpeedTier)

	roundTrip := inst.ToInstanceData()
	assert.Equal(t, "fast", roundTrip.SDKSpeedTier)
}

func TestInstanceData_SDKTranscriptLimits_FromInstanceDataRoundTrip(t *testing.T) {
	data := InstanceData{
		Title:                  "retained-sdk",
		Path:                   "/tmp/repo",
		Branch:                 "plan/retained-sdk",
		Status:                 Paused,
		Program:                "codex",
		ExecutionMode:          ExecutionModeSDK,
		SDKTranscriptLimitsSet: true,
		SDKTranscriptMaxBytes:  2 << 20,
		SDKTranscriptMaxTurns:  250,
		Worktree: GitWorktreeData{
			RepoPath:     "/tmp/repo",
			WorktreePath: "/tmp/repo/.worktrees/retained-sdk",
			SessionName:  "retained-sdk",
			BranchName:   "plan/retained-sdk",
		},
	}

	inst, err := FromInstanceData(data)
	require.NoError(t, err)
	assert.True(t, inst.SDKTranscriptLimitsSet)
	assert.Equal(t, int64(2<<20), inst.SDKTranscriptMaxBytes)
	assert.Equal(t, int64(250), inst.SDKTranscriptMaxTurns)

	roundTrip := inst.ToInstanceData()
	assert.True(t, roundTrip.SDKTranscriptLimitsSet)
	assert.Equal(t, int64(2<<20), roundTrip.SDKTranscriptMaxBytes)
	assert.Equal(t, int64(250), roundTrip.SDKTranscriptMaxTurns)
}
