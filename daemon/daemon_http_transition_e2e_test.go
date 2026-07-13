package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config/taskactions"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration/loop"
	prsvc "github.com/kastheco/kasmos/orchestration/pr"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
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
	// SetMaxOpenConns(1) ensures all callers share the single :memory: database
	// rather than each getting an independent empty DB from the pool.
	db, err := taskstore.OpenSharedDB(":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
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

// httpTransitionE2EFixture bundles the shared setup used by the HTTP-origin
// daemon regression tests: an in-memory SQLite store, a signal gateway, and an
// httptest server hosting the real taskactions handler. Each test seeds a task
// in the required pre-event status, POSTs the lifecycle transition, then calls
// d.tickRepo so we can assert that the HTTP-originated gateway row reaches its
// daemon side effect (reviewer/fixer/master spawn, VerifyApprovedAction+PR,
// VerifyFailedAction+fixer, etc.).
type httpTransitionE2EFixture struct {
	store    *taskstore.SQLiteStore
	gw       taskstore.SignalGateway
	server   *httptest.Server
	project  string
	planFile string
	branch   string
}

func newHTTPTransitionE2EFixture(t *testing.T, initialStatus taskstore.Status) *httpTransitionE2EFixture {
	t.Helper()

	db, err := taskstore.OpenSharedDB(":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
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

	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   initialStatus,
		Branch:   branch,
	}))

	srv := httptest.NewServer(taskactions.NewHandler(store, gw))
	t.Cleanup(srv.Close)

	return &httpTransitionE2EFixture{
		store:    store,
		gw:       gw,
		server:   srv,
		project:  project,
		planFile: planFile,
		branch:   branch,
	}
}

func (f *httpTransitionE2EFixture) postTransition(t *testing.T, event string) {
	t.Helper()
	reqBody, _ := json.Marshal(map[string]string{"event": event})
	resp, err := http.Post(
		f.server.URL+"/v1/projects/"+f.project+"/tasks/"+f.planFile+"/transition",
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	pending, err := f.gw.List(f.project, taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, pending, 1, "HTTP %s must produce exactly one pending gateway signal", event)
}

func (f *httpTransitionE2EFixture) repoEntry(t *testing.T, proc *loop.Processor) RepoEntry {
	t.Helper()
	return RepoEntry{
		Path:          t.TempDir(),
		Project:       f.project,
		Store:         f.store,
		SignalGateway: f.gw,
		Processor:     proc,
	}
}

func (f *httpTransitionE2EFixture) gitRepoEntry(t *testing.T, proc *loop.Processor) (RepoEntry, string) {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	run("commit", "--allow-empty", "-m", "base")
	run("checkout", "-b", f.branch)
	run("commit", "--allow-empty", "-m", "feature")
	return RepoEntry{Path: repo, Project: f.project, Store: f.store, SignalGateway: f.gw, Processor: proc}, run("rev-parse", "HEAD")
}

// TestDaemon_TickRepo_HTTPImplementFinishedSpawnsReviewer proves an
// implement_finished signal originating from the HTTP handler — which applies
// the FSM transition itself before emitting the gateway row — still reaches
// the reviewer spawn path in the daemon tick loop.
func TestDaemon_TickRepo_HTTPImplementFinishedSpawnsReviewer(t *testing.T) {
	t.Parallel()

	f := newHTTPTransitionE2EFixture(t, taskstore.StatusImplementing)
	f.postTransition(t, "implement_finished")

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: f.store, Project: f.project})

	var spawnedOpts loop.SpawnOpts
	spawnCount := 0

	d := &Daemon{
		cfg:         &DaemonConfig{},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnReviewer: func(_ context.Context, opts loop.SpawnOpts) error {
			spawnCount++
			spawnedOpts = opts
			return nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })

	repoEntry := f.repoEntry(t, proc)
	d.tickRepo(context.Background(), repoEntry)

	assert.Equal(t, 1, spawnCount, "tickRepo must spawn exactly one reviewer")
	assert.Equal(t, f.planFile, spawnedOpts.PlanFile)
	assert.Equal(t, f.branch, spawnedOpts.Branch)
	assert.Equal(t, repoEntry.Path, spawnedOpts.RepoPath)
	assert.Equal(t, f.project, spawnedOpts.Project)

	entry, err := f.store.Get(f.project, f.planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReviewing, entry.Status, "task status must be reviewing after HTTP implement_finished")
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseReviewing),
		ActiveAgentType: session.AgentTypeReviewer,
	}, entry.ExecutionState, "execution state must be reviewing/reviewer after SpawnReviewerAction")

	remainingPending, err := f.gw.List(f.project, taskstore.SignalPending)
	require.NoError(t, err)
	assert.Empty(t, remainingPending, "no pending signal rows must remain after tickRepo")

	doneRows, err := f.gw.List(f.project, taskstore.SignalDone)
	require.NoError(t, err)
	assert.Len(t, doneRows, 1, "the implement_finished gateway row must be marked done")
}

