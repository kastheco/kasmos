package daemon

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration/loop"
	prsvc "github.com/kastheco/kasmos/orchestration/pr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePRForApprovedTask_NilStore(t *testing.T) {
	t.Parallel()

	d := &Daemon{logger: slog.Default(), broadcaster: api.NewEventBroadcaster()}
	defer d.broadcaster.Close()

	_, err := d.ensurePRForApprovedTask(RepoEntry{
		Path:    t.TempDir(),
		Project: "test-project",
		Store:   nil,
	}, "plan.md", "LGTM")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "task store unavailable")
}

func TestCreatePRForApprovedTask_NoBranch(t *testing.T) {
	t.Parallel()

	const (
		project  = "test-project"
		planFile = "plan.md"
	)
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReviewing,
		Branch:   "",
	}))

	d := &Daemon{logger: slog.Default(), broadcaster: api.NewEventBroadcaster()}
	defer d.broadcaster.Close()

	res, err := d.ensurePRForApprovedTask(RepoEntry{
		Path:         t.TempDir(),
		Project:      project,
		Store:        store,
		AutoCreatePR: true,
	}, planFile, "LGTM")

	require.NoError(t, err)
	assert.Equal(t, prsvc.OutcomeBlocked, res.Outcome)
}

func TestDaemon_CreatePRAction_NoBranch_EmitsEvent(t *testing.T) {
	t.Parallel()

	b := api.NewEventBroadcaster()
	defer b.Close()
	sub := b.Subscribe()

	const (
		project  = "test-project"
		planFile = "plan.md"
	)
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReviewing,
		Branch:   "",
	}))

	d := &Daemon{
		logger:      slog.Default(),
		broadcaster: b,
	}
	e := RepoEntry{
		Path:         t.TempDir(),
		Project:      project,
		Store:        store,
		AutoCreatePR: true,
	}

	err := d.executeAction(context.Background(), e, loop.CreatePRAction{
		PlanFile:   planFile,
		ReviewBody: "LGTM",
	})
	require.NoError(t, err)

	select {
	case ev := <-sub:
		assert.Equal(t, "pr_create_failed", ev.Kind)
		assert.Contains(t, ev.Detail, "not a git repository")
		assert.Equal(t, planFile, ev.PlanFile)
		assert.Equal(t, e.Path, ev.Repo)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for pr_create_failed event")
	}
}

func TestDaemon_CreatePRAction_UnverifiableRecordedURLPersistsBlockedOutcome(t *testing.T) {
	t.Parallel()
	b := api.NewEventBroadcaster()
	defer b.Close()
	sub := b.Subscribe()
	store := taskstore.NewTestStore(t)
	const project, planFile = "test-project", "plan.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile, Status: taskstore.StatusDone,
		PRURL: "https://example.test/pr/7",
	}))
	d := &Daemon{logger: slog.Default(), broadcaster: b}
	e := RepoEntry{Path: t.TempDir(), Project: project, Store: store, AutoCreatePR: true}

	require.NoError(t, d.executeAction(context.Background(), e, loop.CreatePRAction{PlanFile: planFile}))
	select {
	case ev := <-sub:
		assert.Equal(t, "pr_create_failed", ev.Kind)
		assert.Contains(t, ev.Detail, "branch")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for pr_create_skipped event")
	}
	entry, err := store.Get(project, planFile)
	require.NoError(t, err)
	assert.Equal(t, string(prsvc.OutcomeBlocked), entry.PRCreateState)
	assert.Contains(t, entry.PRCreateError, "branch")
}
