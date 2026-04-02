package taskstore_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiStoreRoutesByProject(t *testing.T) {
	repoA := newTestRepoRoot(t, filepath.Join(t.TempDir(), "alpha-repo"))
	repoB := newTestRepoRoot(t, filepath.Join(t.TempDir(), "beta-repo"))

	projectA := filepath.Base(repoA)
	projectB := filepath.Base(repoB)

	seedRepoTask(t, repoA, taskstore.TaskEntry{Filename: "alpha-task", Status: taskstore.StatusReady, Description: "alpha description"}, "alpha content")
	seedRepoTask(t, repoB, taskstore.TaskEntry{Filename: "beta-task", Status: taskstore.StatusDone, Description: "beta description"}, "beta content")

	multi, err := taskstore.NewMultiStore([]taskstore.RepoConfig{{Path: repoA}, {Path: repoB}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = multi.Close() })

	alphaTask, err := multi.Get(projectA, "alpha-task")
	require.NoError(t, err)
	assert.Equal(t, "alpha description", alphaTask.Description)
	assert.Equal(t, taskstore.StatusReady, alphaTask.Status)

	betaTask, err := multi.Get(projectB, "beta-task")
	require.NoError(t, err)
	assert.Equal(t, "beta description", betaTask.Description)
	assert.Equal(t, taskstore.StatusDone, betaTask.Status)

	_, err = multi.Get(projectA, "beta-task")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	alphaList, err := multi.List(projectA)
	require.NoError(t, err)
	require.Len(t, alphaList, 1)
	assert.Equal(t, "alpha-task", alphaList[0].Filename)

	betaList, err := multi.List(projectB)
	require.NoError(t, err)
	require.Len(t, betaList, 1)
	assert.Equal(t, "beta-task", betaList[0].Filename)

	alphaContent, err := multi.GetContent(projectA, "alpha-task")
	require.NoError(t, err)
	assert.Equal(t, "alpha content", alphaContent)

	betaContent, err := multi.GetContent(projectB, "beta-task")
	require.NoError(t, err)
	assert.Equal(t, "beta content", betaContent)

	_, err = multi.GetContent(projectA, "beta-task")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMultiStoreUnknownProject(t *testing.T) {
	repo := newTestRepoRoot(t, filepath.Join(t.TempDir(), "known-repo"))
	multi, err := taskstore.NewMultiStore([]taskstore.RepoConfig{{Path: repo}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = multi.Close() })

	unknownProject := "missing-repo"
	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "get",
			call: func() error {
				_, err := multi.Get(unknownProject, "task-1")
				return err
			},
		},
		{
			name: "list",
			call: func() error {
				_, err := multi.List(unknownProject)
				return err
			},
		},
		{
			name: "create topic",
			call: func() error {
				return multi.CreateTopic(unknownProject, taskstore.TopicEntry{Name: "topic-a", CreatedAt: time.Now().UTC()})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			require.EqualError(t, err, fmt.Sprintf("project not found: %s", unknownProject))
		})
	}
}

func TestMultiStoreConstructorsRejectBasenameCollision(t *testing.T) {
	repoName := "shared-repo"
	repoA := newTestRepoRoot(t, filepath.Join(t.TempDir(), repoName))
	repoB := newTestRepoRoot(t, filepath.Join(t.TempDir(), repoName))

	expectedErr := fmt.Sprintf("repo with basename %q already registered (path: %s); rename one of the directories or use distinct names", repoName, repoA)

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "multi store",
			call: func() error {
				multi, err := taskstore.NewMultiStore([]taskstore.RepoConfig{{Path: repoA}, {Path: repoB}})
				if multi != nil {
					_ = multi.Close()
				}
				return err
			},
		},
		{
			name: "multi signal gateway",
			call: func() error {
				gw, err := taskstore.NewMultiSignalGateway([]taskstore.RepoConfig{{Path: repoA}, {Path: repoB}})
				if gw != nil {
					_ = gw.Close()
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			require.EqualError(t, err, expectedErr)
		})
	}
}

func TestMultiSignalGatewaySyntheticIDsRouteMarkProcessed(t *testing.T) {
	repoA := newTestRepoRoot(t, filepath.Join(t.TempDir(), "signals-alpha"))
	repoB := newTestRepoRoot(t, filepath.Join(t.TempDir(), "signals-beta"))

	projectA := filepath.Base(repoA)
	projectB := filepath.Base(repoB)

	gw, err := taskstore.NewMultiSignalGateway([]taskstore.RepoConfig{{Path: repoA}, {Path: repoB}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	require.NoError(t, gw.Create(projectA, taskstore.SignalEntry{PlanFile: "plan-a", SignalType: "planner_finished"}))
	require.NoError(t, gw.Create(projectB, taskstore.SignalEntry{PlanFile: "plan-b", SignalType: "implement_finished"}))

	claimedA, err := gw.Claim(projectA, "worker-a")
	require.NoError(t, err)
	require.NotNil(t, claimedA)

	claimedB, err := gw.Claim(projectB, "worker-b")
	require.NoError(t, err)
	require.NotNil(t, claimedB)

	assert.NotEqual(t, claimedA.ID, claimedB.ID)
	assert.Equal(t, projectA, claimedA.Project)
	assert.Equal(t, projectB, claimedB.Project)

	require.NoError(t, gw.MarkProcessed(claimedB.ID, taskstore.SignalDone, "repo-b processed"))
	require.NoError(t, gw.MarkProcessed(claimedA.ID, taskstore.SignalDone, "repo-a processed"))

	directA := openDirectSignalGateway(t, repoA)
	directB := openDirectSignalGateway(t, repoB)

	doneA, err := directA.List(projectA, taskstore.SignalDone)
	require.NoError(t, err)
	require.Len(t, doneA, 1)
	assert.EqualValues(t, 1, doneA[0].ID)
	assert.Equal(t, "repo-a processed", doneA[0].Result)

	doneB, err := directB.List(projectB, taskstore.SignalDone)
	require.NoError(t, err)
	require.Len(t, doneB, 1)
	assert.EqualValues(t, 1, doneB[0].ID)
	assert.Equal(t, "repo-b processed", doneB[0].Result)
	assert.NotEqual(t, claimedB.ID, doneB[0].ID)
}

func newTestRepoRoot(t *testing.T, path string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, 0o755))
	absPath, err := filepath.Abs(path)
	require.NoError(t, err)
	return filepath.Clean(absPath)
}

func seedRepoTask(t *testing.T, repoPath string, entry taskstore.TaskEntry, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, ".kasmos"), 0o755))
	store, err := taskstore.NewSQLiteStore(filepath.Join(repoPath, ".kasmos", "taskstore.db"))
	require.NoError(t, err)
	require.NoError(t, store.Create(filepath.Base(repoPath), entry))
	if content != "" {
		require.NoError(t, store.SetContent(filepath.Base(repoPath), entry.Filename, content))
	}
	require.NoError(t, store.Close())
}

func openDirectSignalGateway(t *testing.T, repoPath string) *taskstore.SQLiteSignalGateway {
	t.Helper()
	gw, err := taskstore.NewSQLiteSignalGateway(filepath.Join(repoPath, ".kasmos", "taskstore.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })
	return gw
}