// TestDaemon_TickRepo_HTTPReviewApprovedChainsVerifyApproved proves a
// review_approved signal from the HTTP handler reaches both the reviewer
// side-effects path and the chained verify_approved → CreatePR path in the
// daemon tick loop when the readiness-review gate is disabled.
func TestDaemon_TickRepo_HTTPReviewApprovedChainsVerifyApproved(t *testing.T) {
	t.Parallel()

	f := newHTTPTransitionE2EFixture(t, taskstore.StatusReviewing)
	// Seed a non-empty reviewing execution state so we can later assert that
	// VerifyApprovedAction cleared it.
	require.NoError(t, f.store.SetExecutionState(f.project, f.planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseReviewing),
		ActiveAgentType: session.AgentTypeReviewer,
	}))
	f.postTransition(t, "review_approved")

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: f.store, Project: f.project})

	prCount := 0
	var prPlanFile string

	d := &Daemon{
		cfg:         &DaemonConfig{},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnReviewer: func(_ context.Context, _ loop.SpawnOpts) error {
			t.Fatalf("review_approved must not spawn a reviewer")
			return nil
		},
		spawnMaster: func(_ context.Context, _ loop.SpawnOpts) error {
			t.Fatalf("review_approved must not spawn a master agent when AutoReadinessReview is disabled")
			return nil
		},
		createPR: func(_ RepoEntry, planFile, _ string) (prsvc.Result, error) {
			prCount++
			prPlanFile = planFile
			return prsvc.Result{Outcome: prsvc.OutcomeCreated, URL: "https://example.test/pr/1"}, nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })

	repoEntry, head := f.gitRepoEntry(t, proc)
	proc = loop.NewProcessor(loop.ProcessorConfig{
		Store: f.store, Project: f.project,
		HeadSHA:      func(string) (string, error) { return head, nil },
		MergeBaseSHA: func(string) (string, error) { return gitpkg.DefaultBranchHeadSHA(repoEntry.Path) },
	})
	repoEntry.Processor = proc
	d.tickRepo(context.Background(), repoEntry)

	assert.Equal(t, 1, prCount, "createPR must fire once after chained verify_approved")
	assert.Equal(t, f.planFile, prPlanFile)

	entry, err := f.store.Get(f.project, f.planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusDone, entry.Status, "task must reach done after chained verify_approved")
	assert.Equal(t, taskstore.ExecutionState{}, entry.ExecutionState, "execution state must be cleared by VerifyApprovedAction")

	remainingPending, err := f.gw.List(f.project, taskstore.SignalPending)
	require.NoError(t, err)
	assert.Empty(t, remainingPending, "no pending signal rows must remain after tickRepo")

	doneRows, err := f.gw.List(f.project, taskstore.SignalDone)
	require.NoError(t, err)
	assert.Len(t, doneRows, 1, "the review_approved gateway row must be marked done")
}

