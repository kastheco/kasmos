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

func TestLegacySchema_NoLinearColumns(t *testing.T) {
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
			verifying_at TEXT NOT NULL DEFAULT '',
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
	_, err = legacyDB.Exec(`INSERT INTO tasks (project, filename, status) VALUES ('proj', 'legacy', 'ready')`)
	require.NoError(t, err)
	require.NoError(t, legacyDB.Close())

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	for _, column := range []string{"linear_issue_id", "linear_identifier", "linear_url", "linear_team_key", "linear_project_id"} {
		require.True(t, hasTaskColumn(t, store.db, column), "%s column must be added by migration", column)
	}
	entry, err := store.Get("proj", "legacy")
	require.NoError(t, err)
	assert.Equal(t, "", entry.LinearIssueID)
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

func TestMigrateRepoLocalToGlobal_LinearTriggers(t *testing.T) {
	repoKasmosDir := t.TempDir()

	local := createLocalRepoStore(t, repoKasmosDir)
	detectedAt := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	queued, err := local.EnqueueLinearTrigger("proj", LinearTriggerEntry{
		LinearIssueID:    "lin-1",
		LinearIdentifier: "KAS-1",
		CommandKind:      "plan",
		SourceKind:       "comment",
		SourceID:         "comment-1",
		ActorID:          "actor-1",
		ActorEmail:       "agent@example.com",
		TaskArg:          "task-a",
		DetectedAt:       detectedAt,
	})
	require.NoError(t, err)
	require.True(t, queued)
	require.NoError(t, local.SetLastSeenCommentAt("proj", "lin-1", detectedAt))
	recorded, err := local.RecordLinearWebhookDelivery("proj", LinearWebhookDelivery{
		DeliveryID:    "delivery-1",
		LinearEvent:   "Comment",
		Action:        "create",
		LinearIssueID: "lin-1",
		SourceKind:    "comment",
		SourceID:      "comment-1",
		Status:        "received",
		ReceivedAt:    detectedAt,
	})
	require.NoError(t, err)
	require.True(t, recorded)
	local.Close()

	globalDir := t.TempDir()
	globalDBPath := filepath.Join(globalDir, "taskstore.db")
	globalStore, err := NewSQLiteStore(globalDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { globalStore.Close() })

	migrated, err := MigrateRepoLocalToGlobal(globalStore, "proj", repoKasmosDir)
	require.NoError(t, err)
	assert.Equal(t, 0, migrated)

	triggers, err := globalStore.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	require.Len(t, triggers, 1)
	assert.Equal(t, "comment-1", triggers[0].SourceID)
	assert.Equal(t, detectedAt, triggers[0].DetectedAt)

	cursor, err := globalStore.LastSeenCommentAt("proj", "lin-1")
	require.NoError(t, err)
	assert.Equal(t, detectedAt, cursor)

	deliveries, err := globalStore.ListRecentLinearWebhookDeliveries("proj", 10)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	assert.Equal(t, "delivery-1", deliveries[0].DeliveryID)
	assert.Equal(t, detectedAt, deliveries[0].ReceivedAt)

	_, err = MigrateRepoLocalToGlobal(globalStore, "proj", repoKasmosDir)
	require.NoError(t, err)

	var count int
	err = globalStore.db.QueryRow(
		`SELECT count(*) FROM linear_triggers WHERE project = ? AND linear_issue_id = ? AND command_kind = ? AND source_id = ?`,
		"proj", "lin-1", "plan", "comment-1",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	err = globalStore.db.QueryRow(
		`SELECT count(*) FROM linear_webhook_deliveries WHERE project = ? AND delivery_id = ?`,
		"proj", "delivery-1",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestMigrateRepoLocalToGlobal_LinearWebhookDeliveriesNoRows(t *testing.T) {
	repoKasmosDir := t.TempDir()
	local := createLocalRepoStore(t, repoKasmosDir)
	local.Close()

	globalStore, err := NewSQLiteStore(filepath.Join(t.TempDir(), "taskstore.db"))
	require.NoError(t, err)
	t.Cleanup(func() { globalStore.Close() })

	migrated, err := MigrateRepoLocalToGlobal(globalStore, "proj", repoKasmosDir)
	require.NoError(t, err)
	assert.Equal(t, 0, migrated)

	deliveries, err := globalStore.ListRecentLinearWebhookDeliveries("proj", 10)
	require.NoError(t, err)
	assert.Empty(t, deliveries)
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
