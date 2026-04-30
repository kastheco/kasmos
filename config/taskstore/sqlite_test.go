package taskstore_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// FromDB constructor tests
// ---------------------------------------------------------------------------

func TestSQLiteStoreFromDB_BasicOps(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "shared.db")

	db, err := taskstore.OpenSharedDB(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)

	// Basic CRUD through a FromDB-constructed store.
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: "from-db-task",
		Status:   taskstore.StatusReady,
	}))

	got, err := store.Get("proj", "from-db-task")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, got.Status)
	assert.Equal(t, "from-db-task", got.Filename)
}

func TestSQLiteStoreFromDB_CloseIsNoOp(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "shared.db")

	db, err := taskstore.OpenSharedDB(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)

	// Close must be a no-op — it must not close the shared *sql.DB.
	require.NoError(t, store.Close())

	// The underlying pool must still be alive.
	require.NoError(t, db.Ping())

	// A second store on the same DB must still work.
	store2, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)
	require.NoError(t, store2.Create("proj", taskstore.TaskEntry{Filename: "still-alive", Status: taskstore.StatusReady}))
	got, err := store2.Get("proj", "still-alive")
	require.NoError(t, err)
	assert.Equal(t, "still-alive", got.Filename)
}

func TestSQLiteStoreFromDB_SchemaComplete(t *testing.T) {
	// NewSQLiteStoreFromDB must run schema migrations and expose a fully functional store.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "taskstore.db")

	db, err := taskstore.OpenSharedDB(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)

	// Schema must be fully functional.
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: "schema-ok",
		Status:   taskstore.StatusReady,
	}))
	got, err := store.Get("proj", "schema-ok")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, got.Status)
}

func newTestStore(t *testing.T) taskstore.Store {
	t.Helper()
	store, err := taskstore.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func newConcreteTestStore(t *testing.T) *taskstore.SQLiteStore {
	t.Helper()
	store, err := taskstore.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func createReadyTask(t *testing.T, store taskstore.Store, project, filename string) {
	t.Helper()
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: filename,
		Status:   taskstore.StatusReady,
	}))
}

func TestSQLiteStore_CreateAndGet(t *testing.T) {
	store := newTestStore(t)
	entry := taskstore.TaskEntry{
		Filename:    "test-plan",
		Status:      taskstore.StatusReady,
		Description: "test plan",
		Branch:      "plan/test-plan",
		CreatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.Create("kasmos", entry))

	got, err := store.Get("kasmos", "test-plan")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, got.Status)
	assert.Equal(t, "test plan", got.Description)
}

func TestSQLiteStore_MdSuffixMigration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "taskstore.db")

	store, err := taskstore.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	// Insert legacy .md-suffixed entries to simulate a pre-migration database.
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{Filename: "foo.md", Status: taskstore.StatusReady}))
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{Filename: "bar.md", Status: taskstore.StatusDone}))
	require.NoError(t, store.SetSubtasks("proj", "foo.md", []taskstore.SubtaskEntry{{TaskNumber: 1, Title: "sub1", Status: taskstore.SubtaskStatusPending}}))
	require.NoError(t, store.Close())

	// Reopen — migration must strip '.md' from both tasks and subtasks.
	store2, err := taskstore.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store2.Close()

	plans, err := store2.List("proj")
	require.NoError(t, err)
	require.Len(t, plans, 2)
	assert.Equal(t, "bar", plans[0].Filename)
	assert.Equal(t, "foo", plans[1].Filename)

	// Subtasks must be retrievable by the stripped filename.
	subs, err := store2.GetSubtasks("proj", "foo")
	require.NoError(t, err)
	assert.Len(t, subs, 1)
	assert.Equal(t, "sub1", subs[0].Title)
}

func TestSQLiteStore_ListByStatus(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "kasmos", "a")
	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "b", Status: taskstore.StatusDone}))
	createReadyTask(t, store, "kasmos", "c")

	plans, err := store.ListByStatus("kasmos", taskstore.StatusReady)
	require.NoError(t, err)
	assert.Len(t, plans, 2)
}

func TestSQLiteStore_ProjectIsolation(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "project-a", "x")
	createReadyTask(t, store, "project-b", "y")

	plans, err := store.List("project-a")
	require.NoError(t, err)
	assert.Len(t, plans, 1)
	assert.Equal(t, "x", plans[0].Filename)
}

