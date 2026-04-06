package daemon

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRepoManager returns a RepoManager that uses a temp-file SQLite store
// opened via the shared-DB path instead of the real global path at
// ~/.config/kasmos/taskstore.db. This keeps tests hermetic.
func newTestRepoManager(t *testing.T) *RepoManager {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	return &RepoManager{
		openDB: func() (*sql.DB, error) {
			return taskstore.OpenSharedDB(dbPath)
		},
	}
}

func TestRepoManager_AddAndList(t *testing.T) {
	rm := newTestRepoManager(t)
	require.NoError(t, rm.Add("/home/user/project-a"))
	require.NoError(t, rm.Add("/home/user/project-b"))

	repos := rm.List()
	assert.Len(t, repos, 2)
	assert.Equal(t, "/home/user/project-a", repos[0].Path)
}

func TestRepoManager_AddDuplicate(t *testing.T) {
	rm := newTestRepoManager(t)
	require.NoError(t, rm.Add("/home/user/project-a"))
	err := rm.Add("/home/user/project-a")
	assert.Error(t, err)
}

func TestRepoManager_Remove(t *testing.T) {
	rm := newTestRepoManager(t)
	require.NoError(t, rm.Add("/home/user/project-a"))
	require.NoError(t, rm.Remove("/home/user/project-a"))
	assert.Len(t, rm.List(), 0)
}

func TestRepoManager_ProjectName(t *testing.T) {
	rm := newTestRepoManager(t)
	require.NoError(t, rm.Add("/home/user/my-project"))
	repos := rm.List()
	assert.Equal(t, "my-project", repos[0].Project)
}

func TestRepoManager_AddDuplicateBasename(t *testing.T) {
	rm := newTestRepoManager(t)
	require.NoError(t, rm.Add("/org-a/my-project"))
	// Different absolute path but same basename — must be rejected.
	err := rm.Add("/org-b/my-project")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "my-project")
}

// TestRepoManager_SharedGlobalStore verifies that multiple repos registered with
// the same RepoManager share a single global store instance.
func TestRepoManager_SharedGlobalStore(t *testing.T) {
	rm := newTestRepoManager(t)
	require.NoError(t, rm.Add("/home/user/project-a"))
	require.NoError(t, rm.Add("/home/user/project-b"))

	repos := rm.List()
	require.Len(t, repos, 2)

	// Both entries must point to the same underlying store instance.
	assert.Same(t, repos[0].Store, repos[1].Store)
	assert.Same(t, repos[0].SignalGateway, repos[1].SignalGateway)
}

// TestRepoManager_SharedGlobalDB verifies that after Add() the manager holds a
// non-nil globalDB and that both repos' store and gateway are derived from it.
// It also checks that a second Add() reuses the same globalDB (no second open).
func TestRepoManager_SharedGlobalDB(t *testing.T) {
	rm := newTestRepoManager(t)

	// Before any Add, globalDB must be nil.
	assert.Nil(t, rm.globalDB)

	require.NoError(t, rm.Add("/home/user/project-a"))

	// After the first Add, globalDB must be open.
	require.NotNil(t, rm.globalDB)
	firstDB := rm.globalDB

	require.NoError(t, rm.Add("/home/user/project-b"))

	// Second Add must reuse the same *sql.DB — pointer equality.
	assert.Same(t, firstDB, rm.globalDB, "globalDB must not be re-opened on subsequent Add calls")

	repos := rm.List()
	require.Len(t, repos, 2)
	// Both repos see the same store and gateway, both backed by globalDB.
	assert.Same(t, repos[0].Store, repos[1].Store)
	assert.Same(t, repos[0].SignalGateway, repos[1].SignalGateway)
}

// TestRepoManager_CloseReleasesGlobalStore verifies that Close() closes the
// shared global store, gateway, and underlying DB, nulling all three fields.
func TestRepoManager_CloseReleasesGlobalStore(t *testing.T) {
	rm := newTestRepoManager(t)
	require.NoError(t, rm.Add("/home/user/project-a"))
	require.NotNil(t, rm.globalStore)
	require.NotNil(t, rm.globalGateway)
	require.NotNil(t, rm.globalDB)

	rm.Close()

	assert.Nil(t, rm.globalStore)
	assert.Nil(t, rm.globalGateway)
	assert.Nil(t, rm.globalDB)
}

// TestRepoManager_RemoveLastClosesGlobalStore verifies that removing the last
// registered repo closes the shared global store, gateway, and underlying DB.
func TestRepoManager_RemoveLastClosesGlobalStore(t *testing.T) {
	rm := newTestRepoManager(t)
	require.NoError(t, rm.Add("/home/user/project-a"))
	require.NotNil(t, rm.globalStore)
	require.NotNil(t, rm.globalDB)

	require.NoError(t, rm.Remove("/home/user/project-a"))

	assert.Nil(t, rm.globalStore)
	assert.Nil(t, rm.globalGateway)
	assert.Nil(t, rm.globalDB)
}

// TestRepoManager_RemoveNonLastKeepsGlobalStore verifies that removing a repo
// when others remain does not close the shared global store.
func TestRepoManager_RemoveNonLastKeepsGlobalStore(t *testing.T) {
	rm := newTestRepoManager(t)
	require.NoError(t, rm.Add("/home/user/project-a"))
	require.NoError(t, rm.Add("/home/user/project-b"))
	storeRef := rm.globalStore
	dbRef := rm.globalDB

	require.NoError(t, rm.Remove("/home/user/project-a"))

	// Store and DB must still be open and reachable via the remaining entry.
	assert.NotNil(t, rm.globalStore)
	assert.NotNil(t, rm.globalDB)
	assert.Same(t, storeRef, rm.globalStore)
	assert.Same(t, dbRef, rm.globalDB)
	repos := rm.List()
	require.Len(t, repos, 1)
	assert.Same(t, storeRef, repos[0].Store)
}

// TestRepoManager_MigratesRepoLocalTasks verifies that Add() copies tasks from
// a legacy per-repo taskstore.db into the global store.
func TestRepoManager_MigratesRepoLocalTasks(t *testing.T) {
	repoDir := t.TempDir()
	kasmosDir := filepath.Join(repoDir, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))

	// Seed a task into a legacy per-repo DB at <repo>/.kasmos/taskstore.db.
	// Use a bare slug (no .md) — NewSQLiteStore runs migrateStripMdSuffix on
	// open, which normalises "foo.md" → "foo", so bare slugs are stable.
	localDBPath := filepath.Join(kasmosDir, "taskstore.db")
	localStore, err := taskstore.NewSQLiteStore(localDBPath)
	require.NoError(t, err)
	project := filepath.Base(repoDir)
	require.NoError(t, localStore.Create(project, taskstore.TaskEntry{
		Filename: "my-task",
		Status:   taskstore.StatusReady,
	}))
	require.NoError(t, localStore.Close())

	rm := newTestRepoManager(t)
	require.NoError(t, rm.Add(repoDir))

	// The task should now exist in the global store.
	require.NotNil(t, rm.globalStore)
	entry, err := rm.globalStore.Get(project, "my-task")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, entry.Status)
}
