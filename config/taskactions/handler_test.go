package taskactions_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskactions"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers --------------------------------------------------------------------

func newTestServer(t *testing.T) (*httptest.Server, taskstore.Store) {
	t.Helper()
	store := taskstore.NewTestSQLiteStore(t)
	srv := httptest.NewServer(taskactions.NewHandler(store))
	t.Cleanup(srv.Close)
	return srv, store
}

func createTask(t *testing.T, store taskstore.Store, project, filename string) {
	t.Helper()
	err := store.Create(project, taskstore.TaskEntry{
		Filename:  filename,
		Status:    taskstore.StatusReady,
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)
}

func doJSON(t *testing.T, srv *httptest.Server, method, path string, body any) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, srv.URL+path, reqBody)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeEntry(t *testing.T, resp *http.Response) taskstore.TaskEntry {
	t.Helper()
	var entry taskstore.TaskEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entry))
	resp.Body.Close()
	return entry
}

func decodeError(t *testing.T, resp *http.Response) string {
	t.Helper()
	var errResp map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	resp.Body.Close()
	return errResp["error"]
}

// validPlanContent is minimal markdown with a goal, wave and task so that
// planner_finished preconditions are satisfied.
const validPlanContent = `**Goal:** build something great

## Wave 1

### Task 1: do the thing

implement the thing
`

// ---- transition tests -------------------------------------------------------

func TestTransition_HappyPath(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "my-task")
	// Set content and mark as planned so implement_start succeeds.
	require.NoError(t, store.SetContent("proj", "my-task", validPlanContent))

	// plan_start → planning
	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/my-task/transition",
		map[string]string{"event": "plan_start"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, taskstore.StatusPlanning, entry.Status)
}

func TestTransition_ReviewChanges_PrimaryToken(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "task-x")
	// Drive the task to reviewing status.
	require.NoError(t, store.Update("proj", "task-x", taskstore.TaskEntry{
		Filename: "task-x",
		Status:   taskstore.StatusReviewing,
	}))

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/task-x/transition",
		map[string]string{"event": "review_changes"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status)
}

func TestTransition_ReviewChangesRequested_Alias(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "task-y")
	require.NoError(t, store.Update("proj", "task-y", taskstore.TaskEntry{
		Filename: "task-y",
		Status:   taskstore.StatusReviewing,
	}))

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/task-y/transition",
		map[string]string{"event": "review_changes_requested"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status)
}

func TestTransition_DraftReady_ImplementStart_Rejected(t *testing.T) {
	srv, store := newTestServer(t)
	// Draft-ready: status=ready, no execution phase set.
	createTask(t, store, "proj", "draft-task")

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/draft-task/transition",
		map[string]string{"event": "implement_start"})
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	errMsg := decodeError(t, resp)
	assert.NotEmpty(t, errMsg)
}

func TestTransition_PlannerFinished_EmptyContent_Rejected(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "plan-task")
	require.NoError(t, store.Update("proj", "plan-task", taskstore.TaskEntry{
		Filename: "plan-task",
		Status:   taskstore.StatusPlanning,
	}))
	// No content set — should reject planner_finished.

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/plan-task/transition",
		map[string]string{"event": "planner_finished"})
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	errMsg := decodeError(t, resp)
	assert.Contains(t, errMsg, "plan content missing")
}

func TestTransition_PlannerFinished_UnparsableContent_Rejected(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "bad-plan")
	require.NoError(t, store.Update("proj", "bad-plan", taskstore.TaskEntry{
		Filename: "bad-plan",
		Status:   taskstore.StatusPlanning,
	}))
	// Content without wave headers — not parseable.
	require.NoError(t, store.SetContent("proj", "bad-plan", "**Goal:** something\n\nno wave sections here"))

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/bad-plan/transition",
		map[string]string{"event": "planner_finished"})
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	errMsg := decodeError(t, resp)
	assert.Contains(t, errMsg, "not implementation-ready")
}

func TestTransition_PlannerFinished_ValidContent_Accepted(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "good-plan")
	require.NoError(t, store.Update("proj", "good-plan", taskstore.TaskEntry{
		Filename: "good-plan",
		Status:   taskstore.StatusPlanning,
	}))
	require.NoError(t, store.SetContent("proj", "good-plan", validPlanContent))

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/good-plan/transition",
		map[string]string{"event": "planner_finished"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, taskstore.StatusReady, entry.Status)
}

// Legacy persisted statuses ("in_progress", "completed") must be normalized by
// the handler before prechecking FSM transitions. Without normalization the
// web task-actions path would return 409 for valid lifecycle events on rows
// created before legacy imports were canonicalized at ingest — see
// config/taskfsm/fsm.go:MapLegacyStatus and config/taskfsm/fsm_test.go:TestMapLegacyStatus.

