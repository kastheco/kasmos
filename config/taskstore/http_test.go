package taskstore_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestHTTPStore creates an HTTPStore backed by an in-memory SQLiteStore
// served over a local httptest.Server. The server is closed when the test ends.
func newTestHTTPStore(t *testing.T) *taskstore.HTTPStore {
	t.Helper()
	backend := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	t.Cleanup(srv.Close)
	return taskstore.NewHTTPStore(srv.URL, "kasmos")
}

func TestHTTPStore_ContentRoundTrip(t *testing.T) {
	store := newTestHTTPStore(t)
	entry := taskstore.TaskEntry{
		Filename: "test",
		Status:   taskstore.StatusReady,
		Content:  "# My Plan\n\nDetails here.",
	}
	require.NoError(t, store.Create("proj", entry))

	content, err := store.GetContent("proj", "test")
	require.NoError(t, err)
	assert.Equal(t, "# My Plan\n\nDetails here.", content)

	require.NoError(t, store.SetContent("proj", "test", "# Updated Plan"))
	content, err = store.GetContent("proj", "test")
	require.NoError(t, err)
	assert.Equal(t, "# Updated Plan", content)
}

func TestHTTPStore_RoundTrip(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()

	client := taskstore.NewHTTPStore(srv.URL, "kasmos")

	// Create
	entry := taskstore.TaskEntry{Filename: "test", Status: taskstore.StatusReady, Description: "test"}
	require.NoError(t, client.Create("kasmos", entry))

	// Get
	got, err := client.Get("kasmos", "test")
	require.NoError(t, err)
	assert.Equal(t, "test", got.Description)

	// Update
	got.Status = taskstore.StatusImplementing
	require.NoError(t, client.Update("kasmos", "test", got))

	// List
	plans, err := client.List("kasmos")
	require.NoError(t, err)
	assert.Len(t, plans, 1)
	assert.Equal(t, taskstore.StatusImplementing, plans[0].Status)
}

func TestHTTPStore_Delete(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()

	client := taskstore.NewHTTPStore(srv.URL, "kasmos")

	require.NoError(t, client.Create("kasmos", taskstore.TaskEntry{Filename: "delete-me", Status: taskstore.StatusReady}))
	require.NoError(t, client.Delete("kasmos", "delete-me"))

	_, err := client.Get("kasmos", "delete-me")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	err = client.Delete("kasmos", "delete-me")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	err = client.Delete("kasmos", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHTTPStore_ServerUnreachable(t *testing.T) {
	client := taskstore.NewHTTPStore("http://127.0.0.1:1", "kasmos")
	_, err := client.List("kasmos")
	require.Error(t, err)
	// Error should be recognizable as a connectivity issue
	assert.Contains(t, err.Error(), "task store unreachable")
}

func TestHTTPStore_Ping(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()

	client := taskstore.NewHTTPStore(srv.URL, "kasmos")
	require.NoError(t, client.Ping())
}

func TestHTTPStore_SubtasksRoundTrip(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()
	client := taskstore.NewHTTPStore(srv.URL, "kasmos")

	require.NoError(t, client.Create("kasmos", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))

	require.NoError(t, client.SetSubtasks("kasmos", "plan", []taskstore.SubtaskEntry{
		{TaskNumber: 1, Title: "first", Status: taskstore.SubtaskStatusPending},
		{TaskNumber: 2, Title: "second", Status: taskstore.SubtaskStatusPending},
	}))

	subtasks, err := client.GetSubtasks("kasmos", "plan")
	require.NoError(t, err)
	assert.Equal(t, 2, len(subtasks))
	assert.Equal(t, 1, subtasks[0].TaskNumber)
	assert.Equal(t, taskstore.SubtaskStatusPending, subtasks[0].Status)

	require.NoError(t, client.UpdateSubtaskStatus("kasmos", "plan", 1, taskstore.SubtaskStatusDone))
	updated, err := client.GetSubtasks("kasmos", "plan")
	require.NoError(t, err)
	assert.Equal(t, taskstore.SubtaskStatusDone, updated[0].Status)

	require.NoError(t, client.SetSubtasks("kasmos", "plan", nil))
	empty, err := client.GetSubtasks("kasmos", "plan")
	require.NoError(t, err)
	assert.Len(t, empty, 0)
}

func TestHTTPStore_PhaseTimestamp(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()
	client := taskstore.NewHTTPStore(srv.URL, "kasmos")

	require.NoError(t, client.Create("kasmos", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))

	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, client.SetPhaseTimestamp("kasmos", "plan", "planning", ts))

	got, err := backend.Get("kasmos", "plan")
	require.NoError(t, err)
	assert.Equal(t, ts, got.PlanningAt)
}

func TestHTTPStore_ClearExecutionStatePreservesCancelledStatus(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()
	client := taskstore.NewHTTPStore(srv.URL, "proj")

	require.NoError(t, client.Create("proj", taskstore.TaskEntry{
		Filename:       "cancel-me",
		Status:         taskstore.StatusReady,
		ExecutionState: taskstore.ExecutionState{Phase: "planned"},
	}))

	ps, err := taskstate.Load(client, "proj", t.TempDir())
	require.NoError(t, err)

	fsm := taskfsm.New(client, "proj", t.TempDir())
	require.NoError(t, fsm.Transition("cancel-me", taskfsm.Cancel))
	require.NoError(t, ps.ClearExecutionState("cancel-me"))

	got, err := client.Get("proj", "cancel-me")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusCancelled, got.Status)
	assert.Equal(t, taskstore.ExecutionState{}, got.ExecutionState)
}

