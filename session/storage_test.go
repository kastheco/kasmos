package session

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStateManager struct {
	helpScreensSeen uint32
	instances       json.RawMessage
}

func (m *mockStateManager) SaveInstances(instancesJSON json.RawMessage) error {
	m.instances = instancesJSON
	return nil
}

func (m *mockStateManager) GetInstances() json.RawMessage {
	if m.instances == nil {
		return json.RawMessage("[]")
	}
	return m.instances
}

func (m *mockStateManager) DeleteAllInstances() error {
	m.instances = json.RawMessage("[]")
	return nil
}

func (m *mockStateManager) GetHelpScreensSeen() uint32 {
	return m.helpScreensSeen
}

func (m *mockStateManager) SetHelpScreensSeen(seen uint32) error {
	m.helpScreensSeen = seen
	return nil
}

func TestLoadInstances_DropsStaleWaveInstancesWithoutTmuxSession(t *testing.T) {
	repoDir := t.TempDir()
	nonce := time.Now().UnixNano()

	records := []InstanceData{
		{
			Title:      fmt.Sprintf("stale-wave-%d", nonce),
			Path:       repoDir,
			Status:     Paused,
			Program:    "opencode",
			TaskFile:   "stale-wave",
			TaskNumber: 1,
			WaveNumber: 1,
			Worktree: GitWorktreeData{
				RepoPath:     repoDir,
				WorktreePath: repoDir,
				SessionName:  fmt.Sprintf("stale-wave-%d", nonce),
				BranchName:   "plan/stale-wave",
			},
		},
		{
			Title:   fmt.Sprintf("keep-fixer-%d", nonce),
			Path:    repoDir,
			Status:  Ready,
			Program: "opencode",
		},
	}

	raw, err := json.Marshal(records)
	require.NoError(t, err)

	state := &mockStateManager{instances: raw}
	storage, err := NewStorage(state)
	require.NoError(t, err)

	instances, err := storage.LoadInstances()
	require.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, records[1].Title, instances[0].Title)
}

// TestLoadInstances_DropsStaleSDKWaveInstances verifies that wave-task records whose
// SDK session no longer exists are dropped on reload, the same way stale tmux wave
// records are dropped. "claude" is used because it is an SDK-supported program;
// the session won't exist, so the record should be filtered out.
func TestLoadInstances_DropsStaleSDKWaveInstances(t *testing.T) {
	repoDir := t.TempDir()
	nonce := time.Now().UnixNano()

	records := []InstanceData{
		{
			Title:         fmt.Sprintf("stale-sdk-wave-%d", nonce),
			Path:          repoDir,
			Status:        Paused,
			Program:       "claude",
			ExecutionMode: ExecutionModeSDK,
			TaskFile:      "stale-sdk",
			TaskNumber:    2,
			WaveNumber:    1,
			Worktree: GitWorktreeData{
				RepoPath:     repoDir,
				WorktreePath: repoDir,
				SessionName:  fmt.Sprintf("stale-sdk-wave-%d", nonce),
				BranchName:   "plan/stale-sdk",
			},
		},
		{
			Title:   fmt.Sprintf("keep-non-wave-%d", nonce),
			Path:    repoDir,
			Status:  Ready,
			Program: "claude",
		},
	}

	raw, err := json.Marshal(records)
	require.NoError(t, err)

	state := &mockStateManager{instances: raw}
	storage, err := NewStorage(state)
	require.NoError(t, err)

	instances, err := storage.LoadInstances()
	require.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, records[1].Title, instances[0].Title)
}

// TestLoadInstances_DropsStaleHeadlessWaveInstances verifies that wave records
// persisted with the legacy "headless" execution mode are also dropped on reload
// when the underlying session no longer exists. The "headless" string normalises
// to SDK before the liveness check, so the behaviour is identical to SDK records.
func TestLoadInstances_DropsStaleHeadlessWaveInstances(t *testing.T) {
	repoDir := t.TempDir()
	nonce := time.Now().UnixNano()

	records := []InstanceData{
		{
			Title:         fmt.Sprintf("stale-headless-wave-%d", nonce),
			Path:          repoDir,
			Status:        Paused,
			Program:       "claude",
			ExecutionMode: ExecutionMode("headless"), // legacy value from old persisted state
			TaskFile:      "stale-hl",
			TaskNumber:    1,
			WaveNumber:    1,
			Worktree: GitWorktreeData{
				RepoPath:     repoDir,
				WorktreePath: repoDir,
				SessionName:  fmt.Sprintf("stale-headless-wave-%d", nonce),
				BranchName:   "plan/stale-hl",
			},
		},
		{
			Title:   fmt.Sprintf("keep-fixer-hl-%d", nonce),
			Path:    repoDir,
			Status:  Ready,
			Program: "opencode",
		},
	}

	raw, err := json.Marshal(records)
	require.NoError(t, err)

	state := &mockStateManager{instances: raw}
	storage, err := NewStorage(state)
	require.NoError(t, err)

	instances, err := storage.LoadInstances()
	require.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, records[1].Title, instances[0].Title)
}