func TestTransition_LegacyInProgress_AcceptsImplementFinished(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "legacy-impl")
	// Simulate a pre-normalization row persisted with the legacy status.
	require.NoError(t, store.Update("proj", "legacy-impl", taskstore.TaskEntry{
		Filename: "legacy-impl",
		Status:   taskstore.Status("in_progress"),
	}))

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/legacy-impl/transition",
		map[string]string{"event": "implement_finished"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, taskstore.StatusReviewing, entry.Status)
}

func TestTransition_LegacyCompleted_AcceptsReimplement(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "legacy-done")
	require.NoError(t, store.Update("proj", "legacy-done", taskstore.TaskEntry{
		Filename: "legacy-done",
		Status:   taskstore.Status("completed"),
	}))

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/legacy-done/transition",
		map[string]string{"event": "reimplement"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status)
}

func TestAvailableActions_LegacyInProgress_ExposesImplementFinished(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "legacy-impl-actions")
	require.NoError(t, store.Update("proj", "legacy-impl-actions", taskstore.TaskEntry{
		Filename: "legacy-impl-actions",
		Status:   taskstore.Status("in_progress"),
	}))

	resp := doJSON(t, srv, http.MethodGet, "/v1/projects/proj/tasks/legacy-impl-actions/available-actions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Transitions []struct{ Event string }
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()

	found := false
	for _, tr := range body.Transitions {
		if tr.Event == "implement_finished" {
			found = true
			break
		}
	}
	assert.True(t, found, "implement_finished should be available for legacy in_progress tasks")
}

func TestAvailableActions_LegacyCompleted_ExposesDoneTransitions(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "legacy-done-actions")
	require.NoError(t, store.Update("proj", "legacy-done-actions", taskstore.TaskEntry{
		Filename: "legacy-done-actions",
		Status:   taskstore.Status("completed"),
	}))

	resp := doJSON(t, srv, http.MethodGet, "/v1/projects/proj/tasks/legacy-done-actions/available-actions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Transitions []struct{ Event string }
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()

	events := make(map[string]bool)
	for _, tr := range body.Transitions {
		events[tr.Event] = true
	}
	// All four transitions valid from StatusDone must be exposed.
	assert.True(t, events["start_over"], "start_over should be available for legacy completed tasks")
	assert.True(t, events["reimplement"], "reimplement should be available for legacy completed tasks")
	assert.True(t, events["request_review"], "request_review should be available for legacy completed tasks")
	assert.True(t, events["cancel"], "cancel should be available for legacy completed tasks")
}

func TestTransition_UnknownEvent_400(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "my-task")

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/my-task/transition",
		map[string]string{"event": "nonexistent_event"})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestTransition_NotFound_404(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/missing/transition",
		map[string]string{"event": "plan_start"})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// ---- available-actions tests ------------------------------------------------

func TestAvailableActions_DraftReady_ExcludesImplementStart(t *testing.T) {
	srv, store := newTestServer(t)
	// Draft-ready: status=ready, no execution phase.
	createTask(t, store, "proj", "draft-task")

	resp := doJSON(t, srv, http.MethodGet, "/v1/projects/proj/tasks/draft-task/available-actions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Transitions []struct{ Event string }
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()

	for _, tr := range body.Transitions {
		assert.NotEqual(t, "implement_start", tr.Event, "implement_start must be excluded for draft-ready tasks")
	}
}

func TestAvailableActions_PlannerFinished_ExcludedWhenNoContent(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "plan-task")
	require.NoError(t, store.Update("proj", "plan-task", taskstore.TaskEntry{
		Filename: "plan-task",
		Status:   taskstore.StatusPlanning,
	}))
	// No content — planner_finished should be excluded.

	resp := doJSON(t, srv, http.MethodGet, "/v1/projects/proj/tasks/plan-task/available-actions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Transitions []struct{ Event string }
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()

	for _, tr := range body.Transitions {
		assert.NotEqual(t, "planner_finished", tr.Event)
	}
}

func TestAvailableActions_PlannerFinished_IncludedWithValidContent(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "plan-task")
	require.NoError(t, store.Update("proj", "plan-task", taskstore.TaskEntry{
		Filename: "plan-task",
		Status:   taskstore.StatusPlanning,
	}))
	require.NoError(t, store.SetContent("proj", "plan-task", validPlanContent))

	resp := doJSON(t, srv, http.MethodGet, "/v1/projects/proj/tasks/plan-task/available-actions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Transitions []struct{ Event string }
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()

	found := false
	for _, tr := range body.Transitions {
		if tr.Event == "planner_finished" {
			found = true
			break
		}
	}
	assert.True(t, found, "planner_finished should be included when content is valid")
}

func TestAvailableActions_NotFound_404(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, srv, http.MethodGet, "/v1/projects/proj/tasks/missing/available-actions", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// ---- status override tests --------------------------------------------------

func TestStatus_ManualOverride_HappyPath(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "task-a")

	resp := doJSON(t, srv, http.MethodPut, "/v1/projects/proj/tasks/task-a/status",
		map[string]string{"target": "implementing"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status)
}

func TestStatus_ManualOverride_Invalid_400(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "task-b")

	resp := doJSON(t, srv, http.MethodPut, "/v1/projects/proj/tasks/task-b/status",
		map[string]string{"target": "bogus-status"})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestStatus_NotFound_404(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, srv, http.MethodPut, "/v1/projects/proj/tasks/missing/status",
		map[string]string{"target": "implementing"})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// ---- rename tests -----------------------------------------------------------

func TestRename_SlugifiesHumanInput(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "old-name")

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/old-name/rename",
		map[string]string{"new_filename": "My Cool Feature!"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, "my-cool-feature", entry.Filename)
}

func TestRename_StripsTrailingMdExtension(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "task-one")

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/task-one/rename",
		map[string]string{"new_filename": "task-two.md"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, "task-two", entry.Filename)
}

