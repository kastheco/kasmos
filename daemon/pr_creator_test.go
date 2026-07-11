package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPRCreatorSweepBoundedRetry(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	store, err := taskstore.NewSQLiteStore(filepath.Join(t.TempDir(), "tasks.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	tasks := []taskstore.TaskEntry{
		{Filename: "due", Status: taskstore.StatusDone, PRCreateState: "failed", PRCreateAttempts: 2, PRCreateAttemptedAt: now.Add(-5 * time.Minute)},
		{Filename: "blocked", Status: taskstore.StatusDone, PRCreateState: "blocked", PRCreateAttempts: 1},
		{Filename: "skipped", Status: taskstore.StatusDone, PRCreateState: "skipped", PRCreateAttempts: 1},
		{Filename: "has-url", Status: taskstore.StatusDone, PRURL: "https://example.test/1", PRCreateState: "failed", PRCreateAttempts: 1},
		{Filename: "exhausted", Status: taskstore.StatusDone, PRCreateState: "failed", PRCreateAttempts: 5},
		{Filename: "waiting", Status: taskstore.StatusDone, PRCreateState: "failed", PRCreateAttempts: 2, PRCreateAttemptedAt: now.Add(-3 * time.Minute)},
	}
	for _, entry := range tasks {
		require.NoError(t, store.Create("proj", entry))
	}

	repos := NewRepoManager()
	repos.repos = []RepoEntry{{Path: t.TempDir(), Project: "proj", Store: store, AutoCreatePR: true}}
	var dispatched []string
	creator := NewPRCreator(defaultDaemonConfig().PRCreator, repos, slog.New(slog.NewTextHandler(io.Discard, nil)), func(_ context.Context, _ RepoEntry, action loop.Action) error {
		dispatched = append(dispatched, action.(loop.CreatePRAction).PlanFile)
		return nil
	})
	creator.now = func() time.Time { return now }
	creator.checkGH = func(context.Context) error { return nil }

	creator.sweepOnce(context.Background())
	assert.Equal(t, []string{"due"}, dispatched)

	creator.now = func() time.Time { return now.Add(2 * time.Minute) }
	creator.sweepOnce(context.Background())
	assert.Equal(t, []string{"due", "due", "waiting"}, dispatched)
}

func TestPRCreatorSweepGuards(t *testing.T) {
	store, err := taskstore.NewSQLiteStore(filepath.Join(t.TempDir(), "tasks.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	entry := taskstore.TaskEntry{Filename: "retry", Status: taskstore.StatusDone, PRCreateState: "failed", PRCreateAttempts: 1, PRCreateAttemptedAt: time.Now().Add(-time.Hour)}
	require.NoError(t, store.Create("proj", entry))

	for _, tc := range []struct {
		name   string
		autoPR bool
		ghErr  error
	}{
		{name: "auto create disabled", autoPR: false},
		{name: "gh unavailable", autoPR: true, ghErr: errors.New("gh executable file not found")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repos := NewRepoManager()
			repos.repos = []RepoEntry{{Project: "proj", Store: store, AutoCreatePR: tc.autoPR}}
			dispatches := 0
			creator := NewPRCreator(defaultDaemonConfig().PRCreator, repos, slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context, RepoEntry, loop.Action) error {
				dispatches++
				return nil
			})
			creator.checkGH = func(context.Context) error { return tc.ghErr }
			creator.sweepOnce(context.Background())
			assert.Zero(t, dispatches)
			got, getErr := store.Get("proj", "retry")
			require.NoError(t, getErr)
			assert.Equal(t, 1, got.PRCreateAttempts)
		})
	}
}
