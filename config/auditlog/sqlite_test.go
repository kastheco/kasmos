package auditlog_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteLogger_EmitAndQuery(t *testing.T) {
	logger, err := auditlog.NewSQLiteLogger(":memory:")
	require.NoError(t, err)
	defer logger.Close()

	logger.Emit(auditlog.Event{
		Kind:          auditlog.EventAgentSpawned,
		Project:       "testproj",
		TaskFile:      "plan.md",
		InstanceTitle: "plan-coder",
		AgentType:     "coder",
		Message:       "spawned coder agent",
	})

	events, err := logger.Query(auditlog.QueryFilter{Project: "testproj", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, auditlog.EventAgentSpawned, events[0].Kind)
	assert.Equal(t, "plan-coder", events[0].InstanceTitle)
	assert.False(t, events[0].Timestamp.IsZero())
}

func TestSQLiteLogger_QueryFilterByPlan(t *testing.T) {
	logger, err := auditlog.NewSQLiteLogger(":memory:")
	require.NoError(t, err)
	defer logger.Close()

	logger.Emit(auditlog.Event{Kind: auditlog.EventAgentSpawned, Project: "p", TaskFile: "a.md"})
	logger.Emit(auditlog.Event{Kind: auditlog.EventAgentSpawned, Project: "p", TaskFile: "b.md"})

	events, err := logger.Query(auditlog.QueryFilter{Project: "p", TaskFile: "a.md", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, events, 1)
}

func TestSQLiteLogger_QueryFilterByKind(t *testing.T) {
	logger, err := auditlog.NewSQLiteLogger(":memory:")
	require.NoError(t, err)
	defer logger.Close()

	logger.Emit(auditlog.Event{Kind: auditlog.EventAgentSpawned, Project: "p"})
	logger.Emit(auditlog.Event{Kind: auditlog.EventPlanTransition, Project: "p"})

	events, err := logger.Query(auditlog.QueryFilter{
		Project: "p",
		Kinds:   []auditlog.EventKind{auditlog.EventPlanTransition},
		Limit:   10,
	})
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, auditlog.EventPlanTransition, events[0].Kind)
}

func TestSQLiteLogger_QueryOrderDesc(t *testing.T) {
	logger, err := auditlog.NewSQLiteLogger(":memory:")
	require.NoError(t, err)
	defer logger.Close()

	logger.Emit(auditlog.Event{Kind: auditlog.EventAgentSpawned, Project: "p", Message: "first"})
	time.Sleep(time.Millisecond)
	logger.Emit(auditlog.Event{Kind: auditlog.EventAgentFinished, Project: "p", Message: "second"})

	events, err := logger.Query(auditlog.QueryFilter{Project: "p", Limit: 10})
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "second", events[0].Message) // newest first
}

func TestSQLiteLogger_SharedDB(t *testing.T) {
	// Verify the logger can be opened on the same DB path as planstore
	// (separate table, no conflicts)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "store.db")

	store, err := taskstore.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	logger, err := auditlog.NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	logger.Emit(auditlog.Event{Kind: auditlog.EventAgentSpawned, Project: "p", Message: "test"})
	events, err := logger.Query(auditlog.QueryFilter{Project: "p", Limit: 1})
	require.NoError(t, err)
	assert.Len(t, events, 1)
}

// ---------------------------------------------------------------------------
// FromDB constructor tests
// ---------------------------------------------------------------------------

func TestSQLiteLoggerFromDB_BasicOps(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "shared.db")

	db, err := taskstore.OpenSharedDB(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	logger, err := auditlog.NewSQLiteLoggerFromDB(db)
	require.NoError(t, err)

	logger.Emit(auditlog.Event{
		Kind:          auditlog.EventAgentSpawned,
		Project:       "proj",
		InstanceTitle: "plan-coder",
		Message:       "from-db test",
	})

	events, err := logger.Query(auditlog.QueryFilter{Project: "proj", Limit: 10})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, auditlog.EventAgentSpawned, events[0].Kind)
	assert.Equal(t, "from-db test", events[0].Message)
	assert.Equal(t, "plan-coder", events[0].InstanceTitle)
	assert.False(t, events[0].Timestamp.IsZero())
}

func TestSQLiteLoggerFromDB_CloseIsNoOp(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "shared.db")

	db, err := taskstore.OpenSharedDB(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	logger, err := auditlog.NewSQLiteLoggerFromDB(db)
	require.NoError(t, err)

	// Close must be a no-op — it must not close the shared *sql.DB.
	require.NoError(t, logger.Close())

	// The underlying pool must still be alive.
	require.NoError(t, db.Ping())

	// A second logger on the same DB must still work.
	logger2, err := auditlog.NewSQLiteLoggerFromDB(db)
	require.NoError(t, err)
	logger2.Emit(auditlog.Event{Kind: auditlog.EventAgentFinished, Project: "proj", Message: "still alive"})
	events, err := logger2.Query(auditlog.QueryFilter{Project: "proj", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "still alive", events[0].Message)
}

func TestSQLiteLoggerFromDB_SharedPoolWithStore(t *testing.T) {
	// Verify that logger and store can share the same *sql.DB without conflict
	// (each operates on separate tables).
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "shared.db")

	db, err := taskstore.OpenSharedDB(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)

	logger, err := auditlog.NewSQLiteLoggerFromDB(db)
	require.NoError(t, err)

	// Interleave store and logger operations.
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{Filename: "shared-task", Status: taskstore.StatusReady}))
	logger.Emit(auditlog.Event{Kind: auditlog.EventAgentSpawned, Project: "proj", TaskFile: "shared-task", Message: "spawned"})
	require.NoError(t, store.Update("proj", "shared-task", taskstore.TaskEntry{Filename: "shared-task", Status: taskstore.StatusImplementing}))
	logger.Emit(auditlog.Event{Kind: auditlog.EventPlanTransition, Project: "proj", TaskFile: "shared-task", Message: "implementing"})

	// Both tables must reflect the writes.
	got, err := store.Get("proj", "shared-task")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, got.Status)

	events, err := logger.Query(auditlog.QueryFilter{Project: "proj", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, events, 2)
}
