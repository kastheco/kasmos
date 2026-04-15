package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kastheco/kasmos/config/taskactions"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDaemon_TickRepo_HTTPPlannerFinishedSignalSpawnsArchitect proves that a
// planner_finished signal originating from an HTTP lifecycle transition (via the
// taskactions handler) is correctly consumed by daemon.tickRepo and causes the
// architect spawn path to execute — matching the regression shape described in
// the plan for web-admin-transitions-spawn-workers.
func TestDaemon_TickRepo_HTTPPlannerFinishedSignalSpawnsArchitect(t *testing.T) {
	t.Parallel()

	// ── shared in-memory SQLite DB so store and gateway see the same data ──
	db, err := taskstore.OpenSharedDB(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)

	gw, err := taskstore.NewSQLiteSignalGatewayFromDB(db)
	require.NoError(t, err)

	const (
		project  = "proj"
		planFile = "feature"
		branch   = "plan/feature"
	)

	// ── seed task in planning status with a Large multi-task plan ──
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusPlanning,
		Branch:   branch,
	}))

	// Large plan with 3 tasks forces the architect (elaborator) path; a plan
	// with ≤ blueprint-skip threshold tasks would spawn a coder instead.
	const planContent = `# Plan

**Goal:** prove HTTP-originated planner_finished reaches the daemon architect path

**Architecture:** daemon-routed lifecycle with architect handoff and multi-wave coding

**Tech Stack:** Go, SQLite

**Size:** Large

---

## Wave 1

### Task 1: First task

Implement the first part.

### Task 2: Second task

Implement the second part.

### Task 3: Third task

Implement the third part.
`
	require.NoError(t, store.SetContent(project, planFile, planContent))

	// ── HTTP server using the taskactions handler with the real gateway ──
	srv := httptest.NewServer(taskactions.NewHandler(store, gw))
	t.Cleanup(srv.Close)

	// POST planner_finished — this is the HTTP transition that must create a
	// gateway row so the daemon's tick loop can consume it.
	reqBody, _ := json.Marshal(map[string]string{"event": "planner_finished"})
	resp, err := http.Post(
		srv.URL+"/v1/projects/"+project+"/tasks/"+planFile+"/transition",
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Confirm that the HTTP transition left exactly one pending gateway row.
	pending, err := gw.List(project, taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, pending, 1, "HTTP planner_finished must produce one pending gateway signal")

	// ── processor + daemon configured like TestDaemon_AutoAdvancePlannerFinished ──
	proc := loop.NewProcessor(loop.ProcessorConfig{
		Store:       store,
		Project:     project,
		AutoAdvance: true,
	})

	var spawnedOpts loop.SpawnOpts
	spawnCount := 0

	d := &Daemon{
		cfg:         &DaemonConfig{AutoAdvance: true},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		forceKillAgent: func(repoPath, taskFile, agentType string) error {
			// no-op stub — no real planner tmux session exists in this test
			return nil
		},
		spawnElaborator: func(_ context.Context, opts loop.SpawnOpts) error {
			spawnCount++
			spawnedOpts = opts
			return nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })

	repoEntry := RepoEntry{
		Path:          t.TempDir(),
		Project:       project,
		Store:         store,
		SignalGateway: gw,
		Processor:     proc,
	}

	// ── single synchronous tick — no poll loop, no timers ──
	d.tickRepo(context.Background(), repoEntry)

	// ── assertions ──

	// Exactly one architect spawn must have been recorded.
	assert.Equal(t, 1, spawnCount, "tickRepo must spawn exactly one architect")

	// SpawnOpts must identify the correct plan, repo, and project.
	assert.Equal(t, planFile, spawnedOpts.PlanFile)
	assert.Equal(t, repoEntry.Path, spawnedOpts.RepoPath)
	assert.Equal(t, project, spawnedOpts.Project)

	// Task must have advanced to implementing/architecting.
	entry, err := store.Get(project, planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status, "task status must be implementing after architect spawn")
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseArchitecting),
		ActiveAgentType: session.AgentTypeElaborator,
	}, entry.ExecutionState, "execution state must be architecting with elaborator agent type")

	// The gateway row must have been moved to done; no pending rows remain.
	remainingPending, err := gw.List(project, taskstore.SignalPending)
	require.NoError(t, err)
	assert.Empty(t, remainingPending, "no pending signal rows must remain after tickRepo processes the planner_finished signal")

	doneRows, err := gw.List(project, taskstore.SignalDone)
	require.NoError(t, err)
	assert.Len(t, doneRows, 1, "the planner_finished gateway row must be marked done")
}