func TestSQLiteStore_Update(t *testing.T) {
	store := newTestStore(t)
	entry := taskstore.TaskEntry{
		Filename:    "update-test",
		Status:      taskstore.StatusReady,
		Description: "original description",
		Branch:      "plan/update-test",
		CreatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.Create("kasmos", entry))

	entry.Status = taskstore.StatusImplementing
	entry.Description = "updated description"
	require.NoError(t, store.Update("kasmos", "update-test", entry))

	got, err := store.Get("kasmos", "update-test")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, got.Status)
	assert.Equal(t, "updated description", got.Description)
}

func TestSQLiteStore_ExecutionStateRoundTrip(t *testing.T) {
	store := newConcreteTestStore(t)
	entry := taskstore.TaskEntry{
		Filename: "execution-state",
		Status:   taskstore.StatusImplementing,
		ExecutionState: taskstore.ExecutionState{
			Phase:           "wave_running",
			ActiveAgentType: "coder",
			ActiveWave:      2,
		},
	}
	require.NoError(t, store.Create("proj", entry))

	got, err := store.Get("proj", "execution-state")
	require.NoError(t, err)
	assert.Equal(t, entry.ExecutionState, got.ExecutionState)

	entry.ExecutionState = taskstore.ExecutionState{
		Phase:           "reviewing",
		ActiveAgentType: "reviewer",
		ActiveWave:      3,
	}
	require.NoError(t, store.Update("proj", "execution-state", entry))
	require.NoError(t, store.SetExecutionState("proj", "execution-state", taskstore.ExecutionState{
		Phase:           "fixing",
		ActiveAgentType: "fixer",
		ActiveWave:      4,
	}))

	got, err = store.Get("proj", "execution-state")
	require.NoError(t, err)
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           "fixing",
		ActiveAgentType: "fixer",
		ActiveWave:      4,
	}, got.ExecutionState)

	entries, err := store.List("proj")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, got.ExecutionState, entries[0].ExecutionState)
}

func TestSQLiteStore_ExecutionStateZeroValueDefaults(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "proj", "defaults")

	got, err := store.Get("proj", "defaults")
	require.NoError(t, err)
	assert.Equal(t, taskstore.ExecutionState{}, got.ExecutionState)

	entries, err := store.List("proj")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, taskstore.ExecutionState{}, entries[0].ExecutionState)
}

// TestSQLiteStore_UpdatePreservesContent verifies that Update does not
// overwrite content stored via SetContent. This is a regression test for a bug
// where every FSM status transition would nuke the content column because
// Update included content in its SET clause and callers passed empty content.
func TestSQLiteStore_UpdatePreservesContent(t *testing.T) {
	store := newTestStore(t)
	entry := taskstore.TaskEntry{
		Filename: "content-preserve",
		Status:   taskstore.StatusPlanning,
		Branch:   "plan/content-preserve",
	}
	require.NoError(t, store.Create("kasmos", entry))
	require.NoError(t, store.SetContent("kasmos", "content-preserve", "# My Plan\n\n## Wave 1\n"))

	// Simulate an FSM transition: update status without setting content.
	entry.Status = taskstore.StatusReady
	require.NoError(t, store.Update("kasmos", "content-preserve", entry))

	content, err := store.GetContent("kasmos", "content-preserve")
	require.NoError(t, err)
	assert.Equal(t, "# My Plan\n\n## Wave 1\n", content, "content must survive a metadata-only Update")
}

func TestSQLiteStore_Rename(t *testing.T) {
	store := newTestStore(t)
	entry := taskstore.TaskEntry{
		Filename:  "old-name",
		Status:    taskstore.StatusReady,
		CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, store.Create("kasmos", entry))

	require.NoError(t, store.Rename("kasmos", "old-name", "new-name"))

	_, err := store.Get("kasmos", "old-name")
	assert.Error(t, err)

	got, err := store.Get("kasmos", "new-name")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, got.Status)
}

