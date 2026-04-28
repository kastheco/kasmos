package daemon

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration/loop"
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

func TestRepoManager_ParallelPlannerArchitectConfig(t *testing.T) {
	t.Run("defaults true without project config", func(t *testing.T) {
		repoDir := t.TempDir()
		rm := newTestRepoManager(t)

		require.NoError(t, rm.Add(repoDir))
		repos := rm.List()
		require.Len(t, repos, 1)
		assert.True(t, repos[0].ParallelPlannerArchitect)
	})

	t.Run("orchestration section without key resolves true", func(t *testing.T) {
		// [orchestration] present but key absent — nil pointer, default true applies.
		repoDir := t.TempDir()
		kasmosDir := filepath.Join(repoDir, ".kasmos")
		require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte("[orchestration]\n"), 0o644))

		rm := newTestRepoManager(t)
		require.NoError(t, rm.Add(repoDir))
		repos := rm.List()
		require.Len(t, repos, 1)
		assert.True(t, repos[0].ParallelPlannerArchitect)
	})

	t.Run("explicit true in project config resolves true", func(t *testing.T) {
		repoDir := t.TempDir()
		kasmosDir := filepath.Join(repoDir, ".kasmos")
		require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
		content := `
[orchestration]
parallel_planner_architect = true
`
		require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(content), 0o644))

		rm := newTestRepoManager(t)
		require.NoError(t, rm.Add(repoDir))
		repos := rm.List()
		require.Len(t, repos, 1)
		assert.True(t, repos[0].ParallelPlannerArchitect)
	})

	t.Run("legacy explicit false in project config is ignored", func(t *testing.T) {
		repoDir := t.TempDir()
		kasmosDir := filepath.Join(repoDir, ".kasmos")
		require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
		content := `
[orchestration]
parallel_planner_architect = false
`
		require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(content), 0o644))

		rm := newTestRepoManager(t)
		require.NoError(t, rm.Add(repoDir))
		repos := rm.List()
		require.Len(t, repos, 1)
		assert.True(t, repos[0].ParallelPlannerArchitect)
	})
}

// TestSDKTranscriptRetention_ResolveRepoConfig verifies that resolveRepoConfig
// reads [sdk] transcript limits from the project TOML and returns defaults when
// the section is absent.
func TestSDKTranscriptRetention_ResolveRepoConfig(t *testing.T) {
	defaults := config.DefaultConfig().SDK

	t.Run("no project config — default SDK limits apply", func(t *testing.T) {
		rm := newTestRepoManager(t)
		_, _, _, _, _, sdk, _, err := rm.resolveRepoConfig(t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, defaults.TranscriptMaxBytes, sdk.TranscriptMaxBytes)
		assert.Equal(t, defaults.TranscriptMaxTurns, sdk.TranscriptMaxTurns)
	})

	t.Run("explicit values in project TOML override defaults", func(t *testing.T) {
		repoDir := t.TempDir()
		kasmosDir := filepath.Join(repoDir, ".kasmos")
		require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
		content := `
[sdk]
transcript_max_bytes = 1048576
transcript_max_turns = 500
`
		require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(content), 0o644))

		rm := newTestRepoManager(t)
		_, _, _, _, _, sdk, _, err := rm.resolveRepoConfig(repoDir)
		require.NoError(t, err)
		assert.Equal(t, int64(1<<20), sdk.TranscriptMaxBytes)
		assert.Equal(t, int64(500), sdk.TranscriptMaxTurns)
	})

	t.Run("explicit zero disables limits", func(t *testing.T) {
		repoDir := t.TempDir()
		kasmosDir := filepath.Join(repoDir, ".kasmos")
		require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
		content := `
[sdk]
transcript_max_bytes = 0
transcript_max_turns = 0
`
		require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(content), 0o644))

		rm := newTestRepoManager(t)
		_, _, _, _, _, sdk, _, err := rm.resolveRepoConfig(repoDir)
		require.NoError(t, err)
		assert.Equal(t, int64(0), sdk.TranscriptMaxBytes, "explicit zero must disable byte limit")
		assert.Equal(t, int64(0), sdk.TranscriptMaxTurns, "explicit zero must disable turn limit")
	})

	t.Run("negative values clamped to zero", func(t *testing.T) {
		repoDir := t.TempDir()
		kasmosDir := filepath.Join(repoDir, ".kasmos")
		require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
		content := `
[sdk]
transcript_max_bytes = -1024
transcript_max_turns = -50
`
		require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(content), 0o644))

		rm := newTestRepoManager(t)
		_, _, _, _, _, sdk, _, err := rm.resolveRepoConfig(repoDir)
		require.NoError(t, err)
		assert.Equal(t, int64(0), sdk.TranscriptMaxBytes, "negative must clamp to 0")
		assert.Equal(t, int64(0), sdk.TranscriptMaxTurns, "negative must clamp to 0")
	})
}

