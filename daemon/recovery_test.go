package daemon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestDaemon_RecoverSessions_RespawnsMissingTmuxReviewer pins recovery for
// tmux-mode agents.
//
// Recovery used to bail on anything whose execution_mode was not "sdk". Every
// agent in the matchfi repo is tmux, so no task there was ever recovered -- a
// daemon restart parked the whole queue with live-looking agents recorded against
// tasks that had no process behind them, and only a human driving tasks by hand
// got anything moving again. Nothing in the respawn path was ever SDK-specific,
// so the gate was subtracting recovery rather than enabling it.
//
// The one genuinely SDK-specific piece is the orphan reap: it pgreps for a
// detached app-server process, which a tmux agent never leaves behind. That must
// stay off for tmux, so this asserts it is not called.
func TestDaemon_RecoverSessions_RespawnsMissingTmuxReviewer(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeRecoveryConfig(t, repoDir, `
[phases]
  reviewing = "reviewer"

[agents]
  [agents.reviewer]
    enabled = true
    program = "claude"
    execution_mode = "tmux"
    permission_default = "prompt"
`)

	project := filepath.Base(repoDir)
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "feature",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/feature",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		},
	}))

	var (
		reapCalls  int
		spawnCount int
		spawned    loop.SpawnOpts
	)
	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		reapSDKOrphan: func(string, string, string) error {
			reapCalls++
			return nil
		},
		spawnReviewer: func(_ context.Context, opts loop.SpawnOpts) error {
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
	assert.Equal(t, 1, spawnCount, "a tmux reviewer with no session must be respawned")
	assert.Equal(t, "feature", spawned.PlanFile)
	assert.Equal(t, config.ExecutionModeTmux, spawned.ExecutionMode)
	assert.Zero(t, reapCalls, "tmux agents leave no app-server process to reap")
}

// A blocked task is indistinguishable from a stalled one by shape alone: both
// sit in implementing/reviewing/verifying with no agent behind them. Recovery
// must tell them apart from the block, because this sweep is driven by nothing
// -- it runs on every boot and every timer tick regardless of signals, so
// treating a block as a stall respawns the same agent forever on a question no
// agent can answer. Observed for real: two blocked tasks were handed fresh
// reviewers 97 seconds after a daemon restart, ~250k tokens after the block.
func TestDaemon_RecoverSessions_SkipsBlockedTask(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeRecoveryConfig(t, repoDir, `
[phases]
  reviewing = "reviewer"

[agents]
  [agents.reviewer]
    enabled = true
    program = "claude"
    execution_mode = "tmux"
    permission_default = "prompt"
`)

	project := filepath.Base(repoDir)
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "feature",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/feature",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		},
	}))
	require.NoError(t, store.SetBlocked(project, "feature", "pick (a) or (b)", "agent"))

	spawnCount := 0
	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnReviewer: func(context.Context, loop.SpawnOpts) error {
			spawnCount++
			return nil
		},
	}
	d.repos.repos = []RepoEntry{{Path: repoDir, Project: project, Store: store}}

	recovered, err := d.RecoverSessions()
	require.NoError(t, err)
	assert.Zero(t, recovered)
	assert.Zero(t, spawnCount, "a blocked task must not be respawned by recovery")
}

// TestDaemon_ReconcileMissingManagedAgents_HonoursGrace pins the window that
// keeps the periodic sweep from racing a spawn that is already in flight. The
// sweep runs alongside normal lifecycle spawning, and a request to start an agent
// is not the same instant as its tmux session existing; respawning inside that gap
// would put two agents on one worktree, both committing to the same branch.
//
// Startup passes zero grace instead, because nothing can be mid-spawn in a daemon
// that has not begun polling -- that path is covered by the RecoverSessions tests
// above, which would hang on the first sweep if grace applied there.
func TestDaemon_ReconcileMissingManagedAgents_HonoursGrace(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeRecoveryConfig(t, repoDir, `
[phases]
  reviewing = "reviewer"

[agents]
  [agents.reviewer]
    enabled = true
    program = "claude"
    execution_mode = "tmux"
    permission_default = "prompt"
`)

	project := filepath.Base(repoDir)
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "feature",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/feature",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		},
	}))

	spawnCount := 0
	d := &Daemon{
		repos:         NewRepoManager(),
		spawner:       NewTmuxSpawner(),
		logger:        slog.Default(),
		broadcaster:   api.NewEventBroadcaster(),
		reapSDKOrphan: func(string, string, string) error { return nil },
		spawnReviewer: func(_ context.Context, _ loop.SpawnOpts) error {
			spawnCount++
			return nil
		},
	}
	entries := []RepoEntry{{Path: repoDir, Project: project, Store: store}}
	d.repos.repos = entries

	ctx := context.Background()
	require.NoError(t, d.reconcileMissingManagedAgents(ctx, entries, time.Hour))
	assert.Zero(t, spawnCount, "first sighting only records the absence")
	require.NoError(t, d.reconcileMissingManagedAgents(ctx, entries, time.Hour))
	assert.Zero(t, spawnCount, "still inside the grace window")

	// Back-date the recorded sighting rather than sleeping an hour. Keyed off
	// whatever the candidate title turned out to be (reviewer titles carry a round
	// number) so the test pins the grace behaviour, not the naming scheme.
	d.mu.Lock()
	require.Len(t, d.agentMissingSince, 1)
	for key := range d.agentMissingSince {
		d.agentMissingSince[key] = time.Now().Add(-2 * time.Hour)
	}
	d.mu.Unlock()

	require.NoError(t, d.reconcileMissingManagedAgents(ctx, entries, time.Hour))
	assert.Equal(t, 1, spawnCount, "absent for longer than the grace window must respawn")
}

// TestDaemon_SweepMissingAgents_RateLimits pins the interval gate that lets the
// sweep hang off the poll loop at all.
//
// tick() fires every second. Re-listing every task in every repo and shelling out
// to tmux at that rate would cost far more than it recovers, so the sweep is what
// makes mid-run recovery affordable enough to run continuously -- without it the
// only options are boot-time-only recovery (which is what this whole change is
// fixing) or a poll loop that spends its time asking tmux questions.
func TestDaemon_SweepMissingAgents_RateLimits(t *testing.T) {
	t.Parallel()

	d := &Daemon{
		repos:   NewRepoManager(),
		spawner: NewTmuxSpawner(),
		logger:  slog.Default(),
	}

	ctx := context.Background()
	d.sweepMissingAgents(ctx)
	first := d.lastAgentSweep
	require.False(t, first.IsZero(), "the first sweep must run")

	d.sweepMissingAgents(ctx)
	assert.Equal(t, first, d.lastAgentSweep, "a second sweep inside the interval must be skipped")

	d.mu.Lock()
	d.lastAgentSweep = time.Now().Add(-2 * agentSweepInterval)
	d.mu.Unlock()

	d.sweepMissingAgents(ctx)
	assert.True(t, d.lastAgentSweep.After(first), "a sweep past the interval must run again")
}

func writeRecoveryConfig(t *testing.T, repoDir, body string) {
	t.Helper()

	configDir := filepath.Join(repoDir, ".kasmos")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, config.TOMLConfigFileName), []byte(strings.TrimSpace(body)+"\n"), 0o644))
}