// TestSQLiteStore_RenameCascadesChildren ensures that renaming a task which
// already has derived rows in subtasks and pr_reviews carries those child rows
// along to the new filename instead of failing a foreign key check or leaving
// orphan rows behind. The pr_reviews / subtasks foreign keys use
// ON DELETE CASCADE only, so the rename implementation must move the children
// explicitly inside a single deferred-FK transaction.
func TestSQLiteStore_RenameCascadesChildren(t *testing.T) {
	store := newTestStore(t)

	createReadyTask(t, store, "proj", "before-rename")
	require.NoError(t, store.SetContent("proj", "before-rename", "# content"))
	require.NoError(t, store.SetSubtasks("proj", "before-rename", []taskstore.SubtaskEntry{
		{TaskNumber: 1, Title: "alpha", Status: taskstore.SubtaskStatusPending},
		{TaskNumber: 2, Title: "beta", Status: taskstore.SubtaskStatusPending},
	}))
	require.NoError(t, store.RecordPRReview("proj", "before-rename", 42, "COMMENTED", "a review", "reviewer"))

	require.NoError(t, store.Rename("proj", "before-rename", "after-rename"))

	_, err := store.Get("proj", "before-rename")
	assert.Error(t, err)

	got, err := store.Get("proj", "after-rename")
	require.NoError(t, err)
	assert.Equal(t, "after-rename", got.Filename)

	subtasks, err := store.GetSubtasks("proj", "after-rename")
	require.NoError(t, err)
	require.Len(t, subtasks, 2)
	assert.Equal(t, "alpha", subtasks[0].Title)
	assert.Equal(t, "beta", subtasks[1].Title)

	oldSubtasks, err := store.GetSubtasks("proj", "before-rename")
	require.NoError(t, err)
	assert.Empty(t, oldSubtasks)

	pending, err := store.ListPendingReviews("proj", "after-rename")
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, 42, pending[0].ReviewID)

	oldPending, err := store.ListPendingReviews("proj", "before-rename")
	require.NoError(t, err)
	assert.Empty(t, oldPending)
}

func TestSQLiteStore_RenameNotFoundRollsBackTransaction(t *testing.T) {
	store := newTestStore(t)

	createReadyTask(t, store, "proj", "existing-task")

	err := store.Rename("proj", "missing-task", "renamed-task")
	require.EqualError(t, err, "plan not found: proj/missing-task")

	got, err := store.Get("proj", "existing-task")
	require.NoError(t, err)
	assert.Equal(t, "existing-task", got.Filename)

	require.NoError(t, store.Rename("proj", "existing-task", "renamed-task"))
}

func TestSQLiteStore_Delete(t *testing.T) {
	store := newTestStore(t)

	createReadyTask(t, store, "proj", "task-a")
	require.NoError(t, store.SetContent("proj", "task-a", "# deleted task"))
	require.NoError(t, store.SetSubtasks("proj", "task-a", []taskstore.SubtaskEntry{{TaskNumber: 1, Title: "child", Status: taskstore.SubtaskStatusPending}}))
	require.NoError(t, store.RecordPRReview("proj", "task-a", 101, "COMMENTED", "delete me", "reviewer"))

	createReadyTask(t, store, "other", "task-a")
	require.NoError(t, store.SetContent("other", "task-a", "# survivor"))

	require.NoError(t, store.Delete("proj", "task-a"))

	_, err := store.Get("proj", "task-a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	_, err = store.GetContent("proj", "task-a")
	require.Error(t, err)
	assert.Equal(t, "plan not found: proj/task-a", err.Error())

	subtasks, err := store.GetSubtasks("proj", "task-a")
	require.NoError(t, err)
	assert.Empty(t, subtasks)

	pending, err := store.ListPendingReviews("proj", "task-a")
	require.NoError(t, err)
	assert.Empty(t, pending)

	otherContent, err := store.GetContent("other", "task-a")
	require.NoError(t, err)
	assert.Equal(t, "# survivor", otherContent)

	err = store.Delete("proj", "task-a")
	require.Error(t, err)
	assert.Equal(t, "plan not found: proj/task-a", err.Error())
}

func TestSQLiteStore_ListByTopic(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "a", Status: taskstore.StatusReady, Topic: "auth"}))
	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "b", Status: taskstore.StatusReady, Topic: "auth"}))
	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "c", Status: taskstore.StatusReady, Topic: "storage"}))

	plans, err := store.ListByTopic("kasmos", "auth")
	require.NoError(t, err)
	assert.Len(t, plans, 2)
}

func TestSQLiteStore_Topics(t *testing.T) {
	store := newTestStore(t)
	topic := taskstore.TopicEntry{
		Name:      "auth",
		CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, store.CreateTopic("kasmos", topic))

	topics, err := store.ListTopics("kasmos")
	require.NoError(t, err)
	assert.Len(t, topics, 1)
	assert.Equal(t, "auth", topics[0].Name)
}

func TestSQLiteStore_Ping(t *testing.T) {
	store := newTestStore(t)
	assert.NoError(t, store.Ping())
}

func TestSQLiteStore_GetNotFound(t *testing.T) {
	store := newTestStore(t)
	_, err := store.Get("kasmos", "nonexistent")
	assert.Error(t, err)
}

