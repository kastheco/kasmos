package auditlog_test

import (
	"encoding/json"
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

func TestSQLite_LinearReceiptEventKinds(t *testing.T) {
	logger, err := auditlog.NewSQLiteLogger(":memory:")
	require.NoError(t, err)
	defer logger.Close()

	for _, kind := range []auditlog.EventKind{
		auditlog.EventTaskLinearReceiptPosted,
		auditlog.EventTaskLinearReceiptFailed,
	} {
		logger.Emit(auditlog.Event{
			Kind:    kind,
			Project: "p",
			Message: kind.String(),
		})
	}

	events, err := logger.Query(auditlog.QueryFilter{
		Project: "p",
		Kinds: []auditlog.EventKind{
			auditlog.EventTaskLinearReceiptPosted,
			auditlog.EventTaskLinearReceiptFailed,
		},
		Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, events, 2)

	byKind := make(map[auditlog.EventKind]auditlog.Event, len(events))
	for _, event := range events {
		byKind[event.Kind] = event
	}
	assert.Equal(t, auditlog.EventTaskLinearReceiptPosted.String(), byKind[auditlog.EventTaskLinearReceiptPosted].Message)
	assert.Equal(t, auditlog.EventTaskLinearReceiptFailed.String(), byKind[auditlog.EventTaskLinearReceiptFailed].Message)
}

func TestSQLite_LinearTriggerEventKinds(t *testing.T) {
	logger, err := auditlog.NewSQLiteLogger(":memory:")
	require.NoError(t, err)
	defer logger.Close()

	kinds := []auditlog.EventKind{
		auditlog.EventTaskLinearTriggerReceived,
		auditlog.EventTaskLinearTriggerDispatched,
		auditlog.EventTaskLinearTriggerRejected,
		auditlog.EventTaskLinearTriggerIgnored,
		auditlog.EventTaskLinearTriggerCommentFailed,
	}
	for _, kind := range kinds {
		detail := linearTriggerDetail(t, kind)
		logger.Emit(auditlog.Event{
			Kind:    kind,
			Project: "p",
			Message: kind.String(),
			Detail:  detail,
		})
	}

	events, err := logger.Query(auditlog.QueryFilter{
		Project: "p",
		Kinds:   kinds,
		Limit:   10,
	})
	require.NoError(t, err)
	require.Len(t, events, len(kinds))

	byKind := make(map[auditlog.EventKind]auditlog.Event, len(events))
	for _, event := range events {
		byKind[event.Kind] = event
	}
	for _, kind := range kinds {
		event, ok := byKind[kind]
		require.True(t, ok, "missing event kind %s", kind)
		assert.Equal(t, kind.String(), event.Message)

		var detail map[string]any
		require.NoError(t, json.Unmarshal([]byte(event.Detail), &detail))
		assert.Equal(t, "plan", detail["command_kind"])
		assert.Equal(t, "comment", detail["source_kind"])
		assert.Equal(t, "comment-uuid", detail["source_id"])
		assert.Equal(t, "ENG-123", detail["linear_identifier"])
		assert.Equal(t, "https://linear.app/acme/issue/ENG-123/title", detail["linear_url"])
		assert.Equal(t, "actor-uuid", detail["actor_id"])
		assert.Equal(t, "operator@example.com", detail["actor_email"])
		if kind == auditlog.EventTaskLinearTriggerRejected {
			assert.Equal(t, "actor_not_allowed", detail["reason"])
		} else {
			assert.NotContains(t, detail, "reason")
		}
		if kind == auditlog.EventTaskLinearTriggerCommentFailed {
			assert.Equal(t, "reaction unsupported", detail["error"])
		} else {
			assert.NotContains(t, detail, "error")
		}
	}
}

func TestSQLite_LinearLinkEventKinds(t *testing.T) {
	logger, err := auditlog.NewSQLiteLogger(":memory:")
	require.NoError(t, err)
	defer logger.Close()

	linked := auditlog.Event{
		Kind:    auditlog.EventTaskLinearLinked,
		Project: "p",
		Detail:  `{"kept":true}`,
	}
	auditlog.WithLinearLink("", "LIN-123", "operator requested")(&linked)
	logger.Emit(linked)

	unlinked := auditlog.Event{
		Kind:    auditlog.EventTaskLinearUnlinked,
		Project: "p",
		Detail:  `{"kept":true}`,
	}
	auditlog.WithLinearLink("LIN-123", "", "")(&unlinked)
	logger.Emit(unlinked)

	events, err := logger.Query(auditlog.QueryFilter{
		Project: "p",
		Kinds: []auditlog.EventKind{
			auditlog.EventTaskLinearLinked,
			auditlog.EventTaskLinearUnlinked,
		},
		Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, events, 2)

	kinds := []auditlog.EventKind{events[0].Kind, events[1].Kind}
	assert.Contains(t, kinds, auditlog.EventTaskLinearLinked)
	assert.Contains(t, kinds, auditlog.EventTaskLinearUnlinked)

	byKind := make(map[auditlog.EventKind]auditlog.Event, len(events))
	for _, event := range events {
		byKind[event.Kind] = event
	}

	var linkedDetail map[string]any
	require.NoError(t, json.Unmarshal([]byte(byKind[auditlog.EventTaskLinearLinked].Detail), &linkedDetail))
	assert.Equal(t, true, linkedDetail["kept"])
	assert.Equal(t, "LIN-123", linkedDetail["new_identifier"])
	assert.Equal(t, "operator requested", linkedDetail["reason"])
	assert.NotContains(t, linkedDetail, "previous_identifier")

	var unlinkedDetail map[string]any
	require.NoError(t, json.Unmarshal([]byte(byKind[auditlog.EventTaskLinearUnlinked].Detail), &unlinkedDetail))
	assert.Equal(t, true, unlinkedDetail["kept"])
	assert.Equal(t, "LIN-123", unlinkedDetail["previous_identifier"])
	assert.NotContains(t, unlinkedDetail, "new_identifier")
	assert.NotContains(t, unlinkedDetail, "reason")
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

func linearTriggerDetail(t *testing.T, kind auditlog.EventKind) string {
	t.Helper()

	detail := map[string]string{
		"command_kind":      "plan",
		"source_kind":       "comment",
		"source_id":         "comment-uuid",
		"linear_identifier": "ENG-123",
		"linear_url":        "https://linear.app/acme/issue/ENG-123/title",
		"actor_id":          "actor-uuid",
		"actor_email":       "operator@example.com",
	}
	switch kind {
	case auditlog.EventTaskLinearTriggerRejected:
		detail["reason"] = "actor_not_allowed"
	case auditlog.EventTaskLinearTriggerCommentFailed:
		detail["error"] = "reaction unsupported"
	}

	encoded, err := json.Marshal(detail)
	require.NoError(t, err)
	return string(encoded)
}
