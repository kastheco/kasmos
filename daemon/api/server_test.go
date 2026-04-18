package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type startPlanStub struct {
	DaemonState
	project          string
	filename         string
	prompt           string
	program          string
	listTasksProject string
	tasks            []TaskStatus
}

func (s *startPlanStub) ListPlans(_ string) ([]taskstore.TaskEntry, error) { return nil, nil }
func (s *startPlanStub) ListTasks(project string) ([]TaskStatus, error) {
	s.listTasksProject = project
	return s.tasks, nil
}
func (s *startPlanStub) ListInstances(_ string) []InstanceStatus { return nil }
func (s *startPlanStub) EventStream() <-chan Event               { return make(chan Event) }
func (s *startPlanStub) StartPlan(project, filename, prompt, program string) error {
	s.project = project
	s.filename = filename
	s.prompt = prompt
	s.program = program
	return nil
}

func TestHandler_Status(t *testing.T) {
	state := &DaemonState{
		Running: true,
		Repos:   []RepoStatus{{Path: "/tmp/test", Project: "test", ActivePlans: 0}},
	}
	h := NewHandler(state)

	req := httptest.NewRequest("GET", "/v1/status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp StatusResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.True(t, resp.Running)
	assert.Len(t, resp.Repos, 1)
}

func TestHandler_ListRepos(t *testing.T) {
	state := &DaemonState{
		Running: true,
		Repos:   []RepoStatus{{Path: "/tmp/a", Project: "a"}, {Path: "/tmp/b", Project: "b"}},
	}
	h := NewHandler(state)

	req := httptest.NewRequest("GET", "/v1/repos", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var repos []RepoStatus
	require.NoError(t, json.NewDecoder(w.Body).Decode(&repos))
	assert.Len(t, repos, 2)
}

func TestHandler_ListTasks(t *testing.T) {
	state := &startPlanStub{tasks: []TaskStatus{{
		Filename:    "verify-every-fsm-transition",
		Status:      "implementing",
		Branch:      "task/w1-t3",
		PRURL:       "https://github.com/kastheco/kasmos/pull/123",
		ReviewCycle: 2,
		Description: "add daemon task-list API tests",
		ExecutionState: taskstore.ExecutionState{
			Phase:           "coding",
			ActiveAgentType: "coder",
			ActiveWave:      1,
		},
	}}}
	h := NewHandler(state)

	req := httptest.NewRequest("GET", "/v1/repos/cms/tasks", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "cms", state.listTasksProject)

	var tasks []TaskStatus
	require.NoError(t, json.NewDecoder(w.Body).Decode(&tasks))
	require.Len(t, tasks, 1)
	assert.Equal(t, state.tasks, tasks)
	assert.Equal(t, "verify-every-fsm-transition", tasks[0].Filename)
	assert.Equal(t, "implementing", tasks[0].Status)
	assert.Equal(t, taskstore.ExecutionState{Phase: "coding", ActiveAgentType: "coder", ActiveWave: 1}, tasks[0].ExecutionState)
	assert.Equal(t, "task/w1-t3", tasks[0].Branch)
	assert.Equal(t, "https://github.com/kastheco/kasmos/pull/123", tasks[0].PRURL)
	assert.Equal(t, 2, tasks[0].ReviewCycle)
	assert.Equal(t, "add daemon task-list API tests", tasks[0].Description)

	nilState := &startPlanStub{}
	nilHandler := NewHandler(nilState)
	nilReq := httptest.NewRequest("GET", "/v1/repos/cms/tasks", nil)
	nilW := httptest.NewRecorder()
	nilHandler.ServeHTTP(nilW, nilReq)

	assert.Equal(t, http.StatusOK, nilW.Code)
	assert.JSONEq(t, `[]`, nilW.Body.String())

	var emptyTasks []TaskStatus
	require.NoError(t, json.NewDecoder(bytes.NewReader(nilW.Body.Bytes())).Decode(&emptyTasks))
	assert.NotNil(t, emptyTasks)
	assert.Len(t, emptyTasks, 0)
}

type instanceActionStub struct {
	DaemonState
	project string
	title   string
	action  string
	err     error
}

func (s *instanceActionStub) ListInstances(_ string) []InstanceStatus { return nil }
func (s *instanceActionStub) EventStream() <-chan Event               { return make(chan Event) }
func (s *instanceActionStub) StartPlan(_, _, _, _ string) error       { return nil }
func (s *instanceActionStub) ListPlans(_ string) ([]taskstore.TaskEntry, error) {
	return nil, nil
}
func (s *instanceActionStub) ListTasks(_ string) ([]TaskStatus, error) { return nil, nil }
func (s *instanceActionStub) PauseInstance(project, title string) error {
	s.project = project
	s.title = title
	s.action = "pause"
	return s.err
}
func (s *instanceActionStub) ResumeInstance(project, title string) error {
	s.project = project
	s.title = title
	s.action = "resume"
	return s.err
}
func (s *instanceActionStub) RestartInstance(project, title string) error {
	s.project = project
	s.title = title
	s.action = "restart"
	return s.err
}
func (s *instanceActionStub) KillInstance(project, title string) error {
	s.project = project
	s.title = title
	s.action = "kill"
	return s.err
}

func TestHandler_InstanceAction_HappyPath(t *testing.T) {
	for _, action := range []string{"pause", "resume", "restart", "kill"} {
		t.Run(action, func(t *testing.T) {
			state := &instanceActionStub{}
			h := NewHandler(state)

			req := httptest.NewRequest("POST", "/v1/repos/myproj/instances/my-agent/"+action, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
			assert.Equal(t, "myproj", state.project)
			assert.Equal(t, "my-agent", state.title)
			assert.Equal(t, action, state.action)
		})
	}
}

func TestHandler_InstanceAction_NotFound(t *testing.T) {
	// Every action (pause/resume/restart/kill) must return 404 when the backing
	// StateProvider reports the instance is not tracked. Locks down the
	// acceptance criterion that missing-title calls — including kill — map to
	// 404 instead of the previous 200 no-op.
	for _, action := range []string{"pause", "resume", "restart", "kill"} {
		t.Run(action, func(t *testing.T) {
			state := &instanceActionStub{err: fmt.Errorf("%w: missing", ErrInstanceNotFound)}
			h := NewHandler(state)

			req := httptest.NewRequest("POST", "/v1/repos/myproj/instances/missing/"+action, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
		})
	}
}

func TestHandler_InstanceAction_Conflict(t *testing.T) {
	// Pause/resume/restart must return 409 when the adapter wraps the error as
	// ErrInvalidTransition. Kill has no "invalid transition" case at the HTTP
	// layer (precondition failures such as dirty worktrees stay as 500), so it
	// is deliberately omitted here.
	for _, action := range []string{"pause", "resume", "restart"} {
		t.Run(action, func(t *testing.T) {
			state := &instanceActionStub{err: fmt.Errorf("%w: bad state", ErrInvalidTransition)}
			h := NewHandler(state)

			req := httptest.NewRequest("POST", "/v1/repos/myproj/instances/my-agent/"+action, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			assert.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
		})
	}
}

// instanceCaptureStub is a StateProvider stub for testing capture/send routes.
type instanceCaptureStub struct {
	DaemonState
	captureProject, captureTitle, captureStart, captureEnd string
	captureOutput                                          string
	captureErr                                             error
	sendProject, sendTitle, sendPrompt                     string
	sendErr                                                error
	permissionProject, permissionTitle                     string
	permissionChoice                                       PermissionChoice
	permissionErr                                          error
}

func (s *instanceCaptureStub) ListInstances(_ string) []InstanceStatus { return nil }
func (s *instanceCaptureStub) EventStream() <-chan Event               { return make(chan Event) }
func (s *instanceCaptureStub) StartPlan(_, _, _, _ string) error       { return nil }
func (s *instanceCaptureStub) ListPlans(_ string) ([]taskstore.TaskEntry, error) {
	return nil, nil
}
func (s *instanceCaptureStub) ListTasks(_ string) ([]TaskStatus, error) { return nil, nil }
func (s *instanceCaptureStub) PauseInstance(_, _ string) error {
	return fmt.Errorf("%w", ErrInstanceNotFound)
}
func (s *instanceCaptureStub) ResumeInstance(_, _ string) error {
	return fmt.Errorf("%w", ErrInstanceNotFound)
}
func (s *instanceCaptureStub) RestartInstance(_, _ string) error {
	return fmt.Errorf("%w", ErrInstanceNotFound)
}
func (s *instanceCaptureStub) KillInstance(_, _ string) error {
	return fmt.Errorf("%w", ErrInstanceNotFound)
}
func (s *instanceCaptureStub) CaptureInstance(project, title, start, end string) (string, error) {
	s.captureProject = project
	s.captureTitle = title
	s.captureStart = start
	s.captureEnd = end
	return s.captureOutput, s.captureErr
}
func (s *instanceCaptureStub) SendInstancePrompt(project, title, prompt string) error {
	s.sendProject = project
	s.sendTitle = title
	s.sendPrompt = prompt
	return s.sendErr
}
func (s *instanceCaptureStub) SendInstancePermissionResponse(project, title string, choice PermissionChoice) error {
	s.permissionProject = project
	s.permissionTitle = title
	s.permissionChoice = choice
	return s.permissionErr
}

func TestHandler_InstanceCapture_HappyPath(t *testing.T) {
	state := &instanceCaptureStub{captureOutput: "pane text\n"}
	h := NewHandler(state)

	req := httptest.NewRequest("GET", "/v1/repos/myproj/instances/my-agent/capture?start=-50&end=0", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "pane text\n", w.Body.String())
	assert.Equal(t, "myproj", state.captureProject)
	assert.Equal(t, "my-agent", state.captureTitle)
	assert.Equal(t, "-50", state.captureStart)
	assert.Equal(t, "0", state.captureEnd)
}

func TestHandler_InstanceCapture_NotFound(t *testing.T) {
	state := &instanceCaptureStub{captureErr: fmt.Errorf("%w: missing", ErrInstanceNotFound)}
	h := NewHandler(state)

	req := httptest.NewRequest("GET", "/v1/repos/myproj/instances/missing/capture", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
}

func TestHandler_InstanceSend_HappyPath(t *testing.T) {
	state := &instanceCaptureStub{}
	h := NewHandler(state)

	body := bytes.NewBufferString(`{"prompt":"hello from test"}`)
	req := httptest.NewRequest("POST", "/v1/repos/myproj/instances/my-agent/send", body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, "myproj", state.sendProject)
	assert.Equal(t, "my-agent", state.sendTitle)
	assert.Equal(t, "hello from test", state.sendPrompt)
}

func TestHandler_InstanceSend_NotFound(t *testing.T) {
	state := &instanceCaptureStub{sendErr: fmt.Errorf("%w: missing", ErrInstanceNotFound)}
	h := NewHandler(state)

	body := bytes.NewBufferString(`{"prompt":"hi"}`)
	req := httptest.NewRequest("POST", "/v1/repos/myproj/instances/missing/send", body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
}

func TestHandler_StartPlan(t *testing.T) {
	state := &startPlanStub{}
	h := NewHandler(state)

	body := bytes.NewBufferString(`{"prompt":"plan prompt","program":"opencode --model x"}`)
	req := httptest.NewRequest("POST", "/v1/repos/cms/plans/api-response-logging/plan", body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "cms", state.project)
	assert.Equal(t, "api-response-logging", state.filename)
	assert.Equal(t, "plan prompt", state.prompt)
	assert.Equal(t, "opencode --model x", state.program)
}

// ---------------------------------------------------------------------------
// Presentation endpoint tests
// ---------------------------------------------------------------------------

// presentationStub is a StateProvider stub for the presentation route.
type presentationStub struct {
	DaemonState
	rawTurns  json.RawMessage
	supported bool
	err       error
}

func (s *presentationStub) ListInstances(_ string) []InstanceStatus { return nil }
func (s *presentationStub) EventStream() <-chan Event               { return make(chan Event) }
func (s *presentationStub) StartPlan(_, _, _, _ string) error       { return nil }
func (s *presentationStub) ListPlans(_ string) ([]taskstore.TaskEntry, error) {
	return nil, nil
}
func (s *presentationStub) ListTasks(_ string) ([]TaskStatus, error) { return nil, nil }
func (s *presentationStub) CapturePresentation(_, _ string) (json.RawMessage, bool, error) {
	return s.rawTurns, s.supported, s.err
}

func TestHandler_InstancePresentation_SDK(t *testing.T) {
	// Pre-encode two turns with the exact wire-format field names.
	rawTurns := json.RawMessage(`[
		{"id":"t1","number":1,"started_at":"0001-01-01T00:00:00Z","completed_at":"0001-01-01T00:00:00Z","interrupted":false,"tool_count":0,"rows":[{"kind":"prose","text":"hello","timestamp":"0001-01-01T00:00:00Z","tool_name":"","is_error":false}]},
		{"id":"t2","number":2,"started_at":"0001-01-01T00:00:00Z","completed_at":"0001-01-01T00:00:00Z","interrupted":false,"tool_count":1,"rows":[{"kind":"tool","text":"bash(ls)","timestamp":"0001-01-01T00:00:00Z","tool_name":"bash","is_error":false}]}
	]`)
	state := &presentationStub{rawTurns: rawTurns, supported: true}
	h := NewHandler(state)

	req := httptest.NewRequest("GET", "/v1/repos/myproj/instances/my-agent/presentation", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var outer map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(w.Body).Decode(&outer))

	var supported bool
	require.NoError(t, json.Unmarshal(outer["supported"], &supported))
	assert.True(t, supported)

	var turns []map[string]any
	require.NoError(t, json.Unmarshal(outer["turns"], &turns))
	require.Len(t, turns, 2)
	assert.Equal(t, "t1", turns[0]["id"])
	assert.Equal(t, "t2", turns[1]["id"])

	var capturedAt time.Time
	require.NoError(t, json.Unmarshal(outer["captured_at"], &capturedAt))
	assert.False(t, capturedAt.IsZero(), "captured_at must parse as a real timestamp")
}

func TestHandler_InstancePresentation_Tmux(t *testing.T) {
	// tmux-backed instance: supported=false, turns=nil.
	// After JSON round-trip, Turns decodes as json.RawMessage("null") — assert
	// the raw token rather than nil equality.
	state := &presentationStub{rawTurns: nil, supported: false}
	h := NewHandler(state)

	req := httptest.NewRequest("GET", "/v1/repos/myproj/instances/tmux-agent/presentation", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var outer map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(w.Body).Decode(&outer))

	var supported bool
	require.NoError(t, json.Unmarshal(outer["supported"], &supported))
	assert.False(t, supported)

	// "turns" must be JSON null (not an array) for tmux instances.
	assert.Equal(t, "null", string(outer["turns"]))
}

func TestHandler_InstancePresentation_NotFound(t *testing.T) {
	state := &presentationStub{err: fmt.Errorf("%w: missing", ErrInstanceNotFound)}
	h := NewHandler(state)

	req := httptest.NewRequest("GET", "/v1/repos/myproj/instances/missing/presentation", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
}

// ---------------------------------------------------------------------------
// Permission endpoint tests
// ---------------------------------------------------------------------------

type permissionStub struct {
	DaemonState
	project string
	title   string
	choice  PermissionChoice
	err     error
}

func (s *permissionStub) ListInstances(_ string) []InstanceStatus { return nil }
func (s *permissionStub) EventStream() <-chan Event               { return make(chan Event) }
func (s *permissionStub) StartPlan(_, _, _, _ string) error       { return nil }
func (s *permissionStub) ListPlans(_ string) ([]taskstore.TaskEntry, error) {
	return nil, nil
}
func (s *permissionStub) ListTasks(_ string) ([]TaskStatus, error) { return nil, nil }
func (s *permissionStub) SendInstancePermissionResponse(project, title string, choice PermissionChoice) error {
	s.project = project
	s.title = title
	s.choice = choice
	return s.err
}

func TestHandler_InstancePermission_HappyPath(t *testing.T) {
	for _, tc := range []struct {
		name   string
		choice PermissionChoice
	}{
		{"allow_once", PermissionAllowOnce},
		{"allow_always", PermissionAllowAlways},
		{"reject", PermissionReject},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &permissionStub{}
			h := NewHandler(state)

			body := bytes.NewBufferString(fmt.Sprintf(`{"choice":%d}`, tc.choice))
			req := httptest.NewRequest("POST", "/v1/repos/myproj/instances/my-agent/permission", body)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNoContent, w.Code, "body: %s", w.Body.String())
			assert.Equal(t, "myproj", state.project)
			assert.Equal(t, "my-agent", state.title)
			assert.Equal(t, tc.choice, state.choice)
		})
	}
}

func TestHandler_InstancePermission_NotFound(t *testing.T) {
	state := &permissionStub{err: fmt.Errorf("%w: missing", ErrInstanceNotFound)}
	h := NewHandler(state)

	body := bytes.NewBufferString(`{"choice":0}`)
	req := httptest.NewRequest("POST", "/v1/repos/myproj/instances/missing/permission", body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
}

func TestHandler_InstancePermission_BadBody(t *testing.T) {
	state := &permissionStub{}
	h := NewHandler(state)

	body := bytes.NewBufferString(`not json`)
	req := httptest.NewRequest("POST", "/v1/repos/myproj/instances/my-agent/permission", body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
}

func TestHandler_InstancePermission_InvalidChoice(t *testing.T) {
	state := &permissionStub{}
	h := NewHandler(state)

	body := bytes.NewBufferString(`{"choice":99}`)
	req := httptest.NewRequest("POST", "/v1/repos/myproj/instances/my-agent/permission", body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, "", state.project, "invalid choices must be rejected before reaching the state layer")
}
