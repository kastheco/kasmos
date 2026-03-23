package taskstore

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

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