func TestSQLiteStore_CreateDuplicate(t *testing.T) {
	store := newTestStore(t)
	entry := taskstore.TaskEntry{Filename: "dup", Status: taskstore.StatusReady}
	require.NoError(t, store.Create("kasmos", entry))
	err := store.Create("kasmos", entry)
	assert.Error(t, err)
}

func TestSQLiteStore_ListSortedByFilename(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "kasmos", "c")
	createReadyTask(t, store, "kasmos", "a")
	createReadyTask(t, store, "kasmos", "b")

	plans, err := store.List("kasmos")
	require.NoError(t, err)
	require.Len(t, plans, 3)
	assert.Equal(t, "a", plans[0].Filename)
	assert.Equal(t, "b", plans[1].Filename)
	assert.Equal(t, "c", plans[2].Filename)
}

func TestSQLiteStore_CreateWithContent(t *testing.T) {
	store := newTestStore(t)
	entry := taskstore.TaskEntry{
		Filename: "test",
		Status:   taskstore.StatusReady,
		Content:  "# Test Plan\n\n## Wave 1\n\n### Task 1: Do thing\n",
	}
	require.NoError(t, store.Create("proj", entry))
	got, err := store.Get("proj", "test")
	require.NoError(t, err)
	assert.Equal(t, entry.Content, got.Content)
}

func TestSQLiteStore_GetContent(t *testing.T) {
	store := newTestStore(t)
	entry := taskstore.TaskEntry{
		Filename: "test",
		Status:   taskstore.StatusReady,
		Content:  "# Full Plan Content",
	}
	require.NoError(t, store.Create("proj", entry))
	content, err := store.GetContent("proj", "test")
	require.NoError(t, err)
	assert.Equal(t, "# Full Plan Content", content)
}

func TestSQLiteStore_SetContent(t *testing.T) {
	store := newTestStore(t)
	entry := taskstore.TaskEntry{Filename: "test", Status: taskstore.StatusReady}
	require.NoError(t, store.Create("proj", entry))
	require.NoError(t, store.SetContent("proj", "test", "# Updated"))
	content, err := store.GetContent("proj", "test")
	require.NoError(t, err)
	assert.Equal(t, "# Updated", content)
}

func TestClickUpTaskIDRoundTrip(t *testing.T) {
	store := newTestStore(t)
	entry := taskstore.TaskEntry{Filename: "clickup-test", Status: taskstore.StatusReady}
	require.NoError(t, store.Create("proj", entry))

	// Initially no task ID
	got, err := store.Get("proj", "clickup-test")
	require.NoError(t, err)
	assert.Equal(t, "", got.ClickUpTaskID, "task ID must be empty before set")

	// Set the task ID
	require.NoError(t, store.SetClickUpTaskID("proj", "clickup-test", "CU-abc123"))

	// Verify it round-trips through Get
	got, err = store.Get("proj", "clickup-test")
	require.NoError(t, err)
	assert.Equal(t, "CU-abc123", got.ClickUpTaskID, "task ID must be persisted after SetClickUpTaskID")

	// Verify it appears in List
	plans, err := store.List("proj")
	require.NoError(t, err)
	require.Len(t, plans, 1)
	assert.Equal(t, "CU-abc123", plans[0].ClickUpTaskID, "task ID must appear in List results")
}