// TestDaemon_TickRepo_HTTPReviewChangesRequestedSpawnsFixer proves a
// review_changes_requested signal from the HTTP handler reaches the fixer
// spawn path when AutoReviewFix is enabled.
func TestDaemon_TickRepo_HTTPReviewChangesRequestedSpawnsFixer(t *testing.T) {
	t.Parallel()

	f := newHTTPTransitionE2EFixture(t, taskstore.StatusReviewing)
	f.postTransition(t, "review_changes")

	proc := loop.NewProcessor(loop.ProcessorConfig{
		Store:         f.store,
		Project:       f.project,
		AutoReviewFix: true,
	})

	var spawnedOpts loop.SpawnOpts
	spawnCount := 0

	d := &Daemon{
		cfg:         &DaemonConfig{},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnFixer: func(_ context.Context, opts loop.SpawnOpts) error {
			spawnCount++
			spawnedOpts = opts
			return nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })

	repoEntry := f.repoEntry(t, proc)
	d.tickRepo(context.Background(), repoEntry)

	assert.Equal(t, 1, spawnCount, "tickRepo must spawn exactly one fixer for HTTP review_changes_requested")
	assert.Equal(t, f.planFile, spawnedOpts.PlanFile)
	assert.Equal(t, f.branch, spawnedOpts.Branch)

	entry, err := f.store.Get(f.project, f.planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status, "task must return to implementing after review_changes")
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseFixing),
		ActiveAgentType: session.AgentTypeFixer,
	}, entry.ExecutionState, "execution state must be fixing/fixer after SpawnFixerAction")

	remainingPending, err := f.gw.List(f.project, taskstore.SignalPending)
	require.NoError(t, err)
	assert.Empty(t, remainingPending, "no pending signal rows must remain after tickRepo")

	doneRows, err := f.gw.List(f.project, taskstore.SignalDone)
	require.NoError(t, err)
	assert.Len(t, doneRows, 1, "the review_changes_requested gateway row must be marked done")
}

// TestDaemon_TickRepo_HTTPVerifyApprovedRequiresSHA proves a pre-applied HTTP
// transition cannot bypass SHA-bound master approval.
func TestDaemon_TickRepo_HTTPVerifyApprovedRequiresSHA(t *testing.T) {
	t.Parallel()

	f := newHTTPTransitionE2EFixture(t, taskstore.StatusVerifying)
	require.NoError(t, f.store.SetExecutionState(f.project, f.planFile, taskstore.ExecutionState{
		ActiveAgentType: session.AgentTypeMaster,
	}))
	f.postTransition(t, "verify_approved")

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: f.store, Project: f.project})

	prCount := 0
	var prPlanFile, prReviewBody string

	d := &Daemon{
		cfg:         &DaemonConfig{},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnMaster: func(context.Context, loop.SpawnOpts) error { return nil },
		createPR: func(_ RepoEntry, planFile, reviewBody string) (prsvc.Result, error) {
			prCount++
			prPlanFile = planFile
			prReviewBody = reviewBody
			return prsvc.Result{Outcome: prsvc.OutcomeCreated, URL: "https://example.test/pr/1"}, nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })

	repoEntry, head := f.gitRepoEntry(t, proc)
	proc = loop.NewProcessor(loop.ProcessorConfig{
		Store: f.store, Project: f.project, AutoReadinessReview: true,
		HeadSHA:      func(string) (string, error) { return head, nil },
		MergeBaseSHA: func(string) (string, error) { return gitpkg.DefaultBranchHeadSHA(repoEntry.Path) },
	})
	repoEntry.Processor = proc
	d.tickRepo(context.Background(), repoEntry)

	assert.Zero(t, prCount, "unbound HTTP verify_approved must not create a PR")
	assert.Empty(t, prPlanFile)
	assert.Empty(t, prReviewBody)

	entry, err := f.store.Get(f.project, f.planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusVerifying, entry.Status, "unbound pre-applied approval must reopen verification")
	assert.Empty(t, entry.VerifiedSHA)
	assert.Empty(t, entry.VerifiedBy)
	assert.Equal(t, session.AgentTypeMaster, entry.ExecutionState.ActiveAgentType, "replacement verification must remain active")

	remainingPending, err := f.gw.List(f.project, taskstore.SignalPending)
	require.NoError(t, err)
	assert.Empty(t, remainingPending, "no pending signal rows must remain after tickRepo")

	doneRows, err := f.gw.List(f.project, taskstore.SignalDone)
	require.NoError(t, err)
	assert.Len(t, doneRows, 1, "the verify_approved gateway row must be marked done")
}

