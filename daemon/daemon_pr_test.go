package daemon

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePRForApprovedTask_NilStore(t *testing.T) {
	t.Parallel()

	d := &Daemon{logger: slog.Default(), broadcaster: api.NewEventBroadcaster()}
	defer d.broadcaster.Close()

	err := d.createPRForApprovedTask(RepoEntry{
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

	err := d.createPRForApprovedTask(RepoEntry{
		Path:    t.TempDir(),
		Project: project,
		Store:   store,
	}, planFile, "LGTM")

	require.NoError(t, err)
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
		Path:    t.TempDir(),
		Project: project,
		Store:   store,
	}

	err := d.executeAction(context.Background(), e, loop.CreatePRAction{
		PlanFile:   planFile,
		ReviewBody: "LGTM",
	})
	require.NoError(t, err)

	select {
	case ev := <-sub:
		assert.Equal(t, "signal_processed", ev.Kind)
		assert.Equal(t, "create PR for "+planFile, ev.Message)
		assert.Equal(t, planFile, ev.PlanFile)
		assert.Equal(t, e.Path, ev.Repo)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for signal_processed event")
	}
}