func TestClickUpTaskIDRoundTrip_NotFound(t *testing.T) {
	store := newTestStore(t)
	err := store.SetClickUpTaskID("proj", "nonexistent", "CU-xyz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLinearLinkRoundTrip(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{Filename: "linear-test", Status: taskstore.StatusReady}))

	link := taskstore.LinearLink{
		LinearIssueID:    "issue-123",
		LinearIdentifier: "KAS-123",
		LinearURL:        "https://linear.app/kas/issue/KAS-123",
		LinearTeamKey:    "KAS",
		LinearProjectID:  "project-456",
	}
	require.NoError(t, store.SetLinearLink("proj", "linear-test", link))

	got, err := store.Get("proj", "linear-test")
	require.NoError(t, err)
	assert.Equal(t, link.LinearIssueID, got.LinearIssueID)
	assert.Equal(t, link.LinearIdentifier, got.LinearIdentifier)
	assert.Equal(t, link.LinearURL, got.LinearURL)
	assert.Equal(t, link.LinearTeamKey, got.LinearTeamKey)
	assert.Equal(t, link.LinearProjectID, got.LinearProjectID)

	plans, err := store.List("proj")
	require.NoError(t, err)
	require.Len(t, plans, 1)
	assert.Equal(t, link.LinearIssueID, plans[0].LinearIssueID)

	require.NoError(t, store.Rename("proj", "linear-test", "linear-renamed"))
	renamed, err := store.Get("proj", "linear-renamed")
	require.NoError(t, err)
	assert.Equal(t, link.LinearIssueID, renamed.LinearIssueID)

	require.NoError(t, store.ClearLinearLink("proj", "linear-renamed"))
	cleared, err := store.Get("proj", "linear-renamed")
	require.NoError(t, err)
	assert.Empty(t, cleared.LinearIssueID)
	require.NoError(t, store.ClearLinearLink("proj", "linear-renamed"))

	require.NoError(t, store.SetLinearLink("proj", "linear-renamed", link))
	updated, err := store.Get("proj", "linear-renamed")
	require.NoError(t, err)
	updated.Description = "metadata-only edit"
	require.NoError(t, store.Update("proj", "linear-renamed", updated))
	afterUpdate, err := store.Get("proj", "linear-renamed")
	require.NoError(t, err)
	assert.Equal(t, "metadata-only edit", afterUpdate.Description)
	assert.Equal(t, link.LinearIssueID, afterUpdate.LinearIssueID)
	assert.Equal(t, link.LinearIdentifier, afterUpdate.LinearIdentifier)
}

func TestLinearLink_FindLinkedTask(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "taskstore.db")
	store, err := taskstore.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	require.NoError(t, store.Create("proj", taskstore.TaskEntry{Filename: "active", Status: taskstore.StatusImplementing}))
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{Filename: "done", Status: taskstore.StatusDone}))
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{Filename: "cancelled", Status: taskstore.StatusCancelled}))
	require.NoError(t, store.Create("other", taskstore.TaskEntry{Filename: "active", Status: taskstore.StatusReady}))
	link := taskstore.LinearLink{LinearIssueID: "issue-123", LinearIdentifier: "KAS-123"}
	require.NoError(t, store.SetLinearLink("proj", "active", link))
	require.NoError(t, store.SetLinearLink("proj", "done", link))
	require.NoError(t, store.SetLinearLink("proj", "cancelled", link))
	require.NoError(t, store.SetLinearLink("other", "active", taskstore.LinearLink{LinearIssueID: "issue-123"}))

	filename, err := store.FindLinkedTask("proj", "issue-123", taskstore.StatusReady, taskstore.StatusPlanning, taskstore.StatusImplementing, taskstore.StatusReviewing, taskstore.StatusVerifying)
	require.NoError(t, err)
	assert.Equal(t, "active", filename)

	_, err = store.FindLinkedTask("proj", "missing", taskstore.StatusReady)
	require.Error(t, err)
	assert.True(t, errors.Is(err, taskstore.ErrNotFound))

	filename, err = store.FindLinkedTask("other", "issue-123", taskstore.StatusReady)
	require.NoError(t, err)
	assert.Equal(t, "active", filename)

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var detail string
	err = db.QueryRow(`EXPLAIN QUERY PLAN SELECT filename FROM tasks INDEXED BY idx_tasks_linear_issue_id WHERE project = ? AND linear_issue_id = ? AND status IN (?) ORDER BY filename ASC LIMIT 1`, "proj", "issue-123", "ready").Scan(new(int), new(int), new(int), &detail)
	require.NoError(t, err)
	assert.Contains(t, detail, "idx_tasks_linear_issue_id")
}

func TestLinearLink_SetIfNoActiveDuplicateConcurrent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "taskstore.db")
	storeA, err := taskstore.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storeA.Close() })
	storeB, err := taskstore.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storeB.Close() })

	require.NoError(t, storeA.Create("proj", taskstore.TaskEntry{Filename: "first", Status: taskstore.StatusImplementing}))
	require.NoError(t, storeA.Create("proj", taskstore.TaskEntry{Filename: "second", Status: taskstore.StatusReviewing}))
	link := taskstore.LinearLink{LinearIssueID: "issue-123", LinearIdentifier: "KAS-123"}

	type result struct {
		filename string
		conflict string
		err      error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for filename, store := range map[string]*taskstore.SQLiteStore{"first": storeA, "second": storeB} {
		wg.Add(1)
		go func(filename string, store *taskstore.SQLiteStore) {
			defer wg.Done()
			<-start
			conflict, err := store.SetLinearLinkIfNoActiveDuplicate("proj", filename, link, taskstore.StatusImplementing, taskstore.StatusReviewing)
			results <- result{filename: filename, conflict: conflict, err: err}
		}(filename, store)
	}
	close(start)
	wg.Wait()
	close(results)

	var successes, conflicts int
	for result := range results {
		require.NoError(t, result.err)
		if result.conflict == "" {
			successes++
			continue
		}
		conflicts++
		assert.NotEqual(t, result.filename, result.conflict)
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, conflicts)

	linked, err := storeA.FindLinkedTask("proj", "issue-123", taskstore.StatusImplementing, taskstore.StatusReviewing)
	require.NoError(t, err)
	first, err := storeA.Get("proj", "first")
	require.NoError(t, err)
	second, err := storeA.Get("proj", "second")
	require.NoError(t, err)
	assert.Contains(t, []string{"first", "second"}, linked)
	assert.NotEqual(t, first.LinearIssueID != "", second.LinearIssueID != "", "exactly one task should be linked")
}

