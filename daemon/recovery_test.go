package daemon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemon_RecoverSessions_RespawnsMissingSDKArchitect(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeRecoveryConfig(t, repoDir, `
[phases]
  elaborating = "architect"

[agents]
  [agents.architect]
    enabled = true
    program = "codex"
    execution_mode = "sdk"
    permission_default = "prompt"
`)

	project := filepath.Base(repoDir)
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "feature",
		Status:   taskstore.StatusImplementing,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseArchitecting),
			ActiveAgentType: session.AgentTypeElaborator,
		},
	}))

	var (
		reapedProject string
		reapedTitle   string
		reapedProgram string
		spawned       loop.SpawnOpts
		spawnCount    int
	)
	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		reapSDKOrphan: func(project, title, program string) error {
			reapedProject = project
			reapedTitle = title
			reapedProgram = program
			return nil
		},
		spawnElaborator: func(_ context.Context, opts loop.SpawnOpts) error {
			spawnCount++
			spawned = opts
			return nil
		},
	}
	d.repos.repos = []RepoEntry{{
		Path:    repoDir,
		Project: project,
		Store:   store,
	}}

	recovered, err := d.RecoverSessions()
	require.NoError(t, err)
	assert.Zero(t, recovered)
	assert.Equal(t, project, reapedProject)
	assert.Equal(t, "feature-architect", reapedTitle)
	assert.Equal(t, "codex", reapedProgram)
	assert.Equal(t, 1, spawnCount)
	assert.Equal(t, "feature", spawned.PlanFile)
	assert.Equal(t, "codex", spawned.Program)
	assert.Equal(t, config.ExecutionModeSDK, spawned.ExecutionMode)
	assert.False(t, spawned.SkipPermissions)
}

func TestDaemon_RecoverSessions_RespawnsMissingSDKWaveTasks(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeRecoveryConfig(t, repoDir, `
[phases]
  implementing = "coder"

[agents]
  [agents.coder]
    enabled = true
    program = "codex"
    execution_mode = "sdk"
    permission_default = "prompt"
`)

	project := filepath.Base(repoDir)
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "feature",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/feature",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
			ActiveAgentType: session.AgentTypeCoder,
			ActiveWave:      2,
		},
		Content: "# Plan\n\n**Goal:** test\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n\n## Wave 2\n\n### Task 2: Second\n\nDo second.\n\n### Task 3: Third\n\nDo third.\n",
	}))

	var reaped []string
	type waveSpawn struct {
		opts          loop.SpawnOpts
		task          taskparser.Task
		prompt        string
		waveTaskIndex int
		peerCount     int
	}
	var spawned []waveSpawn

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		reapSDKOrphan: func(_ string, title, _ string) error {
			reaped = append(reaped, title)
			return nil
		},
		spawnWaveTask: func(_ context.Context, opts loop.SpawnOpts, task taskparser.Task, prompt string, waveTaskIndex int, peerCount int) error {
			spawned = append(spawned, waveSpawn{
				opts:          opts,
				task:          task,
				prompt:        prompt,
				waveTaskIndex: waveTaskIndex,
				peerCount:     peerCount,
			})
			return nil
		},
	}
	d.repos.repos = []RepoEntry{{
		Path:    repoDir,
		Project: project,
		Store:   store,
	}}

	recovered, err := d.RecoverSessions()
	require.NoError(t, err)
	assert.Zero(t, recovered)
	assert.Equal(t, []string{"feature-W2-T2", "feature-W2-T3"}, reaped)
	require.Len(t, spawned, 2)

	assert.Equal(t, 2, spawned[0].task.Number)
	assert.Equal(t, config.ExecutionModeSDK, spawned[0].opts.ExecutionMode)
	assert.Equal(t, "codex", spawned[0].opts.Program)
	assert.False(t, spawned[0].opts.SkipPermissions)
	assert.Equal(t, 2, spawned[0].opts.Wave)
	assert.Equal(t, 1, spawned[0].waveTaskIndex)
	assert.Equal(t, 2, spawned[0].peerCount)
	assert.Contains(t, spawned[0].prompt, "Implement Task 2: Second")

	assert.Equal(t, 3, spawned[1].task.Number)
	assert.False(t, spawned[1].opts.SkipPermissions)
	assert.Equal(t, 2, spawned[1].waveTaskIndex)
	assert.Equal(t, 2, spawned[1].peerCount)
	assert.Contains(t, spawned[1].prompt, "Implement Task 3: Third")
}

func writeRecoveryConfig(t *testing.T, repoDir, body string) {
	t.Helper()

	configDir := filepath.Join(repoDir, ".kasmos")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, config.TOMLConfigFileName), []byte(strings.TrimSpace(body)+"\n"), 0o644))
}