func TestHTTPStore_PlanGoal(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()
	client := taskstore.NewHTTPStore(srv.URL, "kasmos")

	require.NoError(t, client.Create("kasmos", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))

	require.NoError(t, client.SetPlanGoal("kasmos", "plan", "ship faster"))
	got, err := backend.Get("kasmos", "plan")
	require.NoError(t, err)
	assert.Equal(t, "ship faster", got.Goal)
}

func TestHTTPStore_SetPhaseTimestamp_UsesJSONErrorContractOnMalformedBody(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()

	resp, err := http.DefaultClient.Do(func() *http.Request {
		req, rErr := http.NewRequest(http.MethodPut, srv.URL+"/v1/projects/kasmos/tasks/plan/phase-timestamp", strings.NewReader("{"))
		require.NoError(t, rErr)
		req.Header.Set("Content-Type", "application/json")
		return req
	}())
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var errResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	_, has := errResp["error"]
	assert.True(t, has)
}

func TestHTTPStore_PRReviews_RoundTrip(t *testing.T) {
	backend, _ := taskstore.NewSQLiteStore(":memory:")
	t.Cleanup(func() { backend.Close() })
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	t.Cleanup(srv.Close)
	client := taskstore.NewHTTPStore(srv.URL, "proj")

	// Create a task first
	require.NoError(t, client.Create("proj", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))

	// Not processed before recording
	assert.False(t, client.IsReviewProcessed("proj", "plan", 101))

	// Record a review
	require.NoError(t, client.RecordPRReview("proj", "plan", 101, "CHANGES_REQUESTED", "fix this", "alice"))

	// Now processed
	assert.True(t, client.IsReviewProcessed("proj", "plan", 101))

	// List pending — should have one entry
	pending, err := client.ListPendingReviews("proj", "plan")
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, 101, pending[0].ReviewID)
	assert.Equal(t, "CHANGES_REQUESTED", pending[0].ReviewState)
	assert.Equal(t, "fix this", pending[0].ReviewBody)
	assert.Equal(t, "alice", pending[0].ReviewerLogin)
	assert.False(t, pending[0].ReactionPosted)
	assert.False(t, pending[0].FixerDispatched)

	// Mark as reacted
	require.NoError(t, client.MarkReviewReacted("proj", "plan", 101))

	// Still pending (fixer not dispatched)
	pending, err = client.ListPendingReviews("proj", "plan")
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.True(t, pending[0].ReactionPosted)

	// Mark fixer dispatched — disappears from pending list
	require.NoError(t, client.MarkReviewFixerDispatched("proj", "plan", 101))
	pending, err = client.ListPendingReviews("proj", "plan")
	require.NoError(t, err)
	assert.Len(t, pending, 0)
}