func TestLinearLink_NotFound(t *testing.T) {
	store := newTestStore(t)
	err := store.SetLinearLink("proj", "missing", taskstore.LinearLink{LinearIssueID: "issue-123"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, taskstore.ErrNotFound))

	err = store.ClearLinearLink("proj", "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, taskstore.ErrNotFound))
}

func TestSQLiteMigration_PlansTableToTasks(t *testing.T) {
	store, err := taskstore.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	// Store should work — the migration creates the tasks table
	err = store.Create("proj", taskstore.TaskEntry{Filename: "test", Status: taskstore.StatusReady})
	require.NoError(t, err)

	entries, err := store.List("proj")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "test", entries[0].Filename)
}

func TestSQLiteStore_ReviewCycle(t *testing.T) {
	store := newTestStore(t)
	entry := taskstore.TaskEntry{
		Filename: "test",
		Status:   taskstore.StatusReady,
	}
	require.NoError(t, store.Create("proj", entry))

	// Default review cycle is 0.
	got, err := store.Get("proj", "test")
	require.NoError(t, err)
	assert.Equal(t, 0, got.ReviewCycle)

	// Increment and verify.
	require.NoError(t, store.IncrementReviewCycle("proj", "test"))
	got, err = store.Get("proj", "test")
	require.NoError(t, err)
	assert.Equal(t, 1, got.ReviewCycle)

	// Increment again.
	require.NoError(t, store.IncrementReviewCycle("proj", "test"))
	got, err = store.Get("proj", "test")
	require.NoError(t, err)
	assert.Equal(t, 2, got.ReviewCycle)
}

func TestSQLiteStore_SubtaskCRUD(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "kasmos", "plan")

	require.NoError(t, store.SetSubtasks("kasmos", "plan", []taskstore.SubtaskEntry{
		{TaskNumber: 1, Title: "one", Status: taskstore.SubtaskStatusPending},
		{TaskNumber: 2, Title: "two", Status: taskstore.SubtaskStatusPending},
	}))

	got, err := store.GetSubtasks("kasmos", "plan")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, 1, got[0].TaskNumber)
	assert.Equal(t, taskstore.SubtaskStatusPending, got[0].Status)

	require.NoError(t, store.UpdateSubtaskStatus("kasmos", "plan", 1, taskstore.SubtaskStatusClosed))
	updated, err := store.GetSubtasks("kasmos", "plan")
	require.NoError(t, err)
	assert.Equal(t, taskstore.SubtaskStatusClosed, updated[0].Status)

	require.NoError(t, store.SetSubtasks("kasmos", "plan", []taskstore.SubtaskEntry{
		{TaskNumber: 2, Title: "replacement", Status: taskstore.SubtaskStatusDone},
	}))
	replaced, err := store.GetSubtasks("kasmos", "plan")
	require.NoError(t, err)
	assert.Len(t, replaced, 1)
	assert.Equal(t, "replacement", replaced[0].Title)
	assert.Equal(t, taskstore.SubtaskStatusDone, replaced[0].Status)
}

