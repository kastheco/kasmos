package daemon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	prsvc "github.com/kastheco/kasmos/orchestration/pr"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reviewFixSpawnRecord struct {
	agentType   string
	title       string
	reviewCycle int
	branch      string
	feedback    string
	prompt      string
}

func TestDaemon_TickRepoGateway_ReviewFixLoop_HappyPath(t *testing.T) {
	const (
		project  = "proj"
		planFile = "feature.md"
		branch   = "plan/feature"
	)

	feedback := "- [app.go:42] address reviewer feedback"
	wantTitles := []string{"feature.md-review-1", "feature.md-fix-1", "feature.md-review-2"}

	store, gw, entry, daemon, events, spawned := newReviewFixHarness(t, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		Branch:   branch,
		PRURL:    "https://example.com/pr/123",
	})

	require.NoError(t, taskfsm.EmitGatewaySignal(gw, project, string(taskfsm.ImplementFinished), planFile, ""))
	daemon.tickRepo(context.Background(), entry)

	require.Len(t, *spawned, 1)
	assert.Equal(t, []string{session.AgentTypeReviewer}, spawnedAgentTypes(*spawned))
	assert.Equal(t, reviewFixSpawnRecord{
		agentType:   session.AgentTypeReviewer,
		title:       wantTitles[0],
		reviewCycle: 1,
		branch:      branch,
		prompt:      (*spawned)[0].prompt,
	}, (*spawned)[0])
	assertEventKinds(t, drainDaemonEvents(events), api.EventKindAgentSpawned)

	updated, err := store.Get(project, planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReviewing, updated.Status)
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseReviewing),
		ActiveAgentType: session.AgentTypeReviewer,
	}, updated.ExecutionState)
	assert.Zero(t, updated.ReviewCycle)

	require.NoError(t, taskfsm.EmitGatewaySignal(gw, project, string(taskfsm.ReviewChangesRequested), planFile, feedback))
	daemon.tickRepo(context.Background(), entry)

	require.Len(t, *spawned, 2)
	assert.Equal(t, []string{session.AgentTypeReviewer, session.AgentTypeFixer}, spawnedAgentTypes(*spawned))
	assert.Equal(t, session.AgentTypeFixer, (*spawned)[1].agentType)
	assert.Equal(t, wantTitles[1], (*spawned)[1].title)
	assert.Equal(t, 1, (*spawned)[1].reviewCycle)
	assert.Equal(t, branch, (*spawned)[1].branch)
	assert.Equal(t, feedback, (*spawned)[1].feedback)
	assert.Contains(t, (*spawned)[1].prompt, feedback, "fixer prompt should include prior reviewer feedback")
	assertEventKinds(t, drainDaemonEvents(events), api.EventKindAgentSpawned)

	updated, err = store.Get(project, planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, updated.Status)
	assert.Equal(t, 1, updated.ReviewCycle, "review changes should increment the review cycle")
	assert.Equal(t, feedback, updated.LatestReviewFeedback)
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseFixing),
		ActiveAgentType: session.AgentTypeFixer,
	}, updated.ExecutionState)

	require.NoError(t, taskfsm.EmitGatewaySignal(gw, project, string(taskfsm.ImplementFinished), planFile, ""))
	daemon.tickRepo(context.Background(), entry)

	require.Len(t, *spawned, 3)
	assert.Equal(t, []string{session.AgentTypeReviewer, session.AgentTypeFixer, session.AgentTypeReviewer}, spawnedAgentTypes(*spawned))
	assert.Equal(t, session.AgentTypeReviewer, (*spawned)[2].agentType)
	assert.Equal(t, wantTitles[2], (*spawned)[2].title)
	assert.Equal(t, 2, (*spawned)[2].reviewCycle)
	assert.Equal(t, branch, (*spawned)[2].branch)
	assert.Contains(t, (*spawned)[2].prompt, feedback, "second reviewer should receive the prior feedback")
	assertEventKinds(t, drainDaemonEvents(events), api.EventKindAgentSpawned)

	updated, err = store.Get(project, planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReviewing, updated.Status)
	assert.Equal(t, 1, updated.ReviewCycle)
	assert.Equal(t, feedback, updated.LatestReviewFeedback)

	require.NoError(t, taskfsm.EmitGatewaySignal(gw, project, string(taskfsm.ReviewApproved), planFile, "lgtm"))
	daemon.tickRepo(context.Background(), entry)

	updated, err = store.Get(project, planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusDone, updated.Status)
	assert.Equal(t, taskstore.ExecutionState{}, updated.ExecutionState)
	assert.Equal(t, 1, updated.ReviewCycle)
	assert.Equal(t, feedback, updated.LatestReviewFeedback)
	assertEventKinds(t, drainDaemonEvents(events), "review_approved", "signal_processed", "pr_create_skipped")
	assert.Equal(t, string(prsvc.OutcomeSkipped), updated.PRCreateState)
	assert.Equal(t, "auto pr disabled by config", updated.PRCreateError)

	doneSignals, err := gw.List(project, taskstore.SignalDone)
	require.NoError(t, err)
	assert.Len(t, doneSignals, 4)

	failedSignals, err := gw.List(project, taskstore.SignalFailed)
	require.NoError(t, err)
	assert.Empty(t, failedSignals)
}

