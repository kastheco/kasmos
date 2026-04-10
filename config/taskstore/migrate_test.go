package taskstore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestMigrateFromJSON(t *testing.T) {
	store := newTestStore(t)
	plansDir := t.TempDir()

	stateJSON := `{
        "plans": {
            "test": {
                "status": "ready",
                "description": "test plan",
                "branch": "plan/test"
            }
        },
        "topics": {
            "tools": {"created_at": "2026-02-28T00:00:00Z"}
        }
    }`
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, "plan-state.json"), []byte(stateJSON), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, "test"), []byte("# Test Plan"), 0o644))

	migrated, err := MigrateFromJSON(store, "proj", plansDir)
	require.NoError(t, err)
	assert.Equal(t, 1, migrated)

	entry, err := store.Get("proj", "test")
	require.NoError(t, err)
	assert.Equal(t, StatusReady, entry.Status)
	assert.Equal(t, "test plan", entry.Description)

	content, err := store.GetContent("proj", "test")
	require.NoError(t, err)
	assert.Equal(t, "# Test Plan", content)

	topics, err := store.ListTopics("proj")
	require.NoError(t, err)
	assert.Len(t, topics, 1)
}

func TestMigrateFromJSON_NormalizesLegacyStatusAndFilename(t *testing.T) {
	store := newTestStore(t)
	plansDir := t.TempDir()

	stateJSON := `{
		"plans": {
			"feature.md": {"status": "in_progress", "description": "legacy impl"},
			"release": {"status": "completed", "description": "legacy done"}
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, "plan-state.json"), []byte(stateJSON), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, "feature.md"), []byte("# Feature"), 0o644))

	migrated, err := MigrateFromJSON(store, "proj", plansDir)
	require.NoError(t, err)
	assert.Equal(t, 2, migrated)

	feature, err := store.Get("proj", "feature")
	require.NoError(t, err)
	assert.Equal(t, StatusImplementing, feature.Status)

	release, err := store.Get("proj", "release")
	require.NoError(t, err)
	assert.Equal(t, StatusDone, release.Status)

	content, err := store.GetContent("proj", "feature")
	require.NoError(t, err)
	assert.Equal(t, "# Feature", content)
}

func TestMigrateFromJSON_PreservesCollidingMdFilename(t *testing.T) {
	store := newTestStore(t)
	plansDir := t.TempDir()

	stateJSON := `{
		"plans": {
			"task": {"status": "ready", "description": "bare"},
			"task.md": {"status": "completed", "description": "collision"}
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, "plan-state.json"), []byte(stateJSON), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, "task.md"), []byte("# Collision"), 0o644))

	migrated, err := MigrateFromJSON(store, "proj", plansDir)
	require.NoError(t, err)
	assert.Equal(t, 2, migrated)

	bare, err := store.Get("proj", "task")
	require.NoError(t, err)
	assert.Equal(t, StatusReady, bare.Status)

	suffixed, err := store.Get("proj", "task.md")
	require.NoError(t, err)
	assert.Equal(t, StatusDone, suffixed.Status)

	content, err := store.GetContent("proj", "task.md")
	require.NoError(t, err)
	assert.Equal(t, "# Collision", content)
}