func TestSQLiteStore_PhaseTimestamps(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "kasmos", "plan")

	require.NoError(t, store.SetPhaseTimestamp("kasmos", "plan", "planning", time.Now().UTC()))
	require.NoError(t, store.SetPhaseTimestamp("kasmos", "plan", "implementing", time.Now().UTC()))
	require.NoError(t, store.SetPhaseTimestamp("kasmos", "plan", "reviewing", time.Now().UTC()))
	require.NoError(t, store.SetPhaseTimestamp("kasmos", "plan", "verifying", time.Now().UTC()))
	require.NoError(t, store.SetPhaseTimestamp("kasmos", "plan", "done", time.Now().UTC()))

	got, err := store.Get("kasmos", "plan")
	require.NoError(t, err)
	assert.False(t, got.PlanningAt.IsZero())
	assert.False(t, got.ImplementingAt.IsZero())
	assert.False(t, got.ReviewingAt.IsZero())
	assert.False(t, got.VerifyingAt.IsZero())
	assert.False(t, got.DoneAt.IsZero())

	err = store.SetPhaseTimestamp("kasmos", "plan", "unknown", time.Now().UTC())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown phase")
}

func TestSQLiteStore_VerifyingAtRoundTrip(t *testing.T) {
	store := newTestStore(t)
	ts := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	entry := taskstore.TaskEntry{
		Filename:    "verifying-plan",
		Status:      taskstore.StatusVerifying,
		VerifyingAt: ts,
	}
	require.NoError(t, store.Create("proj", entry))

	got, err := store.Get("proj", "verifying-plan")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusVerifying, got.Status)
	assert.Equal(t, ts, got.VerifyingAt, "verifying_at must survive Create+Get round-trip")

	// Update and verify it persists.
	ts2 := ts.Add(time.Hour)
	got.VerifyingAt = ts2
	require.NoError(t, store.Update("proj", "verifying-plan", got))

	got2, err := store.Get("proj", "verifying-plan")
	require.NoError(t, err)
	assert.Equal(t, ts2, got2.VerifyingAt, "verifying_at must survive Update round-trip")

	// Verify it appears in List.
	list, err := store.List("proj")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, ts2, list[0].VerifyingAt, "verifying_at must appear in List results")

	// Verify it appears in ListByStatus.
	byStatus, err := store.ListByStatus("proj", taskstore.StatusVerifying)
	require.NoError(t, err)
	require.Len(t, byStatus, 1)
	assert.Equal(t, ts2, byStatus[0].VerifyingAt)

	// Verify SetPhaseTimestamp works for verifying.
	ts3 := ts.Add(2 * time.Hour)
	require.NoError(t, store.SetPhaseTimestamp("proj", "verifying-plan", "verifying", ts3))
	got3, err := store.Get("proj", "verifying-plan")
	require.NoError(t, err)
	assert.Equal(t, ts3, got3.VerifyingAt, "SetPhaseTimestamp('verifying') must update verifying_at")
}

func TestSQLiteStore_PlanGoal(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "kasmos", "plan")

	require.NoError(t, store.SetPlanGoal("kasmos", "plan", "ship resilient workflow"))

	got, err := store.Get("kasmos", "plan")
	require.NoError(t, err)
	assert.Equal(t, "ship resilient workflow", got.Goal)
}

func TestSQLiteStore_PRMetadata(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	project := "test"
	createReadyTask(t, store, project, "plan")

	require.NoError(t, store.SetPRURL(project, "plan", "https://github.com/org/repo/pull/42"))
	require.NoError(t, store.SetPRState(project, "plan", "APPROVED", "SUCCESS"))

	entry, err := store.Get(project, "plan")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/org/repo/pull/42", entry.PRURL)
	assert.Equal(t, "APPROVED", entry.PRReviewDecision)
	assert.Equal(t, "SUCCESS", entry.PRCheckStatus)
}

func TestSQLiteStore_PRMetadata_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.SetPRURL("test", "nonexistent", "https://github.com/org/repo/pull/42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	err = store.SetPRState("test", "nonexistent", "APPROVED", "SUCCESS")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSQLiteStore_PRReviews_RecordAndList(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "proj", "plan")

	// Record two reviews
	require.NoError(t, store.RecordPRReview("proj", "plan", 101, "CHANGES_REQUESTED", "fix this", "reviewer1"))
	require.NoError(t, store.RecordPRReview("proj", "plan", 102, "COMMENTED", "nit: rename", "reviewer2"))

	pending, err := store.ListPendingReviews("proj", "plan")
	require.NoError(t, err)
	assert.Len(t, pending, 2)
	assert.Equal(t, 101, pending[0].ReviewID)
	assert.Equal(t, "CHANGES_REQUESTED", pending[0].ReviewState)
	assert.Equal(t, "fix this", pending[0].ReviewBody)
	assert.Equal(t, "reviewer1", pending[0].ReviewerLogin)
	assert.False(t, pending[0].ReactionPosted)
	assert.False(t, pending[0].FixerDispatched)
	assert.False(t, pending[0].CreatedAt.IsZero())
}