func TestHTTPStore_PRReviews_DuplicateIdempotent(t *testing.T) {
	backend, _ := taskstore.NewSQLiteStore(":memory:")
	t.Cleanup(func() { backend.Close() })
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	t.Cleanup(srv.Close)
	client := taskstore.NewHTTPStore(srv.URL, "proj")

	require.NoError(t, client.Create("proj", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))

	require.NoError(t, client.RecordPRReview("proj", "plan", 42, "APPROVED", "lgtm", "alice"))
	// Second call with same ID must succeed (idempotent)
	require.NoError(t, client.RecordPRReview("proj", "plan", 42, "CHANGES_REQUESTED", "nope", "bob"))

	// Only one row exists
	pending, err := client.ListPendingReviews("proj", "plan")
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, "APPROVED", pending[0].ReviewState, "first record must win")
}

func TestHTTPStore_PRReviews_EmptyPendingList(t *testing.T) {
	backend, _ := taskstore.NewSQLiteStore(":memory:")
	t.Cleanup(func() { backend.Close() })
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	t.Cleanup(srv.Close)
	client := taskstore.NewHTTPStore(srv.URL, "proj")

	require.NoError(t, client.Create("proj", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))

	pending, err := client.ListPendingReviews("proj", "plan")
	require.NoError(t, err)
	assert.NotNil(t, pending)
	assert.Len(t, pending, 0)
}

func TestHTTPStore_PRReviews_MarkNotFound(t *testing.T) {
	backend, _ := taskstore.NewSQLiteStore(":memory:")
	t.Cleanup(func() { backend.Close() })
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	t.Cleanup(srv.Close)
	client := taskstore.NewHTTPStore(srv.URL, "proj")

	require.NoError(t, client.Create("proj", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))

	err := client.MarkReviewReacted("proj", "plan", 9999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	err = client.MarkReviewFixerDispatched("proj", "plan", 9999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHTTPStore_LinearTriggers_RoundTrip(t *testing.T) {
	backend, err := taskstore.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backend.Close()) })
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	t.Cleanup(srv.Close)

	client := taskstore.NewHTTPStore(srv.URL, "proj")
	detectedAt := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	entry := taskstore.LinearTriggerEntry{
		LinearIssueID:    "lin-1",
		LinearIdentifier: "KAS-1",
		CommandKind:      "plan",
		SourceKind:       "comment",
		SourceID:         "comment-1",
		ActorID:          "actor-1",
		ActorEmail:       "agent@example.com",
		TaskArg:          "task-a",
		DetectedAt:       detectedAt,
	}

	queued, err := client.EnqueueLinearTrigger("proj", entry)
	require.NoError(t, err)
	assert.True(t, queued)
	queued, err = client.EnqueueLinearTrigger("proj", entry)
	require.NoError(t, err)
	assert.False(t, queued)

	triggers, err := client.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	require.Len(t, triggers, 1)
	assert.Equal(t, "lin-1", triggers[0].LinearIssueID)
	assert.Equal(t, detectedAt, triggers[0].DetectedAt)

	require.NoError(t, client.MarkLinearTriggerAck("proj", triggers[0].ID, "ack_failed"))
	require.NoError(t, client.MarkLinearTriggerRejected("proj", triggers[0].ID, "not allowed"))
	triggers, err = client.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	assert.Empty(t, triggers)

	for _, tc := range []struct {
		name string
		mark func(int64) error
	}{
		{name: "dispatched", mark: func(id int64) error { return client.MarkLinearTriggerDispatched("proj", id, "task-a") }},
		{name: "ignored", mark: func(id int64) error { return client.MarkLinearTriggerIgnored("proj", id, "duplicate") }},
		{name: "failed", mark: func(id int64) error { return client.MarkLinearTriggerFailed("proj", id, "boom") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := entry
			e.SourceID = tc.name
			queued, err := client.EnqueueLinearTrigger("proj", e)
			require.NoError(t, err)
			require.True(t, queued)
			pending, err := client.ListUnprocessedLinearTriggers("proj", 10)
			require.NoError(t, err)
			var id int64
			for _, candidate := range pending {
				if candidate.SourceID == tc.name {
					id = candidate.ID
				}
			}
			require.NotZero(t, id)
			require.NoError(t, tc.mark(id))
		})
	}

	unknown, err := client.LastSeenCommentAt("proj", "lin-1")
	require.NoError(t, err)
	assert.True(t, unknown.IsZero())

	first := detectedAt
	second := first.Add(time.Hour)
	require.NoError(t, client.SetLastSeenCommentAt("proj", "lin-1", second))
	require.NoError(t, client.SetLastSeenCommentAt("proj", "lin-1", first))
	got, err := client.LastSeenCommentAt("proj", "lin-1")
	require.NoError(t, err)
	assert.Equal(t, second, got)
}