// TestDaemon_TickRepo_HTTPVerifyFailedSpawnsFixer proves a verify_failed signal
// from the HTTP handler reaches the fixer spawn path when AutoReviewFix is
// enabled, mirroring the existing master-agent driven path.
func TestDaemon_TickRepo_HTTPVerifyFailedSpawnsFixer(t *testing.T) {
	t.Parallel()

	f := newHTTPTransitionE2EFixture(t, taskstore.StatusVerifying)
	f.postTransition(t, "verify_failed")

	proc := loop.NewProcessor(loop.ProcessorConfig{
		Store:         f.store,
		Project:       f.project,
		AutoReviewFix: true,
	})

	var spawnedOpts loop.SpawnOpts
	spawnCount := 0

	d := &Daemon{
		cfg:         &DaemonConfig{},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnFixer: func(_ context.Context, opts loop.SpawnOpts) error {
			spawnCount++
			spawnedOpts = opts
			return nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })

	repoEntry := f.repoEntry(t, proc)
	d.tickRepo(context.Background(), repoEntry)

	assert.Equal(t, 1, spawnCount, "tickRepo must spawn exactly one fixer for HTTP verify_failed")
	assert.Equal(t, f.planFile, spawnedOpts.PlanFile)
	assert.Equal(t, f.branch, spawnedOpts.Branch)

	entry, err := f.store.Get(f.project, f.planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status, "task must return to implementing after verify_failed")
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseFixing),
		ActiveAgentType: session.AgentTypeFixer,
	}, entry.ExecutionState, "execution state must be fixing/fixer after SpawnFixerAction")

	remainingPending, err := f.gw.List(f.project, taskstore.SignalPending)
	require.NoError(t, err)
	assert.Empty(t, remainingPending, "no pending signal rows must remain after tickRepo")

	doneRows, err := f.gw.List(f.project, taskstore.SignalDone)
	require.NoError(t, err)
	assert.Len(t, doneRows, 1, "the verify_failed gateway row must be marked done")
}

// TestDaemon_TickRepo_GatewayVerifyFailedAtReadinessCapStaysFailed covers the
// master-agent path: an MCP-created gateway row is not pre-applied. Reaching a
// configured readiness cycle cap must never rewrite that failure as approval.
func TestDaemon_TickRepo_GatewayVerifyFailedAtReadinessCapStaysFailed(t *testing.T) {
	t.Parallel()

	f := newHTTPTransitionE2EFixture(t, taskstore.StatusVerifying)
	require.NoError(t, f.store.IncrementReviewCycle(f.project, f.planFile))
	require.NoError(t, f.gw.Create(f.project, taskstore.SignalEntry{
		PlanFile:   f.planFile,
		SignalType: "verify_failed",
		Payload:    `{"body":"schema contract mismatch"}`,
	}))

	proc := loop.NewProcessor(loop.ProcessorConfig{
		Store:                    f.store,
		Project:                  f.project,
		AutoReviewFix:            true,
		AutoReadinessReview:      true,
		ReadinessMaxVerifyCycles: 2,
	})

	spawnCount := 0
	d := &Daemon{
		cfg:         &DaemonConfig{},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnFixer: func(_ context.Context, opts loop.SpawnOpts) error {
			spawnCount++
			assert.Equal(t, "schema contract mismatch", opts.Feedback)
			return nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })

	d.tickRepo(context.Background(), f.repoEntry(t, proc))

	entry, err := f.store.Get(f.project, f.planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status)
	assert.Empty(t, entry.VerifiedSHA)
	assert.Equal(t, 1, spawnCount)

	doneRows, err := f.gw.List(f.project, taskstore.SignalDone)
	require.NoError(t, err)
	require.Len(t, doneRows, 1)
	assert.Equal(t, "verify_failed", doneRows[0].SignalType)
}
