package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