func TestHTTPStore_LinearWebhookDeliveries_RoundTrip(t *testing.T) {
	backend, err := taskstore.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backend.Close()) })
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	t.Cleanup(srv.Close)

	client := taskstore.NewHTTPStore(srv.URL, "proj")
	receivedAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	delivery := taskstore.LinearWebhookDelivery{
		DeliveryID:    "delivery/1",
		LinearEvent:   "Comment",
		Action:        "create",
		LinearIssueID: "lin-1",
		SourceKind:    "comment",
		SourceID:      "comment-1",
		Status:        "received",
		ReceivedAt:    receivedAt,
	}

	recorded, err := client.RecordLinearWebhookDelivery("proj", delivery)
	require.NoError(t, err)
	assert.True(t, recorded)
	recorded, err = client.RecordLinearWebhookDelivery("proj", delivery)
	require.NoError(t, err)
	assert.False(t, recorded)

	got, err := client.LinearWebhookDeliveryByID("proj", "delivery/1")
	require.NoError(t, err)
	assert.Equal(t, "received", got.Status)
	assert.Equal(t, receivedAt, got.ReceivedAt)

	require.NoError(t, client.UpdateLinearWebhookDelivery("proj", "delivery/1", "accepted", "queued"))
	got, err = client.LinearWebhookDeliveryByID("proj", "delivery/1")
	require.NoError(t, err)
	assert.Equal(t, "accepted", got.Status)
	assert.Equal(t, "queued", got.Reason)

	delivery.DeliveryID = "delivery-2"
	delivery.Status = "failed"
	delivery.ReceivedAt = receivedAt.Add(time.Minute)
	recorded, err = client.RecordLinearWebhookDelivery("proj", delivery)
	require.NoError(t, err)
	assert.True(t, recorded)

	recent, err := client.ListRecentLinearWebhookDeliveries("proj", 1)
	require.NoError(t, err)
	require.Len(t, recent, 1)
	assert.Equal(t, "delivery-2", recent[0].DeliveryID)

	stats, err := client.LinearWebhookStats("proj", receivedAt.Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 1, stats.Accepted)
	assert.Equal(t, 1, stats.Failed)

	_, err = client.LinearWebhookDeliveryByID("proj", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	err = client.UpdateLinearWebhookDelivery("proj", "missing", "failed", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHTTPStore_VerifyingAtRoundTrip(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()
	client := taskstore.NewHTTPStore(srv.URL, "kasmos")

	ts := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	entry := taskstore.TaskEntry{
		Filename:    "verify-task",
		Status:      taskstore.StatusVerifying,
		VerifyingAt: ts,
	}
	require.NoError(t, client.Create("kasmos", entry))

	got, err := client.Get("kasmos", "verify-task")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusVerifying, got.Status)
	assert.Equal(t, ts, got.VerifyingAt, "verifying_at must survive HTTP Create+Get round-trip")

	// Update and re-fetch.
	ts2 := ts.Add(time.Hour)
	got.VerifyingAt = ts2
	require.NoError(t, client.Update("kasmos", "verify-task", got))

	got2, err := client.Get("kasmos", "verify-task")
	require.NoError(t, err)
	assert.Equal(t, ts2, got2.VerifyingAt, "verifying_at must survive HTTP Update round-trip")

	// Verify it appears in List.
	list, err := client.List("kasmos")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, ts2, list[0].VerifyingAt, "verifying_at must appear in HTTP List results")
}

func TestHTTPStore_PhaseTimestamp_Verifying(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()
	client := taskstore.NewHTTPStore(srv.URL, "kasmos")

	require.NoError(t, client.Create("kasmos", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))

	ts := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	require.NoError(t, client.SetPhaseTimestamp("kasmos", "plan", "verifying", ts))

	got, err := backend.Get("kasmos", "plan")
	require.NoError(t, err)
	assert.Equal(t, ts, got.VerifyingAt, "SetPhaseTimestamp('verifying') must persist verifying_at")
}