func TestRename_NotFound_404(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/missing/rename",
		map[string]string{"new_filename": "new-name"})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// TestRename_PreservesIngestedChildren renames a task that has already been
// through a /content ingest (which creates subtask rows) and has recorded PR
// review activity. The rename must succeed and the derived children must
// follow the parent to the new filename.
func TestRename_PreservesIngestedChildren(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "ingested-task")

	req, err := http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/proj/tasks/ingested-task/content",
		strings.NewReader(validPlanContent))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	require.NoError(t, store.RecordPRReview("proj", "ingested-task", 7, "COMMENTED", "looks ok", "reviewer"))

	subtasksBefore, err := store.GetSubtasks("proj", "ingested-task")
	require.NoError(t, err)
	require.NotEmpty(t, subtasksBefore)

	renameResp := doJSON(t, srv, http.MethodPost, "/v1/projects/proj/tasks/ingested-task/rename",
		map[string]string{"new_filename": "renamed task"})
	require.Equal(t, http.StatusOK, renameResp.StatusCode)
	entry := decodeEntry(t, renameResp)
	assert.Equal(t, "renamed-task", entry.Filename)

	subtasksAfter, err := store.GetSubtasks("proj", "renamed-task")
	require.NoError(t, err)
	assert.Equal(t, len(subtasksBefore), len(subtasksAfter))

	oldSubtasks, err := store.GetSubtasks("proj", "ingested-task")
	require.NoError(t, err)
	assert.Empty(t, oldSubtasks)

	pending, err := store.ListPendingReviews("proj", "renamed-task")
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, 7, pending[0].ReviewID)
}

// ---- topic tests ------------------------------------------------------------

func TestTopic_SetTopic(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "task-t")

	resp := doJSON(t, srv, http.MethodPut, "/v1/projects/proj/tasks/task-t/topic",
		map[string]string{"topic": "backend"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, "backend", entry.Topic)
}

func TestTopic_ClearTopic(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "task-u")
	// Set a topic first.
	require.NoError(t, store.Update("proj", "task-u", taskstore.TaskEntry{
		Filename: "task-u",
		Status:   taskstore.StatusReady,
		Topic:    "frontend",
	}))

	resp := doJSON(t, srv, http.MethodPut, "/v1/projects/proj/tasks/task-u/topic",
		map[string]string{"topic": ""})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, "", entry.Topic)
}

func TestTopic_NotFound_404(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, srv, http.MethodPut, "/v1/projects/proj/tasks/missing/topic",
		map[string]string{"topic": "something"})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// ---- goal tests -------------------------------------------------------------

func TestGoal_SetGoal(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "task-goal")

	resp := doJSON(t, srv, http.MethodPut, "/v1/projects/proj/tasks/task-goal/goal",
		map[string]string{"goal": "ship it"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	entry := decodeEntry(t, resp)
	assert.Equal(t, "ship it", entry.Goal)
}

func TestGoal_NotFound_404(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, srv, http.MethodPut, "/v1/projects/proj/tasks/missing/goal",
		map[string]string{"goal": "ship it"})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// ---- content tests ----------------------------------------------------------

func TestContent_IngestUpdatesGoalAndSubtasks(t *testing.T) {
	srv, store := newTestServer(t)
	createTask(t, store, "proj", "content-task")

	req, err := http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/proj/tasks/content-task/content",
		strings.NewReader(validPlanContent))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	entry := decodeEntry(t, resp)
	assert.Equal(t, "build something great", entry.Goal)

	// Verify subtasks were created.
	subtasks, err := store.GetSubtasks("proj", "content-task")
	require.NoError(t, err)
	assert.Len(t, subtasks, 1)
	assert.Equal(t, "do the thing", subtasks[0].Title)
}

func TestContent_NotFound_404(t *testing.T) {
	srv, _ := newTestServer(t)

	req, err := http.NewRequest(http.MethodPut,
		srv.URL+"/v1/projects/proj/tasks/missing/content",
		strings.NewReader("some content"))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}