func TestLoadInstances_PreservesPausedNonWaveInstanceWithoutTmuxSession(t *testing.T) {
	repoDir := t.TempDir()
	nonce := time.Now().UnixNano()

	records := []InstanceData{
		{
			Title:   fmt.Sprintf("paused-solo-%d", nonce),
			Path:    repoDir,
			Status:  Paused,
			Program: "opencode",
			Worktree: GitWorktreeData{
				RepoPath:     repoDir,
				WorktreePath: repoDir,
				SessionName:  fmt.Sprintf("paused-solo-%d", nonce),
			},
		},
	}

	raw, err := json.Marshal(records)
	require.NoError(t, err)

	state := &mockStateManager{instances: raw}
	storage, err := NewStorage(state)
	require.NoError(t, err)

	instances, err := storage.LoadInstances()
	require.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, records[0].Title, instances[0].Title)
	assert.True(t, instances[0].Paused())
}

// TestInstanceData_SDKSpeedTier_RoundTrip verifies that the sdk_speed_tier JSON
// field is preserved through a marshal/unmarshal cycle and omitted when empty.
func TestInstanceData_SDKSpeedTier_RoundTrip(t *testing.T) {
	t.Run("fast tier persists and restores", func(t *testing.T) {
		data := InstanceData{
			Title:         "fast-codex",
			Path:          "/tmp/repo",
			Branch:        "plan/fast-codex",
			Status:        Paused,
			Program:       "codex",
			ExecutionMode: ExecutionModeSDK,
			SDKSpeedTier:  "fast",
			Worktree: GitWorktreeData{
				RepoPath:     "/tmp/repo",
				WorktreePath: "/tmp/repo/.worktrees/fast-codex",
				SessionName:  "fast-codex",
				BranchName:   "plan/fast-codex",
			},
		}

		raw, err := json.Marshal(data)
		require.NoError(t, err)

		// Verify the field is present in JSON.
		assert.Contains(t, string(raw), `"sdk_speed_tier":"fast"`)

		var restored InstanceData
		require.NoError(t, json.Unmarshal(raw, &restored))
		assert.Equal(t, "fast", restored.SDKSpeedTier)
	})

	t.Run("empty tier is omitted from JSON", func(t *testing.T) {
		data := InstanceData{
			Title:   "default-codex",
			Path:    "/tmp/repo",
			Program: "codex",
		}

		raw, err := json.Marshal(data)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "sdk_speed_tier")
	})
}

func TestInstanceData_SDKTranscriptLimits_RoundTrip(t *testing.T) {
	data := InstanceData{
		Title:                  "retained-codex",
		Path:                   "/tmp/repo",
		Branch:                 "plan/retained-codex",
		Status:                 Paused,
		Program:                "codex",
		ExecutionMode:          ExecutionModeSDK,
		SDKTranscriptLimitsSet: true,
		SDKTranscriptMaxBytes:  0,
		SDKTranscriptMaxTurns:  250,
		Worktree: GitWorktreeData{
			RepoPath:     "/tmp/repo",
			WorktreePath: "/tmp/repo/.worktrees/retained-codex",
			SessionName:  "retained-codex",
			BranchName:   "plan/retained-codex",
		},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"sdk_transcript_limits_set":true`)
	assert.Contains(t, string(raw), `"sdk_transcript_max_turns":250`)

	var restored InstanceData
	require.NoError(t, json.Unmarshal(raw, &restored))
	assert.True(t, restored.SDKTranscriptLimitsSet)
	assert.Equal(t, int64(0), restored.SDKTranscriptMaxBytes)
	assert.Equal(t, int64(250), restored.SDKTranscriptMaxTurns)
}

// TestInstanceData_ResourceProfile_RoundTrip verifies that ResourceProfile is
// serialised and deserialised correctly and that the full resolved policy is
// NOT stored (i.e. nice/env values are not emitted in the JSON).
func TestInstanceData_ResourceProfile_RoundTrip(t *testing.T) {
	t.Parallel()

	data := InstanceData{
		Title:           "test-rc",
		Program:         "claude",
		ResourceProfile: "interactive",
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"resource_profile":"interactive"`)
	// Full resolved policy values must not appear in JSON.
	assert.NotContains(t, string(raw), `"nice"`, "resolved nice value must not be persisted")
	assert.NotContains(t, string(raw), `"ionice"`, "resolved ionice value must not be persisted")

	var restored InstanceData
	require.NoError(t, json.Unmarshal(raw, &restored))
	assert.Equal(t, "interactive", restored.ResourceProfile)
}

// TestInstanceData_ResourceProfile_OmitNormal verifies that the normal profile
// ("") is omitted from JSON (omitempty) to avoid polluting existing state files.
func TestInstanceData_ResourceProfile_OmitNormal(t *testing.T) {
	t.Parallel()

	data := InstanceData{
		Title:   "test-rc-normal",
		Program: "claude",
		// ResourceProfile zero value ("") represents normal / no-op.
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `resource_profile`, "zero-value ResourceProfile must be omitted")
}
