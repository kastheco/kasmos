package daemon

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSocketClient_ListTasks(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		createdAt := time.Date(2026, time.March, 30, 12, 0, 0, 0, time.UTC)
		planningAt := createdAt.Add(5 * time.Minute)
		implementingAt := createdAt.Add(10 * time.Minute)
		reviewingAt := createdAt.Add(15 * time.Minute)
		doneAt := createdAt.Add(20 * time.Minute)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/v1/repos/cms/tasks", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`[{"filename":"verify-every-fsm-transition","status":"reviewing","execution_state":{"execution_phase":"review","active_agent_type":"reviewer","active_wave":1},"branch":"task/verify-every-fsm-transition","pr_url":"https://github.com/kastheco/kasmos/pull/321","review_cycle":3,"description":"add daemon task-list API tests","topic":"core","created_at":"2026-03-30T12:00:00Z","implemented":"yes","planning_at":"2026-03-30T12:05:00Z","implementing_at":"2026-03-30T12:10:00Z","reviewing_at":"2026-03-30T12:15:00Z","done_at":"2026-03-30T12:20:00Z","goal":"keep daemon snapshots authoritative","clickup_task_id":"CU-123","latest_review_feedback":"wave metadata drifted in the sidebar","pr_review_decision":"APPROVED","pr_check_status":"SUCCESS"}]`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

		tasks, err := client.ListTasks("cms")
		require.NoError(t, err)
		require.Len(t, tasks, 1)
		assert.Equal(t, []api.TaskStatus{{
			Filename:             "verify-every-fsm-transition",
			Status:               "reviewing",
			Branch:               "task/verify-every-fsm-transition",
			PRURL:                "https://github.com/kastheco/kasmos/pull/321",
			ReviewCycle:          3,
			Description:          "add daemon task-list API tests",
			Topic:                "core",
			CreatedAt:            createdAt,
			Implemented:          "yes",
			PlanningAt:           planningAt,
			ImplementingAt:       implementingAt,
			ReviewingAt:          reviewingAt,
			DoneAt:               doneAt,
			Goal:                 "keep daemon snapshots authoritative",
			ClickUpTaskID:        "CU-123",
			LatestReviewFeedback: "wave metadata drifted in the sidebar",
			PRReviewDecision:     "APPROVED",
			PRCheckStatus:        "SUCCESS",
			ExecutionState: taskstore.ExecutionState{
				Phase:           "review",
				ActiveAgentType: "reviewer",
				ActiveWave:      1,
			},
		}}, tasks)
	})

	t.Run("empty list", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/repos/cms/tasks", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`[]`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

		tasks, err := client.ListTasks("cms")
		require.NoError(t, err)
		assert.NotNil(t, tasks)
		assert.Len(t, tasks, 0)
	})

	t.Run("non-2xx returns requested path", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/repos/cms/tasks", r.URL.Path)
			http.Error(w, "nope", http.StatusBadGateway)
		}))
		defer srv.Close()

		client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

		tasks, err := client.ListTasks("cms")
		require.Error(t, err)
		assert.Nil(t, tasks)
		assert.Contains(t, err.Error(), "/v1/repos/cms/tasks")
		assert.Contains(t, err.Error(), fmt.Sprintf("status %d", http.StatusBadGateway))
	})
}
