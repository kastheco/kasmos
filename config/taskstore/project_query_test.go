package taskstore_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestListDistinctProjectsFromDB_SortsAndDeduplicates(t *testing.T) {
	dir := t.TempDir()
	db, err := taskstore.OpenSharedDB(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)

	// Insert tasks in non-alphabetical order with duplicates.
	require.NoError(t, store.Create("zebra", taskstore.TaskEntry{Filename: "t1", Status: taskstore.StatusReady}))
	require.NoError(t, store.Create("alpha", taskstore.TaskEntry{Filename: "t2", Status: taskstore.StatusReady}))
	require.NoError(t, store.Create("zebra", taskstore.TaskEntry{Filename: "t3", Status: taskstore.StatusReady}))

	got, err := taskstore.ListDistinctProjectsFromDB(db)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "zebra"}, got)
}

func TestListDistinctProjectsFromDB_DeduplicatesAcrossTablesAndTrims(t *testing.T) {
	dir := t.TempDir()
	db, err := taskstore.OpenSharedDB(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)
	gw, err := taskstore.NewSQLiteSignalGatewayFromDB(db)
	require.NoError(t, err)

	// Same project name appears in both tasks and signals.
	require.NoError(t, store.Create("shared", taskstore.TaskEntry{Filename: "t1", Status: taskstore.StatusReady}))
	require.NoError(t, gw.Create("shared", taskstore.SignalEntry{PlanFile: "plan", SignalType: "planner_finished", Payload: "{}"}))
	// Signal-only project.
	require.NoError(t, gw.Create("signals-only", taskstore.SignalEntry{PlanFile: "plan2", SignalType: "implement_finished", Payload: "{}"}))

	got, err := taskstore.ListDistinctProjectsFromDB(db)
	require.NoError(t, err)
	assert.Equal(t, []string{"shared", "signals-only"}, got)
}

func TestListDistinctProjectsFromDB_EmptyDB(t *testing.T) {
	dir := t.TempDir()
	db, err := taskstore.OpenSharedDB(filepath.Join(dir, "empty.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Initialize both schemas so tables exist but are empty.
	_, err = taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)
	_, err = taskstore.NewSQLiteSignalGatewayFromDB(db)
	require.NoError(t, err)

	got, err := taskstore.ListDistinctProjectsFromDB(db)
	require.NoError(t, err)
	// Expect an empty non-nil slice (never null).
	require.NotNil(t, got)
	assert.Empty(t, got)
}

func TestListDistinctProjectsFromDB_MissingTablesNoError(t *testing.T) {
	// A raw DB with no schema at all — neither tasks nor signals table exists.
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	got, err := taskstore.ListDistinctProjectsFromDB(db)
	require.NoError(t, err, "missing tables must not cause an error")
	assert.Empty(t, got)
}

func TestListDistinctProjectsFromDB_MissingSignalsTable(t *testing.T) {
	dir := t.TempDir()
	db, err := taskstore.OpenSharedDB(filepath.Join(dir, "notasks.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Only initialise task store — signals table is absent.
	store, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)
	require.NoError(t, store.Create("taskonly", taskstore.TaskEntry{Filename: "t1", Status: taskstore.StatusReady}))

	got, err := taskstore.ListDistinctProjectsFromDB(db)
	require.NoError(t, err, "missing signals table must not cause an error")
	assert.Equal(t, []string{"taskonly"}, got)
}

func TestListDistinctProjectsFromDB_MissingTasksTable(t *testing.T) {
	dir := t.TempDir()
	db, err := taskstore.OpenSharedDB(filepath.Join(dir, "nosignals.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Only initialise signal gateway — tasks table is absent.
	gw, err := taskstore.NewSQLiteSignalGatewayFromDB(db)
	require.NoError(t, err)
	require.NoError(t, gw.Create("sigonly", taskstore.SignalEntry{PlanFile: "plan", SignalType: "planner_finished", Payload: "{}"}))

	got, err := taskstore.ListDistinctProjectsFromDB(db)
	require.NoError(t, err, "missing tasks table must not cause an error")
	assert.Equal(t, []string{"sigonly"}, got)
}
