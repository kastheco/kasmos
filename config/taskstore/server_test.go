package taskstore_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_CreateAndGetPlan(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	body := `{"filename":"test","status":"ready","description":"test"}`
	resp, err := http.Post(srv.URL+"/v1/projects/kasmos/tasks", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, err = http.Get(srv.URL + "/v1/projects/kasmos/tasks/test")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var got taskstore.TaskEntry
	json.NewDecoder(resp.Body).Decode(&got)
	assert.Equal(t, taskstore.StatusReady, got.Status)
}

func TestServer_DeleteTask(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/projects/kasmos/tasks", "application/json", strings.NewReader(`{"filename":"delete-me","status":"ready","description":"test"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/v1/projects/kasmos/tasks/delete-me", nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/v1/projects/kasmos/tasks/delete-me")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	var notFound map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&notFound))
	resp.Body.Close()
	assert.Contains(t, notFound["error"], "task not found")
}

func TestServer_DeleteTask_NotFound(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/v1/projects/kasmos/tasks/missing", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	var notFound map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&notFound))
	resp.Body.Close()
	assert.Condition(t, func() bool {
		return strings.Contains(notFound["error"], "task not found") || strings.Contains(notFound["error"], "plan not found")
	})
}

func TestServer_ListByStatus(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	// Create plans with different statuses
	for _, p := range []taskstore.TaskEntry{
		{Filename: "a", Status: taskstore.StatusReady},
		{Filename: "b", Status: taskstore.StatusDone},
	} {
		store.Create("kasmos", p)
	}

	resp, err := http.Get(srv.URL + "/v1/projects/kasmos/tasks?status=ready")
	require.NoError(t, err)
	var plans []taskstore.TaskEntry
	json.NewDecoder(resp.Body).Decode(&plans)
	assert.Len(t, plans, 1)
}

func TestServer_Ping(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/ping")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_SetClickUpTaskID(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	// Create a plan first
	body := `{"filename":"plan","status":"ready"}`
	resp, err := http.Post(srv.URL+"/v1/projects/kasmos/tasks", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// PUT clickup-task-id
	req, err := http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/plan/clickup-task-id",
		strings.NewReader(`{"clickup_task_id":"CU-abc123"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify it was stored
	got, err := store.Get("kasmos", "plan")
	require.NoError(t, err)
	assert.Equal(t, "CU-abc123", got.ClickUpTaskID)
}

func TestServer_SetClickUpTaskID_NotFound(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/nonexistent/clickup-task-id",
		strings.NewReader(`{"clickup_task_id":"CU-xyz"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestServer_SetLinearLink(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/projects/kasmos/tasks", "application/json", strings.NewReader(`{"filename":"plan","status":"ready"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	body := `{"linear_issue_id":"issue-123","linear_identifier":"KAS-123","linear_url":"https://linear.app/kas/issue/KAS-123","linear_team_key":"KAS","linear_project_id":"project-456"}`
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/v1/projects/kasmos/tasks/plan/linear-link", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	got, err := store.Get("kasmos", "plan")
	require.NoError(t, err)
	assert.Equal(t, "issue-123", got.LinearIssueID)
	assert.Equal(t, "KAS-123", got.LinearIdentifier)

	req, err = http.NewRequest(http.MethodPut, srv.URL+"/v1/projects/kasmos/tasks/missing/linear-link", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestServer_ClearLinearLink(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))
	require.NoError(t, store.SetLinearLink("kasmos", "plan", taskstore.LinearLink{LinearIssueID: "issue-123", LinearIdentifier: "KAS-123"}))

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/v1/projects/kasmos/tasks/plan/linear-link", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	got, err := store.Get("kasmos", "plan")
	require.NoError(t, err)
	assert.Empty(t, got.LinearIssueID)

	req, err = http.NewRequest(http.MethodDelete, srv.URL+"/v1/projects/kasmos/tasks/missing/linear-link", nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestServer_FindLinkedTask(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))
	require.NoError(t, store.SetLinearLink("kasmos", "plan", taskstore.LinearLink{LinearIssueID: "issue-123", LinearIdentifier: "KAS-123"}))

	resp, err := http.Get(srv.URL + "/v1/projects/kasmos/tasks/_/linear-link/lookup?issue=issue-123&status=ready")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var payload struct {
		Filename string `json:"filename"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	resp.Body.Close()
	assert.Equal(t, "plan", payload.Filename)

	resp, err = http.Get(srv.URL + "/v1/projects/kasmos/tasks/_/linear-link/lookup?issue=missing&status=ready")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/v1/projects/kasmos/tasks/_/linear-link/lookup?status=ready")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestHTTPStore_SetLinearLinkIfNoActiveDuplicate(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()
	client := taskstore.NewHTTPStore(srv.URL, "kasmos")

	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusPlanning}))
	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "other", Status: taskstore.StatusReviewing}))

	link := taskstore.LinearLink{LinearIssueID: "issue-123", LinearIdentifier: "KAS-123"}
	conflict, err := client.SetLinearLinkIfNoActiveDuplicate("kasmos", "plan", link, taskstore.StatusPlanning, taskstore.StatusReviewing)
	require.NoError(t, err)
	assert.Empty(t, conflict)

	conflict, err = client.SetLinearLinkIfNoActiveDuplicate("kasmos", "other", link, taskstore.StatusPlanning, taskstore.StatusReviewing)
	require.NoError(t, err)
	assert.Equal(t, "plan", conflict)

	other, err := store.Get("kasmos", "other")
	require.NoError(t, err)
	assert.Empty(t, other.LinearIssueID)
}

func TestServer_ContentEndpoints(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	// Create a plan first
	body := `{"filename":"plan","status":"ready","content":"# Initial"}`
	resp, err := http.Post(srv.URL+"/v1/projects/kasmos/tasks", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// GET content
	resp, err = http.Get(srv.URL + "/v1/projects/kasmos/tasks/plan/content")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/markdown", resp.Header.Get("Content-Type"))
	gotBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, "# Initial", string(gotBody))

	// PUT content
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/v1/projects/kasmos/tasks/plan/content", strings.NewReader("# Updated"))
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// GET content again to verify update
	resp, err = http.Get(srv.URL + "/v1/projects/kasmos/tasks/plan/content")
	require.NoError(t, err)
	gotBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, "# Updated", string(gotBody))
}

func TestServer_SubtasksEndpoints(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/projects/kasmos/tasks", "application/json", strings.NewReader(`{"filename":"plan","status":"ready"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/v1/projects/kasmos/tasks/plan/subtasks")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var got []taskstore.SubtaskEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	resp.Body.Close()
	assert.Len(t, got, 0)

	req, err := http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/plan/subtasks",
		strings.NewReader(`[{"task_number":1,"title":"first","status":"pending"}]`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	req, err = http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/plan/subtasks/1/status",
		strings.NewReader(`{"status":"done"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/v1/projects/kasmos/tasks/plan/subtasks")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var updated []taskstore.SubtaskEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&updated))
	resp.Body.Close()
	assert.Equal(t, taskstore.SubtaskStatusDone, updated[0].Status)
}

func TestServer_Subtasks_ContractErrors(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/projects/kasmos/tasks", "application/json", strings.NewReader(`{"filename":"plan","status":"ready"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/v1/projects/kasmos/tasks/missing/subtasks")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	var notFound map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&notFound))
	resp.Body.Close()
	assert.Contains(t, notFound["error"], "plan not found")

	req, err := http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/missing/subtasks",
		strings.NewReader(`{"task_number":1,"title":"bad","status":"pending"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&notFound))
	resp.Body.Close()
	assert.Contains(t, notFound["error"], "plan not found")

	req, err = http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/plan/subtasks",
		strings.NewReader("{"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var badRequest map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&badRequest))
	resp.Body.Close()
	assert.Contains(t, badRequest["error"], "invalid request body")

	req, err = http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/plan/subtasks/1/status",
		strings.NewReader("{"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&badRequest))
	resp.Body.Close()
	assert.Contains(t, badRequest["error"], "invalid request body")

	req, err = http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/plan/subtasks/1/status",
		strings.NewReader(`{"status":"done"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&badRequest))
	resp.Body.Close()
	assert.Contains(t, badRequest["error"], "subtask not found")
}

func TestServer_PhaseTimestampAndGoalEndpoints(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/projects/kasmos/tasks", "application/json", strings.NewReader(`{"filename":"plan","status":"ready"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	req, err := http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/plan/phase-timestamp",
		strings.NewReader(`{"phase":"planning","timestamp":"2026-01-02T03:04:05Z"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	req, err = http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/plan/goal",
		strings.NewReader(`{"goal":"ship faster"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	got, err := store.Get("kasmos", "plan")
	require.NoError(t, err)
	assert.Equal(t, "ship faster", got.Goal)
	assert.False(t, got.PlanningAt.IsZero())

	bad, err := http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/plan/phase-timestamp",
		strings.NewReader("{"))
	require.NoError(t, err)
	bad.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(bad)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var badPayload map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&badPayload))
	resp.Body.Close()
	assert.Contains(t, badPayload["error"], "invalid request body")

	bad, err = http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/missing/goal",
		strings.NewReader(`{"goal":"ship faster"}`))
	require.NoError(t, err)
	bad.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(bad)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	var missing map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&missing))
	resp.Body.Close()
	assert.Contains(t, missing["error"], "plan not found")

	bad, err = http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/missing/phase-timestamp",
		strings.NewReader(`{"phase":"planning","timestamp":"2026-01-02T03:04:05Z"}`))
	require.NoError(t, err)
	bad.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(bad)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&missing))
	resp.Body.Close()
	assert.Contains(t, missing["error"], "plan not found")
}

func TestServer_PRReviewsEndpoints(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	// Create plan
	resp, err := http.Post(srv.URL+"/v1/projects/proj/tasks", "application/json", strings.NewReader(`{"filename":"plan","status":"ready"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Record a review
	req, err := http.NewRequest(http.MethodPost,
		srv.URL+"/v1/projects/proj/tasks/plan/pr-reviews",
		strings.NewReader(`{"review_id":101,"review_state":"CHANGES_REQUESTED","review_body":"fix it","reviewer_login":"alice"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Check processed — should be true now
	resp, err = http.Get(srv.URL + "/v1/projects/proj/tasks/plan/pr-reviews/101/processed")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var processed map[string]bool
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&processed))
	resp.Body.Close()
	assert.True(t, processed["processed"])

	// Check processed for non-existent review — should be false
	resp, err = http.Get(srv.URL + "/v1/projects/proj/tasks/plan/pr-reviews/999/processed")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&processed))
	resp.Body.Close()
	assert.False(t, processed["processed"])

	// List pending
	resp, err = http.Get(srv.URL + "/v1/projects/proj/tasks/plan/pr-reviews/pending")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var pending []taskstore.PRReviewEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pending))
	resp.Body.Close()
	require.Len(t, pending, 1)
	assert.Equal(t, 101, pending[0].ReviewID)

	// Mark reacted
	req, err = http.NewRequest(http.MethodPost,
		srv.URL+"/v1/projects/proj/tasks/plan/pr-reviews/101/reacted", nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Mark fixer dispatched
	req, err = http.NewRequest(http.MethodPost,
		srv.URL+"/v1/projects/proj/tasks/plan/pr-reviews/101/fixer-dispatched", nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Pending list should now be empty
	resp, err = http.Get(srv.URL + "/v1/projects/proj/tasks/plan/pr-reviews/pending")
	require.NoError(t, err)
	var empty []taskstore.PRReviewEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&empty))
	resp.Body.Close()
	assert.Len(t, empty, 0)
}