func TestSQLiteStore_PRReviews_DuplicateInsertIdempotent(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "proj", "plan")

	// Insert same review ID twice — second call must be a no-op
	require.NoError(t, store.RecordPRReview("proj", "plan", 42, "APPROVED", "lgtm", "alice"))
	require.NoError(t, store.RecordPRReview("proj", "plan", 42, "CHANGES_REQUESTED", "should error but won't", "bob"))

	// Only one row should exist, with the original data
	pending, err := store.ListPendingReviews("proj", "plan")
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, "APPROVED", pending[0].ReviewState, "first record must win")
	assert.Equal(t, "alice", pending[0].ReviewerLogin, "first reviewer must win")
}

func TestSQLiteStore_PRReviews_IsReviewProcessed(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "proj", "plan")

	// Not recorded yet
	assert.False(t, store.IsReviewProcessed("proj", "plan", 99))

	require.NoError(t, store.RecordPRReview("proj", "plan", 99, "COMMENTED", "looks good", "reviewer"))

	// Now recorded
	assert.True(t, store.IsReviewProcessed("proj", "plan", 99))
}

func TestSQLiteStore_PRReviews_MarkReacted(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "proj", "plan")
	require.NoError(t, store.RecordPRReview("proj", "plan", 10, "COMMENTED", "body", "reviewer"))

	require.NoError(t, store.MarkReviewReacted("proj", "plan", 10))

	pending, err := store.ListPendingReviews("proj", "plan")
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.True(t, pending[0].ReactionPosted, "reaction_posted must be true after MarkReviewReacted")

	// Not found error
	err = store.MarkReviewReacted("proj", "plan", 9999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSQLiteStore_PRReviews_MarkFixerDispatched(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "proj", "plan")
	require.NoError(t, store.RecordPRReview("proj", "plan", 20, "CHANGES_REQUESTED", "fix it", "reviewer"))

	require.NoError(t, store.MarkReviewFixerDispatched("proj", "plan", 20))

	// After marking fixer dispatched, the review should no longer appear in pending list
	pending, err := store.ListPendingReviews("proj", "plan")
	require.NoError(t, err)
	assert.Len(t, pending, 0, "fixer-dispatched reviews must not appear in pending list")

	// Not found error
	err = store.MarkReviewFixerDispatched("proj", "plan", 9999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSQLiteStore_PRReviews_EmptyPendingList(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "proj", "plan")

	pending, err := store.ListPendingReviews("proj", "plan")
	require.NoError(t, err)
	// Must return empty slice, not nil
	assert.NotNil(t, pending)
	assert.Len(t, pending, 0)
}

func TestSQLiteStore_PRReviews_OrderedByReviewID(t *testing.T) {
	store := newTestStore(t)
	createReadyTask(t, store, "proj", "plan")

	// Insert in non-sequential order
	require.NoError(t, store.RecordPRReview("proj", "plan", 300, "COMMENTED", "c", "r3"))
	require.NoError(t, store.RecordPRReview("proj", "plan", 100, "COMMENTED", "a", "r1"))
	require.NoError(t, store.RecordPRReview("proj", "plan", 200, "COMMENTED", "b", "r2"))

	pending, err := store.ListPendingReviews("proj", "plan")
	require.NoError(t, err)
	require.Len(t, pending, 3)
	assert.Equal(t, 100, pending[0].ReviewID)
	assert.Equal(t, 200, pending[1].ReviewID)
	assert.Equal(t, 300, pending[2].ReviewID)
}

func TestSQLiteStore_EmptySlices(t *testing.T) {
	store := newTestStore(t)

	// Seed one task so Get-dependent queries work.
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: "plan",
		Status:   taskstore.StatusReady,
	}))

	t.Run("List returns non-nil empty slice", func(t *testing.T) {
		got, err := store.List("empty-project")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("ListByTopic returns non-nil empty slice", func(t *testing.T) {
		got, err := store.ListByTopic("proj", "missing-topic")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("ListByStatus with matches returns non-nil empty slice", func(t *testing.T) {
		got, err := store.ListByStatus("proj", taskstore.StatusDone)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("ListByStatus zero-arg returns non-nil empty slice", func(t *testing.T) {
		got, err := store.ListByStatus("proj")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("GetSubtasks returns non-nil empty slice", func(t *testing.T) {
		got, err := store.GetSubtasks("proj", "plan")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("ListTopics returns non-nil empty slice", func(t *testing.T) {
		got, err := store.ListTopics("proj")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Empty(t, got)
	})
}