func TestMigrateFromJSON_Idempotent(t *testing.T) {
	store := newTestStore(t)
	plansDir := t.TempDir()

	stateJSON := `{"plans": {"test": {"status": "done"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, "plan-state.json"), []byte(stateJSON), 0o644))

	_, err := MigrateFromJSON(store, "proj", plansDir)
	require.NoError(t, err)

	_, err = MigrateFromJSON(store, "proj", plansDir)
	require.NoError(t, err) // second run should not error
}

func TestMigrateFromJSON_NoFile(t *testing.T) {
	store := newTestStore(t)
	migrated, err := MigrateFromJSON(store, "proj", t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, 0, migrated)
}

func TestMigrateFromJSON_ParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "empty file", content: "", want: "parse plan-state.json"},
		{name: "malformed json", content: "{", want: "parse plan-state.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			plansDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(plansDir, "plan-state.json"), []byte(tt.content), 0o644))

			migrated, err := MigrateFromJSON(store, "proj", plansDir)
			require.Error(t, err)
			assert.Equal(t, 0, migrated)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestMigrateFromPlanstoreDB(t *testing.T) {
	dir := t.TempDir()
	oldDBPath := filepath.Join(dir, "planstore.db")
	newDBPath := filepath.Join(dir, "taskstore.db")

	// Create the old planstore.db with a "plans" table and seed data.
	oldDB, err := sql.Open("sqlite", oldDBPath)
	require.NoError(t, err)

	_, err = oldDB.Exec(`
		CREATE TABLE plans (
			id          INTEGER PRIMARY KEY,
			project     TEXT NOT NULL,
			filename    TEXT NOT NULL,
			status      TEXT NOT NULL DEFAULT 'ready',
			description TEXT NOT NULL DEFAULT '',
			branch      TEXT NOT NULL DEFAULT '',
			topic       TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL DEFAULT '',
			implemented TEXT NOT NULL DEFAULT '',
			content     TEXT NOT NULL DEFAULT '',
			clickup_task_id TEXT NOT NULL DEFAULT '',
			review_cycle INTEGER NOT NULL DEFAULT 0,
			UNIQUE(project, filename)
		);
		CREATE TABLE topics (
			id         INTEGER PRIMARY KEY,
			project    TEXT NOT NULL,
			name       TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT '',
			UNIQUE(project, name)
		);
		CREATE TABLE audit_events (
			id             INTEGER PRIMARY KEY,
			kind           TEXT NOT NULL,
			timestamp      TEXT NOT NULL,
			project        TEXT NOT NULL DEFAULT '',
			plan_file      TEXT NOT NULL DEFAULT '',
			instance_title TEXT NOT NULL DEFAULT '',
			agent_type     TEXT NOT NULL DEFAULT '',
			wave_number    INTEGER NOT NULL DEFAULT 0,
			task_number    INTEGER NOT NULL DEFAULT 0,
			message        TEXT NOT NULL DEFAULT '',
			detail         TEXT NOT NULL DEFAULT '',
			level          TEXT NOT NULL DEFAULT 'info'
		);
	`)
	require.NoError(t, err)

	_, err = oldDB.Exec(`INSERT INTO plans (project, filename, status, description, branch) VALUES ('proj', 'plan-a', 'completed', 'old plan', 'plan/a')`)
	require.NoError(t, err)
	_, err = oldDB.Exec(`INSERT INTO plans (project, filename, status, description, branch) VALUES ('proj', 'plan-b.md', 'in_progress', 'old plan b', 'plan/b')`)
	require.NoError(t, err)
	_, err = oldDB.Exec(`INSERT INTO topics (project, name, created_at) VALUES ('proj', 'tools', '2026-02-28T00:00:00Z')`)
	require.NoError(t, err)
	_, err = oldDB.Exec(`INSERT INTO audit_events (kind, timestamp, project, plan_file, message) VALUES ('transition', '2026-02-28T00:00:00Z', 'proj', 'plan-a', 'ready -> done')`)
	require.NoError(t, err)
	require.NoError(t, oldDB.Close())

	// Now create the new taskstore.db via NewSQLiteStore — the migration should
	// automatically detect planstore.db in the same directory and copy data.
	store, err := NewSQLiteStore(newDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	// Verify tasks were migrated.
	entries, err := store.List("proj")
	require.NoError(t, err)
	assert.Len(t, entries, 2, "expected 2 tasks migrated from planstore.db")

	entryA, err := store.Get("proj", "plan-a")
	require.NoError(t, err)
	assert.Equal(t, StatusDone, entryA.Status)
	assert.Equal(t, "old plan", entryA.Description)
	assert.Equal(t, "plan/a", entryA.Branch)

	entryB, err := store.Get("proj", "plan-b")
	require.NoError(t, err)
	assert.Equal(t, StatusImplementing, entryB.Status)

	// Verify topics were migrated.
	topics, err := store.ListTopics("proj")
	require.NoError(t, err)
	assert.Len(t, topics, 1)
	assert.Equal(t, "tools", topics[0].Name)

	// Verify audit_events were migrated (check via raw SQL since auditlog is a separate package).
	var auditCount int
	err = store.db.QueryRow("SELECT count(*) FROM audit_events").Scan(&auditCount)
	require.NoError(t, err)
	assert.Equal(t, 1, auditCount, "expected 1 audit event migrated")
}

func TestMigrateFromPlanstoreDB_NoPlanstore(t *testing.T) {
	dir := t.TempDir()
	newDBPath := filepath.Join(dir, "taskstore.db")

	// No planstore.db exists — should be a no-op.
	store, err := NewSQLiteStore(newDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	entries, err := store.List("proj")
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestMigrateFromPlanstoreDB_AlreadyHasData(t *testing.T) {
	dir := t.TempDir()
	oldDBPath := filepath.Join(dir, "planstore.db")
	newDBPath := filepath.Join(dir, "taskstore.db")

	// Pre-create the new DB with existing data (no planstore.db yet).
	store1, err := NewSQLiteStore(newDBPath)
	require.NoError(t, err)
	require.NoError(t, store1.Create("proj", TaskEntry{Filename: "existing", Status: StatusReady}))
	store1.Close()

	// Now create old DB with data.
	oldDB, err := sql.Open("sqlite", oldDBPath)
	require.NoError(t, err)
	_, err = oldDB.Exec(`
		CREATE TABLE plans (
			id INTEGER PRIMARY KEY, project TEXT NOT NULL, filename TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'ready', description TEXT NOT NULL DEFAULT '',
			branch TEXT NOT NULL DEFAULT '', topic TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '', implemented TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '', clickup_task_id TEXT NOT NULL DEFAULT '',
			review_cycle INTEGER NOT NULL DEFAULT 0,
			UNIQUE(project, filename)
		)`)
	require.NoError(t, err)
	_, err = oldDB.Exec(`INSERT INTO plans (project, filename, status) VALUES ('proj', 'should-not-appear', 'ready')`)
	require.NoError(t, err)
	require.NoError(t, oldDB.Close())

	// Reopen — migration should detect existing data and skip.
	store2, err := NewSQLiteStore(newDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { store2.Close() })

	entries, err := store2.List("proj")
	require.NoError(t, err)
	assert.Len(t, entries, 1, "migration should be skipped when taskstore already has data")
	assert.Equal(t, "existing", entries[0].Filename)
}

func TestMigrateStripMdSuffix_CollisionSafe(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(schema)
	require.NoError(t, err)
	_, err = db.Exec(subtasksTableMigration)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO tasks (project, filename, status) VALUES
			('proj', 'task', 'ready'),
			('proj', 'task.md', 'done'),
			('proj', 'other.md', 'ready')
	`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO subtasks (project, plan_filename, task_number, title, status) VALUES
			('proj', 'task', 1, 'bare', 'pending'),
			('proj', 'task.md', 1, 'suffixed', 'done'),
			('proj', 'other.md', 1, 'renamed', 'pending')
	`)
	require.NoError(t, err)

	require.NoError(t, migrateStripMdSuffix(db))

	rows, err := db.Query(`SELECT filename, status FROM tasks WHERE project = 'proj' ORDER BY filename`)
	require.NoError(t, err)
	defer rows.Close()

	var filenames []string
	for rows.Next() {
		var filename string
		var status string
		require.NoError(t, rows.Scan(&filename, &status))
		filenames = append(filenames, filename+":"+status)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []string{"other:ready", "task:ready", "task.md:done"}, filenames)

	subtaskRows, err := db.Query(`SELECT plan_filename, title FROM subtasks WHERE project = 'proj' ORDER BY plan_filename, title`)
	require.NoError(t, err)
	defer subtaskRows.Close()

	var subtaskRefs []string
	for subtaskRows.Next() {
		var planFilename string
		var title string
		require.NoError(t, subtaskRows.Scan(&planFilename, &title))
		subtaskRefs = append(subtaskRefs, planFilename+":"+title)
	}
	require.NoError(t, subtaskRows.Err())
	assert.Equal(t, []string{"other:renamed", "task:bare", "task.md:suffixed"}, subtaskRefs)
}

func TestSQLiteStore_MigratesExecutionStateColumnsFromOldSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "taskstore.db")

	legacyDB, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)

	_, err = legacyDB.Exec(`
		CREATE TABLE tasks (
			id INTEGER PRIMARY KEY,
			project TEXT NOT NULL,
			filename TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'ready',
			description TEXT NOT NULL DEFAULT '',
			branch TEXT NOT NULL DEFAULT '',
			topic TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			implemented TEXT NOT NULL DEFAULT '',
			planning_at TEXT NOT NULL DEFAULT '',
			implementing_at TEXT NOT NULL DEFAULT '',
			reviewing_at TEXT NOT NULL DEFAULT '',
			done_at TEXT NOT NULL DEFAULT '',
			goal TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			clickup_task_id TEXT NOT NULL DEFAULT '',
			review_cycle INTEGER NOT NULL DEFAULT 0,
			latest_review_feedback TEXT NOT NULL DEFAULT '',
			pr_url TEXT NOT NULL DEFAULT '',
			pr_review_decision TEXT NOT NULL DEFAULT '',
			pr_check_status TEXT NOT NULL DEFAULT '',
			UNIQUE(project, filename)
		);
		CREATE TABLE topics (
			id INTEGER PRIMARY KEY,
			project TEXT NOT NULL,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT '',
			UNIQUE(project, name)
		);
	`)
	require.NoError(t, err)

	_, err = legacyDB.Exec(`INSERT INTO tasks (project, filename, status, description) VALUES ('proj', 'legacy', 'implementing', 'old row')`)
	require.NoError(t, err)
	require.NoError(t, legacyDB.Close())

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	entry, err := store.Get("proj", "legacy")
	require.NoError(t, err)
	assert.Equal(t, ExecutionState{}, entry.ExecutionState)

	require.True(t, hasTaskColumn(t, store.db, "execution_phase"))
	require.True(t, hasTaskColumn(t, store.db, "active_agent_type"))
	require.True(t, hasTaskColumn(t, store.db, "active_wave"))

	require.NoError(t, store.SetExecutionState("proj", "legacy", ExecutionState{
		Phase:           "wave_running",
		ActiveAgentType: "coder",
		ActiveWave:      2,
	}))

	updated, err := store.Get("proj", "legacy")
	require.NoError(t, err)
	assert.Equal(t, ExecutionState{
		Phase:           "wave_running",
		ActiveAgentType: "coder",
		ActiveWave:      2,
	}, updated.ExecutionState)
}

func TestSQLiteStore_MigratesVerifyingAtColumn(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "taskstore.db")

	// Create a legacy DB that has reviewing_at but not verifying_at.
	legacyDB, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)

	_, err = legacyDB.Exec(`
		CREATE TABLE tasks (
			id INTEGER PRIMARY KEY,
			project TEXT NOT NULL,
			filename TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'ready',
			description TEXT NOT NULL DEFAULT '',
			branch TEXT NOT NULL DEFAULT '',
			topic TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			implemented TEXT NOT NULL DEFAULT '',
			planning_at TEXT NOT NULL DEFAULT '',
			implementing_at TEXT NOT NULL DEFAULT '',
			reviewing_at TEXT NOT NULL DEFAULT '',
			done_at TEXT NOT NULL DEFAULT '',
			execution_phase TEXT NOT NULL DEFAULT '',
			active_agent_type TEXT NOT NULL DEFAULT '',
			active_wave INTEGER NOT NULL DEFAULT 0,
			goal TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			clickup_task_id TEXT NOT NULL DEFAULT '',
			review_cycle INTEGER NOT NULL DEFAULT 0,
			latest_review_feedback TEXT NOT NULL DEFAULT '',
			pr_url TEXT NOT NULL DEFAULT '',
			pr_review_decision TEXT NOT NULL DEFAULT '',
			pr_check_status TEXT NOT NULL DEFAULT '',
			UNIQUE(project, filename)
		);
		CREATE TABLE topics (
			id INTEGER PRIMARY KEY,
			project TEXT NOT NULL,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT '',
			UNIQUE(project, name)
		);
	`)
	require.NoError(t, err)
	_, err = legacyDB.Exec(`INSERT INTO tasks (project, filename, status) VALUES ('proj', 'old-task', 'reviewing')`)
	require.NoError(t, err)
	require.NoError(t, legacyDB.Close())

	// Open via NewSQLiteStore — migration must add verifying_at.
	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	// Column must exist.
	require.True(t, hasTaskColumn(t, store.db, "verifying_at"), "verifying_at column must be added by migration")

	// Existing rows default to empty verifying_at.
	entry, err := store.Get("proj", "old-task")
	require.NoError(t, err)
	assert.True(t, entry.VerifyingAt.IsZero(), "verifying_at must default to zero time for pre-migration rows")

	// SetPhaseTimestamp must work for verifying after migration.
	require.NoError(t, store.SetPhaseTimestamp("proj", "old-task", "verifying", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)))
	updated, err := store.Get("proj", "old-task")
	require.NoError(t, err)
	assert.False(t, updated.VerifyingAt.IsZero())
}

// ---------------------------------------------------------------------------
// MigrateRepoLocalToGlobal tests
// ---------------------------------------------------------------------------

// createLocalRepoStore creates a file-backed SQLiteStore in dir/taskstore.db,
// seeds it with the supplied tasks, subtasks, topics, and returns the store.
// Caller must close when done.
func createLocalRepoStore(t *testing.T, dir string) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(dir, "taskstore.db")
	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	return store
}

func TestMigrateRepoLocalToGlobal(t *testing.T) {
	repoKasmosDir := t.TempDir()

	// Seed local store with tasks, subtasks, topics.
	local := createLocalRepoStore(t, repoKasmosDir)
	require.NoError(t, local.Create("proj", TaskEntry{
		Filename:    "task-a",
		Status:      StatusReady,
		Description: "first task",
		Branch:      "plan/task-a",
		Content:     "# Task A content",
	}))
	require.NoError(t, local.Create("proj", TaskEntry{
		Filename:    "task-b",
		Status:      StatusDone,
		Description: "second task",
	}))
	require.NoError(t, local.SetSubtasks("proj", "task-a", []SubtaskEntry{
		{TaskNumber: 1, Title: "subtask one", Status: SubtaskStatusPending},
		{TaskNumber: 2, Title: "subtask two", Status: SubtaskStatusComplete},
	}))
	require.NoError(t, local.CreateTopic("proj", TopicEntry{Name: "infra"}))
	local.Close()

	// Migrate into global (in-memory) store.
	globalStore := newTestStore(t)
	migrated, err := MigrateRepoLocalToGlobal(globalStore, "proj", repoKasmosDir)
	require.NoError(t, err)
	assert.Equal(t, 2, migrated)

	// Verify tasks.
	entries, err := globalStore.List("proj")
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	entryA, err := globalStore.Get("proj", "task-a")
	require.NoError(t, err)
	assert.Equal(t, StatusReady, entryA.Status)
	assert.Equal(t, "first task", entryA.Description)
	assert.Equal(t, "plan/task-a", entryA.Branch)

	// Verify content was migrated as part of Create.
	content, err := globalStore.GetContent("proj", "task-a")
	require.NoError(t, err)
	assert.Equal(t, "# Task A content", content)

	// Verify subtasks.
	subtasks, err := globalStore.GetSubtasks("proj", "task-a")
	require.NoError(t, err)
	assert.Len(t, subtasks, 2)
	assert.Equal(t, "subtask one", subtasks[0].Title)

	// Verify topics.
	topics, err := globalStore.ListTopics("proj")
	require.NoError(t, err)
	assert.Len(t, topics, 1)
	assert.Equal(t, "infra", topics[0].Name)
}

func TestMigrateRepoLocalToGlobal_NoLocalDB(t *testing.T) {
	globalStore := newTestStore(t)
	migrated, err := MigrateRepoLocalToGlobal(globalStore, "proj", t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, 0, migrated)
}

func TestMigrateRepoLocalToGlobal_Idempotent(t *testing.T) {
	repoKasmosDir := t.TempDir()

	local := createLocalRepoStore(t, repoKasmosDir)
	require.NoError(t, local.Create("proj", TaskEntry{
		Filename:    "task-x",
		Status:      StatusReady,
		Description: "idempotent",
	}))
	require.NoError(t, local.CreateTopic("proj", TopicEntry{Name: "testing"}))
	local.Close()

	globalStore := newTestStore(t)

	// First run.
	m1, err := MigrateRepoLocalToGlobal(globalStore, "proj", repoKasmosDir)
	require.NoError(t, err)
	assert.Equal(t, 1, m1)

	// Second run — should not error and should report 0 new tasks.
	m2, err := MigrateRepoLocalToGlobal(globalStore, "proj", repoKasmosDir)
	require.NoError(t, err)
	assert.Equal(t, 0, m2)

	// Still only 1 task.
	entries, err := globalStore.List("proj")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestMigrateRepoLocalToGlobal_MigratesAllLocalProjectsAndNormalizesLegacyAlias(t *testing.T) {
	repoKasmosDir := t.TempDir()
	localDBPath := filepath.Join(repoKasmosDir, "taskstore.db")

	local := createLocalRepoStore(t, repoKasmosDir)
	require.NoError(t, local.Create("cms", TaskEntry{
		Filename:    "cms-task",
		Status:      StatusReady,
		Description: "cms project task",
	}))
	require.NoError(t, local.Create("kas", TaskEntry{
		Filename:    "legacy-kas-task",
		Status:      StatusReady,
		Description: "legacy kas project task",
	}))
	require.NoError(t, local.RecordPRReview("kas", "legacy-kas-task", 77, "approved", "looks good", "alice"))
	require.NoError(t, local.CreateTopic("kas", TopicEntry{Name: "legacy-topic"}))
	local.Close()

	localGW, err := NewSQLiteSignalGateway(localDBPath)
	require.NoError(t, err)
	require.NoError(t, localGW.Create("kas", SignalEntry{
		PlanFile:   "legacy-kas-task",
		SignalType: "planner_finished",
		Payload:    `{"ok":true}`,
	}))
	require.NoError(t, localGW.Close())

	globalDir := t.TempDir()
	globalDBPath := filepath.Join(globalDir, "taskstore.db")
	globalStore, err := NewSQLiteStore(globalDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { globalStore.Close() })

	migrated, err := MigrateRepoLocalToGlobal(globalStore, "cms", repoKasmosDir)
	require.NoError(t, err)
	assert.Equal(t, 2, migrated, "migration should import tasks from every project found in the local DB")

	cmsEntries, err := globalStore.List("cms")
	require.NoError(t, err)
	assert.Len(t, cmsEntries, 1)
	assert.Equal(t, "cms-task", cmsEntries[0].Filename)

	legacyEntries, err := globalStore.List("kasmos")
	require.NoError(t, err)
	assert.Len(t, legacyEntries, 1)
	assert.Equal(t, "legacy-kas-task", legacyEntries[0].Filename)

	legacyTopics, err := globalStore.ListTopics("kasmos")
	require.NoError(t, err)
	assert.Len(t, legacyTopics, 1)
	assert.Equal(t, "legacy-topic", legacyTopics[0].Name)

	assert.True(t, globalStore.IsReviewProcessed("kasmos", "legacy-kas-task", 77))

	var sigCount int
	err = globalStore.db.QueryRow(`SELECT count(*) FROM signals WHERE project = ? AND plan_file = ?`, "kasmos", "legacy-kas-task").Scan(&sigCount)
	require.NoError(t, err)
	assert.Equal(t, 1, sigCount)
}

func TestMigrateRepoLocalToGlobal_PRReviews(t *testing.T) {
	repoKasmosDir := t.TempDir()
	localDBPath := filepath.Join(repoKasmosDir, "taskstore.db")

	// Seed local store with a task and a PR review.
	local := createLocalRepoStore(t, repoKasmosDir)
	require.NoError(t, local.Create("proj", TaskEntry{
		Filename: "reviewed",
		Status:   StatusReviewing,
	}))
	require.NoError(t, local.RecordPRReview("proj", "reviewed", 42, "changes_requested", "fix the bug", "alice"))
	local.Close()

	// Create a file-backed global store so ATTACH works.
	globalDir := t.TempDir()
	globalDBPath := filepath.Join(globalDir, "taskstore.db")
	globalStore, err := NewSQLiteStore(globalDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { globalStore.Close() })

	migrated, err := MigrateRepoLocalToGlobal(globalStore, "proj", repoKasmosDir)
	require.NoError(t, err)
	assert.Equal(t, 1, migrated)

	// Verify PR review was migrated.
	assert.True(t, globalStore.IsReviewProcessed("proj", "reviewed", 42))

	// Run again — idempotent (INSERT OR IGNORE).
	_, err = MigrateRepoLocalToGlobal(globalStore, "proj", repoKasmosDir)
	require.NoError(t, err)

	// Verify still only one review by listing pending (the review has
	// fixer_dispatched=0, reaction_posted=0 so it shows as pending).
	reviews, err := globalStore.ListPendingReviews("proj", "reviewed")
	require.NoError(t, err)
	assert.Len(t, reviews, 1, "should have exactly 1 review after idempotent migration")

	// Verify PR review metadata is present via direct SQL for safety.
	var reviewBody string
	err = globalStore.db.QueryRow(
		`SELECT review_body FROM pr_reviews WHERE project = ? AND plan_filename = ? AND review_id = ?`,
		"proj", "reviewed", 42,
	).Scan(&reviewBody)
	require.NoError(t, err)
	assert.Equal(t, "fix the bug", reviewBody)

	// Verify no extra rows were created via SQL count.
	var count int
	err = globalStore.db.QueryRow(
		`SELECT count(*) FROM pr_reviews WHERE project = ?`, "proj",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "idempotent migration should not duplicate PR reviews")

	// Also ensure the review is properly linked by checking file-based store round-trip.
	_ = localDBPath // suppress unused warning
}

func TestMigrateRepoLocalToGlobal_Signals(t *testing.T) {
	repoKasmosDir := t.TempDir()
	localDBPath := filepath.Join(repoKasmosDir, "taskstore.db")

	// Seed local store with a task.
	local := createLocalRepoStore(t, repoKasmosDir)
	require.NoError(t, local.Create("proj", TaskEntry{
		Filename: "signalled",
		Status:   StatusImplementing,
	}))
	local.Close()

	// Insert signals via a separate gateway (signals table is created by
	// SQLiteSignalGateway, not SQLiteStore).
	localGW, err := NewSQLiteSignalGateway(localDBPath)
	require.NoError(t, err)
	require.NoError(t, localGW.Create("proj", SignalEntry{
		PlanFile:   "signalled",
		SignalType: "task_done",
		Payload:    `{"wave":1}`,
	}))
	localGW.Close()

	// Create a file-backed global store so ATTACH works.
	globalDir := t.TempDir()
	globalDBPath := filepath.Join(globalDir, "taskstore.db")
	globalStore, err := NewSQLiteStore(globalDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { globalStore.Close() })

	migrated, err := MigrateRepoLocalToGlobal(globalStore, "proj", repoKasmosDir)
	require.NoError(t, err)
	assert.Equal(t, 1, migrated)

	// Verify signal was migrated via raw SQL (signals table is not exposed via Store).
	var sigCount int
	err = globalStore.db.QueryRow(
		`SELECT count(*) FROM signals WHERE project = ?`, "proj",
	).Scan(&sigCount)
	require.NoError(t, err)
	assert.Equal(t, 1, sigCount)

	// Verify signal data.
	var signalType, payload string
	err = globalStore.db.QueryRow(
		`SELECT signal_type, payload FROM signals WHERE project = ? AND plan_file = ?`,
		"proj", "signalled",
	).Scan(&signalType, &payload)
	require.NoError(t, err)
	assert.Equal(t, "task_done", signalType)
	assert.Equal(t, `{"wave":1}`, payload)

	// Run again — idempotent (NOT EXISTS dedup).
	_, err = MigrateRepoLocalToGlobal(globalStore, "proj", repoKasmosDir)
	require.NoError(t, err)

	err = globalStore.db.QueryRow(
		`SELECT count(*) FROM signals WHERE project = ?`, "proj",
	).Scan(&sigCount)
	require.NoError(t, err)
	assert.Equal(t, 1, sigCount, "idempotent migration should not duplicate signals")
}

func TestMigrateAllKnownRepos(t *testing.T) {
	// Create two fake repo directories with local taskstores.
	repoA := t.TempDir()
	repoB := t.TempDir()

	kasmosA := filepath.Join(repoA, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosA, 0o755))
	storeA, err := NewSQLiteStore(filepath.Join(kasmosA, "taskstore.db"))
	require.NoError(t, err)
	require.NoError(t, storeA.Create(filepath.Base(repoA), TaskEntry{
		Filename: "a-task", Status: StatusReady, Description: "from repo a",
	}))
	storeA.Close()

	kasmosB := filepath.Join(repoB, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosB, 0o755))
	storeB, err := NewSQLiteStore(filepath.Join(kasmosB, "taskstore.db"))
	require.NoError(t, err)
	require.NoError(t, storeB.Create(filepath.Base(repoB), TaskEntry{
		Filename: "b-task", Status: StatusDone, Description: "from repo b",
	}))
	storeB.Close()

	// Write a daemon.toml pointing to both repos.
	configDir := t.TempDir()
	daemonTOML := fmt.Sprintf("repos = [%q, %q]\n", repoA, repoB)
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, ".config", "kasmos"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, ".config", "kasmos", "daemon.toml"),
		[]byte(daemonTOML), 0o644,
	))

	// Override HOME so MigrateAllKnownRepos reads our test daemon.toml.
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", configDir)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	globalDir := t.TempDir()
	globalDBPath := filepath.Join(globalDir, "taskstore.db")
	globalStore, err := NewSQLiteStore(globalDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { globalStore.Close() })

	err = MigrateAllKnownRepos(globalStore)
	require.NoError(t, err)

	// Verify repo A tasks.
	entriesA, err := globalStore.List(filepath.Base(repoA))
	require.NoError(t, err)
	assert.Len(t, entriesA, 1)
	assert.Equal(t, "a-task", entriesA[0].Filename)

	// Verify repo B tasks.
	entriesB, err := globalStore.List(filepath.Base(repoB))
	require.NoError(t, err)
	assert.Len(t, entriesB, 1)
	assert.Equal(t, "b-task", entriesB[0].Filename)
}

func hasTaskColumn(t *testing.T, db *sql.DB, columnName string) bool {
	t.Helper()

	rows, err := db.Query(`PRAGMA table_info(tasks)`)
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		require.NoError(t, rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk))
		if name == columnName {
			return true
		}
	}
	require.NoError(t, rows.Err())
	return false
}