func TestDaemon_TickRepoGateway_ReviewFixLoop_LimitPath(t *testing.T) {
	const (
		project  = "proj"
		planFile = "feature.md"
		branch   = "plan/feature"
	)

	feedback := "review cycle limit reached"

	store, gw, entry, daemon, events, spawned := newReviewFixHarness(t, taskstore.TaskEntry{
		Filename:    planFile,
		Status:      taskstore.StatusReviewing,
		Branch:      branch,
		ReviewCycle: 2,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		},
	})

	require.NoError(t, taskfsm.EmitGatewaySignal(gw, project, string(taskfsm.ReviewChangesRequested), planFile, feedback))
	daemon.tickRepo(context.Background(), entry)

	assert.Empty(t, *spawned, "cycle-limit path must not spawn another fixer")
	assertEventKinds(t, drainDaemonEvents(events), "review_cycle_limit")

	updated, err := store.Get(project, planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, updated.Status)
	assert.Equal(t, 2, updated.ReviewCycle, "cycle-limit path must not increment review cycle")
	assert.Equal(t, feedback, updated.LatestReviewFeedback)

	doneSignals, err := gw.List(project, taskstore.SignalDone)
	require.NoError(t, err)
	assert.Len(t, doneSignals, 1)

	failedSignals, err := gw.List(project, taskstore.SignalFailed)
	require.NoError(t, err)
	assert.Empty(t, failedSignals)
}

func newReviewFixHarness(t *testing.T, task taskstore.TaskEntry) (taskstore.Store, taskstore.SignalGateway, RepoEntry, *Daemon, <-chan api.Event, *[]reviewFixSpawnRecord) {
	t.Helper()

	const project = "proj"

	repoDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".kasmos", "signals"), 0o755))

	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, task))

	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	broadcaster := api.NewEventBroadcaster()
	events := broadcaster.Subscribe()
	t.Cleanup(func() {
		broadcaster.Unsubscribe(events)
		broadcaster.Close()
	})

	spawned := make([]reviewFixSpawnRecord, 0, 4)
	d := &Daemon{
		cfg:         &DaemonConfig{AutoReviewFix: true, MaxReviewFixCycles: 2, PollInterval: time.Second},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: broadcaster,
		spawnReviewer: func(_ context.Context, opts loop.SpawnOpts) error {
			spawned = append(spawned, reviewFixSpawnRecord{
				agentType:   session.AgentTypeReviewer,
				title:       orchestration.BuildLifecycleAgentTitle(opts.PlanFile, session.AgentTypeReviewer, opts.ReviewCycle),
				reviewCycle: opts.ReviewCycle,
				branch:      opts.Branch,
				prompt:      opts.Prompt,
			})
			return nil
		},
		spawnFixer: func(_ context.Context, opts loop.SpawnOpts) error {
			spawned = append(spawned, reviewFixSpawnRecord{
				agentType:   session.AgentTypeFixer,
				title:       orchestration.BuildLifecycleAgentTitle(opts.PlanFile, session.AgentTypeFixer, opts.ReviewCycle),
				reviewCycle: opts.ReviewCycle,
				branch:      opts.Branch,
				feedback:    opts.Feedback,
				prompt:      opts.Prompt,
			})
			return nil
		},
	}

	entry := RepoEntry{
		Path:          repoDir,
		Project:       project,
		Store:         store,
		SignalsDir:    filepath.Join(repoDir, ".kasmos", "signals"),
		Processor:     loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project, AutoReviewFix: true, MaxReviewFixCycles: 2}),
		SignalGateway: gw,
	}

	return store, gw, entry, d, events, &spawned
}

func spawnedAgentTypes(records []reviewFixSpawnRecord) []string {
	types := make([]string, 0, len(records))
	for _, record := range records {
		types = append(types, record.agentType)
	}
	return types
}

func drainDaemonEvents(events <-chan api.Event) []api.Event {
	out := make([]api.Event, 0)
	for {
		select {
		case event := <-events:
			out = append(out, event)
		default:
			return out
		}
	}
}

func assertEventKinds(t *testing.T, events []api.Event, wantKinds ...string) {
	t.Helper()
	gotKinds := make([]string, 0, len(events))
	for _, event := range events {
		gotKinds = append(gotKinds, event.Kind)
	}
	assert.Equal(t, wantKinds, gotKinds)
}