func TestServer_PRReviews_NotFoundErrors(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	// Create plan but no reviews
	resp, err := http.Post(srv.URL+"/v1/projects/proj/tasks", "application/json", strings.NewReader(`{"filename":"plan","status":"ready"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Mark reacted on non-existent review
	req, err := http.NewRequest(http.MethodPost,
		srv.URL+"/v1/projects/proj/tasks/plan/pr-reviews/9999/reacted", nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	var errBody map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
	resp.Body.Close()
	assert.Contains(t, errBody["error"], "not found")

	// Mark fixer dispatched on non-existent review
	req, err = http.NewRequest(http.MethodPost,
		srv.URL+"/v1/projects/proj/tasks/plan/pr-reviews/9999/fixer-dispatched", nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
	resp.Body.Close()
	assert.Contains(t, errBody["error"], "not found")
}

func TestServer_PRReviews_BadRequestOnMalformedBody(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost,
		srv.URL+"/v1/projects/proj/tasks/plan/pr-reviews",
		strings.NewReader("{"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var errBody map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
	resp.Body.Close()
	assert.Contains(t, errBody["error"], "invalid request body")
}

func TestServer_LinearTriggers_BadRequests(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/projects/proj/linear-triggers")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, err = http.Post(srv.URL+"/v1/projects/proj/linear-triggers/not-an-id/dispatched", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestServer_NormalizeFilename verifies that .md-suffixed names and bare slugs
// are treated as the same stored task across every ingress point.
func TestServer_NormalizeFilename(t *testing.T) {
	t.Run("create with .md suffix stores bare slug", func(t *testing.T) {
		store := newTestStore(t)
		srv := httptest.NewServer(taskstore.NewHandler(store))
		defer srv.Close()

		resp, err := http.Post(srv.URL+"/v1/projects/kasmos/tasks", "application/json",
			strings.NewReader(`{"filename":"plan.md","status":"ready","description":"test"}`))
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		resp.Body.Close()

		// The stored entry must use the bare slug.
		got, err := store.Get("kasmos", "plan")
		require.NoError(t, err)
		assert.Equal(t, "plan", got.Filename)
	})

	t.Run("GET with .md suffix and bare slug retrieve same entry", func(t *testing.T) {
		store := newTestStore(t)
		srv := httptest.NewServer(taskstore.NewHandler(store))
		defer srv.Close()

		require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))

		for _, variant := range []string{"plan.md", "plan"} {
			t.Run(variant, func(t *testing.T) {
				resp, err := http.Get(srv.URL + "/v1/projects/kasmos/tasks/" + variant)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				var got taskstore.TaskEntry
				require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
				assert.Equal(t, "plan", got.Filename)
			})
		}
	})

	t.Run("PUT content with .md suffix and GET bare slug round-trip", func(t *testing.T) {
		store := newTestStore(t)
		srv := httptest.NewServer(taskstore.NewHandler(store))
		defer srv.Close()

		require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))

		// Write via .md-suffixed path.
		req, err := http.NewRequest(http.MethodPut,
			srv.URL+"/v1/projects/kasmos/tasks/plan.md/content",
			strings.NewReader("# hello"))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Read back via bare slug.
		resp, err = http.Get(srv.URL + "/v1/projects/kasmos/tasks/plan/content")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "# hello", string(raw))
	})

	t.Run("rename with .md new_filename stores bare slug", func(t *testing.T) {
		store := newTestStore(t)
		srv := httptest.NewServer(taskstore.NewHandler(store))
		defer srv.Close()

		require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReady}))

		req, err := http.NewRequest(http.MethodPost,
			srv.URL+"/v1/projects/kasmos/tasks/plan/rename",
			strings.NewReader(`{"new_filename":"renamed.md"}`))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Old name gone, new bare slug exists.
		_, err = store.Get("kasmos", "plan")
		assert.Error(t, err)
		got, err := store.Get("kasmos", "renamed")
		require.NoError(t, err)
		assert.Equal(t, "renamed", got.Filename)
	})
}

func TestServer_PhaseTimestamp_Verifying(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/projects/kasmos/tasks", "application/json",
		strings.NewReader(`{"filename":"plan","status":"ready"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	req, err := http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/kasmos/tasks/plan/phase-timestamp",
		strings.NewReader(`{"phase":"verifying","timestamp":"2026-03-15T10:00:00Z"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	got, err := store.Get("kasmos", "plan")
	require.NoError(t, err)
	assert.False(t, got.VerifyingAt.IsZero(), "verifying_at must be set after phase-timestamp PUT with verifying phase")
}

func TestServer_EmptyListEndpointsReturnJSONArray(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(taskstore.NewHandler(store))
	defer srv.Close()

	// Seed one task so /subtasks route passes the not-found preflight.
	body := `{"filename":"plan","status":"ready","description":"test"}`
	resp, err := http.Post(srv.URL+"/v1/projects/proj/tasks", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	tests := []struct {
		name string
		path string
	}{
		{"empty tasks list", "/v1/projects/empty/tasks"},
		{"empty subtasks list", "/v1/projects/proj/tasks/plan/subtasks"},
		{"empty topics list", "/v1/projects/proj/topics"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(srv.URL + tt.path)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
			raw, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "[]\n", string(raw))
		})
	}
}