func TestRepoManager_Add_InvalidResourcesConfigFails(t *testing.T) {
	repoDir := t.TempDir()
	kasmosDir := filepath.Join(repoDir, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
	content := `
[resources]
profile = "custom"
build_jobs = 0
go_package_parallelism = 0
`
	require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(content), 0o644))

	rm := newTestRepoManager(t)
	err := rm.Add(repoDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project [resources] config")
	assert.Empty(t, rm.List(), "invalid resources config must not register the repo with a downgraded profile")
}

// TestSDKTranscriptRetention_RepoEntryPropagated verifies that the SDK limits
// are stored on the RepoEntry returned by Add/List so spawners can access them.
func TestSDKTranscriptRetention_RepoEntryPropagated(t *testing.T) {
	repoDir := t.TempDir()
	kasmosDir := filepath.Join(repoDir, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
	content := `
[sdk]
transcript_max_bytes = 2097152
transcript_max_turns = 100
`
	require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(content), 0o644))

	rm := newTestRepoManager(t)
	require.NoError(t, rm.Add(repoDir))
	repos := rm.List()
	require.Len(t, repos, 1)
	assert.Equal(t, int64(2<<20), repos[0].SDK.TranscriptMaxBytes)
	assert.Equal(t, int64(100), repos[0].SDK.TranscriptMaxTurns)
}

// TestWithSDKTranscriptRetention_SetsLimits verifies that withSDKTranscriptRetention
// copies entry.SDK limits into the SpawnOpts and sets SDKTranscriptLimitsSet.
func TestWithSDKTranscriptRetention_SetsLimits(t *testing.T) {
	entry := RepoEntry{
		SDK: config.SDKConfig{
			TranscriptMaxBytes: 4 << 20,
			TranscriptMaxTurns: 2000,
		},
	}
	base := loop.SpawnOpts{PlanFile: "plan.md"}
	got := withSDKTranscriptRetention(entry, base)

	assert.True(t, got.SDKTranscriptLimitsSet, "SDKTranscriptLimitsSet must be set")
	assert.Equal(t, int64(4<<20), got.SDKTranscriptMaxBytes)
	assert.Equal(t, int64(2000), got.SDKTranscriptMaxTurns)
	// Existing fields must be preserved.
	assert.Equal(t, "plan.md", got.PlanFile)
}

// TestWithSDKTranscriptRetention_ZeroLimitsAreValid verifies that zero values
// (unlimited) are forwarded with SDKTranscriptLimitsSet=true so the renderer
// explicitly disables its limits rather than falling back to defaults.
func TestWithSDKTranscriptRetention_ZeroLimitsAreValid(t *testing.T) {
	entry := RepoEntry{
		SDK: config.SDKConfig{
			TranscriptMaxBytes: 0, // unlimited
			TranscriptMaxTurns: 0, // unlimited
		},
	}
	got := withSDKTranscriptRetention(entry, loop.SpawnOpts{})
	assert.True(t, got.SDKTranscriptLimitsSet)
	assert.Equal(t, int64(0), got.SDKTranscriptMaxBytes)
	assert.Equal(t, int64(0), got.SDKTranscriptMaxTurns)
}
